// package main

// import (
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"io"
// 	"os"
// 	"path/filepath"

// 	"gopkg.in/yaml.v3"
// )

// // / Интерфейс для чтения артефактов.
// type ArtifactsReaderInterface interface {
// 	ReadArtifactDefinitionValues(definitionValues map[string]interface{}) (*ArtifactDefinition, error)
// 	ReadDirectory(path string, extension string) ([]*ArtifactDefinition, error)
// 	ReadFile(filename string) ([]*ArtifactDefinition, error)
// 	ReadFileObject(fileObject io.Reader) ([]*ArtifactDefinition, error)
// }

// type BaseArtifactsReader struct {
// 	supportedOS map[string]bool
// }

// func NewBaseArtifactsReader() *BaseArtifactsReader {
// 	return &BaseArtifactsReader{
// 		supportedOS: map[string]bool{},
// 	}
// }

// // ArtifactsReader – реализация общих методов.
// type ArtifactsReader struct {
// 	*BaseArtifactsReader
// }

// func NewArtifactsReader() *ArtifactsReader {
// 	supported := make(map[string]bool)
// 	for osName, ok := range SUPPORTED_OS {
// 		if ok {
// 			supported[osName] = true
// 		}
// 	}
// 	return &ArtifactsReader{
// 		BaseArtifactsReader: &BaseArtifactsReader{
// 			supportedOS: supported,
// 		},
// 	}
// }

// // _readSupportedOS читает поле supported_os из definitionValues и,
// // если оно задано, присваивает его объекту artifactDefinition.
// // Здесь объектом является только ArtifactDefinition, т.к. Source не содержит данного поля.
// func (r *ArtifactsReader) _readSupportedOS(definitionValues map[string]interface{}, artifactDefinition *ArtifactDefinition, name string) error {
// 	raw, exists := definitionValues["supported_os"]
// 	if exists {
// 		rawList, ok := raw.([]interface{})
// 		if !ok {
// 			return FormatError{msg: fmt.Sprintf("Invalid supported_os type: %T", raw)}
// 		}
// 		var supportedOSList []string
// 		for _, v := range rawList {
// 			s, ok := v.(string)
// 			if !ok {
// 				return FormatError{msg: fmt.Sprintf("supported_os element is not a string: %v", v)}
// 			}
// 			if !r.supportedOS[s] {
// 				return FormatError{msg: fmt.Sprintf("Artifact definition: %s undefined supported operating system: %s.", name, s)}
// 			}
// 			supportedOSList = append(supportedOSList, s)
// 		}
// 		artifactDefinition.SupportedOS = supportedOSList
// 	}
// 	return nil
// }

// // artifactsreader.go
// func (r *ArtifactsReader) _readSources(artifactDefinitionValues map[string]interface{}, artifactDefinition *ArtifactDefinition, name string) error {
// 	rawSources, exists := artifactDefinitionValues["sources"]
// 	if !exists {
// 		return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s missing sources.", name)}
// 	}

// 	sourcesList, ok := rawSources.([]interface{})
// 	if !ok {
// 		return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s sources is not a list.", name)}
// 	}

// 	for _, rawSource := range sourcesList {
// 		sourceMap, ok := rawSource.(map[string]interface{})
// 		if !ok {
// 			return FormatError{msg: fmt.Sprintf("Invalid source format in artifact: %s", name)}
// 		}

// 		rawType, exists := sourceMap["type"]
// 		if !exists {
// 			return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s source type.", name)}
// 		}

// 		typeIndicator, ok := rawType.(string)
// 		if !ok || typeIndicator == "" {
// 			return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s source type.", name)}
// 		}

// 		var attributes map[string]interface{}
// 		if rawAttr, exists := sourceMap["attributes"]; exists {
// 			attributes, ok = rawAttr.(map[string]interface{})
// 			if !ok {
// 				return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s attributes is not a map.", name)}
// 			}
// 		} else {
// 			attributes = make(map[string]interface{})
// 		}

// 		// Если задано устаревшее поле "returned_types"
// 		if _, exists := sourceMap["returned_types"]; exists {
// 			return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s returned_types no longer supported.", name)}
// 		}

