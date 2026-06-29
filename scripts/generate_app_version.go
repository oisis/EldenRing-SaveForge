package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const outputPath = "app_version_generated.go"

func main() {
	version, err := readMakefileVersion("Makefile")
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate app version: %v\n", err)
		os.Exit(1)
	}

	content := fmt.Sprintf(`package main

func init() {
	appVersion = %q
}
`, version)

	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "generate app version: write %s: %v\n", outputPath, err)
		os.Exit(1)
	}
}

func readMakefileVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`(?m)^VERSION\s*=\s*(\S+)\s*$`)
	match := re.FindStringSubmatch(string(data))
	if len(match) != 2 {
		return "", fmt.Errorf("VERSION not found in %s", path)
	}

	version := strings.TrimSpace(match[1])
	if version == "" {
		return "", fmt.Errorf("VERSION is empty in %s", path)
	}
	return version, nil
}
