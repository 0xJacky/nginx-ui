package config

import "testing"

func TestSortByIndex(t *testing.T) {
	configs := func() []Config {
		return []Config{{Name: "b", Index: 24}, {Name: "c", Index: 3}, {Name: "a", Index: 27}, {Name: "d", Index: 11}}
	}
	for _, tc := range []struct {
		order string
		want  []uint64
	}{
		{"desc", []uint64{27, 24, 11, 3}},
		{"asc", []uint64{3, 11, 24, 27}},
	} {
		sorted := Sort("index", tc.order, configs())
		for i, c := range sorted {
			if c.Index != tc.want[i] {
				t.Fatalf("Sort(index, %s) = %v, want %v", tc.order, indexes(sorted), tc.want)
			}
		}
	}
}

func indexes(configs []Config) []uint64 {
	out := make([]uint64, len(configs))
	for i, c := range configs {
		out[i] = c.Index
	}
	return out
}
