package main

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"math/big"
	"time"

	"u2u-tps-benchmark/internal"
)

func main() {
	// Command-line flags
	configFile := flag.String("config", "", "Path to config file")
	generateConfig := flag.Bool("generate-config", false, "Generate default config file")
	keysFile := flag.String("keys", "test_keys.json", "Path to private keys file")
	numAccounts := flag.Int("accounts", 10, "Number of accounts to use when no config file is supplied")
	rpcURL := flag.String("rpc", "https://rpc-nebulas-testnet.uniultra.xyz", "RPC endpoint URL")
	duration := flag.Int("duration", 60, "Benchmark duration in seconds")

	flag.Parse()

	// Generate default config
	if *generateConfig {
		config := internal.DefaultConfig()
		err := config.Save("benchmark_config.json")
		if err != nil {
			log.Fatalf("\nFailed to save config: %v", err)
		}
		fmt.Println("Default config file generated: benchmark_config.json")
		fmt.Println("Edit this file and run with: -config benchmark_config.json")
		return
	}

	// Load or create config
	var config *internal.Config
	var err error

	if *configFile != "" {
		config, err = internal.LoadConfig(*configFile)
		if err != nil {
			log.Fatalf("\nFailed to load config: %v", err)
		}
	} else {
		config = internal.DefaultConfig()
		config.RPCURL = *rpcURL
		config.NumAccounts = *numAccounts
		config.DurationSeconds = *duration
		config.PrivateKeysFile = *keysFile
	}

	fmt.Println("╔════════════════════════════════════════════╗")
	fmt.Println("║        U2U Blockchain TPS Benchmark        ║")
	fmt.Println("╚════════════════════════════════════════════╝")

	// Connect to RPC with optimized connection pool
	fmt.Printf("🔌 Connecting to RPC: %s\n", config.RPCURL)
	// Use connection pool that supports 2000+ concurrent connections
	client, err := internal.CreateOptimizedClient(config.RPCURL, 2000)
	if err != nil {
		log.Fatalf("\nFailed to connect to RPC: %v", err)
	}
	defer client.Close()

	// Verify connection
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Fatalf("\nFailed to get chain ID: %v", err)
	}
	fmt.Printf("✅ Connected to chain ID: %s\n", chainID.String())

	// Load private keys
	var privateKeys []*ecdsa.PrivateKey

	// Load existing keys
	privateKeys, err = internal.LoadPrivateKeys(config.PrivateKeysFile)
	if err != nil {
		log.Fatalf("\nFailed to load private keys: %v\n", err)
		log.Fatalf("\nHint: Use `go run cmd/generate-keys/main.go -accounts %d -output %s` to create keys", config.NumAccounts, config.PrivateKeysFile)
	}

	// Limit to num_accounts if specified and config file is used
	if *configFile != "" && config.NumAccounts > 0 && config.NumAccounts < len(privateKeys) {
		fmt.Printf("Using %d out of %d available accounts (as per config)\n", config.NumAccounts, len(privateKeys))
		privateKeys = privateKeys[:config.NumAccounts]
	}

	// Initialize accounts
	accounts, err := internal.InitializeAccounts(client, privateKeys)
	if err != nil {
		log.Fatalf("\nFailed to initialize accounts: %v", err)
	}

	// Check balances
	minBalance := big.NewInt(1e17) // 0.1 U2U minimum (sufficient for ~50 transactions)
	err = internal.CheckBalances(client, accounts, minBalance)
	if err != nil {
		log.Fatalf("\nFailed to check balances: %v", err)
	}

	// Create and start benchmark
	benchmark, err := internal.NewBenchmark(config, client, accounts)
	if err != nil {
		log.Fatalf("\nFailed to create benchmark: %v", err)
	}

	// Confirmation prompt
	fmt.Println("⚡ Ready to start benchmark. Press Ctrl+C to abort, or wait 5 seconds...")
	time.Sleep(5 * time.Second)

	benchmark.Start()
}
