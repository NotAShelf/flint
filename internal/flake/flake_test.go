package flake

import (
	"encoding/json"
	"slices"
	"testing"
)

func loadLock(t *testing.T, data string) FlakeLock {
	t.Helper()
	var lock FlakeLock
	if err := json.Unmarshal([]byte(data), &lock); err != nil {
		t.Fatalf("failed to unmarshal lock: %v", err)
	}
	return lock
}

func TestAnalyzeFlake_SingleInput(t *testing.T) {
	lockData := `
{
  "nodes": {
    "nixpkgs": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-abc",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "abcdef",
        "type": "github"
      }
    },
    "root": {
      "inputs": {
        "nixpkgs": "nixpkgs"
      }
    }
  },
  "root": "root",
  "version": 7
}
`
	lock := loadLock(t, lockData)
	result := AnalyzeFlake(lock)

	if len(result.Deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result.Deps))
	}
	for url, aliases := range result.Deps {
		if url != "github:NixOS/nixpkgs?rev=abcdef&narHash=sha256-abc" {
			t.Errorf("unexpected url: %s", url)
		}
		if len(aliases) != 1 || aliases[0] != "root" {
			t.Errorf("unexpected aliases: %v", aliases)
		}
	}
}

func TestAnalyzeFlake_DuplicateInputs(t *testing.T) {
	lockData := `
{
  "nodes": {
    "nixpkgs": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-abc",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "abcdef",
        "type": "github"
      }
    },
    "foo": {
      "inputs": {
        "nixpkgs": "nixpkgs"
      }
    },
    "bar": {
      "inputs": {
        "nixpkgs": "nixpkgs"
      }
    }
  },
  "root": "foo",
  "version": 7
}
`
	lock := loadLock(t, lockData)
	result := AnalyzeFlake(lock)

	if len(result.Deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result.Deps))
	}
	for url, aliases := range result.Deps {
		if url != "github:NixOS/nixpkgs?rev=abcdef&narHash=sha256-abc" {
			t.Errorf("unexpected url: %s", url)
		}
		if len(aliases) != 2 || !(slices.Contains(aliases, "foo") && slices.Contains(aliases, "bar")) {
			t.Errorf("expected aliases to contain both 'foo' and 'bar', got %v", aliases)
		}
	}

	// Simulate two different versions by adding a second node with different rev
	lockData2 := `
{
  "nodes": {
    "nixpkgs": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-abc",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "abcdef",
        "type": "github"
      }
    },
    "nixpkgs2": {
      "locked": {
        "lastModified": 1759381079,
        "narHash": "sha256-def",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "fedcba",
        "type": "github"
      }
    },
    "foo": {
      "inputs": {
        "nixpkgs": "nixpkgs"
      }
    },
    "bar": {
      "inputs": {
        "nixpkgs": "nixpkgs2"
      }
    }
  },
  "root": "foo",
  "version": 7
}
`
	lock2 := loadLock(t, lockData2)
	result2 := AnalyzeFlake(lock2)

	if len(result2.Deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(result2.Deps))
	}
	found := 0
	for url, aliases := range result2.Deps {
		switch url {
		case "github:NixOS/nixpkgs?rev=abcdef&narHash=sha256-abc":
			found++
			if len(aliases) != 1 || aliases[0] != "foo" {
				t.Errorf("unexpected aliases for nixpkgs: %v", aliases)
			}
		case "github:NixOS/nixpkgs?rev=fedcba&narHash=sha256-def":
			found++
			if len(aliases) != 1 || aliases[0] != "bar" {
				t.Errorf("unexpected aliases for nixpkgs2: %v", aliases)
			}
		}
	}
	if found != 2 {
		t.Errorf("expected to find both nixpkgs urls, found %d", found)
	}
}

