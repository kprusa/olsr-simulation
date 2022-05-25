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
		c.nodeChannels[config.ID] = in

		node := NewNode(in, c.inputLink, config.ID, config.Message, c.tickDuration)
		c.nodes = append(c.nodes, *node)
	}
}

func (c *Controller) handleHelloMessage(hm *HelloMessage, epoch time.Time) {
	// Send the hello message along all neighbor links that are UP.
	for _, node := range c.nodes {
		if node.id == hm.Source {
			continue
		}
		q := QueryMsg{
			FromNode: hm.Source,
			ToNode:   node.id,
			AtTime:   int(time.Since(epoch) / c.tickDuration),
		}
		if c.topology.Query(q) {
			// Send the hello if a link is available.
			c.nodeChannels[node.id] <- hm
		}
	}
}

func (c *Controller) handleTCMessage(tcm *TCMessage, epoch time.Time) {
	// Send the TC message along all neighbor links that are UP.
	for _, node := range c.nodes {
		if node.id == tcm.Source {
			continue
		}
		q := QueryMsg{
			FromNode: tcm.FromNeighbor,
			ToNode:   node.id,
			AtTime:   int(time.Since(epoch) / c.tickDuration),
		}
		if c.topology.Query(q) {
			c.nodeChannels[node.id] <- tcm
		}
	}
}

func (c *Controller) handleDataMessage(dm *DataMessage, epoch time.Time) {
	// Send the Data message to the specified next-hop, if the link is UP.
	q := QueryMsg{
		FromNode: dm.FromNeighbor,
		ToNode:   dm.NextHop,
		AtTime:   int(time.Since(epoch) / c.tickDuration),
	}
	if c.topology.Query(q) {
		c.nodeChannels[dm.NextHop] <- dm
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
			n.Run(ctx)
		}(node)
	}

	// Launch a goroutine to handle routing of messages between nodes using the network topology.
	go func() {
		for {
			select {
			case msg := <-c.inputLink:
				switch t := msg.(type) {
				case *HelloMessage:
					go c.handleHelloMessage(msg.(*HelloMessage), epoch)
				case *DataMessage:
					go c.handleDataMessage(msg.(*DataMessage), epoch)
				case *TCMessage:
					go c.handleTCMessage(msg.(*TCMessage), epoch)
				default:
					log.Panicf("controller: invalid message type: %s\n", t)
				}
			}
		}
	}()

	// Launch a goroutine to send a done message to all nodes after the timer expires.
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

// NewController creates a Controller based on the supplied network typology.
func NewController(topology NetworkTypology, tickDuration time.Duration) *Controller {
	c := &Controller{}
	c.topology = topology
	c.nodeChannels = make(map[NodeID]chan interface{})
	c.tickDuration = tickDuration
	return c
}

// NodeConfig is used for the creation of nodes by a Controller during initialization.
type NodeConfig struct {
	ID      NodeID
	Message NodeMessage
}

// ReadNodeConfiguration parses newline separated node configurations from an io.ReadCloser.
// Configurations should be in the form: {Source} {Destination} "{Message}" {Delay}
func ReadNodeConfiguration(in io.Reader) ([]NodeConfig, error) {
	configs := make([]NodeConfig, 0)

	re := regexp.MustCompile(`(?P<Source>\d{1,2}) (?P<Destination>\d{1,2}) (?P<Message>".*?") (?P<Delay>\d{1,2})`)

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
			return nil, fmt.Errorf("invalid node config: ID is not an int: %s", line)
		}
		dst, err := strconv.Atoi(matches[2])
		if err != nil {
			return nil, fmt.Errorf("invalid node config: Destination is not an int: %s", line)
		}
		delay, err := strconv.Atoi(matches[4])
		if err != nil {
			return nil, fmt.Errorf("invalid node config: Delay is not an int: %s", line)
		}

		c := NodeConfig{
			ID: NodeID(id),
			Message: NodeMessage{
				Message:     matches[3][1 : len(matches[3])-1],
				Delay:       delay,
				Destination: NodeID(dst),
				Sent:        false,
			},
		}

		configs = append(configs, c)
	}
	return configs, nil
}
