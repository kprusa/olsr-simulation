package main

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestReadNodeConfiguration(t *testing.T) {
	type args struct {
		in io.ReadCloser
	}
	tests := []struct {
		name    string
		args    args
		want    []NodeConfig
		wantErr bool
	}{
		{
			name: "working",
			args: args{in: io.NopCloser(strings.NewReader("0 2 \"(0 -> 2)\" 30\n"))},
			want: []NodeConfig{
				{
					id: 0,
					msg: NodeMsg{
						msg:   "(0 -> 2)",
						delay: 30,
						dst:   2,
						sent:  false,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadNodeConfiguration(tt.args.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadNodeConfiguration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadNodeConfiguration() got = %v, want %v", got, tt.want)
			}
		})
	}
}
