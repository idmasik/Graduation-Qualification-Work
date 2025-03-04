package main

// import (
// 	"fmt"
// 	"os"
// 	"path/filepath"
// 	"reflect"
// 	"sort"
// 	"testing"

// 	"github.com/shirou/gopsutil/disk"
// )

// // fakeCollector реализует интерфейс CollectorOutput и собирает все вызовы в срез.
// type fakeCollector struct {
// 	collected []*PathObject
// }

// func (fc *fakeCollector) AddCollectedFileInfo(artifact string, path *PathObject) error {
// 	fc.collected = append(fc.collected, path)
// 	return nil
// }

// func (fc *fakeCollector) AddCollectedFile(artifact string, path *PathObject) error {
// 	fc.collected = append(fc.collected, path)
// 	return nil
// }

// // fileArtifact создаёт артефакт с типом источника TYPE_INDICATOR_FILE и атрибутом paths.
// func fileArtifact(name, pattern string) *ArtifactDefinition {
// 	artifact := NewArtifactDefinition(name, nil, "")
// 	// Если AppendSource возвращает ошибку – тест прервётся.
// 	_, err := artifact.AppendSource(TYPE_INDICATOR_FILE, map[string]interface{}{
// 		"paths": []interface{}{pattern},
// 	})
// 	if err != nil {
// 		panic(err)
// 	}
// 	return artifact
// }

// // stringSlicesEqualIgnoreOrder сравнивает два среза строк без учета порядка.
// func stringSlicesEqualIgnoreOrder(a, b []string) bool {
// 	if len(a) != len(b) {
// 		return false
// 	}
// 	m := make(map[string]int)
// 	for _, s := range a {
// 		m[s]++
// 	}
// 	for _, s := range b {
// 		if m[s] == 0 {
// 			return false
// 		}
// 		m[s]--
// 	}
// 	return true
// }

// // stringSlicesEqual сравнивает два среза строк с учетом порядка.
// func stringSlicesEqual(a, b []string) bool {
// 	return reflect.DeepEqual(a, b)
// }

// func TestGetPath(t *testing.T) {
// 	fsm, err := NewFileSystemManager()
// 	if err != nil {
// 		t.Fatalf("Error creating FileSystemManager: %v", err)
// 	}

// 	// Для теста создаём временную директорию и определяем две фиктивные точки монтирования:
// 	// 1. Для пути "/passwords.txt" – точка монтирования "/" с fstype, поддерживаемым TSKFileSystem.
// 	// 2. Для пути, основанного на fsRoot, – другая точка с fstype, не поддерживаемым TSK, т.е. OSFileSystem.
// 	fsRoot := ("C:\\Users\\Dmitr\\Desktop\\Graduation-Qualification-Work\\artifacts\\tests\\test_data")
// 	fmt.Printf("Временная директория: %s", fsRoot)
// 	fakePartitions := []disk.PartitionStat{
// 		{
// 			Device:     "/dev/sda1",
// 			Mountpoint: "/",
// 			Fstype:     "ntfs", // TSK поддерживает ntfs
// 			Opts:       "",
// 		},
// 		{
// 			Device:     "/dev/sda2",
// 			Mountpoint: fsRoot,
// 			Fstype:     "dummy", // не поддерживается, используется OSFileSystem
// 			Opts:       "",
// 		},
// 	}
// 	fsm.mountPoints = fakePartitions

// 	// Проверяем для "/passwords.txt"
// 	po, err := fsm.GetPathObject("/passwords.txt")
// 	if err != nil {
// 		t.Fatalf("Error in GetPathObject: %v", err)
// 	}
// 	switch po.filesystem.(type) {
// 	case *TSKFileSystem:
// 		// Всё верно.
// 	default:
// 		t.Errorf("Expected TSKFileSystem for '/passwords.txt', got %T", po.filesystem)
// 	}

// 	// Проверяем для пути из fsRoot, например, "root.txt"
// 	rootPath := filepath.Join(fsRoot, "root.txt")
// 	po, err = fsm.GetPathObject(rootPath)
// 	if err != nil {
// 		t.Fatalf("Error in GetPathObject: %v", err)
// 	}
// 	switch po.filesystem.(type) {
// 	case *OSFileSystem:
// 		// Всё верно.
// 	default:
// 		t.Errorf("Expected OSFileSystem for '%s', got %T", rootPath, po.filesystem) //проходит
// 	}
// }

// // TestAddArtifacts проверяет регистрацию двух артефактов и сбор путей.
// func TestAddArtifacts(t *testing.T) {
// 	fsm, err := NewFileSystemManager()
// 	if err != nil {
// 		t.Fatalf("Error creating FileSystemManager: %v", err)
// 	}

// 	fsRoot := t.TempDir()

