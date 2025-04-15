package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	MasterPort         = "12345"
	DefaultSlaves      = 3
	DefaultClients     = 5
	TestDuration       = 30 * time.Second
	WarmupDuration     = 5 * time.Second
	KeySpaceSize       = 1000
	WritePercentage    = 30
	MetricsSampleRate  = 1 * time.Second
	RequestTimeout     = 2 * time.Second
	ConnectionPoolSize = 10
	ResultsDir         = "results"
	PythonVisualizer   = "visualize.py"
)

type Metrics struct {
	Timestamp      time.Time `json:"timestamp" csv:"timestamp"`
	ClientCount    int       `json:"client_count" csv:"client_count"`
	SlaveCount     int       `json:"slave_count" csv:"slave_count"`
	TotalRequests  uint64    `json:"total_requests" csv:"total_requests"`
	SuccessCount   uint64    `json:"success_count" csv:"success_count"`
	ErrorCount     uint64    `json:"error_count" csv:"error_count"`
	ReadLatency    float64   `json:"read_latency_us" csv:"read_latency_us"`
	WriteLatency   float64   `json:"write_latency_us" csv:"write_latency_us"`
	Throughput     float64   `json:"throughput_rps" csv:"throughput_rps"`
	ReadCount      uint64    `json:"read_count" csv:"read_count"`
	WriteCount     uint64    `json:"write_count" csv:"write_count"`
}

var globalMetrics struct {
	totalRequests  uint64
	successCount   uint64
	errorCount     uint64
	readLatencySum uint64
	writeLatencySum uint64
	readCount      uint64
	writeCount     uint64
	currentClients int32
	currentSlaves  int32
	startTime      time.Time
}

type TestClient struct {
	ID        int
	ConnPool  []net.Conn
	StopChan  chan struct{}
	WaitGroup *sync.WaitGroup
}

type SlaveProcess struct {
	Port int
	Cmd  *exec.Cmd
}

func NewSlaveProcess(port int) *SlaveProcess {
	cmd := exec.Command("go", "run", "slave/main.go", strconv.Itoa(port))
	return &SlaveProcess{
		Port: port,
		Cmd:  cmd,
	}
}

func (s *SlaveProcess) Start() error {
	return s.Cmd.Start()
}

func (s *SlaveProcess) Stop() {
	s.Cmd.Process.Signal(os.Interrupt)
}

func NewTestClient(id int) *TestClient {
	connPool := make([]net.Conn, ConnectionPoolSize)
	for i := 0; i < ConnectionPoolSize; i++ {
		conn, err := net.Dial("tcp", "localhost:"+MasterPort)
		if err != nil {
			log.Fatalf("Client %d: Failed to connect: %v", id, err)
		}
		conn.Write([]byte("CLIENT"))
		connPool[i] = conn
	}
	return &TestClient{
		ID:        id,
		ConnPool:  connPool,
		StopChan:  make(chan struct{}),
		WaitGroup: &sync.WaitGroup{},
	}
}

