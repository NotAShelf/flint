package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	args "notashelf.dev/flint/internal/args"
	flake "notashelf.dev/flint/internal/flake"
)

func printDependencies(deps map[string][]string, reverseDeps map[string][]string, options args.Options) {
	if options.OutputFormat == "json" {
		output := map[string]interface{}{
			"dependencies":         deps,
			"reverse_dependencies": reverseDeps,
		}
		jsonData, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(jsonData))
		return
	}

	// Titles, bold and underlined.
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("5")).
		Bold(true).
		Underline(true)

	// Input name
	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true)

	// Aliases to an input
	aliasStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("4")).
		Italic(true)

	// Inputs that depend on a given input
	depStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("2"))

	// Inputs that depend on an input
	summaryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")).
		Bold(true)

	hasMultipleVersions := false
	fmt.Println(titleStyle.Render("Dependency Analysis Report"))

	for url, aliases := range deps {
		if len(aliases) == 1 {
			continue // skip single-version dependencies
		}
		fmt.Println(inputStyle.Render(fmt.Sprintf("Input: %s", url)))
		for _, alias := range aliases {
			fmt.Println(aliasStyle.Render(fmt.Sprintf("  Alias: %s", alias)))
			fmt.Print(depStyle.Render("    Dependants: "))

			dependants := reverseDeps[alias]
			if len(dependants) > 0 {
				fmt.Print(strings.Join(dependants, ", "))
			}

			if options.Verbose {
				fmt.Println(depStyle.Render(fmt.Sprintf("    [Debug] %d inputs depend on %s", len(dependants), alias)))
			}
			fmt.Println()
		}

		hasMultipleVersions = true
	}

	if hasMultipleVersions {
		fmt.Println(summaryStyle.Render("Duplicate inputs detected. Please review above output."))
	} else {
		fmt.Println(summaryStyle.Render("No duplicate inputs detected in the repositories analyzed."))
	}
}

func main() {
	options := args.ParseArgs()

	data, err := os.ReadFile(options.LockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading flake.lock: %v\n", err)
		os.Exit(1)
	}

	var flakeLock map[string]interface{}
	if err := json.Unmarshal(data, &flakeLock); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding flake.lock: %v\n", err)
		os.Exit(1)
	}

	flake := flake.AnalyzeFlake(flakeLock)

	// Print the dependencies
	printDependencies(flake.Deps, flake.ReverseDeps, options)

	// Exit with an error if multiple versions were found and the flag is set
	if options.FailIfMultipleVersions && len(flake.Deps) > 0 {
		os.Exit(1)
	}
}
