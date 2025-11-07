package flake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
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

	// Skip if the input points to a specific commit (evergreen)
	if node.Original != nil && node.Original.Ref != "" && isCommitHash(node.Original.Ref) {
		if verbose {
			fmt.Printf("Skipping %s: pinned to specific commit\n", inputName)
		}
		update.CurrentRev = node.Locked.Rev
		update.CurrentURL = buildFlakeURL(node.Locked)
		update.LatestRev = node.Locked.Rev
		update.LatestURL = update.CurrentURL
		update.IsUpdate = false
		return update
	}

	// Build current URL
	update.CurrentRev = node.Locked.Rev
	update.CurrentURL = buildFlakeURL(node.Locked)

	latestURL, latestRev, err := getLatestRevision(node, verbose)
	if err != nil {
		update.Error = fmt.Sprintf("failed to get latest revision: %v", err)
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

// Check if a string is a commit hash
func isCommitHash(s string) bool {
	if len(s) != 40 {
		return false
	}

	for _, c := range s {
		// Check if character is a valid hex digit (0-9 or a-f)
		// Git commit hashes use lowercase hexadecimal only
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}

	return true
}

// Get the latest revision using HTTP and git ls-remote
func getLatestRevision(node Node, verbose bool) (string, string, error) {
	if node.Locked == nil {
		return "", "", fmt.Errorf("no locked information")
	}

	var gitURL string
	var ref string

	// Determine the type and construct URL accordingly
	if node.Original != nil {
		switch node.Original.Type {
		case "github", "gitlab":
			host := node.Original.Type + ".com"
			if node.Locked.Host != "" {
				host = node.Locked.Host
			}
			gitURL = fmt.Sprintf("https://%s/%s/%s.git", host, node.Locked.Owner, node.Original.Repo)
			ref = node.Original.Ref
		case "git":
			if node.Locked != nil {
				gitURL = node.Locked.URL
			}
			ref = node.Original.Ref

			// Skip git+ssh URLs
			// XXX: can we actually handle this? Needs research.
			if strings.HasPrefix(gitURL, "ssh://") {
				return "", "", fmt.Errorf("git+ssh URLs not supported")
			}
		case "tarball":
			return getLatestRevisionFromTarball(node, verbose)
		default:
			return "", "", fmt.Errorf("unsupported input type: %s", node.Original.Type)
		}
	} else {
		// Fallback to locked info if no original
		switch node.Locked.Type {
		case "github", "gitlab", "sourcehut":
			host := node.Locked.Type + ".com"
			if node.Locked.Host != "" {
				host = node.Locked.Host
			}
			gitURL = fmt.Sprintf("https://%s/%s/%s.git", host, node.Locked.Owner, node.Locked.Repo)
		case "git":
			gitURL = node.Locked.URL
		default:
			return "", "", fmt.Errorf("unsupported locked type: %s", node.Locked.Type)
		}
	}

	if verbose {
		fmt.Printf("Checking %s for updates (ref: %s)\n", gitURL, ref)
	}

	latestRev, err := getLatestCommitHTTP(gitURL, ref, verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to get latest commit: %w", err)
	}

	latestURL := buildFlakeURL(node.Locked)
	return latestURL, latestRev, nil
}

// Get latest revision from tarball URL by reconstructing the repo URL
func getLatestRevisionFromTarball(node Node, verbose bool) (string, string, error) {
	if node.Locked == nil || node.Locked.URL == "" {
		return "", "", fmt.Errorf("no tarball URL found")
	}

	tarballURL := node.Locked.URL

	// Regex to extract repo URL and ref from tarball URL
	// Pattern: https://site.tld/$owner/$repo/archive/$ref.tar.gz
	// XXX: is this accurate? All Git forges generally follow the same pattern
	// but there may be something I'm missing. Investigate.
	re := regexp.MustCompile(`(https?://[^/]+/[^/]+/[^/]+)/(?:archive|releases/download)/(?:refs/tags/)?([^/]+)(?:/[^/]+)?(?:\.tar\.gz|\.zip|\.tar\.xz)`)
	matches := re.FindStringSubmatch(tarballURL)

	if len(matches) != 3 {
		return "", "", fmt.Errorf("cannot parse tarball URL: %s", tarballURL)
	}

	repoURL := matches[1] + ".git"
	ref := matches[2]

	// Skip if ref is a commit hash
	if isCommitHash(ref) {
		return "", "", fmt.Errorf("tarball points to specific commit, skipping")
	}

	if verbose {
		fmt.Printf("Reconstructed git URL from tarball: %s (ref: %s)\n", repoURL, ref)
	}

	latestRev, err := getLatestCommitHTTP(repoURL, ref, verbose)
	if err != nil {
		return "", "", fmt.Errorf("failed to get latest commit: %w", err)
	}

	latestURL := buildFlakeURL(node.Locked)
	return latestURL, latestRev, nil
}

// HTTP client for API requests
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
	},
}

