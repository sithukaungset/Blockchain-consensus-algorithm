package main

import (
	"log"
	"os"
)

const nodeCount = 4

//Client listening address
var clientAddr = "127.0.0.1:8888"

// Node pool, mainly used to store listening address
var nodeTable map[string]string

func main() {

	// Generate public and private keys for four nodes
	genRsaKeys()
	nodeTable = map[string]string{
		"N0": "127.0.0.1:8000",
		"N1": "127.0.0.1:8001",
		"N2": "127.0.0.1:8002",
		"N3": "127.0.0.1:8003",
	}
	if len(os.Args) != 2 {
		log.Panic("Entered Incorrect parameter！")
	}
	nodeID := os.Args[1]
	if nodeID == "client" {
		clientSendMessageAndListen() //start the client program
	} else if addr, ok := nodeTable[nodeID]; ok {
		p := NewPBFT(nodeID, addr)
		go p.tcpListen() // start the node
	} else {
		log.Fatal("No such node number！")
	}
	select {}
}
