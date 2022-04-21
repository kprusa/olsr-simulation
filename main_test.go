package main

import (
    "io"
    "os"
    "reflect"
    "testing"
)

func TestLinkState_String(t *testing.T) {
    type fields struct {
        time     int
        status   LinkStatus
        fromNode string
        toNode   string
    }
    tests := []struct {
        name   string
        fields fields
        want   string
    }{
        {
            name: "valid",
            fields: fields{
                time:     10,
                status:   UP,
                fromNode: "A",
                toNode:   "B",
            },
            want: "10 UP A B",
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            l := &LinkState{
                time:     tt.fields.time,
                status:   tt.fields.status,
                fromNode: tt.fields.fromNode,
                toNode:   tt.fields.toNode,
            }
            if got := l.String(); got != tt.want {
                t.Errorf("String() = %v, want %v", got, tt.want)
            }
        })
    }
}

func TestLink_isUp(t *testing.T) {
    type fields struct {
        fromNode string
        toNode   string
        states   []LinkState
    }
    type args struct {
        time int
    }
    tests := []struct {
        name   string
        fields fields
        args   args
        want   bool
    }{
        {
            name: "no states",
            fields: fields{
                fromNode: "A",
                toNode:   "B",
                states:   []LinkState{},
            },
            args: args{time: 0},
            want: false,
        },
        {
            name: "is up inclusive",
            fields: fields{
                fromNode: "A",
                toNode:   "B",
                states: []LinkState{
                    {
                        time:     1,
                        status:   UP,
                        fromNode: "A",
                        toNode:   "B",
                    },
                },
            },
            args: args{time: 1},
            want: true,
        },
        {
            name: "up then down",
            fields: fields{
                fromNode: "A",
                toNode:   "B",
                states: []LinkState{
                    {
                        time:     1,
                        status:   UP,
                        fromNode: "A",
                        toNode:   "B",
                    },
                    {
                        time:     3,
                        status:   DOWN,
                        fromNode: "A",
                        toNode:   "B",
                    },
                },
            },
            args: args{time: 4},
            want: false,
        },
        {
            name: "down then up",
            fields: fields{
                fromNode: "A",
                toNode:   "B",
                states: []LinkState{
                    {
                        time:     1,
                        status:   DOWN,
                        fromNode: "A",
                        toNode:   "B",
                    },
                    {
                        time:     3,
                        status:   UP,
                        fromNode: "A",
                        toNode:   "B",
                    },
                },
            },
            args: args{time: 4},
            want: true,
        },
        {
            name: "between states",
            fields: fields{
                fromNode: "A",
                toNode:   "B",
                states: []LinkState{
                    {
                        time:     1,
                        status:   DOWN,
                        fromNode: "A",
                        toNode:   "B",
                    },
                    {
                        time:     3,
                        status:   UP,
                        fromNode: "A",
                        toNode:   "B",
                    },
                },
            },
            args: args{time: 2},
            want: false,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            l := &Link{
                fromNode: tt.fields.fromNode,
                toNode:   tt.fields.toNode,
                states:   tt.fields.states,
            }
            if got := l.isUp(tt.args.time); got != tt.want {
                t.Errorf("isUp() = %v, want %v", got, tt.want)
            }
        })
    }
}

func TestNetworkTypology_Query(t *testing.T) {
    type fields struct {
        links map[string]map[string]Link
    }
    type args struct {
        msg QueryMsg
    }
    tests := []struct {
        name   string
        fields fields
        args   args
        want   bool
    }{
        {
            name:   "is up",
            fields: fields{links: goodTopology().links},
            args: args{msg: QueryMsg{
                fromNodeLabel: "X",
                toNodeLabel:   "Y",
                timeQuantum:   10,
            }},
            want: true,
        },
        {
            name:   "is down",
            fields: fields{links: goodTopology().links},
            args: args{msg: QueryMsg{
                fromNodeLabel: "X",
                toNodeLabel:   "Y",
                timeQuantum:   20,
            }},
            want: false,
        },
        {
            name:   "is up end",
            fields: fields{links: goodTopology().links},
            args: args{msg: QueryMsg{
                fromNodeLabel: "Z",
                toNodeLabel:   "X",
                timeQuantum:   25,
            }},
            want: true,
        },
        {
            name:   "label not in topology",
            fields: fields{links: goodTopology().links},
            args: args{msg: QueryMsg{
                fromNodeLabel: "Y",
                toNodeLabel:   "X",
                timeQuantum:   0,
            }},
            want: false,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            n := &NetworkTypology{
                links: tt.fields.links,
            }
            if got := n.Query(tt.args.msg); got != tt.want {
                t.Errorf("Query() = %v, want %v", got, tt.want)
            }
        })
    }
}

