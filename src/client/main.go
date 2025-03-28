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

func main() {
	var port string
	if checkPortInUse("12345") {
		port = "12345"
	} else {
		port = "12346"
	}

	for {
		conn, err := net.DialTimeout("tcp", "localhost:"+port, 2*time.Second)
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		fmt.Println("Connected to Master Server!!")
		conn.Write([]byte("CLIENT")) // Send message

		for {
			var input string
			fmt.Printf("Enter the Operation you would like to perform (READ/WRITE/EXIT): ")
			fmt.Scanln(&input)
			input = strings.ToUpper(input)
			if input == "EXIT" {
				fmt.Println("Exiting Client...")
				os.Exit(0)
			} else if input == "READ" || input == "WRITE" {
				fmt.Printf("Enter the key you want to perform the operation on: ")
				var key string
				fmt.Scanf("%s", &key)
				if input == "READ" {

					fmt.Printf("Sending READ Operation for key %s\n\n", key)
					conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
					_, err := conn.Write([]byte(input + " " + key + " "))
					if err != nil {
						panic(err)
					}

					buf := make([]byte, 1024)
					conn.SetReadDeadline(time.Now().Add(10 * time.Second))
					n, err := conn.Read(buf) // Read response
					if err != nil {
						panic(err)
					}
					fmt.Printf("Server response: %s\n", string(buf[:n]))
					// res := strings.Split(string(buf[:n]), " ")
					// fmt.Printf("Value for the key %s = %s\n", key, res[0])

				} else if input == "WRITE" {

					var new_val string
					fmt.Scanln(&new_val)
					fmt.Printf("Sending WRITE Operation for key %s with value %s", key, new_val)
					conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

					_, err := conn.Write([]byte(input + " " + key + " " + new_val))
					if err != nil {
						panic(err)
					}

					buf := make([]byte, 1024)
					conn.SetReadDeadline(time.Now().Add(10 * time.Second))
					n, err := conn.Read(buf) // Read response
					if err != nil {
						panic(err)
					}
					fmt.Printf("Server response: %s\n", string(buf[:n]))

				}
			} else {
				fmt.Println("Invalid Operation! Please Try Again.")
			}
			time.Sleep(time.Second)
		}
	}
}
