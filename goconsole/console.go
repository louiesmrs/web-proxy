package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("**************************** PROXY MANAGEMENT SYSTEM ************************************")
	fmt.Println("-> To Block a site, type 'block [url]'")
	fmt.Println("-> To Unblock a site, type 'unblock [url]'")
	fmt.Println("-> To Unblock all sites, type 'unblock all'")
	fmt.Println("-> For help type 'help'")
	fmt.Println("-> To exit type 'exit'")
	for scanner.Scan() {
		cmd := strings.ToLower(scanner.Text())
		if strings.HasPrefix(cmd, "block ") || strings.HasPrefix(cmd, "unblock ") || strings.HasPrefix(cmd, "unblock all") {
			res, err := http.Post("http://localhost:8081", "text/plain", bytes.NewBuffer([]byte(cmd)))
			if err != nil {
				log.Printf("Error %v with command: %s \n", err, cmd)
			} else {
				defer res.Body.Close()
				log.Printf("Command: %s was successful\n", cmd)
			}
		} else if strings.HasPrefix(cmd, "help") {
			fmt.Println("-> To Block a site, type 'block [url]")
			fmt.Println("-> To Unblock a site, type 'unblock [url]")
			fmt.Println("-> To Unblock all sites, type 'unblock all'")
			fmt.Println("-> For help type 'help'")
			fmt.Println("-> To exit type 'exit'")
		} else if strings.HasPrefix(cmd, "exit") || strings.HasPrefix(cmd, "quit") {
			os.Exit(0)
		} else {
			log.Printf("Invalid command: %s\n", cmd)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
