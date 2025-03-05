package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/shirou/gopsutil/disk"
)

// stringSlicesEqualIgnoreOrder сравнивает два среза строк без учёта порядка.
func stringSlicesEqualIgnoreOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]int)
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		if m[s] == 0 {
			return false
		}
		m[s]--
	}
	return true
}

// stringSlicesEqual сравнивает два среза строк с учетом порядка.
func stringSlicesEqual(a, b []string) bool {
	return reflect.DeepEqual(a, b)
}

// resolvedPaths преобразует собранные файлы из Outputs в срез относительных путей.
// Для каждого полного пути (ключа в addedFiles) вычисляется относительный путь от fs.rootPath.
func resolvedPaths(fs *OSFileSystem, output *Outputs) []string {
	var paths []string
	for fullpath := range output.addedFiles {
		rel, err := filepath.Rel(fs.rootPath, fullpath)
		if err != nil {
			rel = fullpath
		}
		paths = append(paths, filepath.ToSlash(rel))
	}
	sort.Strings(paths)
	return paths
}

// fileArtifact создаёт артефакт с типом источника TYPE_INDICATOR_FILE и атрибутом paths.
func fileArtifact(name, pattern string) *ArtifactDefinition {
	artifact := NewArtifactDefinition(name, nil, "")
	_, err := artifact.AppendSource(TYPE_INDICATOR_FILE, map[string]interface{}{
		"paths": []interface{}{pattern},
	})
	if err != nil {
		panic(err)
	}
	return artifact
}

func TestGetPath(t *testing.T) {
	fsm, err := NewFileSystemManager()
	if err != nil {
		t.Fatalf("Error creating FileSystemManager: %v", err)
	}

	// Для теста создаём фиктивную директорию и определяем две точки монтирования:
	// 1. Для пути "/passwords.txt" – точка монтирования "/" с поддерживаемым fstype (TSKFileSystem).
	// 2. Для пути, основанного на fsRoot – точка монтирования с неподдерживаемым fstype (OSFileSystem).
	fsRoot := "C:\\Users\\Dmitr\\Desktop\\Graduation-Qualification-Work\\artifacts\\tests\\test_data"
	fmt.Printf("Временная директория: %s", fsRoot)
	fakePartitions := []disk.PartitionStat{
		{
			Device:     "/dev/sda1",
			Mountpoint: "/",
			Fstype:     "ntfs", // TSK поддерживает ntfs
			Opts:       "",
		},
		{
			Device:     "/dev/sda2",
			Mountpoint: fsRoot,
			Fstype:     "dummy", // не поддерживается, используется OSFileSystem
			Opts:       "",
		},
	}
	fsm.mountPoints = fakePartitions

	// Проверяем для "/passwords.txt"
	po, err := fsm.GetPathObject("/passwords.txt")
	if err != nil {
		t.Fatalf("Error in GetPathObject: %v", err)
	}
	switch po.filesystem.(type) {
	case *TSKFileSystem:
		// Всё верно.
	default:
		t.Errorf("Expected TSKFileSystem for '/passwords.txt', got %T", po.filesystem)
	}

	// Проверяем для пути из fsRoot, например, "root.txt"
	rootPath := filepath.Join(fsRoot, "root.txt")
	po, err = fsm.GetPathObject(rootPath)
	if err != nil {
		t.Fatalf("Error in GetPathObject: %v", err)
	}
	switch po.filesystem.(type) {
	case *OSFileSystem:
		// Всё верно.
	default:
		t.Errorf("Expected OSFileSystem for '%s', got %T", rootPath, po.filesystem)
	}
}

