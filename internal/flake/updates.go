package flake

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Check for available updates for flake inputs
func CheckUpdates(flakeLock FlakeLock, verbose bool) (UpdateResults, error) {
	var results UpdateResults
	var wg sync.WaitGroup

	rootNode, exists := flakeLock.Nodes["root"]
	if !exists || rootNode.Inputs == nil {
		return results, fmt.Errorf("no root inputs found")
	}

	updates := make([]UpdateStatus, 0, len(rootNode.Inputs))
	var mu sync.Mutex

	// Check each input for updates in parallel
	for inputName, inputRef := range rootNode.Inputs {
		wg.Add(1)
		go func(name string, ref any) {
			defer wg.Done()

			inputRefStr, ok := ref.(string)
			if !ok {
				update := UpdateStatus{
					InputName: name,
					Error:     "invalid input reference type",
				}
				mu.Lock()
				updates = append(updates, update)
				mu.Unlock()
				return
			}

			update := checkInputUpdate(flakeLock, name, inputRefStr, verbose)

			mu.Lock()
			updates = append(updates, update)
			mu.Unlock()
		}(inputName, inputRef)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	results.Updates = updates
	return results, nil
}

// Check a single input for updates
func checkInputUpdate(flakeLock FlakeLock, inputName, inputRef string, verbose bool) UpdateStatus {
	update := UpdateStatus{
		InputName: inputName,
	}

	// Get the current node
	node, exists := flakeLock.Nodes[inputRef]
	if !exists {
		update.Error = fmt.Sprintf("input node %s not found", inputRef)
		return update
	}

	if node.Locked == nil {
		update.Error = fmt.Sprintf("input %s has no locked version", inputName)
		return update
	}

	// Build current URL
	update.CurrentRev = node.Locked.Rev
	update.CurrentURL = buildFlakeURL(node.Locked)

	// Check for update using nix flake info
	latestURL, latestRev, err := getLatestFlakeInfo(update.CurrentURL, verbose)
	if err != nil {
		update.Error = fmt.Sprintf("failed to get latest info: %v", err)
		return update
	}

	update.LatestURL = latestURL
	update.LatestRev = latestRev
	update.IsUpdate = latestRev != "" && latestRev != update.CurrentRev

	return update
}

// Construct a flake URL from Locked info
func buildFlakeURL(locked *Locked) string {
	if locked == nil {
		return ""
	}

	switch locked.Type {
	case "github", "gitlab", "sourcehut":
		url := fmt.Sprintf("%s:%s/%s", locked.Type, locked.Owner, locked.Repo)
		if locked.Host != "" && locked.Host != "github.com" && locked.Host != "gitlab.com" {
			url += fmt.Sprintf("?host=%s", locked.Host)
		}
		return url
	case "git":
		return locked.URL
	case "path":
		return locked.Path
	case "tarball":
		return locked.URL
	default:
		return ""
	}
}

// Get the latest flake information by parsing the output of `nix flake info`
func getLatestFlakeInfo(flakeURL string, verbose bool) (string, string, error) {
	// Run nix flake info --json to get latest information
	cmd := exec.Command("nix", "flake", "info", "--json", flakeURL)

	if verbose {
		fmt.Printf("Running: %s\n", strings.Join(cmd.Args, " "))
	}

	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("nix flake info failed: %w", err)
	}

	// Parse the JSON output
	var flakeInfo map[string]any
	if err := json.Unmarshal(output, &flakeInfo); err != nil {
		return "", "", fmt.Errorf("failed to parse nix output: %w", err)
	}

	// Extract the revision from the locked information
	locked, ok := flakeInfo["locked"].(map[string]any)
	if !ok {
		return "", "", fmt.Errorf("no locked information in nix output")
	}

	rev, ok := locked["rev"].(string)
	if !ok {
		return "", "", fmt.Errorf("no revision in locked information")
	}

	// Rebuild the URL to ensure consistency
	var latestURL string
	if flakeType, ok := locked["type"].(string); ok {
		switch flakeType {
		case "github", "gitlab", "sourcehut":
			if owner, ok := locked["owner"].(string); ok {
				if repo, ok := locked["repo"].(string); ok {
					latestURL = fmt.Sprintf("%s:%s/%s", flakeType, owner, repo)
					if host, ok := locked["host"].(string); ok && host != "github.com" && host != "gitlab.com" {
						latestURL += fmt.Sprintf("?host=%s", host)
					}
				}
			}
		case "git":
			if url, ok := locked["url"].(string); ok {
				latestURL = url
			}
		case "path":
			if path, ok := locked["path"].(string); ok {
				latestURL = path
			}
		case "tarball":
			if url, ok := locked["url"].(string); ok {
				latestURL = url
			}
		}
	}

	return latestURL, rev, nil
}
