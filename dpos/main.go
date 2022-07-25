package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"sort"
	"strconv"
	"time"
)

const (
	voteNodeNum      = 100
	superNodeNum     = 10
	mineSuperNodeNum = 3
)

type block struct {
	prehash string

	hash string

	timestamp string

	data string

	height int

	address string
}

var blockchain []block

type node struct {
	votes int

	address string
}

type superNode struct {
	node
}

var voteNodesPool []node

var starNodesPool []superNode

var superStarNodesPool []superNode

func generateNewBlock(oldBlock block, data string, address string) block {
	newBlock := block{}
	newBlock.prehash = oldBlock.hash
	newBlock.data = data
	newBlock.timestamp = time.Now().Format("2006-01-02 15:04:05")
	newBlock.height = oldBlock.height + 1
	newBlock.address = address
	newBlock.getHash()
	return newBlock
}

func (b *block) getHash() {
	sumString := b.prehash + b.timestamp + b.data + b.address + strconv.Itoa(b.height)
	hash := sha256.Sum256([]byte(sumString))
	b.hash = hex.EncodeToString(hash[:])
}

func voting() {
	for _, v := range voteNodesPool {
		rInt, err := rand.Int(rand.Reader, big.NewInt(superNodeNum+1))
		if err != nil {
			log.Panic(err)
		}
		starNodesPool[int(rInt.Int64())].votes += v.votes
	}
}

func sortMineNodes() {
	sort.Slice(starNodesPool, func(i, j int) bool {
		return starNodesPool[i].votes > starNodesPool[j].votes
	})
	superStarNodesPool = starNodesPool[:mineSuperNodeNum]
}

func init() {

	for i := 0; i <= voteNodeNum; i++ {
		rInt, err := rand.Int(rand.Reader, big.NewInt(10000))
		if err != nil {
			log.Panic(err)
		}
		voteNodesPool = append(voteNodesPool, node{int(rInt.Int64()), "投票节点" + strconv.Itoa(i)})
	}

	for i := 0; i <= superNodeNum; i++ {
		starNodesPool = append(starNodesPool, superNode{node{0, "超级节点" + strconv.Itoa(i)}})
	}
}

func main() {
	fmt.Println("初始化", voteNodeNum, "个投票节点...")
	fmt.Println(voteNodesPool)
	fmt.Println("当前存在的", superNodeNum, "个竞选节点")
	fmt.Println(starNodesPool)
	fmt.Println("投票节点们开始进行投票...")
	voting()
	fmt.Println("结束投票，查看竞选节点们获得票数...")
	fmt.Println(starNodesPool)
	fmt.Println("对竞选节点按获得票数排序，前", mineSuperNodeNum, "名，当选超级节点")
	sortMineNodes()
	fmt.Println(superStarNodesPool)
	fmt.Println("开始挖矿...")
	genesisBlock := block{"0000000000000000000000000000000000000000000000000000000000000000", "", time.Now().Format("2006-01-02 15:04:05"), "我是创世区块", 1, "000000000"}
	genesisBlock.getHash()
	blockchain = append(blockchain, genesisBlock)
	fmt.Println(blockchain[0])
	i, j := 0, 0
	for {
		time.Sleep(time.Second)
		newBlock := generateNewBlock(blockchain[i], "我是区块内容", superStarNodesPool[j].address)
		blockchain = append(blockchain, newBlock)
		fmt.Println(blockchain[i+1])
		i++
		j++
		j = j % len(superStarNodesPool)
	}
}
