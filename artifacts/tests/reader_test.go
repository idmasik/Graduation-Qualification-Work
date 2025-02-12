package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	definitionInvalidSupportedOS1 = `
name: BadSupportedOS
doc: supported_os should be an array of strings.
sources:
- type: ARTIFACT_GROUP
  attributes:
    names:
      - 'SystemEventLogEvtx'
supported_os: Windows
`

	definitionInvalidSupportedOS2 = `
name: BadTopSupportedOS
doc: Top supported_os should match supported_os from sources.
sources:
- type: ARTIFACT_GROUP
  attributes:
    names:
      - 'SystemEventLogEvtx'
  supported_os: [Windows]
`

	definitionInvalidURLs = `
name: BadUrls
doc: badurls.
sources:
- type: ARTIFACT_GROUP
  attributes:
    names:
      - 'SystemEventLogEvtx'
supported_os: [Windows]
urls: 'http://example.com'
`

	definitionWithExtraKey = `
name: WithExtraKey
doc: definition with extra_key
sources:
- type: ARTIFACT_GROUP
  attributes:
    names:
      - 'SystemEventLogEvtx'
extra_key: 'wrong'
supported_os: [Windows]
`

	definitionWithReturnTypes = `
name: WithReturnTypes
doc: definition with return_types
sources:
- type: ARTIFACT_GROUP
  attributes:
    names: [WindowsRunKeys, WindowsServices]
  returned_types: [PersistenceFile]
`

	definitionWithoutDoc = `
name: NoDoc
sources:
- type: ARTIFACT_GROUP
  attributes:
    names:
      - 'SystemEventLogEvtx'
`

	definitionWithoutName = `
doc: Missing names attr.
sources:
- type: ARTIFACT_GROUP
  attributes:
    - 'SystemEventLogEvtx'
`

	definitionWithoutSources = `
name: BadSources
doc: must have one sources.
supported_os: [Windows]
`
)

func TestYamlArtifactsReader_ReadFileObject(t *testing.T) {
	reader := NewYamlArtifactsReader()
	testFile := filepath.Join("test_data", "definitions.yaml")
	f, err := os.Open(testFile)
	if os.IsNotExist(err) {
		t.Skipf("test file %s not found", testFile)
	} else if err != nil {
		t.Fatalf("Error opening test file: %v", err)
	}
	defer f.Close()

	artifacts, err := reader.ReadFileObject(f)
	if err != nil {
		t.Fatalf("ReadFileObject failed: %v", err)
	}

	if len(artifacts) != 7 {
		t.Fatalf("Expected 7 artifacts, got %d", len(artifacts))
	}

	// Test SecurityEventLogEvtxFile
	artifact := artifacts[0]
	if artifact.Name != "SecurityEventLogEvtxFile" {
		t.Errorf("Expected name SecurityEventLogEvtxFile, got %s", artifact.Name)
	}
	expectedDoc := "Windows Security Event log for Vista or later systems."
	if artifact.Description != expectedDoc {
		t.Errorf("Expected doc %q, got %q", expectedDoc, artifact.Description)
	}
	if len(artifact.Sources) != 1 {
		t.Fatalf("Expected 1 source, got %d", len(artifact.Sources))
	}
	source := artifact.Sources[0]
	if source.TypeIndicator != TYPE_INDICATOR_FILE {
		t.Errorf("Expected type FILE, got %s", source.TypeIndicator)
	}
	expectedPaths := []interface{}{"%%environ_systemroot%%\\System32\\winevt\\Logs\\Security.evtx"}
	if !reflect.DeepEqual(source.Attributes["paths"], expectedPaths) {
		t.Errorf("Expected paths %v, got %v", expectedPaths, source.Attributes["paths"])
	}
	if !contains(artifact.SupportedOS, "Windows") {
		t.Errorf("Expected Windows in supported_os, got %v", artifact.SupportedOS)
	}
	if len(artifact.URLs) != 1 || artifact.URLs[0] != "http://www.forensicswiki.org/wiki/Windows_XML_Event_Log_(EVTX)" {
		t.Errorf("Unexpected URLs: %v", artifact.URLs)
	}
}

func TestYamlArtifactsReader_ReadFileObject_Errors(t *testing.T) {
	tests := []struct {
		name        string
		definition  string
		expectError bool
	}{
		{
			name:        "Invalid supported_os type",
			definition:  definitionInvalidSupportedOS1,
			expectError: true,
		},
		{
			name:        "Invalid top supported_os",
			definition:  definitionInvalidSupportedOS2,
			expectError: true,
		},
		{
			name:        "Invalid URLs",
			definition:  definitionInvalidURLs,
			expectError: true,
		},
		{
			name:        "Extra key",
			definition:  definitionWithExtraKey,
			expectError: true,
		},
		{
			name:        "Return types",
			definition:  definitionWithReturnTypes,
			expectError: true,
		},
		{
			name:        "Without doc",
			definition:  definitionWithoutDoc,
			expectError: true,
		},
		{
			name:        "Without name",
			definition:  definitionWithoutName,
			expectError: true,
		},
		{
			name:        "Without sources",
			definition:  definitionWithoutSources,
			expectError: true,
		},
	}

	reader := NewYamlArtifactsReader()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.definition)
			_, err := reader.ReadFileObject(r)
			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestYamlArtifactsReader_ReadFile(t *testing.T) {
	reader := NewYamlArtifactsReader()
	testFile := filepath.Join("test_data", "definitions.yaml")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skipf("Test file %s not found", testFile)
	}

	artifacts, err := reader.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(artifacts) != 7 {
		t.Errorf("Expected 7 artifacts, got %d", len(artifacts))
	}
}

func TestYamlArtifactsReader_ReadDirectory(t *testing.T) {
	reader := NewYamlArtifactsReader()
	testDir := "test_data"
	artifacts, err := reader.ReadDirectory(testDir, "yaml")
	if err != nil {
		t.Fatalf("ReadDirectory failed: %v", err)
	}
	if len(artifacts) != 7 {
		t.Errorf("Expected 7 artifacts, got %d", len(artifacts))
	}
}

func TestYamlArtifactsReader_AsDict(t *testing.T) {
	reader := NewYamlArtifactsReader()
	testFile := filepath.Join("test_data", "definitions.yaml")
	f, err := os.Open(testFile)
	if os.IsNotExist(err) {
		t.Skipf("Test file %s not found", testFile)
	} else if err != nil {
		t.Fatalf("Error opening file: %v", err)
	}
	defer f.Close()

	artifacts, err := reader.ReadFileObject(f)
	if err != nil {
		t.Fatalf("ReadFileObject failed: %v", err)
	}

	for _, artifact := range artifacts {
		dict := artifact.AsDict()
		if dict["name"] != artifact.Name {
			t.Errorf("AsDict name mismatch: %v vs %s", dict["name"], artifact.Name)
		}
	}
}

func TestJsonArtifactsReader_ReadFile(t *testing.T) {
	reader := NewJsonArtifactsReader()
	testFile := filepath.Join("test_data", "definitions.json")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skipf("Test file %s not found", testFile)
	}

	artifacts, err := reader.ReadFile(testFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(artifacts) != 7 {
		t.Errorf("Expected 7 artifacts, got %d", len(artifacts))
	}
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
