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

	// FIXME: This needs to be changed to the originator address of the TC message.
	// originator is the originator of the TCMessage (last-hop node to the destination).
	originator NodeID

	// holdUntil determines how long an entry will be held for before being expelled.
	holdUntil int

	// seq
	seq int
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

	// receivedLog is where the Node will write all data it has received.
	receivedLog io.WriteCloser

	// input represents the Node's wireless receiver.
	input <-chan interface{}

	// output represents the Node's wireless transmitter.
	output chan<- interface{}

	// nodeMsg will be sent by the node based on the message's delay.
	nodeMsg NodeMsg

	// routingTable maps destinations to routing entries.
	routingTable map[NodeID]RoutingEntry

	// routesChanged determines if the routingTable needs to be recalculated.
	routesChanged bool

	// topologyTable represents the Node's current perception of the network topology.
	// First NodeID destination's MPR, while the second ID is the destination.
	topologyTable map[NodeID]map[NodeID]TopologyEntry

	// topologyHoldTime is how long, in ticks, topology table entries will be held until they are expelled.
	topologyHoldTime int

	// tcSequenceNum is the current TCMessage sequence number.
	tcSequenceNum int

	// oneHopNeighbors is the set of 1-hop neighbors discovered by this node.
	oneHopNeighbors map[NodeID]OneHopNeighborEntry

	// twoHopNeighbors represents the 2-hop neighbors that can be reached via a 1-hop neighbor.
	// The second map is used for uniqueness and merely maps NodeID(s) to themselves.
	twoHopNeighbors map[NodeID]map[NodeID]NodeID

	// msSet
	msSet map[NodeID]NodeID

	// currentTick is the number of ticks since the node came online.
	currentTick int

	// neighborHoldTime is how long, in ticks, neighbor table entries will be held until they are expelled.
	neighborHoldTime int

	// tickDuration controls the Node's ticker.
	tickDuration time.Duration

	// helloSequences ensures the node ignores hello messages sent out-of-order.
	helloSequences map[NodeID]int

	// helloSequenceNum is the Node's HelloMessage sequence number.
	helloSequenceNum int
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
		if n.currentTick == n.nodeMsg.delay && !n.nodeMsg.sent {
			// Attempt to send data message
			msg := &DataMessage{
				src:     n.id,
				dst:     n.nodeMsg.dst,
				nxtHop:  0,
				fromnbr: 0,
				data:    n.nodeMsg.msg,
			}
			if !n.sendData(msg) {
				n.nodeMsg.delay += 30
			} else {
				n.nodeMsg.sent = true
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

func (n *Node) sendData(msg *DataMessage) bool {
	route, ok := n.routingTable[msg.dst]
	if ok {
		msg.fromnbr = n.id
		msg.nxtHop = route.nextHop

		n.output <- msg
		_, err := fmt.Fprintln(n.inputLog, msg)
		if err != nil {
			log.Panicf("%d could not write out log: %s", n.id, err)
		}
		log.Printf("node %d: sent:\t%s\n", n.id, msg)
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
		seq:    n.helloSequenceNum,
	}
	n.helloSequenceNum++
	n.output <- hello
	log.Printf("node %d: sent:\t%s", n.id, hello)
	_, err := fmt.Fprintln(n.outputLog, hello)
	if err != nil {
		log.Panicf("node %d: unable to log hello msg to output: %s", n.id, err)
	}
}

// sendTC sends a TCMessage including the most recent ms set for this node.
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

// calculateRoutingTable calculates all reachable destinations based on the topologyTable.
func (n *Node) calculateRoutingTable() {
	// Wipe the table clean, ensuring no stale routes.
	n.routingTable = make(map[NodeID]RoutingEntry)

	// Add all symmetric one-hop neighbors.
	for _, neighbor := range n.oneHopNeighbors {
		if neighbor.state == Bidirectional || neighbor.state == MPR {
			n.routingTable[neighbor.neighborID] = RoutingEntry{
				dst:      neighbor.neighborID,
				nextHop:  neighbor.neighborID,
				distance: 1,
			}
		}
	}

	// Add all two-hop neighbors.
	for neighbor, reachableTwoHops := range n.twoHopNeighbors {
		for dst := range reachableTwoHops {
			_, ok := n.routingTable[dst]
			if !ok {
				n.routingTable[dst] = RoutingEntry{
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
				_, ok := n.routingTable[entry.dst]
				if !ok {
					// No destination. Check if there's a routing entry that can reach the MPR of the destination.
					rEntry, ok := n.routingTable[entry.originator]
					if ok && rEntry.distance == h {
						newEntry = true
						n.routingTable[entry.dst] = RoutingEntry{
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
	for _, nodeID := range append(msg.bidir, msg.mpr...) {
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
	nodes := make([]struct {
		id      NodeID
		reaches int
	}, 0)
	for neighbor, twoHops := range twoHopNeighbors {
		// Only consider nodes as MPRs if they are bidirectional.
		ohn, _ := oneHopNeighbors[neighbor]
		if ohn.state == Unidirectional {
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
	// Ignore hello messages sent out-of-order
	seq, ok := n.helloSequences[msg.src]
	if !ok {
		n.helloSequences[msg.src] = msg.seq
	} else {
		if msg.seq <= seq {
			return
		} else {
			n.helloSequences[msg.src] = msg.seq
		}
	}

	// Update one-hop neighbors.
	n.oneHopNeighbors = updateOneHopNeighbors(msg, n.oneHopNeighbors, n.currentTick+n.neighborHoldTime, n.id)

	// Update two-hop neighbors
	n.twoHopNeighbors = updateTwoHopNeighbors(msg, n.twoHopNeighbors, n.id)

	n.oneHopNeighbors = calculateMPRs(n.oneHopNeighbors, n.twoHopNeighbors)

	// Update the msSet
	_, ok = n.msSet[msg.src]
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

	n.routesChanged = true
}

func (n *Node) handleData(msg *DataMessage) {
	if msg.dst == n.id {
		_, err := fmt.Fprintln(n.receivedLog, msg.data)
		if err != nil {
			log.Panicf("node %d: unable to log data to output: %s", n.id, err)
		}
		return
	}
	n.sendData(msg)
}

func updateTopologyTable(msg *TCMessage, topologyTable map[NodeID]map[NodeID]TopologyEntry, holdUntil int, id NodeID) map[NodeID]map[NodeID]TopologyEntry {
	entries, ok := topologyTable[msg.src]
	if ok {
		// Check if sequence number is new.
		for _, dst := range msg.ms {
			entry, ok := entries[dst]
			if ok && entry.seq > msg.seq {
				return topologyTable
			}
		}
	}
	// New sequence TC message. Clear all old entries and add new entries.
	topologyTable[msg.src] = make(map[NodeID]TopologyEntry)

	for _, dst := range msg.ms {
		if dst == id {
			continue
		}
		entries, _ := topologyTable[msg.src]
		entries[dst] = TopologyEntry{
			dst:        dst,
			originator: msg.src,
			holdUntil:  holdUntil,
			seq:        msg.seq,
		}
		topologyTable[msg.src] = entries
	}

	return topologyTable
}

func (n *Node) handleTC(msg *TCMessage) {
	// Ignore TC messages sent by this node.
	if msg.src == n.id {
		return
	}

	n.topologyTable = updateTopologyTable(msg, n.topologyTable, n.currentTick+n.topologyHoldTime, n.id)
	n.routesChanged = true

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
	sent  bool
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

	n.routingTable = make(map[NodeID]RoutingEntry)
	n.routesChanged = true

	n.topologyTable = make(map[NodeID]map[NodeID]TopologyEntry)
	n.topologyHoldTime = 30

	n.oneHopNeighbors = make(map[NodeID]OneHopNeighborEntry)
	n.twoHopNeighbors = make(map[NodeID]map[NodeID]NodeID)
	n.msSet = make(map[NodeID]NodeID)
	n.neighborHoldTime = 15
	return &n
}
