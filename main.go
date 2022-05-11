package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	tf := flag.String("tf", "", "Topology file path (Required)")
	nf := flag.String("nf", "", "Node configuration file path (Required)")
	t := flag.Int("t", 1000, "Tick duration in milliseconds. Specifies how fast the simulation will run")
	d := flag.Int("rt", 120, "Number of ticks to run the simulation for.")
	flag.Parse()

	if *tf == "" || *nf == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	f, err := os.Open(*tf)
	if err != nil {
		fmt.Printf("unable to open topology file: %s", *tf)
		os.Exit(1)
	}
	nwt, err := NewNetworkTypology(f)
	if err != nil {
		fmt.Printf("invalid network topology file: %s", err)
		os.Exit(1)
	}

	f, err = os.Open(*nf)
	if err != nil {
		fmt.Printf("unable to open topology file: %s", *tf)
		os.Exit(1)
	}
	configs, err := ReadNodeConfiguration(f)
	if err != nil {
		fmt.Printf("invalid node configuration file: %s", err)
		os.Exit(1)
	}

	td := time.Millisecond * time.Duration(*t)
	c := NewController(*nwt, td)
	c.Initialize(configs)
	c.Start(*d)
}
