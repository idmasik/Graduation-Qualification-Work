package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FakeFilePathObject реализует интерфейс FilePathObject для тестирования.
type FakeFilePathObject struct {
	path   string
	size   int64
	chunks [][]byte
}

func (f FakeFilePathObject) GetPath() string {
	return f.path
}

func (f FakeFilePathObject) GetSize() int64 {
	return f.size
}

func (f FakeFilePathObject) ReadChunks() ([][]byte, error) {
	return f.chunks, nil
}

// FakeFileCollector обрабатывает источники с типами FILE и FILE_INFO.
type FakeFileCollector struct {
	fileArtifacts     map[string]string
	fileInfoArtifacts map[string]string
}

func NewFakeFileCollector() *FakeFileCollector {
	return &FakeFileCollector{
		fileArtifacts:     make(map[string]string),
		fileInfoArtifacts: make(map[string]string),
	}
}

// Реализация метода RegisterSource с тремя аргументами: артефакт, источник и *HostVariables.
func (fc *FakeFileCollector) RegisterSource(ad *ArtifactDefinition, source *Source, hv *HostVariables) bool {
	if source.TypeIndicator == TYPE_INDICATOR_FILE {
		paths, ok := source.Attributes["paths"].([]string)
		if !ok || len(paths) == 0 {
			return false
		}
		fc.fileArtifacts[ad.Name] = paths[0]
		return true
	}
	if source.TypeIndicator == FILE_INFO_TYPE {
		paths, ok := source.Attributes["paths"].([]string)
		if !ok || len(paths) == 0 {
			return false
		}
		fc.fileInfoArtifacts[ad.Name] = paths[0]
		return true
	}
	return false
}

func (fc *FakeFileCollector) Collect(output *Outputs) {
	// Для каждого артефакта типа FILE создаём фиктивный FilePathObject и вызываем AddCollectedFile.
	for artifact, path := range fc.fileArtifacts {
		fakeFile := FakeFilePathObject{
			path:   path,
			size:   10, // произвольный небольшой размер
			chunks: [][]byte{[]byte("dummy file content")},
		}
		err := output.AddCollectedFile(artifact, fakeFile)
		if err != nil {
			fmt.Printf("Error collecting file for %s: %v\n", artifact, err)
		}
	}
	// Для каждого артефакта типа FILE_INFO создаём фиктивный FilePathObject и вызываем AddCollectedFileInfo.
	for artifact, path := range fc.fileInfoArtifacts {
		fakeFile := FakeFilePathObject{
			path:   path,
			size:   10,
			chunks: [][]byte{[]byte("dummy file content")},
		}
		err := output.AddCollectedFileInfo(artifact, fakeFile)
		if err != nil {
			fmt.Printf("Error collecting file info for %s: %v\n", artifact, err)
		}
	}
}

// FakeCommandCollector обрабатывает источники типа COMMAND.
type FakeCommandCollector struct {
	commandArtifacts map[string]string
}

func NewFakeCommandCollector() *FakeCommandCollector {
	return &FakeCommandCollector{
		commandArtifacts: make(map[string]string),
	}
}

// Реализация метода RegisterSource с тремя аргументами.
func (cc *FakeCommandCollector) RegisterSource(ad *ArtifactDefinition, source *Source, hv *HostVariables) bool {
	if source.TypeIndicator == TYPE_INDICATOR_COMMAND {
		cmd, ok := source.Attributes["cmd"].(string)
		if !ok {
			return false
		}
		cc.commandArtifacts[ad.Name] = cmd
		return true
	}
	return false
}

func (cc *FakeCommandCollector) Collect(output *Outputs) {
	for artifact, cmd := range cc.commandArtifacts {
		output.AddCollectedCommand(artifact, cmd, []byte("test output"))
	}
}

