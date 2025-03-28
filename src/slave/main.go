package main

import (
	"fmt"
	"net"
	"strconv"
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

func connectToMaster(host string, port string) net.Conn {
	port_int, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	}

	for {
		port_str := strconv.Itoa(port_int)
		if !checkPortInUse(port_str) {
			port_int := port_int + 1
			port_now := strconv.Itoa(port_int)
			return connectToMaster(host, port_now)
		} else {
			conn, err := net.DialTimeout("tcp", "localhost:"+port, 2*time.Second)
			if err != nil {
				fmt.Printf("Timeout occured, retrying in 10 seconds")
				time.Sleep(10 * time.Second)
				continue
			}
			fmt.Println("Connected to Master Server")
			conn.Write([]byte("SLAVE"))
			return conn
		}
	}
}

var data_store map[string]string = make(map[string]string)

func main() {
	for {
		conn := connectToMaster("localhost", "12345")
		defer conn.Close()

		for {
			buf := make([]byte, 1024)
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			n, err := conn.Read(buf) // Read response
			if err != nil {
				fmt.Printf("Timeout occurred listening to the message, retrying..\n")
				continue
			}
			if n == 0 {
				break
			}
			result := strings.Split(string(buf[:n]), " ")

			command := result[0]
			key := result[1]
			value := result[2]
			fmt.Printf("Received Command from the Master: %s %s %s\n", command, key, value)

			if command == "READ" {
				fmt.Printf("Performing READ Operation for key %s", key)
				value, ok := data_store[key]
				if !ok {
					value = "NOT FOUND"
				}
				response := key + " " + value
				fmt.Printf("Sending Response: %s\n", response)

				conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				_, err := conn.Write([]byte(response))
				if err != nil {
					panic(err)
				}
			} else if command == "WRITE" {
				fmt.Printf("Performing WRITE Operation for key %s with value %s", key, value)
				data_store[key] = value
				response := key + " " + value + " ACK"
				fmt.Printf("Sending ACK: %s\n", response)
				conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				_, err := conn.Write([]byte(response))
				if err != nil {
					panic(err)
				}
			}
		}
	}
}
