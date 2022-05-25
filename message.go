package main

import (
	"fmt"
	"strings"
)

// separatedString creates a string from the items separated by the separator.
func separatedString[T fmt.Stringer](items []T, separator string) string {
	var strs []string
	for _, item := range items {
		strs = append(strs, item.String())
	}
	return strings.Join(strs, separator)
}

// HelloMessage represents a HELLO OLSR message.
type HelloMessage struct {
	Source          NodeID
	Unidirectional  []NodeID
	Bidirectional   []NodeID
	MultipointRelay []NodeID

	// Sequence numbers are added to ensure hello messages are delivered in order.
	// The sequence number is needed for the simulation, as hello messages may be delivered out-of-order due to
	// scheduling of goroutines.
	// In a real life scenario, a hello message transmitted by a node could never arrive at a neighbor before a
	// previously transmitted hello message.
	Sequence int
}

func (m HelloMessage) String() string {
	f := "* %d HELLO UNIDIR %s BIDIR %s MPR %s"
	return fmt.Sprintf(
		f,
		m.Source,
		separatedString(m.Unidirectional, " "),
		separatedString(m.Bidirectional, " "),
		separatedString(m.MultipointRelay, " "),
	)
}

// DataMessage represents a DATA OLSR message.
type DataMessage struct {
	Source       NodeID
	Destination  NodeID
	NextHop      NodeID
	FromNeighbor NodeID
	Data         string
}

func (m DataMessage) String() string {
	f := "%d %d DATA %d %d %s"
	return fmt.Sprintf(f, m.NextHop, m.FromNeighbor, m.Source, m.Destination, m.Data)
}

// TCMessage represents a topology control (TC) OLSR message.
type TCMessage struct {
	Source             NodeID
	FromNeighbor       NodeID
	Sequence           int
	MultipointRelaySet []NodeID
}

func (m TCMessage) String() string {
	f := "* %d TC %d %d MS %s"
	return fmt.Sprintf(f, m.FromNeighbor, m.Source, m.Sequence, separatedString(m.MultipointRelaySet, " "))
}
