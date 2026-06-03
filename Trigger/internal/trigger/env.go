package trigger

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ResolveEnv(env map[string]string, basePath string) (map[string]string, error) {
	result := make(map[string]string, len(env))

	for key, val := range env {
		resolved, err := resolveValue(val, basePath)
		if err != nil {
			return nil, fmt.Errorf("resolve env '%s': %w", key, err)
		}
		result[key] = resolved
	}

	return result, nil
}

func safePath(basePath, rel string) (string, error) {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("resolve base path: %w", err)
	}
	joined := filepath.Join(absBase, rel)
	cleaned := filepath.Clean(joined)
	if !strings.HasPrefix(cleaned, absBase+string(filepath.Separator)) && cleaned != absBase {
		return "", fmt.Errorf("path traversal detected: %s escapes base directory", rel)
	}
	return cleaned, nil
}

func resolveValue(val, basePath string) (string, error) {
	if !strings.HasPrefix(val, "file:") {

		return val, nil
	}

	rest := strings.TrimPrefix(val, "file:")
	parts := strings.LastIndex(rest, ":")
	if parts == -1 || parts == 0 {

		path, err := safePath(basePath, rest)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read file '%s': %w", path, err)
		}
		return string(data), nil
	}

	path, err := safePath(basePath, rest[:parts])
	if err != nil {
		return "", err
	}
	lineSpec := rest[parts+1:]

	lineCount, err := strconv.Atoi(lineSpec)
	if err != nil {

		path, err = safePath(basePath, rest)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read file '%s': %w", path, err)
		}
		return string(data), nil
	}

	if lineCount >= 0 {
		return readFileHead(path, lineCount)
	}
	return readFileTail(path, -lineCount)
}

func readFileHead(path string, n int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	count := 0
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		count++
		if count >= n {
			break
		}
	}
	return strings.Join(lines, "\n"), scanner.Err()
}

func readFileTail(path string, n int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) <= n {
		return strings.Join(lines, "\n"), scanner.Err()
	}
	return strings.Join(lines[len(lines)-n:], "\n"), scanner.Err()
}
