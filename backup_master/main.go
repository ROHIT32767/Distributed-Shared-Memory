package main

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"
	"flag"
)

var Reset = "\033[0m"
var Red = "\033[31m"
var Green = "\033[32m"
var Yellow = "\033[33m"
var Blue = "\033[34m"
var Magenta = "\033[35m"
var Cyan = "\033[36m"
var Gray = "\033[37m"
var White = "\033[97m"

type Slave struct {
	conn net.Conn
}

func checkPortInUse(port string) bool {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return true
	}
	ln.Close()
	return false
}

type KeyValueStore struct {
	slaves       []*Slave
	keyToSlaves  map[string][]*Slave
	keyValueData map[string]string // Local store for key-value pairs from log
	slaveMutex   sync.Mutex
	keySlavesMux sync.Mutex
	dataMutex    sync.Mutex
	logFile      *os.File
}

func NewKeyValueStore() *KeyValueStore {
	// Open log file for writing
	logFile, err := os.OpenFile("../master/kv_store.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf(Red+"Error opening log file: %v\n"+Reset, err)
	}


	return &KeyValueStore{
		slaves:       make([]*Slave, 0),
		keyToSlaves:  make(map[string][]*Slave),
		keyValueData: make(map[string]string),
		logFile:     logFile,
	}
}


func (kvs *KeyValueStore) closeResources() {
	if kvs.logFile != nil {
		kvs.logFile.Close()
	}
}


func (kvs *KeyValueStore) logOperation(operation, key, value string) {
	if kvs.logFile != nil {
		logEntry := fmt.Sprintf("%s %s %s\n", operation, key, value)
		_, err := kvs.logFile.WriteString(logEntry)
		if err != nil {
			fmt.Printf(Red+"Error writing to log file: %v\n"+Reset, err)
		}
		kvs.logFile.Sync() // Ensure data is written to disk
	}
}

func (kvs *KeyValueStore) sendRequestToSlave(slave *Slave, requestData string, timeout time.Duration) (string, error) {
	slave.conn.SetReadDeadline(time.Now().Add(timeout))

	_, err := slave.conn.Write([]byte(requestData))
	if err != nil {
		return "", err
	}

	buffer := make([]byte, 1024)
	n, err := slave.conn.Read(buffer)
	if err != nil {
		return "", err
	}

	response := string(buffer[:n])
	fmt.Printf(Green+"Received valid response from a slave: %s\n", response+Reset)
	return response, nil
}

func (kvs *KeyValueStore) sendRequestsToAllSlaves(requestData string, timeout time.Duration) map[*Slave]string {
	responses := make(map[*Slave]string)
	var wg sync.WaitGroup

	for _, slave := range kvs.slaves {
		wg.Add(1)
		go func(s *Slave) {
			defer wg.Done()
			response, err := kvs.sendRequestToSlave(s, requestData, timeout)
			if err == nil {
				kvs.slaveMutex.Lock()
				responses[s] = response
				kvs.slaveMutex.Unlock()
			}
		}(slave)
	}

	wg.Wait()
	return responses
}

func (kvs *KeyValueStore) receiveAckFromSlaves(requestData string, timeout time.Duration) map[*Slave]bool {
	acks := make(map[*Slave]bool)
	var wg sync.WaitGroup

	for _, slave := range kvs.slaves {
		wg.Add(1)
		go func(s *Slave) {
			defer wg.Done()
			_, err := kvs.sendRequestToSlave(s, requestData, timeout)
			if err == nil {
				kvs.slaveMutex.Lock()
				acks[s] = true
				kvs.slaveMutex.Unlock()
			}
		}(slave)
	}

	wg.Wait()
	return acks
}

