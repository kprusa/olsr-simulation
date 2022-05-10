package main

import "testing"

func TestTCMessage_String(t *testing.T) {
	type fields struct {
		type_  MsgType
		src    NodeID
		frombr NodeID
		seq    uint
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
				src:     tt.fields.src,
				fromnbr: tt.fields.frombr,
				seq:     tt.fields.seq,
				ms:      tt.fields.ms,
			}
			if got := m.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataMessage_String(t *testing.T) {
	type fields struct {
		type_   MsgType
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
				type_:   DataType,
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
				src:     tt.fields.src,
				dst:     tt.fields.dst,
				nxtHop:  tt.fields.nxtHop,
				fromnbr: tt.fields.fromnbr,
				data:    tt.fields.data,
			}
			if got := m.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHelloMessage_String(t *testing.T) {
	type fields struct {
		type_  MsgType
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
				type_:  HelloType,
				src:    4,
				unidir: []NodeID{1, 2, 3},
				bidir:  []NodeID{5, 6},
				mpr:    []NodeID{7, 8},
			},
			want: "* 4 HELLO UNIDIR 1 2 3 BIDIR 5 6 MPR 7 8",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &HelloMessage{
				src:    tt.fields.src,
				unidir: tt.fields.unidir,
				bidir:  tt.fields.bidir,
				mpr:    tt.fields.mpr,
			}
			if got := m.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