// GitHub API response
type githubRef struct {
	Ref    string `json:"ref"`
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
	} `json:"object"`
}

type githubRepo struct {
	DefaultBranch string `json:"default_branch"`
}

// Get latest commit using direct HTTP APIs instead of git ls-remote
// Slightly more performance by default, but we would have wasted more time
// if this fails, because we fall back to executing 'git ls-remote' anyway.
func getLatestCommitHTTP(gitURL, ref string, verbose bool) (string, error) {
	// Determine API endpoint
	if strings.Contains(gitURL, "github.com") {
		return getGitHubCommit(gitURL, ref, verbose)
	} else if strings.Contains(gitURL, "gitlab.com") {
		return getGitLabCommit(gitURL, ref, verbose)
	}

	// Fallback to generic git protocol for other hosts
	return getGenericGitCommit(gitURL, ref, verbose)
}

// Get commit from GitHub API
func getGitHubCommit(gitURL, ref string, verbose bool) (string, error) {
	// Extract owner/repo from git URL
	re := regexp.MustCompile(`github\.com/([^/]+)/([^/]+)\.git`)
	matches := re.FindStringSubmatch(gitURL)
	if len(matches) != 3 {
		return "", fmt.Errorf("invalid GitHub URL format: %s", gitURL)
	}

	owner, repo := matches[1], strings.TrimSuffix(matches[2], ".git")

	// If no ref specified, get default branch
	if ref == "" || ref == "HEAD" {
		repoURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
		resp, err := httpClient.Get(repoURL)
		if err != nil {
			return "", fmt.Errorf("GitHub API request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
		}

		var repoInfo githubRepo
		if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
			return "", fmt.Errorf("failed to decode GitHub response: %w", err)
		}

		ref = repoInfo.DefaultBranch
	}

	// Get the commit SHA for the ref
	// Try heads/ first for branches, then tags/
	refURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", owner, repo, ref)
	if verbose {
		fmt.Printf("Fetching: %s\n", refURL)
	}

	resp, err := httpClient.Get(refURL)
	if err != nil {
		return "", fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Try as a tag
		tagURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/tags/%s", owner, repo, ref)
		resp.Body.Close()
		resp, err = httpClient.Get(tagURL)
		if err != nil {
			return "", fmt.Errorf("GitHub API request failed: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned status %d for ref %s", resp.StatusCode, ref)
	}

	var refInfo githubRef
	if err := json.NewDecoder(resp.Body).Decode(&refInfo); err != nil {
		return "", fmt.Errorf("failed to decode GitHub response: %w", err)
	}

	// If it's a tag object, get the target commit
	if refInfo.Object.Type == "tag" {
		tagURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/tags/%s", owner, repo, refInfo.Object.SHA)
		resp, err := httpClient.Get(tagURL)
		if err != nil {
			return "", fmt.Errorf("GitHub API request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			var tagInfo struct {
				Object struct {
					SHA string `json:"sha"`
				} `json:"object"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&tagInfo); err == nil {
				return tagInfo.Object.SHA, nil
			}
		}
	}

	return refInfo.Object.SHA, nil
}

// GitLab API response
type gitlabRef struct {
	Name   string `json:"name"`
	Commit struct {
		ID string `json:"id"`
	} `json:"commit"`
}

type gitlabRepo struct {
	DefaultBranch string `json:"default_branch"`
}

// Get commit from GitLab API
func getGitLabCommit(gitURL, ref string, verbose bool) (string, error) {
	// Extract owner/repo from git URL
	re := regexp.MustCompile(`gitlab\.com/([^/]+)/([^/]+)\.git`)
	matches := re.FindStringSubmatch(gitURL)
	if len(matches) != 3 {
		return "", fmt.Errorf("invalid GitLab URL format: %s", gitURL)
	}

	owner, repo := matches[1], strings.TrimSuffix(matches[2], ".git")

	// If no ref specified, get default branch
	if ref == "" || ref == "HEAD" {
		repoURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s%%2F%s", owner, repo)
		resp, err := httpClient.Get(repoURL)
		if err != nil {
			return "", fmt.Errorf("GitLab API request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return "", fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
		}

		var repoInfo gitlabRepo
		if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
			return "", fmt.Errorf("failed to decode GitLab response: %w", err)
		}

		ref = repoInfo.DefaultBranch
	}

	// Get the commit SHA for the ref
	// Try branches first
	refURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s%%2F%s/repository/branches/%s", owner, repo, ref)
	if verbose {
		fmt.Printf("Fetching: %s\n", refURL)
	}

	resp, err := httpClient.Get(refURL)
	if err != nil {
		return "", fmt.Errorf("GitLab API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Try as a tag
		tagURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s%%2F%s/repository/tags/%s", owner, repo, ref)
		resp.Body.Close()
		resp, err = httpClient.Get(tagURL)
		if err != nil {
			return "", fmt.Errorf("GitLab API request failed: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitLab API returned status %d for ref %s", resp.StatusCode, ref)
	}

	var refInfo gitlabRef
	if err := json.NewDecoder(resp.Body).Decode(&refInfo); err != nil {
		return "", fmt.Errorf("failed to decode GitLab response: %w", err)
	}

	return refInfo.Commit.ID, nil
}

// Generic git protocol using smart HTTP protocol
func getGenericGitCommit(gitURL, ref string, verbose bool) (string, error) {
	// Convert git URL to HTTP smart protocol URL
	httpURL := strings.Replace(gitURL, "git://", "https://", 1)
	if !strings.HasPrefix(httpURL, "https://") && !strings.HasPrefix(httpURL, "http://") {
		httpURL = "https://" + httpURL
	}

	// Use git ls-remote as fallback for non-GitHub/GitLab hosts
	// This is still more efficient than the original approach since we don't use Nix
	// which is incredibly inefficient.
	if verbose {
		fmt.Printf("Using git ls-remote for: %s\n", httpURL)
	}

	// XXX: maybe it'll be more efficient to us a Git library as fallback, or simply not
	// fallback at all.
	var args []string
	args = append(args, "ls-remote")

	if ref == "" || ref == "HEAD" {
		args = append(args, httpURL, "HEAD")
	} else {
		args = append(args, "--branches", "--tags", httpURL, ref, ref+"^{}")
	}

	cmd := exec.Command("git", args...)

	if verbose {
		fmt.Printf("Running: %s\n", strings.Join(cmd.Args, " "))
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git ls-remote failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return "", fmt.Errorf("no output from git ls-remote")
	}

	// Parse the output to find the right commit hash
	if ref == "" || ref == "HEAD" {
		fields := strings.Fields(lines[0])
		if len(fields) >= 1 {
			return fields[0], nil
		}
	} else {
		var bestHash string

		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				hash := fields[0]
				refName := fields[1]

				if strings.HasSuffix(refName, "^{}") {
					return hash, nil
				}

				if bestHash == "" {
					bestHash = hash
				}
			}
		}

		if bestHash != "" {
			return bestHash, nil
		}
	}

	return "", fmt.Errorf("could not parse git ls-remote output")
}
