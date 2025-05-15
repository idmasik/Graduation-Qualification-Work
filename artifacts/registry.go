package main

import (
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
