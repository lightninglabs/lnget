package cli

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestDotPathToYAML verifies that dotPathToYAML correctly converts
// dot-separated paths and values into nested YAML.
func TestDotPathToYAML(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		value string
		want  string
	}{
		{
			name:  "single key",
			path:  "timeout",
			value: "30s",
			want:  "timeout: \"30s\"\n",
		},
		{
			name:  "nested two levels",
			path:  "l402.max_cost_sats",
			value: "5000",
			want:  "l402:\n  max_cost_sats: \"5000\"\n",
		},
		{
			name:  "nested three levels",
			path:  "ln.lnd.host",
			value: "localhost:10009",
			want: "ln:\n  lnd:\n    host: " +
				"\"localhost:10009\"\n",
		},
		{
			name:  "value with YAML special chars is quoted",
			path:  "key",
			value: ": true",
			want:  "key: \": true\"\n",
		},
		{
			name:  "value with quotes is escaped",
			path:  "key",
			value: `he said "hello"`,
			want:  "key: \"he said \\\"hello\\\"\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dotPathToYAML(tt.path, tt.value)
			if got != tt.want {
				t.Errorf("dotPathToYAML(%q, %q)\ngot:  %q\nwant: %q",
					tt.path, tt.value, got, tt.want)
			}

			// Verify it's valid YAML.
			var m map[string]any
			err := yaml.Unmarshal([]byte(got), &m)
			if err != nil {
				t.Errorf("produced invalid YAML: %v", err)
			}
		})
	}
}

// TestDeepMerge verifies that deepMerge correctly combines nested
// maps with src overriding dst for non-map values.
func TestDeepMerge(t *testing.T) {
	tests := []struct {
		name string
		dst  map[string]any
		src  map[string]any
		want map[string]any
	}{
		{
			name: "simple override",
			dst:  map[string]any{"a": 1, "b": 2},
			src:  map[string]any{"b": 3},
			want: map[string]any{"a": 1, "b": 3},
		},
		{
			name: "add new key",
			dst:  map[string]any{"a": 1},
			src:  map[string]any{"b": 2},
			want: map[string]any{"a": 1, "b": 2},
		},
		{
			name: "nested merge preserves unset keys",
			dst: map[string]any{
				"l402": map[string]any{
					"max_cost": float64(1000),
					"max_fee":  float64(10),
				},
			},
			src: map[string]any{
				"l402": map[string]any{
					"max_cost": float64(5000),
				},
			},
			want: map[string]any{
				"l402": map[string]any{
					"max_cost": float64(5000),
					"max_fee":  float64(10),
				},
			},
		},
		{
			name: "src overwrites non-map with map",
			dst:  map[string]any{"a": "string"},
			src:  map[string]any{"a": map[string]any{"b": 1}},
			want: map[string]any{"a": map[string]any{"b": 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deepMerge(tt.dst, tt.src)

			got, _ := json.Marshal(tt.dst)
			want, _ := json.Marshal(tt.want)

			if string(got) != string(want) {
				t.Errorf("deepMerge:\ngot:  %s\nwant: %s",
					got, want)
			}
		})
	}
}
