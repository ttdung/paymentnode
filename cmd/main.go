package main

//
//
import (
	node "github.com/ttdung/paymentnode/node"
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

	paymentnode.Start(os.Args)
}
