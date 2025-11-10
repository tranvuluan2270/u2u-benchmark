# U2U TPS Benchmark Tool

A comprehensive benchmarking tool for measuring **Transactions Per Second (TPS)** on the U2U Network blockchain. This tool provides detailed performance metrics, account management, and real-time monitoring capabilities.


## 🎯 Overview

The U2U TPS Benchmark Tool is designed to:

- **Measure network throughput** by sending parallel transactions from multiple accounts
- **Track real-time metrics** including submitted TPS, confirmed TPS, latency, and error rates
- **Manage test accounts** with tools for key generation, funding, and status checking
- **Generate detailed reports** with per-account statistics and historical TPS data

### Key Features

- ✅ **Parallel transaction execution** from multiple accounts simultaneously
- ✅ **Round-robin transfers** (Account i → Account i+1) for balanced load distribution
- ✅ **Real-time monitoring** with configurable reporting intervals
- ✅ **Comprehensive metrics** including RPC accept rate, confirmation rate, and latency
- ✅ **Account management** tools for key generation, funding, and status verification
- ✅ **JSON result export** for further analysis

## 📦 Prerequisites

- **Go 1.19+** installed and configured
- **U2U Network RPC access** (testnet or mainnet)
- **Funder account** with sufficient balance (for funding test accounts)
- **Network connectivity** to the RPC endpoint

## 🚀 Installation

1. **Clone or navigate to the project directory:**
   ```bash
   cd u2u-tps-benchmark
   ```

2. **Install dependencies:**
   ```bash
   go mod download
   ```

3. **Verify installation:**
   ```bash
   go run cmd/benchmark/main.go -generate-config
   ```

## 🏃 Quick Start

### Step 1: Generate Test Accounts

Create private keys for benchmarking:

```bash
go run cmd/keygen/main.go -accounts 10 -output test_keys.json
```

**Options:**
- `-accounts`: Number of accounts to generate (default: 10)
- `-output`: Output file path (default: `test_keys.json`)
- `-overwrite`: Overwrite existing file if it exists

**Output:**
```
╔════════════════════════════════════════╗
║            U2U Key Generator           ║
╚════════════════════════════════════════╝

🔑 Generating 10 private keys...
✅ Private keys saved to test_keys.json
⚠️  Remember to fund these accounts before running the benchmark.
```

### Step 2: Fund Test Accounts

Set your funder's private key and fund the accounts:

**Windows PowerShell:**
```powershell
$env:FUNDER_PRIVATE_KEY="your_private_key_hex_without_0x"
go run cmd/fund/main.go
```

**Linux/Mac:**
```bash
export FUNDER_PRIVATE_KEY="your_private_key_hex_without_0x"
go run cmd/fund/main.go
```

**Options:**
- `-config`: Path to config file (default: `benchmark_config.json`)
- `-amount`: Amount to fund per account in U2U (default: `1`)
- `-accounts`: Number of accounts to fund (0 = all, default: 0)
- `-rpc`: RPC endpoint URL (overrides config)
- `-keys`: Path to private keys file (overrides config)

**Output:**
```
╔══════════════════════════════════════╗
║          U2U Account Funding         ║
╚══════════════════════════════════════╝

👤 Funder Address: 0x...
💰 Funder Balance: 100.000000 U2U
💵 Amount per account: 1 U2U
💰 Total needed: 10.00 U2U

💸 Starting to fund accounts...
✅ Account  0: 0x... (tx: 0x...)
✅ Account  1: 0x... (tx: 0x...)
...
✅ Successfully funded 10/10 accounts
```

### Step 3: Check Account Status (Optional)

Verify accounts are ready before benchmarking:

```bash
go run cmd/check/main.go
```

**Options:**
- `-config`: Path to config file (default: `benchmark_config.json`)
- `-accounts`: Number of accounts to check (0 = all, default: 0)
- `-rpc`: RPC endpoint URL (overrides config)
- `-keys`: Path to private keys file (overrides config)

**Output:**
```
╔════════════════════════════════════════╗
║     U2U Nonce Sync & Balance Check     ║
╚════════════════════════════════════════╝

Account  | Address              | Confirmed Nonce | Pending Nonce   | Local Nonce     | Status
0        | 0xa15240...7Ea214    | 8268            | 8269            | 8269            | ✅ Synced
1        | 0x9D6f42...1bAc41    | 8272            | 8272            | 8272            | ✅ Synced
...

📊 Summary:
  Total Accounts Checked: 10
  Total Pending Transactions: 0
  Status: ✅ All accounts are synced
```

