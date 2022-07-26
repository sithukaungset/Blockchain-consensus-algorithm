package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"sync"
)

//The local message pool (simulation the persistance layer)
//will only be  stored in this pool after the successful submission
// is confirmed
var localMessagePool = []Message{}

type node struct {
	//Node ID
	nodeID string
	//Node listening address
	addr string
	//RSA private key
	rsaPrivKey []byte
	//RSA public key
	rsaPubKey []byte
}

type pbft struct {
	//Node information
	node node
	//Self-incrementing sequence number for each request
	sequenceID int
	//Lock
	lock sync.Mutex
	//Temporary message pool, message digest corresponds to message body
	messagePool map[string]Request
	//Store the number of prepares received (at least 2f need to be received and confirmed), corresponding to the summary.
	prePareConfirmCount map[string]map[string]bool
	//Store the number of received commits (at least 2f + 1 need to be received and confirmed), corresponding to the summary.
	commitConfirmCount map[string]map[string]bool
	//Whether the message has been commit broadcast
	isCommitBordcast map[string]bool
	//Whether the message has been replied to the client
	isReply map[string]bool
}

func NewPBFT(nodeID, addr string) *pbft {
	p := new(pbft)
	p.node.nodeID = nodeID
	p.node.addr = addr
	p.node.rsaPrivKey = p.getPivKey(nodeID) //Read from the generated private key file
	p.node.rsaPubKey = p.getPubKey(nodeID)  //Read from the generated private key file
	p.sequenceID = 0
	p.messagePool = make(map[string]Request)
	p.prePareConfirmCount = make(map[string]map[string]bool)
	p.commitConfirmCount = make(map[string]map[string]bool)
	p.isCommitBordcast = make(map[string]bool)
	p.isReply = make(map[string]bool)
	return p
}

func (p *pbft) handleRequest(data []byte) {
	//Cut the message and call different functions according to the message command
	cmd, content := splitMessage(data)
	switch command(cmd) {
	case cRequest:
		p.handleClientRequest(content)
	case cPrePrepare:
		p.handlePrePrepare(content)
	case cPrepare:
		p.handlePrepare(content)
	case cCommit:
		p.handleCommit(content)
	}
}

//Handle requests from clients
func (p *pbft) handleClientRequest(content []byte) {
	fmt.Println("主节点已接收到客户端发来的request ...")
	//Use json to parse out the Request structure
	r := new(Request)
	err := json.Unmarshal(content, r)
	if err != nil {
		log.Panic(err)
	}
	//Add information serial number
	p.sequenceIDAdd()
	//Get message digest
	digest := getDigest(*r)
	fmt.Println("The request has been stored in the temporary message pool")
	//Stored in a temporary message pool
	p.messagePool[digest] = *r
	//The master node signs the message digest
	digestByte, _ := hex.DecodeString(digest)
	signInfo := p.RsaSignWithSha256(digestByte, p.node.rsaPrivKey)
	//Spliced into PrePrepare, ready to send to the follower node.
	pp := PrePrepare{*r, digest, p.sequenceID, signInfo}
	b, err := json.Marshal(pp)
	if err != nil {
		log.Panic(err)
	}
	fmt.Println(" PrePrepare broadcast to other nodes...")
	//Make a PrePrepare broadcast
	p.broadcast(cPrePrepare, b)
	fmt.Println("PrePrepare broadcast complete")
}

//Handling prepared messages
func (p *pbft) handlePrePrepare(content []byte) {
	fmt.Println(" This node has received the PrePrepare sent by the master node...")
	//	Use json to parse out the PrePrepare structure
	pp := new(PrePrepare)
	err := json.Unmarshal(content, pp)
	if err != nil {
		log.Panic(err)
	}
	//Get the public key of the master node for digital signature verification
	primaryNodePubKey := p.getPubKey("N0")
	digestByte, _ := hex.DecodeString(pp.Digest)
	if digest := getDigest(pp.RequestMessage); digest != pp.Digest {
		fmt.Println("The information summary is not correct, refuse to prepare broadcast")
	} else if p.sequenceID+1 != pp.SequenceID {
		fmt.Println("The sequence number of the message does not match, and the prepare broadcast is refused")
	} else if !p.RsaVerySignWithSha256(digestByte, pp.Sign, primaryNodePubKey) {
		fmt.Println("Master node signature verification failed, refuse to prepare broadcast")
	} else {
		//ordinal assignment
		p.sequenceID = pp.SequenceID
		//Store information in a temporary message pool
		fmt.Println("已将消息存入临时节点池")
		p.messagePool[pp.Digest] = pp.RequestMessage
		//The node signs it with the private key
		sign := p.RsaSignWithSha256(digestByte, p.node.rsaPrivKey)
		//Splicing into Prepare
		pre := Prepare{pp.Digest, pp.SequenceID, p.node.nodeID, sign}
		bPre, err := json.Marshal(pre)
		if err != nil {
			log.Panic(err)
		}
		//broadcast in preparation
		fmt.Println("Prepare broadcast in progress ...")
		p.broadcast(cPrepare, bPre)
		fmt.Println("Prepare broadcast complete")
	}
}

