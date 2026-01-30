package cli

import (
	"fmt"
	"os"

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

			// Output as YAML.
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
			fmt.Println(config.ConfigFilePath())
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
			if _, err := os.Stat(configPath); err == nil {
				return fmt.Errorf("config file already exists: %s",
					configPath)
			}

			// Create default config.
			cfg := config.DefaultConfig()

			// Ensure directory exists.
			if err := config.EnsureDirectories(cfg); err != nil {
				return err
			}

			// Marshal to YAML.
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			// Write config file.
			if err := os.WriteFile(configPath, data, 0600); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			fmt.Printf("Created config file: %s\n", configPath)

			return nil
		},
	}
}