// 		// --- Валидация источника через фабричный метод CreateSourceType ---
// 		// Попытка создать объект источника для проверки валидности атрибутов.
// 		_, err := globalSourceTypeFactory.CreateSourceType(typeIndicator, attributes)
// 		if err != nil {
// 			return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s, with error: %s", name, err.Error())}
// 		}
// 		// ---------------------------------------------------------------------

// 		// Если валидация прошла, добавляем источник в определение.
// 		_, err = artifactDefinition.AppendSource(typeIndicator, attributes)
// 		if err != nil {
// 			return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s, with error: %s", name, err.Error())}
// 		}

// 		// Обработка опционального поля supported_os (если задано).
// 		if rawSupported, exists := sourceMap["supported_os"]; exists {
// 			rawList, ok := rawSupported.([]interface{})
// 			if !ok {
// 				return FormatError{msg: fmt.Sprintf("Invalid supported_os type: %T", rawSupported)}
// 			}

// 			var sourceSupported []string
// 			for _, v := range rawList {
// 				s, ok := v.(string)
// 				if !ok {
// 					return FormatError{msg: fmt.Sprintf("supported_os element is not a string: %v", v)}
// 				}
// 				if !r.supportedOS[s] {
// 					return FormatError{msg: fmt.Sprintf("Artifact definition: %s undefined supported OS: %s", name, s)}
// 				}
// 				sourceSupported = append(sourceSupported, s)
// 			}

// 			for _, osStr := range sourceSupported {
// 				if !containsString(artifactDefinition.SupportedOS, osStr) {
// 					return FormatError{msg: fmt.Sprintf("Source OS %s not in artifact supported OS for: %s", osStr, name)}
// 				}
// 			}
// 		}
// 	}
// 	return nil
// }

// func containsString(slice []string, s string) bool {
// 	for _, item := range slice {
// 		if item == s {
// 			return true
// 		}
// 	}
// 	return false
// }

// func (r *ArtifactsReader) ReadArtifactDefinitionValues(artifactDefinitionValues map[string]interface{}) (*ArtifactDefinition, error) {
// 	if artifactDefinitionValues == nil {
// 		return nil, FormatError{msg: "Missing artifact definition values."}
// 	}

// 	for key := range artifactDefinitionValues {
// 		if !TOP_LEVEL_KEYS[key] {
// 			return nil, FormatError{msg: fmt.Sprintf("Undefined keys: %s", key)}
// 		}
// 	}

// 	rawName, exists := artifactDefinitionValues["name"]
// 	if !exists {
// 		return nil, FormatError{msg: "Invalid artifact definition missing name."}
// 	}
// 	name, ok := rawName.(string)
// 	if !ok || name == "" {
// 		return nil, FormatError{msg: "Invalid artifact definition missing name."}
// 	}

// 	rawDoc, exists := artifactDefinitionValues["doc"]
// 	if !exists {
// 		return nil, FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s missing description.", name)}
// 	}
// 	description, ok := rawDoc.(string)
// 	if !ok || description == "" {
// 		return nil, FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s missing description.", name)}
// 	}

// 	var aliases []string
// 	if rawAliases, exists := artifactDefinitionValues["aliases"]; exists {
// 		if aliasSlice, ok := rawAliases.([]interface{}); ok {
// 			for _, a := range aliasSlice {
// 				if as, ok := a.(string); ok {
// 					aliases = append(aliases, as)
// 				}
// 			}
// 		} else {
// 			return nil, FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s aliases is not a list.", name)}
// 		}
// 	}

// 	if rawCollectors, exists := artifactDefinitionValues["collectors"]; exists {
// 		if collectors, ok := rawCollectors.([]interface{}); ok && len(collectors) > 0 {
// 			return nil, FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s still uses collectors.", name)}
// 		}
// 	}

// 	artifactDef := NewArtifactDefinition(name, aliases, description)

// 	if rawURLs, exists := artifactDefinitionValues["urls"]; exists {
// 		urlList, ok := rawURLs.([]interface{})
// 		if !ok {
// 			return nil, FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s urls is not a list.", name)}
// 		}
// 		for _, v := range urlList {
// 			if s, ok := v.(string); ok {
// 				artifactDef.URLs = append(artifactDef.URLs, s)
// 			}
// 		}
// 	}

// 	if err := r._readSupportedOS(artifactDefinitionValues, artifactDef, name); err != nil {
// 		return nil, err
// 	}

