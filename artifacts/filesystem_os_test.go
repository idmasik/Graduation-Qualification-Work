package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fp возвращает полный путь, соединяя fsRoot с относительным путём.
func fp(fsRoot, relPath string) string {
	return filepath.Join(fsRoot, relPath)
}

// getFSRoot вычисляет абсолютный путь к каталогу data/filesystem.
func getFSRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("test_data", "filesystem"))
	if err != nil {
		t.Fatalf("Ошибка получения абсолютного пути: %v", err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatalf("Каталог %s не найден или не является директорией", root)
	}
	return root
}

// getCollectedPaths извлекает относительные пути файлов, собранных в *Outputs.
// Каждый полный путь приводится к виду относительно fsRoot.
func getCollectedPaths(fsRoot string, outputs *Outputs) []string {
	var paths []string
	for fullpath := range outputs.addedFiles {
		rel, err := filepath.Rel(fsRoot, fullpath)
		if err != nil {
			rel = fullpath
		}
		paths = append(paths, filepath.ToSlash(rel))
	}
	sort.Strings(paths)
	return paths
}

// checkSuffixes выполняет проверку: число собранных объектов равно ожидаемому,
// а для каждого ожидаемого элемента существует собранный путь, оканчивающийся на него.
func checkSuffixes(t *testing.T, expected, collected []string) {
	t.Helper()
	if len(collected) != len(expected) {
		t.Errorf("Ожидалось количество объектов %d, получено %d", len(expected), len(collected))
	}
	for _, exp := range expected {
		found := false
		for _, coll := range collected {
			if strings.HasSuffix(coll, exp) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Не найден элемент, оканчивающийся на %q, среди полученных %v", exp, collected)
		}
	}
}

// createTestOutputs создаёт экземпляр *Outputs для тестирования, используя временный каталог t.TempDir().
func createTestOutputs(t *testing.T) *Outputs {
	t.Helper()
	tempDir := t.TempDir()
	outputs, err := NewOutputs(tempDir, "0", false, false, "")
	if err != nil {
		t.Fatalf("Не удалось создать Outputs: %v", err)
	}
	return outputs
}

func TestPathResolutionSimple(t *testing.T) {
	fsRoot := getFSRoot(t)
	fs := NewOSFileSystem(fsRoot)
	fs.AddPattern("TestArtifact", fp(fsRoot, "root.txt"), "")
	outputs := createTestOutputs(t)
	defer outputs.Close()

	fs.Collect(outputs)
	collected := getCollectedPaths(fsRoot, outputs)
	expected := []string{"root.txt"}
	checkSuffixes(t, expected, collected)
}

func TestPathResolutionSimple2(t *testing.T) {
	fsRoot := getFSRoot(t)
	fs := NewOSFileSystem(fsRoot)
	fs.AddPattern("TestArtifact", fp(fsRoot, "l1/l2/l2.txt"), "")
	outputs := createTestOutputs(t)
	defer outputs.Close()

	fs.Collect(outputs)
	collected := getCollectedPaths(fsRoot, outputs)
	expected := []string{"l1/l2/l2.txt"}
	checkSuffixes(t, expected, collected)
}

func TestPathResolutionGlobbingStar(t *testing.T) {
	fsRoot := getFSRoot(t)
	fs := NewOSFileSystem(fsRoot)
	fs.AddPattern("TestArtifact", fp(fsRoot, "*.txt"), "")
	outputs := createTestOutputs(t)
	defer outputs.Close()

	fs.Collect(outputs)
	collected := getCollectedPaths(fsRoot, outputs)
	expected := []string{"root.txt", "root2.txt", "test.txt"}
	checkSuffixes(t, expected, collected)
}

func TestPathResolutionGlobbingStar2(t *testing.T) {
	fsRoot := getFSRoot(t)
	fs := NewOSFileSystem(fsRoot)
	fs.AddPattern("TestArtifact", fp(fsRoot, "root*.txt"), "")
	outputs := createTestOutputs(t)
	defer outputs.Close()

	fs.Collect(outputs)
	collected := getCollectedPaths(fsRoot, outputs)
	expected := []string{"root.txt", "root2.txt"}
	checkSuffixes(t, expected, collected)
}

func TestPathResolutionGlobbingQuestion(t *testing.T) {
	fsRoot := getFSRoot(t)
	fs := NewOSFileSystem(fsRoot)
	fs.AddPattern("TestArtifact", fp(fsRoot, "root?.txt"), "")
	outputs := createTestOutputs(t)
	defer outputs.Close()

	fs.Collect(outputs)
	collected := getCollectedPaths(fsRoot, outputs)
	expected := []string{"root2.txt"}
	checkSuffixes(t, expected, collected)
}

func TestPathResolutionGlobbingStarDirectory(t *testing.T) {
	fsRoot := getFSRoot(t)
	fs := NewOSFileSystem(fsRoot)
	fs.AddPattern("TestArtifact", fp(fsRoot, "l1/*/l2.txt"), "")
	outputs := createTestOutputs(t)
	defer outputs.Close()

	fs.Collect(outputs)
	collected := getCollectedPaths(fsRoot, outputs)
	expected := []string{"l1/l2/l2.txt"}
	checkSuffixes(t, expected, collected)
}

func TestPathResolutionRecursiveStar(t *testing.T) {
	fsRoot := getFSRoot(t)
	fs := NewOSFileSystem(fsRoot)
	fs.AddPattern("TestArtifact", fp(fsRoot, "**/l2.txt"), "")
	outputs := createTestOutputs(t)
	defer outputs.Close()

	fs.Collect(outputs)
	collected := getCollectedPaths(fsRoot, outputs)
	expected := []string{"l1/l2/l2.txt"}
	checkSuffixes(t, expected, collected)
}

func TestPathResolutionRecursiveStarCustomDepth(t *testing.T) {
	fsRoot := getFSRoot(t)
	fs := NewOSFileSystem(fsRoot)
	fs.AddPattern("TestArtifact", fp(fsRoot, "**4/*.txt"), "")
	outputs := createTestOutputs(t)
	defer outputs.Close()

	fs.Collect(outputs)
	collected := getCollectedPaths(fsRoot, outputs)
	expected := []string{"l1/l1.txt", "l1/l2/l2.txt", "l1/l2/l3/l3.txt", "l1/l2/l3/l4/l4.txt"}
	checkSuffixes(t, expected, collected)
}

func TestIsSymlink(t *testing.T) {
	fsRoot := getFSRoot(t)
	fs := NewOSFileSystem(fsRoot)
	pathObj := fs.GetFullPath(fp(fsRoot, "root.txt"))
	if fs.IsSymlink(pathObj) {
		t.Errorf("Ожидалось, что root.txt не является символьной ссылкой")
	}
}
