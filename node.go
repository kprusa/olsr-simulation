package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"sort"
	"time"
)

type TopologyEntry struct {
	// dst is the MPR selector in the received TCMessage.
	dst NodeID

	// dstMPR is the originator of the TCMessage (last-hop node to the destination).
	dstMPR []NodeID

	// msSeqNum is the MPR selector (MS) sequence number, used to determine if a TCMessage contains new information.
	msSeqNum int

	// holdingTime determines how long an entry will be held for before being expelled.
	holdingTime int
}

type RoutingEntry struct {
	// dst is the destination node address (NodeID in this case).
	dst NodeID

	// nextHop is where to send a message to in order to reach the destination.
	nextHop NodeID

	// distance is the number of hops needed to reach the destination.
	distance int
}

type NeighborState int

const (
	Bidirectional NeighborState = iota
	Unidirectional
	MPR
)

type OneHopNeighborEntry struct {
	neighborID NodeID
	state      NeighborState
	holdUntil  int
}

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
	input <-chan interface{}

	// output represents the Node's wireless transmitter.
	output chan<- interface{}

	// nodeMsg will be sent by the node based on the message's delay.
	nodeMsg NodeMsg

	topologyTable []TopologyEntry

	routingTable []RoutingEntry

	oneHopNeighbors map[NodeID]OneHopNeighborEntry

	// twoHopNeighbors represents the 2-hop neighbors that can be reached via a 1-hop neighbor.
	// The second map is used for uniqueness and merely maps NodeID(s) to themselves.
	twoHopNeighbors map[NodeID]map[NodeID]NodeID

	currentTime int

	neighborHoldTime int
}

// run starts the Node "listening" for messages.
func (n *Node) run(done <-chan struct{}) {
	// Continuously listen for new messages until done received by Controller.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	n.currentTime = 0
	for _ = range ticker.C {
		select {
		case msg := <-n.input:
			_, err := fmt.Fprintln(n.inputLog, msg)
			if err != nil {
				log.Panicf("%d could not write out log: %s", n.id, err)
			}
			log.Printf("node %d: received:\t\t%s\n", n.id, msg)

			n.handler(msg)

		case <-done:
			log.Printf("node %d: recevied done message", n.id)
			return

		// Send the desired message after the specified delay.
		case <-time.After(n.nodeMsg.delay):
			n.output <- &DataMessage{
				src:     0,
				dst:     1,
				nxtHop:  1,
				fromnbr: 0,
				data:    n.nodeMsg.msg,
			}
		}
		// TODO: Update routing and topology tables.

		n.currentTime++
	}
}

// handler de-multiplexes messages to their respective handlers.
func (n *Node) handler(msg interface{}) {
	switch t := msg.(type) {
	case *HelloMessage:
		n.handleHello(msg.(*HelloMessage))
	case *DataMessage:
		n.handleData(msg.(*DataMessage))
	case *TCMessage:
		n.handleTC(msg.(*TCMessage))
	default:
		log.Panicf("node %d: invalid message type: %s\n", n.id, t)
	}
}

// updateOneHopNeighbors adds all new one-hop neighbors that can be reached.
func updateOneHopNeighbors(msg *HelloMessage, oneHopNeighbors map[NodeID]OneHopNeighborEntry, time, holdTime int, id NodeID) map[NodeID]OneHopNeighborEntry {
	entry, ok := oneHopNeighbors[msg.src]
	if !ok {
		// First time neighbor
		oneHopNeighbors[msg.src] = OneHopNeighborEntry{
			neighborID: msg.src,
			state:      Unidirectional,
			holdUntil:  time + holdTime,
		}
	} else {
		// Already unidirectional neighbor
		entry.holdUntil = time + holdTime

		// Check if the link state should be updated.
		for _, nodeID := range append(msg.unidir, msg.bidir...) {
			if nodeID == id {
				entry.state = Bidirectional
				break
			}
		}

		oneHopNeighbors[msg.src] = entry
	}
	return oneHopNeighbors
}

// updateTwoHopNeighbors adds all new two-hop neighbors that can be reached.
func updateTwoHopNeighbors(msg *HelloMessage, twoHopNeighbors map[NodeID]map[NodeID]NodeID, id NodeID) map[NodeID]map[NodeID]NodeID {
	// Delete all previous entries for the source by creating a new map.
	twoHops := make(map[NodeID]NodeID)
	for _, nodeID := range append(msg.unidir, msg.bidir...) {
		// Check for own id.
		if nodeID == id {
			continue
		}
		twoHops[nodeID] = nodeID
	}
	twoHopNeighbors[msg.src] = twoHops
	return twoHopNeighbors
}