//Process prepare message
func (p *pbft) handlePrepare(content []byte) {
	//Use json to parse the Prepare structure
	pre := new(Prepare)
	err := json.Unmarshal(content, pre)
	if err != nil {
		log.Panic(err)
	}
	fmt.Printf("This node has received the messgae from node %s Prepare ... \n", pre.NodeID)
	//Get the public key of the message source node for digital signature verification
	MessageNodePubKey := p.getPubKey(pre.NodeID)
	digestByte, _ := hex.DecodeString(pre.Digest)
	if _, ok := p.messagePool[pre.Digest]; !ok {
		fmt.Println("The current temporary message pool does not have this summary and refuses to perform commit broadcast")
	} else if p.sequenceID != pre.SequenceID {
		fmt.Println("The message sequence numbers do not match, and the commit broadcast is refused")
	} else if !p.RsaVerySignWithSha256(digestByte, pre.Sign, MessageNodePubKey) {
		fmt.Println("Node signature verification failed! refuse to execute commit broadcast")
	} else {
		p.setPrePareConfirmMap(pre.Digest, pre.NodeID, true)
		count := 0
		for range p.prePareConfirmCount[pre.Digest] {
			count++
		}
		//Because the master node will not send Prepare, it does not include itself
		specifiedCount := 0
		if p.node.nodeID == "N0" {
			specifiedCount = nodeCount / 3 * 2
		} else {
			specifiedCount = (nodeCount / 3 * 2) - 1
		}
		//If the node has received at least 2f prepare messages (including itself), and has not performed commit broadcast, then perform commit broadcast.
		p.lock.Lock()
		//Get the public key of the messgae source node for digital signature verification
		if count >= specifiedCount && !p.isCommitBordcast[pre.Digest] {
			fmt.Println(" This node has received Prepare information from at least 2f nodes (including the local node) ...")
			//The node signs it with the private key
			sign := p.RsaSignWithSha256(digestByte, p.node.rsaPrivKey)
			c := Commit{pre.Digest, pre.SequenceID, p.node.nodeID, sign}
			bc, err := json.Marshal(c)
			if err != nil {
				log.Panic(err)
			}
			//Broadcast submissions
			fmt.Println("Commit broadcast in progress")
			p.broadcast(cCommit, bc)
			p.isCommitBordcast[pre.Digest] = true
			fmt.Println("commit broadcast completed")
		}
		p.lock.Unlock()
	}
}

//Process the commit confirmation message
func (p *pbft) handleCommit(content []byte) {
	//Use json to parse out the Commit structure
	c := new(Commit)
	err := json.Unmarshal(content, c)
	if err != nil {
		log.Panic(err)
	}
	fmt.Printf("This node has received the Commit send by the %s node ... \n", c.NodeID)
	//Get the public key of the message source node for digital signature verification
	MessageNodePubKey := p.getPubKey(c.NodeID)
	digestByte, _ := hex.DecodeString(c.Digest)
	if _, ok := p.prePareConfirmCount[c.Digest]; !ok {
		fmt.Println("The current prepare pool does not have this summary and refuses to persist the information to the local message pool")
	} else if p.sequenceID != c.SequenceID {
		fmt.Println("The message sequence numbers do not match and refuse to persist the information to the local message pool")
	} else if !p.RsaVerySignWithSha256(digestByte, c.Sign, MessageNodePubKey) {
		fmt.Println("Node signature verification failed!, refuse to persist information to the local message pool")
	} else {
		p.setCommitConfirmMap(c.Digest, c.NodeID, true)
		count := 0
		for range p.commitConfirmCount[c.Digest] {
			count++
		}
		//If the node has received at least 2f +1 commit messages (including itself), and the node has not responded, and the commit broadcast has been performed, the information is submitted to the local message pool, and the reply is successfully marked to the client!
		p.lock.Lock()
		if count >= nodeCount/3*2 && !p.isReply[c.Digest] && p.isCommitBordcast[c.Digest] {
			fmt.Println("This node has received Commit information from at least 2f+1 nodes (including the local node)...")
			//Submit message information to the local message pool！
			localMessagePool = append(localMessagePool, p.messagePool[c.Digest].Message)
			info := p.node.nodeID + "Node has msgid:" + strconv.Itoa(p.messagePool[c.Digest].ID) + "Stored in the local message pool, the message content is：" + p.messagePool[c.Digest].Content
			fmt.Println(info)
			fmt.Println("replying the client...")
			tcpDial([]byte(info), p.messagePool[c.Digest].ClientAddr)
			p.isReply[c.Digest] = true
			fmt.Println("reply completed")
		}
		p.lock.Unlock()
	}
}

//Serial number accumulation
func (p *pbft) sequenceIDAdd() {
	p.lock.Lock()
	p.sequenceID++
	p.lock.Unlock()
}

//Broadcast to other nodes except yourself
func (p *pbft) broadcast(cmd command, content []byte) {
	for i := range nodeTable {
		if i == p.node.nodeID {
			continue
		}
		message := jointMessage(cmd, content)
		go tcpDial(message, nodeTable[i])
	}
}

//Open assignment for multimap
func (p *pbft) setPrePareConfirmMap(val, val2 string, b bool) {
	if _, ok := p.prePareConfirmCount[val]; !ok {
		p.prePareConfirmCount[val] = make(map[string]bool)
	}
	p.prePareConfirmCount[val][val2] = b
}

//Open assignment for multimap
func (p *pbft) setCommitConfirmMap(val, val2 string, b bool) {
	if _, ok := p.commitConfirmCount[val]; !ok {
		p.commitConfirmCount[val] = make(map[string]bool)
	}
	p.commitConfirmCount[val][val2] = b
}

//Pass in the node number to get the corresponding public key
func (p *pbft) getPubKey(nodeID string) []byte {
	key, err := ioutil.ReadFile("Keys/" + nodeID + "/" + nodeID + "_RSA_PUB")
	if err != nil {
		log.Panic(err)
	}
	return key
}

//Pass in the node number to get the corresponding private key
func (p *pbft) getPivKey(nodeID string) []byte {
	key, err := ioutil.ReadFile("Keys/" + nodeID + "/" + nodeID + "_RSA_PIV")
	if err != nil {
		log.Panic(err)
	}
	return key
}
