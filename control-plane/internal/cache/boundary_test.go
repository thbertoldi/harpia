package cache

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNoRawValkeyBypassOutsideBoundary(t *testing.T) {
	moduleRoot := controlPlaneRoot(t)
	allowed := filepath.Clean("internal/cache/valkey.go")
	rawClientAccess := ".r" + "db."
	rawImport := "github.com/redis/" + "go-redis"

	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "gen" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		rel, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.Clean(rel)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source := string(data)

		if rel != allowed && strings.Contains(source, rawClientAccess) {
			t.Fatalf("raw Valkey client used outside tenant boundary: %s", rel)
		}
		if rel != allowed && strings.Contains(source, rawImport) {
			t.Fatalf("go-redis imported outside tenant boundary: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func controlPlaneRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
}
