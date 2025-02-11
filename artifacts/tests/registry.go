// package main

// import (
// 	"errors"
// 	"fmt"
// 	"strings"
// )

// // ArtifactDefinitionsRegistry управляет регистрацией определений артефактов и их источников.
// type ArtifactDefinitionsRegistry struct {
// 	sourceTypeFactory          *SourceTypeFactory
// 	artifactDefinitionsByName  map[string]*ArtifactDefinition
// 	artifactDefinitionsByAlias map[string]*ArtifactDefinition
// 	definedArtifactNames       map[string]struct{}
// 	artifactNameReferences     map[string]struct{}
// }

// // NewArtifactDefinitionsRegistry создает новый реестр артефактов.
// func NewArtifactDefinitionsRegistry() *ArtifactDefinitionsRegistry {
// 	return &ArtifactDefinitionsRegistry{
// 		sourceTypeFactory:          NewSourceTypeFactory(),
// 		artifactDefinitionsByName:  make(map[string]*ArtifactDefinition),
// 		artifactDefinitionsByAlias: make(map[string]*ArtifactDefinition),
// 		definedArtifactNames:       make(map[string]struct{}),
// 		artifactNameReferences:     make(map[string]struct{}),
// 	}

// }

// // CreateSourceType создает объект типа источника на основе индикатора и атрибутов.
// func (r *ArtifactDefinitionsRegistry) CreateSourceType(typeIndicator string, attributes map[string]interface{}) (SourceType, error) {
// 	return r.sourceTypeFactory.CreateSourceType(typeIndicator, attributes)
// }

// // DeregisterDefinition удаляет регистрацию определения артефакта.
// func (r *ArtifactDefinitionsRegistry) DeregisterDefinition(definition *ArtifactDefinition) error {
// 	nameKey := strings.ToLower(definition.Name)
// 	if _, exists := r.artifactDefinitionsByName[nameKey]; !exists {
// 		return fmt.Errorf("artifact definition not set for name: %s", definition.Name)
// 	}

// 	for _, alias := range definition.Aliases {
// 		aliasKey := strings.ToLower(alias)
// 		if _, exists := r.artifactDefinitionsByAlias[aliasKey]; !exists {
// 			return fmt.Errorf("artifact definition not set for alias: %s", alias)
// 		}
// 	}

// 	delete(r.artifactDefinitionsByName, nameKey)
// 	delete(r.definedArtifactNames, definition.Name)

// 	for _, alias := range definition.Aliases {
// 		delete(r.artifactDefinitionsByAlias, strings.ToLower(alias))
// 	}

// 	// Удаление ссылок на группы артефактов
// 	for _, source := range definition.Sources {
// 		if source.TypeIndicator == TYPE_INDICATOR_ARTIFACT_GROUP {
// 			for _, name := range source.Attributes["names"].([]string) {
// 				delete(r.artifactNameReferences, name)
// 			}
// 		}
// 	}

// 	return nil
// }

// // GetDefinitionByAlias возвращает артефакт по алиасу.
// func (r *ArtifactDefinitionsRegistry) GetDefinitionByAlias(alias string) *ArtifactDefinition {
// 	if alias == "" {
// 		return nil
// 	}
// 	return r.artifactDefinitionsByAlias[strings.ToLower(alias)]
// }

// // GetDefinitionByName возвращает артефакт по имени.
// func (r *ArtifactDefinitionsRegistry) GetDefinitionByName(name string) *ArtifactDefinition {
// 	if name == "" {
// 		return nil
// 	}
// 	return r.artifactDefinitionsByName[strings.ToLower(name)]
// }

// // GetDefinitions возвращает все зарегистрированные артефакты.
// func (r *ArtifactDefinitionsRegistry) GetDefinitions() []*ArtifactDefinition {
// 	definitions := make([]*ArtifactDefinition, 0, len(r.artifactDefinitionsByName))
// 	for _, def := range r.artifactDefinitionsByName {
// 		definitions = append(definitions, def)
// 	}
// 	return definitions
// }

// // GetUndefinedArtifacts возвращает имена неопределенных артефактов.
// func (r *ArtifactDefinitionsRegistry) GetUndefinedArtifacts() []string {
// 	undefined := make([]string, 0)
// 	for name := range r.artifactNameReferences {
// 		if _, defined := r.definedArtifactNames[name]; !defined {
// 			undefined = append(undefined, name)
// 		}
// 	}
// 	return undefined
// }