// 	if err := r._readSources(artifactDefinitionValues, artifactDef, name); err != nil {
// 		return nil, err
// 	}

// 	return artifactDef, nil
// }

// func (r *ArtifactsReader) ReadDirectory(path string, extension string) ([]*ArtifactDefinition, error) {
// 	var definitions []*ArtifactDefinition
// 	var pattern string
// 	if extension != "" {
// 		pattern = filepath.Join(path, "*."+extension)
// 	} else {
// 		pattern = filepath.Join(path, "*")
// 	}

// 	files, err := filepath.Glob(pattern)
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, file := range files {
// 		defs, err := r.ReadFile(file)
// 		if err != nil {
// 			return nil, err
// 		}
// 		definitions = append(definitions, defs...)
// 	}
// 	return definitions, nil
// }

// func (r *ArtifactsReader) ReadFile(filename string) ([]*ArtifactDefinition, error) {
// 	f, err := os.Open(filename)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer f.Close()
// 	return r.ReadFileObject(f)
// }

// // ReadFileObject остаётся абстрактным и реализуется в конкретных читателях (JSON/YAML)
// func (r *ArtifactsReader) ReadFileObject(fileObject io.Reader) ([]*ArtifactDefinition, error) {
// 	return nil, fmt.Errorf("ReadFileObject not implemented")
// }

// // ======================================================================
// // JsonArtifactsReader – JSON‑читатель артефактов.
// // ======================================================================

// type JsonArtifactsReader struct {
// 	*ArtifactsReader
// }

// func NewJsonArtifactsReader() *JsonArtifactsReader {
// 	return &JsonArtifactsReader{
// 		ArtifactsReader: NewArtifactsReader(),
// 	}
// }

// func (r *JsonArtifactsReader) ReadFileObject(fileObject io.Reader) ([]*ArtifactDefinition, error) {
// 	data, err := io.ReadAll(fileObject)
// 	if err != nil {
// 		return nil, err
// 	}
// 	var jsonDefinitions []map[string]interface{}
// 	if err := json.Unmarshal(data, &jsonDefinitions); err != nil {
// 		return nil, FormatError{msg: err.Error()}
// 	}
// 	var definitionsList []*ArtifactDefinition
// 	var lastArtifact *ArtifactDefinition
// 	for _, defMap := range jsonDefinitions {
// 		artifactDef, err := r.ReadArtifactDefinitionValues(defMap)
// 		if err != nil {
// 			errorLocation := "At start"
// 			if lastArtifact != nil {
// 				errorLocation = fmt.Sprintf("After: %s", lastArtifact.Name)
// 			}
// 			return nil, FormatError{msg: fmt.Sprintf("%s %s", errorLocation, err.Error())}
// 		}
// 		definitionsList = append(definitionsList, artifactDef)
// 		lastArtifact = artifactDef
// 	}
// 	return definitionsList, nil
// }

// // ======================================================================
// // YamlArtifactsReader – YAML‑читатель артефактов.
// // ======================================================================

// type YamlArtifactsReader struct {
// 	*ArtifactsReader
// }

// func NewYamlArtifactsReader() *YamlArtifactsReader {
// 	return &YamlArtifactsReader{
// 		ArtifactsReader: NewArtifactsReader(),
// 	}
// }

// func (r *YamlArtifactsReader) ReadFileObject(fileObject io.Reader) ([]*ArtifactDefinition, error) {
// 	decoder := yaml.NewDecoder(fileObject)
// 	var definitionsList []*ArtifactDefinition
// 	var lastArtifact *ArtifactDefinition

// 	for {
// 		var doc map[string]interface{}
// 		err := decoder.Decode(&doc)
// 		if errors.Is(err, io.EOF) {
// 			break
// 		}
// 		if err != nil {
// 			return nil, FormatError{msg: err.Error()}
// 		}
// 		if doc == nil {
// 			continue
// 		}

// 		artifactDef, err := r.ReadArtifactDefinitionValues(doc)
// 		if err != nil {
// 			errorLocation := "At start"
// 			if lastArtifact != nil {
// 				errorLocation = fmt.Sprintf("After: %s", lastArtifact.Name)
// 			}
// 			return nil, FormatError{msg: fmt.Sprintf("%s %s", errorLocation, err.Error())}
// 		}

// 		definitionsList = append(definitionsList, artifactDef)
// 		lastArtifact = artifactDef
// 	}

// 	return definitionsList, nil
// }

// // func (r *YamlArtifactsReader) ReadFileObject(fileObject io.Reader) ([]*ArtifactDefinition, error) {
// // 	var yamlDefinitions []map[string]interface{}
// // 	if err := yaml.NewDecoder(fileObject).Decode(&yamlDefinitions); err != nil {
// // 		return nil, FormatError{msg: err.Error()}
// // 	}

// // 	var definitionsList []*ArtifactDefinition
// // 	for _, defMap := range yamlDefinitions {
// // 		artifactDef, err := r.ReadArtifactDefinitionValues(defMap)
// // 		if err != nil {
// // 			return nil, FormatError{msg: fmt.Sprintf("Error in artifact: %v", err)}
// // 		}
// // 		definitionsList = append(definitionsList, artifactDef)
// // 	}
// // 	return definitionsList, nil
// // }

// // ФУНКЦИЮ ПРИ НЕОБХОДИМОСТИ РАЗМНОЖИТЬ ДЛЯ ВСЕХ ТИПОВ func (r *YamlArtifactsReader)
// func (r *YamlArtifactsReader) ReadFile(filename string) ([]*ArtifactDefinition, error) {
// 	f, err := os.Open(filename)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer f.Close()
// 	return r.ReadFileObject(f) // вызов метода ReadFileObject из *YamlArtifactsReader
// }

// // ФУНКЦИЮ ПРИ НЕОБХОДИМОСТИ РАЗМНОЖИТЬ ДЛЯ ВСЕХ ТИПОВ func (r *YamlArtifactsReader)
// func (r *JsonArtifactsReader) ReadFile(filename string) ([]*ArtifactDefinition, error) {
// 	f, err := os.Open(filename)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer f.Close()
// 	return r.ReadFileObject(f) // вызов метода ReadFileObject из *YamlArtifactsReader
// }

// // ======================================================================
// // Пример использования
// // ======================================================================

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// / Интерфейс для чтения артефактов.
type ArtifactsReaderInterface interface {
	ReadArtifactDefinitionValues(definitionValues map[string]interface{}) (*ArtifactDefinition, error)
	ReadDirectory(path string, extension string) ([]*ArtifactDefinition, error)
	ReadFile(filename string) ([]*ArtifactDefinition, error)
	ReadFileObject(fileObject io.Reader) ([]*ArtifactDefinition, error)
}

type BaseArtifactsReader struct {
	supportedOS map[string]bool
}

func NewBaseArtifactsReader() *BaseArtifactsReader {
	return &BaseArtifactsReader{
		supportedOS: map[string]bool{},
	}
}

// ArtifactsReader – реализация общих методов.
type ArtifactsReader struct {
	*BaseArtifactsReader
}

func NewArtifactsReader() *ArtifactsReader {
	supported := make(map[string]bool)
	for osName, ok := range SUPPORTED_OS {
		if ok {
			supported[osName] = true
		}
	}
	return &ArtifactsReader{
		BaseArtifactsReader: &BaseArtifactsReader{
			supportedOS: supported,
		},
	}
}

// _readSupportedOS читает поле supported_os из definitionValues и,
// если оно задано, присваивает его объекту artifactDefinition.
// Здесь объектом является только ArtifactDefinition, т.к. Source не содержит данного поля.
func (r *ArtifactsReader) _readSupportedOS(definitionValues map[string]interface{}, artifactDefinition *ArtifactDefinition, name string) error {
	raw, exists := definitionValues["supported_os"]
	if exists {
		rawList, ok := raw.([]interface{})
		if !ok {
			return FormatError{msg: fmt.Sprintf("Invalid supported_os type: %T", raw)}
		}
		var supportedOSList []string
		for _, v := range rawList {
			s, ok := v.(string)
			if !ok {
				return FormatError{msg: fmt.Sprintf("supported_os element is not a string: %v", v)}
			}
			if !r.supportedOS[s] {
				return FormatError{msg: fmt.Sprintf("Artifact definition: %s undefined supported operating system: %s.", name, s)}
			}
			supportedOSList = append(supportedOSList, s)
		}
		artifactDefinition.SupportedOS = supportedOSList
	}
	return nil
}

// _readSources читает список источников и добавляет их в artifactDefinition.
func (r *ArtifactsReader) _readSources(artifactDefinitionValues map[string]interface{}, artifactDefinition *ArtifactDefinition, name string) error {
	rawSources, exists := artifactDefinitionValues["sources"]
	if !exists {
		return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s missing sources.", name)}
	}
	sourcesList, ok := rawSources.([]interface{})
	if !ok {
		return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s sources is not a list.", name)}
	}

	for _, rawSource := range sourcesList {
		sourceMap, ok := rawSource.(map[string]interface{})
		if !ok {
			return FormatError{msg: fmt.Sprintf("Invalid source format in artifact: %s", name)}
		}
		rawType, exists := sourceMap["type"]
		if !exists {
			return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s source type.", name)}
		}
		typeIndicator, ok := rawType.(string)
		if !ok || typeIndicator == "" {
			return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s source type.", name)}
		}
		var attributes map[string]interface{}
		rawAttr, exists := sourceMap["attributes"]
		if exists {
			attributes, ok = rawAttr.(map[string]interface{})
			if !ok {
				return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s attributes is not a map.", name)}
			}
		}

		if _, exists := sourceMap["returned_types"]; exists {
			return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s returned_types no longer supported.", name)}
		}

		_, err := artifactDefinition.AppendSource(typeIndicator, attributes)
		if err != nil {
			return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s, with error: %s", name, err.Error())}
		}

		// В оригинальном Python‑коде здесь выполнялась проверка supported_os для источника:
		//   - читалось поле supported_os из source (если задано)
		//   - проверялось, что все его значения содержатся в наборе поддерживаемых ОС,
		//     а также что они являются подмножеством supported_os самого artifactDefinition.
		// Здесь выполняем аналогичную проверку.
		if rawSupported, exists := sourceMap["supported_os"]; exists {
			rawList, ok := rawSupported.([]interface{})
			if !ok {
				return FormatError{msg: fmt.Sprintf("Invalid supported_os type: %T", rawSupported)}
			}
			var sourceSupported []string
			for _, v := range rawList {
				s, ok := v.(string)
				if !ok {
					return FormatError{msg: fmt.Sprintf("supported_os element is not a string: %v", v)}
				}
				// Проверка, что ОС определена.
				if !r.supportedOS[s] {
					return FormatError{msg: fmt.Sprintf("Artifact definition: %s undefined supported operating system: %s.", name, s)}
				}
				sourceSupported = append(sourceSupported, s)
			}
			// Проверка, что все ОС источника содержатся в artifactDefinition.SupportedOS.
			for _, osStr := range sourceSupported {
				if !containsString(artifactDefinition.SupportedOS, osStr) {
					return FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s missing supported_os.", name)}
				}
			}
		}
	}
	return nil
}
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (r *ArtifactsReader) ReadArtifactDefinitionValues(artifactDefinitionValues map[string]interface{}) (*ArtifactDefinition, error) {
	if artifactDefinitionValues == nil {
		return nil, FormatError{msg: "Missing artifact definition values."}
	}

	for key := range artifactDefinitionValues {
		if !TOP_LEVEL_KEYS[key] {
			return nil, FormatError{msg: fmt.Sprintf("Undefined keys: %s", key)}
		}
	}

	rawName, exists := artifactDefinitionValues["name"]
	if !exists {
		return nil, FormatError{msg: "Invalid artifact definition missing name."}
	}
	name, ok := rawName.(string)
	if !ok || name == "" {
		return nil, FormatError{msg: "Invalid artifact definition missing name."}
	}

	rawDoc, exists := artifactDefinitionValues["doc"]
	if !exists {
		return nil, FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s missing description.", name)}
	}
	description, ok := rawDoc.(string)
	if !ok || description == "" {
		return nil, FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s missing description.", name)}
	}

	var aliases []string
	if rawAliases, exists := artifactDefinitionValues["aliases"]; exists {
		if aliasSlice, ok := rawAliases.([]interface{}); ok {
			for _, a := range aliasSlice {
				if as, ok := a.(string); ok {
					aliases = append(aliases, as)
				}
			}
		} else {
			return nil, FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s aliases is not a list.", name)}
		}
	}

	if rawCollectors, exists := artifactDefinitionValues["collectors"]; exists {
		if collectors, ok := rawCollectors.([]interface{}); ok && len(collectors) > 0 {
			return nil, FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s still uses collectors.", name)}
		}
	}

	artifactDef := NewArtifactDefinition(name, aliases, description)

	if rawURLs, exists := artifactDefinitionValues["urls"]; exists {
		urlList, ok := rawURLs.([]interface{})
		if !ok {
			return nil, FormatError{msg: fmt.Sprintf("Invalid artifact definition: %s urls is not a list.", name)}
		}
		for _, v := range urlList {
			if s, ok := v.(string); ok {
				artifactDef.URLs = append(artifactDef.URLs, s)
			}
		}
	}

	if err := r._readSupportedOS(artifactDefinitionValues, artifactDef, name); err != nil {
		return nil, err
	}

	if err := r._readSources(artifactDefinitionValues, artifactDef, name); err != nil {
		return nil, err
	}

	return artifactDef, nil
}

func (r *ArtifactsReader) ReadDirectory(path string, extension string) ([]*ArtifactDefinition, error) {
	var definitions []*ArtifactDefinition
	var pattern string
	if extension != "" {
		pattern = filepath.Join(path, "*."+extension)
	} else {
		pattern = filepath.Join(path, "*")
	}

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		defs, err := r.ReadFile(file)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	}
	return definitions, nil
}

