package output

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	gloss "github.com/charmbracelet/lipgloss"
	flake "notashelf.dev/flint/internal/flake"
	util "notashelf.dev/flint/internal/util"
)

// Group by repository identity
func DetectDuplicatesByRepo(deps map[string][]string) map[string][]string {
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

// You cannot imagine how much I'm missing clap right now.
// Or Rust in general...
func ValidateOutputFormat(format string) error {
	validFormats := []string{"json", "plain", "pretty"}

	if slices.Contains(validFormats, format) {
		return nil
	}

	return fmt.Errorf("invalid output format '%s'. Valid formats are: %s", format, strings.Join(validFormats, ", "))
}

func ShouldFailOnDuplicates(options Options, deps map[string][]string) bool {
	if !options.FailIfMultipleVersions {
		return false
	}

	duplicateDeps := DetectDuplicatesByRepo(deps)
	return len(duplicateDeps) > 0
}

func PrintUpdates(results flake.UpdateResults, options Options) error {
	// Validate output format first, even in quiet mode
	if err := ValidateOutputFormat(options.OutputFormat); err != nil {
		return err
	}

	if options.Quiet {
		return nil
	}

	if options.OutputFormat == "json" {
		jsonData, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON output: %w", err)
		}

		fmt.Println(string(jsonData))
		return nil
	}

	// Choose output format
	switch options.OutputFormat {
	case "plain":
		printPlainUpdateOutput(results, options)
	case "pretty":
		printFormattedUpdateOutput(results, options)
	default:
		// Default to pretty for backward compatibility
		printFormattedUpdateOutput(results, options)
	}
	return nil
}

func PrintDependencies(deps map[string][]string, reverseDeps map[string][]string, options Options) error {
	// Validate output format first, even in quiet mode
	if err := ValidateOutputFormat(options.OutputFormat); err != nil {
		return err
	}

	if options.Quiet {
		return nil
	}

	duplicateDeps := DetectDuplicatesByRepo(deps)

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
		printPlainOutput(deps, urlToDependants, options)
	case "pretty":
		printFormattedOutput(deps, urlToDependants, options)
	default:
		// Default to pretty for backward compatibility
		printFormattedOutput(deps, urlToDependants, options)
	}
	return nil
}

