package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FilePathObjectAdapter — адаптер для *PathObject, реализующий интерфейс FilePathObject.
// Он переопределяет методы GetPath и ReadChunks.
type FilePathObjectAdapter struct {
	*PathObject
}

// GetPath возвращает путь из внутреннего поля path.
func (a *FilePathObjectAdapter) GetPath() string {
	return a.path
}

// ReadChunks вызывает метод ReadChunks у обёрнутого PathObject,
// получая ([]byte, error), и оборачивает результат в срез ([][]byte, error),
// чтобы удовлетворить требуемую сигнатуру.
func (a *FilePathObjectAdapter) ReadChunks() ([][]byte, error) {
	chunk, err := a.PathObject.ReadChunks()
	if err != nil {
		return nil, err
	}
	return [][]byte{chunk}, nil
}

// -------------------------------------------------------------------
// Тесты, соответствующие функционалу Python‑тестов.

// TestParseHumanSize проверяет функцию parseHumanSize.
func TestParseHumanSize(t *testing.T) {
	tests := []struct {
		input       string
		expected    int64
		shouldError bool
	}{
		{"1", 1, false},
		{"2B", 2, false},
		{"3K", 3072, false},
		{"4M", 4194304, false},
		{"5G", 5368709120, false},
		{"124XS", 0, true},
	}

	for _, tt := range tests {
		val, err := parseHumanSize(tt.input)
		if tt.shouldError {
			if err == nil {
				t.Errorf("Ожидалась ошибка для %q, а получена nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("Неожиданная ошибка для %q: %v", tt.input, err)
			}
			if val != tt.expected {
				t.Errorf("parseHumanSize(%q) = %d, ожидалось %d", tt.input, val, tt.expected)
			}
		}
	}
}

// TestNormalizeFilepath проверяет функцию normalizeFilepath.
func TestNormalizeFilepath(t *testing.T) {
	// Первый тест: преобразуем "C:/test" с учётом разделителя текущей ОС.
	path1 := strings.Replace("C:/test", "/", string(os.PathSeparator), -1)
	expected1 := filepath.Join("C", "test")
	res1 := normalizeFilepath(path1)
	if res1 != expected1 {
		t.Errorf("normalizeFilepath(%q) = %q, ожидалось %q", path1, res1, expected1)
	}

	// Второй тест: если изменений не требуется.
	path2 := filepath.Join("", "usr", "share")
	res2 := normalizeFilepath(path2)
	if res2 != path2 {
		t.Errorf("normalizeFilepath(%q) = %q, ожидалось %q", path2, res2, path2)
	}
}

// TestLogging проверяет, что лог-сообщение записывается в файл.
func TestLogging(t *testing.T) {
	tempDir := t.TempDir()
	out, err := NewOutputs(tempDir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	// Записываем сообщение через глобальный логгер.
	logger.Log(LevelInfo, "test log message")
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	// Файл логов находится по пути: <out.dirpath>/<hostname>-logs.txt
	logFile := filepath.Join(out.dirpath, fmt.Sprintf("%s-logs.txt", out.hostname))
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "test log message") {
		t.Errorf("Файл логов не содержит ожидаемого сообщения")
	}
}

// TestCollectFileInfo проверяет сбор информации о файле.
func TestCollectFileInfo(t *testing.T) {
	tempDir := t.TempDir()
	// Создаём тестовый файл с содержимым "MZtest content" (14 байт).
	testFile := filepath.Join(tempDir, "test_file.txt")
	if err := os.WriteFile(testFile, []byte("MZtest content"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := NewOutputs(tempDir, "", false)
	if err != nil {
		t.Fatal(err)
	}

	// Используем OSFileSystem для оборачивания пути.
	fs := NewOSFileSystem("/")
	// Оборачиваем *PathObject адаптером.
	fpObj := &FilePathObjectAdapter{fs.GetFullPath(testFile)}
	if err := out.AddCollectedFileInfo("TestArtifact", fpObj); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	// Читаем JSONL-файл с информацией о файле.
	fiPath := filepath.Join(out.dirpath, fmt.Sprintf("%s-file_info.jsonl", out.hostname))
	data, err := os.ReadFile(fiPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatal("Записей о файлах не найдено")
	}
	var record map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatal(err)
	}

	if _, ok := record["@timestamp"]; !ok {
		t.Error("Отсутствует поле '@timestamp'")
	}

	fileInfo, ok := record["file"].(map[string]interface{})
	if !ok {
		t.Fatal("Поле 'file' отсутствует или имеет неверный формат")
	}
	pathStr, ok := fileInfo["path"].(string)
	if !ok {
		t.Fatal("Поле 'file.path' не является строкой")
	}
	if !strings.HasSuffix(pathStr, "test_file.txt") {
		t.Errorf("file.path = %q, ожидалось окончание на 'test_file.txt'", pathStr)
	}
	size, ok := fileInfo["size"].(float64)
	if !ok {
		t.Fatal("Поле 'file.size' имеет неверный формат")
	}
	if int64(size) != 14 {
		t.Errorf("file.size = %d, ожидалось 14", int64(size))
	}
	mime, ok := fileInfo["mime_type"].(string)
	if !ok {
		t.Fatal("Поле 'file.mime_type' имеет неверный формат")
	}
	if mime != "application/x-msdownload" {
		t.Errorf("file.mime_type = %q, ожидалось 'application/x-msdownload'", mime)
	}

	hashes, ok := fileInfo["hash"].(map[string]interface{})
	if !ok {
		t.Fatal("Поле 'file.hash' отсутствует или имеет неверный формат")
	}
	expectedMD5 := "10dbf3e392abcc57f8fae061c7c0aeec"
	if hashes["md5"] != expectedMD5 {
		t.Errorf("file.hash.md5 = %v, ожидалось %q", hashes["md5"], expectedMD5)
	}
	expectedSHA1 := "7ef0fe6c3855fbac1884e95622d9e45ce1d4ae9b"
	if hashes["sha1"] != expectedSHA1 {
		t.Errorf("file.hash.sha1 = %v, ожидалось %q", hashes["sha1"], expectedSHA1)
	}
	expectedSHA256 := "cfb91ddbf08c52ff294fdf1657081a98c090d270dbb412a91ace815b3df947b6"
	if hashes["sha256"] != expectedSHA256 {
		t.Errorf("file.hash.sha256 = %v, ожидалось %q", hashes["sha256"], expectedSHA256)
	}
}

// TestCollectPEFileInfo проверяет сбор информации для PE-файла (MSVCR71.dll).
func TestCollectPEFileInfo(t *testing.T) {
	tempDir := t.TempDir()
	// Файл MSVCR71.dll ожидается в папке data относительно тестов.
	testPE := filepath.Join("test_data", "MSVCR71.dll")
	if _, err := os.Stat(testPE); err != nil {
		t.Skip("Файл MSVCR71.dll не найден – пропускаем тест")
	}

	out, err := NewOutputs(tempDir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	fs := NewOSFileSystem("/")
	fpObj := &FilePathObjectAdapter{fs.GetFullPath(testPE)}
	if err := out.AddCollectedFileInfo("TestArtifact", fpObj); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	fiPath := filepath.Join(out.dirpath, fmt.Sprintf("%s-file_info.jsonl", out.hostname))
	data, err := os.ReadFile(fiPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatal("Записей о файлах не найдено")
	}
	var record map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatal(err)
	}

	if _, ok := record["@timestamp"]; !ok {
		t.Error("Отсутствует поле '@timestamp'")
	}

	labels, ok := record["labels"].(map[string]interface{})
	if !ok {
		t.Fatal("Поле 'labels' отсутствует или имеет неверный формат")
	}
	if labels["artifact"] != "TestArtifact" {
		t.Errorf("labels.artifact = %v, ожидалось 'TestArtifact'", labels["artifact"])
	}

	fileInfo, ok := record["file"].(map[string]interface{})
	if !ok {
		t.Fatal("Поле 'file' отсутствует или имеет неверный формат")
	}
	pathStr, ok := fileInfo["path"].(string)
	if !ok {
		t.Fatal("Поле 'file.path' не является строкой")
	}
	if !strings.HasSuffix(pathStr, "MSVCR71.dll") {
		t.Errorf("file.path = %q, ожидалось окончание на 'MSVCR71.dll'", pathStr)
	}
	size, ok := fileInfo["size"].(float64)
	if !ok {
		t.Fatal("Поле 'file.size' имеет неверный формат")
	}
	if int64(size) != 348160 {
		t.Errorf("file.size = %d, ожидалось 348160", int64(size))
	}
	mime, ok := fileInfo["mime_type"].(string)
	if !ok {
		t.Fatal("Поле 'file.mime_type' имеет неверный формат")
	}
	if mime != "application/x-msdownload" {
		t.Errorf("file.mime_type = %q, ожидалось 'application/x-msdownload'", mime)
	}

	hashes, ok := fileInfo["hash"].(map[string]interface{})
	if !ok {
		t.Fatal("Поле 'file.hash' отсутствует или имеет неверный формат")
	}
	expectedMD5 := "86f1895ae8c5e8b17d99ece768a70732"
	if hashes["md5"] != expectedMD5 {
		t.Errorf("file.hash.md5 = %v, ожидалось %q", hashes["md5"], expectedMD5)
	}
	expectedSHA1 := "d5502a1d00787d68f548ddeebbde1eca5e2b38ca"
	if hashes["sha1"] != expectedSHA1 {
		t.Errorf("file.hash.sha1 = %v, ожидалось %q", hashes["sha1"], expectedSHA1)
	}
	expectedSHA256 := "8094af5ee310714caebccaeee7769ffb08048503ba478b879edfef5f1a24fefe"
	if hashes["sha256"] != expectedSHA256 {
		t.Errorf("file.hash.sha256 = %v, ожидалось %q", hashes["sha256"], expectedSHA256)
	}

	peInfo, ok := fileInfo["pe"].(map[string]interface{})
	if !ok {
		t.Fatal("Поле 'file.pe' отсутствует или имеет неверный формат")
	}
	if peInfo["company"] != "Microsoft Corporation" {
		t.Errorf("file.pe.company = %v, ожидалось 'Microsoft Corporation'", peInfo["company"])
	}
	if peInfo["description"] != "Microsoft® C Runtime Library" {
		t.Errorf("file.pe.description = %v, ожидалось 'Microsoft® C Runtime Library'", peInfo["description"])
	}
	if peInfo["file_version"] != "7.10.3052.4" {
		t.Errorf("file.pe.file_version = %v, ожидалось '7.10.3052.4'", peInfo["file_version"])
	}
	if peInfo["original_file_name"] != "MSVCR71.DLL" {
		t.Errorf("file.pe.original_file_name = %v, ожидалось 'MSVCR71.DLL'", peInfo["original_file_name"])
	}
	if peInfo["product"] != "Microsoft® Visual Studio .NET" {
		t.Errorf("file.pe.product = %v, ожидалось 'Microsoft® Visual Studio .NET'", peInfo["product"])
	}
	if peInfo["imphash"] != "7acc8c379c768a1ecd81ec502ff5f33e" {
		t.Errorf("file.pe.imphash = %v, ожидалось '7acc8c379c768a1ecd81ec502ff5f33e'", peInfo["imphash"])
	}
	if peInfo["compilation"] != "2003-02-21T12:42:20" {
		t.Errorf("file.pe.compilation = %v, ожидалось '2003-02-21T12:42:20'", peInfo["compilation"])
	}
}

// TestCollectFile проверяет, что файл добавляется в zip-архив.
func TestCollectFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_file.txt")
	if err := os.WriteFile(testFile, []byte("MZtest content"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := NewOutputs(tempDir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	fs := NewOSFileSystem("/")
	fpObj := &FilePathObjectAdapter{fs.GetFullPath(testFile)}
	if err := out.AddCollectedFile("TestArtifact", fpObj); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(out.dirpath, fmt.Sprintf("%s-files.zip", out.hostname))
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	if len(zr.File) != 1 {
		t.Errorf("Ожидался 1 файл в zip, получено %d", len(zr.File))
	}
	if !strings.HasSuffix(zr.File[0].Name, "test_file.txt") {
		t.Errorf("Имя файла в архиве = %q, ожидалось окончание на 'test_file.txt'", zr.File[0].Name)
	}
}

// TestCollectFileSizeFilter проверяет фильтрацию файлов по размеру.
func TestCollectFileSizeFilter(t *testing.T) {
	tempDir := t.TempDir()
	// Файл, который должен быть добавлен (14 байт).
	testFile := filepath.Join(tempDir, "test_file.txt")
	if err := os.WriteFile(testFile, []byte("MZtest content"), 0644); err != nil {
		t.Fatal(err)
	}
	// Файл, который не должен быть добавлен (больше 15 байт).
	bigFile := filepath.Join(tempDir, "test_big_file.txt")
	if err := os.WriteFile(bigFile, []byte("some bigger content"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := NewOutputs(tempDir, "15", false)
	if err != nil {
		t.Fatal(err)
	}
	fs := NewOSFileSystem("/")
	fpObjSmall := &FilePathObjectAdapter{fs.GetFullPath(testFile)}
	fpObjBig := &FilePathObjectAdapter{fs.GetFullPath(bigFile)}
	if err := out.AddCollectedFile("TestArtifact", fpObjSmall); err != nil {
		t.Fatal(err)
	}
	if err := out.AddCollectedFile("TestArtifact", fpObjBig); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(out.dirpath, fmt.Sprintf("%s-files.zip", out.hostname))
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	if len(zr.File) != 1 {
		t.Errorf("Ожидался 1 файл в zip, получено %d", len(zr.File))
	}
	if !strings.HasSuffix(zr.File[0].Name, "test_file.txt") {
		t.Errorf("Имя файла в архиве = %q, ожидалось окончание на 'test_file.txt'", zr.File[0].Name)
	}

	// Проверяем, что в логах содержится упоминание об игнорировании большого файла.
	logFile := filepath.Join(out.dirpath, fmt.Sprintf("%s-logs.txt", out.hostname))
	logData, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	logStr := string(logData)
	if !strings.Contains(logStr, "Ignoring file") || !strings.Contains(logStr, "test_big_file.txt") {
		t.Errorf("Лог-файл не содержит сообщение об игнорировании файла %q", "test_big_file.txt")
	}
}

// TestCollectCommand проверяет сбор результата команды.
func TestCollectCommand(t *testing.T) {
	tempDir := t.TempDir()
	out, err := NewOutputs(tempDir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	out.AddCollectedCommand("TestArtifact", "command", []byte("output"))
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	cmdPath := filepath.Join(out.dirpath, fmt.Sprintf("%s-commands.json", out.hostname))
	data, err := os.ReadFile(cmdPath)
	if err != nil {
		t.Fatal(err)
	}
	var cmds map[string]map[string]string
	if err := json.Unmarshal(data, &cmds); err != nil {
		t.Fatal(err)
	}
	if artifact, ok := cmds["TestArtifact"]; !ok {
		t.Error("TestArtifact отсутствует в командах")
	} else {
		if artifact["command"] != "output" {
			t.Errorf("command = %q, ожидалось 'output'", artifact["command"])
		}
	}
}

// TestCollectWMI проверяет сбор WMI-результатов.
func TestCollectWMI(t *testing.T) {
	tempDir := t.TempDir()
	out, err := NewOutputs(tempDir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	out.AddCollectedWMI("TestArtifact", "query", "output")
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	wmiPath := filepath.Join(out.dirpath, fmt.Sprintf("%s-wmi.json", out.hostname))
	data, err := os.ReadFile(wmiPath)
	if err != nil {
		t.Fatal(err)
	}
	var wmi map[string]map[string]string
	if err := json.Unmarshal(data, &wmi); err != nil {
		t.Fatal(err)
	}
	if artifact, ok := wmi["TestArtifact"]; !ok {
		t.Error("TestArtifact отсутствует в WMI")
	} else {
		if artifact["query"] != "output" {
			t.Errorf("query = %q, ожидалось 'output'", artifact["query"])
		}
	}
}

// TestCollectRegistry проверяет сбор значений реестра.
func TestCollectRegistry(t *testing.T) {
	tempDir := t.TempDir()
	out, err := NewOutputs(tempDir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	out.AddCollectedRegistryValue("TestArtifact", "key", "name", "value", "type")
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	regPath := filepath.Join(out.dirpath, fmt.Sprintf("%s-registry.json", out.hostname))
	data, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatal(err)
	}
	var registry map[string]map[string]map[string]interface{}
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatal(err)
	}
	artifact, ok := registry["TestArtifact"]
	if !ok {
		t.Error("TestArtifact отсутствует в реестре")
	}
	key, ok := artifact["key"]
	if !ok {
		t.Error("Ключ 'key' отсутствует для TestArtifact")
	}
	entry, ok := key["name"].(map[string]interface{})
	if !ok {
		t.Fatal("Значение для 'name' имеет неверный формат")
	}
	if entry["value"] != "value" || entry["type"] != "type" {
		t.Errorf("Получено %v, ожидалось {value: %q, type: %q}", entry, "value", "type")
	}
}
