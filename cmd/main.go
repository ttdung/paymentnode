package main

//
//
import (
	node "github.com/dungtt-astra/paymentnode/node"
	"os"
)

// // Server Variable
var machine *node.Node

// // Init Function
func init() {
}

// // Main Function
func main() {
	// Starting Seoser

	machine.Start(os.Args)
}
