package main

import (
	"os"
)

func main() {
	f, err := os.Open("./testdata/test_topology.txt")
	if err != nil {
		panic(err)
	}

	nwt, err := NewNetworkTypology(f)
	if err != nil {
		panic(err)
	}

	configs := []NodeConfig{
		{
			id: 0,
			msg: NodeMsg{
				msg:   "(0 -> 6)",
				delay: 30,
				dst:   6,
			},
		},
		{
			id: 1,
			msg: NodeMsg{
				msg:   "(1 -> 4)",
				delay: 30,
				dst:   4,
			},
		},
		{
			id: 2,
			msg: NodeMsg{
				msg:   "(2 -> 3)",
				delay: 30,
				dst:   3,
			},
		},
		{
			id: 3,
			msg: NodeMsg{
				msg:   "(3 -> 2)",
				delay: 30,
				dst:   2,
			},
		},
		{
			id: 4,
			msg: NodeMsg{
				msg:   "(4 -> 0)",
				delay: 30,
				dst:   0,
			},
		},
		{
			id: 5,
			msg: NodeMsg{
				msg:   "(5 -> 1)",
				delay: 30,
				dst:   1,
			},
		},
		{
			id: 6,
			msg: NodeMsg{
				msg:   "(6 -> 5)",
				delay: 30,
				dst:   5,
			},
		},
	}

	c := NewController(*nwt)
	c.Initialize(configs)
	c.Start()
}