func TestAddArtifacts(t *testing.T) {
	fsm, err := NewFileSystemManager()
	if err != nil {
		t.Fatalf("Error creating FileSystemManager: %v", err)
	}

	fsRoot := t.TempDir()

	// Создаём фейковые точки монтирования с сортировкой: длинное значение Mountpoint имеет приоритет.
	fakePartitions := []disk.PartitionStat{
		{
			Device:     "/dev/sda1",
			Mountpoint: "/",
			Fstype:     "ntfs", // TSKFileSystem (ntfs)
			Opts:       "",
		},
		{
			Device:     "/dev/sda2",
			Mountpoint: fsRoot, // OSFileSystem (dummy)
			Fstype:     "dummy",
			Opts:       "",
		},
	}
	sort.Slice(fakePartitions, func(i, j int) bool {
		return len(fakePartitions[i].Mountpoint) > len(fakePartitions[j].Mountpoint)
	})
	fsm.mountPoints = fakePartitions

	output := createTestOutputs(t)
	defer output.Close()

	hv := NewHostVariables(func(h *HostVariables) {})

	// Регистрируем артефакт для /passwords.txt (TSK)
	artifact1 := fileArtifact("TestArtifact", "/passwords.txt")
	source1 := artifact1.Sources[0]
	if !fsm.RegisterSource(artifact1, source1, hv) {
		t.Fatal("RegisterSource failed for artifact1")
	}

	// Создаём файл root.txt и регистрируем артефакт (OS)
	rootPath := filepath.Join(fsRoot, "root.txt")
	if err := os.WriteFile(rootPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	artifact2 := fileArtifact("TestArtifact2", rootPath)
	source2 := artifact2.Sources[0]
	if !fsm.RegisterSource(artifact2, source2, hv) {
		t.Fatal("RegisterSource failed for artifact2")
	}

	fsm.Collect(output)

	t.Logf("Collected files: %+v", output.addedFiles)

	// Преобразуем пути через OSFileSystem.
	osfs := NewOSFileSystem(fsRoot)
	collected := resolvedPaths(osfs, output)
	expected := []string{
		"/passwords.txt",            // Ожидается абсолютный путь для TSK.
		osfs.relativePath(rootPath), // Ожидается "root.txt" для OS.
	}

	if !stringSlicesEqualIgnoreOrder(collected, expected) {
		t.Errorf("Collected paths %v do not match expected %v", collected, expected)
	}
}

func TestArtifactAllMountpoints(t *testing.T) {
	fsm, err := NewFileSystemManager()
	if err != nil {
		t.Fatalf("Error creating FileSystemManager: %v", err)
	}
	// Оставляем одну точку монтирования с TSK (например, "/")
	fakePartitions := []disk.PartitionStat{
		{
			Device:     "/dev/sda1",
			Mountpoint: "/",
			Fstype:     "ntfs", // поддерживается TSK
			Opts:       "",
		},
	}
	fsm.mountPoints = fakePartitions

	output := createTestOutputs(t)
	defer output.Close()

	hv := NewHostVariables(func(h *HostVariables) {})

	artifact := fileArtifact("TestArtifact", "\\passwords.txt")
	source := artifact.Sources[0]
	if !fsm.RegisterSource(artifact, source, hv) {
		t.Errorf("RegisterSource failed for artifact")
	}
	fsm.Collect(output)

	// Создаем OSFileSystem для корневой точки "/" – относительный путь останется неизменным.
	osfs := NewOSFileSystem("/")
	collected := resolvedPaths(osfs, output)
	expected := []string{osfs.relativePath("/passwords.txt")}
	if !stringSlicesEqual(collected, expected) {
		t.Errorf("Collected paths %v do not match expected %v", collected, expected)
	}
}

func TestNoMountpoint(t *testing.T) {
	fsm, err := NewFileSystemManager()
	if err != nil {
		t.Fatalf("Error creating FileSystemManager: %v", err)
	}
	// Пустой список точек монтирования.
	fsm.mountPoints = []disk.PartitionStat{}
	_, err = fsm.GetPathObject("im_not_a_mountpoint/file.txt")
	if err == nil {
		t.Errorf("Expected error for non-existent mountpoint, but got none")
	}
}