func (kvs *KeyValueStore) handleWrite(command []string) bool {
	key := command[1]
	value := command[2]
	slaveCount := int(math.Ceil(float64(len(kvs.slaves)+1) * 0.5))
	selectedSlaves := make([]*Slave, 0, slaveCount)
	var indices map[int]bool = make(map[int]bool)

	// Only select slaves if we have any
	if len(kvs.slaves) > 0 {
		for i := 0; i < slaveCount; i++ {
			index := rand.Intn(len(kvs.slaves))
			_, ok := indices[index]
			if !ok {
				indices[index] = true
				selectedSlaves = append(selectedSlaves, kvs.slaves[index])
			}
		}
	}

	kvs.keySlavesMux.Lock()
	kvs.keyToSlaves[key] = selectedSlaves
	kvs.keySlavesMux.Unlock()

	// Store the key-value pair in our local backup map
	kvs.dataMutex.Lock()
	kvs.keyValueData[key] = value
	kvs.dataMutex.Unlock()

	// Only check acks if we have slaves
	if len(selectedSlaves) > 0 {
		acks := kvs.receiveAckFromSlaves(strings.Join(command, " "), 3*time.Second)

		notReceivedSlaves := make([]*Slave, 0)
		for slave, acked := range acks {
			if !acked {
				notReceivedSlaves = append(notReceivedSlaves, slave)
			}
		}

		if len(notReceivedSlaves) > 0 {
			fmt.Printf(Red+"No acknowledgment received from some slaves. Removing them.\n"+Reset)
			for _, slave := range notReceivedSlaves {
				kvs.removeSlave(slave)
			}
		}
	}

	kvs.logOperation("WRITE", key, value)

	fmt.Printf(Magenta+"Write operation successful.\n"+Reset)
	return true
}

func (kvs *KeyValueStore) handleRead(command []string) string {
	key := command[1]
	
	// First check if we have the key in our local store
	kvs.dataMutex.Lock()
	value, exists := kvs.keyValueData[key]
	kvs.dataMutex.Unlock()
	
	if exists {
		return key + " " + value
	}
	
	var responses []string
	var associatedSlaves []*Slave

	kvs.keySlavesMux.Lock()
	savedSlaves, exists := kvs.keyToSlaves[key]
	kvs.keySlavesMux.Unlock()

	if !exists {
		fmt.Printf(Red+"Key does not exist in Map somehow: %s\n\n", key+Reset)
		slaveResponses := kvs.sendRequestsToAllSlaves(strings.Join(command, " "), 3*time.Second)
		for slave, response := range slaveResponses {
			if response != key+" NOT FOUND" {
				responses = append(responses, response)
				associatedSlaves = append(associatedSlaves, slave)
			}
		}

		if len(responses) == 0 {
			return "NOT FOUND"
		}

		// Find majority response
		majorityResponse := findMajorityResponse(responses)
		kvs.keySlavesMux.Lock()
		kvs.keyToSlaves[key] = associatedSlaves
		kvs.keySlavesMux.Unlock()

		return majorityResponse
	}

	// Use saved slaves for this key
	for _, slave := range savedSlaves {
		response, err := kvs.sendRequestToSlave(slave, strings.Join(command, " "), 3*time.Second)
		if err == nil {
			return response
		}
	}

	return "NOT FOUND"
}

func (kvs *KeyValueStore) removeSlave(slave *Slave) {
	kvs.slaveMutex.Lock()
	defer kvs.slaveMutex.Unlock()

	for i, s := range kvs.slaves {
		if s == slave {
			kvs.slaves = append(kvs.slaves[:i], kvs.slaves[i+1:]...)
			break
		}
	}

	// Remove slave from key to slaves mapping
	for key, slaves := range kvs.keyToSlaves {
		for j, s := range slaves {
			if s == slave {
				kvs.keyToSlaves[key] = append(slaves[:j], slaves[j+1:]...)
				break
			}
		}
	}
}

