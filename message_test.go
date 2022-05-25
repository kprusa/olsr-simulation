package main

import "testing"

func TestTCMessage_String(t *testing.T) {
	type fields struct {
		src    NodeID
		frombr NodeID
		seq    int
		ms     []NodeID
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "check format",
			fields: fields{
				src:    0,
				frombr: 10,
				seq:    2,
				ms:     []NodeID{1, 2},
			},
			want: "* 10 TC 0 2 MS 1 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &TCMessage{
				Source:             tt.fields.src,
				FromNeighbor:       tt.fields.frombr,
				Sequence:           tt.fields.seq,
				MultipointRelaySet: tt.fields.ms,
			}
			if got := m.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataMessage_String(t *testing.T) {
	type fields struct {
		src     NodeID
		dst     NodeID
		nxtHop  NodeID
		fromnbr NodeID
		data    string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "check format",
			fields: fields{
				src:     1,
				dst:     4,
				nxtHop:  3,
				fromnbr: 9,
				data:    "hello there",
			},
			want: "3 9 DATA 1 4 hello there",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &DataMessage{
				Source:       tt.fields.src,
				Destination:  tt.fields.dst,
				NextHop:      tt.fields.nxtHop,
				FromNeighbor: tt.fields.fromnbr,
				Data:         tt.fields.data,
			}
			if got := m.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHelloMessage_String(t *testing.T) {
	type fields struct {
		src    NodeID
		unidir []NodeID
		bidir  []NodeID
		mpr    []NodeID
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "check format",
			fields: fields{
				src:    4,
				unidir: []NodeID{1, 2, 3},
				bidir:  []NodeID{5, 6},
				mpr:    []NodeID{7, 8},
			},
			want: "* 4 HELLO UNIDIR 1 2 3 BIDIR 5 6 MultipointRelay 7 8",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &HelloMessage{
				Source:          tt.fields.src,
				Unidirectional:  tt.fields.unidir,
				Bidirectional:   tt.fields.bidir,
				MultipointRelay: tt.fields.mpr,
			}
			if got := m.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
