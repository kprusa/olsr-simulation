======================

SETUP

    Simply execute setup.sh. The script will install Go and build the executable. Once the script successfully runs,
    an executable named "network-simulation" will be created in the working directory.

======================

EXECUTION

    Executing "network-simulation" with no arguments will show a usage message.

    ----------------------

    REQUIRED ARGUMENTS

        -nf string

            Node configuration file path.

            A path to a text file which includes newline separated node configurations. These configurations are the
            same as those described in the project specification.

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

            A path to a text file which includes newline separated topology values. These values are the same as those
            described in the project specification.

            The values have the following format:

                {TICK_NUM} {UP | DOWN} {FROM_NODE_ID} {TO_NODE_ID}

            EXAMPLE FILE CONTENTS

                10 UP 0 1
                10 UP 1 0
                20 DOWN 0 1
                20 DOWN 1 0
                21 UP 0 2
                25 UP 2 0

    ----------------------

    OPTIONAL ARGUMENTS

        -t int

            Tick duration in milliseconds. Specifies how fast the simulation will run. (default 1000)

        -rt int

            Number of ticks the simulation will run for. (default 120)

    ----------------------


======================

EXAMPLE EXECUTION

    ----------------------

    BASIC EXAMPLE

        The following command uses the included testdata to run a demonstration simulation.

            ./network-simulation -nf ./testdata/test_node_config.txt -tf ./testdata/test_topology.txt

    ----------------------

    INCREASING SIMULATION SPEED

        The following command sets the tick rate to 100ms, increasing the simulation speed.

            ./network-simulation -nf ./testdata/test_node_config.txt -tf ./testdata/test_topology.txt -t 100

    ----------------------

======================
