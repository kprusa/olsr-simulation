package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"time"
)

type topologyEntry struct {
	// dst is the mpr selector in the received TCMessage.
	dst NodeID

	// originator is the originator of the TCMessage (last-hop node to the destination).
	originator NodeID

	// holdUntil determines how long an entry will be held for before being expelled.
	holdUntil int

	// seq
	seq int
}

type routingEntry struct {
	// dst is the destination node address (NodeID in this case).
	dst NodeID

	// nextHop is where to send a message to in order to reach the destination.
	nextHop NodeID

	// distance is the number of hops needed to reach the destination.
	distance int
}

// NeighborState represents a Node's perception of the state of a link with a neighbor, based on HelloMessage(s).
type NeighborState int

const (
	// bidirectional is a link which the Node has received a HelloMessage via, where the HelloMessage includes
	// the receiving Node's ID in the unidirectional list.
	bidirectional NeighborState = iota

	// unidirectional is a link which a Node has received a HelloMessage via.
	unidirectional

	// mpr is a link which a Node has selected as a multipoint relay.
	mpr
)

// oneHopNeighborEntry are neighbors that can be reached along a direct link.
type oneHopNeighborEntry struct {
	neighborID NodeID
	state      NeighborState
	holdUntil  int
}

// NodeID is a unique identifier used to differentiate nodes.
type NodeID uint

func (n NodeID) String() string {
	return strconv.Itoa(int(n))
}

// Node represents a network node in the ad-hoc network.
type Node struct {
	id NodeID

	// outputLog is where the Node will write all messages that it has Sent.
	outputLog io.WriteCloser

	// inputLog is where the Node will write all messages it has received.
	inputLog io.WriteCloser

	// receivedLog is where the Node will write all Data it has received.
	receivedLog io.WriteCloser

	// input represents the Node's wireless receiver.
	input <-chan interface{}

	// output represents the Node's wireless transmitter.
	output chan<- interface{}

	// nodeMsg will be Sent by the node based on the message's Delay.
	nodeMsg NodeMessage

	// routingTable maps destinations to routing entries.
	routingTable map[NodeID]routingEntry

	// routesChanged determines if the routingTable needs to be recalculated.
	routesChanged bool

	// topologyTable represents the Node's current perception of the network topology.
	// First NodeID destination's mpr, while the second ID is the destination.
	topologyTable map[NodeID]map[NodeID]topologyEntry

	// topologyHoldTime is how long, in ticks, topology table entries will be held until they are expelled.
	topologyHoldTime int

	// tcSequenceNum is the current TCMessage sequence number.
	tcSequenceNum int

	// oneHopNeighbors is the set of 1-hop neighbors discovered by this node.
	oneHopNeighbors map[NodeID]oneHopNeighborEntry

	// twoHopNeighbors represents the 2-hop neighbors that can be reached via a 1-hop neighbor.
	// The second map is used for uniqueness and merely maps NodeID(s) to themselves.
	twoHopNeighbors map[NodeID]map[NodeID]NodeID

	// msSet is the set of nodes that have selected this Node as an mpr.
	msSet map[NodeID]NodeID

	// currentTick is the number of ticks since the node came online.
	currentTick int

	// neighborHoldTime is how long, in ticks, neighbor table entries will be held until they are expelled.
	neighborHoldTime int

	// tickDuration controls the Node's ticker.
	tickDuration time.Duration

	// helloSequences ensures the node ignores hello messages Sent out-of-order.
	helloSequences map[NodeID]int

	// helloSequenceNum is the Node's HelloMessage sequence number.
	helloSequenceNum int
}

