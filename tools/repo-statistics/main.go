package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	ErrInvalidCommitNumber = errors.New("invalid commit number")
	ErrCommitOutOfRange    = errors.New("commit number out of range")
	ErrInvalidRangeFormat  = errors.New("invalid range format")
	ErrInvalidStartCommit  = errors.New("invalid start commit")
	ErrInvalidEndCommit    = errors.New("invalid end commit")
	ErrStartGreaterThanEnd = errors.New("start commit is greater than end commit")
)

type Commit struct {
	Hash    string
	Date    string
	Subject string
	Added   int
	Removed int
}

func main() {
	repoPath := "."
	commitRange := ""

	// Parse arguments: [repo_path] [commit_range]
	// Examples:
	//   ./repo-statistics
	//   ./repo-statistics .
	//   ./repo-statistics 10-100
	//   ./repo-statistics . 10-100
	if len(os.Args) > 1 {
		arg := os.Args[1]
		// Check if it's a range (contains - or is a number)
		if strings.Contains(arg, "-") || isNumber(arg) {
			commitRange = arg
		} else {
			repoPath = arg
			if len(os.Args) > 2 {
				commitRange = os.Args[2]
			}
		}
	}

	// Change to repo directory
	if err := os.Chdir(repoPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Cannot access repository at %s: %v\n", repoPath, err)
		os.Exit(1)
	}

	fmt.Println("Fetching git statistics...")
	commits, err := getGitStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching git stats: %v\n", err)
		os.Exit(1)
	}

	if len(commits) == 0 {
		fmt.Println("No commits found in repository")
		os.Exit(0)
	}

	fmt.Printf("Found %d commits\n", len(commits))

	// Apply commit range filter
	var start, end int
	if commitRange != "" {
		var err error
		start, end, err = parseRange(commitRange, len(commits))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing range: %v\n", err)
			fmt.Fprintf(os.Stderr, "Usage: repo-statistics [path] [range]\n")
			fmt.Fprintf(os.Stderr, "Examples: 10-100, 10-, -100, 50\n")
			os.Exit(1)
		}
		commits = commits[start-1 : end]
		fmt.Printf("Analyzing commits %d to %d (%d commits)\n", start, end, len(commits))
	}

	// Calculate cumulative stats
	totalAdded := 0
	totalRemoved := 0

	// Create secure temporary data file
	f, err := os.CreateTemp("", "repo_stats_*.dat")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating data file: %v\n", err)
		os.Exit(1)
	}
	dataFile := f.Name()
	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "Error closing data file: %v\n", closeErr)
		}
		_ = os.Remove(dataFile)
	}()

	for i, commit := range commits {
		totalAdded += commit.Added
		totalRemoved += commit.Removed
		if _, err := fmt.Fprintf(f, "%d %s %d %d %d %d\n",
			i+1, commit.Date, commit.Added, commit.Removed, totalAdded, totalRemoved); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to data file: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Total Lines Added: %d\n", totalAdded)
	fmt.Printf("Total Lines Removed: %d\n", totalRemoved)
	fmt.Printf("Net Change: %d lines\n", totalAdded-totalRemoved)

	// Determine date range
	startDate := commits[0].Date
	endDate := commits[len(commits)-1].Date

	// Create output directory
	outputDir := ".github/images"
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Output to a fixed path that can be referenced in README
	outputPath := filepath.Join(outputDir, "repo_statistics.png")

	summaryFile, err := os.CreateTemp("", "repo_stats_summary_*.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating summary file: %v\n", err)
		os.Exit(1)
	}
	summaryPath := summaryFile.Name()
	_ = summaryFile.Close()
	defer func() { _ = os.Remove(summaryPath) }()

	// Create gnuplot script
	scriptPath, err := createGnuplotScript(len(commits), totalAdded, totalRemoved, startDate, endDate, dataFile, outputPath, summaryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating gnuplot script: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = os.Remove(scriptPath) }()

	// Validate gnuplot executable exists in PATH
	gnuplotPath, err := exec.LookPath("gnuplot")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: gnuplot not found in PATH: %v\n", err)
		os.Exit(1)
	}

	// Run gnuplot with validated paths
	// gnuplotPath is from LookPath (system executable)
	// scriptPath is internally generated secure temp file
	fmt.Println("Generating graph with gnuplot...")
	cmd := exec.Command(gnuplotPath, scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running gnuplot: %v\n%s\n", err, output)
		os.Exit(1)
	}

	fmt.Println(string(output))
	fmt.Printf("\n✓ Graph saved to %s\n", outputPath)

	// Try to open the image
	openImage(outputPath)
}

func getGitStats() ([]Commit, error) {
	cmd := exec.Command("git", "log", "--numstat", "--pretty=format:%H|%ad|%s",
		"--date=short", "--reverse")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	var commits []Commit
	var currentCommit *Commit

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check if it's a commit line (contains | but not tab)
		if strings.Contains(line, "|") && !strings.Contains(line, "\t") {
			// Save previous commit
			if currentCommit != nil {
				commits = append(commits, *currentCommit)
			}

			// Parse new commit
			parts := strings.SplitN(line, "|", 3)
			currentCommit = &Commit{
				Hash:    parts[0],
				Date:    parts[1],
				Subject: "",
				Added:   0,
				Removed: 0,
			}
			if len(parts) > 2 {
				currentCommit.Subject = parts[2]
			}
		} else if currentCommit != nil && strings.Contains(line, "\t") {
			// Parse file stat line
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				added, removed := parts[0], parts[1]

				// Handle binary files (marked with -)
				if added != "-" {
					if n, err := strconv.Atoi(added); err == nil {
						currentCommit.Added += n
					}
				}
				if removed != "-" {
					if n, err := strconv.Atoi(removed); err == nil {
						currentCommit.Removed += n
					}
				}
			}
		}
	}

	// Add last commit
	if currentCommit != nil {
		commits = append(commits, *currentCommit)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return commits, nil
}

