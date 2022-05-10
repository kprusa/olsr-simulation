package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"time"
)

type TopologyEntry struct {
	// dst is the MPR selector in the received TCMessage.
	dst NodeID

	// dstNextHop is the originator of the TCMessage (last-hop node to the destination).
	dstNextHop NodeID

	// holdUntil determines how long an entry will be held for before being expelled.
	holdUntil int
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
	outputLog io.WriteCloser

	// inputLog is where the Node will write all messages it has received.
	inputLog io.WriteCloser

	// input represents the Node's wireless receiver.
	input <-chan interface{}

	// output represents the Node's wireless transmitter.
	output chan<- interface{}

	// nodeMsg will be sent by the node based on the message's delay.
	nodeMsg NodeMsg

	// topologyTable represents the Node's current perception of the network topology.
	// First NodeID is the next-hop neighbor, while the second ID is the destination.
	topologyTable map[NodeID]map[NodeID]TopologyEntry

	// topologySequences maps each NodeID which this node has received a TCMessage from to that Node's sequence number.
	topologySequences map[NodeID]int

	// tcSequenceNum is the current TCMessage sequence number.
	tcSequenceNum int

	// topologyHoldTime is how long, in ticks, topology table entries will be held until they are expelled.
	topologyHoldTime int

	routingTable []RoutingEntry

	// oneHopNeighbors is the set of 1-hop neighbors discovered by this node.
	oneHopNeighbors map[NodeID]OneHopNeighborEntry

	// twoHopNeighbors represents the 2-hop neighbors that can be reached via a 1-hop neighbor.
	// The second map is used for uniqueness and merely maps NodeID(s) to themselves.
	twoHopNeighbors map[NodeID]map[NodeID]NodeID

	// msSet
	msSet map[NodeID]NodeID

	// prevMSSet is the most recent TCMessage sent, enabling a check to be performed to determine if a new TCMessage
	// needs to be sent.
	prevMSSet []NodeID

	// currentTick is the number of ticks since the node came online.
	currentTick int

	// neighborHoldTime is how long, in ticks, neighbor table entries will be held until they are expelled.
	neighborHoldTime int

	// tickDuration controls the Node's ticker.
	tickDuration time.Duration
}

// run starts the Node "listening" for messages.
func (n *Node) run(ctx context.Context) {
	// Continuously listen for new messages until done received by Controller.
	ticker := time.NewTicker(n.tickDuration)
	defer ticker.Stop()
	defer func(log io.WriteCloser) {
		_ = log.Close()
	}(n.inputLog)
	defer func(log io.WriteCloser) {
		_ = log.Close()
	}(n.outputLog)

	n.currentTick = 0
	for _ = range ticker.C {
		select {
		case <-ctx.Done():
			log.Printf("node %d: recevied done message", n.id)
			return

		case msg := <-n.input:
			_, err := fmt.Fprintln(n.inputLog, msg)
			if err != nil {
				log.Panicf("%d could not write out log: %s", n.id, err)
			}
			log.Printf("node %d: received:\t%s\n", n.id, msg)

			n.handler(msg)
		default:
		}

		if n.currentTick%5 == 0 {
			n.sendHello()
		}
		if n.currentTick%10 == 0 && len(n.msSet) > 0 {
			n.sendTC()
		}
		if n.currentTick == n.nodeMsg.delay {
			// send data msg
		}

		// Remove old entries from the neighbor tables.
		for k, entry := range n.oneHopNeighbors {
			if entry.holdUntil <= n.currentTick {
				delete(n.oneHopNeighbors, k)
				delete(n.twoHopNeighbors, k)
			}
		}
		// Remove old entries from the TC tables.
		for _, dst := range n.topologyTable {
			for k, entry := range dst {
				if entry.holdUntil <= n.currentTick {
					delete(dst, k)
				}
			}
		}
		// TODO: Recalculate the routing table, if necessary.

		n.currentTick++
	}
}

// sendHello sends a HelloMessage for this node.
func (n *Node) sendHello() {
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

	hello := &HelloMessage{
		src:    n.id,
		unidir: uniNeighbors,
		bidir:  biNeighbors,
		mpr:    mprNeighbors,
	}
	n.output <- hello
	log.Printf("node %d: sent:\t%s", n.id, hello)
	_, err := fmt.Fprintln(n.outputLog, hello)
	if err != nil {
		log.Panicf("node %d: unable to log hello msg to output: %s", n.id, err)
	}
}

