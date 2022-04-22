package main

import (
	"reflect"
	"testing"
)

func TestLinkState_String(t *testing.T) {
	type fields struct {
		time     int
		status   LinkStatus
		fromNode NodeID
		toNode   NodeID
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
				fromNode: 0,
				toNode:   1,
			},
			want: "10 UP 0 1",
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
		fromNode NodeID
		toNode   NodeID
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
				fromNode: 0,
				toNode:   1,
				states:   []LinkState{},
			},
			args: args{time: 0},
			want: false,
		},
		{
			name: "is up inclusive",
			fields: fields{
				fromNode: 0,
				toNode:   1,
				states: []LinkState{
					{
						time:     1,
						status:   UP,
						fromNode: 0,
						toNode:   1,
					},
				},
			},
			args: args{time: 1},
			want: true,
		},
		{
			name: "up then down",
			fields: fields{
				fromNode: 0,
				toNode:   1,
				states: []LinkState{
					{
						time:     1,
						status:   UP,
						fromNode: 0,
						toNode:   1,
					},
					{
						time:     3,
						status:   DOWN,
						fromNode: 0,
						toNode:   1,
					},
				},
			},
			args: args{time: 4},
			want: false,
		},
		{
			name: "down then up",
			fields: fields{
				fromNode: 0,
				toNode:   1,
				states: []LinkState{
					{
						time:     1,
						status:   DOWN,
						fromNode: 0,
						toNode:   1,
					},
					{
						time:     3,
						status:   UP,
						fromNode: 0,
						toNode:   1,
					},
				},
			},
			args: args{time: 4},
			want: true,
		},
		{
			name: "between states",
			fields: fields{
				fromNode: 0,
				toNode:   1,
				states: []LinkState{
					{
						time:     1,
						status:   DOWN,
						fromNode: 0,
						toNode:   1,
					},
					{
						time:     3,
						status:   UP,
						fromNode: 0,
						toNode:   1,
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
			args: args{state: "10 UP 0 1"},
			want: &LinkState{
				time:     10,
				status:   UP,
				fromNode: 0,
				toNode:   1,
			},
			wantErr: false,
		},
		{
			name:    "invalid syntax",
			args:    args{state: "10UP 0 1"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid time",
			args:    args{state: "x UP 0 1"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "no negative time",
			args:    args{state: "-1 UP 0 1"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid status",
			args:    args{state: "1 x 0 1"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid id",
			args:    args{state: "1 UP X 1"},
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