### Step 4: Run Benchmark

Execute the TPS benchmark:

```bash
go run cmd/benchmark/main.go -config benchmark_config.json
```

**Options:**
- `-config`: Path to config file (default: uses defaults)
- `-keys`: Path to private keys file (default: `test_keys.json`)
- `-accounts`: Number of accounts to use (default: 10)
- `-rpc`: RPC endpoint URL (default: testnet)
- `-duration`: Benchmark duration in seconds (default: 60)
- `-generate-config`: Generate default config file

**Output:**
```
╔════════════════════════════════════════════╗
║        U2U Blockchain TPS Benchmark        ║
╚════════════════════════════════════════════╝

🚀 Starting main benchmark...
Time       | Submitted TPS | Total Submitted | Errors     | Avg Latency
00:01      | 64            | 64              | 0          | 75ms        
00:02      | 62            | 126             | 0          | 79ms        
...

📊 Overall Statistics:
  Duration:           10s
  Total Submitted:    669 transactions
  Total Errors:       0 transactions
  RPC Accept Rate:    100.00%
```

## 📖 Command Reference

### Generate Keys (`cmd/generate-keys`)

Creates new private keys for benchmarking accounts.

```bash
go run cmd/generate-keys/main.go [flags]
```

**Flags:**
- `-accounts int`: Number of accounts to generate (default: 10)
- `-output string`: Output file path (default: `test_keys.json`)
- `-overwrite`: Overwrite existing file if it exists

**Example:**
```bash
go run cmd/generate-keys/main.go -accounts 20 -output my_keys.json -overwrite
```

### Fund Accounts (`cmd/fund`)

Funds test accounts with U2U tokens from a funder account.

```bash
go run cmd/fund/main.go [flags]
```

**Flags:**
- `-config string`: Path to config file (default: `benchmark_config.json`)
- `-amount string`: Amount to fund per account in U2U (default: `1`)
- `-accounts int`: Number of accounts to fund (0 = all, default: 0)
- `-rpc string`: RPC endpoint URL (overrides config)
- `-keys string`: Path to private keys file (overrides config)

**Environment Variable:**
- `FUNDER_PRIVATE_KEY`: Private key of the funding account (hex, without 0x prefix)

**Example:**
```bash
export FUNDER_PRIVATE_KEY="abc123..."
go run cmd/fund/main.go -amount 2.5 -accounts 5
```

### Check Accounts (`cmd/check`)

Inspects account status including nonces and balances.

```bash
go run cmd/check/main.go [flags]
```

**Flags:**
- `-config string`: Path to config file (default: `benchmark_config.json`)
- `-accounts int`: Number of accounts to check (0 = all, default: 0)
- `-rpc string`: RPC endpoint URL (overrides config)
- `-keys string`: Path to private keys file (overrides config)

**What it shows:**
- **Confirmed Nonce**: Last confirmed transaction's nonce (matches blockchain explorer)
- **Pending Nonce**: Next nonce to use (includes pending transactions)
- **Local Nonce**: Tool's internal nonce counter
- **Status**: Sync status (Synced, Pending, or Local ahead)

**Example:**
```bash
go run cmd/check/main.go -accounts 5
```

### Run Benchmark (`cmd/benchmark`)

Executes the TPS benchmark test.

```bash
go run cmd/benchmark/main.go [flags]
```

**Flags:**
- `-config string`: Path to config file
- `-keys string`: Path to private keys file (default: `test_keys.json`)
- `-accounts int`: Number of accounts to use (default: 10)
- `-rpc string`: RPC endpoint URL (default: testnet)
- `-duration int`: Benchmark duration in seconds (default: 60)
- `-generate-config`: Generate default config file

**Example:**
```bash
go run cmd/benchmark/main.go -config benchmark_config.json -duration 120
```

## ⚙️ Configuration

### Config File: `benchmark_config.json`

```json
{
  "rpc_url": "https://rpc-nebulas-testnet.uniultra.xyz",
  "num_accounts": 10,
  "duration_seconds": 60,
  "gas_limit": 21000,
  "transfer_amount_wei": "1000000000000000",
  "private_keys_file": "test_keys.json",
  "report_interval_seconds": 1,
  "output_file": "benchmark_results.json",
  "max_retries": 3,
  "retry_delay_ms": 100,
  "warmup_duration_seconds": 5
}
```

