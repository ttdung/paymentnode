package main

//
//
import (
	node "github.com/ttdung/paymentnode/node"
	"log"
	"os"
)

// // Server Variable
var paymentnode *node.Node

// // Init Function
func init() {
}

// // Main Function
func main() {
	// Starting Seoser

	log.Println("Node start listen...")
	paymentnode.Start(os.Args)
}
