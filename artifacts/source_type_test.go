package main

import (
	"fmt"
	"testing"
)

func TestArtifactGroupSourceType(t *testing.T) {
	names := []string{"example_artifact"}
	artifactGroup, err := NewArtifactGroupSourceType(names)
	if err != nil {
		t.Fatalf("Failed to create ArtifactGroupSourceType: %v", err)
	}

	if artifactGroup.TypeIndicator() != TYPE_INDICATOR_ARTIFACT_GROUP {
		t.Errorf("Expected TypeIndicator to be %s, got %s", TYPE_INDICATOR_ARTIFACT_GROUP, artifactGroup.TypeIndicator())
	}

	if artifactGroup.AsDict()["names"].([]string)[0] != "example_artifact" {
		t.Errorf("Expected names to contain 'example_artifact'")
	}

	_, err = NewArtifactGroupSourceType([]string{})
	if err == nil {
		t.Errorf("Expected error for missing names, got nil")
	}
}

func TestCommandSourceType(t *testing.T) {
	cmd := "ls"
	args := []string{"-l", "-a"}
	command, err := NewCommandSourceType(cmd, args)
	if err != nil {
		t.Fatalf("Failed to create CommandSourceType: %v", err)
	}

	if command.TypeIndicator() != TYPE_INDICATOR_COMMAND {
		t.Errorf("Expected TypeIndicator to be %s, got %s", TYPE_INDICATOR_COMMAND, command.TypeIndicator())
	}

	if command.AsDict()["cmd"] != "ls" {
		t.Errorf("Expected cmd to be 'ls'")
	}

	if len(command.AsDict()["args"].([]string)) != 2 {
		t.Errorf("Expected args length to be 2")
	}

	_, err = NewCommandSourceType("", args)
	if err == nil {
		t.Errorf("Expected error for missing cmd, got nil")
	}
}

func TestDirectorySourceType(t *testing.T) {
	paths := []string{"/path/to/dir"}
	separator := "\\"
	directory, err := NewDirectorySourceType(paths, separator)
	if err != nil {
		t.Fatalf("Failed to create DirectorySourceType: %v", err)
	}

	if directory.TypeIndicator() != TYPE_INDICATOR_DIRECTORY {
		t.Errorf("Expected TypeIndicator to be %s, got %s", TYPE_INDICATOR_DIRECTORY, directory.TypeIndicator())
	}

	if directory.AsDict()["separator"] != "\\" {
		t.Errorf("Expected separator to be '\\'")
	}

	_, err = NewDirectorySourceType([]string{}, separator)
	if err == nil {
		t.Errorf("Expected error for missing paths, got nil")
	}
}

func TestWindowsRegistryKeySourceType(t *testing.T) {
	keys := []string{"HKEY_LOCAL_MACHINE\\Software"}
	registryKey, err := NewWindowsRegistryKeySourceType(keys)
	if err != nil {
		t.Fatalf("Failed to create WindowsRegistryKeySourceType: %v", err)
	}

	if registryKey.TypeIndicator() != TYPE_INDICATOR_WINDOWS_REGISTRY_KEY {
		t.Errorf("Expected TypeIndicator to be %s, got %s", TYPE_INDICATOR_WINDOWS_REGISTRY_KEY, registryKey.TypeIndicator())
	}

	if len(registryKey.AsDict()["keys"].([]string)) != 1 {
		t.Errorf("Expected keys length to be 1")
	}

	_, err = NewWindowsRegistryKeySourceType([]string{"INVALID_KEY"})
	if err == nil {
		t.Errorf("Expected error for invalid key, got nil")
	}
}

func TestWMIQuerySourceType(t *testing.T) {
	query := "SELECT * FROM Win32_OperatingSystem"
	baseObject := "Win32_BaseObject"
	wmiQuery, err := NewWMIQuerySourceType(baseObject, query)
	if err != nil {
		t.Fatalf("Failed to create WMIQuerySourceType: %v", err)
	}

	if wmiQuery.TypeIndicator() != TYPE_INDICATOR_WMI_QUERY {
		t.Errorf("Expected TypeIndicator to be %s, got %s", TYPE_INDICATOR_WMI_QUERY, wmiQuery.TypeIndicator())
	}

	if wmiQuery.AsDict()["query"] != query {
		t.Errorf("Expected query to be %s", query)
	}

	_, err = NewWMIQuerySourceType("", "")
	if err == nil {
		t.Errorf("Expected error for missing query, got nil")
	}
}

func TestSourceTypeFactory(t *testing.T) {
	factory := NewSourceTypeFactory()

	factory.RegisterSourceType(TYPE_INDICATOR_ARTIFACT_GROUP, func(attributes map[string]interface{}) (SourceType, error) {
		names, ok := attributes["names"].([]string)
		if !ok {
			return nil, &FormatError{"invalid or missing 'names' attribute"}
		}
		return NewArtifactGroupSourceType(names)
	})

	attributes := map[string]interface{}{
		"names": []string{"example_artifact"},
	}
	source, err := factory.CreateSourceType(TYPE_INDICATOR_ARTIFACT_GROUP, attributes)
	if err != nil {
		t.Fatalf("Failed to create source type: %v", err)
	}

	if source.TypeIndicator() != TYPE_INDICATOR_ARTIFACT_GROUP {
		t.Errorf("Expected TypeIndicator to be %s, got %s", TYPE_INDICATOR_ARTIFACT_GROUP, source.TypeIndicator())
	}
}

func TestArtifact(t *testing.T) {
	factory := NewSourceTypeFactory()

	// Регистрируем тип ARTIFACT_GROUP в фабрике
	factory.RegisterSourceType(TYPE_INDICATOR_ARTIFACT_GROUP, func(attributes map[string]interface{}) (SourceType, error) {
		names, ok := attributes["names"].([]string)
		if !ok {
			return nil, &FormatError{"invalid or missing 'names' attribute"}
		}
		return NewArtifactGroupSourceType(names)
	})

	source, err := factory.CreateSourceType(TYPE_INDICATOR_ARTIFACT_GROUP, map[string]interface{}{
		"names": []string{"example_artifact"},
	})
	if err != nil {
		t.Fatalf("Failed to create source type: %v", err)
	}

	fmt.Println("Source Type:", source.AsDict())

	artifact := NewArtifactDefinition("ExampleArtifact", []string{"ex_art"}, "Пример определения артефакта")
	attrs := map[string]interface{}{"path": "/var/log/example.log"}
	if _, err := artifact.AppendSource("FILE", attrs); err != nil {
		t.Errorf("Ошибка добавления источника: %v", err)
	}

	fmt.Printf("Artifact as dict: %#v\n", artifact.AsDict())
}
