package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"u2u-tps-benchmark/internal"
)

func main() {
	accounts := flag.Int("accounts", 10, "Number of accounts to generate")
	output := flag.String("output", "test_keys.json", "Output file for the generated private keys")
	overwrite := flag.Bool("overwrite", false, "Overwrite the output file if it already exists")

	flag.Parse()

	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║            U2U Key Generator           ║")
	fmt.Println("╚════════════════════════════════════════╝")

	if *accounts <= 0 {
		log.Fatalf("\nNumber of accounts must be greater than zero")
	}

	if !*overwrite {
		if _, err := os.Stat(*output); err == nil {
			log.Fatalf("\nOutput file %s already exists. Use -overwrite to replace it.", *output)
		}
	}

	fmt.Printf("\n🔑 Generating %d private keys...\n", *accounts)
	keys, err := internal.GenerateAccounts(*accounts)
	if err != nil {
		log.Fatalf("\nFailed to generate keys: %v", err)
	}

	if err := internal.SavePrivateKeys(keys, *output); err != nil {
		log.Fatalf("\nFailed to save keys: %v", err)
	}

	fmt.Printf("\n✅ Private keys saved to %s\n", *output)
	fmt.Println("⚠️  Remember to fund these accounts before running the benchmark.")
	fmt.Println("   You can use `go run cmd/fund/main.go` to fund them.")
}