// Load key-value data from log file
func (kvs *KeyValueStore) loadDataFromLog() {
	file, err := os.Open("../master/kv_store.log")
	if err != nil {
		fmt.Printf(Yellow+"Could not open log file: %v\n"+Reset, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")
		if len(parts) >= 3 {
			key := parts[1]
			value := parts[2]
			kvs.dataMutex.Lock()
			kvs.keyValueData[key] = value
			kvs.dataMutex.Unlock()
			fmt.Printf(Green+"Loaded from log: %s = %s\n"+Reset, key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf(Red+"Error reading log file: %v\n"+Reset, err)
	}
}

// Watch log file for changes
func (kvs *KeyValueStore) watchLogFile() {
	lastSize := int64(0)
	
	for {
		time.Sleep(1 * time.Second)
		
		file, err := os.Open("../master/kv_store.log")
		if err != nil {
			continue
		}
		
		info, err := file.Stat()
		if err != nil {
			file.Close()
			continue
		}

		
		if info.Size() > lastSize {
			// File has grown, read new entries
			_, err = file.Seek(lastSize, 0)
			if err != nil {
				file.Close()
				continue
			}
			
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.Split(line, " ")
				fmt.Printf(parts[0], parts[1], parts[2], " \n")
				if len(parts) >= 3 {
					key := parts[1]
					value := parts[2]
					kvs.dataMutex.Lock()
					kvs.keyValueData[key] = value
					kvs.dataMutex.Unlock()
					fmt.Printf(Green+"Updated from log: %s = %s\n"+Reset, key, value)
				}
			}
			
			lastSize = info.Size()
		}
		
		file.Close()
	}
}

func findMajorityResponse(responses []string) string {
	responseCount := make(map[string]int)
	for _, response := range responses {
		responseCount[response]++
	}

	var majorityResponse string
	maxCount := 0
	for response, count := range responseCount {
		if count > maxCount {
			majorityResponse = response
			maxCount = count
		}
	}

	return majorityResponse
}

func handleClient(conn net.Conn, kvs *KeyValueStore) {
	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			break
		}

		data := string(buffer[:n])
		fmt.Printf(Yellow+"Received from Client: %s\n\n", data+Reset)

		command := strings.Split(data, " ")
		if len(command) < 2 {
			continue
		}

		var response string
		switch command[0] {
		case "WRITE":
			if kvs.handleWrite(command) {
				response = "WRITE_DONE"
			}
		case "READ":
			response = kvs.handleRead(command)
		default:
			response = "INVALID_COMMAND"
		}
		conn.Write([]byte(response))
	}
}

func handleSlave(conn net.Conn) {
	for {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf(Red+"Read error: %v%s\n", err, Reset)
			return
		}
		var data string = string(buf[:n])
		fmt.Println(Green+"Received:\n", data+Reset)
		if n == 0 {
			break
		}
	}
}

func handleConnection(conn net.Conn, kvs *KeyValueStore) {
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println(Red+"Read error: %v%s\n", err, Reset)
		return
	}
	var data string = string(buf[:n])
	fmt.Println(Yellow+"Received:", data+Reset)

	if data == "CLIENT" {
		fmt.Println(Green+"Client Connected"+Reset)
		go handleClient(conn, kvs)
	} else if data == "SLAVE" {
		fmt.Println(Green+"Slave Connected"+Reset)
		remoteAddr := conn.RemoteAddr()
		var ip string
		var port int

		switch addr := remoteAddr.(type) {
		case *net.TCPAddr:
			ip = addr.IP.String()
			port = addr.Port
		case *net.UDPAddr:
			ip = addr.IP.String()
			port = addr.Port
		default:
			fmt.Println("Error")
		}
		fmt.Printf("Connection of slave: %s %d\n", ip, port)
		slave := Slave{conn: conn}
		kvs.slaves = append(kvs.slaves, &slave)
		go handleSlave(conn)
	} else if data == "MASTER" {
		fmt.Println(Green+"Master server connected for sync"+Reset)
		// Handle sync with master if needed
	}
}



func main() {
	fmt.Println("Distributed Key-Value Store Backup Server")
	portPtr := flag.String("port", "12346", "Port number for the server to listen on")
	flag.Parse()

	port := *portPtr
	fmt.Printf("Backup Master Server Started\n\n")

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	kvs := NewKeyValueStore()
	defer kvs.closeResources()

	fmt.Printf("Backup Server is listening on port %s...\n", port)

	// Load existing data from log
	kvs.loadDataFromLog()
	
	// Start watching log file for changes
	go kvs.watchLogFile()
	
	for {
		conn, err := ln.Accept() // Accept a connection
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go handleConnection(conn, kvs) // Handle each client concurrently
	}
}