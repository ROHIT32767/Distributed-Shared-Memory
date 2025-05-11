package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func checkPortInUse(port string) bool {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return true // Port is in use
	}
	ln.Close() // Close the listener if we successfully opened it
	return false
}

func connectToServer(primaryPort string, backupPort [3]string) (net.Conn, bool) {
	// Try to connect to the primary master first
	conn, err := net.DialTimeout("tcp", "localhost:"+primaryPort, 2*time.Second)
	if err == nil {
		fmt.Println("Connected to Master Server!!")
		conn.Write([]byte("CLIENT")) // Send message
		return conn, true
	}
	
	// If primary connection fails, try backup
	fmt.Println("Connection to Master Server failed, connecting to Backup Master...")
	for i := 0; i < 3; i++ {
		conn, err = net.DialTimeout("tcp", "localhost:"+backupPort[i], 2*time.Second)
		if err == nil {
			fmt.Println("Connected to Backup Master Server !!", i)
			conn.Write([]byte("CLIENT")) // Send message
			return conn, false
		} 
	}
	
	fmt.Println("Failed to connect to both Master and Backup servers.")
	return nil, false
}

func main() {
	primaryPort := "12345"
	backupPort := [3]string{"12346","12347","12348"}
	connectedToPrimary := true

	for {
		conn, isPrimary := connectToServer(primaryPort, backupPort)
		if conn == nil {
			fmt.Println("Retrying connection in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}
		
		connectedToPrimary = isPrimary
		defer conn.Close()

		for {
			var input string
			if connectedToPrimary {
				fmt.Printf("[PRIMARY] Enter the Operation you would like to perform (READ/WRITE/EXIT): ")
			} else {
				fmt.Printf("[BACKUP] Enter the Operation you would like to perform (READ/WRITE/EXIT): ")
			}
			fmt.Scanln(&input)
			input = strings.ToUpper(input)
			
			if input == "EXIT" {
				fmt.Println("Exiting Client...")
				os.Exit(0)
			} else if input == "READ" || input == "WRITE" {
				fmt.Printf("Enter the key you want to perform the operation on: ")
				var key string
				fmt.Scanln(&key)
				
				if input == "READ" {
					fmt.Printf("Sending READ Operation for key %s\n\n", key)
					conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
					_, err := conn.Write([]byte(input + " " + key + " "))
					if err != nil {
						fmt.Println("Error sending request to server:", err)
						break
					}

					buf := make([]byte, 1024)
					conn.SetReadDeadline(time.Now().Add(10 * time.Second))
					n, err := conn.Read(buf) // Read response
					if err != nil {
						fmt.Println("Error receiving response from server:", err)
						break
					}
					
					response := string(buf[:n])
					fmt.Printf("Server response: %s\n", response)
					if response == "NOT FOUND" {
						fmt.Printf("Key %s not found\n", key)
					} else {
						parts := strings.Split(response, " ")
						if len(parts) >= 2 {
							fmt.Printf("Value for key %s = %s\n", key, parts[1])
						}
					}
				} else if input == "WRITE" {
					fmt.Printf("Enter the value: ")
					var new_val string
					fmt.Scanln(&new_val)
					fmt.Printf("Sending WRITE Operation for key %s with value %s\n", key, new_val)
					
					conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
					_, err := conn.Write([]byte(input + " " + key + " " + new_val))
					if err != nil {
						fmt.Println("Error sending request to server:", err)
						break
					}

					buf := make([]byte, 1024)
					conn.SetReadDeadline(time.Now().Add(10 * time.Second))
					n, err := conn.Read(buf) // Read response
					if err != nil {
						fmt.Println("Error receiving response from server:", err)
						break
					}
					fmt.Printf("Server response: %s\n", string(buf[:n]))
				}
			} else {
				fmt.Println("Invalid Operation! Please Try Again.")
			}
			
			// Check if the connection is still alive
			conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
			_, err := conn.Write([]byte{0}) // Send a zero byte to test connection
			if err != nil {
				fmt.Println("Connection lost. Attempting to reconnect...")
				break
			}
			
			time.Sleep(time.Second)
		}
	}
}