func TestAnalyzeFlake_MultipleAliasesSameVersion(t *testing.T) {
	lockData := `
{
  "nodes": {
    "nixpkgs": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-abc",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "abcdef",
        "type": "github"
      }
    },
    "foo": {
      "inputs": {
        "nixpkgs": "nixpkgs"
      }
    },
    "bar": {
      "inputs": {
        "nixpkgs": "nixpkgs"
      }
    }
  },
  "root": "foo",
  "version": 7
}
`
	lock := loadLock(t, lockData)
	result := AnalyzeFlake(lock)

	if len(result.Deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result.Deps))
	}
	for url, aliases := range result.Deps {
		if url != "github:NixOS/nixpkgs?rev=abcdef&narHash=sha256-abc" {
			t.Errorf("unexpected url: %s", url)
		}
		if len(aliases) != 2 || !(slices.Contains(aliases, "foo") && slices.Contains(aliases, "bar")) {
			t.Errorf("expected aliases to contain both 'foo' and 'bar', got %v", aliases)
		}
	}
}

func TestAnalyzeFlake_TransitiveDependencies(t *testing.T) {
	// Test foo -> bar -> baz chain
	lockData := `
{
  "nodes": {
    "baz": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-baz",
        "owner": "example",
        "repo": "baz",
        "rev": "baz123",
        "type": "github"
      }
    },
    "bar": {
      "locked": {
        "lastModified": 1759381077,
        "narHash": "sha256-bar",
        "owner": "example",
        "repo": "bar",
        "rev": "bar456",
        "type": "github"
      },
      "inputs": {
        "baz": "baz"
      }
    },
    "foo": {
      "locked": {
        "lastModified": 1759381076,
        "narHash": "sha256-foo",
        "owner": "example",
        "repo": "foo",
        "rev": "foo789",
        "type": "github"
      },
      "inputs": {
        "bar": "bar"
      }
    },
    "root": {
      "inputs": {
        "foo": "foo"
      }
    }
  },
  "root": "root",
  "version": 7
}
`
	lock := loadLock(t, lockData)
	result := AnalyzeFlake(lock)

	if len(result.Deps) != 3 {
		t.Fatalf("expected 3 deps, got %d", len(result.Deps))
	}

	// Check that each dependency has the correct aliases
	expectedDeps := map[string][]string{
		"github:example/baz?rev=baz123&narHash=sha256-baz": {"bar"},
		"github:example/bar?rev=bar456&narHash=sha256-bar": {"foo"},
		"github:example/foo?rev=foo789&narHash=sha256-foo": {"root"},
	}

	for url, expectedAliases := range expectedDeps {
		aliases, exists := result.Deps[url]
		if !exists {
			t.Errorf("missing dependency: %s", url)
			continue
		}
		if len(aliases) != len(expectedAliases) {
			t.Errorf("expected %d aliases for %s, got %d", len(expectedAliases), url, len(aliases))
		}
		for _, expected := range expectedAliases {
			if !slices.Contains(aliases, expected) {
				t.Errorf("expected alias %s not found in %v for %s", expected, aliases, url)
			}
		}
	}
}

func TestAnalyzeFlake_MultipleVersionsSameRepo(t *testing.T) {
	// Test different versions of same repository through different paths
	lockData := `
{
  "nodes": {
    "nixpkgs-old": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-old",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "old123",
        "type": "github"
      }
    },
    "nixpkgs-new": {
      "locked": {
        "lastModified": 1759381079,
        "narHash": "sha256-new",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "new456",
        "type": "github"
      }
    },
    "stable-package": {
      "locked": {
        "lastModified": 1759381075,
        "narHash": "sha256-stable",
        "owner": "example",
        "repo": "stable-pkg",
        "rev": "stable789",
        "type": "github"
      },
      "inputs": {
        "nixpkgs": "nixpkgs-old"
      }
    },
    "unstable-package": {
      "locked": {
        "lastModified": 1759381076,
        "narHash": "sha256-unstable",
        "owner": "example",
        "repo": "unstable-pkg",
        "rev": "unstable012",
        "type": "github"
      },
      "inputs": {
        "nixpkgs": "nixpkgs-new"
      }
    },
    "root": {
      "inputs": {
        "stable": "stable-package",
        "unstable": "unstable-package"
      }
    }
  },
  "root": "root",
  "version": 7
}
`
	lock := loadLock(t, lockData)
	result := AnalyzeFlake(lock)

	if len(result.Deps) != 4 {
		t.Fatalf("expected 4 deps, got %d", len(result.Deps))
	}

	// Verify we have both versions of nixpkgs as separate dependencies
	foundOld, foundNew := false, false
	for url := range result.Deps {
		if url == "github:NixOS/nixpkgs?rev=old123&narHash=sha256-old" {
			foundOld = true
		}
		if url == "github:NixOS/nixpkgs?rev=new456&narHash=sha256-new" {
			foundNew = true
		}
	}
	if !foundOld {
		t.Error("old version of nixpkgs not found")
	}
	if !foundNew {
		t.Error("new version of nixpkgs not found")
	}
}