// TestCollector проверяет, что при регистрации корректных источников создаются файлы с результатами.
func TestCollector(t *testing.T) {
	// Создаём временную директорию.
	tempDir := t.TempDir()
	outputs, err := NewOutputs(tempDir, "50M", false, false, "")
	if err != nil {
		t.Fatalf("Error creating Outputs: %v", err)
	}

	// Создаём артефакты.
	commandEcho := NewArtifactDefinition("EchoCommand", nil, "")
	_, err = commandEcho.AppendSource(TYPE_INDICATOR_COMMAND, map[string]interface{}{
		"cmd":  "echo",
		"args": []string{"test"},
	})
	if err != nil {
		t.Fatalf("Error appending source to EchoCommand: %v", err)
	}

	passwordsFile := NewArtifactDefinition("PasswordsFile", nil, "")
	_, err = passwordsFile.AppendSource(TYPE_INDICATOR_FILE, map[string]interface{}{
		"paths": []string{"/passwords.txt"},
	})
	if err != nil {
		t.Fatalf("Error appending source to PasswordsFile: %v", err)
	}

	passwordsFileInfo := NewArtifactDefinition("PasswordsFileInfo", nil, "")
	_, err = passwordsFileInfo.AppendSource(FILE_INFO_TYPE, map[string]interface{}{
		"paths": []string{"/passwords.txt"},
	})
	if err != nil {
		t.Fatalf("Error appending source to PasswordsFileInfo: %v", err)
	}

	// Получаем название ОС.
	osName, err := getOperatingSystem()
	if err != nil {
		t.Fatalf("Error getting operating system: %v", err)
	}

	// Создаём Collector и подменяем сборщиков на fake-реализации.
	collector := NewCollector(osName, nil)
	fakeFileCollector := NewFakeFileCollector()
	fakeCommandCollector := NewFakeCommandCollector()
	collector.collectors = []AbstractCollector{fakeFileCollector, fakeCommandCollector}

	// Регистрируем источники.
	if len(commandEcho.Sources) == 0 {
		t.Fatal("No sources in EchoCommand")
	}
	collector.RegisterSource(commandEcho, commandEcho.Sources[0])
	if len(passwordsFile.Sources) == 0 {
		t.Fatal("No sources in PasswordsFile")
	}
	collector.RegisterSource(passwordsFile, passwordsFile.Sources[0])
	if len(passwordsFileInfo.Sources) == 0 {
		t.Fatal("No sources in PasswordsFileInfo")
	}
	collector.RegisterSource(passwordsFileInfo, passwordsFileInfo.Sources[0])

	// Выполняем сбор артефактов.
	collector.Collect(outputs)

	// Проверяем, что в результирующем каталоге появились ожидаемые файлы.
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Error reading tempDir: %v", err)
	}
	var finalDir string
	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(entry.Name(), "-") {
			finalDir = filepath.Join(tempDir, entry.Name())
			break
		}
	}
	if finalDir == "" {
		t.Fatal("Final outputs directory not found")
	}

	files, err := os.ReadDir(finalDir)
	if err != nil {
		t.Fatalf("Error reading final outputs directory: %v", err)
	}

	var zipFileFound, commandsFileFound, fileInfoFileFound bool
	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, "-files.zip") {
			zipFileFound = true
		}
		if strings.HasSuffix(name, "-commands.json") {
			commandsFileFound = true
		}
		if strings.HasSuffix(name, "-file_info.jsonl") {
			fileInfoFileFound = true
		}
	}

	if !zipFileFound {
		t.Error("Expected zip file for collected files not found")
	}
	if !commandsFileFound {
		t.Error("Expected commands JSON file not found")
	}
	if !fileInfoFileFound {
		t.Error("Expected file info JSONL file not found")
	}
}

// TestUnsupportedSource проверяет, что при регистрации источника с типом PATH генерируется предупреждение.
func TestUnsupportedSource(t *testing.T) {
	// Перенаправляем вывод глобального логгера в буфер.
	var buf bytes.Buffer
	logger.SetOutput(&buf)

	// Создаём артефакт с источником типа PATH.
	pathArtifact := NewArtifactDefinition("PathArtifact", nil, "")
	_, err := pathArtifact.AppendSource(TYPE_INDICATOR_PATH, map[string]interface{}{
		"paths": []string{"/passwords.txt"},
	})
	if err != nil {
		t.Fatalf("Error appending source to PathArtifact: %v", err)
	}

	// Получаем название ОС.
	osName, err := getOperatingSystem()
	if err != nil {
		t.Fatalf("Error getting operating system: %v", err)
	}

	// Создаём Collector с fake-сборщиками, которые не поддерживают PATH.
	collector := NewCollector(osName, nil)
	fakeFileCollector := NewFakeFileCollector()
	fakeCommandCollector := NewFakeCommandCollector()
	collector.collectors = []AbstractCollector{fakeFileCollector, fakeCommandCollector}

	// Регистрируем источник.
	if len(pathArtifact.Sources) == 0 {
		t.Fatal("No sources in PathArtifact")
	}
	collector.RegisterSource(pathArtifact, pathArtifact.Sources[0])

	// Проверяем, что в логах присутствует ожидаемое предупреждение.
	logOutput := buf.String()
	expectedSubstring := "Cannot process source for 'PathArtifact' because type 'PATH' is not supported"
	if !strings.Contains(logOutput, expectedSubstring) {
		t.Errorf("Expected log message to contain '%s', got '%s'", expectedSubstring, logOutput)
	}
}
