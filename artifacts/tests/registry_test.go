package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestTestSourceType проверяет создание TestSourceType.
func TestTestSourceType(t *testing.T) {
	// Проверка создания с валидными атрибутами
	source, err := NewTestSourceType(map[string]interface{}{"test": "value"})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	testSource := source.(*TestSourceType)
	if testSource.Test != "value" {
		t.Errorf("Expected test='value', got %s", testSource.Test)
	}

	// Проверка ошибки при отсутствии атрибута
	_, err = NewTestSourceType(map[string]interface{}{})
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

// TestArtifactDefinitionsRegistry тестирует основные функции реестра артефактов.
func TestArtifactDefinitionsRegistry(t *testing.T) {
	registry := NewArtifactDefinitionsRegistry()
	reader := NewYamlArtifactsReader()

	// Загрузка тестового файла
	testFilePath := filepath.Join("test_data", "definitions.yaml")
	file, err := os.Open(testFilePath)
	if err != nil {
		t.Skipf("Missing test file: %s", testFilePath)
	}
	defer file.Close()

	definitions, err := reader.ReadFileObject(file)
	if err != nil {
		t.Fatalf("ReadFileObject failed: %v", err)
	}

	// Регистрация всех артефактов
	for _, def := range definitions {
		if err := registry.RegisterDefinition(def); err != nil {
			t.Fatalf("RegisterDefinition failed: %v", err)
		}
	}

	// Проверка количества зарегистрированных артефактов
	defs := registry.GetDefinitions()
	if len(defs) != 7 {
		t.Errorf("Expected 7 definitions, got %d", len(defs))
	}

	// Проверка получения артефакта по имени
	artifact := registry.GetDefinitionByName("SecurityEventLogEvtxFile")
	if artifact == nil {
		t.Fatal("GetDefinitionByName('CrowdstrikeQuarantine') returned nil")
	}

	// Попытка повторной регистрации
	err = registry.RegisterDefinition(artifact)
	if err == nil {
		t.Fatal("Expected error on duplicate registration, got nil")
	}

	// Дерегистрация артефакта
	if err := registry.DeregisterDefinition(artifact); err != nil {
		t.Fatalf("DeregisterDefinition failed: %v", err)
	}

	// Проверка обновленного количества
	defs = registry.GetDefinitions()
	if len(defs) != 6 {
		t.Errorf("Expected 6 definitions after deregister, got %d", len(defs))
	}

	// Проверка обработки некорректных данных
	badData := bytes.NewBufferString(`
name: SecurityEventLogEvtx
doc: Windows Security Event log for Vista or later systems
sources:
- type: FILE
  attributes: {broken: ['%environ_systemroot%\System32\winevt\Logs\Security.evtx']}
`)
	_, err = reader.ReadFileObject(badData)
	if err == nil {
		t.Fatal("Expected error on invalid data, got nil")
	}
}

// TestSourceTypeFunctions тестирует работу с типами источников.
func TestSourceTypeFunctions(t *testing.T) {
	registry := NewArtifactDefinitionsRegistry()
	initialCount := len(registry.sourceTypeFactory.sourceTypeConstructors)

	// Регистрация нового типа
	registry.sourceTypeFactory.RegisterSourceType("test", NewTestSourceType)

	// Проверка количества типов
	if len(registry.sourceTypeFactory.sourceTypeConstructors) != initialCount+1 {
		t.Errorf("Expected %d source types, got %d", initialCount+1, len(registry.sourceTypeFactory.sourceTypeConstructors))
	}

	// Создание валидного источника
	source, err := registry.CreateSourceType("test", map[string]interface{}{"test": "value"})
	if err != nil {
		t.Fatalf("CreateSourceType failed: %v", err)
	}
	testSource := source.(*TestSourceType)
	if testSource.Test != "value" {
		t.Errorf("Expected test='value', got %s", testSource.Test)
	}

	// Создание с невалидными атрибутами
	_, err = registry.CreateSourceType("test", map[string]interface{}{})
	if err == nil {
		t.Fatal("Expected error on invalid attributes, got nil")
	}

	// Дерегистрация
	registry.sourceTypeFactory.DeregisterSourceType("test")
	if len(registry.sourceTypeFactory.sourceTypeConstructors) != initialCount {
		t.Errorf("Expected %d source types after deregister, got %d", initialCount, len(registry.sourceTypeFactory.sourceTypeConstructors))
	}
}

// TestSourceType реализация
type TestSourceType struct {
	BaseSourceType
	Test string
}

func NewTestSourceType(attributes map[string]interface{}) (SourceType, error) {
	test, ok := attributes["test"].(string)
	if !ok || test == "" {
		return nil, errors.New("missing test value")
	}
	return &TestSourceType{
		BaseSourceType: BaseSourceType{TypeIndicatorValue: "test"},
		Test:           test,
	}, nil
}

func (t *TestSourceType) AsDict() map[string]interface{} {
	return map[string]interface{}{"test": t.Test}
}