func TestAnalyzeFlake_MixedRepositoryTypes(t *testing.T) {
	// Test mixed repository types: github, gitlab, git, path, tarball
	lockData := `
{
  "nodes": {
    "github-repo": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-github",
        "owner": "owner1",
        "repo": "repo1",
        "rev": "github123",
        "type": "github"
      }
    },
    "gitlab-repo": {
      "locked": {
        "lastModified": 1759381077,
        "narHash": "sha256-gitlab",
        "owner": "owner2",
        "repo": "repo2",
        "rev": "gitlab456",
        "type": "gitlab"
      }
    },
    "git-repo": {
      "locked": {
        "lastModified": 1759381076,
        "narHash": "sha256-git",
        "rev": "git789",
        "type": "git",
        "url": "https://example.com/repo.git"
      }
    },
    "path-repo": {
      "locked": {
        "lastModified": 1759381075,
        "narHash": "sha256-path",
        "type": "path",
        "path": "/local/path"
      }
    },
    "tarball-repo": {
      "locked": {
        "lastModified": 1759381074,
        "narHash": "sha256-tarball",
        "type": "tarball",
        "url": "https://example.com/archive.tar.gz"
      }
    },
    "sourcehut-repo": {
      "locked": {
        "lastModified": 1759381073,
        "narHash": "sha256-srht",
        "owner": "owner3",
        "repo": "repo3",
        "rev": "srht012",
        "type": "sourcehut"
      }
    },
    "root": {
      "inputs": {
        "github": "github-repo",
        "gitlab": "gitlab-repo",
        "git": "git-repo",
        "path": "path-repo",
        "tarball": "tarball-repo",
        "sourcehut": "sourcehut-repo"
      }
    }
  },
  "root": "root",
  "version": 7
}
`
	lock := loadLock(t, lockData)
	result := AnalyzeFlake(lock)

	if len(result.Deps) != 6 {
		t.Fatalf("expected 6 deps, got %d", len(result.Deps))
	}

	// TODO: investigate flake URLs further to see if those are all combinations
	// that we need to be worried about.
	expectedURLs := map[string]string{
		"github-repo":    "github:owner1/repo1?rev=github123&narHash=sha256-github",
		"gitlab-repo":    "gitlab:owner2/repo2?rev=gitlab456&narHash=sha256-gitlab",
		"git-repo":       "git:https://example.com/repo.git?rev=git789&narHash=sha256-git",
		"path-repo":      "path:/local/path?narHash=sha256-path",
		"tarball-repo":   "tarball:https://example.com/archive.tar.gz?narHash=sha256-tarball",
		"sourcehut-repo": "sourcehut:owner3/repo3?rev=srht012&narHash=sha256-srht",
	}

	for nodeName, expectedURL := range expectedURLs {
		found := false
		for url, aliases := range result.Deps {
			if url == expectedURL {
				found = true
				if len(aliases) != 1 || aliases[0] != "root" {
					t.Errorf("expected alias 'root' for %s, got %v", nodeName, aliases)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected URL %s for node %s not found", expectedURL, nodeName)
		}
	}
}

func TestAnalyzeFlake_ComplexDependencyTree(t *testing.T) {
	// Test a more complex scenario with multiple levels and shared dependencies
	lockData := `
{
  "nodes": {
    "nixpkgs": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-nixpkgs",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "nixpkgs123",
        "type": "github"
      }
    },
    "home-manager": {
      "locked": {
        "lastModified": 1759381077,
        "narHash": "sha256-hm",
        "owner": "nix-community",
        "repo": "home-manager",
        "rev": "hm456",
        "type": "github"
      },
      "inputs": {
        "nixpkgs": "nixpkgs"
      }
    },
    "neovim-flake": {
      "locked": {
        "lastModified": 1759381076,
        "narHash": "sha256-nvim",
        "owner": "neovim",
        "repo": "neovim-flake",
        "rev": "nvim789",
        "type": "github"
      },
      "inputs": {
        "nixpkgs": "nixpkgs"
      }
    },
    "my-config": {
      "locked": {
        "lastModified": 1759381075,
        "narHash": "sha256-config",
        "type": "path",
        "path": "/home/user/config"
      },
      "inputs": {
        "home-manager": "home-manager",
        "neovim": "neovim-flake"
      }
    },
    "work-config": {
      "locked": {
        "lastModified": 1759381074,
        "narHash": "sha256-work",
        "type": "git",
        "url": "https://gitlab.com/company/work-config.git",
        "rev": "work012"
      },
      "inputs": {
        "home-manager": "home-manager",
        "nixpkgs": "nixpkgs"
      }
    },
    "root": {
      "inputs": {
        "config": "my-config",
        "work": "work-config"
      }
    }
  },
  "root": "root",
  "version": 7
}
`
	lock := loadLock(t, lockData)
	result := AnalyzeFlake(lock)

	if len(result.Deps) != 5 {
		t.Fatalf("expected 5 deps, got %d", len(result.Deps))
	}

	// Check nixpkgs is shared by multiple nodes
	nixpkgsURL := "github:NixOS/nixpkgs?rev=nixpkgs123&narHash=sha256-nixpkgs"
	nixpkgsAliases, exists := result.Deps[nixpkgsURL]
	if !exists {
		t.Fatal("nixpkgs dependency not found")
	}
	expectedAliases := []string{"home-manager", "neovim-flake", "work-config"}
	if len(nixpkgsAliases) != len(expectedAliases) {
		t.Errorf("expected %d aliases for nixpkgs, got %d. Actual aliases: %v", len(expectedAliases), len(nixpkgsAliases), nixpkgsAliases)
	}
	for _, expected := range expectedAliases {
		if !slices.Contains(nixpkgsAliases, expected) {
			t.Errorf("expected alias %s not found in nixpkgs aliases: %v", expected, nixpkgsAliases)
		}
	}

	// Check home-manager is shared by my-config and work-config
	hmURL := "github:nix-community/home-manager?rev=hm456&narHash=sha256-hm"
	hmAliases, exists := result.Deps[hmURL]
	if !exists {
		t.Fatal("home-manager dependency not found")
	}
	expectedHMAliases := []string{"my-config", "work-config"}
	if len(hmAliases) != len(expectedHMAliases) {
		t.Errorf("expected %d aliases for home-manager, got %d", len(expectedHMAliases), len(hmAliases))
	}
	for _, expected := range expectedHMAliases {
		if !slices.Contains(hmAliases, expected) {
			t.Errorf("expected alias %s not found in home-manager aliases: %v", expected, hmAliases)
		}
	}
}

func TestAnalyzeFlake_EdgeCases(t *testing.T) {
	// Test empty inputs
	t.Run("empty inputs", func(t *testing.T) {
		lockData := `{
  "nodes": {
    "root": {}
  },
  "root": "root",
  "version": 7
}`
		lock := loadLock(t, lockData)
		result := AnalyzeFlake(lock)
		if len(result.Deps) != 0 {
			t.Errorf("expected 0 deps for empty inputs, got %d", len(result.Deps))
		}
	})

	// Test node with locked but no inputs
	t.Run("locked but no inputs", func(t *testing.T) {
		lockData := `{
  "nodes": {
    "some-repo": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-test",
        "owner": "owner",
        "repo": "repo",
        "rev": "rev123",
        "type": "github"
      }
    },
    "root": {
      "inputs": {}
    }
  },
  "root": "root",
  "version": 7
}`
		lock := loadLock(t, lockData)
		result := AnalyzeFlake(lock)
		if len(result.Deps) != 0 {
			t.Errorf("expected 0 deps when repo is not referenced, got %d", len(result.Deps))
		}
	})

	// Test circular reference (should not crash)
	t.Run("circular reference", func(t *testing.T) {
		lockData := `{
  "nodes": {
    "a": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-a",
        "owner": "owner",
        "repo": "a",
        "rev": "a123",
        "type": "github"
      },
      "inputs": {
        "b": "b"
      }
    },
    "b": {
      "locked": {
        "lastModified": 1759381077,
        "narHash": "sha256-b",
        "owner": "owner",
        "repo": "b",
        "rev": "b456",
        "type": "github"
      },
      "inputs": {
        "a": "a"
      }
    },
    "root": {
      "inputs": {
        "a": "a"
      }
    }
  },
  "root": "root",
  "version": 7
}`
		lock := loadLock(t, lockData)
		result := AnalyzeFlake(lock)
		if len(result.Deps) != 2 {
			t.Errorf("expected 2 deps for circular reference, got %d", len(result.Deps))
		}
	})

	// Test missing referenced node (should not crash)
	t.Run("missing reference", func(t *testing.T) {
		lockData := `{
  "nodes": {
    "root": {
      "inputs": {
        "nonexistent": "missing-node"
      }
    }
  },
  "root": "root",
  "version": 7
}`
		lock := loadLock(t, lockData)
		result := AnalyzeFlake(lock)
		if len(result.Deps) != 0 {
			t.Errorf("expected 0 deps for missing reference, got %d", len(result.Deps))
		}
	})

	// Test array inputs (follows)
	t.Run("array inputs", func(t *testing.T) {
		lockData := `{
  "nodes": {
    "shared": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-shared",
        "owner": "owner",
        "repo": "shared",
        "rev": "shared123",
        "type": "github"
      }
    },
    "package1": {
      "locked": {
        "lastModified": 1759381077,
        "narHash": "sha256-pkg1",
        "owner": "owner",
        "repo": "package1",
        "rev": "pkg1456",
        "type": "github"
      },
      "inputs": {
        "shared": ["shared"]
      }
    },
    "root": {
      "inputs": {
        "pkg1": "package1"
      }
    }
  },
  "root": "root",
  "version": 7
}`
		lock := loadLock(t, lockData)
		result := AnalyzeFlake(lock)
		if len(result.Deps) != 2 {
			t.Errorf("expected 2 deps for array inputs, got %d", len(result.Deps))
		}
	})
}

func TestAnalyzeFlake_HostedGitLab(t *testing.T) {
	// Test self-hosted GitLab instance
	lockData := `
{
  "nodes": {
    "gitlab-repo": {
      "locked": {
        "lastModified": 1759381078,
        "narHash": "sha256-gitlab",
        "owner": "user",
        "repo": "project",
        "rev": "gitlab123",
        "type": "gitlab",
        "host": "gitlab.example.com"
      }
    },
    "root": {
      "inputs": {
        "gitlab": "gitlab-repo"
      }
    }
  },
  "root": "root",
  "version": 7
}
`
	lock := loadLock(t, lockData)
	result := AnalyzeFlake(lock)

	if len(result.Deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result.Deps))
	}

	expectedURL := "gitlab:user/project?host=gitlab.example.com?rev=gitlab123&narHash=sha256-gitlab"
	for url, aliases := range result.Deps {
		if url != expectedURL {
			t.Errorf("expected URL %s, got %s", expectedURL, url)
		}
		if len(aliases) != 1 || aliases[0] != "root" {
			t.Errorf("expected alias 'root', got %v", aliases)
		}
	}
}

func TestExtractRepoIdentity(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "github:NixOS/nixpkgs?rev=abcdef&narHash=sha256-abc",
			expected: "github:NixOS/nixpkgs",
		},
		{
			input:    "gitlab:user/project?host=gitlab.example.com?rev=123&narHash=hash",
			expected: "gitlab:user/project?host=gitlab.example.com",
		},
		{
			input:    "git:https://example.com/repo.git?rev=abc&narHash=hash",
			expected: "git:https://example.com/repo.git",
		},
		{
			input:    "path:/local/path?narHash=hash",
			expected: "path:/local/path",
		},
		{
			input:    "tarball:https://example.com/archive.tar.gz?narHash=hash",
			expected: "tarball:https://example.com/archive.tar.gz",
		},
		{
			input:    "github:owner/repo",
			expected: "github:owner/repo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := ExtractRepoIdentity(tc.input)
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}