### Configuration Parameters

| Parameter                 | Description                 | Default                    | Notes                                |
|---------------------------|-----------------------------|----------------------------|--------------------------------------|
| `rpc_url`                 | RPC endpoint URL            | Testnet                    | Use mainnet for production testing   |
| `num_accounts`            | Number of parallel accounts | 10                         | More accounts = higher potential TPS |
| `duration_seconds`        | Benchmark duration          | 60                         | Longer = more stable averages        |
| `gas_limit`               | Gas per transaction         | 21000                      | Standard transfer gas limit          |
| `transfer_amount_wei`     | Amount per transfer         | `"1000000000000000"`       | 0.001 U2U (balance-neutral)          |
| `private_keys_file`       | Path to keys file           | `"test_keys.json"`         | Generated by `generate-keys`         |
| `report_interval_seconds` | Metrics report frequency    | 1                          | How often to print stats             |
| `output_file`             | Results JSON file           | `"benchmark_results.json"` | Detailed results export              |
| `max_retries`             | Max retry attempts          | 3                          | For failed transactions              |
| `retry_delay_ms`          | Retry delay                 | 100                        | Milliseconds between retries         |
| `warmup_duration_seconds` | Warmup period               | 5                          | Excluded from metrics                |

### Generate Default Config

```bash
go run cmd/benchmark/main.go -generate-config
```

This creates `benchmark_config.json` with default values that you can customize.

## 📊 Understanding Results

### Real-Time Metrics

During the benchmark, you'll see a table updated every second:

```
Time       | Submitted TPS | Total Submitted | Errors     | Avg Latency
00:01      | 64            | 64              | 0          | 75ms        
00:02      | 62            | 126             | 0          | 79ms        
```

**Columns:**
- **Time**: Elapsed time (MM:SS format)
- **Submitted TPS**: Transactions sent to RPC in this interval
- **Total Submitted**: Cumulative transactions sent
- **Errors**: Number of errors in this interval
- **Avg Latency**: Average RPC response time

### Final Summary

After the benchmark completes:

```
📊 Overall Statistics:
  Duration:           10s
  Total Submitted:    669 transactions
  Total Errors:       0 transactions
  RPC Accept Rate:    100.00%

⚡ Submitted TPS Metrics:
  Average TPS:        65.48
  Peak TPS:           70
  Minimum TPS:        62
  Median TPS:         68

⏱️  Latency:
  Average Latency:    74ms

👥 Per-Account Statistics:
  Account  0:    137 sent,    0 errors (100.0%)
  Account  1:    135 sent,    0 errors (100.0%)
  ...
```

### JSON Results File

Detailed results are saved to `benchmark_results.json`:

```json
{
  "timestamp": "2025-01-15T10:30:00Z",
  "config": {
    "rpc_url": "https://rpc-nebulas-testnet.uniultra.xyz",
    "duration_seconds": 10,
    "num_accounts": 5
  },
  "total_submitted": 669,
  "total_errors": 0,
  "rpc_accept_rate": 100.0,
  "average_submitted_tps": 65.48,
  "peak_submitted_tps": 70,
  "min_submitted_tps": 62,
  "median_submitted_tps": 68,
  "average_latency_ms": 74,
  "submitted_tps_history": [64, 62, 68, ...],
  "account_statistics": [
    {
      "account_id": 0,
      "address": "0xa15240...7Ea214",
      "sent": 137,
      "errors": 0,
      "success_rate": 100.0
    }
  ]
}
```

### Key Metrics Explained

- **RPC Accept Rate**: Percentage of transactions accepted by the RPC node (100% = all accepted)
- **Submitted TPS**: Transactions sent to the network (RPC layer performance)
- **Latency**: Time from sending to RPC response (network + RPC processing time)

### Checking Transaction Confirmations

The benchmark focuses on submission metrics. To check how many transactions confirmed on-chain, run the check tool after the benchmark:

```bash
go run cmd/check/main.go
```

This shows each account's confirmed nonce, which tells you how many transactions have been confirmed. Compare the nonces before and after the benchmark to calculate confirmed transactions.

## 🐛 Troubleshooting

### "Failed to load private keys"

**Solution:** Generate keys first:
```bash
go run cmd/generate-keys/main.go -accounts 10
```

