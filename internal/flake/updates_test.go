package flake

import (
	"testing"
)

func TestBuildFlakeURL(t *testing.T) {
	testCases := []struct {
		name     string
		locked   *Locked
		expected string
	}{
		{
			name: "github repository",
			locked: &Locked{
				Type:  "github",
				Owner: "NixOS",
				Repo:  "nixpkgs",
			},
			expected: "github:NixOS/nixpkgs",
		},
		{
			name: "gitlab with custom host",
			locked: &Locked{
				Type:  "gitlab",
				Owner: "user",
				Repo:  "project",
				Host:  "gitlab.example.com",
			},
			expected: "gitlab:user/project?host=gitlab.example.com",
		},
		{
			name: "gitlab with default host",
			locked: &Locked{
				Type:  "gitlab",
				Owner: "user",
				Repo:  "project",
				Host:  "gitlab.com",
			},
			expected: "gitlab:user/project",
		},
		{
			name: "git repository",
			locked: &Locked{
				Type: "git",
				URL:  "https://example.com/repo.git",
			},
			expected: "https://example.com/repo.git",
		},
		{
			name: "path repository",
			locked: &Locked{
				Type: "path",
				Path: "/local/path",
			},
			expected: "/local/path",
		},
		{
			name: "tarball repository",
			locked: &Locked{
				Type: "tarball",
				URL:  "https://example.com/archive.tar.gz",
			},
			expected: "https://example.com/archive.tar.gz",
		},
		{
			name:     "nil locked",
			locked:   nil,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildFlakeURL(tc.locked)
			if result != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestCheckInputUpdate(t *testing.T) {
	t.Run("nonexistent input", func(t *testing.T) {
		flakeLock := FlakeLock{
			Nodes: map[string]Node{
				"root": {
					Inputs: map[string]any{},
				},
			},
			Root: "root",
		}

		update := checkInputUpdate(flakeLock, "nonexistent", "missing", false)

		if update.InputName != "nonexistent" {
			t.Errorf("expected input name 'nonexistent', got '%s'", update.InputName)
		}

		if update.Error == "" {
			t.Error("expected error for missing input")
		}
	})

	t.Run("input without locked version", func(t *testing.T) {
		flakeLock := FlakeLock{
			Nodes: map[string]Node{
				"no-lock": {
					Original: &Original{
						Owner: "someone",
						Repo:  "something",
					},
				},
				"root": {
					Inputs: map[string]any{
						"no-lock": "no-lock",
					},
				},
			},
			Root: "root",
		}

		update := checkInputUpdate(flakeLock, "no-lock", "no-lock", false)

		if update.InputName != "no-lock" {
			t.Errorf("expected input name 'no-lock', got '%s'", update.InputName)
		}

		if update.Error == "" {
			t.Error("expected error for input without locked version")
		}
	})

	t.Run("valid input structure", func(t *testing.T) {
		flakeLock := FlakeLock{
			Nodes: map[string]Node{
				"nixpkgs": {
					Locked: &Locked{
						LastModified: 1759381078,
						NarHash:      "sha256-abc",
						Owner:        "NixOS",
						Repo:         "nixpkgs",
						Rev:          "abcdef1234567890",
						Type:         "github",
					},
				},
				"root": {
					Inputs: map[string]any{
						"nixpkgs": "nixpkgs",
					},
				},
			},
			Root: "root",
		}

		t.Skip("Skipping test that requires nix command in CI environment")

		update := checkInputUpdate(flakeLock, "nixpkgs", "nixpkgs", false)

		if update.InputName != "nixpkgs" {
			t.Errorf("expected input name 'nixpkgs', got '%s'", update.InputName)
		}

		if update.CurrentRev != "abcdef1234567890" {
			t.Errorf("expected current rev 'abcdef1234567890', got '%s'", update.CurrentRev)
		}

		if update.CurrentURL != "github:NixOS/nixpkgs" {
			t.Errorf("expected current URL 'github:NixOS/nixpkgs', got '%s'", update.CurrentURL)
		}
	})
}

func TestCheckUpdates(t *testing.T) {
	t.Run("multiple inputs", func(t *testing.T) {
		t.Skip("Skipping test that requires nix command in CI environment")

		flakeLock := FlakeLock{
			Nodes: map[string]Node{
				"nixpkgs": {
					Locked: &Locked{
						LastModified: 1759381078,
						NarHash:      "sha256-abc",
						Owner:        "NixOS",
						Repo:         "nixpkgs",
						Rev:          "abcdef1234567890",
						Type:         "github",
					},
				},
				"home-manager": {
					Locked: &Locked{
						LastModified: 1759381077,
						NarHash:      "sha256-hm",
						Owner:        "nix-community",
						Repo:         "home-manager",
						Rev:          "hm4567890123456",
						Type:         "github",
					},
				},
				"root": {
					Inputs: map[string]any{
						"nixpkgs":      "nixpkgs",
						"home-manager": "home-manager",
					},
				},
			},
			Root: "root",
		}

		results, err := CheckUpdates(flakeLock, false)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(results.Updates) != 2 {
			t.Errorf("expected 2 updates, got %d", len(results.Updates))
		}

		// Check that we have both inputs
		inputNames := make(map[string]bool)
		for _, update := range results.Updates {
			inputNames[update.InputName] = true
		}

		if !inputNames["nixpkgs"] {
			t.Error("missing nixpkgs input")
		}
		if !inputNames["home-manager"] {
			t.Error("missing home-manager input")
		}
	})

	t.Run("no root inputs", func(t *testing.T) {
		emptyLock := FlakeLock{
			Nodes: map[string]Node{
				"root": {},
			},
			Root:    "root",
			Version: 7,
		}

		results, err := CheckUpdates(emptyLock, false)

		if err == nil {
			t.Error("expected error for no root inputs")
		}

		if len(results.Updates) != 0 {
			t.Errorf("expected 0 updates, got %d", len(results.Updates))
		}
	})

	t.Run("no root node", func(t *testing.T) {
		noRootLock := FlakeLock{
			Nodes:   map[string]Node{},
			Root:    "root",
			Version: 7,
		}

		results, err := CheckUpdates(noRootLock, false)

		if err == nil {
			t.Error("expected error for no root node")
		}

		if len(results.Updates) != 0 {
			t.Errorf("expected 0 updates, got %d", len(results.Updates))
		}
	})
}
