## Summary

This project uses Go to simulate a simplified version of an OSLR ad-hoc network as defined by [RFC 3626](https://datatracker.ietf.org/doc/html/rfc3626). For the simulation, each node
is run as a goroutine, with an input and output channel. A controller is used to
facilitate message interchange based on a supplied network topology.

There is a single executable, with no need to spawn additional processes.

All communication is facilitated through Go channels and each node logs all
communication to files.

---
## Execution

Executing "network-simulation" with no arguments will show a usage message.

During execution, all messages sent and received by nodes will be logged to stdout.

Post execution, a new directory "log" will appear. This directory will include three
log files for each node:

    {NODE_ID}_in.txt:

        A log file containing all messages that the given node received during the
        execution.

    {NODE_ID}_out.txt:

        A log file containing all messages that the given node sent during the
        execution.

    {NODE_ID}_received.txt:

        A log file containing all data that the given node received during the
        execution.

### Required Arguments

    -nf string

        Node configuration file path.

        A path to a text file which includes newline separated node configurations.
        These configurations are the same as those described in the project
        specification.

        The configurations have the following format:

            {SRC_NODE_ID} {DST_NODE_ID} "{MSG}" {MSG_DELAY}

        EXAMPLE FILE CONTENTS

            0 2 "(0 -> 2)" 30
            1 4 "(1 -> 4)" 40
            2 3 "hello 3, from 2" 40
            3 6 "(3 -> 6)" 40
            4 0 "(4 -> 0)" 30
            5 1 "this is 5, 1" 30
            6 5 "(6 -> 5)" 30

    -tf string

        Topology file path.

        A path to a text file which includes newline separated topology values. These
        values are the same as those described in the project specification.

        The values have the following format:

            {TICK_NUM} {UP | DOWN} {FROM_NODE_ID} {TO_NODE_ID}

        EXAMPLE FILE CONTENTS

            10 UP 0 1
            10 UP 1 0
            20 DOWN 0 1
            20 DOWN 1 0
            21 UP 0 2
            25 UP 2 0

### Optional Arguments

    -t int

        Tick duration in milliseconds. Specifies how fast the simulation will run.
        (default 1000)

    -rt int

        Number of ticks the simulation will run for. (default 120)

---
## Example Execution


### Basic Example

The following command uses the included testdata to run a demonstration
simulation.

```text
./network-simulation -nf ./testdata/test_node_config.txt -tf ./testdata/test_topology.txt
```


### Increasing Simulation Speed

The following command sets the tick rate to 100ms, increasing the simulation speed.

```text
./network-simulation -nf ./testdata/test_node_config.txt -tf ./testdata/test_topology.txt -t 100
```
