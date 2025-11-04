package util

import (
	"os"
	"testing"
)

func TestIsNoColor(t *testing.T) {
	testCases := []struct {
		name     string
		envVar   string
		envValue string
		expected bool
	}{
		{
			name:     "NO_COLOR not set",
			envVar:   "NO_COLOR",
			envValue: "", // will be unset
			expected: false,
		},
		{
			name:     "NO_COLOR set to any value",
			envVar:   "NO_COLOR",
			envValue: "1",
			expected: true,
		},
		{
			name:     "NO_COLOR set to empty string",
			envVar:   "NO_COLOR",
			envValue: "",
			expected: false, // empty string is treated as unset
		},
		{
			name:     "NO_COLOR set to 0",
			envVar:   "NO_COLOR",
			envValue: "0",
			expected: true, // any value means env var exists
		},
		{
			name:     "NO_COLOR set to false",
			envVar:   "NO_COLOR",
			envValue: "false",
			expected: true, // any value means env var exists
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv("NO_COLOR")

			// Set env var if specified
			if tc.envVar != "" && tc.envValue != "" {
				os.Setenv(tc.envVar, tc.envValue)
			}

			// Call function and check result
			result := IsNoColor()
			if result != tc.expected {
				t.Errorf("expected IsNoColor() to return %v, got %v", tc.expected, result)
			}

			// Clean up
			os.Unsetenv(tc.envVar)
		})
	}
}
