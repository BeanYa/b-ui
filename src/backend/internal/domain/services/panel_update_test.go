package service

import "testing"

func TestCompareReleaseTags(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		target   string
		expected string
	}{
		{name: "older current version", current: "v0.1.5", target: "v0.1.6", expected: "older"},
		{name: "same version", current: "v0.1.6", target: "v0.1.6", expected: "same"},
		{name: "newer current version", current: "v0.1.7", target: "v0.1.6", expected: "newer"},
		{name: "ignores tag prefix", current: "0.1.6", target: "v0.1.6", expected: "same"},
		{name: "compares missing patch segment", current: "v0.1", target: "v0.1.1", expected: "older"},
		{name: "prerelease stays below stable", current: "v0.1.6-beta.1", target: "v0.1.6", expected: "older"},
		{name: "invalid version returns unknown", current: "dev-build", target: "v0.1.6", expected: "unknown"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := compareReleaseTags(testCase.current, testCase.target); actual != testCase.expected {
				t.Fatalf("compareReleaseTags(%q, %q) = %q, want %q", testCase.current, testCase.target, actual, testCase.expected)
			}
		})
	}
}
