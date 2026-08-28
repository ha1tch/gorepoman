package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Matches file targets like `go.mod`, `cmd/repoman/main.go`, `pkg/config/config.go`
// in headers (with or without backticks and numbering).
var fileHeaderRe = regexp.MustCompile(`(?i)(?:^|[\s#` + "`" + `])([a-zA-Z0-9_./-]+\.(?:go|mod))(?:[\s` + "`" + `]|$)`)

func extractFromFile(mdPath, targetDir string) (int, error) {
	f, err := os.Open(mdPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var (
		pendingPath string
		currentPath string
		inCodeBlock bool
		codeBuffer  strings.Builder
		count       int
	)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Look for heading or marker specifying a target file path
		if !inCodeBlock {
			if matches := fileHeaderRe.FindStringSubmatch(line); len(matches) > 1 {
				// Normalize path (e.g. converting backslashes if any)
				candidate := filepath.Clean(filepath.ToSlash(matches[1]))
				// Exclude any false positives
				if strings.HasSuffix(candidate, ".go") || strings.HasSuffix(candidate, "go.mod") {
					pendingPath = candidate
				}
			}
		}

		// Detect Markdown code fence start
		if !inCodeBlock && strings.HasPrefix(trimmed, "```") {
			if pendingPath != "" {
				inCodeBlock = true
				currentPath = pendingPath
				pendingPath = ""
				codeBuffer.Reset()
				continue
			}
		} else if inCodeBlock && strings.HasPrefix(trimmed, "```") {
			// Code fence closed — flush the code buffer to destination
			inCodeBlock = false
			outPath := filepath.Join(targetDir, currentPath)

			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
				return count, fmt.Errorf("failed to create directory for %s: %w", outPath, err)
			}

			// Write source file
			content := codeBuffer.String()
			if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
				return count, fmt.Errorf("failed to write %s: %w", outPath, err)
			}

			fmt.Printf("  [+] Extracted: %s (%d bytes)\n", filepath.Join(targetDir, currentPath), len(content))
			count++
			currentPath = ""
			codeBuffer.Reset()
			continue
		}

		if inCodeBlock {
			codeBuffer.WriteString(line)
			codeBuffer.WriteString("\n")
		}
	}

	return count, scanner.Err()
}

func main() {
	outDir := flag.String("out", "repoman", "Target output directory for the project")
	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		// Default to part1.md through part5.md if no files are explicitly passed
		for i := 1; i <= 5; i++ {
			filename := fmt.Sprintf("part%d.md", i)
			if _, err := os.Stat(filename); err == nil {
				files = append(files, filename)
			}
		}
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No markdown input files found.")
		fmt.Fprintln(os.Stderr, "Usage: go run extract.go [-out <target_dir>] [part1.md part2.md ...]")
		os.Exit(1)
	}

	fmt.Printf("Extracting Go project into '%s/' from %d markdown file(s)...\n\n", *outDir, len(files))

	totalFiles := 0
	for _, mdFile := range files {
		fmt.Printf("Processing %s:\n", mdFile)
		n, err := extractFromFile(mdFile, *outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [!] Error processing %s: %v\n", mdFile, err)
			os.Exit(1)
		}
		totalFiles += n
	}

	fmt.Printf("\nDone! Successfully extracted %d project files into '%s/'.\n", totalFiles, *outDir)
	fmt.Printf("\nTo build and verify:\n")
	fmt.Printf("  cd %s\n", *outDir)
	fmt.Printf("  go build -o repoman ./cmd/repoman\n")
	fmt.Printf("  ./repoman selftest\n")
}