// // RegisterDefinition регистрирует новое определение артефакта.
// func (r *ArtifactDefinitionsRegistry) RegisterDefinition(definition *ArtifactDefinition) error {
// 	nameKey := strings.ToLower(definition.Name)
// 	if _, exists := r.artifactDefinitionsByName[nameKey]; exists {
// 		return fmt.Errorf("artifact definition already set for name: %s", definition.Name)
// 	}

// 	for _, alias := range definition.Aliases {
// 		aliasKey := strings.ToLower(alias)
// 		if _, exists := r.artifactDefinitionsByAlias[aliasKey]; exists {
// 			return fmt.Errorf("artifact definition already set for alias: %s", alias)
// 		}
// 		if _, exists := r.artifactDefinitionsByName[aliasKey]; exists {
// 			return fmt.Errorf("alias '%s' conflicts with existing artifact name", alias)
// 		}
// 	}

// 	r.artifactDefinitionsByName[nameKey] = definition
// 	r.definedArtifactNames[definition.Name] = struct{}{}

// 	for _, alias := range definition.Aliases {
// 		r.artifactDefinitionsByAlias[strings.ToLower(alias)] = definition
// 	}

// 	// Обработка ссылок на группы артефактов
// 	for _, source := range definition.Sources {
// 		if source.TypeIndicator == TYPE_INDICATOR_ARTIFACT_GROUP {
// 			if names, ok := source.Attributes["names"].([]string); ok {
// 				for _, name := range names {
// 					r.artifactNameReferences[name] = struct{}{}
// 				}
// 			}
// 		}
// 	}

// 	return nil
// }

// // // RegisterSourceType регистрирует новый тип источника.
// // func (r *ArtifactDefinitionsRegistry) RegisterSourceType(sourceType SourceType) error {
// // 	indicator := sourceType.TypeIndicator()
// // 	if r.sourceTypeFactory.sourceTypeConstructors[indicator] != nil {
// // 		return fmt.Errorf("source type already registered for indicator: %s", indicator)
// // 	}

// // 	// Регистрация конструктора для типа
// // 	r.sourceTypeFactory.RegisterSourceType(indicator, func(attrs map[string]interface{}) (SourceType, error) {
// // 		// Здесь должна быть логика создания конкретного типа на основе атрибутов
// // 		// Пример для ArtifactGroupSourceType:
// // 		if indicator == TYPE_INDICATOR_ARTIFACT_GROUP {
// // 			names, ok := attrs["names"].([]string)
// // 			if !ok || len(names) == 0 {
// // 				return nil, errors.New("invalid names for artifact group")
// // 			}
// // 			return NewArtifactGroupSourceType(names)
// // 		}
// // 		return nil, fmt.Errorf("unsupported type: %s", indicator)
// // 	})

// // 	return nil
// // }

// // RegisterSourceType регистрирует новый тип источника.
// func (r *ArtifactDefinitionsRegistry) RegisterSourceType(sourceType SourceType) error {
// 	indicator := sourceType.TypeIndicator()
// 	if r.sourceTypeFactory.sourceTypeConstructors[indicator] != nil {
// 		return fmt.Errorf("source type already registered for indicator: %s", indicator)
// 	}

// 	// Регистрация конструктора для типа
// 	r.sourceTypeFactory.RegisterSourceType(indicator, func(attrs map[string]interface{}) (SourceType, error) {
// 		switch indicator {
// 		case TYPE_INDICATOR_ARTIFACT_GROUP:
// 			names, ok := attrs["names"].([]string)
// 			if !ok || len(names) == 0 {
// 				return nil, errors.New("invalid names for artifact group")
// 			}
// 			return NewArtifactGroupSourceType(names)

// 		case TYPE_INDICATOR_COMMAND:
// 			cmd, ok1 := attrs["cmd"].(string)
// 			args, ok2 := attrs["args"].([]string)
// 			if !ok1 || !ok2 || cmd == "" || len(args) == 0 {
// 				return nil, errors.New("missing cmd or args for command source")
// 			}
// 			return NewCommandSourceType(cmd, args)

// 		case TYPE_INDICATOR_DIRECTORY:
// 			paths, ok := convertToStringSlice(attrs["paths"])
// 			if !ok || len(paths) == 0 {
// 				return nil, errors.New("invalid paths for directory source")
// 			}
// 			separator, _ := attrs["separator"].(string)
// 			return NewDirectorySourceType(paths, separator)

