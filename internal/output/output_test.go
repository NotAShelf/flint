package output

import (
	"strings"
	"testing"
)

func TestValidateOutputFormat(t *testing.T) {
	testCases := []struct {
		name        string
		format      string
		expectError bool
	}{
		{
			name:        "valid json format",
			format:      "json",
			expectError: false,
		},
		{
			name:        "valid plain format",
			format:      "plain",
			expectError: false,
		},
		{
			name:        "valid pretty format",
			format:      "pretty",
			expectError: false,
		},
		{
			name:        "invalid format",
			format:      "invalid",
			expectError: true,
		},
		{
			name:        "empty format",
			format:      "",
			expectError: true,
		},
		{
			name:        "case sensitive json",
			format:      "JSON",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateOutputFormat(tc.format)
			if tc.expectError && err == nil {
				t.Errorf("expected error for format '%s', got nil", tc.format)
			}
			if !tc.expectError && err != nil {
				t.Errorf("expected no error for format '%s', got: %v", tc.format, err)
			}

			// Test that error message contains valid formats
			if tc.expectError && err != nil {
				errorMsg := err.Error()
				if !strings.Contains(errorMsg, "json, plain, pretty") {
					t.Errorf("error message should list valid formats, got: %s", errorMsg)
				}
			}
		})
	}
}

func TestShouldFailOnDuplicates(t *testing.T) {
	testCases := []struct {
		name       string
		options    Options
		deps       map[string][]string
		shouldFail bool
	}{
		{
			name: "fail flag disabled",
			options: Options{
				FailIfMultipleVersions: false,
			},
			deps: map[string][]string{
				"github:owner/repo1?rev=abc": {"node1"},
				"github:owner/repo1?rev=def": {"node2"},
			},
			shouldFail: false,
		},
		{
			name: "fail flag enabled with duplicates",
			options: Options{
				FailIfMultipleVersions: true,
			},
			deps: map[string][]string{
				"github:owner/repo1?rev=abc": {"node1"},
				"github:owner/repo1?rev=def": {"node2"},
			},
			shouldFail: true,
		},
		{
			name: "fail flag enabled with no duplicates",
			options: Options{
				FailIfMultipleVersions: true,
			},
			deps: map[string][]string{
				"github:owner/repo1?rev=abc": {"node1"},
				"github:owner/repo2?rev=def": {"node2"},
			},
			shouldFail: false,
		},
		{
			name: "empty dependencies",
			options: Options{
				FailIfMultipleVersions: true,
			},
			deps:       map[string][]string{},
			shouldFail: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ShouldFailOnDuplicates(tc.options, tc.deps)
			if result != tc.shouldFail {
				t.Errorf("expected ShouldFailOnDuplicates to return %v, got %v", tc.shouldFail, result)
			}
		})
	}
}
