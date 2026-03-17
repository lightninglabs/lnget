package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/lightninglabs/lnget/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewConfigCmd creates the config subcommand.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage lnget configuration",
		Long:  "View and manage lnget configuration settings.",
	}

	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigPathCmd())
	cmd.AddCommand(newConfigInitCmd())
	cmd.AddCommand(newConfigSetCmd())

	return cmd
}

// newConfigShowCmd creates the config show subcommand.
func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Long:  "Display the current lnget configuration, including defaults.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// JSON mode emits the config as structured JSON.
			if isJSONOutput(cmd) {
				return writeJSON(cmd.OutOrStdout(), cfg)
			}

			// Human mode uses YAML for readability.
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			fmt.Print(string(data))

			return nil
		},
	}
}

// newConfigPathCmd creates the config path subcommand.
func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show config file path",
		Long:  "Display the path to the configuration file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ConfigFilePath()

			if isJSONOutput(cmd) {
				return writeJSON(cmd.OutOrStdout(), map[string]string{
					"path": path,
				})
			}

			fmt.Println(path)

			return nil
		},
	}
}

// newConfigInitCmd creates the config init subcommand.
func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration file",
		Long: `Create a default configuration file if one doesn't exist.
This will create ~/.lnget/config.yaml with default settings.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := config.ConfigFilePath()

			// Check if config already exists.
			_, err := os.Stat(configPath)
			if err == nil {
				return fmt.Errorf("config file already exists: %s",
					configPath)
			}

			// Create default config.
			cfg := config.DefaultConfig()

			// Ensure directory exists.
			err = config.EnsureDirectories(cfg)
			if err != nil {
				return err
			}

			// Marshal to YAML.
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			// Write config file.
			err = os.WriteFile(configPath, data, 0600)
			if err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			if isJSONOutput(cmd) {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"created": true,
					"path":    configPath,
				})
			}

			fmt.Printf("Created config file: %s\n", configPath)

			return nil
		},
	}
}

// newConfigSetCmd creates the config set subcommand for bulk or
// key-value config mutation.
func newConfigSetCmd() *cobra.Command {
	var jsonInput string

	cmd := &cobra.Command{
		Use:   "set [key value]",
		Short: "Update configuration values",
		Long: `Update configuration values via JSON or key=value pairs.

  lnget config set --from-json '{"l402": {"max_cost_sats": 5000}}'
  lnget config set l402.max_cost_sats 5000`,
		Args: cobra.RangeArgs(0, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := config.ConfigFilePath()

			// Load existing config (or defaults if no file).
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if jsonInput != "" {
				// Deep-merge JSON into config via marshal
				// round-trip.
				cfgJSON, err := json.Marshal(cfg)
				if err != nil {
					return fmt.Errorf("failed to marshal config: %w",
						err)
				}

				var cfgMap map[string]any
				if err := json.Unmarshal(cfgJSON, &cfgMap); err != nil {
					return fmt.Errorf("failed to unmarshal config: %w",
						err)
				}

				var overlay map[string]any
				if err := json.Unmarshal([]byte(jsonInput), &overlay); err != nil {
					return ErrInvalidArgsf(
						"invalid JSON: %v", err,
					)
				}

				deepMerge(cfgMap, overlay)

				// Marshal merged map back to config struct
				// via JSON round-trip.
				merged, err := json.Marshal(cfgMap)
				if err != nil {
					return fmt.Errorf("failed to marshal merged config: %w",
						err)
				}

				if err := json.Unmarshal(merged, cfg); err != nil {
					return fmt.Errorf("failed to apply merged config: %w",
						err)
				}
			} else if len(args) == 2 {
				// Single key=value update via YAML overlay.
				// Convert dot-path to nested YAML (e.g.
				// "l402.max_cost_sats" -> "l402:\n  max_cost_sats:").
				yamlStr := dotPathToYAML(args[0], args[1])

				if err := yaml.Unmarshal([]byte(yamlStr), cfg); err != nil {
					return ErrInvalidArgsf(
						"invalid key/value: %v", err,
					)
				}
			} else {
				return ErrInvalidArgsf(
					"provide --from-json or key value pair",
				)
			}

			// Validate the resulting config.
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config after update: %w", err)
			}

			// Write back as YAML.
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			if err := os.WriteFile(configPath, data, 0600); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			if isJSONOutput(cmd) {
				return writeJSON(cmd.OutOrStdout(), cfg)
			}

			fmt.Printf("Configuration updated: %s\n", configPath)

			return nil
		},
	}

	cmd.Flags().StringVar(&jsonInput, "from-json", "",
		"JSON object to deep-merge into configuration")

	return cmd
}

// deepMerge recursively merges src into dst. For nested maps, values
// are merged recursively. For all other types, src overwrites dst.
func deepMerge(dst, src map[string]any) {
	for key, srcVal := range src {
		dstVal, exists := dst[key]
		if !exists {
			dst[key] = srcVal

			continue
		}

		// If both are maps, recurse.
		dstMap, dstOk := dstVal.(map[string]any)
		srcMap, srcOk := srcVal.(map[string]any)

		if dstOk && srcOk {
			deepMerge(dstMap, srcMap)

			continue
		}

		// Otherwise overwrite.
		dst[key] = srcVal
	}
}

// dotPathToYAML converts a dot-separated key path and value into a
// nested YAML string. For example, "l402.max_cost_sats" and "5000"
// becomes "l402:\n  max_cost_sats: 5000\n".
func dotPathToYAML(path, value string) string {
	parts := strings.Split(path, ".")

	var b strings.Builder
	indent := ""

	for i, part := range parts {
		if i == len(parts)-1 {
			fmt.Fprintf(&b, "%s%s: %q\n", indent, part, value)
		} else {
			fmt.Fprintf(&b, "%s%s:\n", indent, part)
			indent += "  "
		}
	}

	return b.String()
}