// 		case TYPE_INDICATOR_FILE:
// 			// Получаем paths из атрибутов
// 			pathsInterface, ok := attrs["paths"]
// 			if !ok {
// 				return nil, errors.New("missing paths for file source")
// 			}

// 			// Проверяем тип paths
// 			pathsSlice, ok := pathsInterface.([]interface{})
// 			if !ok {
// 				return nil, errors.New("paths must be a list")
// 			}

// 			// Конвертируем элементы в строки
// 			var paths []string
// 			for _, p := range pathsSlice {
// 				pathStr, ok := p.(string)
// 				if !ok {
// 					return nil, errors.New("path must be a string")
// 				}
// 				paths = append(paths, pathStr)
// 			}

// 			// Проверяем наличие хотя бы одного пути
// 			if len(paths) == 0 {
// 				return nil, errors.New("empty paths list for file source")
// 			}

// 			// Получаем разделитель (с дефолтным значением)
// 			separator, _ := attrs["separator"].(string)
// 			if separator == "" {
// 				separator = "/"
// 			}

// 			return NewFileSourceType(paths, separator)

// 		case TYPE_INDICATOR_PATH:
// 			paths, ok := convertToStringSlice(attrs["paths"])
// 			if !ok || len(paths) == 0 {
// 				return nil, errors.New("invalid paths for path source")
// 			}
// 			separator, _ := attrs["separator"].(string)
// 			return NewPathSourceType(paths, separator)

// 		case TYPE_INDICATOR_WINDOWS_REGISTRY_KEY:
// 			keys, ok := convertToStringSlice(attrs["keys"])
// 			if !ok || len(keys) == 0 {
// 				return nil, errors.New("invalid keys for registry key source")
// 			}
// 			return NewWindowsRegistryKeySourceType(keys)

// 		case TYPE_INDICATOR_WINDOWS_REGISTRY_VALUE:
// 			pairs, ok := attrs["key_value_pairs"].([]map[string]string)
// 			if !ok || len(pairs) == 0 {
// 				return nil, errors.New("invalid key-value pairs for registry value source")
// 			}
// 			return NewWindowsRegistryValueSourceType(pairs)

// 		case TYPE_INDICATOR_WMI_QUERY:
// 			query, ok := attrs["query"].(string)
// 			if !ok || query == "" {
// 				return nil, errors.New("missing query for WMI source")
// 			}
// 			baseObj, _ := attrs["base_object"].(string)
// 			return NewWMIQuerySourceType(baseObj, query)

// 		default:
// 			return nil, fmt.Errorf("unsupported type: %s", indicator)
// 		}
// 	})

// 	return nil
// }

// Вспомогательная функция для конвертации интерфейса в []string

// // ArtifactDefinition представляет полное определение артефакта

// type ArtifactDefinition struct {
// 	Name        string
// 	Aliases     []string
// 	Description string
// 	Sources     []*Source
// 	SupportedOS []string
// 	URLs        []string
// }

// type Source struct {
// 	TypeIndicator string
// 	Attributes    map[string]interface{}
// }

// ///////////
package main

import (
	"errors"
	"fmt"
	"strings"
)

// ArtifactDefinitionsRegistry управляет регистрацией определений артефактов и их источников.
type ArtifactDefinitionsRegistry struct {
	sourceTypeFactory          *SourceTypeFactory
	artifactDefinitionsByName  map[string]*ArtifactDefinition
	artifactDefinitionsByAlias map[string]*ArtifactDefinition
	definedArtifactNames       map[string]struct{}
	artifactNameReferences     map[string]struct{}
}

// NewArtifactDefinitionsRegistry создает новый реестр артефактов.
func NewArtifactDefinitionsRegistry() *ArtifactDefinitionsRegistry {
	return &ArtifactDefinitionsRegistry{
		sourceTypeFactory:          NewSourceTypeFactory(),
		artifactDefinitionsByName:  make(map[string]*ArtifactDefinition),
		artifactDefinitionsByAlias: make(map[string]*ArtifactDefinition),
		definedArtifactNames:       make(map[string]struct{}),
		artifactNameReferences:     make(map[string]struct{}),
	}
}

