package cycle

import (
	"reflect"
	"testing"
)

func TestFindCycles(t *testing.T) {
	tests := []struct {
		name  string
		graph Graph
		want  [][]string
	}{
		{
			name: "DAG (no cycles)",
			graph: Graph{
				Nodes: []string{"A", "B", "C"},
				Edges: map[string][]string{
					"A": {"B"},
					"B": {"C"},
				},
			},
			want: nil,
		},
		{
			name: "simple 2-node cycle",
			graph: Graph{
				Nodes: []string{"A", "B", "C"},
				Edges: map[string][]string{
					"A": {"B"},
					"B": {"A", "C"},
				},
			},
			want: [][]string{
				{"A", "B"},
			},
		},
		{
			name: "self import",
			graph: Graph{
				Nodes: []string{"A", "B"},
				Edges: map[string][]string{
					"A": {"A", "B"},
				},
			},
			want: [][]string{
				{"A"},
			},
		},
		{
			name: "multiple cycles",
			graph: Graph{
				Nodes: []string{"A", "B", "C", "D"},
				Edges: map[string][]string{
					"A": {"B"},
					"B": {"A", "C"},
					"C": {"D"},
					"D": {"C"},
				},
			},
			want: [][]string{
				{"A", "B"},
				{"C", "D"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindCycles(tt.graph)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("FindCycles() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
