package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/unicornultrafoundation/go-u2u/core/types"
	"github.com/unicornultrafoundation/go-u2u/ethclient"
)

type Benchmark struct {
	config   *Config
	client   *ethclient.Client
	accounts []*AccountSender

	// Transaction settings
	transferValue *big.Int
	gasPrice      *big.Int

	// Metrics
	sentCount    uint64 // Submitted to RPC
	errorCount   uint64
	totalLatency int64 // nanoseconds

	// Per-second metrics
	tpsHistory []uint64

	// Nonce resync queue (buffered to avoid blocking)
	resyncQueue chan *AccountSender

	// Control
	stopChan        chan struct{} // For sender workers
	stopMetricsChan chan struct{} // For metrics reporter
	wg              sync.WaitGroup

	// Start time
	startTime time.Time
}

func NewBenchmark(config *Config, client *ethclient.Client, accounts []*AccountSender) (*Benchmark, error) {
	transferValue := new(big.Int)
	transferValue.SetString(config.TransferAmount, 10)

	// Get current gas price
	ctx := context.Background()
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %v", err)
	}

	fmt.Printf("\nBenchmark Configuration:\n")
	fmt.Printf("  Transfer Mode: Round-Robin (Account i → Account i+1)\n")
	fmt.Printf("  Transfer Value: %s wei\n", transferValue.String())
	fmt.Printf("  Gas Price: %s wei\n", gasPrice.String())
	fmt.Printf("  Gas Limit: %d\n", config.GasLimit)
	fmt.Printf("  Duration: %v\n", config.GetDuration())
	fmt.Printf("  Accounts: %d\n", len(accounts))
	fmt.Printf("  Concurrent Senders/Account: %d \n", config.ConcurrentSendersPerAccount)

	return &Benchmark{
		config:          config,
		client:          client,
		accounts:        accounts,
		transferValue:   transferValue,
		gasPrice:        gasPrice,
		stopChan:        make(chan struct{}),
		stopMetricsChan: make(chan struct{}),
		tpsHistory:      make([]uint64, 0),
		resyncQueue:     make(chan *AccountSender, 1000), // Buffer for nonce resync requests (large to handle bursts)
	}, nil
}

func (b *Benchmark) Start() {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("STARTING BENCHMARK")
	fmt.Println(strings.Repeat("=", 70))

	b.startTime = time.Now()

	fmt.Printf("\n🚀 Starting main benchmark...")

	// Multiple concurrent senders per account for pipelining
	concurrentSenders := b.config.ConcurrentSendersPerAccount
	if concurrentSenders <= 0 {
		concurrentSenders = 1 // Fallback to at least 1
	}

	totalWorkers := len(b.accounts) * concurrentSenders
	fmt.Printf("\nWorkers: %d accounts × %d senders = %d concurrent workers\n",
		len(b.accounts), concurrentSenders, totalWorkers)

	// Start multiple sender goroutines per account
	for i, account := range b.accounts {
		for w := 0; w < concurrentSenders; w++ {
			b.wg.Add(1)
			go b.senderWorker(i, account)
		}
	}

	// Start metrics reporter
	go b.metricsReporter()

	// Run for specified duration
	time.Sleep(b.config.GetDuration())

	// Capture metrics EXACTLY at duration end (before stopping senders)
	finalSent := atomic.LoadUint64(&b.sentCount)
	finalErrors := atomic.LoadUint64(&b.errorCount)
	finalLatency := atomic.LoadInt64(&b.totalLatency)

	// Stop sender workers immediately (no more transactions)
	close(b.stopChan)
	b.wg.Wait()

	// Give metrics reporter time to print the final line
	time.Sleep(150 * time.Millisecond)

	// Stop metrics reporter
	close(b.stopMetricsChan)

	fmt.Println("\n⏸️  Benchmark stopped")

	b.printFinalReport(finalSent, finalErrors, finalLatency)
}