func (r *ArtifactsReader) ReadFile(filename string) ([]*ArtifactDefinition, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return r.ReadFileObject(f)
}

// ReadFileObject остаётся абстрактным и реализуется в конкретных читателях (JSON/YAML)
func (r *ArtifactsReader) ReadFileObject(fileObject io.Reader) ([]*ArtifactDefinition, error) {
	return nil, fmt.Errorf("ReadFileObject not implemented")
}

// ======================================================================
// JsonArtifactsReader – JSON‑читатель артефактов.
// ======================================================================

type JsonArtifactsReader struct {
	*ArtifactsReader
}

func NewJsonArtifactsReader() *JsonArtifactsReader {
	return &JsonArtifactsReader{
		ArtifactsReader: NewArtifactsReader(),
	}
}

func (r *JsonArtifactsReader) ReadFileObject(fileObject io.Reader) ([]*ArtifactDefinition, error) {
	data, err := io.ReadAll(fileObject)
	if err != nil {
		return nil, err
	}
	var jsonDefinitions []map[string]interface{}
	if err := json.Unmarshal(data, &jsonDefinitions); err != nil {
		return nil, FormatError{msg: err.Error()}
	}
	var definitionsList []*ArtifactDefinition
	var lastArtifact *ArtifactDefinition
	for _, defMap := range jsonDefinitions {
		artifactDef, err := r.ReadArtifactDefinitionValues(defMap)
		if err != nil {
			errorLocation := "At start"
			if lastArtifact != nil {
				errorLocation = fmt.Sprintf("After: %s", lastArtifact.Name)
			}
			return nil, FormatError{msg: fmt.Sprintf("%s %s", errorLocation, err.Error())}
		}
		definitionsList = append(definitionsList, artifactDef)
		lastArtifact = artifactDef
	}
	return definitionsList, nil
}