// Run starts the Node "listening" for messages.
func (n *Node) Run(ctx context.Context) {
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
	for range ticker.C {
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
		if n.currentTick == n.nodeMsg.Delay && !n.nodeMsg.Sent {
			// Attempt to send Data message
			msg := &DataMessage{
				Source:       n.id,
				Destination:  n.nodeMsg.Destination,
				NextHop:      0,
				FromNeighbor: 0,
				Data:         n.nodeMsg.Message,
			}
			if !n.sendData(msg) {
				n.nodeMsg.Delay += 30
			} else {
				n.nodeMsg.Sent = true
			}
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

		if n.routesChanged {
			n.calculateRoutingTable()
			n.routesChanged = false
		}

		n.currentTick++
	}
}

// sendData sends the Node's NodeMessage as a DataMessage if there is a route to the destination.
func (n *Node) sendData(msg *DataMessage) bool {
	route, in := n.routingTable[msg.Destination]
	if in {
		msg.FromNeighbor = n.id
		msg.NextHop = route.nextHop

		n.output <- msg
		_, err := fmt.Fprintln(n.inputLog, msg)
		if err != nil {
			log.Panicf("%d could not write out log: %s", n.id, err)
		}
		log.Printf("node %d: Sent:\t%s\n", n.id, msg)
		return true
	}
	return false
}

// sendHello sends a HelloMessage for this node.
func (n *Node) sendHello() {
	// Gather one-hop neighbor entries.
	biNeighbors := make([]NodeID, 0)
	uniNeighbors := make([]NodeID, 0)
	mprNeighbors := make([]NodeID, 0)
	for _, o := range n.oneHopNeighbors {
		switch o.state {
		case unidirectional:
			uniNeighbors = append(uniNeighbors, o.neighborID)
		case bidirectional:
			biNeighbors = append(biNeighbors, o.neighborID)
		case mpr:
			mprNeighbors = append(mprNeighbors, o.neighborID)
		default:
			log.Panicf("node %d: invalid one-hop neighbor type: %d", n.id, o.state)
		}
	}

	hello := &HelloMessage{
		Source:          n.id,
		Unidirectional:  uniNeighbors,
		Bidirectional:   biNeighbors,
		MultipointRelay: mprNeighbors,
		Sequence:        n.helloSequenceNum,
	}
	n.helloSequenceNum++
	n.output <- hello
	log.Printf("node %d: Sent:\t%s", n.id, hello)
	_, err := fmt.Fprintln(n.outputLog, hello)
	if err != nil {
		log.Panicf("node %d: unable to log hello Message to output: %s", n.id, err)
	}
}

// sendTC sends a TCMessage including the most recent MultipointRelaySet set for this node.
func (n *Node) sendTC() {
	// Get the MS set node IDs to include in the TC message.
	msSet := make([]NodeID, 0)
	for _, id := range n.msSet {
		msSet = append(msSet, id)
	}
	sort.SliceStable(msSet, func(i, j int) bool {
		return msSet[i] < msSet[j]
	})

	tc := &TCMessage{
		Source:             n.id,
		FromNeighbor:       n.id,
		Sequence:           n.tcSequenceNum,
		MultipointRelaySet: msSet,
	}
	n.output <- tc
	log.Printf("node %d: Sent:\t%s", n.id, tc)
	_, err := fmt.Fprintln(n.outputLog, tc)
	if err != nil {
		log.Panicf("node %d: unable to log tc Message to output: %s", n.id, err)
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

// calculateRoutingTable calculates all reachable destinations based on the topologyTable.
func (n *Node) calculateRoutingTable() {
	// Wipe the table clean, ensuring no stale routes.
	n.routingTable = make(map[NodeID]routingEntry)

	// Add all symmetric one-hop neighbors.
	for _, neighbor := range n.oneHopNeighbors {
		if neighbor.state == bidirectional || neighbor.state == mpr {
			n.routingTable[neighbor.neighborID] = routingEntry{
				dst:      neighbor.neighborID,
				nextHop:  neighbor.neighborID,
				distance: 1,
			}
		}
	}

	// Add all two-hop neighbors.
	for neighbor, reachableTwoHops := range n.twoHopNeighbors {
		for dst := range reachableTwoHops {
			_, in := n.routingTable[dst]
			if !in {
				n.routingTable[dst] = routingEntry{
					dst:      dst,
					nextHop:  neighbor,
					distance: 2,
				}
			}
		}
	}

	// Add all remaining routes from topology table.
	for h := 2; h < 256; h++ {
		newEntry := false
		for _, neighborDsts := range n.topologyTable {
			for _, entry := range neighborDsts {
				// Check if there already exists a routing entry for the destination.
				_, in := n.routingTable[entry.dst]
				if !in {
					// No destination. Check if there's a routing entry that can reach the MultipointRelay of the destination.
					rEntry, in := n.routingTable[entry.originator]
					if in && rEntry.distance == h {
						newEntry = true
						n.routingTable[entry.dst] = routingEntry{
							dst:      entry.dst,
							nextHop:  rEntry.nextHop,
							distance: h + 1,
						}
					}
				}
			}
		}
		if !newEntry {
			break
		}
	}
}

// updateOneHopNeighbors adds all new one-hop neighbors that can be reached.
func updateOneHopNeighbors(msg *HelloMessage, oneHopNeighbors map[NodeID]oneHopNeighborEntry, holdUntil int, id NodeID) map[NodeID]oneHopNeighborEntry {
	entry, in := oneHopNeighbors[msg.Source]
	if !in {
		// First time neighbor
		oneHopNeighbors[msg.Source] = oneHopNeighborEntry{
			neighborID: msg.Source,
			state:      unidirectional,
			holdUntil:  holdUntil,
		}
	} else {
		// Already unidirectional neighbor
		entry.holdUntil = holdUntil

		// Check if the link state should be updated.
		included := false
		for _, nodeID := range append(msg.Unidirectional, append(msg.Bidirectional, msg.MultipointRelay...)...) {
			if nodeID == id {
				included = true
				break
			}
		}

		if included {
			entry.state = bidirectional
		} else {
			entry.state = unidirectional
		}

		oneHopNeighbors[msg.Source] = entry
	}
	return oneHopNeighbors
}

// updateTwoHopNeighbors adds all new two-hop neighbors that can be reached.
func updateTwoHopNeighbors(msg *HelloMessage, twoHopNeighbors map[NodeID]map[NodeID]NodeID, id NodeID) map[NodeID]map[NodeID]NodeID {
	// Delete all previous entries for the source by creating a new map.
	twoHops := make(map[NodeID]NodeID)
	for _, nodeID := range append(msg.Bidirectional, msg.MultipointRelay...) {
		// Check for own ID.
		if nodeID == id {
			continue
		}
		twoHops[nodeID] = nodeID
	}
	twoHopNeighbors[msg.Source] = twoHops
	return twoHopNeighbors
}

// calculateMPRs creates a new mpr set based on the current neighbor tables.
func calculateMPRs(oneHopNeighbors map[NodeID]oneHopNeighborEntry, twoHopNeighbors map[NodeID]map[NodeID]NodeID) map[NodeID]oneHopNeighborEntry {
	// Copy one hop neighbors
	remainingTwoHops := make(map[NodeID]NodeID)
	nodes := make([]struct {
		id      NodeID
		reaches int
	}, 0)
	for neighbor, twoHops := range twoHopNeighbors {
		// Only consider nodes as MPRs if they are bidirectional.
		ohn, _ := oneHopNeighbors[neighbor]
		if ohn.state == unidirectional {
			continue
		}
		nodes = append(nodes, struct {
			id      NodeID
			reaches int
		}{id: neighbor, reaches: len(twoHops)})

		for k := range twoHops {
			remainingTwoHops[k] = k
		}
	}

	// Sort neighbors based on the number of two-hop neighbors they reach.
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].reaches > nodes[j].reaches
	})

	// Set of MPRs
	mprs := make(map[NodeID]NodeID)

	for len(remainingTwoHops) > 0 {
		maxTwoHops := nodes[0]
		nodes = nodes[1:]

		mprs[maxTwoHops.id] = maxTwoHops.id

		for k := range twoHopNeighbors[maxTwoHops.id] {
			delete(remainingTwoHops, k)
		}
	}

	// Update states of one-hop neighbors based on newly selected MPRs.
	for id, neigh := range oneHopNeighbors {
		_, in := mprs[id]
		if in {
			neigh.state = mpr
			oneHopNeighbors[id] = neigh
		} else {
			if neigh.state == mpr {
				neigh.state = bidirectional
				oneHopNeighbors[id] = neigh
			}
		}
	}
	return oneHopNeighbors
}

