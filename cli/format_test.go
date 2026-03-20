package cli

import (
	"testing"
)

// TestFilterFields verifies that filterFields correctly selects only
// the requested JSON fields from a struct.
func TestFilterFields(t *testing.T) {
	type sample struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email"`
	}

	tests := []struct {
		name       string
		data       any
		fields     []string
		wantKeys   []string
		wantAbsent []string
	}{
		{
			name:     "empty fields returns all",
			data:     sample{Name: "alice", Age: 30, Email: "a@b.c"},
			fields:   nil,
			wantKeys: nil,
		},
		{
			name:       "select single field",
			data:       sample{Name: "alice", Age: 30, Email: "a@b.c"},
			fields:     []string{"name"},
			wantKeys:   []string{"name"},
			wantAbsent: []string{"age", "email"},
		},
		{
			name:       "select multiple fields",
			data:       sample{Name: "alice", Age: 30, Email: "a@b.c"},
			fields:     []string{"name", "age"},
			wantKeys:   []string{"name", "age"},
			wantAbsent: []string{"email"},
		},
		{
			name:       "nonexistent field returns empty map",
			data:       sample{Name: "alice", Age: 30, Email: "a@b.c"},
			fields:     []string{"nonexistent"},
			wantKeys:   nil,
			wantAbsent: []string{"name", "age", "email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := filterFields(tt.data, tt.fields)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// When fields is nil, data is returned as-is.
			if tt.fields == nil {
				return
			}

			m, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("expected map, got %T", result)
			}

			for _, key := range tt.wantKeys {
				if _, exists := m[key]; !exists {
					t.Errorf("expected key %q in result",
						key)
				}
			}

			for _, key := range tt.wantAbsent {
				if _, exists := m[key]; exists {
					t.Errorf("unexpected key %q in result",
						key)
				}
			}
		})
	}
}
