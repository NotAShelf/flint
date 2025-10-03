package flake

import (
	"fmt"
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

	for name, node := range flakeLock.Nodes {
		if node.Inputs != nil {
			for _, input := range node.Inputs {
				switch v := input.(type) {
				case string:
					reverseDeps[v] = append(reverseDeps[v], name)
				case []any:
					for _, i := range v {
						if str, ok := i.(string); ok {
							reverseDeps[str] = append(reverseDeps[str], name)
						}
					}
				}
			}
		}

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
				deps[url] = append(deps[url], name)
			}
		}
	}

	return Relations{Deps: deps, ReverseDeps: reverseDeps}
}
