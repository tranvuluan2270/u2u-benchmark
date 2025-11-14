package internal

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/unicornultrafoundation/go-u2u/common"
	"github.com/unicornultrafoundation/go-u2u/crypto"
	"github.com/unicornultrafoundation/go-u2u/ethclient"
	"github.com/unicornultrafoundation/go-u2u/rpc"
)

type AccountSender struct {
	client     *ethclient.Client
	privateKey *ecdsa.PrivateKey
	from       common.Address
	chainID    *big.Int
	nonce      uint64 // Atomic nonce counter (use atomic operations only!)

	// Statistics per account (atomic)
	sent   uint64
	errors uint64
}

type KeyStore struct {
	Keys []string `json:"private_keys"`
}

// CreateOptimizedClient creates an ethclient with optimized HTTP connection pooling
// This allows thousands of concurrent requests without connection overhead
func CreateOptimizedClient(rpcURL string, maxConnections int) (*ethclient.Client, error) {
	// Create aggressive HTTP transport for high throughput
	// Force HTTP/1.1 by setting TLSNextProto to empty map to avoid HTTP/2 GOAWAY errors
	transport := &http.Transport{
		MaxIdleConns:          maxConnections,
		MaxIdleConnsPerHost:   maxConnections,
		MaxConnsPerHost:       maxConnections,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,                  // Keep connections alive
		DisableCompression:    true,                   // Reduce CPU overhead
		TLSHandshakeTimeout:   5 * time.Second,        // Faster TLS handshake timeout
		ExpectContinueTimeout: 500 * time.Millisecond, // Faster expect-continue
		ResponseHeaderTimeout: 5 * time.Second,        // Don't wait forever for headers
		// Disable HTTP/2 by setting TLSNextProto to empty map (forces HTTP/1.1)
		TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second, // Aggressive timeout for fast failure
	}

	// Create RPC client with our optimized HTTP client
	rpcClient, err := rpc.DialHTTPWithClient(rpcURL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to dial RPC: %v", err)
	}

	// Wrap with ethclient
	client := ethclient.NewClient(rpcClient)
	return client, nil
}

// GenerateAccounts creates new private keys
func GenerateAccounts(count int) ([]*ecdsa.PrivateKey, error) {
	keys := make([]*ecdsa.PrivateKey, count)

	fmt.Printf("Generating %d accounts...\n", count)
	for i := 0; i < count; i++ {
		key, err := crypto.GenerateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate key: %d: %v", i, err)
		}
		keys[i] = key

		address := crypto.PubkeyToAddress(key.PublicKey)
		fmt.Printf("Account %d: %s\n", i, address.Hex())
	}

	return keys, nil
}

// SavePrivateKeys saves keys to file
func SavePrivateKeys(keys []*ecdsa.PrivateKey, filename string) error {
	keyStore := KeyStore{
		Keys: make([]string, len(keys)),
	}

	for i, key := range keys {
		keyBytes := crypto.FromECDSA(key)
		keyStore.Keys[i] = hex.EncodeToString(keyBytes)
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(keyStore)
}

// LoadPrivateKeys loads keys from file
func LoadPrivateKeys(filename string) ([]*ecdsa.PrivateKey, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var keyStore KeyStore
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&keyStore)
	if err != nil {
		return nil, err
	}

	keys := make([]*ecdsa.PrivateKey, len(keyStore.Keys))
	for i, keyHex := range keyStore.Keys {
		keyBytes, err := hex.DecodeString(strings.TrimPrefix(keyHex, "0x"))
		if err != nil {
			return nil, fmt.Errorf("failed to decode key: %d: %v", i, err)
		}

		key, err := crypto.ToECDSA(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse key: %d: %v", i, err)
		}
		keys[i] = key
	}

	fmt.Printf("✅ Loaded %d private keys from %s\n", len(keys), filename)
	return keys, nil
}

// InitializeAccounts creates AccountSender instances
func InitializeAccounts(client *ethclient.Client, privateKeys []*ecdsa.PrivateKey) ([]*AccountSender, error) {
	ctx := context.Background()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %v", err)
	}

	fmt.Printf("Initializing %d accounts...\n", len(privateKeys))
	accounts := make([]*AccountSender, len(privateKeys))

	for i, key := range privateKeys {
		from := crypto.PubkeyToAddress(key.PublicKey)

		//Get current nonce
		nonce, err := client.PendingNonceAt(ctx, from)
		if err != nil {
			return nil, fmt.Errorf("failed to get nonce for account %d: %v", i, err)
		}

		// Get balance
		balance, err := client.BalanceAt(ctx, from, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get balance for account %d: %v", i, err)
		}

		accounts[i] = &AccountSender{
			client:     client,
			privateKey: key,
			from:       from,
			chainID:    chainID,
			nonce:      nonce,
		}

		balanceEth := new(big.Float).Quo(
			new(big.Float).SetInt(balance),
			new(big.Float).SetInt(big.NewInt(1e18)),
		)

		fmt.Printf("Account %d: %s (nonce: %d, balance: %.6f U2U)\n",
			i, from.Hex(), nonce, balanceEth)

		// Small delay to avoid overwhelming RPC during initialization
		// Only add delay every 10 accounts to balance speed vs stability
		if (i+1)%10 == 0 && i < len(privateKeys)-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	return accounts, nil
}

// CheckBalances verifies all accounts have sufficient balance
func CheckBalances(client *ethclient.Client, accounts []*AccountSender, minBalance *big.Int) error {
	ctx := context.Background()

	fmt.Printf("\nChecking account balances...\n")
	insufficientFunds := false

	for i, account := range accounts {
		balance, err := client.BalanceAt(ctx, account.from, nil)
		if err != nil {
			return fmt.Errorf("failed to check balance for account %d: %v", i, err)
		}

		if balance.Cmp(minBalance) < 0 {
			balanceEth := new(big.Float).Quo(
				new(big.Float).SetInt(balance),
				new(big.Float).SetInt(big.NewInt(1e18)),
			)
			minEth := new(big.Float).Quo(
				new(big.Float).SetInt(minBalance),
				new(big.Float).SetInt(big.NewInt(1e18)),
			)

			fmt.Printf("⚠️  Account %d has insufficient balance: %.6f U2U (need %.6f U2U)\n",
				i, balanceEth, minEth)
			insufficientFunds = true
		}
	}

	if insufficientFunds {
		return fmt.Errorf("some accounts have insufficient balance")
	}

	fmt.Println("✅ All accounts have sufficient balance")
	return nil
}

// GetNextNonce atomically gets and increments the nonce (lock-free)
// This allows multiple workers to pipeline transactions without blocking
func (a *AccountSender) GetNextNonce() uint64 {
	return atomic.AddUint64(&a.nonce, 1) - 1
}

// IncrementNonce increments the local nonce (for compatibility, but GetNextNonce already does this)
func (a *AccountSender) IncrementNonce() {
	atomic.AddUint64(&a.nonce, 1)
}

// ResyncNonce fetches nonce from blockchain and updates atomically
func (a *AccountSender) ResyncNonce(ctx context.Context) error {
	nonce, err := a.client.PendingNonceAt(ctx, a.from)
	if err != nil {
		return err
	}

	// Atomically store the new nonce
	atomic.StoreUint64(&a.nonce, nonce)
	return nil
}

// From returns the account address
func (a *AccountSender) From() common.Address {
	return a.from
}

// CurrentNonce returns the current local nonce without incrementing (thread-safe)
func (a *AccountSender) CurrentNonce() uint64 {
	return atomic.LoadUint64(&a.nonce)
}
