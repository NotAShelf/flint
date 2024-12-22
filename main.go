package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Flake struct {
	Deps        map[string][]string
	ReverseDeps map[string][]string
}

type Options struct {
	LockPath               string
	Verbose                bool
	FailIfMultipleVersions bool
	OutputFormat           string
}

func flakeURL(dep map[string]interface{}) string {
	locked, ok := dep["locked"].(map[string]interface{})
	if !ok {
		return ""
	}
	lockedType, ok := locked["type"].(string)
	if !ok {
		return ""
	}
	switch lockedType {
	case "github", "gitlab", "sourcehut":
		url := fmt.Sprintf("%s:%s/%s", lockedType, locked["owner"], locked["repo"])
		if host, ok := locked["host"].(string); ok {
			url += fmt.Sprintf("?host=%s", host)
		}
		return url
	case "git", "hg", "tarball":
		return fmt.Sprintf("%s:%s", lockedType, locked["url"])
	case "path":
		return fmt.Sprintf("%s:%s", lockedType, locked["path"])
	}
	return ""
}

func analyzeFlake(flakeLock map[string]interface{}) Flake {
	deps := make(map[string][]string)
	reverseDeps := make(map[string][]string)

	nodes, _ := flakeLock["nodes"].(map[string]interface{})
	for name, depInterface := range nodes {
		dep, _ := depInterface.(map[string]interface{})
		if inputs, ok := dep["inputs"].(map[string]interface{}); ok {
			for _, input := range inputs {
				switch v := input.(type) {
				case string:
					reverseDeps[v] = append(reverseDeps[v], name)
				case []interface{}:
					for _, i := range v {
						if str, ok := i.(string); ok {
							reverseDeps[str] = append(reverseDeps[str], name)
						}
					}
				}
			}
		}
		url := flakeURL(dep)
		if url != "" {
			deps[url] = append(deps[url], name)
		}
	}
	return Flake{Deps: deps, ReverseDeps: reverseDeps}
}

func parseArgs() Options {
	var lockPath string
	var verbose bool
	var failIfMultipleVersions bool
	var outputFormat string

	flag.StringVar(&lockPath, "lockfile", "flake.lock", "path to flake.lock")
	flag.BoolVar(&verbose, "verbose", false, "enable verbose output")
	flag.BoolVar(&failIfMultipleVersions, "fail-if-multiple-versions", false, "exit with error if multiple versions found")
	flag.StringVar(&outputFormat, "output", "plain", "output format: plain or json")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --lockfile=/path/to/flake.lock --verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --lockfile=/path/to/flake.lock --output=json\n", os.Args[0])
	}

	flag.Parse()

	return Options{
		LockPath:               lockPath,
		Verbose:                verbose,
		FailIfMultipleVersions: failIfMultipleVersions,
		OutputFormat:           outputFormat,
	}
}

func printDependencies(deps map[string][]string, reverseDeps map[string][]string, options Options) {
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

		fmt.Println(inputStyle.Render(fmt.Sprintf("- Input: %s", url)))
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
	options := parseArgs()

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

	flake := analyzeFlake(flakeLock)

	// Print the dependencies
	printDependencies(flake.Deps, flake.ReverseDeps, options)

	// Exit with an error if multiple versions were found and the flag is set
	if options.FailIfMultipleVersions && len(flake.Deps) > 0 {
		os.Exit(1)
	}
}
