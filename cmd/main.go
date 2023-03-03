package main

//
//
import (
	"github.com/dungtt-astra/paymentnode/config"
	node "github.com/dungtt-astra/paymentnode/node"
	"log"
	"os"
)

// // Server Variable
var machine *node.Node

// // Init Function
func init() {

	var cfg = &config.Config{
		ChainId:       "astra_11110-1",
		Endpoint:      "http://128.199.238.171:26657",
		CoinType:      60,
		PrefixAddress: "astra",
		TokenSymbol:   "aastra",
		NodeAddr:      ":50005",
		Tcp:           "tcp",
	}

	log.Println("Init ....")

	machine = node.NewNode(cfg)
}

// // Main Function
func main() {
	// Starting Seoser

	machine.Start(os.Args)
}