func (c *TestClient) Run() {
	c.WaitGroup.Add(1)
	defer c.WaitGroup.Done()

	connIndex := 0
	for {
		select {
		case <-c.StopChan:
			for _, conn := range c.ConnPool {
				conn.Close()
			}
			return
		default:
			conn := c.ConnPool[connIndex]
			connIndex = (connIndex + 1) % ConnectionPoolSize

			key := fmt.Sprintf("key%d", rand.Intn(KeySpaceSize))
			isWrite := rand.Intn(100) < WritePercentage

			start := time.Now()
			var err error

			if isWrite {
				value := fmt.Sprintf("value%d", rand.Intn(10000))
				_, err = conn.Write([]byte(fmt.Sprintf("WRITE %s %s", key, value)))
				if err == nil {
					buf := make([]byte, 1024)
					n, err := conn.Read(buf)
					if err == nil && string(buf[:n]) != "WRITE_DONE" {
						err = fmt.Errorf("unexpected write response")
					}
				}

				latency := uint64(time.Since(start).Microseconds())
				atomic.AddUint64(&globalMetrics.writeLatencySum, latency)
				atomic.AddUint64(&globalMetrics.writeCount, 1)
			} else {
				_, err = conn.Write([]byte(fmt.Sprintf("READ %s", key)))
				if err == nil {
					buf := make([]byte, 1024)
					n, err := conn.Read(buf)
					if err == nil && strings.Contains(string(buf[:n]), "NOT FOUND") {
						err = nil
					}
				}

				latency := uint64(time.Since(start).Microseconds())
				atomic.AddUint64(&globalMetrics.readLatencySum, latency)
				atomic.AddUint64(&globalMetrics.readCount, 1)
			}

			atomic.AddUint64(&globalMetrics.totalRequests, 1)
			if err == nil {
				atomic.AddUint64(&globalMetrics.successCount, 1)
			} else {
				atomic.AddUint64(&globalMetrics.errorCount, 1)
			}

			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (c *TestClient) Stop() {
	close(c.StopChan)
	c.WaitGroup.Wait()
}

type TestController struct {
	Clients      []*TestClient
	SlaveProcs   []*SlaveProcess
	ResultsFile  *os.File
	CsvFile      *os.File
	CsvWriter    *csv.Writer
	StopChan     chan struct{}
	TestDuration time.Duration
}

func NewTestController(numSlaves, numClients int) *TestController {
	if err := os.MkdirAll(ResultsDir, 0755); err != nil {
		log.Fatal(err)
	}

	jsonFile, err := os.Create(filepath.Join(ResultsDir, "metrics.json"))
	if err != nil {
		log.Fatal(err)
	}

	csvFile, err := os.OpenFile(filepath.Join(ResultsDir, "metrics.csv"), 
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	fileInfo, err := csvFile.Stat()
	if err != nil {
		log.Fatal(err)
	}

	csvWriter := csv.NewWriter(csvFile)
	
	if fileInfo.Size() == 0 {
		header := []string{
			"timestamp",
			"client_count",
			"slave_count",
			"total_requests",
			"success_count",
			"error_count",
			"read_latency_us",
			"write_latency_us",
			"throughput_rps",
			"read_count",
			"write_count",
		}
		if err := csvWriter.Write(header); err != nil {
			log.Fatal(err)
		}
		csvWriter.Flush()
	}

	slaveProcs := make([]*SlaveProcess, numSlaves)
	for i := 0; i < numSlaves; i++ {
		slave := NewSlaveProcess(20000 + i)
		if err := slave.Start(); err != nil {
			log.Fatalf("Failed to start slave on port %d: %v", 20000+i, err)
		}
		slaveProcs[i] = slave
		time.Sleep(500 * time.Millisecond)
	}

	atomic.StoreInt32(&globalMetrics.currentSlaves, int32(numSlaves))
	atomic.StoreInt32(&globalMetrics.currentClients, int32(numClients))

	return &TestController{
		SlaveProcs:   slaveProcs,
		ResultsFile:  jsonFile,
		CsvFile:      csvFile,
		CsvWriter:    csvWriter,
		StopChan:     make(chan struct{}),
		TestDuration: TestDuration,
	}
}

func (tc *TestController) writeMetrics(metrics Metrics) {
	json.NewEncoder(tc.ResultsFile).Encode(metrics)

	record := []string{
		metrics.Timestamp.Format(time.RFC3339),
		strconv.Itoa(metrics.ClientCount),
		strconv.Itoa(metrics.SlaveCount),
		strconv.FormatUint(metrics.TotalRequests, 10),
		strconv.FormatUint(metrics.SuccessCount, 10),
		strconv.FormatUint(metrics.ErrorCount, 10),
		strconv.FormatFloat(metrics.ReadLatency, 'f', 2, 64),
		strconv.FormatFloat(metrics.WriteLatency, 'f', 2, 64),
		strconv.FormatFloat(metrics.Throughput, 'f', 2, 64),
		strconv.FormatUint(metrics.ReadCount, 10),
		strconv.FormatUint(metrics.WriteCount, 10),
	}

	if err := tc.CsvWriter.Write(record); err != nil {
		log.Printf("Error writing to CSV: %v", err)
	}
	tc.CsvWriter.Flush()
	if err := tc.CsvFile.Sync(); err != nil {
		log.Printf("Error syncing CSV file: %v", err)
	}
}

func (tc *TestController) CollectMetrics() {
	ticker := time.NewTicker(MetricsSampleRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			total := atomic.SwapUint64(&globalMetrics.totalRequests, 0)
			success := atomic.SwapUint64(&globalMetrics.successCount, 0)
			fail := atomic.SwapUint64(&globalMetrics.errorCount, 0)
			readLat := atomic.SwapUint64(&globalMetrics.readLatencySum, 0)
			writeLat := atomic.SwapUint64(&globalMetrics.writeLatencySum, 0)
			readCount := atomic.SwapUint64(&globalMetrics.readCount, 0)
			writeCount := atomic.SwapUint64(&globalMetrics.writeCount, 0)
			clientCount := int(atomic.LoadInt32(&globalMetrics.currentClients))
			slaveCount := int(atomic.LoadInt32(&globalMetrics.currentSlaves))

			var avgReadLat, avgWriteLat float64
			if readCount > 0 {
				avgReadLat = float64(readLat) / float64(readCount)
			}
			if writeCount > 0 {
				avgWriteLat = float64(writeLat) / float64(writeCount)
			}

			throughput := float64(total) / MetricsSampleRate.Seconds()

			metrics := Metrics{
				Timestamp:      time.Now(),
				ClientCount:    clientCount,
				SlaveCount:     slaveCount,
				TotalRequests:  total,
				SuccessCount:   success,
				ErrorCount:     fail,
				ReadLatency:    avgReadLat,
				WriteLatency:   avgWriteLat,
				Throughput:     throughput,
				ReadCount:      readCount,
				WriteCount:     writeCount,
			}

			fmt.Printf("\n=== Metrics (Clients: %d, Slaves: %d) ===\n", clientCount, slaveCount)
			fmt.Printf("Total Requests: %d\n", total)
			fmt.Printf("Successful: %d (%.2f%%)\n", success, float64(success)/float64(total)*100)
			fmt.Printf("Failed: %d (%.2f%%)\n", fail, float64(fail)/float64(total)*100)
			fmt.Printf("Throughput: %.2f req/s\n", throughput)
			fmt.Printf("Avg Read Latency: %.2f μs\n", avgReadLat)
			fmt.Printf("Avg Write Latency: %.2f μs\n", avgWriteLat)
			fmt.Printf("Read/Write Ratio: %d/%d\n", readCount, writeCount)

			tc.writeMetrics(metrics)

		case <-tc.StopChan:
			return
		}
	}
}

func (tc *TestController) StartClients(numClients int) {
	tc.Clients = make([]*TestClient, numClients)
	for i := 0; i < numClients; i++ {
		tc.Clients[i] = NewTestClient(i)
		go tc.Clients[i].Run()
	}
	atomic.StoreInt32(&globalMetrics.currentClients, int32(numClients))
}

func (tc *TestController) GenerateVisualizations() {
	if _, err := exec.LookPath("python3"); err != nil {
		log.Println("Python not found - skipping visualizations")
		return
	}

	script := `import pandas as pd
import matplotlib.pyplot as plt
import os
import sys

def main():
    if len(sys.argv) < 2:
        print("Usage: python visualize.py <results_dir>")
        return
    
    results_dir = sys.argv[1]
    csv_path = os.path.join(results_dir, "metrics.csv")
    
    try:
        df = pd.read_csv(csv_path)
    except Exception as e:
        print(f"Error reading CSV: {e}")
        return
    
    plt.figure(figsize=(15, 10))
    
    # Throughput vs Clients
    plt.subplot(2, 2, 1)
    plt.plot(df['client_count'], df['throughput_rps'], 'b-o')
    plt.title('Throughput vs Number of Clients')
    plt.xlabel('Number of Clients')
    plt.ylabel('Requests per Second')
    plt.grid(True)
    
    # Latency vs Clients
    plt.subplot(2, 2, 2)
    plt.plot(df['client_count'], df['read_latency_us'], 'g-o', label='Read Latency')
    plt.plot(df['client_count'], df['write_latency_us'], 'r-o', label='Write Latency')
    plt.title('Latency vs Number of Clients')
    plt.xlabel('Number of Clients')
    plt.ylabel('Latency (μs)')
    plt.legend()
    plt.grid(True)
    
    # Error Rate vs Clients
    plt.subplot(2, 2, 3)
    error_rate = (df['error_count'] / df['total_requests']) * 100
    plt.plot(df['client_count'], error_rate, 'm-o')
    plt.title('Error Rate vs Number of Clients')
    plt.xlabel('Number of Clients')
    plt.ylabel('Error Rate (%)')
    plt.grid(True)
    
    # Operation Distribution
    plt.subplot(2, 2, 4)
    plt.bar(df['client_count'], df['read_count'], width=3, label='Reads')
    plt.bar(df['client_count'], df['write_count'], width=3, bottom=df['read_count'], label='Writes')
    plt.title('Operation Distribution')
    plt.xlabel('Number of Clients')
    plt.ylabel('Operation Count')
    plt.legend()
    plt.grid(True)
    
    plt.tight_layout()
    plot_path = os.path.join(results_dir, 'performance_metrics.png')
    plt.savefig(plot_path)
    print(f"Saved performance metrics plot to {plot_path}")

if __name__ == "__main__":
    main()`

	vizPath := filepath.Join(ResultsDir, PythonVisualizer)
	if _, err := os.Stat(vizPath); os.IsNotExist(err) {
		if err := os.WriteFile(vizPath, []byte(script), 0755); err != nil {
			log.Printf("Failed to create visualizer: %v", err)
			return
		}
	}

	cmd := exec.Command("python3", vizPath, ResultsDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("Visualization failed: %v", err)
	}
}

func (tc *TestController) Run(numClients int) {
	defer func() {
		tc.ResultsFile.Close()
		tc.CsvFile.Close()
		for _, slave := range tc.SlaveProcs {
			slave.Stop()
		}
	}()

	tc.StartClients(numClients)
	globalMetrics.startTime = time.Now()

	go tc.CollectMetrics()

	log.Println("Starting warm-up period...")
	time.Sleep(WarmupDuration)

	log.Printf("Running test with %d clients and %d slaves...", numClients, len(tc.SlaveProcs))
	time.Sleep(tc.TestDuration)

	close(tc.StopChan)
	for _, client := range tc.Clients {
		client.Stop()
	}

	log.Println("Test completed. Generating visualizations...")
	tc.GenerateVisualizations()
}

func main() {
	var numSlaves, numClients int
	flag.IntVar(&numSlaves, "slaves", DefaultSlaves, "Number of slave instances")
	flag.IntVar(&numClients, "clients", DefaultClients, "Number of client instances")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fmt.Println("\nReceived interrupt, stopping tests...")
		os.Exit(0)
	}()

	fmt.Printf("Starting test with %d slaves and %d clients\n", numSlaves, numClients)
	controller := NewTestController(numSlaves, numClients)
	controller.Run(numClients)
	fmt.Println("Test completed")
}