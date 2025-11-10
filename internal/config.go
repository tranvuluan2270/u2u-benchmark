package internal

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	// RPC Configuration
	RPCURL string `json:"rpc_url"`

	// Benchmark Settings
	NumAccounts     int `json:"num_accounts"`
	DurationSeconds int `json:"duration_seconds"` // Duration in seconds

	// Transaction Settings
	GasLimit       uint64 `json:"gas_limit"`
	TransferAmount string `json:"transfer_amount_wei"` // in wei

	// Account Management
	PrivateKeysFile string `json:"private_keys_file"`

	// Reporting
	ReportInterval int    `json:"report_interval_seconds"`
	OutputFile     string `json:"output_file"`

	// Advanced
	MaxRetries     int `json:"max_retries"`
	RetryDelay     int `json:"retry_delay_ms"`
	WarmupDuration int `json:"warmup_duration_seconds"`
}

// GetDuration returns the duration as time.Duration
func (c *Config) GetDuration() time.Duration {
	return time.Duration(c.DurationSeconds) * time.Second
}

func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func DefaultConfig() *Config {
	return &Config{
		RPCURL:          "https://rpc-nebulas-testnet.uniultra.xyz",
		NumAccounts:     10,
		DurationSeconds: 60, // Duration in seconds
		GasLimit:        21000,
		TransferAmount:  "1000000000000000", // 0.001 U2U
		ReportInterval:  1,
		OutputFile:      "benchmark_results.json",
		MaxRetries:      3,
		RetryDelay:      100,
		WarmupDuration:  5,
		PrivateKeysFile: "test_keys.json",
	}
}

func (c *Config) Save(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(c)
}
