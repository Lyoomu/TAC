package tui

import (
	"sort"
	"strings"
)

func envMapToString(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+env[key])
	}
	return strings.Join(parts, ",")
}

func parseEnvString(input string) map[string]string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	env := make(map[string]string)
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, val, ok := strings.Cut(part, "=")
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if !ok {
			env[key] = ""
			continue
		}
		env[key] = strings.TrimSpace(val)
	}

	if len(env) == 0 {
		return nil
	}
	return env
}
