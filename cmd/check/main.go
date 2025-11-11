package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"u2u-tps-benchmark/internal"

	"github.com/unicornultrafoundation/go-u2u/ethclient"
)

func main() {
	// Command-line flags
	configFile := flag.String("config", "benchmark_config.json", "Path to config file")
	rpcURL := flag.String("rpc", "", "RPC endpoint URL (overrides config)")
	keysFile := flag.String("keys", "", "Path to private keys file (overrides config)")
	numAccounts := flag.Int("accounts", 0, "Number of accounts to check (0 = all, overrides config)")

	flag.Parse()

	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║     U2U Nonce Sync & Balance Check     ║")
	fmt.Println("╚════════════════════════════════════════╝")

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

	// Load private keys
	privateKeys, err := internal.LoadPrivateKeys(keysFilePath)
	if err != nil {
		log.Fatalf("\nFailed to load private keys: %v\n", err)
	}

	// Limit accounts based on config or flag
	accountsToUse := *numAccounts
	if accountsToUse == 0 && config.NumAccounts > 0 {
		accountsToUse = config.NumAccounts
	}

	if accountsToUse > 0 && accountsToUse < len(privateKeys) {
		fmt.Printf("🔍 Checking %d out of %d available accounts", accountsToUse, len(privateKeys))
		if *configFile != "" && *numAccounts == 0 {
			fmt.Printf(" (as per config)")
		}
		fmt.Printf("\n\n")
		privateKeys = privateKeys[:accountsToUse]
	} else {
		fmt.Printf("🔍 Checking %d accounts\n\n", len(privateKeys))
	}

	// Initialize accounts
	accounts, err := internal.InitializeAccounts(client, privateKeys)
	if err != nil {
		log.Fatalf("\nFailed to initialize accounts: %v", err)
	}

	ctx := context.Background()

	// Check nonce sync for each account
	fmt.Println(strings.Repeat("=", 100))
	fmt.Printf("%-8s | %-20s | %-15s | %-15s | %-15s | %-10s\n",
		"Account", "Address", "Confirmed Nonce", "Next Nonce", "Local Nonce", "Status")
	fmt.Println(strings.Repeat("=", 100))

	totalPending := 0
	allSynced := true

	for i, account := range accounts {
		// Get next nonce to use (confirmed txs only)
		nextConfirmedNonce, err := client.NonceAt(ctx, account.From(), nil)
		if err != nil {
			log.Printf("Failed to get confirmed nonce for account %d: %v", i, err)
			continue
		}

		// Get next nonce to use (includeing pending txs)
		nextPendingNonce, err := client.PendingNonceAt(ctx, account.From())
		if err != nil {
			log.Printf("Failed to get pending nonce for account %d: %v", i, err)
			continue
		}

		// Calculate last confirmed nonce (what explorer shows)
		lastConfirmedNonce := uint64(0)
		if nextConfirmedNonce > 0 {
			lastConfirmedNonce = nextConfirmedNonce - 1
		}

		// Get local nonce (from AccountSender)
		// Resync first to get the latest from blockchain
		account.ResyncNonce(ctx)
		localNonce := account.CurrentNonce()

		// Calculate pending transactions
		pendingTxs := int(nextPendingNonce - nextConfirmedNonce)
		if pendingTxs > 0 {
			totalPending += pendingTxs
		}

		// Determine status
		status := "✅ Synced"
		if nextPendingNonce > nextConfirmedNonce {
			status = fmt.Sprintf("⏳ %d pending", pendingTxs)
			allSynced = false
		}
		if localNonce > nextPendingNonce {
			status = "⚠️  Local ahead"
			allSynced = false
		}

		addrShort := account.From().Hex()[:8] + "..." + account.From().Hex()[len(account.From().Hex())-6:]
		fmt.Printf("%-8d | %-20s | %-15d | %-15d | %-15d | %-10s\n",
			i, addrShort, lastConfirmedNonce, nextPendingNonce, localNonce, status)
	}

	fmt.Println(strings.Repeat("=", 100))

	// Summary
	fmt.Printf("📊 Summary:\n")
	fmt.Printf("Total Accounts Checked: %d\n", len(accounts))
	fmt.Printf("Total Pending Transactions: %d\n", totalPending)
	if allSynced {
		fmt.Printf("Status: ✅ All accounts are synced\n")
	} else {
		fmt.Printf("Status: ⚠️  Some accounts have pending transactions\n")
		fmt.Printf("Note: Pending transactions will confirm when blocks are produced\n")
	}
}
