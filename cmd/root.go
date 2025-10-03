package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	flake "notashelf.dev/flint/internal/flake"
	output "notashelf.dev/flint/internal/output"
)

var (
	Version                string
	lockPath               string
	verbose                bool
	failIfMultipleVersions bool
	outputFormat           string
	merge                  bool
)

var rootCmd = &cobra.Command{
	Use:   "flint",
	Short: "Flake Input Linter - analyze flake.lock for duplicate inputs",
	Long: `Flint (flake input linter) is a utility for analyzing a given flake.lock
for duplicate inputs. It helps identify when multiple versions of the same
dependency are present in your Nix flake dependency tree.`,
	Example: `  flint --lockfile=/path/to/flake.lock --verbose
  flint --lockfile=/path/to/flake.lock --output=json
  flint --lockfile=/path/to/flake.lock --output=plain
  flint --merge`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runFlint()
	},
}

func init() {
	rootCmd.Flags().StringVarP(&lockPath, "lockfile", "l", "flake.lock", "path to flake.lock")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.Flags().BoolVar(&failIfMultipleVersions, "fail-if-multiple-versions", false, "exit with error if multiple versions found")
	rootCmd.Flags().StringVarP(&outputFormat, "output", "o", "pretty", "output format: plain, pretty, or json")
	rootCmd.Flags().BoolVarP(&merge, "merge", "m", false, "merge all dependants into one list for each input")

	rootCmd.SetVersionTemplate(`{{printf "%s version %s\n" .Name .Version}}`)
}

func runFlint() error {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return fmt.Errorf("error reading flake.lock: %w", err)
	}

	var flakeLock map[string]any
	if err := json.Unmarshal(data, &flakeLock); err != nil {
		return fmt.Errorf("error decoding flake.lock: %w", err)
	}

	flakeData := flake.AnalyzeFlake(flakeLock)

	options := output.Options{
		OutputFormat:           outputFormat,
		Verbose:                verbose,
		Merge:                  merge,
		FailIfMultipleVersions: failIfMultipleVersions,
	}

	// Print the dependencies
	if err := output.PrintDependencies(flakeData.Deps, flakeData.ReverseDeps, options); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Return an error if multiple versions were found and the flag is set
	if failIfMultipleVersions {
		for _, aliases := range flakeData.Deps {
			if len(aliases) > 1 {
				return fmt.Errorf("multiple versions detected: exiting with error as requested")
			}
		}
	}

	return nil
}

func Execute() {
	if Version != "" {
		rootCmd.Version = Version
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
