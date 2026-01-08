package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Fixed list of linters to track (alphabetically sorted)
var trackedLinters = []string{
	"bodyclose",
	"containedctx",
	"contextcheck",
	"cyclop",
	"depguard",
	"dupl",
	"err113",
	"errcheck",
	"errchkjson",
	"exhaustive",
	"exhaustruct",
	"forbidigo",
	"forcetypeassert",
	"funcorder",
	"funlen",
	"gochecknoglobals",
	"gochecknoinits",
	"gocognit",
	"goconst",
	"gocritic",
	"gocyclo",
	"godox",
	"gosec",
	"ireturn",
	"lll",
	"mnd",
	"modernize",
	"nestif",
	"noctx",
	"noinlineerr",
	"paralleltest",
	"revive",
	"staticcheck",
	"tagliatelle",
	"testifylint",
	"testpackage",
	"unconvert",
	"unparam",
	"usetesting",
	"varnamelen",
	"wrapcheck",
}

func main() {
	snapshot := flag.Bool("snapshot", false, "Run linter and add snapshot to CSV")
	snapshotShort := flag.Bool("s", false, "Run linter and add snapshot to CSV (short)")
	flag.Parse()

	csvPath := "linter.csv"

	if *snapshot || *snapshotShort {
		fmt.Println("Running golangci-lint to capture snapshot...")
		if err := captureSnapshot(csvPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error capturing snapshot: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Snapshot added to", csvPath)
	}

	// Generate visualization
	fmt.Println("Generating visualization...")
	if err := visualize(csvPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating visualization: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Visualization saved to /tmp/linter_stats.png")
}

func captureSnapshot(csvPath string) error {
	// Run golangci-lint
	cmd := exec.Command("golangci-lint", "run", "./...", "--timeout=5m")
	output, _ := cmd.CombinedOutput() // Ignore error - linter returns non-zero when issues found

	// Parse the output for issue counts
	counts := parseLinterOutput(string(output))

	// Read existing CSV
	file, err := os.Open(csvPath)
	if err != nil {
		// If CSV doesn't exist, create it with headers
		return initializeCSV(csvPath, counts)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) == 0 {
		// Empty CSV, initialize it
		file.Close()
		return initializeCSV(csvPath, counts)
	}

	// Build new row based on fixed linter list
	newRow := make([]string, len(trackedLinters))
	for i, linter := range trackedLinters {
		if count, ok := counts[linter]; ok {
			newRow[i] = strconv.Itoa(count)
		} else {
			newRow[i] = "0"
		}
	}

	// Append new row
	records = append(records, newRow)

	// Write back to CSV
	file.Close()
	outFile, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	if err := writer.WriteAll(records); err != nil {
		return fmt.Errorf("failed to write CSV: %w", err)
	}
	writer.Flush()

	return nil
}

func initializeCSV(csvPath string, counts map[string]int) error {
	// Create new CSV with headers and first data row
	records := [][]string{
		trackedLinters, // Header row
	}

	// Add first data row
	dataRow := make([]string, len(trackedLinters))
	for i, linter := range trackedLinters {
		if count, ok := counts[linter]; ok {
			dataRow[i] = strconv.Itoa(count)
		} else {
			dataRow[i] = "0"
		}
	}
	records = append(records, dataRow)

	// Write to CSV
	outFile, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	if err := writer.WriteAll(records); err != nil {
		return fmt.Errorf("failed to write CSV: %w", err)
	}
	writer.Flush()

	return nil
}

func parseLinterOutput(output string) map[string]int {
	counts := make(map[string]int)

	// Look for the summary section at the end
	// Format: "* lintername: count"
	re := regexp.MustCompile(`^\* (\w+): (\d+)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			linter := matches[1]
			count, _ := strconv.Atoi(matches[2])
			counts[linter] = count
		}
	}

	return counts
}

func visualize(csvPath string) error {
	// Read CSV
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("need at least header and one data row")
	}

	headers := records[0]
	dataRows := records[1:]

	// Calculate top issues to show
	topLinters := getTopLinters(headers, dataRows)

	// Generate gnuplot script
	if err := generateGnuplotScript(headers, dataRows, topLinters); err != nil {
		return err
	}

	// Run gnuplot
	cmd := exec.Command("gnuplot", "/tmp/linter_stats.gnuplot")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gnuplot failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func getTopLinters(headers []string, dataRows [][]string) []string {
	// Get the latest row (last row)
	if len(dataRows) == 0 {
		return nil
	}

	latestRow := dataRows[len(dataRows)-1]

	// Create map of linter -> count
	type linterCount struct {
		name  string
		count int
	}

	var linters []linterCount
	for i, header := range headers {
		if i < len(latestRow) {
			count, _ := strconv.Atoi(latestRow[i])
			if count > 0 {
				linters = append(linters, linterCount{name: header, count: count})
			}
		}
	}

	// Sort by count descending
	for i := 0; i < len(linters)-1; i++ {
		for j := i + 1; j < len(linters); j++ {
			if linters[j].count > linters[i].count {
				linters[i], linters[j] = linters[j], linters[i]
			}
		}
	}

	// Take top 10
	top := 10
	if len(linters) < top {
		top = len(linters)
	}

	result := make([]string, top)
	for i := 0; i < top; i++ {
		result[i] = linters[i].name
	}

	return result
}

func generateGnuplotScript(headers []string, dataRows [][]string, topLinters []string) error {
	script := `
set terminal pngcairo size 1400,900 enhanced font 'Arial,12'
set output '/tmp/linter_stats.png'

set title "Linter Issues Over Time" font 'Arial,16'
set xlabel "Snapshot"
set ylabel "Issue Count"
set grid

set key outside right top

set style data linespoints
set pointsize 1.5

# Use snapshot numbers as x-axis
set xtics 1

`

	// Build plot command
	var plotLines []string
	for _, linter := range topLinters {
		// Find column index for this linter
		colIdx := -1
		for i, h := range headers {
			if h == linter {
				colIdx = i + 1 // gnuplot columns are 1-indexed
				break
			}
		}

		if colIdx > 0 {
			plotLines = append(plotLines, fmt.Sprintf("'-' using 1:%d with linespoints title '%s'", colIdx, linter))
		}
	}

	script += "plot " + strings.Join(plotLines, ", \\\n     ") + "\n\n"

	// Add data for each linter
	for range topLinters {
		for i, row := range dataRows {
			snapshot := i + 1
			script += fmt.Sprintf("%d", snapshot)
			for _, val := range row {
				script += fmt.Sprintf(" %s", val)
			}
			script += "\n"
		}
		script += "e\n"
	}

	return os.WriteFile("/tmp/linter_stats.gnuplot", []byte(script), 0o600)
}