// ======================================================================
// YamlArtifactsReader – YAML‑читатель артефактов.
// ======================================================================

type YamlArtifactsReader struct {
	*ArtifactsReader
}

func NewYamlArtifactsReader() *YamlArtifactsReader {
	return &YamlArtifactsReader{
		ArtifactsReader: NewArtifactsReader(),
	}
}

// func (r *YamlArtifactsReader) ReadFileObject(fileObject io.Reader) ([]*ArtifactDefinition, error) {
// 	decoder := yaml.NewDecoder(fileObject)
// 	var definitionsList []*ArtifactDefinition
// 	var lastArtifact *ArtifactDefinition
// 	for {
// 		var doc map[string]interface{}
// 		err := decoder.Decode(&doc)
// 		if err == io.EOF {
// 			break
// 		}
// 		if err != nil {
// 			return nil, FormatError{msg: err.Error()}
// 		}
// 		if doc == nil {
// 			continue
// 		}
// 		artifactDef, err := r.ReadArtifactDefinitionValues(doc)
// 		if err != nil {
// 			errorLocation := "At start"
// 			if lastArtifact != nil {
// 				errorLocation = fmt.Sprintf("After: %s", lastArtifact.Name)
// 			}
// 			return nil, FormatError{msg: fmt.Sprintf("%s %s", errorLocation, err.Error())}
// 		}
// 		definitionsList = append(definitionsList, artifactDef)
// 		lastArtifact = artifactDef
// 	}
// 	return definitionsList, nil
// }

