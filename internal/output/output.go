package output

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	util "notashelf.dev/flint/internal/util"
	"notashelf.dev/flint/internal/flake"
)



// Group by repository identity
func detectDuplicatesByRepo(deps map[string][]string) map[string][]string {
	repoGroups := make(map[string][]string)

	for url := range deps {
		repoIdentity := flake.ExtractRepoIdentity(url)
		repoGroups[repoIdentity] = append(repoGroups[repoIdentity], url)
	}

	// Only return repositories that have multiple versions
	duplicates := make(map[string][]string)
	for repoIdentity, urls := range repoGroups {
		if len(urls) > 1 {
			duplicates[repoIdentity] = urls
		}
	}

	return duplicates
}

type Options struct {
	OutputFormat           string
	Verbose                bool
	Merge                  bool
	FailIfMultipleVersions bool
	Quiet                  bool
}

func PrintDependencies(deps map[string][]string, reverseDeps map[string][]string, options Options) error {
	if options.Quiet {
		return nil
	}

	duplicateDeps := detectDuplicatesByRepo(deps)

	// Build a mapping from URL to dependants for easier lookup. The dependants
	// of a URL are the nodes that directly reference it
	urlToDependants := make(map[string][]string)
	for url, aliases := range deps {
		dependantsSet := make(map[string]struct{})
		for _, alias := range aliases {
			dependantsSet[alias] = struct{}{}
		}

		dependants := make([]string, 0, len(dependantsSet))
		for dependant := range dependantsSet {
			dependants = append(dependants, dependant)
		}

		urlToDependants[url] = dependants
	}

	if options.OutputFormat == "json" {
		output := map[string]any{
			"dependencies":         deps,
			"reverse_dependencies": reverseDeps,
			"duplicates":           duplicateDeps,
		}

		jsonData, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON output: %w", err)
		}

		fmt.Println(string(jsonData))
		return nil
	}

	// Choose output format
	switch options.OutputFormat {
	case "plain":
		printPlainOutput(duplicateDeps, urlToDependants, options)
	case "pretty":
		printFormattedOutput(duplicateDeps, urlToDependants, options)
	default:
		// Default to pretty for backward compatibility
		printFormattedOutput(duplicateDeps, urlToDependants, options)
	}
	return nil
}

func printFormattedOutput(deps map[string][]string, urlToDependants map[string][]string, options Options) {
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

	for _, urls := range deps {
		if len(urls) > 1 {
			duplicateInputs++
			totalDuplicates += len(urls) - 1
		}
	}

	fmt.Println(headerStyle.Render("🔍 Flint - Dependency Analysis Report"))

	// Print analysis summary upfront
	if totalInputs == 0 {
		fmt.Println(infoStyle.Render(fmt.Sprintf("%s No inputs found in lockfile", infoIcon)))
		fmt.Println()
		return
	}

	fmt.Println(infoStyle.Render(fmt.Sprintf("%s Analyzing %d unique repositories...", infoIcon, totalInputs)))

	if duplicateInputs == 0 {
		fmt.Println(successStyle.Render(fmt.Sprintf("%s No duplicate repositories detected", successIcon)))
		fmt.Println()
		fmt.Println(dimStyle.Render("All repositories use unique versions. Your dependency tree is optimized!"))
		return
	}

	fmt.Println(warningStyle.Render(fmt.Sprintf("%s Found %d repositories with multiple versions (%d total duplicates)",
		warningIcon, duplicateInputs, totalDuplicates)))
	fmt.Println()

	// Print detailed findings
	fmt.Println(boldStyle.Render("📋 Detailed Analysis:"))
	fmt.Println()

	processedCount := 0
	for repoIdentity, urls := range deps {
		if len(urls) <= 1 {
			continue
		}
		processedCount++

		// Extract repository name from identity for display
		repoName := repoIdentity
		if lastSlash := strings.LastIndex(repoIdentity, "/"); lastSlash != -1 {
			repoName = repoIdentity[lastSlash+1:]
		}

		fmt.Println(errorStyle.Render(fmt.Sprintf("(%d) %s",
			processedCount, repoName)))
		fmt.Printf("   %s %s\n", dimStyle.Render("├─"), boldStyle.Render("Repository: ")+urlStyle.Render(repoIdentity))
		fmt.Printf("   %s %s\n", dimStyle.Render("├─"), warningStyle.Render(fmt.Sprintf("Versions: %d", len(urls))))

		if options.Merge {
			// Build dependants set; find all nodes that use any version of this repo
			dependantsSet := make(map[string]struct{})
			for _, url := range urls {
				if dependants, exists := urlToDependants[url]; exists {
					for _, dependant := range dependants {
						dependantsSet[dependant] = struct{}{}
					}
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
		} else {
			// Show each version
			for i, url := range urls {
				isLast := i == len(urls)-1
				connector := "├─"
				if isLast {
					connector = "└─"
				}

				// Extract version info from URL
				versionInfo := ""
				if revIdx := strings.Index(url, "?rev="); revIdx != -1 {
					revStart := revIdx + 5
					revEnd := strings.Index(url[revStart:], "&")
					if revEnd == -1 {
						revEnd = len(url)
					} else {
						revEnd += revStart
					}
					if revEnd > revStart {
						versionInfo = url[revStart:revEnd] // full rev
						versionInfo = " (" + versionInfo + ")"
					}
				}

				fmt.Printf("   %s %s\n", dimStyle.Render(connector), aliasStyle.Render(fmt.Sprintf("Version%s", versionInfo)))

				// Find dependants for this specific URL
				dependants := []string{}
				if deps, exists := urlToDependants[url]; exists {
					dependants = deps
				}

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
		}
		fmt.Println()
	}

	// Summary with "actionable" advice
	fmt.Println(dimStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println(boldStyle.Render("📊 Summary:"))
	fmt.Println()

	// TODO: surely this can be done in a less generic way; "haha fix your inputs" is not a good message
	// and maybe we should suggest using `follows` in the flake.nix for each input that is detected. If
	// I can find a good way, I can even add --patch flag to generate an actually actionable patch.
	if duplicateInputs > 0 {
		fmt.Println(errorStyle.Render(fmt.Sprintf("%s %d repositories have duplicate versions",
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

func printPlainOutput(deps map[string][]string, urlToDependants map[string][]string, options Options) {
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

	for repoIdentity, urls := range deps {
		if len(urls) <= 1 {
			continue
		}

		hasMultipleVersions = true
		fmt.Println(inputStyle.Render(fmt.Sprintf("Repository: %s", repoIdentity)))

		if options.Merge {
			// Build dependants set
			dependantsSet := make(map[string]struct{})
			for _, url := range urls {
				if dependants, exists := urlToDependants[url]; exists {
					for _, dependant := range dependants {
						dependantsSet[dependant] = struct{}{}
					}
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
			for _, url := range urls {
				fmt.Println(aliasStyle.Render(fmt.Sprintf("  Version: %s", url)))
				if dependants, exists := urlToDependants[url]; exists && len(dependants) > 0 {
					fmt.Println(depStyle.Render(fmt.Sprintf("    Dependants: %s", strings.Join(dependants, ", "))))
				}
				if options.Verbose {
					fmt.Println(depStyle.Render(fmt.Sprintf("    [Debug] %d inputs depend on this version", len(urlToDependants[url]))))
				}
				fmt.Println()
			}
		}
	}

	if !hasMultipleVersions {
		fmt.Println(summaryStyle.Render("No duplicate repositories detected in the lockfile."))
	}
}