func createGnuplotScript(numCommits, totalAdded, totalRemoved int, startDate, endDate, dataFile, outputPath, summaryPath string) (string, error) {
	script := fmt.Sprintf(`set terminal pngcairo size 1600,1000 enhanced font 'Arial,11'
set output '%s'

set multiplot layout 2,1 title "Git Repository Statistics" font ",18"

# Top plot - Cumulative lines
set xlabel "Commit Number" font ",13"
set ylabel "Lines of Code" font ",13"
set title "Cumulative Lines Added and Removed Over Time" font ",15"
set grid
set key left top

set style fill transparent solid 0.3
set style line 1 lc rgb '#00AA00' lt 1 lw 2 pt 7 ps 0.5
set style line 2 lc rgb '#DD0000' lt 1 lw 2 pt 7 ps 0.5

plot '%s' using 1:5 with filledcurves x1 lc rgb '#00AA00' notitle, \
     '' using 1:5 with linespoints ls 1 title 'Lines Added (Cumulative)', \
     '' using 1:6 with filledcurves x1 lc rgb '#DD0000' notitle, \
     '' using 1:6 with linespoints ls 2 title 'Lines Removed (Cumulative)'

# Bottom plot - Per-commit changes
set xlabel "Commit Number" font ",13"
set ylabel "Lines Changed" font ",13"
set title "Lines Added/Removed Per Commit" font ",15"
set grid ytics
set key left top

set style fill solid 0.7
set boxwidth 0.8
set style line 3 lc rgb '#00AA00'
set style line 4 lc rgb '#DD0000'

plot '%s' using 1:3 with boxes ls 3 title 'Lines Added', \
     '' using 1:(-$4) with boxes ls 4 title 'Lines Removed'

unset multiplot

# Summary info
set print "%s"
print sprintf("Repository Statistics")
print sprintf("=====================")
print sprintf("Total Commits: %%d", %d)
print sprintf("Total Lines Added: %%d", %d)
print sprintf("Total Lines Removed: %%d", %d)
print sprintf("Net Change: %%d lines", %d)
print sprintf("Period: %%s to %%s", "%s", "%s")
`, outputPath, dataFile, dataFile, summaryPath, numCommits, totalAdded, totalRemoved, totalAdded-totalRemoved, startDate, endDate)

	// Create secure temporary script file
	scriptFile, err := os.CreateTemp("", "repo_stats_*.gnuplot")
	if err != nil {
		return "", fmt.Errorf("failed to create script file: %w", err)
	}
	scriptPath := scriptFile.Name()

	// Write script with secure permissions
	if _, err := scriptFile.WriteString(script); err != nil {
		_ = scriptFile.Close()
		_ = os.Remove(scriptPath)
		return "", fmt.Errorf("failed to write script: %w", err)
	}

	if err := scriptFile.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return "", fmt.Errorf("failed to close script file: %w", err)
	}

	return scriptPath, nil
}

func openImage(path string) {
	// Validate the path is an absolute path and exists
	absPath, err := filepath.Abs(path)
	if err != nil || absPath != path {
		// Path is not absolute or invalid, don't open
		return
	}
	if _, err := os.Stat(absPath); err != nil {
		// File doesn't exist, don't open
		return
	}

	// Try different commands to open the image
	// These are well-known, safe system commands for opening files
	// absPath has been validated to be absolute and to exist
	commands := []string{"xdg-open", "open", "start", "eog", "feh"}

	for _, cmdName := range commands {
		if _, err := exec.LookPath(cmdName); err == nil {
			// Launch system image viewer with validated absolute path
			// cmdName is from our safe list, absPath is validated above
			go func(name, path string) {
				_ = exec.Command(name, path).Start()
			}(cmdName, absPath)

			return
		}
	}
}

func isNumber(s string) bool {
	_, err := strconv.Atoi(s)

	return err == nil
}

func parseRange(rangeStr string, maxCommits int) (start, end int, err error) {
	rangeStr = strings.TrimSpace(rangeStr)

	// Handle single number (start from that commit to end)
	if !strings.Contains(rangeStr, "-") {
		start, err = strconv.Atoi(rangeStr)
		if err != nil {
			return 0, 0, fmt.Errorf("%w: %s", ErrInvalidCommitNumber, rangeStr)
		}
		if start < 1 || start > maxCommits {
			return 0, 0, fmt.Errorf("%w: %d (1-%d)", ErrCommitOutOfRange, start, maxCommits)
		}

		return start, maxCommits, nil
	}

	// Handle range (start-end)
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("%w: %s (use start-end)", ErrInvalidRangeFormat, rangeStr)
	}

	// Parse start
	if parts[0] == "" {
		start = 1
	} else {
		start, err = strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return 0, 0, fmt.Errorf("%w: %s", ErrInvalidStartCommit, parts[0])
		}
	}

	// Parse end
	if parts[1] == "" {
		end = maxCommits
	} else {
		end, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return 0, 0, fmt.Errorf("%w: %s", ErrInvalidEndCommit, parts[1])
		}
	}

	// Validate range
	if start < 1 || start > maxCommits {
		return 0, 0, fmt.Errorf("%w: start %d (1-%d)", ErrCommitOutOfRange, start, maxCommits)
	}
	if end < 1 || end > maxCommits {
		return 0, 0, fmt.Errorf("%w: end %d (1-%d)", ErrCommitOutOfRange, end, maxCommits)
	}
	if start > end {
		return 0, 0, fmt.Errorf("%w: %d > %d", ErrStartGreaterThanEnd, start, end)
	}

	return start, end, nil
}