func (b *Benchmark) senderWorker(id int, account *AccountSender) {
	defer b.wg.Done()

	// Ultra-minimal jitter for maximum throughput
	if id > 0 {
		jitter := time.Duration(rand.Intn(2)) * time.Millisecond // 0-2ms only
		time.Sleep(jitter)
	}

	ctx := context.Background()
	consecutiveErrors := 0
	const maxRetriesPerNonce = 2 // Minimal retries for maximum throughput
	firstTransaction := true

	for {
		select {
		case <-b.stopChan:
			return
		default:
			var err error
			var latency time.Duration

			// Retry logic: attempt same nonce multiple times before giving up
			// Give first transaction extra retries to handle initial congestion
			maxRetries := maxRetriesPerNonce
			if firstTransaction {
				maxRetries = 8 // More retries for initial connection
			}

			for retry := 0; retry < maxRetries; retry++ {
				start := time.Now()
				err = b.sendTransaction(ctx, id, account)
				latency = time.Since(start)

				if err == nil {
					// Success! Nonce already incremented by GetNextNonce()
					atomic.AddUint64(&b.sentCount, 1)
					atomic.AddInt64(&b.totalLatency, latency.Nanoseconds())
					atomic.AddUint64(&account.sent, 1)
					consecutiveErrors = 0
					firstTransaction = false
					break
				}

				// Check if it's a nonce-related error
				if isNonceError(err) {
					// Nonce already incremented by GetNextNonce() - transaction likely submitted
					// No resync needed - atomic nonces handle this automatically
					consecutiveErrors = 0
					firstTransaction = false
					break
				}

				// For non-nonce errors (network, timeout), retry with same nonce
				// Ultra-minimal backoff for maximum throughput
				if retry < maxRetries-1 {
					time.Sleep(1 * time.Millisecond) // 1ms backoff only
				}
			}

			// If all retries failed, count as error
			// But don't count nonce errors - they usually mean TX was already submitted
			if err != nil {
				if !isNonceError(err) {
					// Only count non-nonce errors (real failures)
					atomic.AddUint64(&b.errorCount, 1)
					atomic.AddUint64(&account.errors, 1)
					consecutiveErrors++

					// Ultra-minimal backoff, maximize throughput
					if consecutiveErrors < 5 {
						time.Sleep(5 * time.Millisecond) // 5ms backoff
					}
					// Note: Nonce resync workers disabled - atomic nonces handle everything
				} else {
					// Nonce error - don't count as failure, reset consecutive error counter
					consecutiveErrors = 0
				}
			}
		}
	}
}

// Helper function to detect nonce-related errors
func isNonceError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "nonce") ||
		strings.Contains(errStr, "nonce too low") ||
		strings.Contains(errStr, "already known") ||
		strings.Contains(errStr, "replacement transaction underpriced")
}

func (b *Benchmark) sendTransaction(ctx context.Context, accountID int, account *AccountSender) error {
	nonce := account.GetNextNonce()

	// Round-robin: Account i sends to Account (i+1) % total_accounts
	targetIndex := (accountID + 1) % len(b.accounts)
	targetAddress := b.accounts[targetIndex].from

	tx := types.NewTransaction(
		nonce,
		targetAddress,
		b.transferValue,
		b.config.GasLimit,
		b.gasPrice,
		nil,
	)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(account.chainID), account.privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	err = account.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return err
	}

	return nil
}

func (b *Benchmark) metricsReporter() {
	ticker := time.NewTicker(time.Duration(b.config.ReportInterval) * time.Second)
	defer ticker.Stop()

	lastSent := uint64(0)
	reportCount := 0

	fmt.Println("\n" + strings.Repeat("-", 85))
	fmt.Printf("%-10s | %-13s | %-15s | %-10s | %-12s\n",
		"Time", "Submitted TPS", "Total Submitted", "Errors", "Avg Latency")
	fmt.Println(strings.Repeat("-", 85))

	for {
		select {
		case <-b.stopMetricsChan:
			return
		case <-ticker.C:
			reportCount++
			sent := atomic.LoadUint64(&b.sentCount)
			errors := atomic.LoadUint64(&b.errorCount)
			totalLat := atomic.LoadInt64(&b.totalLatency)

			submittedTPS := sent - lastSent
			b.tpsHistory = append(b.tpsHistory, submittedTPS)

			avgLatency := time.Duration(0)
			if sent > 0 {
				avgLatency = time.Duration(totalLat / int64(sent))
			}

			elapsed := time.Since(b.startTime)
			fmt.Printf("%-10s | %-13d | %-15d | %-10d | %-12s\n",
				formatDuration(elapsed), submittedTPS, sent, errors,
				avgLatency.Round(time.Millisecond))

			lastSent = sent
		}
	}
}

func (b *Benchmark) printFinalReport(sent, errors uint64, totalLat int64) {
	elapsed := time.Since(b.startTime)

	avgSubmittedTPS := float64(sent) / elapsed.Seconds()
	avgLatency := time.Duration(0)
	if sent > 0 {
		avgLatency = time.Duration(totalLat / int64(sent))
	}

	// Calculate min/max/median TPS for submitted
	minSubmittedTPS, maxSubmittedTPS, medianSubmittedTPS := calculateTPSStats(b.tpsHistory)

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("BENCHMARK RESULTS")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Printf("\n📊 Overall Statistics:\n")
	fmt.Printf("  Duration:           %v\n", elapsed.Round(time.Second))
	fmt.Printf("  Total Submitted:    %d transactions\n", sent)
	fmt.Printf("  Total Errors:       %d transactions\n", errors)
	fmt.Printf("  RPC Accept Rate:    %.2f%%\n", float64(sent)/float64(sent+errors)*100)

	fmt.Printf("\n⚡ Submitted TPS Metrics:\n")
	fmt.Printf("  Average TPS:        %.2f\n", avgSubmittedTPS)
	fmt.Printf("  Peak TPS:           %d\n", maxSubmittedTPS)
	fmt.Printf("  Minimum TPS:        %d\n", minSubmittedTPS)
	fmt.Printf("  Median TPS:         %d\n", medianSubmittedTPS)

	fmt.Printf("\n⏱️  Latency:\n")
	fmt.Printf("  Average Latency:    %v\n", avgLatency.Round(time.Millisecond))

	fmt.Printf("\n👥 Per-Account Statistics:\n")
	for i, account := range b.accounts {
		sent := atomic.LoadUint64(&account.sent)
		errors := atomic.LoadUint64(&account.errors)
		successRate := 100.0
		if sent+errors > 0 {
			successRate = float64(sent) / float64(sent+errors) * 100
		}
		fmt.Printf("  Account %2d: %6d sent, %4d errors (%.1f%%)\n",
			i, sent, errors, successRate)
	}

	fmt.Println("\n" + strings.Repeat("=", 70))

	// Save results
	b.saveResults(elapsed, avgSubmittedTPS, sent, errors,
		minSubmittedTPS, maxSubmittedTPS, medianSubmittedTPS, avgLatency)
}