// CreateSourceType создает объект типа источника на основе индикатора и атрибутов.
func (r *ArtifactDefinitionsRegistry) CreateSourceType(typeIndicator string, attributes map[string]interface{}) (SourceType, error) {
	return r.sourceTypeFactory.CreateSourceType(typeIndicator, attributes)
}

// DeregisterDefinition удаляет регистрацию определения артефакта.
func (r *ArtifactDefinitionsRegistry) DeregisterDefinition(definition *ArtifactDefinition) error {
	nameKey := strings.ToLower(definition.Name)
	if _, exists := r.artifactDefinitionsByName[nameKey]; !exists {
		return fmt.Errorf("artifact definition not set for name: %s", definition.Name)
	}

	for _, alias := range definition.Aliases {
		aliasKey := strings.ToLower(alias)
		if _, exists := r.artifactDefinitionsByAlias[aliasKey]; !exists {
			return fmt.Errorf("artifact definition not set for alias: %s", alias)
		}
	}

	delete(r.artifactDefinitionsByName, nameKey)
	delete(r.definedArtifactNames, definition.Name)

	for _, alias := range definition.Aliases {
		delete(r.artifactDefinitionsByAlias, strings.ToLower(alias))
	}

	// Удаление ссылок на группы артефактов
	for _, source := range definition.Sources {
		if source.TypeIndicator == TYPE_INDICATOR_ARTIFACT_GROUP {
			for _, name := range source.Attributes["names"].([]string) {
				delete(r.artifactNameReferences, name)
			}
		}
	}

	return nil
}

// GetDefinitionByAlias возвращает артефакт по алиасу.
func (r *ArtifactDefinitionsRegistry) GetDefinitionByAlias(alias string) *ArtifactDefinition {
	if alias == "" {
		return nil
	}
	return r.artifactDefinitionsByAlias[strings.ToLower(alias)]
}

// GetDefinitionByName возвращает артефакт по имени.
func (r *ArtifactDefinitionsRegistry) GetDefinitionByName(name string) *ArtifactDefinition {
	if name == "" {
		return nil
	}
	return r.artifactDefinitionsByName[strings.ToLower(name)]
}

// GetDefinitions возвращает все зарегистрированные артефакты.
func (r *ArtifactDefinitionsRegistry) GetDefinitions() []*ArtifactDefinition {
	definitions := make([]*ArtifactDefinition, 0, len(r.artifactDefinitionsByName))
	for _, def := range r.artifactDefinitionsByName {
		definitions = append(definitions, def)
	}
	return definitions
}

// GetUndefinedArtifacts возвращает имена неопределенных артефактов.
func (r *ArtifactDefinitionsRegistry) GetUndefinedArtifacts() []string {
	undefined := make([]string, 0)
	for name := range r.artifactNameReferences {
		if _, defined := r.definedArtifactNames[name]; !defined {
			undefined = append(undefined, name)
		}
	}
	return undefined
}

// RegisterDefinition регистрирует новое определение артефакта.
func (r *ArtifactDefinitionsRegistry) RegisterDefinition(definition *ArtifactDefinition) error {
	nameKey := strings.ToLower(definition.Name)
	if _, exists := r.artifactDefinitionsByName[nameKey]; exists {
		return fmt.Errorf("artifact definition already set for name: %s", definition.Name)
	}

	for _, alias := range definition.Aliases {
		aliasKey := strings.ToLower(alias)
		if _, exists := r.artifactDefinitionsByAlias[aliasKey]; exists {
			return fmt.Errorf("artifact definition already set for alias: %s", alias)
		}
		if _, exists := r.artifactDefinitionsByName[aliasKey]; exists {
			return fmt.Errorf("alias '%s' conflicts with existing artifact name", alias)
		}
	}

	r.artifactDefinitionsByName[nameKey] = definition
	r.definedArtifactNames[definition.Name] = struct{}{}

	for _, alias := range definition.Aliases {
		r.artifactDefinitionsByAlias[strings.ToLower(alias)] = definition
	}

	// Обработка ссылок на группы артефактов
	for _, source := range definition.Sources {
		if source.TypeIndicator == TYPE_INDICATOR_ARTIFACT_GROUP {
			if names, ok := source.Attributes["names"].([]string); ok {
				for _, name := range names {
					r.artifactNameReferences[name] = struct{}{}
				}
			}
		}
	}

	return nil
}