func printFormattedOutput(deps map[string][]string, urlToDependants map[string][]string, options Options) {
	duplicateDeps := DetectDuplicatesByRepo(deps)
	// Styles for CI-friendly output
	var (
		headerStyle, successStyle, warningStyle, errorStyle, infoStyle,
		dimStyle, boldStyle, urlStyle, aliasStyle, dependantStyle gloss.Style
	)

	// Status symbols
	var successIcon, warningIcon, errorIcon, infoIcon string

	if util.IsNoColor() {
		// Plain text fallbacks for CI environments
		emptyStyle := gloss.NewStyle()
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
		headerStyle = gloss.NewStyle().
			Foreground(gloss.Color("12")).
			Bold(true).
			Underline(true)

		successStyle = gloss.NewStyle().
			Foreground(gloss.Color("10")).
			Bold(true)

		warningStyle = gloss.NewStyle().
			Foreground(gloss.Color("11")).
			Bold(true)

		errorStyle = gloss.NewStyle().
			Foreground(gloss.Color("9")).
			Bold(true)

		infoStyle = gloss.NewStyle().
			Foreground(gloss.Color("14")).
			Bold(true)

		dimStyle = gloss.NewStyle().
			Foreground(gloss.Color("8"))

		boldStyle = gloss.NewStyle().
			Bold(true)

		urlStyle = gloss.NewStyle().
			Foreground(gloss.Color("6")).
			Underline(true)

		aliasStyle = gloss.NewStyle().
			Foreground(gloss.Color("13")).
			Italic(true)

		dependantStyle = gloss.NewStyle().
			Foreground(gloss.Color("3"))

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

	for _, urls := range duplicateDeps {
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
	for repoIdentity, duplicateUrls := range duplicateDeps {
		processedCount++

		// Extract repository name from identity for display
		repoName := repoIdentity
		if lastSlash := strings.LastIndex(repoIdentity, "/"); lastSlash != -1 {
			repoName = repoIdentity[lastSlash+1:]
		}

		fmt.Println(errorStyle.Render(fmt.Sprintf("(%d) %s",
			processedCount, repoName)))
		fmt.Printf("   %s %s\n", dimStyle.Render("├─"), boldStyle.Render("Repository: ")+urlStyle.Render(repoIdentity))
		fmt.Printf("   %s %s\n", dimStyle.Render("├─"), warningStyle.Render(fmt.Sprintf("Versions: %d", len(duplicateUrls))))

		if options.Merge {
			// Build dependants set; find all nodes that use any version of this repo
			dependantsSet := make(map[string]struct{})
			for _, url := range duplicateUrls {
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
			for i, url := range duplicateUrls {
				isLast := i == len(duplicateUrls)-1
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
	duplicateDeps := DetectDuplicatesByRepo(deps)
	// Simple styles for backward compatibility
	var titleStyle, inputStyle, aliasStyle, depStyle, summaryStyle gloss.Style

	if util.IsNoColor() {
		// Reuse a single empty style
		emptyStyle := gloss.NewStyle()
		titleStyle = emptyStyle
		inputStyle = emptyStyle
		aliasStyle = emptyStyle
		depStyle = emptyStyle
		summaryStyle = emptyStyle
	} else {
		// Titles, bold and underlined.
		titleStyle = gloss.NewStyle().
			Foreground(gloss.Color("5")).
			Bold(true).
			Underline(true)

		// Name of the input from a flake's root
		inputStyle = gloss.NewStyle().
			Foreground(gloss.Color("6")).
			Bold(true)

		// Aliases to an input
		aliasStyle = gloss.NewStyle().
			Foreground(gloss.Color("4")).
			Italic(true)

		// Inputs that depend on a given input
		depStyle = gloss.NewStyle().
			Foreground(gloss.Color("2"))

		// Summary at the end
		summaryStyle = gloss.NewStyle().
			Foreground(gloss.Color("3")).
			Bold(true)
	}

	hasMultipleVersions := false
	fmt.Println(titleStyle.Render("Dependency Analysis Report"))

	for repoIdentity, urls := range duplicateDeps {
		hasMultipleVersions = true

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

func printFormattedUpdateOutput(results flake.UpdateResults, _ Options) {
	// Styles for CI-friendly output
	var (
		headerStyle, successStyle, warningStyle, errorStyle, infoStyle,
		dimStyle, boldStyle, urlStyle, inputStyle gloss.Style
	)

	// Status symbols
	var successIcon, warningIcon, errorIcon, infoIcon string

	if util.IsNoColor() {
		// Plain text fallbacks for CI environments
		emptyStyle := gloss.NewStyle()
		headerStyle = emptyStyle
		successStyle = emptyStyle
		warningStyle = emptyStyle
		errorStyle = emptyStyle
		infoStyle = emptyStyle
		dimStyle = emptyStyle
		boldStyle = emptyStyle
		urlStyle = emptyStyle
		inputStyle = emptyStyle

		// Unicode-safe symbols for CI
		successIcon = "[✓]"
		warningIcon = "[!]"
		errorIcon = "[✗]"
		infoIcon = "[i]"
	} else {
		// Rich colors and styles for interactive terminals
		headerStyle = gloss.NewStyle().
			Foreground(gloss.Color("12")).
			Bold(true).
			Underline(true)

		successStyle = gloss.NewStyle().
			Foreground(gloss.Color("10")).
			Bold(true)

		warningStyle = gloss.NewStyle().
			Foreground(gloss.Color("11")).
			Bold(true)

		errorStyle = gloss.NewStyle().
			Foreground(gloss.Color("9")).
			Bold(true)

		infoStyle = gloss.NewStyle().
			Foreground(gloss.Color("14")).
			Bold(true)

		dimStyle = gloss.NewStyle().
			Foreground(gloss.Color("8"))

		boldStyle = gloss.NewStyle().
			Bold(true)

		urlStyle = gloss.NewStyle().
			Foreground(gloss.Color("6")).
			Underline(true)

		inputStyle = gloss.NewStyle().
			Foreground(gloss.Color("13")).
			Italic(true)

		// Rich symbols for interactive terminals
		successIcon = "✓"
		warningIcon = "⚠"
		errorIcon = "✗"
		infoIcon = "ℹ"
	}

	fmt.Println(headerStyle.Render("🔄 Flint - Update Check Report"))
	fmt.Println()

	// Count statistics
	totalInputs := len(results.Updates)
	availableUpdates := 0
	errors := 0

	for _, update := range results.Updates {
		if update.IsUpdate {
			availableUpdates++
		}
		if update.Error != "" {
			errors++
		}
	}

	if totalInputs == 0 {
		fmt.Println(infoStyle.Render(fmt.Sprintf("%s No inputs found to check", infoIcon)))
		return
	}

	fmt.Println(infoStyle.Render(fmt.Sprintf("%s Checked %d inputs for updates...", infoIcon, totalInputs)))

	if availableUpdates == 0 && errors == 0 {
		fmt.Println(successStyle.Render(fmt.Sprintf("%s All inputs are up to date!", successIcon)))
		return
	}

	if availableUpdates > 0 {
		fmt.Println(warningStyle.Render(fmt.Sprintf("%s %d updates available", warningIcon, availableUpdates)))
	}
	if errors > 0 {
		fmt.Println(errorStyle.Render(fmt.Sprintf("%s %d errors encountered", errorIcon, errors)))
	}
	fmt.Println()

	// Print detailed results
	fmt.Println(boldStyle.Render("📋 Detailed Results:"))
	fmt.Println()

	for i, update := range results.Updates {
		fmt.Printf("%d. %s\n", i+1, inputStyle.Render(update.InputName))

		if update.Error != "" {
			fmt.Printf("   %s %s\n", errorIcon, errorStyle.Render("Error: "+update.Error))
		} else if update.IsUpdate {
			fmt.Printf("   %s %s\n", warningIcon, warningStyle.Render("Update available"))
			fmt.Printf("   %s %s\n", dimStyle.Render("├─"), boldStyle.Render("Current: ")+dimStyle.Render(update.CurrentRev[:8]+"..."))
			fmt.Printf("   %s %s\n", dimStyle.Render("├─"), boldStyle.Render("Latest:  ")+successStyle.Render(update.LatestRev[:8]+"..."))
			fmt.Printf("   %s %s\n", dimStyle.Render("└─"), urlStyle.Render(update.CurrentURL))
		} else {
			fmt.Printf("   %s %s\n", successIcon, successStyle.Render("Up to date"))
			fmt.Printf("   %s %s\n", dimStyle.Render("└─"), dimStyle.Render(update.CurrentRev[:8]+"..."))
		}
		fmt.Println()
	}

	// Summary
	fmt.Println(dimStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println(boldStyle.Render("📊 Summary:"))
	fmt.Println()

	if availableUpdates > 0 {
		fmt.Println(warningStyle.Render(fmt.Sprintf("%s %d inputs have updates available", warningIcon, availableUpdates)))
		fmt.Println(infoStyle.Render("Run 'nix flake update' to update all inputs, or update specific inputs:"))
		fmt.Println(dimStyle.Render("  nix flake update <input-name>"))
		fmt.Println()
	}

	if errors > 0 {
		fmt.Println(errorStyle.Render(fmt.Sprintf("%s %d inputs could not be checked", errorIcon, errors)))
		fmt.Println(infoStyle.Render("This may be due to network issues or unavailable repositories."))
		fmt.Println()
	}

	if availableUpdates == 0 && errors == 0 {
		fmt.Println(successStyle.Render(fmt.Sprintf("%s All inputs are at the latest version", successIcon)))
	}
}

func printPlainUpdateOutput(results flake.UpdateResults, _ Options) {
	// Simple styles for backward compatibility
	var titleStyle, inputStyle, statusStyle, errorStyle gloss.Style

	if util.IsNoColor() {
		emptyStyle := gloss.NewStyle()
		titleStyle = emptyStyle
		inputStyle = emptyStyle
		statusStyle = emptyStyle
		errorStyle = emptyStyle
	} else {
		titleStyle = gloss.NewStyle().
			Foreground(gloss.Color("5")).
			Bold(true).
			Underline(true)

		inputStyle = gloss.NewStyle().
			Foreground(gloss.Color("6")).
			Bold(true)

		statusStyle = gloss.NewStyle().
			Foreground(gloss.Color("2"))

		errorStyle = gloss.NewStyle().
			Foreground(gloss.Color("9")).
			Bold(true)
	}

	fmt.Println(titleStyle.Render("Update Check Report"))
	fmt.Println()

	availableUpdates := 0
	errors := 0

	for _, update := range results.Updates {
		if update.IsUpdate {
			availableUpdates++
		}
		if update.Error != "" {
			errors++
		}
	}

	if availableUpdates == 0 && errors == 0 {
		fmt.Println(statusStyle.Render("All inputs are up to date."))
		return
	}

	for _, update := range results.Updates {
		fmt.Println(inputStyle.Render(fmt.Sprintf("Input: %s", update.InputName)))

		if update.Error != "" {
			fmt.Println(errorStyle.Render(fmt.Sprintf("  Error: %s", update.Error)))
		} else if update.IsUpdate {
			fmt.Println(statusStyle.Render("  Status: Update available"))
			fmt.Printf("  Current: %s\n", update.CurrentRev[:8]+"...")
			fmt.Printf("  Latest:  %s\n", update.LatestRev[:8]+"...")
			fmt.Printf("  URL: %s\n", update.CurrentURL)
		} else {
			fmt.Println(statusStyle.Render("  Status: Up to date"))
			fmt.Printf("  Version: %s\n", update.CurrentRev[:8]+"...")
		}
		fmt.Println()
	}

	// Summary
	if availableUpdates > 0 {
		fmt.Printf("%d inputs have updates available\n", availableUpdates)
	}
	if errors > 0 {
		fmt.Printf("%d inputs could not be checked\n", errors)
	}
}
