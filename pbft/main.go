package main

import (
	"log"
	"os"
)

const nodeCount = 4

var clientAddr = "127.0.0.1:8888"

var nodeTable map[string]string

func main() {

	genRsaKeys()
	nodeTable = map[string]string{
		"N0": "127.0.0.1:8000",
		"N1": "127.0.0.1:8001",
		"N2": "127.0.0.1:8002",
		"N3": "127.0.0.1:8003",
	}
	if len(os.Args) != 2 {
		log.Panic("输入的参数有误！")
	}
	nodeID := os.Args[1]
	if nodeID == "client" {
		clientSendMessageAndListen()
	} else if addr, ok := nodeTable[nodeID]; ok {
		p := NewPBFT(nodeID, addr)
		go p.tcpListen()
	} else {
		log.Fatal("无此节点编号！")
	}
	select {}
}
