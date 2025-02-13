// package main

// import (
// 	"encoding/json"
// 	"path/filepath"
// 	"testing"
// )

// func TestJsonArtifactsWriter(t *testing.T) {
// 	artifact := NewArtifactDefinition(
// 		"TestArtifact",
// 		[]string{"alias1", "alias2"},
// 		"Test description",
// 	)
// 	artifact.URLs = []string{"http://example.com"}
// 	artifact.SupportedOS = []string{"Linux", "Windows"}

// 	_, err := artifact.AppendSource("FILE", map[string]interface{}{
// 		"paths": []string{"/path/to/file"},
// 	})
// 	if err != nil {
// 		t.Fatalf("Failed to append source: %v", err)
// 	}

// 	tmpDir := t.TempDir()
// 	tmpFile := filepath.Join(tmpDir, "test.json")

// 	writer := JsonArtifactsWriter{}
// 	writer.WriteArtifactsFile([]ArtifactDefinition{*artifact}, tmpFile)

// 	reader := NewJsonArtifactsReader()
// 	readArtifacts, err := reader.ReadFile(tmpFile)
// 	if err != nil {
// 		t.Fatalf("Failed to read artifacts: %v", err)
// 	}

// 	if len(readArtifacts) != 1 {
// 		t.Fatalf("Expected 1 artifact, got %d", len(readArtifacts))
// 	}

// 	originalJSON, _ := json.Marshal(artifact.AsDict())
// 	readJSON, _ := json.Marshal(readArtifacts[0].AsDict())

// 	if string(originalJSON) != string(readJSON) {
// 		t.Errorf("Mismatch:\nOriginal: %s\nRead: %s", originalJSON, readJSON)
// 	}
// }

// ПРОВЕРИТЬ!

// func TestYamlArtifactsWriter(t *testing.T) {
// 	artifact := NewArtifactDefinition(
// 		"TestArtifactYAML",
// 		[]string{"alias3", "alias4"},
// 		"Test YAML description",
// 	)
// 	artifact.URLs = []string{"http://example.org"}
// 	artifact.SupportedOS = []string{"Darwin", "Linux"}

// 	_, err := artifact.AppendSource("DIRECTORY", map[string]interface{}{
// 		"paths": []string{"/path/to/dir"},
// 	})
// 	if err != nil {
// 		t.Fatalf("Failed to append source: %v", err)
// 	}

// 	tmpDir := t.TempDir()
// 	tmpFile := filepath.Join(tmpDir, "test.yaml")

// 	writer := YamlArtifactsWriter{}
// 	writer.WriteArtifactsFile([]ArtifactDefinition{*artifact}, tmpFile)

// 	reader := NewYamlArtifactsReader()
// 	readArtifacts, err := reader.ReadFile(tmpFile)
// 	if err != nil {
// 		t.Fatalf("Failed to read artifacts: %v", err)
// 	}

// 	if len(readArtifacts) != 1 {
// 		t.Fatalf("Expected 1 artifact, got %d", len(readArtifacts))
// 	}

// 	// Сериализуем обе структуры в YAML для корректного сравнения
// 	originalYAML, _ := yaml.Marshal(artifact.AsDict())
// 	readYAML, _ := yaml.Marshal(readArtifacts[0].AsDict())

//		if string(originalYAML) != string(readYAML) {
//			t.Errorf("Mismatch:\nOriginal:\n%s\nRead:\n%s", originalYAML, readYAML)
//		}
//	}
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// ArtifactsWriterInterface объединяет типы, которые умеют записывать артефакты в файл.
type ArtifactsWriterInterface interface {
	// WriteArtifactsFile записывает артефакты в указанный файл.
	WriteArtifactsFile(artifacts []ArtifactDefinition, filename string)
}

// getTestFilePath возвращает путь до тестового файла, предполагая, что тестовые файлы лежат в каталоге "testdata".
func getTestFilePath(filename string) string {
	return filepath.Join("test_data", filename)
}

// convertArtifacts преобразует срез указателей на ArtifactDefinition в срез значений.
func convertArtifacts(artifacts []*ArtifactDefinition) []ArtifactDefinition {
	out := make([]ArtifactDefinition, len(artifacts))
	for i, art := range artifacts {
		out[i] = *art
	}
	return out
}

// testArtifactsConversion выполняет следующие шаги:
//  1. Читает оригинальные артефакт-определения из файла filename,
//  2. Создаёт временный каталог, записывает артефакты в новый файл с помощью artifactWriter,
//  3. Читает обратно артефакты из записанного файла и сравнивает с оригиналом.
func testArtifactsConversion(t *testing.T, artifactReader ArtifactsReaderInterface, artifactWriter ArtifactsWriterInterface, filename string) {
	testFile := getTestFilePath(filename)
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file does not exist: " + testFile)
	}

	// Чтение оригинальных артефактов.
	originalArtifactsPtr, err := artifactReader.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read original artifacts from file %s: %v", testFile, err)
	}
	originalArtifacts := convertArtifacts(originalArtifactsPtr)

	// Создаём временный каталог для записи.
	tempDir, err := os.MkdirTemp("", "artifacts_test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outputFile := filepath.Join(tempDir, filename)

	// Запись артефактов в новый файл.
	artifactWriter.WriteArtifactsFile(originalArtifacts, outputFile)

	// Чтение артефактов из записанного файла.
	convertedArtifactsPtr, err := artifactReader.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read converted artifacts from file %s: %v", outputFile, err)
	}
	convertedArtifacts := convertArtifacts(convertedArtifactsPtr)

	// Сравнение количества артефактов.
	if len(originalArtifacts) != len(convertedArtifacts) {
		t.Fatalf("Artifact count mismatch: original %d, converted %d", len(originalArtifacts), len(convertedArtifacts))
	}

	// Сравнение содержимого каждого артефакта через их представление в виде map.
	for i := range originalArtifacts {
		origDict := originalArtifacts[i].AsDict()
		convDict := convertedArtifacts[i].AsDict()
		if !reflect.DeepEqual(origDict, convDict) {
			t.Errorf("Artifact mismatch at index %d:\nOriginal: %v\nConverted: %v", i, origDict, convDict)
		}
	}
}

// TestJsonWriter проверяет, что для формата JSON
// исходные артефакт-определения после записи и последующего чтения не изменились.
func TestJsonWriter(t *testing.T) {
	artifactReader := NewJsonArtifactsReader()
	artifactWriter := &JsonArtifactsWriter{}
	testArtifactsConversion(t, artifactReader, artifactWriter, "definitions.json")
}

// TestYamlWriter проверяет, что для формата YAML
// исходные артефакт-определения после записи и последующего чтения не изменились.
func TestYamlWriter(t *testing.T) {
	artifactReader := NewYamlArtifactsReader()
	artifactWriter := &YamlArtifactsWriter{}
	testArtifactsConversion(t, artifactReader, artifactWriter, "definitions.yaml")
}
