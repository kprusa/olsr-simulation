package main

// HelloMessage represents a HELLO OLSR message.
type HelloMessage struct {
	src    NodeID
	unidir []NodeID
	bidir  []NodeID
	mpr    []NodeID
}

// DataMessage represents a DATA OLSR message.
type DataMessage struct {
	src     NodeID
	dst     NodeID
	nxtHop  NodeID
	fromnbr NodeID
	data    string
}

// TCMessage represents a topology control (TC) OLSR message.
type TCMessage struct {
	src    NodeID
	frombr NodeID
	seq    uint
	ms     []NodeID
}