// func (r *YamlArtifactsReader) ReadFileObject(fileObject io.Reader) ([]*ArtifactDefinition, error) {
// 	var yamlDefinitions []map[string]interface{}
// 	if err := yaml.NewDecoder(fileObject).Decode(&yamlDefinitions); err != nil {
// 		return nil, FormatError{msg: err.Error()}
// 	}

// 	var definitionsList []*ArtifactDefinition
// 	for _, defMap := range yamlDefinitions {
// 		artifactDef, err := r.ReadArtifactDefinitionValues(defMap)
// 		if err != nil {
// 			return nil, FormatError{msg: fmt.Sprintf("Error in artifact: %v", err)}
// 		}
// 		definitionsList = append(definitionsList, artifactDef)
// 	}
// 	return definitionsList, nil
// }

// ФУНКЦИЮ ПРИ НЕОБХОДИМОСТИ РАЗМНОЖИТЬ ДЛЯ ВСЕХ ТИПОВ func (r *YamlArtifactsReader)

func (r *YamlArtifactsReader) ReadFileObject(fileObject io.Reader) ([]*ArtifactDefinition, error) {
	// Читаем всё содержимое файла.
	data, err := ioutil.ReadAll(fileObject)
	if err != nil {
		return nil, err
	}

	// Пытаемся декодировать содержимое как один документ,
	// представляющий собой срез артефакт-описаний.
	var docs []map[string]interface{}
	if err := yaml.Unmarshal(data, &docs); err == nil && len(docs) > 0 {
		definitions := make([]*ArtifactDefinition, 0, len(docs))
		var lastArtifact *ArtifactDefinition
		for _, docMap := range docs {
			artifactDef, err := r.ReadArtifactDefinitionValues(docMap)
			if err != nil {
				errorLocation := "At start"
				if lastArtifact != nil {
					errorLocation = fmt.Sprintf("After: %s", lastArtifact.Name)
				}
				return nil, FormatError{msg: fmt.Sprintf("%s %s", errorLocation, err.Error())}
			}
			definitions = append(definitions, artifactDef)
			lastArtifact = artifactDef
		}
		return definitions, nil
	}

	// Если предыдущая попытка не сработала, пробуем обрабатывать как много-документный YAML.
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var definitions []*ArtifactDefinition
	for {
		var doc map[string]interface{}
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, FormatError{msg: err.Error()}
		}
		// Если документ пустой, переходим к следующему.
		if doc == nil {
			continue
		}
		artifactDef, err := r.ReadArtifactDefinitionValues(doc)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, artifactDef)
	}
	return definitions, nil
}

