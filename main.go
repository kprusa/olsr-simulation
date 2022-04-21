package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
)

// Node represents a network node in the ad-hoc network.
type Node struct {
	label int

	// outputLog is where the Node will write all messages that it has sent.
	outputLog io.Writer

	// inputLog is where the Node will write all messages it has received.
	inputLog io.Writer

	// input represents the Node's wireless receiver.
	input <-chan string

	// output represents the Node's wireless transmitter.
	output chan<- string
}

// run starts the Node "listening" for messages.
func (n *Node) run(done <-chan struct{}) {
	// Continuously listen for new messages until done received by Controller.
	for {
		select {
		case in := <-n.input:
			_, err := fmt.Fprintln(n.inputLog, in)
			if err != nil {
				log.Panicf("%d could not write out log: %s", n.label, err)
			}
			log.Printf("%d received msg: %s\n", n.label, in)

		case <-done:
			log.Printf("%d recevied done message", n.label)
			return
		}
	}
}

// NewNode creates a network Node.
func NewNode(input <-chan string, label int) *Node {
	n := Node{}
	n.label = label
	n.input = input
	n.output = make(chan<- string)
	return &n
}

// LinkStatus represents whether a link is available or not.
type LinkStatus string

const (
	// UP represents a link that is available.
	UP LinkStatus = "UP"

	// DOWN represents a link that is unavailable.
	DOWN = "DOWN"
)

// LinkState represents a link's state at a given moment in time.
type LinkState struct {
	// time is the moment in time, inclusive, this state becomes valid.
	time int

	// status is the status of the link.
	status LinkStatus

	// fromNode is the source Node label.
	fromNode int

	// toNode is the destination Node label.
	toNode int
}

func (l *LinkState) String() string {
	return fmt.Sprintf("%d %s %d %d", l.time, l.status, l.fromNode, l.toNode)
}

func parseLinkState(state string) (*LinkState, error) {
	ls := &LinkState{}

	// Basic validation
	splitState := strings.Split(state, " ")
	if len(splitState) != 4 {
		return nil, ErrParseLinkState{msg: "must be of the form: '{TIME} {UP | DOWN} {LABEL} {LABEL}'"}
	}

	// Parse time
	time, err := strconv.Atoi(splitState[0])
	if err != nil {
		return nil, ErrParseLinkState{msg: fmt.Sprintf("time is not an integer: '%s'", splitState[0])}
	}
	if time < 0 {
		return nil, ErrParseLinkState{msg: fmt.Sprintf("time must be greater than 0: '%s'", splitState[0])}
	}
	ls.time = time

	// Parse status
	switch LinkStatus(splitState[1]) {
	case UP:
		ls.status = UP
	case DOWN:
		ls.status = DOWN
	default:
		return nil, ErrParseLinkState{msg: fmt.Sprintf("invalid status: '%s': must be {UP | DOWN}", splitState[1])}
	}

	// Parse labels
	lre, err := regexp.Compile("^\\d$")
	if err != nil {
		panic(err)
	}
	if !lre.Match([]byte(splitState[2])) {
		return nil, ErrParseLinkState{msg: fmt.Sprintf("invalid label: '%s': must be '^[0-9]$'", splitState[2])}
	}
	if !lre.Match([]byte(splitState[3])) {
		return nil, ErrParseLinkState{msg: fmt.Sprintf("invalid label: '%s': must be '^[0-9]$'", splitState[3])}
	}
	ls.fromNode, _ = strconv.Atoi(splitState[2])
	ls.toNode, _ = strconv.Atoi(splitState[3])

	return ls, nil
}

type Link struct {
	// fromNode is the source Node label.
	fromNode int

	// toNode is the destination Node label.
	toNode int

	states []LinkState
}

// isUp determines whether the link is available at the given time.
func (l *Link) isUp(time int) bool {
	up := false
	for _, state := range l.states {
		if time >= state.time && state.status == UP {
			up = true
			continue
		}
		if time >= state.time && state.status == DOWN {
			up = false
			continue
		}
	}
	return up
}

// QueryMsg enables the Controller to query the NetworkTopology to determine the state of a link at a given moment
// in time.
type QueryMsg struct {
	// fromNodeLabel is the source of the link.
	fromNodeLabel int

	// toNodeLabel is the destination of the link.
	toNodeLabel int

	// timeQuantum is the moment in time to check the status of the link.
	timeQuantum int
}

// NetworkTypology represents the ad-hoc network typology and is used by the Controller.
type NetworkTypology struct {
	links map[int]map[int]Link
}

type ErrParseLinkState struct {
	msg string
}

func (e ErrParseLinkState) Error() string {
	return fmt.Sprintf("parse link state: %s", e.msg)
}

func NewNetworkTypology(in io.ReadCloser) (*NetworkTypology, error) {
	defer func(in io.ReadCloser) {
		err := in.Close()
		if err != nil {
			log.Printf("unable to close input file: %s\n", err)
		}
	}(in)

	n := &NetworkTypology{}
	n.links = make(map[int]map[int]Link)

	r := bufio.NewReader(in)
	currTime := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		line = strings.TrimSuffix(line, "\n")

		ls, err := parseLinkState(line)
		if err != nil {
			log.Fatalln(err)
		}

		if ls.time < currTime {
			return nil, errors.New("entries in input must be sorted by increasing time")
		}
		currTime = ls.time

		// Add the new LinkState to the applicable link. If there is not a link, create one.
		dsts, ok := n.links[ls.fromNode]
		if !ok {
			link := Link{fromNode: ls.fromNode, toNode: ls.toNode}
			link.states = append(link.states, *ls)

			srcMap := make(map[int]Link)
			srcMap[ls.toNode] = link
			n.links[ls.fromNode] = srcMap
			continue
		}
		dst, ok := dsts[ls.toNode]
		if !ok {
			link := Link{fromNode: ls.fromNode, toNode: ls.toNode}
			link.states = append(link.states, *ls)

			dsts[ls.toNode] = link
			continue
		}

		dst.states = append(dst.states, *ls)
		dsts[ls.toNode] = dst
	}

	return n, nil
}

// Query enables to Controller to determine the current link-state at a time quantum.
func (n *NetworkTypology) Query(msg QueryMsg) bool {
	links, in := n.links[msg.fromNodeLabel]
	if !in {
		return false
	}

	link, in := links[msg.toNodeLabel]
	if !in {
		return false
	}

	return link.isUp(msg.timeQuantum)
}

// Controller is aware of the entire network typology and acts as a wireless network.
// Only used for the simulation (a real ad-hoc network would not have a centralized controller).
type Controller struct {
}

func main() {}
