package studentbooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NodeType defines the type of a library node.
type NodeType string

const (
	NodeGrade   NodeType = "grade"
	NodeBook    NodeType = "book"
	NodeChapter NodeType = "chapter"
)

// Node represents a generic item in the library tree (grade, book, chapter, or section).
type Node struct {
	Name string   `json:"name"`
	Type NodeType `json:"type"`
	Path string   `json:"path"`
}

// BookMetadata contains information about a book found during search.
type BookMetadata struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Grade   string `json:"grade"`
	Path    string `json:"path"`
	Summary string `json:"summary"` // Short description of the book
}

// Service defines the interface for managing and discovering student books.
type Service interface {
	// ListNodes returns all items (grades, books, chapters, or sections) at the given path.
	ListNodes(path string) ([]Node, error)
	// GetChapterContent reads the text content of a specific resource.
	GetChapterContent(path string) (string, error)
}

// FileSystemService implements Service by reading from the local file system.
type FileSystemService struct {
	baseDir string
}

// NewFileSystemService creates a new FileSystemService.
func NewFileSystemService(baseDir string) *FileSystemService {
	return &FileSystemService{
		baseDir: baseDir,
	}
}

// ListNodes lists items at a given path. If path is a file, it returns headings.
func (s *FileSystemService) ListNodes(path string) ([]Node, error) {
	// Security check
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("invalid path")
	}

	fullPath := filepath.Join(s.baseDir, path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access path: %w", err)
	}

	if !info.IsDir() {
		return s.extractHeadings(path, fullPath)
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var nodes []Node
	for _, entry := range entries {
		// Skip hidden files/dirs, library index, and metadata
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "LIBRARY_INDEX.md" || entry.Name() == "metadata.json" {
			continue
		}

		node := Node{
			Name: entry.Name(),
			Path: filepath.Clean(filepath.Join(path, entry.Name())),
		}

		// Determine type based on depth/extension
		parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
		if path == "" || path == "." {
			node.Type = NodeGrade
		} else if len(parts) == 1 {
			node.Type = NodeBook
		} else {
			if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				node.Type = NodeChapter
			} else {
				node.Type = "directory" // internal/images etc
			}
		}

		// Only return relevant types for discovery
		if node.Type != "directory" {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

// extractHeadings reads a markdown file and returns its heading structure as nodes.
func (s *FileSystemService) extractHeadings(relPath, fullPath string) ([]Node, error) {
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	var nodes []Node
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			// Extract heading level and title
			level := 0
			for i := 0; i < len(line) && line[i] == '#'; i++ {
				level++
			}
			title := strings.TrimSpace(line[level:])
			if title == "" {
				continue
			}

			nodes = append(nodes, Node{
				Name: title,
				Type: NodeType(fmt.Sprintf("heading-%d", level)),
				Path: relPath, // Heading is within the file
			})
		}
	}
	return nodes, nil
}

// GetChapterContent reads the content of a chapter file.
func (s *FileSystemService) GetChapterContent(path string) (string, error) {
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("invalid path")
	}

	fullPath := filepath.Join(s.baseDir, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}
