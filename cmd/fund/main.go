package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"u2u-tps-benchmark/internal"

	"github.com/unicornultrafoundation/go-u2u/core/types"
	"github.com/unicornultrafoundation/go-u2u/crypto"
	"github.com/unicornultrafoundation/go-u2u/ethclient"
)

func main() {
	// Command-line flags
	configFile := flag.String("config", "benchmark_config.json", "Path to config file")
	rpcURL := flag.String("rpc", "", "RPC endpoint URL (overrides config)")
	keysFile := flag.String("keys", "", "Path to private keys file (overrides config)")
	amount := flag.String("amount", "1", "Amount to fund per account in U2U")
	numAccounts := flag.Int("accounts", 0, "Number of accounts to fund (0 = all, overrides config)")

	flag.Parse()

	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║          U2U Account Funding         ║")
	fmt.Println("╚══════════════════════════════════════╝")

	// Get funder private key from environment
	funderPrivateKeyHex := os.Getenv("FUNDER_PRIVATE_KEY")
	if funderPrivateKeyHex == "" {
		log.Fatal("\nFUNDER_PRIVATE_KEY environment variable is not set")
	}

	// Load or create config
	var config *internal.Config
	var err error

	if *configFile != "" {
		config, err = internal.LoadConfig(*configFile)
		if err != nil {
			// If config file doesn't exist, use defaults
			config = internal.DefaultConfig()
		}
	} else {
		config = internal.DefaultConfig()
	}

	// Use config values, but allow flags to override
	rpcEndpoint := config.RPCURL
	if *rpcURL != "" {
		rpcEndpoint = *rpcURL // Flag overrides config
	}

	keysFilePath := config.PrivateKeysFile
	if *keysFile != "" {
		keysFilePath = *keysFile // Flag overrides config
	}

	// Connect to RPC
	fmt.Printf("🔌 Connecting to RPC: %s\n", rpcEndpoint)
	client, err := ethclient.Dial(rpcEndpoint)
	if err != nil {
		log.Fatalf("\nFailed to connect to RPC: %v", err)
	}
	defer client.Close()

	// Verify connection
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Fatalf("\nFailed to get chain ID: %v", err)
	}
	fmt.Printf("✅ Connected to chain ID: %s\n\n", chainID.String())

	// Parse funder private key
	funderKey, err := crypto.HexToECDSA(funderPrivateKeyHex)
	if err != nil {
		log.Fatalf("\nInvalid private key: %v", err)
	}

	funderAddr := crypto.PubkeyToAddress(funderKey.PublicKey)
	fmt.Printf("👤 Funder Address: %s\n", funderAddr.Hex())

	// Check funder balance
	balance, err := client.BalanceAt(context.Background(), funderAddr, nil)
	if err != nil {
		log.Fatalf("\nFailed to check funder balance: %v", err)
	}
	balanceU2U := new(big.Float).Quo(
		new(big.Float).SetInt(balance),
		new(big.Float).SetInt(big.NewInt(1e18)),
	)
	fmt.Printf("💰 Funder Balance: %.6f U2U\n\n", balanceU2U)

	// Load test account keys
	testKeys, err := internal.LoadPrivateKeys(keysFilePath)
	if err != nil {
		log.Fatalf("\nFailed to load test keys: %v", err)
	}

	// Limit accounts based on config or flag
	accountsToFund := *numAccounts
	if accountsToFund == 0 && config.NumAccounts > 0 {
		accountsToFund = config.NumAccounts
	}

	if accountsToFund > 0 && accountsToFund < len(testKeys) {
		fmt.Printf("💸 Funding %d out of %d available accounts", accountsToFund, len(testKeys))
		if *configFile != "" && *numAccounts == 0 {
			fmt.Printf(" (as per config)")
		}
		fmt.Printf("\n")
		testKeys = testKeys[:accountsToFund]
	} else {
		fmt.Printf("💸 Funding %d accounts\n", len(testKeys))
	}

	// Parse funding amount
	amountFloat, _ := new(big.Float).SetString(*amount)
	totalNeeded := new(big.Float).Mul(
		big.NewFloat(float64(len(testKeys))),
		amountFloat,
	)
	fmt.Printf("💵 Amount per account: %s U2U\n", *amount)
	fmt.Printf("💰 Total needed: %.2f U2U\n\n", totalNeeded)

	// Check if funder has sufficient balance
	if balanceU2U.Cmp(totalNeeded) < 0 {
		log.Fatalf("\n❌ Funder has insufficient balance! Need %.2f U2U, have %.6f U2U", totalNeeded, balanceU2U)
	}

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatalf("\nFailed to get gas price: %v", err)
	}

	// Get starting nonce
	nonce, err := client.PendingNonceAt(context.Background(), funderAddr)
	if err != nil {
		log.Fatalf("\nFailed to get nonce: %v", err)
	}

	// Convert amount to wei
	amountWei := new(big.Int)
	amountWei.SetString(*amount+"000000000000000000", 10)

	// Start funding
	fmt.Println("💸 Starting to fund accounts...")

	ctx := context.Background()
	successCount := 0
	errorCount := 0

	for i, key := range testKeys {
		to := crypto.PubkeyToAddress(key.PublicKey)

		// Create transaction
		tx := types.NewTransaction(nonce, to, amountWei, 21000, gasPrice, nil)
		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), funderKey)
		if err != nil {
			fmt.Printf("❌ Account %2d: %s - Failed to sign: %v\n", i, to.Hex(), err)
			errorCount++
			continue
		}

		// Send transaction
		err = client.SendTransaction(ctx, signedTx)
		if err != nil {
			fmt.Printf("❌ Account %2d: %s - Failed to send: %v\n", i, to.Hex(), err)
			errorCount++
		} else {
			// Truncate transaction hash for display (first 10 + last 8 chars)
			txHash := signedTx.Hash().Hex()
			txHashShort := txHash[:10] + "..." + txHash[len(txHash)-8:]
			fmt.Printf("✅ Account %2d: %s (tx: %s)\n", i, to.Hex(), txHashShort)
			successCount++
		}

		nonce++
	}
	fmt.Printf("\n✅ Successfully funded %d/%d accounts\n", successCount, len(testKeys))
}
