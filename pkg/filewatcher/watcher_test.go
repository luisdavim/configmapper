package filewatcher

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestGetFilesFromPathReturnsSingleFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	files, err := getFilesFromPath(file)
	if err != nil {
		t.Fatalf("getFilesFromPath() error = %v", err)
	}

	if !reflect.DeepEqual(files, []string{file}) {
		t.Fatalf("getFilesFromPath() = %v, want %v", files, []string{file})
	}
}

func TestGetFilesFromPathReturnsImmediateFilesOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := filepath.Join(dir, "a.txt")
	second := filepath.Join(dir, "b.txt")
	nestedDir := filepath.Join(dir, "nested")
	nestedFile := filepath.Join(nestedDir, "c.txt")

	if err := os.WriteFile(first, []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", first, err)
	}
	if err := os.WriteFile(second, []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", second, err)
	}
	if err := os.Mkdir(nestedDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.WriteFile(nestedFile, []byte("c"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", nestedFile, err)
	}

	files, err := getFilesFromPath(dir)
	if err != nil {
		t.Fatalf("getFilesFromPath() error = %v", err)
	}

	sort.Strings(files)
	want := []string{first, second}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("getFilesFromPath() = %v, want %v", files, want)
	}
}

func TestGetDataReadsDirectoryFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("one"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "users.yaml"), []byte("two"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "ignored.yaml"), []byte("three"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	data, err := getData(dir)
	if err != nil {
		t.Fatalf("getData() error = %v", err)
	}

	want := map[string]string{
		"config.yaml": "one",
		"users.yaml":  "two",
	}
	if !reflect.DeepEqual(data, want) {
		t.Fatalf("getData() = %v, want %v", data, want)
	}
}
