package main

// import (
// 	"os"
// 	"path/filepath"
// 	"reflect"
// 	"strings"
// 	"testing"
// )

// type mockOutput struct {
// 	files []string
// }

// func (m *mockOutput) AddCollectedFileInfo(artifact string, path *PathObject) error {
// 	return nil
// }

// func (m *mockOutput) AddCollectedFile(artifact string, path *PathObject) error {
// 	relPath := strings.TrimPrefix(path.path, path.filesystem.(*OSFileSystem).rootPath)
// 	relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
// 	relPath = filepath.ToSlash(relPath)
// 	m.files = append(m.files, relPath)
// 	return nil
// }

// func createTestFS(t *testing.T, structure map[string]string) string {
// 	tmpDir := t.TempDir()
// 	for path, content := range structure {
// 		fullPath := filepath.Join(tmpDir, path)
// 		dir := filepath.Dir(fullPath)
// 		if err := os.MkdirAll(dir, 0755); err != nil {
// 			t.Fatal(err)
// 		}
// 		if content != "" {
// 			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
// 				t.Fatal(err)
// 			}
// 		}
// 	}
// 	return tmpDir
// }

// func TestPathResolutionSimple(t *testing.T) {
// 	tmpDir := createTestFS(t, map[string]string{
// 		"root.txt": "test",
// 	})

// 	fs := NewOSFileSystem(tmpDir)
// 	mockOut := &mockOutput{}

// 	fs.AddPattern("TestArtifact", filepath.Join(tmpDir, "root.txt"))
// 	fs.Collect(mockOut)

// 	expected := []string{"root.txt"}
// 	if !reflect.DeepEqual(mockOut.files, expected) {
// 		t.Errorf("Expected %v, got %v", expected, mockOut.files)
// 	}
// }

// func TestPathResolutionGlobbingStar(t *testing.T) {
// 	tmpDir := createTestFS(t, map[string]string{
// 		"root.txt":  "test",
// 		"root2.txt": "test",
// 		"test.txt":  "test",
// 		"dir/file":  "test",
// 	})

// 	fs := NewOSFileSystem(tmpDir)
// 	mockOut := &mockOutput{}

// 	fs.AddPattern("TestArtifact", filepath.Join(tmpDir, "*.txt"))
// 	fs.Collect(mockOut)

// 	expected := []string{"root.txt", "root2.txt", "test.txt"}
// 	if len(mockOut.files) != len(expected) {
// 		t.Fatalf("Expected %d files, got %d", len(expected), len(mockOut.files))
// 	}
// 	for _, f := range expected {
// 		if !contains(mockOut.files, f) {
// 			t.Errorf("Missing file %s", f)
// 		}
// 	}
// }

// func TestPathResolutionRecursiveStar(t *testing.T) {
// 	tmpDir := createTestFS(t, map[string]string{
// 		"l1/l2/l2.txt":       "test",
// 		"l1/l2/l3/l3.txt":    "test",
// 		"l1/l2/l3/l4/l4.txt": "test",
// 	})

// 	fs := NewOSFileSystem(tmpDir)
// 	mockOut := &mockOutput{}

// 	fs.AddPattern("TestArtifact", filepath.Join(tmpDir, "**/l2.txt"))
// 	fs.Collect(mockOut)

// 	expected := []string{"l1/l2/l2.txt"}
// 	if !reflect.DeepEqual(mockOut.files, expected) {
// 		t.Errorf("Expected %v, got %v", expected, mockOut.files)
// 	}
// }

// func TestIsSymlink(t *testing.T) {
// 	tmpDir := t.TempDir()
// 	target := filepath.Join(tmpDir, "target.txt")
// 	if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
// 		t.Fatal(err)
// 	}

// 	symlink := filepath.Join(tmpDir, "link.txt")
// 	if err := os.Symlink(target, symlink); err != nil {
// 		t.Fatal(err)
// 	}

// 	fs := NewOSFileSystem(tmpDir)
// 	pathObj := fs.GetFullPath(symlink)

// 	if !fs.IsSymlink(pathObj) {
// 		t.Error("Expected symlink detection")
// 	}
// }
