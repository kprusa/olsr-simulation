package main

import (
	"reflect"
	"testing"
)

func Test_updateOneHopNeighbors(t *testing.T) {
	type args struct {
		msg             *HelloMessage
		oneHopNeighbors map[NodeID]oneHopNeighborEntry
		time            int
		holdTime        int
		id              NodeID
	}
	tests := []struct {
		name string
		args args
		want map[NodeID]oneHopNeighborEntry
	}{
		{
			name: "new unidirectional neighbor",
			args: args{
				msg: &HelloMessage{
					Source:          1,
					Unidirectional:  nil,
					Bidirectional:   []NodeID{2, 3},
					MultipointRelay: nil,
				},
				oneHopNeighbors: map[NodeID]oneHopNeighborEntry{
					NodeID(2): {
						neighborID: 1,
						state:      unidirectional,
						holdUntil:  15,
					},
				},
				time:     10,
				holdTime: 10,
				id:       0,
			},
			want: map[NodeID]oneHopNeighborEntry{
				NodeID(2): {
					neighborID: 1,
					state:      unidirectional,
					holdUntil:  15,
				},
				NodeID(1): {
					neighborID: 1,
					state:      unidirectional,
					holdUntil:  20,
				},
			},
		},
		{
			name: "new bidirectional neighbor",
			args: args{
				msg: &HelloMessage{
					Source:          1,
					Unidirectional:  nil,
					Bidirectional:   []NodeID{0, 2, 3},
					MultipointRelay: nil,
				},
				oneHopNeighbors: map[NodeID]oneHopNeighborEntry{
					NodeID(1): {
						neighborID: 1,
						state:      unidirectional,
						holdUntil:  15,
					},
					NodeID(2): {
						neighborID: 1,
						state:      unidirectional,
						holdUntil:  15,
					},
				},
				time:     10,
				holdTime: 10,
				id:       0,
			},
			want: map[NodeID]oneHopNeighborEntry{
				NodeID(1): {
					neighborID: 1,
					state:      bidirectional,
					holdUntil:  20,
				},
				NodeID(2): {
					neighborID: 1,
					state:      unidirectional,
					holdUntil:  15,
				},
			},
		},
		{
			name: "new bidirectional neighbor from MultipointRelay",
			args: args{
				msg: &HelloMessage{
					Source:          1,
					Unidirectional:  nil,
					Bidirectional:   nil,
					MultipointRelay: []NodeID{0},
				},
				oneHopNeighbors: map[NodeID]oneHopNeighborEntry{
					NodeID(1): {
						neighborID: 1,
						state:      unidirectional,
						holdUntil:  15,
					},
				},
				time:     10,
				holdTime: 10,
				id:       0,
			},
			want: map[NodeID]oneHopNeighborEntry{
				NodeID(1): {
					neighborID: 1,
					state:      bidirectional,
					holdUntil:  20,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateOneHopNeighbors(tt.args.msg, tt.args.oneHopNeighbors, tt.args.time+tt.args.holdTime, tt.args.id); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateOneHopNeighbors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateTwoHopNeighbors(t *testing.T) {
	type args struct {
		msg             *HelloMessage
		twoHopNeighbors map[NodeID]map[NodeID]NodeID
		id              NodeID
	}
	tests := []struct {
		name string
		args args
		want map[NodeID]map[NodeID]NodeID
	}{
		{
			name: "new two hop",
			args: args{
				msg: &HelloMessage{
					Source:          1,
					Unidirectional:  nil,
					Bidirectional:   []NodeID{2},
					MultipointRelay: nil,
				},
				twoHopNeighbors: map[NodeID]map[NodeID]NodeID{},
				id:              0,
			},
			want: map[NodeID]map[NodeID]NodeID{
				NodeID(1): {
					NodeID(2): NodeID(2),
				},
			},
		},
		{
			name: "include mprs",
			args: args{
				msg: &HelloMessage{
					Source:          1,
					Unidirectional:  nil,
					Bidirectional:   []NodeID{2},
					MultipointRelay: []NodeID{3},
				},
				twoHopNeighbors: map[NodeID]map[NodeID]NodeID{},
				id:              0,
			},
			want: map[NodeID]map[NodeID]NodeID{
				NodeID(1): {
					NodeID(2): NodeID(2),
					NodeID(3): NodeID(3),
				},
			},
		},

		{
			name: "delete previous entries",
			args: args{
				msg: &HelloMessage{
					Source:          1,
					Unidirectional:  nil,
					Bidirectional:   []NodeID{3},
					MultipointRelay: nil,
				},
				twoHopNeighbors: map[NodeID]map[NodeID]NodeID{
					NodeID(1): {
						NodeID(2): NodeID(2),
					},
				},
				id: 0,
			},
			want: map[NodeID]map[NodeID]NodeID{
				NodeID(1): {
					NodeID(3): NodeID(3),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateTwoHopNeighbors(tt.args.msg, tt.args.twoHopNeighbors, tt.args.id); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateTwoHopNeighbors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_calculateMPRs(t *testing.T) {
	type args struct {
		oneHopNeighbors map[NodeID]oneHopNeighborEntry
		twoHopNeighbors map[NodeID]map[NodeID]NodeID
	}
	tests := []struct {
		name string
		args args
		want map[NodeID]oneHopNeighborEntry
	}{
		{
			name: "ensure greedy",
			args: struct {
				oneHopNeighbors map[NodeID]oneHopNeighborEntry
				twoHopNeighbors map[NodeID]map[NodeID]NodeID
			}{
				oneHopNeighbors: map[NodeID]oneHopNeighborEntry{
					NodeID(1): {
						neighborID: 1,
						state:      bidirectional,
						holdUntil:  20,
					},
					NodeID(2): {
						neighborID: 1,
						state:      bidirectional,
						holdUntil:  20,
					},
				},
				twoHopNeighbors: map[NodeID]map[NodeID]NodeID{
					NodeID(1): {
						NodeID(3): NodeID(3),
						NodeID(4): NodeID(4),
					},
					NodeID(2): {
						NodeID(3): NodeID(3),
					},
				},
			},
			want: map[NodeID]oneHopNeighborEntry{
				NodeID(1): {
					neighborID: 1,
					state:      mpr,
					holdUntil:  20,
				},
				NodeID(2): {
					neighborID: 1,
					state:      bidirectional,
					holdUntil:  20,
				},
			},
		},
		{
			name: "ensure coverage",
			args: struct {
				oneHopNeighbors map[NodeID]oneHopNeighborEntry
				twoHopNeighbors map[NodeID]map[NodeID]NodeID
			}{
				oneHopNeighbors: map[NodeID]oneHopNeighborEntry{
					NodeID(1): {
						neighborID: 1,
						state:      bidirectional,
						holdUntil:  20,
					},
					NodeID(2): {
						neighborID: 1,
						state:      bidirectional,
						holdUntil:  20,
					},
				},
				twoHopNeighbors: map[NodeID]map[NodeID]NodeID{
					NodeID(1): {
						NodeID(3): NodeID(3),
					},
					NodeID(2): {
						NodeID(4): NodeID(4),
					},
				},
			},
			want: map[NodeID]oneHopNeighborEntry{
				NodeID(1): {
					neighborID: 1,
					state:      mpr,
					holdUntil:  20,
				},
				NodeID(2): {
					neighborID: 1,
					state:      mpr,
					holdUntil:  20,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateMPRs(tt.args.oneHopNeighbors, tt.args.twoHopNeighbors); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("calculateMPRs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateTopologyTable1(t *testing.T) {
	type args struct {
		msg           *TCMessage
		topologyTable map[NodeID]map[NodeID]topologyEntry
		holdTime      int
		id            NodeID
	}
	tests := []struct {
		name string
		args args
		want map[NodeID]map[NodeID]topologyEntry
	}{
		{
			name: "new nodes",
			args: args{
				msg: &TCMessage{
					Source:       2,
					FromNeighbor: 1,
					Sequence:     0,
					MultipointRelaySet: []NodeID{
						NodeID(1),
						NodeID(3),
					},
				},
				topologyTable: map[NodeID]map[NodeID]topologyEntry{},
				holdTime:      30,
			},
			want: map[NodeID]map[NodeID]topologyEntry{
				NodeID(2): {
					NodeID(1): topologyEntry{
						dst:        1,
						originator: 2,
						holdUntil:  30,
						seq:        0,
					},
					NodeID(3): topologyEntry{
						dst:        3,
						originator: 2,
						holdUntil:  30,
						seq:        0,
					},
				},
			},
		},
		{
			name: "multiple mprs",
			args: args{
				msg: &TCMessage{
					Source:       1,
					FromNeighbor: 1,
					Sequence:     0,
					MultipointRelaySet: []NodeID{
						NodeID(2),
					},
				},
				topologyTable: map[NodeID]map[NodeID]topologyEntry{
					NodeID(3): {
						NodeID(2): topologyEntry{
							dst:        2,
							originator: 3,
							holdUntil:  30,
							seq:        0,
						},
					},
				},
				holdTime: 30,
			},
			want: map[NodeID]map[NodeID]topologyEntry{
				NodeID(3): {
					NodeID(2): topologyEntry{
						dst:        2,
						originator: 3,
						holdUntil:  30,
						seq:        0,
					},
				},
				NodeID(1): {
					NodeID(2): topologyEntry{
						dst:        2,
						originator: 1,
						holdUntil:  30,
						seq:        0,
					},
				},
			},
		},
		{
			name: "ignore Destination if same as ID",
			args: args{
				msg: &TCMessage{
					Source:       1,
					FromNeighbor: 1,
					Sequence:     0,
					MultipointRelaySet: []NodeID{
						NodeID(2),
						NodeID(0),
					},
				},
				topologyTable: map[NodeID]map[NodeID]topologyEntry{},
				holdTime:      30,
				id:            NodeID(0),
			},
			want: map[NodeID]map[NodeID]topologyEntry{
				NodeID(1): {
					NodeID(2): topologyEntry{
						dst:        2,
						originator: 1,
						holdUntil:  30,
						seq:        0,
					},
				},
			},
		},
		{
			name: "update if larger sequence",
			args: args{
				msg: &TCMessage{
					Source:       1,
					FromNeighbor: 1,
					Sequence:     1,
					MultipointRelaySet: []NodeID{
						NodeID(2),
						NodeID(3),
					},
				},
				topologyTable: map[NodeID]map[NodeID]topologyEntry{
					NodeID(1): {
						NodeID(2): topologyEntry{
							dst:        2,
							originator: 1,
							holdUntil:  23,
							seq:        0,
						},
					},
				},
				holdTime: 30,
				id:       NodeID(0),
			},
			want: map[NodeID]map[NodeID]topologyEntry{
				NodeID(1): {
					NodeID(2): topologyEntry{
						dst:        2,
						originator: 1,
						holdUntil:  30,
						seq:        1,
					},
					NodeID(3): topologyEntry{
						dst:        3,
						originator: 1,
						holdUntil:  30,
						seq:        1,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateTopologyTable(tt.args.msg, tt.args.topologyTable, tt.args.holdTime, tt.args.id); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateTopologyTable() = %v, want %v", got, tt.want)
			}
		})
	}
}