// handleHello handles the processing of a HelloMessage.
func (n *Node) handleHello(msg *HelloMessage) {
	// Ignore hello messages Sent out-of-order
	seq, in := n.helloSequences[msg.Source]
	if !in {
		n.helloSequences[msg.Source] = msg.Sequence
	} else {
		if msg.Sequence <= seq {
			return
		} else {
			n.helloSequences[msg.Source] = msg.Sequence
		}
	}

	// Update one-hop neighbors.
	n.oneHopNeighbors = updateOneHopNeighbors(msg, n.oneHopNeighbors, n.currentTick+n.neighborHoldTime, n.id)

	// Update two-hop neighbors
	n.twoHopNeighbors = updateTwoHopNeighbors(msg, n.twoHopNeighbors, n.id)

	n.oneHopNeighbors = calculateMPRs(n.oneHopNeighbors, n.twoHopNeighbors)

	// Update the msSet
	_, in = n.msSet[msg.Source]
	isMS := false
	// Check if this node is in the MultipointRelay set from the HELLO message.
	for _, nodeID := range msg.MultipointRelay {
		if nodeID == n.id {
			isMS = true
			break
		}
	}
	// Previously an MS, but no longer are.
	if in && !isMS {
		delete(n.msSet, msg.Source)
	}
	// New MS.
	if !in && isMS {
		n.msSet[msg.Source] = msg.Source
	}

	n.routesChanged = true
}

