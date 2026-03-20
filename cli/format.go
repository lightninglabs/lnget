package cli

import (
	"encoding/json"
	"io"
	"os"

	"github.com/lightninglabs/lnget/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// resolveOutputFormat determines the output format by checking (in
// priority order): the --json flag, the --human flag, and then TTY
// detection. When stdout is not a TTY, JSON is the default so agents
// piping output always get machine-readable results.
func resolveOutputFormat(cmd *cobra.Command) config.OutputFormat {
	// Explicit flags take highest priority.
	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		return config.OutputFormatJSON
	}

	humanFlag, _ := cmd.Flags().GetBool("human")
	if humanFlag {
		return config.OutputFormatHuman
	}

	// Fall back to TTY detection: non-TTY defaults to JSON.
	if !isTTY() {
		return config.OutputFormatJSON
	}

	return config.OutputFormatHuman
}

// isJSONOutput is a convenience wrapper that returns true when the
// resolved format is JSON. Commands use this for if/else branching
// on output format.
func isJSONOutput(cmd *cobra.Command) bool {
	return resolveOutputFormat(cmd) == config.OutputFormatJSON
}

// isTTY reports whether stdout is connected to an interactive terminal.
func isTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// writeJSON encodes data as indented JSON to the given writer.
func writeJSON(w io.Writer, data any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(data)
}

// filterFields takes an arbitrary struct or map and returns a new map
// containing only the specified JSON field names. If fields is empty,
// the data is returned as-is via a marshal/unmarshal round-trip.
func filterFields(data any, fields []string) (any, error) {
	if len(fields) == 0 {
		return data, nil
	}

	// Marshal to JSON, then unmarshal into a generic map.
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var fullMap map[string]any
	if err := json.Unmarshal(raw, &fullMap); err != nil {
		return nil, err
	}

	// Build a filtered map with only the requested fields.
	keep := make(map[string]bool, len(fields))
	for _, f := range fields {
		keep[f] = true
	}

	filtered := make(map[string]any, len(fields))
	for k, v := range fullMap {
		if keep[k] {
			filtered[k] = v
		}
	}

	return filtered, nil
}
