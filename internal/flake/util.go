package flake

import (
	"fmt"
	"strings"
)

// Safely retrieves a string value from a map, returning an empty string
// if the value doesn't exist or isn't a string.
func safeGetString(m map[string]any, key string) string {
	if value, ok := m[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

func flakeURL(locked map[string]any) string {
	repo := Input{
		Type:  safeGetString(locked, "type"),
		Owner: safeGetString(locked, "owner"),
		Repo:  safeGetString(locked, "repo"),
	}

	repo.Host = safeGetString(locked, "host")
	repo.URL = safeGetString(locked, "url")
	repo.Path = safeGetString(locked, "path")

	url := generateRepoURL(repo)
	rev := safeGetString(locked, "rev")
	narHash := safeGetString(locked, "narHash")

	// Compose a unique key for the dependency, including version info
	if rev != "" || narHash != "" {
		url += "?"
		if rev != "" {
			url += "rev=" + rev
		}
		if narHash != "" {
			if rev != "" {
				url += "&"
			}
			url += "narHash=" + narHash
		}
	}
	return url
}

func generateRepoURL(repo Input) string {
	switch repo.Type {

	case "github", "gitlab", "sourcehut":
		url := fmt.Sprintf("%s:%s/%s", repo.Type, repo.Owner, repo.Repo)
		if repo.Host != "" {
			url += fmt.Sprintf("?host=%s", repo.Host)
		}
		return url

	case "git", "hg", "tarball":
		return fmt.Sprintf("%s:%s", repo.Type, repo.URL)

	case "path":
		return fmt.Sprintf("%s:%s", repo.Type, repo.Path)

	default:
		return ""
	}
}

func AnalyzeFlake(flakeLock FlakeLock) Relations {
	deps := make(map[string][]string)
	reverseDeps := make(map[string][]string)

	// First we build a map from node name to its locked version key (url)
	nodeToURL := make(map[string]string)
	for nodeName, node := range flakeLock.Nodes {
		if node.Locked != nil {
			lockedMap := map[string]any{
				"type":    node.Locked.Type,
				"owner":   node.Locked.Owner,
				"repo":    node.Locked.Repo,
				"host":    node.Locked.Host,
				"url":     node.Locked.URL,
				"path":    node.Locked.Path,
				"rev":     node.Locked.Rev,
				"narHash": node.Locked.NarHash,
			}
			url := flakeURL(lockedMap)
			if url != "" {
				nodeToURL[nodeName] = url
			}
		}
	}

	// Then, for each node with inputs, we map the input name to the locked
	// node/version and use the referencing node as alias
	for nodeName, node := range flakeLock.Nodes {
		if node.Inputs != nil {
			for _, input := range node.Inputs {
				switch v := input.(type) {
				case string:
					if url, ok := nodeToURL[v]; ok {
						deps[url] = append(deps[url], nodeName)
						reverseDeps[v] = append(reverseDeps[v], nodeName)
					}
				case []any:
					for _, i := range v {
						if str, ok := i.(string); ok {
							if url, ok := nodeToURL[str]; ok {
								deps[url] = append(deps[url], nodeName)
								reverseDeps[str] = append(reverseDeps[str], nodeName)
							}
						}
					}
				}
			}
		}
	}

	return Relations{Deps: deps, ReverseDeps: reverseDeps}
}

// Extract repository identity from URL (without version info)
func ExtractRepoIdentity(url string) string {
	// Handle special case for gitlab/github with host parameter
	if strings.Contains(url, "?host=") {
		// Find the first ? that starts version parameters (not host)
		hostIdx := strings.Index(url, "?host=")
		afterHost := url[hostIdx+len("?host="):]
		// Find the next ? that starts version parameters
		if versionIdx := strings.Index(afterHost, "?"); versionIdx != -1 {
			return url[:hostIdx+len("?host=")+versionIdx]
		}
		return url
	}

	// Remove query parameters (rev, narHash, etc.)
	if idx := strings.Index(url, "?"); idx != -1 {
		return url[:idx]
	}
	return url
}
