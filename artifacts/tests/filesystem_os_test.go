// filesystem_os_test.go
package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
)

// FakeOutput реализует интерфейс CollectorOutput и собирает вызовы методов AddCollectedFile и AddCollectedFileInfo.
type FakeOutput struct {
	collectedFiles []collectedFile
}

type collectedFile struct {
	artifact string
	pathObj  *PathObject
}

func (fo *FakeOutput) AddCollectedFileInfo(artifact string, pathObj *PathObject) error {
	// Для тестов не требуется собирать file info.
	return nil
}

func (fo *FakeOutput) AddCollectedFile(artifact string, pathObj *PathObject) error {
	if artifact != "TestArtifact" {
		return fmt.Errorf("unexpected artifact: %s", artifact)
	}
	fo.collectedFiles = append(fo.collectedFiles, collectedFile{artifact, pathObj})
	return nil
}

// resolvedPaths вычисляет относительные пути собранных файлов.
func resolvedPaths(fs *OSFileSystem, output *FakeOutput) []string {
	var paths []string
	for _, cf := range output.collectedFiles {
		paths = append(paths, fs.relativePath(cf.pathObj.path))
	}
	return paths
}

// getFsRoot возвращает абсолютный путь к директории test_data/filesystem,
// относительно расположения файла теста.
func getFsRoot(t *testing.T) string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Не удалось получить путь к файлу теста")
	}
	return filepath.Join(filepath.Dir(filename), "test_data", "filesystem")
}

// fp возвращает полный путь, объединив fsRoot с относительным путём.
func fp(fsRoot, relative string) string {
	return filepath.Join(fsRoot, filepath.FromSlash(relative))
}

// equalStringSlices сравнивает два слайса строк без учёта порядка.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aCopy := append([]string(nil), a...)
	bCopy := append([]string(nil), b...)
	sort.Strings(aCopy)
	sort.Strings(bCopy)
	return reflect.DeepEqual(aCopy, bCopy)
}

func TestPathResolutionSimple(t *testing.T) {
	fsRoot := getFsRoot(t)
	fs := NewOSFileSystem(fsRoot)
	output := &FakeOutput{}

	fs.AddPattern("TestArtifact", fp(fsRoot, "root.txt"), "")
	fs.Collect(output)

	paths := resolvedPaths(fs, output)
	expected := []string{"root.txt"}
	if !reflect.DeepEqual(paths, expected) {
		t.Errorf("Ожидалось %v, получено %v", expected, paths)
	}
}

func TestPathResolutionSimple2(t *testing.T) {
	fsRoot := getFsRoot(t)
	fs := NewOSFileSystem(fsRoot)
	output := &FakeOutput{}

	fs.AddPattern("TestArtifact", fp(fsRoot, "l1/l2/l2.txt"), "")
	fs.Collect(output)

	paths := resolvedPaths(fs, output)
	expected := []string{"l1/l2/l2.txt"}
	if !reflect.DeepEqual(paths, expected) {
		t.Errorf("Ожидалось %v, получено %v", expected, paths)
	}
}

func TestPathResolutionGlobbingStar(t *testing.T) {
	fsRoot := getFsRoot(t)
	fs := NewOSFileSystem(fsRoot)
	output := &FakeOutput{}

	fs.AddPattern("TestArtifact", fp(fsRoot, "*.txt"), "")
	fs.Collect(output)

	paths := resolvedPaths(fs, output)
	expected := []string{"root.txt", "root2.txt", "test.txt"}
	if !equalStringSlices(paths, expected) {
		t.Errorf("Ожидалось множество %v, получено %v", expected, paths)
	}
}

func TestPathResolutionGlobbingStar2(t *testing.T) {
	fsRoot := getFsRoot(t)
	fs := NewOSFileSystem(fsRoot)
	output := &FakeOutput{}

	fs.AddPattern("TestArtifact", fp(fsRoot, "root*.txt"), "")
	fs.Collect(output)

	paths := resolvedPaths(fs, output)
	expected := []string{"root.txt", "root2.txt"}
	if !equalStringSlices(paths, expected) {
		t.Errorf("Ожидалось множество %v, получено %v", expected, paths)
	}
}