### "FUNDER_PRIVATE_KEY environment variable is not set"

**Solution:** Set the environment variable before running fund:
```bash
# Windows PowerShell
$env:FUNDER_PRIVATE_KEY="your_key_hex"

# Linux/Mac
export FUNDER_PRIVATE_KEY="your_key_hex"
```

### "Insufficient balance" errors

**Solution:** Fund accounts with sufficient balance:
```bash
go run cmd/fund/main.go -amount 2.0
```

Each account needs at least 0.1 U2U for ~50 transactions.

### "Failed to connect to RPC"

**Solution:**
- Verify RPC URL is correct
- Check network connectivity
- Try alternative RPC endpoint
- Ensure RPC endpoint is accessible

### High transaction failure rate

**Solution:**
- Reduce number of accounts: `-accounts 5`
- Check account balances: `go run cmd/check/main.go`
- Verify RPC stability
- Increase gas price (if configurable)

### Low confirmed TPS

**Possible causes:**
- Network congestion (many pending transactions)
- Slow block production
- RPC node synchronization issues

**Check pending transactions:**
```bash
go run cmd/check/main.go
```

If you see many pending transactions, wait for them to confirm before running another benchmark.

### Nonce synchronization issues

**Symptoms:** "Local ahead" status in check tool

**Solution:**
- Wait for pending transactions to confirm
- The tool automatically resyncs nonces on errors
- If persistent, regenerate keys and start fresh

## 📁 Project Structure

```
u2u-tps-benchmark/
├── cmd/
│   ├── benchmark/          # Main benchmark tool
│   │   └── main.go
│   ├── fund/               # Account funding tool
│   │   └── main.go
│   ├── check/              # Account status checker
│   │   └── main.go
│   └── generate-keys/      # Key generation tool
│       └── main.go
├── internal/
│   ├── account.go          # Account management
│   ├── benchmark.go        # Benchmark engine
│   └── config.go          # Configuration handling
├── benchmark_config.json   # Default configuration
├── test_keys.json          # Generated private keys (gitignored)
├── benchmark_results.json  # Benchmark results (gitignored)
├── .gitignore             # Security rules
├── go.mod                 # Go module file
├── go.sum                 # Dependency checksums
└── README.md              # This file
```

## 🔐 Security Notes

- ⚠️ **Private keys are sensitive**: Never commit `test_keys.json` to version control
- ⚠️ **Use testnet for testing**: Never use mainnet keys in this tool
- ⚠️ **Secure storage**: Store keys in a secure location outside the repo
- ⚠️ **Environment variables**: Don't log or expose `FUNDER_PRIVATE_KEY`

The `.gitignore` file automatically excludes:
- `test_keys.json` and `*_keys.json`
- `benchmark_results.json`
- `.env` files

## 💡 Best Practices

1. **Start small**: Begin with 5-10 accounts, then scale up
2. **Run longer tests**: Use at least 60 seconds for stable averages
3. **Monitor balances**: Check account balances before each run
4. **Verify sync**: Use `cmd/check` to ensure accounts are synced
5. **Use stable RPC**: Choose reliable RPC endpoints for consistent results
6. **Save results**: Keep `benchmark_results.json` for trend analysis
7. **Test different times**: Network conditions vary throughout the day

## 📈 Performance Expectations

| Network        | Expected Submitted TPS | Expected Confirmed TPS | Notes                        |
|----------------|------------------------|------------------------|------------------------------|
| **Testnet**    | 50-70                  | 10-20                  | Lower load, good for testing |
| **Mainnet**    | 30-60                  | 5-15                   | Higher load, real conditions |
| **Local Node** | 100+                   | 100+                   | No network latency           |

*Actual TPS depends on network conditions, RPC performance, block production rate, and configuration.*

## 🔄 Typical Workflow

1. **Generate keys** (one-time setup):
   ```bash
   go run cmd/generate-keys/main.go -accounts 10
   ```

2. **Fund accounts** (before each test session):
   ```bash
   export FUNDER_PRIVATE_KEY="..."
   go run cmd/fund/main.go
   ```

3. **Check status** (verify readiness):
   ```bash
   go run cmd/check/main.go
   ```

4. **Run benchmark**:
   ```bash
   go run cmd/benchmark/main.go -config benchmark_config.json
   ```

5. **Review results** in console and `benchmark_results.json`

6. **Repeat** steps 3-5 as needed

