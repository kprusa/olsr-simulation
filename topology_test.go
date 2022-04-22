package main

import (
	"io"
	"os"
	"reflect"
	"testing"
)

func TestNetworkTypology_Query(t *testing.T) {
	type fields struct {
		links map[NodeID]map[NodeID]Link
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
				fromNode:    0,
				toNode:      1,
				timeQuantum: 10,
			}},
			want: true,
		},
		{
			name:   "is down",
			fields: fields{links: goodTopology().links},
			args: args{msg: QueryMsg{
				fromNode:    0,
				toNode:      1,
				timeQuantum: 20,
			}},
			want: false,
		},
		{
			name:   "is up end",
			fields: fields{links: goodTopology().links},
			args: args{msg: QueryMsg{
				fromNode:    2,
				toNode:      0,
				timeQuantum: 25,
			}},
			want: true,
		},
		{
			name:   "id not in topology",
			fields: fields{links: goodTopology().links},
			args: args{msg: QueryMsg{
				fromNode:    1,
				toNode:      0,
				timeQuantum: 0,
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
				links: map[NodeID]map[NodeID]Link{
					0: {
						1: {
							fromNode: 0,
							toNode:   1,
							states: []LinkState{
								{
									time:     10,
									status:   UP,
									fromNode: 0,
									toNode:   1,
								},
								{
									time:     20,
									status:   DOWN,
									fromNode: 0,
									toNode:   1,
								},
							},
						},
						2: {
							fromNode: 0,
							toNode:   2,
							states: []LinkState{
								{
									time:     21,
									status:   UP,
									fromNode: 0,
									toNode:   2,
								},
							},
						},
					},
					1: {
						0: {
							fromNode: 1,
							toNode:   0,
							states: []LinkState{
								{
									time:     10,
									status:   UP,
									fromNode: 1,
									toNode:   0,
								},
								{
									time:     20,
									status:   DOWN,
									fromNode: 1,
									toNode:   0,
								},
							},
						},
					},
					2: {
						0: {
							fromNode: 2,
							toNode:   0,
							states: []LinkState{
								{
									time:     25,
									status:   UP,
									fromNode: 2,
									toNode:   0,
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