// calculateMPRs creates a new MPR set based on the current neighbor tables.
func calculateMPRs(oneHopNeighbors map[NodeID]OneHopNeighborEntry, twoHopNeighbors map[NodeID]map[NodeID]NodeID) map[NodeID]OneHopNeighborEntry {
	// Copy one hop neighbors
	remainingTwoHops := make(map[NodeID]NodeID)
	nodes := make([]NodeID, 0)
	for node, v := range twoHopNeighbors {
		// Only consider nodes as MPRs if they are bidirectional.
		ohn, _ := oneHopNeighbors[node]
		if ohn.state == Unidirectional {
			continue
		}
		nodes = append(nodes, node)
		for k, _ := range v {
			remainingTwoHops[k] = k
		}
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i] < nodes[j]
	})

	// Set of MPRs
	mprs := make(map[NodeID]NodeID)

	for len(remainingTwoHops) > 0 {
		maxTwoHopsID := nodes[0]
		nodes = nodes[1:]

		mprs[maxTwoHopsID] = maxTwoHopsID

		for k, _ := range twoHopNeighbors[maxTwoHopsID] {
			delete(remainingTwoHops, k)
		}
	}

	// Update states of one-hop neighbors based on newly selected MPRs.
	for id, neigh := range oneHopNeighbors {
		_, ok := mprs[id]
		if ok {
			neigh.state = MPR
			oneHopNeighbors[id] = neigh
		} else {
			if neigh.state == MPR {
				neigh.state = Bidirectional
				oneHopNeighbors[id] = neigh
			}
		}
	}
	return oneHopNeighbors
}

// handleHello handles the processing of a HelloMessage.
func (n *Node) handleHello(msg *HelloMessage) {
	// Update one-hop neighbors.
	n.oneHopNeighbors = updateOneHopNeighbors(msg, n.oneHopNeighbors, n.currentTime, n.neighborHoldTime, n.id)

	// Update two-hop neighbors
	n.twoHopNeighbors = updateTwoHopNeighbors(msg, n.twoHopNeighbors, n.id)

	n.oneHopNeighbors = calculateMPRs(n.oneHopNeighbors, n.twoHopNeighbors)

	// Gather one-hop neighbor entries.
	biNeighbors := make([]NodeID, 0)
	uniNeighbors := make([]NodeID, 0)
	mprNeighbors := make([]NodeID, 0)
	for _, o := range n.oneHopNeighbors {
		switch o.state {
		case Unidirectional:
			uniNeighbors = append(uniNeighbors, o.neighborID)
		case Bidirectional:
			biNeighbors = append(biNeighbors, o.neighborID)
		case MPR:
			mprNeighbors = append(mprNeighbors, o.neighborID)
		default:
			log.Panicf("node %d: invalid one-hop neighbor type: %d", n.id, o.state)
		}
	}

	// Construct new HelloMessage.
	hello := &HelloMessage{
		src:    n.id,
		unidir: uniNeighbors,
		bidir:  biNeighbors,
		mpr:    mprNeighbors,
	}
	// Send HelloMessage.
	n.output <- hello

	log.Printf("node %d: sent:\t\t%s", n.id, hello)
}

func (n *Node) handleData(msg *DataMessage) {
	fmt.Printf("node %d: received message of type: %s\n", n.id, DataType)
}

func (n *Node) handleTC(msg *TCMessage) {
	fmt.Printf("node %d: received message of type: %s\n", n.id, TCType)
}

type NodeMsg struct {
	msg   string
	delay time.Duration
	dst   NodeID
}

// NewNode creates a network Node.
func NewNode(input <-chan interface{}, output chan<- interface{}, id NodeID, nodeMsg NodeMsg) *Node {
	n := Node{}
	n.id = id
	n.input = input
	n.output = output
	n.nodeMsg = nodeMsg
	n.inputLog = ioutil.Discard
	n.outputLog = ioutil.Discard
	n.oneHopNeighbors = make(map[NodeID]OneHopNeighborEntry)
	n.twoHopNeighbors = make(map[NodeID]map[NodeID]NodeID)
	n.neighborHoldTime = 15
	return &n
}