// sendTC sends a TCMessage if there has been a change in this nodes MS set.
func (n *Node) sendTC() {
	// Get the MS set node IDs to include in the TC message.
	msSet := make([]NodeID, 0)
	for _, id := range n.msSet {
		msSet = append(msSet, id)
	}
	sort.SliceStable(msSet, func(i, j int) bool {
		return msSet[i] < msSet[j]
	})

	changed := false
	if len(n.prevMSSet) == len(msSet) {
		for i, _ := range n.prevMSSet {
			if n.prevMSSet[i] != msSet[i] {
				changed = true
				break
			}
		}
	} else {
		changed = true
	}
	// Only send a new TCMessage if the MS set has changed.
	if !changed {
		return
	}
	n.prevMSSet = msSet

	tc := &TCMessage{
		src:     n.id,
		fromnbr: n.id,
		seq:     n.tcSequenceNum,
		ms:      msSet,
	}
	n.output <- tc
	log.Printf("node %d: sent:\t%s", n.id, tc)
	_, err := fmt.Fprintln(n.outputLog, tc)
	if err != nil {
		log.Panicf("node %d: unable to log tc msg to output: %s", n.id, err)
	}

	n.tcSequenceNum++
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

func (n *Node) calculateRoutingTable() {

}

// updateOneHopNeighbors adds all new one-hop neighbors that can be reached.
func updateOneHopNeighbors(msg *HelloMessage, oneHopNeighbors map[NodeID]OneHopNeighborEntry, holdUntil int, id NodeID) map[NodeID]OneHopNeighborEntry {
	entry, ok := oneHopNeighbors[msg.src]
	if !ok {
		// First time neighbor
		oneHopNeighbors[msg.src] = OneHopNeighborEntry{
			neighborID: msg.src,
			state:      Unidirectional,
			holdUntil:  holdUntil,
		}
	} else {
		// Already unidirectional neighbor
		entry.holdUntil = holdUntil

		// Check if the link state should be updated.
		included := false
		for _, nodeID := range append(msg.unidir, append(msg.bidir, msg.mpr...)...) {
			if nodeID == id {
				included = true
				break
			}
		}

		if included {
			entry.state = Bidirectional
		} else {
			entry.state = Unidirectional
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
	n.oneHopNeighbors = updateOneHopNeighbors(msg, n.oneHopNeighbors, n.currentTick+n.neighborHoldTime, n.id)

	// Update two-hop neighbors
	n.twoHopNeighbors = updateTwoHopNeighbors(msg, n.twoHopNeighbors, n.id)

	n.oneHopNeighbors = calculateMPRs(n.oneHopNeighbors, n.twoHopNeighbors)

	// Update the msSet
	_, ok := n.msSet[msg.src]
	isMS := false
	// Check if this node is in the MPR set from the HELLO message.
	for _, nodeID := range msg.mpr {
		if nodeID == n.id {
			isMS = true
			break
		}
	}
	// Previously an MS, but no longer are.
	if ok && !isMS {
		delete(n.msSet, msg.src)
	}
	// New MS.
	if !ok && isMS {
		n.msSet[msg.src] = msg.src
	}
}

func (n *Node) handleData(msg *DataMessage) {
	fmt.Printf("node %d: received message of type: %s\n", n.id, DataType)
}

func updateTopologyTable(msg *TCMessage, topologyTable map[NodeID]map[NodeID]TopologyEntry, holdUntil int, id NodeID) map[NodeID]map[NodeID]TopologyEntry {
	for _, dst := range msg.ms {
		if dst == id {
			continue
		}
		entries, ok := topologyTable[msg.fromnbr]
		if !ok {
			// First time seeing this destination
			entries = make(map[NodeID]TopologyEntry)
			entries[dst] = TopologyEntry{
				dst:        dst,
				dstNextHop: msg.fromnbr,
				holdUntil:  holdUntil,
			}
			topologyTable[msg.fromnbr] = entries
			continue
		}

		entry, ok := entries[dst]
		if !ok {
			// First time seeing this destination for the neighbor.
			entries[dst] = TopologyEntry{
				dst:        dst,
				dstNextHop: msg.fromnbr,
				holdUntil:  holdUntil,
			}
			continue
		} else {
			entry.holdUntil = holdUntil
			entries[dst] = entry
		}
	}

	return topologyTable
}

func (n *Node) handleTC(msg *TCMessage) {
	// Ignore TC messages sent by this node.
	if msg.src == n.id {
		return
	}

	// Ignore TC messages we've already seen.
	seq, ok := n.topologySequences[msg.src]
	if !ok {
		n.topologySequences[msg.src] = msg.seq
	} else {
		if msg.seq <= seq {
			return
		} else {
			n.topologySequences[msg.src] = msg.seq
		}
	}

	n.topologyTable = updateTopologyTable(msg, n.topologyTable, n.currentTick+n.topologyHoldTime, n.id)

	// Only forward TC message if this node is an MPR of the neighbor which sent the TC message.
	doFwd := false
	for _, id := range n.msSet {
		if id == msg.fromnbr {
			doFwd = true
		}
	}
	if !doFwd {
		return
	}

	// Update the from-neighbor field.
	msg.fromnbr = n.id

	// Send the updated msg.
	n.output <- msg

	log.Printf("node %d: sent:\t%s", n.id, msg)
	_, err := fmt.Fprintln(n.outputLog, msg)
	if err != nil {
		log.Panicf("node %d: unable to log tc msg to output: %s", n.id, err)
	}
}

type NodeMsg struct {
	msg   string
	delay int
	dst   NodeID
}

// NewNode creates a network Node.
func NewNode(input <-chan interface{}, output chan<- interface{}, id NodeID, nodeMsg NodeMsg, tickDur time.Duration) *Node {
	n := Node{}
	n.id = id
	n.input = input
	n.output = output
	n.nodeMsg = nodeMsg
	n.tickDuration = tickDur

	_ = os.Mkdir("./log", 0750)

	// Create logging files for this node.
	inputLog, err := os.Create(fmt.Sprintf("./log/%d_received.txt", n.id))
	if err != nil {
		panic(err)
	}
	n.inputLog = inputLog
	outputLog, err := os.Create(fmt.Sprintf("./log/%d_sent.txt", n.id))
	if err != nil {
		panic(err)
	}
	n.outputLog = outputLog

	n.topologyTable = make(map[NodeID]map[NodeID]TopologyEntry)
	n.topologySequences = make(map[NodeID]int)

	n.oneHopNeighbors = make(map[NodeID]OneHopNeighborEntry)
	n.twoHopNeighbors = make(map[NodeID]map[NodeID]NodeID)
	n.msSet = make(map[NodeID]NodeID)
	n.prevMSSet = make([]NodeID, 0)
	n.neighborHoldTime = 15
	return &n
}