func (r *YamlArtifactsReader) ReadFile(filename string) ([]*ArtifactDefinition, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return r.ReadFileObject(f) // вызов метода ReadFileObject из *YamlArtifactsReader
}

// ФУНКЦИЮ ПРИ НЕОБХОДИМОСТИ РАЗМНОЖИТЬ ДЛЯ ВСЕХ ТИПОВ func (r *YamlArtifactsReader)
func (r *JsonArtifactsReader) ReadFile(filename string) ([]*ArtifactDefinition, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return r.ReadFileObject(f) // вызов метода ReadFileObject из *YamlArtifactsReader
}

// Добавляем метод ReadDirectory в YamlArtifactsReader
func (r *YamlArtifactsReader) ReadDirectory(path string, extension string) ([]*ArtifactDefinition, error) {
	var definitions []*ArtifactDefinition
	pattern := filepath.Join(path, "*."+extension)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		defs, err := r.ReadFile(file)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	}
	return definitions, nil
}

// Аналогично для JsonArtifactsReader (если требуется)
func (r *JsonArtifactsReader) ReadDirectory(path string, extension string) ([]*ArtifactDefinition, error) {
	var definitions []*ArtifactDefinition
	pattern := filepath.Join(path, "*."+extension)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		defs, err := r.ReadFile(file)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, defs...)
	}
	return definitions, nil
}
