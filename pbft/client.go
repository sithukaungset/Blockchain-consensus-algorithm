package main

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"
)

func clientSendMessageAndListen() {
	// turn on the local monitoring of the client
	go clientTcpListen()
	fmt.Printf("The client starts listening, the address：%s\n", clientAddr)

	fmt.Println(" ---------------------------------------------------------------------------------")
	fmt.Println("|  You have entered the PBFT test Demo client, please start all nodes before sending a message！ :)  |")
	fmt.Println(" ---------------------------------------------------------------------------------")
	fmt.Println("Please enter the information to be stored in the node below：")
	// First get user input via command line
	stdReader := bufio.NewReader(os.Stdin)
	for {
		data, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}
		r := new(Request)
		r.Timestamp = time.Now().UnixNano()
		r.ClientAddr = clientAddr
		r.Message.ID = getRandom()
		// The message content is the user's input
		r.Message.Content = strings.TrimSpace(data)
		br, err := json.Marshal(r)
		if err != nil {
			log.Panic(err)
		}
		fmt.Println(string(br))
		content := jointMessage(cRequest, br)
		// The default N0 is the master node, and the request information is directly sent to N0.
		tcpDial(content, nodeTable["N0"])
	}
}

// Return a ten-digit random number as msgid
func getRandom() int {
	x := big.NewInt(10000000000)
	for {
		result, err := rand.Int(rand.Reader, x)
		if err != nil {
			log.Panic(err)
		}
		if result.Int64() > 1000000000 {
			return int(result.Int64())
		}
	}
}
