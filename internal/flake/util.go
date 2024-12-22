package flake

import (
	"fmt"
)

// Safely retrieves a string value from a map, returning an empty string
// if the value doesn't exist or isn't a string.
func safeGetString(m map[string]interface{}, key string) string {
	if value, ok := m[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

func flakeURL(dep map[string]interface{}) string {
	locked, ok := dep["locked"].(map[string]interface{})
	if !ok {
		return ""
	}

	repo := Input{
		Type:  safeGetString(locked, "type"),
		Owner: safeGetString(locked, "owner"),
		Repo:  safeGetString(locked, "repo"),
	}

	repo.Host = safeGetString(locked, "host")
	repo.URL = safeGetString(locked, "url")
	repo.Path = safeGetString(locked, "path")

	return generateRepoURL(repo)
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

func AnalyzeFlake(flakeLock map[string]interface{}) Relations {
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

	return Relations{Deps: deps, ReverseDeps: reverseDeps}
}