func (b *Benchmark) saveResults(duration time.Duration, avgSubmittedTPS float64, sent, errors uint64,
	minSubmittedTPS, maxSubmittedTPS, medianSubmittedTPS uint64, avgLatency time.Duration) {

	// Calculate rates
	rpcAcceptRate := 0.0
	if sent+errors > 0 {
		rpcAcceptRate = float64(sent) / float64(sent+errors) * 100
	}

	// Collect per-account statistics
	accountStats := make([]map[string]interface{}, 0, len(b.accounts))
	for i, account := range b.accounts {
		sent := atomic.LoadUint64(&account.sent)
		errors := atomic.LoadUint64(&account.errors)
		accountSuccessRate := 100.0
		if sent+errors > 0 {
			accountSuccessRate = float64(sent) / float64(sent+errors) * 100
		}
		accountStats = append(accountStats, map[string]interface{}{
			"account_id":   i,
			"address":      account.from.Hex(),
			"sent":         sent,
			"errors":       errors,
			"success_rate": accountSuccessRate,
		})
	}

	// Use struct to ensure consistent field order
	type BenchmarkResults struct {
		Timestamp           string                   `json:"timestamp"`
		Config              map[string]interface{}   `json:"config"`
		TotalSubmitted      uint64                   `json:"total_submitted"`
		TotalErrors         uint64                   `json:"total_errors"`
		RPCAcceptRate       float64                  `json:"rpc_accept_rate"`
		AvgSubmittedTPS     float64                  `json:"average_submitted_tps"`
		PeakSubmittedTPS    uint64                   `json:"peak_submitted_tps"`
		MinSubmittedTPS     uint64                   `json:"min_submitted_tps"`
		MedianSubmittedTPS  uint64                   `json:"median_submitted_tps"`
		AvgLatencyMs        int64                    `json:"average_latency_ms"`
		SubmittedTPSHistory []uint64                 `json:"submitted_tps_history"`
		AccountStats        []map[string]interface{} `json:"account_statistics"`
	}

	results := BenchmarkResults{
		Timestamp: time.Now().Format(time.RFC3339),
		Config: map[string]interface{}{
			"rpc_url":             b.config.RPCURL,
			"gas_limit":           b.config.GasLimit,
			"transfer_amount_wei": b.config.TransferAmount,
			"duration_seconds":    duration.Seconds(),
			"num_accounts":        len(b.accounts),
		},
		TotalSubmitted:      sent,
		TotalErrors:         errors,
		RPCAcceptRate:       rpcAcceptRate,
		AvgSubmittedTPS:     avgSubmittedTPS,
		PeakSubmittedTPS:    maxSubmittedTPS,
		MinSubmittedTPS:     minSubmittedTPS,
		MedianSubmittedTPS:  medianSubmittedTPS,
		AvgLatencyMs:        avgLatency.Milliseconds(),
		SubmittedTPSHistory: b.tpsHistory,
		AccountStats:        accountStats,
	}

	file, err := os.Create(b.config.OutputFile)
	if err != nil {
		fmt.Printf("Failed to save results: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.Encode(results)

	fmt.Printf("📝 Results saved to %s\n", b.config.OutputFile)
}

// Helper functions

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d % time.Minute) / time.Second
	return fmt.Sprintf("%02d:%02d", m, s)
}

func calculateTPSStats(tpsHistory []uint64) (min, max, median uint64) {
	if len(tpsHistory) == 0 {
		return 0, 0, 0
	}

	// Make a copy and sort
	sorted := make([]uint64, len(tpsHistory))
	copy(sorted, tpsHistory)

	// Bubble sort (ascending order)
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	min = sorted[0]
	max = sorted[len(sorted)-1]
	median = sorted[len(sorted)/2]

	return
}
