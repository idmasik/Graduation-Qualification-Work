package main

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

// NewArtifactDefinition создаёт новый объект ArtifactDefinition.
func NewArtifactDefinition(name string, aliases []string, description string) *ArtifactDefinition {
	return &ArtifactDefinition{
		Name:        name,
		Aliases:     aliases,
		Description: description,
		Sources:     []*Source{},
		SupportedOS: []string{},
		URLs:        []string{},
	}
}

// AppendSource добавляет новый источник к определению артефакта.
func (a *ArtifactDefinition) AppendSource(typeIndicator string, attributes map[string]interface{}) (*Source, error) {
	if typeIndicator == "" {
		return nil, FormatError{msg: "Missing type indicator."}
	}
	source := &Source{
		TypeIndicator: typeIndicator,
		Attributes:    attributes,
	}
	a.Sources = append(a.Sources, source)
	return source, nil
}

// AsDict возвращает представление определения артефакта в виде словаря (map).
func (a *ArtifactDefinition) AsDict() map[string]interface{} {
	sources := make([]map[string]interface{}, 0, len(a.Sources))
	for _, source := range a.Sources {
		sourceDict := map[string]interface{}{
			"type":       source.TypeIndicator,
			"attributes": source.Attributes,
		}
		sources = append(sources, sourceDict)
	}

	artifactDict := map[string]interface{}{
		"name":    a.Name,
		"doc":     a.Description,
		"sources": sources,
	}
	if len(a.Aliases) > 0 {
		artifactDict["aliases"] = a.Aliases
	}
	if len(a.SupportedOS) > 0 {
		artifactDict["supported_os"] = a.SupportedOS
	}
	if len(a.URLs) > 0 {
		artifactDict["urls"] = a.URLs
	}
	return artifactDict
}
