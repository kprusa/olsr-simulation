package main

import (
	"fmt"
	"strconv"
	"strings"
)

func separatedString(items []NodeID, sep string) string {
	var strs []string
	for _, item := range items {
		strs = append(strs, strconv.Itoa(int(item)))
	}
	return strings.Join(strs, sep)
}

type MsgType string

const (
	HelloType MsgType = "HELLO"
	DataType  MsgType = "DATA"
	TCType    MsgType = "TC"
)

// HelloMessage represents a HELLO OLSR message.
type HelloMessage struct {
	type_  MsgType
	src    NodeID
	unidir []NodeID
	bidir  []NodeID
	mpr    []NodeID
}

func (m *HelloMessage) Type() MsgType {
	return HelloType
}

func (m *HelloMessage) String() string {
	f := "* %d HELLO UNIDIR %s BIDIR %s MPR %s"
	return fmt.Sprintf(
		f,
		m.src,
		separatedString(m.unidir, " "),
		separatedString(m.bidir, " "),
		separatedString(m.mpr, " "),
	)
}

// DataMessage represents a DATA OLSR message.
type DataMessage struct {
	type_   MsgType
	src     NodeID
	dst     NodeID
	nxtHop  NodeID
	fromnbr NodeID
	data    string
}

func (m *DataMessage) Type() MsgType {
	return DataType
}

func (m *DataMessage) String() string {
	f := "%d %d DATA %d %d %s"
	return fmt.Sprintf(f, m.nxtHop, m.fromnbr, m.src, m.dst, m.data)
}

// TCMessage represents a topology control (TC) OLSR message.
type TCMessage struct {
	type_  MsgType
	src    NodeID
	frombr NodeID
	seq    uint
	ms     []NodeID
}

func (m *TCMessage) Type() MsgType {
	return TCType
}

func (m *TCMessage) String() string {
	f := "* %d TC %d %d MS %s"
	return fmt.Sprintf(f, m.frombr, m.src, m.seq, separatedString(m.ms, " "))
}

type Message interface {
	fmt.Stringer
	Type() MsgType
}
