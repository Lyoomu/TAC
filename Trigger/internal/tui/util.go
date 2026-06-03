package tui

import "strings"

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// padToHeight ensures the rendered content is exactly `height` lines tall.
// If content is shorter, blank lines are appended.
// If content is longer, it is truncated from the bottom.
func padToHeight(content string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	// Remove trailing empty line from final \n if present
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// minInt returns the smaller of a, b.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