func TestPathResolutionGlobbingQuestion(t *testing.T) {
	fsRoot := getFsRoot(t)
	fs := NewOSFileSystem(fsRoot)
	output := &FakeOutput{}

	fs.AddPattern("TestArtifact", fp(fsRoot, "root?.txt"), "")
	fs.Collect(output)

	paths := resolvedPaths(fs, output)
	expected := []string{"root2.txt"}
	if !reflect.DeepEqual(paths, expected) {
		t.Errorf("Ожидалось %v, получено %v", expected, paths)
	}
}

func TestPathResolutionGlobbingStarDirectory(t *testing.T) {
	fsRoot := getFsRoot(t)
	fs := NewOSFileSystem(fsRoot)
	output := &FakeOutput{}

	fs.AddPattern("TestArtifact", fp(fsRoot, "l1/*/l2.txt"), "")
	fs.Collect(output)

	paths := resolvedPaths(fs, output)
	expected := []string{"l1/l2/l2.txt"}
	if !reflect.DeepEqual(paths, expected) {
		t.Errorf("Ожидалось %v, получено %v", expected, paths)
	}
}

func TestPathResolutionRecursiveStar(t *testing.T) {
	fsRoot := getFsRoot(t)
	fs := NewOSFileSystem(fsRoot)
	output := &FakeOutput{}

	fs.AddPattern("TestArtifact", fp(fsRoot, "**/l2.txt"), "")
	fs.Collect(output)

	paths := resolvedPaths(fs, output)
	expected := []string{"l1/l2/l2.txt"}
	if !reflect.DeepEqual(paths, expected) {
		t.Errorf("Ожидалось %v, получено %v", expected, paths)
	}
}

func TestPathResolutionRecursiveStarDefaultDepth(t *testing.T) {
	fsRoot := getFsRoot(t)
	fs := NewOSFileSystem(fsRoot)
	output := &FakeOutput{}

	fs.AddPattern("TestArtifact", fp(fsRoot, "**/*.txt"), "")
	fs.Collect(output)

	paths := resolvedPaths(fs, output)
	expected := []string{
		"l1/l1.txt",
		"l1/l2/l2.txt",
		"l1/l2/l3/l3.txt",
	}
	if !equalStringSlices(paths, expected) {
		t.Errorf("Ожидалось множество %v, получено %v", expected, paths)
	}
}

func TestPathResolutionRecursiveStarCustomDepth(t *testing.T) {
	fsRoot := getFsRoot(t)
	fs := NewOSFileSystem(fsRoot)
	output := &FakeOutput{}

	fs.AddPattern("TestArtifact", fp(fsRoot, "**4/*.txt"), "")
	fs.Collect(output)

	paths := resolvedPaths(fs, output)
	expected := []string{
		"l1/l1.txt",
		"l1/l2/l2.txt",
		"l1/l2/l3/l3.txt",
		"l1/l2/l3/l4/l4.txt",
	}
	if !equalStringSlices(paths, expected) {
		t.Errorf("Ожидалось множество %v, получено %v", expected, paths)
	}
}

func TestPathResolutionRecursiveStarRoot(t *testing.T) {
	fsRoot := getFsRoot(t)
	fs := NewOSFileSystem(fsRoot)
	output := &FakeOutput{}

	fs.AddPattern("TestArtifact", fp(fsRoot, "**.txt"), "")
	fs.Collect(output)

	paths := resolvedPaths(fs, output)
	expected := []string{
		"root.txt",
		"root2.txt",
		"test.txt",
		"l1/l1.txt",
		"l1/l2/l2.txt",
	}
	if !equalStringSlices(paths, expected) {
		t.Errorf("Ожидалось множество %v, получено %v", expected, paths)
	}
}

func TestIsSymlink(t *testing.T) {
	fsRoot := getFsRoot(t)
	fs := NewOSFileSystem(fsRoot)

	pathObj := fs.GetFullPath(fp(fsRoot, "root.txt"))
	if fs.IsSymlink(pathObj) {
		t.Errorf("Ожидалось, что %q не является символьной ссылкой", pathObj.path)
	}
}
