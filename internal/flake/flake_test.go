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
		if len(aliases) != 2 {
			t.Errorf("expected 2 aliases for url %s, got %v", url, aliases)
		}
		if !(slices.Contains(aliases, "foo") && slices.Contains(aliases, "bar")) {
			t.Errorf("expected aliases to contain both 'foo' and 'bar', got %v", aliases)
		}
	}
}
