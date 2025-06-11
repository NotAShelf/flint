package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	util "notashelf.dev/flint/internal/util"
)

type Options struct {
	OutputFormat           string
	Verbose                bool
	Merge                  bool
	FailIfMultipleVersions bool
}

func PrintDependencies(deps map[string][]string, reverseDeps map[string][]string, options Options) {
	if options.OutputFormat == "json" {
		output := map[string]any{
			"dependencies":         deps,
			"reverse_dependencies": reverseDeps,
		}
		jsonData, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON output: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
		return
	}

	// Choose output format
	switch options.OutputFormat {
	case "plain":
		printPlainOutput(deps, reverseDeps, options)
	case "pretty":
		printFormattedOutput(deps, reverseDeps, options)
	default:
		// Default to pretty for backward compatibility
		printFormattedOutput(deps, reverseDeps, options)
	}
}

func printFormattedOutput(deps map[string][]string, reverseDeps map[string][]string, options Options) {
	// Styles for CI-friendly output
	var (
		headerStyle, successStyle, warningStyle, errorStyle, infoStyle,
		dimStyle, boldStyle, urlStyle, aliasStyle, dependantStyle lipgloss.Style
	)

	// Status symbols
	var successIcon, warningIcon, errorIcon, infoIcon string

	if util.IsNoColor() {
		// Plain text fallbacks for CI environments
		emptyStyle := lipgloss.NewStyle()
		headerStyle = emptyStyle
		successStyle = emptyStyle
		warningStyle = emptyStyle
		errorStyle = emptyStyle
		infoStyle = emptyStyle
		dimStyle = emptyStyle
		boldStyle = emptyStyle
		urlStyle = emptyStyle
		aliasStyle = emptyStyle
		dependantStyle = emptyStyle

		// Unicode-safe symbols for CI
		successIcon = "[✓]"
		warningIcon = "[!]"
		errorIcon = "[✗]"
		infoIcon = "[i]"
	} else {
		// Rich colors and styles for interactive terminals
		headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true).
			Underline(true)

		successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)

		warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)

		errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)

		infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Bold(true)

		dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

		boldStyle = lipgloss.NewStyle().
			Bold(true)

		urlStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Underline(true)

		aliasStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("13")).
			Italic(true)

		dependantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))

		// Rich symbols for interactive terminals
		// Nerdfonts would be preferable, but we use basic symbols for compatibility
		// and to avoid confusing clueless users.
		successIcon = "✓"
		warningIcon = "⚠"
		errorIcon = "✗"
		infoIcon = "ℹ"
	}

	// Count statistics for summary
	totalInputs := len(deps)
	duplicateInputs := 0
	totalDuplicates := 0

	for _, aliases := range deps {
		if len(aliases) > 1 {
			duplicateInputs++
			totalDuplicates += len(aliases) - 1
		}
	}

	fmt.Println(headerStyle.Render("🔍 Flint - Dependency Analysis Report"))

	// Print analysis summary upfront
	if totalInputs == 0 {
		fmt.Println(infoStyle.Render(fmt.Sprintf("%s No inputs found in lockfile", infoIcon)))
		fmt.Println()
		return
	}

	fmt.Println(infoStyle.Render(fmt.Sprintf("%s Analyzing %d unique inputs...", infoIcon, totalInputs)))

	if duplicateInputs == 0 {
		fmt.Println(successStyle.Render(fmt.Sprintf("%s No duplicate inputs detected", successIcon)))
		fmt.Println()
		fmt.Println(dimStyle.Render("All inputs use unique versions. Your dependency tree is optimized!"))
		return
	}

	fmt.Println(warningStyle.Render(fmt.Sprintf("%s Found %d inputs with multiple versions (%d total duplicates)",
		warningIcon, duplicateInputs, totalDuplicates)))
	fmt.Println()

	// Print detailed findings
	fmt.Println(boldStyle.Render("📋 Detailed Analysis:"))
	fmt.Println()

	processedCount := 0
	for url, aliases := range deps {
		if options.Merge {
			if len(aliases) <= 1 {
				continue
			}
			processedCount++

			// Extract main input name
			// We'll prefer the shortest alias or first one
			mainInputName := aliases[0]
			for _, alias := range aliases {
				if len(alias) < len(mainInputName) {
					mainInputName = alias
				}
			}

			// Merge mode output
			fmt.Println(errorStyle.Render(fmt.Sprintf("(%d) %s",
				processedCount, mainInputName)))
			fmt.Printf("   %s %s\n", dimStyle.Render("├─"), boldStyle.Render("URL: ")+urlStyle.Render(url))
			fmt.Printf("   %s %s\n", dimStyle.Render("├─"), warningStyle.Render(fmt.Sprintf("Repeats: %d", len(aliases))))

			// Build dependants set only when needed
			dependantsSet := make(map[string]struct{})
			for _, alias := range aliases {
				for _, dependant := range reverseDeps[alias] {
					dependantsSet[dependant] = struct{}{}
				}
			}

			if len(dependantsSet) > 0 {
				dependants := make([]string, 0, len(dependantsSet))
				for dependant := range dependantsSet {
					dependants = append(dependants, dependant)
				}
				fmt.Printf("   %s %s\n", dimStyle.Render("└─"), dependantStyle.Render(fmt.Sprintf("Used by: %s",
					strings.Join(dependants, ", "))))
			} else {
				fmt.Printf("   %s %s\n", dimStyle.Render("└─"), dimStyle.Render("No direct dependants"))
			}
			fmt.Println()
		} else {
			if len(aliases) == 1 {
				continue
			}
			processedCount++

			// Extract main input name for non-merge mode too
			mainInputName := aliases[0]
			for _, alias := range aliases {
				if len(alias) < len(mainInputName) {
					mainInputName = alias
				}
			}

			fmt.Println(errorStyle.Render(fmt.Sprintf("(%d) %s",
				processedCount, mainInputName)))
			fmt.Printf("   %s %s\n", dimStyle.Render("├─"), boldStyle.Render("URL: ")+urlStyle.Render(url))
			fmt.Printf("   %s %s\n", dimStyle.Render("├─"), warningStyle.Render(fmt.Sprintf("Repeats: %d", len(aliases))))

			for i, alias := range aliases {
				isLast := i == len(aliases)-1
				connector := "├─"
				if isLast {
					connector = "└─"
				}

				fmt.Printf("   %s %s\n", dimStyle.Render(connector), aliasStyle.Render(fmt.Sprintf("Alias: %s", alias)))

				dependants := reverseDeps[alias]
				if len(dependants) > 0 {
					subConnector := "│"
					if isLast {
						subConnector = " "
					}
					fmt.Printf("   %s     %s %s\n", dimStyle.Render(subConnector), dimStyle.Render("└─"),
						dependantStyle.Render(fmt.Sprintf("Used by: %s", strings.Join(dependants, ", "))))
				}

				if options.Verbose {
					subConnector := "│"
					if isLast {
						subConnector = " "
					}
					fmt.Printf("   %s     %s %s\n", dimStyle.Render(subConnector), dimStyle.Render("└─"),
						dimStyle.Render(fmt.Sprintf("Debug: %d dependants", len(dependants))))
				}
			}
			fmt.Println()
		}
	}

	// Summary with "actionable" advice
	fmt.Println(dimStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println(boldStyle.Render("📊 Summary:"))
	fmt.Println()

	// TODO: surely this can be done in a less generic way; "haha fix your inputs" is not a good message
	// and maybe we should suggest using `follows` in the flake.nix for each input that is detected. If
	// I can find a good way, I can even add --patch flag to generate an actually actionable patch.
	if duplicateInputs > 0 {
		fmt.Println(errorStyle.Render(fmt.Sprintf("%s %d inputs have duplicate versions",
			errorIcon, duplicateInputs)))
		fmt.Println(warningStyle.Render(fmt.Sprintf("%s %d total duplicate dependencies detected",
			warningIcon, totalDuplicates)))
		fmt.Println()
		fmt.Println(infoStyle.Render(fmt.Sprintf("%s Recommendation:", infoIcon)))
		fmt.Println("   Consider using 'inputs.<name>.follows' in your flake.nix to deduplicate")
		fmt.Println("   dependencies and reduce closure size.")
		fmt.Println()
		fmt.Println(dimStyle.Render("   Example:"))
		fmt.Println(dimStyle.Render("   inputs.someInput.inputs.nixpkgs.follows = \"nixpkgs\";"))
	}
}

