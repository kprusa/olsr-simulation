package main

import (
	"fmt"
	"io"
	"log"
)

// NodeID is a unique identifier used to differentiate nodes.
type NodeID uint

// Node represents a network node in the ad-hoc network.
type Node struct {
	id NodeID

	// outputLog is where the Node will write all messages that it has sent.
	outputLog io.Writer

	// inputLog is where the Node will write all messages it has received.
	inputLog io.Writer

	// input represents the Node's wireless receiver.
	input <-chan Message

	// output represents the Node's wireless transmitter.
	output chan<- Message
}

// run starts the Node "listening" for messages.
func (n *Node) run(done <-chan struct{}) {
	// Continuously listen for new messages until done received by Controller.
	for {
		select {
		case in := <-n.input:
			_, err := fmt.Fprintln(n.inputLog, in)
			if err != nil {
				log.Panicf("%d could not write out log: %s", n.id, err)
			}
			log.Printf("%d received msg: %s\n", n.id, in)

		case <-done:
			log.Printf("%d recevied done message", n.id)
			return
		}
	}
}

// NewNode creates a network Node.
func NewNode(input <-chan Message, label NodeID) *Node {
	n := Node{}
	n.id = label
	n.input = input
	n.output = make(chan<- Message)
	return &n
}
