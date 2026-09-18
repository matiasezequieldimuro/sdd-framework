package infra

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"sdd-cli/embeds"
	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type FSAdapterRepository struct{}

type adapterMetadata struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

type adapterFile struct {
	target string
	data   []byte
}

func NewFSAdapterRepository() *FSAdapterRepository {
	return &FSAdapterRepository{}
}

func (repository *FSAdapterRepository) ListAdapters() ([]ports.AdapterDescriptor, error) {
	adapterFS, err := adapterResources()
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(adapterFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded adapter catalog: %w", err)
	}

	adapters := make([]ports.AdapterDescriptor, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		descriptor, err := readAdapterDescriptor(adapterFS, entry.Name())
		if err != nil {
			return nil, err
		}
		adapters = append(adapters, descriptor)
	}
	sort.Slice(adapters, func(i, j int) bool {
		return adapters[i].ID < adapters[j].ID
	})
	return adapters, nil
}

func (repository *FSAdapterRepository) InstallAdapter(targetDir string, adapterID string) (*ports.AdapterInstallation, error) {
	adapterFS, err := adapterResources()
	if err != nil {
		return nil, err
	}
	if _, err := readAdapterDescriptor(adapterFS, adapterID); err != nil {
		return nil, err
	}
	if err := requireInitializedProject(targetDir); err != nil {
		return nil, err
	}

	files, err := collectAdapterFiles(adapterFS, adapterID)
	if err != nil {
		return nil, err
	}
	roots := adapterRootTargets(files)
	if err := assertNoInstallConflicts(targetDir, roots); err != nil {
		return nil, err
	}

	stageDir, err := os.MkdirTemp(targetDir, ".sdd-adapter-install-")
	if err != nil {
		return nil, fmt.Errorf("create adapter staging directory: %w", err)
	}
	stageOwned := true
	defer func() {
		if stageOwned {
			_ = os.RemoveAll(stageDir)
		}
	}()

	for _, file := range files {
		stagePath := filepath.Join(stageDir, filepath.FromSlash(file.target))
		if err := os.MkdirAll(filepath.Dir(stagePath), 0755); err != nil {
			return nil, fmt.Errorf("create staging directory for %s: %w", file.target, err)
		}
		if err := writeInitializedFile(stagePath, file.data); err != nil {
			return nil, fmt.Errorf("write staged adapter file %s: %w", file.target, err)
		}
	}

	published := make([]string, 0, len(roots))
	for _, root := range roots {
		targetPath, err := containedPath(targetDir, filepath.FromSlash(root))
		if err != nil {
			rollbackPublishedRoots(targetDir, published)
			return nil, err
		}
		stagePath := filepath.Join(stageDir, filepath.FromSlash(root))
		if err := os.Rename(stagePath, targetPath); err != nil {
			rollbackPublishedRoots(targetDir, published)
			return nil, fmt.Errorf("publish adapter path %s: %w", root, err)
		}
		published = append(published, root)
	}

	installed := make([]string, 0, len(files))
	for _, file := range files {
		installed = append(installed, file.target)
	}
	sort.Strings(installed)
	if err := os.RemoveAll(stageDir); err != nil {
		return nil, fmt.Errorf("cleanup adapter staging directory: %w", err)
	}
	stageOwned = false
	return &ports.AdapterInstallation{ID: adapterID, Files: installed}, nil
}

func adapterResources() (fs.FS, error) {
	adapterFS, err := fs.Sub(embeds.DefaultAdapterResources, "default_adapters")
	if err != nil {
		return nil, fmt.Errorf("access embedded adapters: %w", err)
	}
	return adapterFS, nil
}

func readAdapterDescriptor(adapterFS fs.FS, adapterID string) (ports.AdapterDescriptor, error) {
	if err := domain.ValidateIdentifier("adapter id", adapterID); err != nil {
		return ports.AdapterDescriptor{}, err
	}
	data, err := fs.ReadFile(adapterFS, filepath.ToSlash(filepath.Join(adapterID, "adapter.yaml")))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ports.AdapterDescriptor{}, fmt.Errorf("%w: %s", domain.ErrAdapterNotFound, adapterID)
		}
		return ports.AdapterDescriptor{}, fmt.Errorf("read adapter metadata %s: %w", adapterID, err)
	}
	var metadata adapterMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return ports.AdapterDescriptor{}, fmt.Errorf("parse adapter metadata %s: %w", adapterID, err)
	}
	if metadata.ID != adapterID {
		return ports.AdapterDescriptor{}, fmt.Errorf("%w: adapter metadata id %q does not match directory %q", domain.ErrInvalidIdentifier, metadata.ID, adapterID)
	}
	return ports.AdapterDescriptor{
		ID:          metadata.ID,
		Title:       metadata.Title,
		Description: metadata.Description,
	}, nil
}

func requireInitializedProject(targetDir string) error {
	sddPath, err := containedPath(targetDir, ".sdd")
	if err != nil {
		return err
	}
	info, err := os.Stat(sddPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: .sdd directory not found; run sdd-cli init first", domain.ErrInvalidPath)
		}
		return fmt.Errorf("inspect .sdd directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: .sdd must be a directory", domain.ErrInvalidPath)
	}
	return nil
}

func collectAdapterFiles(adapterFS fs.FS, adapterID string) ([]adapterFile, error) {
	var files []adapterFile
	err := fs.WalkDir(adapterFS, adapterID, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(adapterID, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.ToSlash(relative) == "adapter.yaml" {
			return nil
		}
		data, err := fs.ReadFile(adapterFS, path)
		if err != nil {
			return err
		}
		files = append(files, adapterFile{
			target: filepath.ToSlash(relative),
			data:   data,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect adapter files %s: %w", adapterID, err)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].target < files[j].target
	})
	return files, nil
}

func adapterRootTargets(files []adapterFile) []string {
	seen := make(map[string]struct{})
	for _, file := range files {
		root := strings.SplitN(file.target, "/", 2)[0]
		seen[root] = struct{}{}
	}
	roots := make([]string, 0, len(seen))
	for root := range seen {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

func assertNoInstallConflicts(targetDir string, roots []string) error {
	for _, root := range roots {
		targetPath, err := containedPath(targetDir, filepath.FromSlash(root))
		if err != nil {
			return err
		}
		if _, err := os.Lstat(targetPath); err == nil {
			return fmt.Errorf("%w: %s already exists", domain.ErrAdapterInstallConflict, root)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect adapter target %s: %w", root, err)
		}
	}
	return nil
}

func rollbackPublishedRoots(targetDir string, published []string) {
	for index := len(published) - 1; index >= 0; index-- {
		path, err := containedPath(targetDir, filepath.FromSlash(published[index]))
		if err == nil {
			_ = os.RemoveAll(path)
		}
	}
}