func printPlainOutput(deps map[string][]string, reverseDeps map[string][]string, options Options) {
	// Simple styles for backward compatibility
	var titleStyle, inputStyle, aliasStyle, depStyle, summaryStyle lipgloss.Style

	if util.IsNoColor() {
		// Reuse a single empty style
		emptyStyle := lipgloss.NewStyle()
		titleStyle = emptyStyle
		inputStyle = emptyStyle
		aliasStyle = emptyStyle
		depStyle = emptyStyle
		summaryStyle = emptyStyle
	} else {
		// Titles, bold and underlined.
		titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("5")).
			Bold(true).
			Underline(true)

		// Name of the input from a flake's root
		inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

		// Aliases to an input
		aliasStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")).
			Italic(true)

		// Inputs that depend on a given input
		depStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))

		// Summary at the end
		summaryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Bold(true)
	}

	hasMultipleVersions := false
	fmt.Println(titleStyle.Render("Dependency Analysis Report"))

	for url, aliases := range deps {
		if options.Merge {
			if len(aliases) <= 1 {
				continue
			}
			fmt.Println(inputStyle.Render(fmt.Sprintf("Input: %s", url)))

			// Only build the set when actually needed in merge mode
			dependantsSet := make(map[string]struct{})
			for _, alias := range aliases {
				for _, dependant := range reverseDeps[alias] {
					dependantsSet[dependant] = struct{}{}
				}
			}

			if len(dependantsSet) > 0 {
				dependants := make([]string, 0, len(dependantsSet))
				for dependant := range dependantsSet {
					dependants = append(dependants, dependant)
				}
				fmt.Println(depStyle.Render(fmt.Sprintf("  Dependants: %s", strings.Join(dependants, ", "))))
			}
		} else {
			if len(aliases) == 1 {
				continue
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
		}

		hasMultipleVersions = true
	}

	if !hasMultipleVersions {
		fmt.Println(summaryStyle.Render("No duplicate inputs detected in the repositories analyzed."))
	}
}
