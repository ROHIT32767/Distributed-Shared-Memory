package main

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
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
	slaveMutex   sync.Mutex
	keySlavesMux sync.Mutex
}

func NewKeyValueStore() *KeyValueStore {
	return &KeyValueStore{
		slaves:      make([]*Slave, 0),
		keyToSlaves: make(map[string][]*Slave),
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
	fmt.Printf(Green + "Received valid response from a slave: %s\n", response + Reset)
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
	slaveCount := int(math.Ceil(float64(len(kvs.slaves)+1) * 0.5))
	selectedSlaves := make([]*Slave, slaveCount)
	var indices map[int]bool = make(map[int]bool)

	for i := 0; i < slaveCount; i++ {
		index := rand.Intn(len(kvs.slaves))
		_, ok := indices[index]
		if !ok {
			indices[index] = true
			selectedSlaves[i] = kvs.slaves[index]
		}
	}

	kvs.keySlavesMux.Lock()
	kvs.keyToSlaves[key] = selectedSlaves
	kvs.keySlavesMux.Unlock()

	acks := kvs.receiveAckFromSlaves(strings.Join(command, " "), 3*time.Second)

	notReceivedSlaves := make([]*Slave, 0)
	for slave, acked := range acks {
		if !acked {
			notReceivedSlaves = append(notReceivedSlaves, slave)
		}
	}

	if len(notReceivedSlaves) > 0 {
		fmt.Printf(Red + "No acknowledgment received from some slaves. Removing them.\n" + Reset)
		for _, slave := range notReceivedSlaves {
			kvs.removeSlave(slave)
		}
		return true
	}

	fmt.Printf(Magenta + "Write operation successful.\n" + Reset)
	return true
}

func (kvs *KeyValueStore) handleRead(command []string) string {
	key := command[1]
	var responses []string
	var associatedSlaves []*Slave

	kvs.keySlavesMux.Lock()
	savedSlaves, exists := kvs.keyToSlaves[key]
	kvs.keySlavesMux.Unlock()

	if !exists {
		fmt.Printf(Red + "Key does not exist in Map somehow: %s\n\n", key + Reset)
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
		// color.Yellow("Received from Client: %s", data)
		fmt.Printf(Yellow + "Received from Client: %s\n\n", data + Reset)

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
			fmt.Println(Red + "Read error: %v%s", err, Reset)
			return
		}
		var data string = string(buf[:n])
		fmt.Println(Green + "Received:\n", data + Reset)
		if n == 0 {
			break
		}
	}
}

func handleConnection(conn net.Conn, kvs *KeyValueStore) {
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println(Red + "Read error:%v%s", err, Reset)
		return
	}
	var data string = string(buf[:n])
	fmt.Println(Yellow + "Received:", data + Reset)

	if data == "CLIENT" {
		fmt.Println(Green + "Client Connected" + Reset)
		go handleClient(conn, kvs)
	} else if data == "SLAVE" {
		fmt.Println(Green + "Slave Connected" + Reset)
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
		fmt.Printf("connection of slave: %s %s", ip, port)
		slave := Slave{conn: conn}
		kvs.slaves = append(kvs.slaves, &slave)
		go handleSlave(conn)
	} else if data == "BACKUP" {
		fmt.Println(Green + "Backup server connected" + Reset)
	}
}

func main() {
	fmt.Println("Distributed Key-Value Store Server")
	var port string = "12345"
	var port_int int = 12345
	if checkPortInUse(port) {
		port_int++
	}
	port = strconv.Itoa(port_int)
	fmt.Printf("Master Server Started\n\n")

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	fmt.Printf("Server is listening on port %s...", port)

	var kvs *KeyValueStore = NewKeyValueStore()
	for {
		conn, err := ln.Accept() // Accept a connection
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go handleConnection(conn, kvs) // Handle each client concurrently
	}
}