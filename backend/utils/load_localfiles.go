package utils

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// visitFile is the callback function called for each file or directory found recursively

// readFilesRecursively reads all files in a directory recursively
func readFilesRecursively(rootDir string, sessionString string, chunkSize int, chunkOverlap int) error {
	visitFile := func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			if filepath.Ext(path) != ".md" {
				return nil
			}
			file, err := os.Open(path)
			if err != nil {
				slog.Error("Error opening file", "path", path, "error", err)
				return err
			}
			defer file.Close()
			text, err := io.ReadAll(file)
			if err != nil {
				slog.Error("Error reading file", "path", path, "error", err)
				return err
			}

			LoadMarkdownToVectorDB(context.Background(), string(text), sessionString, chunkSize, chunkOverlap, path)
			slog.Info("Loaded file", "path", path)
		}
		return nil
	}
	return filepath.Walk(rootDir, visitFile)
}

func LoadLocalFiles(sessionString string, chunkSize int, chunkOverlap int) {
	rootDir := "/localfiles" // Replace with the path to your directory
	if err := readFilesRecursively(rootDir, sessionString, chunkSize, chunkOverlap); err != nil {
		fmt.Println("Error:", err)
	}
}
