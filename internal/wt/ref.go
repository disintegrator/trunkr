package wt

import (
	"errors"
	"fmt"
	"strings"
)

// PRRef normalizes user input naming a pull request into a wt switch ref:
// "123" or "#123" become "pr:123"; "pr:123" / "mr:45" pass through; an
// http(s) URL passes through, since wt resolves PR URLs itself.
func PRRef(input string) (string, error) {
	s := strings.TrimPrefix(strings.TrimSpace(input), "#")
	if s == "" {
		return "", errors.New("empty pull request reference")
	}
	if allDigits(s) {
		return "pr:" + s, nil
	}
	lower := strings.ToLower(s)
	for _, prefix := range []string{"pr:", "mr:"} {
		if strings.HasPrefix(lower, prefix) && allDigits(lower[len(prefix):]) {
			return lower, nil
		}
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return s, nil
	}
	return "", fmt.Errorf("cannot parse %q as a pull request: use a number, pr:N, or a PR URL", input)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
