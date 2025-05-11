package main

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// Global map to store key-value pairs
var data_store map[string]string = make(map[string]string)

// Try to connect to either master or backup server
func connectToServer() net.Conn {
	primaryMaster := "localhost:12345"
	backupMaster := [3]string{"localhost:12346","localhost:12347","localhost:12348"}
	
	// Try primary master first
	fmt.Printf("Attempting to connect to primary master at %s\n", primaryMaster)
	conn, err := net.DialTimeout("tcp", primaryMaster, 5*time.Second)
	if err == nil {
		fmt.Println("Connected to Primary Master Server")
		_, err = conn.Write([]byte("SLAVE"))
		if err == nil {
			return conn
		}
		conn.Close()
	}
	
	// If primary master fails, try backup master
	for i:=0;i<3;i++{
		fmt.Printf("Primary master connection failed, trying backup master at %s\n", backupMaster[i])
		conn, err = net.DialTimeout("tcp", backupMaster[i], 5*time.Second)
		if err == nil {
			fmt.Println("Connected to Backup Master Server",i)
			_, err = conn.Write([]byte("SLAVE"))
			if err == nil {
				return conn
			}
			conn.Close()
		}
	}
	
	// Both connections failed
	return nil
}

// Handles a connected session with a master server
func handleMasterSession(conn net.Conn) bool {
	// Set initial read deadline
	err := conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	if err != nil {
		fmt.Printf("Error setting initial read deadline: %v\n", err)
		return false
	}
	
	for {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() {
				fmt.Println("Read timeout - master may still be connected")
				// Ping the master to see if it's still alive
				err = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err != nil {
					fmt.Printf("Error setting write deadline: %v\n", err)
					return false
				}
				
				_, err = conn.Write([]byte("PING"))
				if err != nil {
					fmt.Printf("Failed to ping master: %v\n", err)
					return false
				}
				
				// Reset read deadline
				err = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
				if err != nil {
					fmt.Printf("Error resetting read deadline: %v\n", err)
					return false
				}
				continue
			} else {
				fmt.Printf("Connection error: %v\n", err)
				return false
			}
		}
		
		if n == 0 {
			fmt.Println("Received empty message, connection may be closed")
			return false
		}
		
		// Reset read deadline after successful read
		err = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		if err != nil {
			fmt.Printf("Error resetting read deadline: %v\n", err)
			return false
		}
		
		command := string(buf[:n])
		fmt.Printf("Received from master: %s\n", command)
		
		// Handle PING response
		if command == "PONG" {
			fmt.Println("Received PONG from master - connection still active")
			continue
		}
		
		// Parse the command
		parts := strings.Split(command, " ")
		if len(parts) < 3 {
			fmt.Printf("Invalid command format: %s\n", command)
			continue
		}
		
		cmd, key, value := parts[0], parts[1], parts[2]
		fmt.Printf("Processing command: %s %s %s\n", cmd, key, value)
		
		var response string
		
		switch cmd {
		case "READ":
			storedValue, exists := data_store[key]
			if !exists {
				storedValue = "NOT FOUND"
			}
			response = key + " " + storedValue
			
		case "WRITE":
			data_store[key] = value
			response = key + " " + value + " ACK"
			
		default:
			fmt.Printf("Unknown command: %s\n", cmd)
			continue
		}
		
		// Send response
		err = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			fmt.Printf("Error setting write deadline: %v\n", err)
			return false
		}
		
		_, err = conn.Write([]byte(response))
		if err != nil {
			fmt.Printf("Error sending response: %v\n", err)
			return false
		}
		
		fmt.Printf("Sent response: %s\n", response)
	}
}

func main() {
	fmt.Println("Starting Slave Server...")
	
	// Exponential backoff parameters
	baseDelay := 5 * time.Second
	maxDelay := 2 * time.Minute
	currentDelay := baseDelay
	
	for {
		// Try to connect to either master
		conn := connectToServer()
		
		if conn == nil {
			fmt.Printf("Failed to connect to any master server. Retrying in %v...\n", currentDelay)
			time.Sleep(currentDelay)
			
			// Exponential backoff
			currentDelay *= 2
			if currentDelay > maxDelay {
				currentDelay = maxDelay
			}
			continue
		}
		
		// Reset delay on successful connection
		currentDelay = baseDelay
		
		// Handle the session
		fmt.Println("Starting session with master server")
		sessionOk := handleMasterSession(conn)
		
		// Close the connection
		conn.Close()
		
		if sessionOk {
			fmt.Println("Session ended normally")
		} else {
			fmt.Println("Session ended with errors")
		}
		
		// Brief pause before reconnection attempt
		time.Sleep(1 * time.Second)
	}
}