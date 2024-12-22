package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

type Flake struct {
	Deps        map[string][]string
	ReverseDeps map[string][]string
}

type Options struct {
	LockPath               string
	Verbose                bool
	FailIfMultipleVersions bool
}

// flakeURL constructs a dependency URL from a node definition
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

// analyzeFlake processes the flake.lock data to extract dependencies and their reverse dependencies
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

// parseArgs parses command-line arguments into an Options struct
func parseArgs() Options {
	var lockPath string
	var verbose bool
	var failIfMultipleVersions bool

	flag.StringVar(&lockPath, "flake_lock", "flake.lock", "Path to flake.lock")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&failIfMultipleVersions, "fail-if-multiple-versions", false, "Exit with error if multiple versions found")
	flag.Parse()

	return Options{
		LockPath:               lockPath,
		Verbose:                verbose,
		FailIfMultipleVersions: failIfMultipleVersions,
	}
}

// printDependencies outputs dependencies with multiple versions and their reverse dependencies
func printDependencies(deps map[string][]string, reverseDeps map[string][]string, verbose bool) {
	hasMultipleVersions := false

	for url, aliases := range deps {
		if len(aliases) == 1 {
			continue // skip single-version dependencies
		}

		// Print repository information
		fmt.Printf("%s\n", url)
		fmt.Println(strings.Repeat("=", len(url)))

		// List aliases and their reverse dependencies
		hasMultipleVersions = true

		for _, alias := range aliases {
			fmt.Printf("%s:\n", alias)
			for _, dep := range reverseDeps[alias] {
				fmt.Printf("    - %s\n", dep)
			}

			if verbose {
				fmt.Printf("Verbose info: %s has %d dependencies\n", alias, len(reverseDeps[alias]))
			}
		}
	}

	if hasMultipleVersions {
		fmt.Println("Multiple versions found across repositories.")
	} else if verbose {
		fmt.Println("No multiple versions detected.")
	}
}

func main() {
	options := parseArgs()

	// Read the flake.lock file
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
	printDependencies(flake.Deps, flake.ReverseDeps, options.Verbose)

	// Exit with an error if multiple versions were found and the flag is set
	if options.FailIfMultipleVersions && len(flake.Deps) > 0 {
		os.Exit(1)
	}
}
