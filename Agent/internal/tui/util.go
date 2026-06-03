package tui

import "strings"

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func wrapText(s string, width int) string {
	if width <= 0 {
		return s
	}
	var result []string
	paragraphs := strings.Split(s, "\n")
	for _, para := range paragraphs {
		if para == "" {
			result = append(result, "")
			continue
		}
		var line string
		words := strings.Fields(para)
		for _, word := range words {
			if line == "" {
				line = word
			} else if len(line)+1+len(word) <= width {
				line += " " + word
			} else {
				result = append(result, line)
				line = word
			}
		}
		if line != "" {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}
