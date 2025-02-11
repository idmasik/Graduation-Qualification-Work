package main

import (
	"bytes" // Добавить импорт пакета bytes
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Source представляет источник артефакта.
type Source struct {
	Type       string `yaml:"type"`
	Attributes struct {
		Paths         []string `yaml:"paths"`
		Separator     string   `yaml:"separator"`
		Cmd           string   `yaml:"cmd"`
		Args          []string `yaml:"args"`
		KeyValuePairs []struct {
			Key   string `yaml:"key"`
			Value string `yaml:"value"`
		} `yaml:"key_value_pairs"`
	} `yaml:"attributes"`
	SupportedOS []string `yaml:"supported_os"`
}

// Artifact представляет структуру артефакта из YAML-файла.
type Artifact struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"doc"`
	Sources     []Source `yaml:"sources"`
	SupportedOS []string `yaml:"supported_os"`
	Urls        []string `yaml:"urls,omitempty"` // Опциональное поле для URL
}

// FormatError представляет ошибку, связанную с некорректным форматом данных YAML.
type FormatError struct {
	Message string
}

func (e *FormatError) Error() string {
	return e.Message
}

// YamlArtifactsReader отвечает за чтение и обработку YAML-файлов с артефактами.
type YamlArtifactsReader struct{}

// ReadFileObject читает артефакты из данных файлового объекта (в виде байтов).
func (r *YamlArtifactsReader) ReadFileObject(data []byte) ([]Artifact, error) {
	var artifacts []Artifact
	decoder := yaml.NewDecoder(bytes.NewReader(data))

	for {
		var artifact Artifact
		err := decoder.Decode(&artifact)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, &FormatError{Message: fmt.Sprintf("Ошибка разбора YAML: %v", err)}
		}
		artifacts = append(artifacts, artifact)
	}

	if len(artifacts) == 0 {
		return nil, &FormatError{Message: "Файл YAML должен содержать список артефактов"}
	}

	return artifacts, nil
}

// ReadFile читает артефакты из файла по указанному пути.
func (r *YamlArtifactsReader) ReadFile(path string) ([]Artifact, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("файл не найден: %s", path)
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл: %v", err)
	}

	return r.ReadFileObject(data)
}

// ReadDirectory читает артефакты из всех YAML-файлов в указанной директории.
func (r *YamlArtifactsReader) ReadDirectory(path string) ([]Artifact, error) {
	var artifacts []Artifact

	err := filepath.Walk(path, func(filePath string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("ошибка при обходе директории: %v", err)
		}

		if !info.IsDir() && filepath.Ext(filePath) == ".yaml" {
			fileArtifacts, err := r.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("ошибка чтения файла %s: %v", filePath, err)
			}
			artifacts = append(artifacts, fileArtifacts...)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return artifacts, nil
}

// ReadArtifacts читает артефакты из файла или директории.
func (r *YamlArtifactsReader) ReadArtifacts(path string) ([]Artifact, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения информации о пути: %v", err)
	}

	if info.IsDir() {
		return r.ReadDirectory(path)
	}

	return r.ReadFile(path)
}

// //---------------------------------------------------------------------------------------------------
// //package artifacts

// // import (
// // 	"errors"
// // 	"fmt"
// // 	"os"
// // 	"path/filepath"
// // )

// type ArtifactDefinition struct {
// 	Name   string
// 	Aliases []string
// }

// type SourceType struct {
// 	TypeIndicator string
// 	Attributes    map[string]interface{}
// }

// type ArtifactDefinitionsRegistry struct {
// 	definitions     map[string]*ArtifactDefinition
// 	aliases         map[string]*ArtifactDefinition
// 	sourceTypes     map[string]SourceType
// 	undefinedGroups map[string]struct{}
// }

// func NewArtifactDefinitionsRegistry() *ArtifactDefinitionsRegistry {
// 	return &ArtifactDefinitionsRegistry{
// 		definitions:     make(map[string]*ArtifactDefinition),
// 		aliases:         make(map[string]*ArtifactDefinition),
// 		sourceTypes:     make(map[string]SourceType),
// 		undefinedGroups: make(map[string]struct{}),
// 	}
// }

// func (r *ArtifactDefinitionsRegistry) RegisterDefinition(artifact *ArtifactDefinition) error {
//     name := artifact.Name
//     if _, exists := r.definitions[name]; exists {
//         return fmt.Errorf("artifact definition already registered for name: %s", name)
//     }

//     for _, alias := range artifact.Aliases {
//         r.aliases[alias] = append(r.aliases[alias], artifact) // Добавляем в список
//     }

//     r.definitions[name] = artifact
//     return nil
// }

// func (r *ArtifactDefinitionsRegistry) DeregisterDefinition(artifact *ArtifactDefinition) error {
// 	name := artifact.Name
// 	if _, exists := r.definitions[name]; !exists {
// 		return fmt.Errorf("no artifact definition found for name: %s", name)
// 	}
// 	delete(r.definitions, name)

// 	for _, alias := range artifact.Aliases {
// 		delete(r.aliases, alias)
// 	}

// 	return nil
// }

// func (r *ArtifactDefinitionsRegistry) GetDefinitionByName(name string) (*ArtifactDefinition, error) {
// 	if def, exists := r.definitions[name]; exists {
// 		return def, nil
// 	}
// 	return nil, fmt.Errorf("artifact definition not found for name: %s", name)
// }

// func (r *ArtifactDefinitionsRegistry) GetDefinitionByAlias(alias string) (*ArtifactDefinition, error) {
// 	if def, exists := r.aliases[alias]; exists {
// 		return def, nil
// 	}
// 	return nil, fmt.Errorf("artifact definition not found for alias: %s", alias)
// }

// func (r *ArtifactDefinitionsRegistry) GetDefinitions() []*ArtifactDefinition {
// 	definitions := make([]*ArtifactDefinition, 0, len(r.definitions))
// 	for _, def := range r.definitions {
// 		definitions = append(definitions, def)
// 	}
// 	return definitions
// }

// func (r *ArtifactDefinitionsRegistry) RegisterSourceType(typeIndicator string, attributes map[string]interface{}) error {
// 	if _, exists := r.sourceTypes[typeIndicator]; exists {
// 		return fmt.Errorf("source type already registered: %s", typeIndicator)
// 	}

// 	sourceType := SourceType{
// 		TypeIndicator: typeIndicator,
// 		Attributes:    attributes,
// 	}
// 	r.sourceTypes[typeIndicator] = sourceType
// 	return nil
// }

// func (r *ArtifactDefinitionsRegistry) DeregisterSourceType(typeIndicator string) error {
// 	if _, exists := r.sourceTypes[typeIndicator]; !exists {
// 		return fmt.Errorf("no source type found for type indicator: %s", typeIndicator)
// 	}
// 	delete(r.sourceTypes, typeIndicator)
// 	return nil
// }

// func (r *ArtifactDefinitionsRegistry) GetUndefinedArtifacts() []string {
// 	undefined := make([]string, 0, len(r.undefinedGroups))
// 	for name := range r.undefinedGroups {
// 		undefined = append(undefined, name)
// 	}
// 	return undefined
// }

// func (r *ArtifactDefinitionsRegistry) ReadFromFile(filename string) error {
// 	file, err := os.Open(filename)
// 	if err != nil {
// 		return err
// 	}
// 	defer file.Close()

// 	// Implement YAML parsing or other file reading logic here
// 	// Example pseudo-code:
// 	// parsedArtifacts := parseYAML(file)
// 	// for _, artifact := range parsedArtifacts {
// 	//     r.RegisterDefinition(artifact)
// 	// }

// 	return nil
// }

// func (r *ArtifactDefinitionsRegistry) ReadFromDirectory(path string, extension string) error {
// 	return filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			return err
// 		}

// 		if filepath.Ext(info.Name()) == extension {
// 			if err := r.ReadFromFile(path); err != nil {
// 				return err
// 			}
// 		}
// 		return nil
// 	})
// }

// // Пример использования YamlArtifactsReader.
// func main() {
// 	reader := &YamlArtifactsReader{}
// 	path := "D:\\Projects\\GolangProjects\\Graduation Qualifying Work\\data"

// 	library := False

// 	artifacts, err := reader.ReadArtifacts(path)

// 	registry := NewArtifactDefinitionsRegistry()
//     artifactsRegistry, err := GetArtifactsRegistry(*library, directories)
// // 	if err != nil {
// // 		fmt.Printf("Ошибка: %v\n", err)
// // 		return
// // 	}
// //
// // 	for _, artifact := range artifacts {
// // 		fmt.Printf("Артефакт: %+v\n", artifact)
// // 	}
// }