func getTestData(p string) io.ReadCloser {
    f, err := os.Open(p)
    if err != nil {
        panic(err)
    }
    return f
}

func goodTopologyReadyCloser() io.ReadCloser {
    return getTestData("./testdata/good_topology.txt")
}

func badTopologyReadCloser() io.ReadCloser {
    return getTestData("./testdata/topology_bad_order.txt")
}

func goodTopology() *NetworkTypology {
    t, err := NewNetworkTypology(goodTopologyReadyCloser())
    if err != nil {
        panic(err)
    }
    return t
}

func TestNewNetworkTypology(t *testing.T) {
    type args struct {
        in io.ReadCloser
    }
    tests := []struct {
        name    string
        args    args
        want    *NetworkTypology
        wantErr bool
    }{
        {
            name: "good topology",
            args: args{in: goodTopologyReadyCloser()},
            want: &NetworkTypology{
                links: map[string]map[string]Link{
                    "X": {
                        "Y": {
                            fromNode: "X",
                            toNode:   "Y",
                            states: []LinkState{
                                {
                                    time:     10,
                                    status:   UP,
                                    fromNode: "X",
                                    toNode:   "Y",
                                },
                                {
                                    time:     20,
                                    status:   DOWN,
                                    fromNode: "X",
                                    toNode:   "Y",
                                },
                            },
                        },
                        "Z": {
                            fromNode: "X",
                            toNode:   "Z",
                            states: []LinkState{
                                {
                                    time:     21,
                                    status:   UP,
                                    fromNode: "X",
                                    toNode:   "Z",
                                },
                            },
                        },
                    },
                    "Y": {
                        "X": {
                            fromNode: "Y",
                            toNode:   "X",
                            states: []LinkState{
                                {
                                    time:     10,
                                    status:   UP,
                                    fromNode: "Y",
                                    toNode:   "X",
                                },
                                {
                                    time:     20,
                                    status:   DOWN,
                                    fromNode: "Y",
                                    toNode:   "X",
                                },
                            },
                        },
                    },
                    "Z": {
                        "X": {
                            fromNode: "Z",
                            toNode:   "X",
                            states: []LinkState{
                                {
                                    time:     25,
                                    status:   UP,
                                    fromNode: "Z",
                                    toNode:   "X",
                                },
                            },
                        },
                    },
                },
            },
            wantErr: false,
        },
        {
            name:    "bad topology",
            args:    args{in: badTopologyReadCloser()},
            want:    nil,
            wantErr: true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := NewNetworkTypology(tt.args.in)
            if (err != nil) != tt.wantErr {
                t.Errorf("NewNetworkTypology() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("NewNetworkTypology() got = %v, want %v", got, tt.want)
            }
        })
    }
}

func Test_parseLinkState(t *testing.T) {
    type args struct {
        state string
    }
    tests := []struct {
        name    string
        args    args
        want    *LinkState
        wantErr bool
    }{
        {
            name: "valid",
            args: args{state: "10 UP X Y"},
            want: &LinkState{
                time:     10,
                status:   UP,
                fromNode: "X",
                toNode:   "Y",
            },
            wantErr: false,
        },
        {
            name:    "invalid syntax",
            args:    args{state: "10UP X Y"},
            want:    nil,
            wantErr: true,
        },
        {
            name:    "invalid time",
            args:    args{state: "x UP X Y"},
            want:    nil,
            wantErr: true,
        },
        {
            name:    "no negative time",
            args:    args{state: "-1 UP X Y"},
            want:    nil,
            wantErr: true,
        },
        {
            name:    "invalid status",
            args:    args{state: "1 x X Y"},
            want:    nil,
            wantErr: true,
        },
        {
            name:    "invalid label",
            args:    args{state: "1 UP Xa Y"},
            want:    nil,
            wantErr: true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := parseLinkState(tt.args.state)
            if (err != nil) != tt.wantErr {
                t.Errorf("parseLinkState() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("parseLinkState() got = %v, want %v", got, tt.want)
            }
        })
    }
}
