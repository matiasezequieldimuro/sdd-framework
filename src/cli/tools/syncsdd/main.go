//go:build ignore

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Obtener cwd actual
	cwd, err := os.Getwd()
	if err != nil {
		fail("Failed to get working directory", err)
	}

	// Buscar .sdd subiendo directorios
	sourceSDD := findSDD(cwd)
	if sourceSDD == "" {
		fail("Source .sdd directory not found", nil)
	}

	targetEmbeds := filepath.Join(cwd, "embeds", "default_sdd")

	// Limpiar target
	if err := os.RemoveAll(targetEmbeds); err != nil {
		fail(fmt.Sprintf("Failed to remove %s", targetEmbeds), err)
	}

	// Crear directorio destino
	if err := os.MkdirAll(targetEmbeds, 0755); err != nil {
		fail(fmt.Sprintf("Failed to create %s", targetEmbeds), err)
	}

	fmt.Printf("Synchronizing %s -> %s\n", sourceSDD, targetEmbeds)

	// Copiar desde src/.sdd/ (excluyendo tests y metadatos)
	copyCount := 0
	err = filepath.WalkDir(sourceSDD, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(sourceSDD, path)
		if err != nil {
			return err
		}

		// Procesar raíz
		if rel == "." {
			return nil
		}

		// Excluir .git, .DS_Store y otros metadatos
		// (tests SÍ se copia: la CLI los necesita, e init_uc filtra en proyectos de usuario)
		if strings.HasPrefix(rel, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		target := filepath.Join(targetEmbeds, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		copyCount++
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		fail("Failed to copy .sdd", err)
	}
	fmt.Printf("Copied %d files\n", copyCount)

	// Eliminar work-items (son datos, no template)
	workItemsPath := filepath.Join(targetEmbeds, "work-items")
	if err := os.RemoveAll(workItemsPath); err != nil {
		fail("Failed to remove work-items from embeds", err)
	}

	fmt.Println("✓ Synchronization completed")
}

func fail(msg string, err error) {
	fmt.Fprintf(os.Stderr, "Error: %s: %v\n", msg, err)
	os.Exit(1)
}

// findSDD busca el directorio .sdd subiendo desde cwd
func findSDD(start string) string {
	current := start
	for {
		candidate := filepath.Join(current, ".sdd")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Llegó a la raíz
			return ""
		}
		current = parent
	}
}
