//go:build ignore

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fail("get working directory", err)
	}

	source := filepath.Clean(filepath.Join(cwd, "..", "adapters"))
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		fail("source adapters directory not found", err)
	}
	target := filepath.Join(cwd, "embeds", "default_adapters")
	if err := os.RemoveAll(target); err != nil {
		fail("remove generated adapters", err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		fail("create generated adapters directory", err)
	}

	count := 0
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		count++
		return os.WriteFile(destination, data, 0644)
	})
	if err != nil {
		fail("copy adapters", err)
	}
	fmt.Printf("Synchronized %d adapter file(s)\n", count)
}

func fail(message string, err error) {
	if err == nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", message)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s: %v\n", message, err)
	}
	os.Exit(1)
}
