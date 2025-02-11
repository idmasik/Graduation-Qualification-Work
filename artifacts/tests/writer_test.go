package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestJsonArtifactsWriter(t *testing.T) {
	artifact := NewArtifactDefinition(
		"TestArtifact",
		[]string{"alias1", "alias2"},
		"Test description",
	)
	artifact.URLs = []string{"http://example.com"}
	artifact.SupportedOS = []string{"Linux", "Windows"}

	_, err := artifact.AppendSource("FILE", map[string]interface{}{
		"paths": []string{"/path/to/file"},
	})
	if err != nil {
		t.Fatalf("Failed to append source: %v", err)
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.json")

	writer := JsonArtifactsWriter{}
	writer.WriteArtifactsFile([]ArtifactDefinition{*artifact}, tmpFile)

	reader := NewJsonArtifactsReader()
	readArtifacts, err := reader.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read artifacts: %v", err)
	}

	if len(readArtifacts) != 1 {
		t.Fatalf("Expected 1 artifact, got %d", len(readArtifacts))
	}

	originalJSON, _ := json.Marshal(artifact.AsDict())
	readJSON, _ := json.Marshal(readArtifacts[0].AsDict())

	if string(originalJSON) != string(readJSON) {
		t.Errorf("Mismatch:\nOriginal: %s\nRead: %s", originalJSON, readJSON)
	}
}

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

// 	if string(originalYAML) != string(readYAML) {
// 		t.Errorf("Mismatch:\nOriginal:\n%s\nRead:\n%s", originalYAML, readYAML)
// 	}
// }