// 	// Создаём фейковые точки монтирования с сортировкой
// 	fakePartitions := []disk.PartitionStat{
// 		{
// 			Device:     "/dev/sda1",
// 			Mountpoint: "/", // TSKFileSystem (ntfs)
// 			Fstype:     "ntfs",
// 			Opts:       "",
// 		},
// 		{
// 			Device:     "/dev/sda2",
// 			Mountpoint: fsRoot, // OSFileSystem (dummy)
// 			Fstype:     "dummy",
// 			Opts:       "",
// 		},
// 	}
// 	// Явная сортировка точек монтирования
// 	sort.Slice(fakePartitions, func(i, j int) bool {
// 		return len(fakePartitions[i].Mountpoint) > len(fakePartitions[j].Mountpoint)
// 	})
// 	fsm.mountPoints = fakePartitions

// 	output := &FakeOutput{}
// 	hv := NewHostVariables(func(h *HostVariables) {})

// 	// Регистрируем артефакт для /passwords.txt (TSK)
// 	artifact1 := fileArtifact("TestArtifact", "/passwords.txt")
// 	source1 := artifact1.Sources[0]
// 	if !fsm.RegisterSource(artifact1, source1, hv) {
// 		t.Fatal("RegisterSource failed for artifact1")
// 	}

// 	// Создаём файл root.txt и регистрируем артефакт (OS)
// 	rootPath := filepath.Join(fsRoot, "root.txt")
// 	if err := os.WriteFile(rootPath, []byte("test"), 0644); err != nil {
// 		t.Fatal(err)
// 	}
// 	artifact2 := fileArtifact("TestArtifact2", rootPath)
// 	source2 := artifact2.Sources[0]
// 	if !fsm.RegisterSource(artifact2, source2, hv) {
// 		t.Fatal("RegisterSource failed for artifact2")
// 	}

// 	fsm.Collect(output)

// 	// Логирование для отладки
// 	t.Logf("Collected files: %+v", output.collectedFiles)

// 	// Преобразуем пути через OSFileSystem
// 	osfs := NewOSFileSystem(fsRoot)
// 	collected := resolvedPaths(osfs, output)
// 	expected := []string{
// 		"/passwords.txt",            // Ожидается абсолютный путь для TSK
// 		osfs.relativePath(rootPath), // Ожидается "root.txt"
// 	}

// 	if !stringSlicesEqualIgnoreOrder(collected, expected) {
// 		t.Errorf("Collected paths %v do not match expected %v", collected, expected)
// 	}
// }

// // TestArtifactAllMountpoints проверяет, что при паттерне, начинающемся с обратного слеша,
// // он применяется ко всем точкам монтирования TSK.
// func TestArtifactAllMountpoints(t *testing.T) {
// 	fsm, err := NewFileSystemManager()
// 	if err != nil {
// 		t.Fatalf("Error creating FileSystemManager: %v", err)
// 	}
// 	// Оставляем одну точку монтирования с TSK (например, "/")
// 	fakePartitions := []disk.PartitionStat{
// 		{
// 			Device:     "/dev/sda1",
// 			Mountpoint: "/",
// 			Fstype:     "ntfs", // поддерживается TSK
// 			Opts:       "",
// 		},
// 	}
// 	fsm.mountPoints = fakePartitions

// 	output := &FakeOutput{}
// 	hv := NewHostVariables(func(h *HostVariables) {})

// 	artifact := fileArtifact("TestArtifact", "\\passwords.txt")
// 	source := artifact.Sources[0]
// 	if !fsm.RegisterSource(artifact, source, hv) {
// 		t.Errorf("RegisterSource failed for artifact")
// 	}
// 	fsm.Collect(output)

// 	// Создаем OSFileSystem для корневой точки "/" – относительный путь останется неизменным.
// 	osfs := NewOSFileSystem("/")
// 	collected := resolvedPaths(osfs, output)
// 	expected := []string{osfs.relativePath("/passwords.txt")}
// 	if !stringSlicesEqual(collected, expected) {
// 		t.Errorf("Collected paths %v do not match expected %v", collected, expected)
// 	}
// }

// // TestNoMountpoint проверяет, что для пути, для которого не найдена точка монтирования, возвращается ошибка.
// func TestNoMountpoint(t *testing.T) {
// 	fsm, err := NewFileSystemManager()
// 	if err != nil {
// 		t.Fatalf("Error creating FileSystemManager: %v", err)
// 	}
// 	// Пустой список точек монтирования.
// 	fsm.mountPoints = []disk.PartitionStat{}
// 	_, err = fsm.GetPathObject("im_not_a_mountpoint/file.txt")
// 	if err == nil {
// 		t.Errorf("Expected error for non-existent mountpoint, but got none")
// 	}
// }