func (n *Node) handleData(msg *DataMessage) {
	if msg.Destination == n.id {
		_, err := fmt.Fprintln(n.receivedLog, msg.Data)
		if err != nil {
			log.Panicf("node %d: unable to log Data to output: %s", n.id, err)
		}
		return
	}
	n.sendData(msg)
}

func updateTopologyTable(msg *TCMessage, topologyTable map[NodeID]map[NodeID]topologyEntry, holdUntil int, id NodeID) map[NodeID]map[NodeID]topologyEntry {
	entries, in := topologyTable[msg.Source]
	if in {
		// Check if sequence number is new.
		for _, dst := range msg.MultipointRelaySet {
			entry, in := entries[dst]
			if in && entry.seq > msg.Sequence {
				return topologyTable
			}
		}
	}
	// New sequence TC message. Clear all old entries and add new entries.
	topologyTable[msg.Source] = make(map[NodeID]topologyEntry)

	for _, dst := range msg.MultipointRelaySet {
		if dst == id {
			continue
		}
		entries, _ := topologyTable[msg.Source]
		entries[dst] = topologyEntry{
			dst:        dst,
			originator: msg.Source,
			holdUntil:  holdUntil,
			seq:        msg.Sequence,
		}
		topologyTable[msg.Source] = entries
	}

	return topologyTable
}

func (n *Node) handleTC(msg *TCMessage) {
	// Ignore TC messages Sent by this node.
	if msg.Source == n.id {
		return
	}

	n.topologyTable = updateTopologyTable(msg, n.topologyTable, n.currentTick+n.topologyHoldTime, n.id)
	n.routesChanged = true

	// Only forward TC message if this node is an MultipointRelay of the neighbor which Sent the TC message.
	doFwd := false
	for _, id := range n.msSet {
		if id == msg.FromNeighbor {
			doFwd = true
		}
	}
	if !doFwd {
		return
	}

	// Update the from-neighbor field.
	msg.FromNeighbor = n.id

	// Send the updated Message.
	n.output <- msg

	log.Printf("node %d: Sent:\t%s", n.id, msg)
	_, err := fmt.Fprintln(n.outputLog, msg)
	if err != nil {
		log.Panicf("node %d: unable to log tc Message to output: %s", n.id, err)
	}
}

// NodeMessage is a message sent by a Node after the specified Delay.
type NodeMessage struct {
	Message     string
	Delay       int
	Destination NodeID
	Sent        bool
}

// NewNode creates a network Node.
func NewNode(input <-chan interface{}, output chan<- interface{}, id NodeID, nodeMsg NodeMessage, tickDur time.Duration) *Node {
	n := Node{}
	n.id = id
	n.input = input
	n.output = output
	n.nodeMsg = nodeMsg
	n.tickDuration = tickDur

	_ = os.Mkdir("./log", 0750)

	// Create logging files for this node.
	inputLog, err := os.Create(fmt.Sprintf("./log/%d_in.txt", n.id))
	if err != nil {
		panic(err)
	}
	n.inputLog = inputLog
	outputLog, err := os.Create(fmt.Sprintf("./log/%d_out.txt", n.id))
	if err != nil {
		panic(err)
	}
	n.outputLog = outputLog
	receivedLog, err := os.Create(fmt.Sprintf("./log/%d_received.txt", n.id))
	if err != nil {
		panic(err)
	}
	n.receivedLog = receivedLog

	n.helloSequences = make(map[NodeID]int)

	n.routingTable = make(map[NodeID]routingEntry)
	n.routesChanged = true

	n.topologyTable = make(map[NodeID]map[NodeID]topologyEntry)
	n.topologyHoldTime = 30

	n.oneHopNeighbors = make(map[NodeID]oneHopNeighborEntry)
	n.twoHopNeighbors = make(map[NodeID]map[NodeID]NodeID)
	n.msSet = make(map[NodeID]NodeID)
	n.neighborHoldTime = 15
	return &n
}