// RegisterSourceType регистрирует новый тип источника.
func (r *ArtifactDefinitionsRegistry) RegisterSourceType(sourceType SourceType) error {
	indicator := sourceType.TypeIndicator()
	if r.sourceTypeFactory.sourceTypeConstructors[indicator] != nil {
		return fmt.Errorf("source type already registered for indicator: %s", indicator)
	}

	// Регистрация конструктора для типа
	r.sourceTypeFactory.RegisterSourceType(indicator, func(attrs map[string]interface{}) (SourceType, error) {
		switch indicator {
		case TYPE_INDICATOR_ARTIFACT_GROUP:
			names, ok := attrs["names"].([]string)
			if !ok || len(names) == 0 {
				return nil, errors.New("invalid names for artifact group")
			}
			return NewArtifactGroupSourceType(names)

		case TYPE_INDICATOR_COMMAND:
			cmd, ok1 := attrs["cmd"].(string)
			args, ok2 := attrs["args"].([]string)
			if !ok1 || !ok2 || cmd == "" || len(args) == 0 {
				return nil, errors.New("missing cmd or args for command source")
			}
			return NewCommandSourceType(cmd, args)

		case TYPE_INDICATOR_DIRECTORY:
			paths, ok := convertToStringSlice(attrs["paths"])
			if !ok || len(paths) == 0 {
				return nil, errors.New("invalid paths for directory source")
			}
			separator, _ := attrs["separator"].(string)
			return NewDirectorySourceType(paths, separator)

		case TYPE_INDICATOR_FILE:
			// Получаем paths из атрибутов
			pathsInterface, ok := attrs["paths"]
			if !ok {
				return nil, errors.New("missing paths for file source")
			}

			// Проверяем тип paths
			pathsSlice, ok := pathsInterface.([]interface{})
			if !ok {
				return nil, errors.New("paths must be a list")
			}

			// Конвертируем элементы в строки
			var paths []string
			for _, p := range pathsSlice {
				pathStr, ok := p.(string)
				if !ok {
					return nil, errors.New("path must be a string")
				}
				paths = append(paths, pathStr)
			}

			// Проверяем наличие хотя бы одного пути
			if len(paths) == 0 {
				return nil, errors.New("empty paths list for file source")
			}

			// Получаем разделитель (с дефолтным значением)
			separator, _ := attrs["separator"].(string)
			if separator == "" {
				separator = "/"
			}

			return NewFileSourceType(paths, separator)

		case TYPE_INDICATOR_PATH:
			paths, ok := convertToStringSlice(attrs["paths"])
			if !ok || len(paths) == 0 {
				return nil, errors.New("invalid paths for path source")
			}
			separator, _ := attrs["separator"].(string)
			return NewPathSourceType(paths, separator)

		case TYPE_INDICATOR_WINDOWS_REGISTRY_KEY:
			keys, ok := convertToStringSlice(attrs["keys"])
			if !ok || len(keys) == 0 {
				return nil, errors.New("invalid keys for registry key source")
			}
			return NewWindowsRegistryKeySourceType(keys)

		case TYPE_INDICATOR_WINDOWS_REGISTRY_VALUE:
			pairs, ok := attrs["key_value_pairs"].([]map[string]string)
			if !ok || len(pairs) == 0 {
				return nil, errors.New("invalid key-value pairs for registry value source")
			}
			return NewWindowsRegistryValueSourceType(pairs)

		case TYPE_INDICATOR_WMI_QUERY:
			query, ok := attrs["query"].(string)
			if !ok || query == "" {
				return nil, errors.New("missing query for WMI source")
			}
			baseObj, _ := attrs["base_object"].(string)
			return NewWMIQuerySourceType(baseObj, query)

		default:
			return nil, fmt.Errorf("unsupported type: %s", indicator)
		}
	})

	return nil
}

func convertToStringSlice(val interface{}) ([]string, bool) {
	slice, ok := val.([]interface{})
	if !ok {
		return nil, false
	}

	result := make([]string, len(slice))
	for i, v := range slice {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		result[i] = s
	}
	return result, true
}

// ArtifactDefinition представляет полное определение артефакта

type ArtifactDefinition struct {
	Name        string
	Aliases     []string
	Description string
	Sources     []*Source
	SupportedOS []string
	URLs        []string
}

type Source struct {
	TypeIndicator string
	Attributes    map[string]interface{}
}
