package infra

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sdd-cli/internal/domain"
)

func containedPath(root string, parts ...string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("%w: resolve root: %v", domain.ErrInvalidPath, err)
	}
	candidateAbs, err := filepath.Abs(filepath.Join(append([]string{rootAbs}, parts...)...))
	if err != nil {
		return "", fmt.Errorf("%w: resolve candidate: %v", domain.ErrInvalidPath, err)
	}
	if !isWithin(rootAbs, candidateAbs) {
		return "", fmt.Errorf("%w: %s escapes %s", domain.ErrInvalidPath, candidateAbs, rootAbs)
	}

	resolvedRoot, err := resolveExistingPath(rootAbs)
	if err != nil {
		return "", err
	}
	resolvedCandidate, err := resolveExistingPath(candidateAbs)
	if err != nil {
		return "", err
	}
	if !isWithin(resolvedRoot, resolvedCandidate) {
		return "", fmt.Errorf("%w: resolved path %s escapes %s", domain.ErrInvalidPath, resolvedCandidate, resolvedRoot)
	}

	return candidateAbs, nil
}

func resolveExistingPath(path string) (string, error) {
	current := path
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			if current == path {
				return resolved, nil
			}
			remainder, relErr := filepath.Rel(current, path)
			if relErr != nil {
				return "", fmt.Errorf("%w: resolve path remainder: %v", domain.ErrInvalidPath, relErr)
			}
			return filepath.Join(resolved, remainder), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("%w: resolve symlinks for %s: %v", domain.ErrInvalidPath, current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path), nil
		}
		current = parent
	}
}

func isWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
