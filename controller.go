package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Controller is aware of the entire network typology and acts as a wireless network.
// Only used for the simulation (a real ad-hoc network would not have a centralized controller).
type Controller struct {
	// topology represents the network topology for the given set of nodes.
	topology NetworkTypology

	// inputLink is an aggregate channel to which all nodes send messages to.
	inputLink chan interface{}

	// nodeChannels is a mapping between each node and its input channel.
	nodeChannels map[NodeID]chan interface{}

	// nodes holds all running nodes which this controller is responsible for.
	nodes []Node

	// tickDuration controls how quickly the simulation runs.
	tickDuration time.Duration
}

// Initialize creates new nodes based on the supplied configuration and establishes channels.
func (c *Controller) Initialize(nodes []NodeConfig) {
	c.inputLink = make(chan interface{})
	for _, config := range nodes {
		in := make(chan interface{})
		c.nodeChannels[config.id] = in

		node := NewNode(in, c.inputLink, config.id, config.msg, c.tickDuration)
		c.nodes = append(c.nodes, *node)
	}
}

func (c *Controller) handleHello(hm *HelloMessage, epoch time.Time) {
	// Send the hello message along all neighbor links that are UP.
	for _, node := range c.nodes {
		if node.id == hm.src {
			continue
		}
		q := QueryMsg{
			fromNode:    hm.src,
			toNode:      node.id,
			timeQuantum: int(time.Since(epoch) / c.tickDuration),
		}
		if c.topology.Query(q) {
			// Send the hello if a link is available.
			c.nodeChannels[node.id] <- hm
		}
	}
}

func (c *Controller) handleTC(tcm *TCMessage, epoch time.Time) {
	// Send the hello message along all neighbor links that are UP.
	for _, node := range c.nodes {
		if node.id == tcm.src {
			continue
		}
		q := QueryMsg{
			fromNode:    tcm.fromnbr,
			toNode:      node.id,
			timeQuantum: int(time.Since(epoch) / c.tickDuration),
		}
		if c.topology.Query(q) {
			c.nodeChannels[node.id] <- tcm
		}
	}
}

func (c *Controller) handleData(dm *DataMessage, epoch time.Time) {
	// Send the hello message along all neighbor links that are UP.
	q := QueryMsg{
		fromNode:    dm.fromnbr,
		toNode:      dm.nxtHop,
		timeQuantum: int(time.Since(epoch) / c.tickDuration),
	}
	if c.topology.Query(q) {
		c.nodeChannels[dm.nxtHop] <- dm
	}
}

// Start runs all nodes and starts the controller.
func (c *Controller) Start(ticks int) {
	// Define a context to enable sending a done message to all nodes.
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	// Establish an epoch, which will be used in conjunction with the NetworkTopology.
	epoch := time.Now()

	// Start up all the nodes
	for _, node := range c.nodes {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()
			n.run(ctx)
		}(node)
	}

	// Launch a go routine to handle routing of messages between nodes using the network topology.
	go func() {
		for {
			select {
			case msg := <-c.inputLink:
				switch t := msg.(type) {
				case *HelloMessage:
					go c.handleHello(msg.(*HelloMessage), epoch)
				case *DataMessage:
					go c.handleData(msg.(*DataMessage), epoch)
				case *TCMessage:
					go c.handleTC(msg.(*TCMessage), epoch)
				default:
					log.Panicf("controller: invalid message type: %s\n", t)
				}
			}
		}
	}()

	// Launch a go routine to send a done message to all nodes after the timer expires.
	go func() {
		<-time.NewTimer(c.tickDuration * time.Duration(ticks)).C
		cancel()
		// Flush the input link, ensuring all nodes will receive the done message.
		for len(c.inputLink) > 0 {
			<-c.inputLink
		}
	}()

	// Wait for all nodes to return.
	wg.Wait()
}

// NodeConfig is used for the creation of nodes by a Controller during initialization.
type NodeConfig struct {
	id  NodeID
	msg NodeMsg
}

// ReadNodeConfiguration parses newline separated node configurations from an io.ReadCloser.
// Configurations should be in the form: {src} {dst} "{msg}" {delay}
func ReadNodeConfiguration(in io.ReadCloser) ([]NodeConfig, error) {
	configs := make([]NodeConfig, 0)

	re := regexp.MustCompile(`(?P<src>\d{1,2}) (?P<dst>\d{1,2}) (?P<msg>".*?") (?P<delay>\d{1,2})`)

	r := bufio.NewReader(in)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		line = strings.TrimSuffix(line, "\n")
		matches := re.FindStringSubmatch(line)

		id, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("invalid node config: id is not an int: %s", line)
		}
		dst, err := strconv.Atoi(matches[2])
		if err != nil {
			return nil, fmt.Errorf("invalid node config: dst is not an int: %s", line)
		}
		delay, err := strconv.Atoi(matches[4])
		if err != nil {
			return nil, fmt.Errorf("invalid node config: delay is not an int: %s", line)
		}

		c := NodeConfig{
			id: NodeID(id),
			msg: NodeMsg{
				msg:   matches[3][1 : len(matches[3])-1],
				delay: delay,
				dst:   NodeID(dst),
				sent:  false,
			},
		}

		configs = append(configs, c)
	}
	return configs, nil
}

// NewController creates a Controller based on the supplied network typology.
func NewController(topology NetworkTypology, tickDuration time.Duration) *Controller {
	c := &Controller{}
	c.topology = topology
	c.nodeChannels = make(map[NodeID]chan interface{})
	c.tickDuration = tickDuration
	return c
}
