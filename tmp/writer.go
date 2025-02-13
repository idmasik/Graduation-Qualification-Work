package main

import (
	"encoding/json"
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

// BaseArtifactsWriter is the interface for artifact writers.
type BaseArtifactsWriter interface {
	FormatArtifacts(artifacts []ArtifactDefinition) string
	WriteArtifactsFile(artifacts []ArtifactDefinition, filename string)
}

// ArtifactWriter provides a base implementation for writing artifacts.
type ArtifactWriter struct{}

// WriteArtifactsFile writes artifacts to a file using the formatted data from FormatArtifacts.
// This method should be overridden in embedded structs if needed, but in Go, we need to use composition.
func (aw *ArtifactWriter) WriteArtifactsFile(artifacts []ArtifactDefinition, filename string) {
	// This method cannot directly call FormatArtifacts as in Python.
	// Actual implementation will be in the specific writers.
}

// JsonArtifactsWriter writes artifacts in JSON format.
type JsonArtifactsWriter struct {
	ArtifactWriter
}

// FormatArtifacts formats artifacts into a JSON string.
func (j *JsonArtifactsWriter) FormatArtifacts(artifacts []ArtifactDefinition) string {
	definitions := make([]map[string]interface{}, len(artifacts))
	for i, artifact := range artifacts {
		definitions[i] = artifact.AsDict()
	}
	jsonData, _ := json.Marshal(definitions)
	return string(jsonData)
}

// WriteArtifactsFile writes the JSON formatted artifacts to a file.
func (j *JsonArtifactsWriter) WriteArtifactsFile(artifacts []ArtifactDefinition, filename string) {
	data := j.FormatArtifacts(artifacts)
	ioutil.WriteFile(filename, []byte(data), 0644)
}

// YamlArtifactsWriter writes artifacts in YAML format.
type YamlArtifactsWriter struct {
	ArtifactWriter
}

// FormatArtifacts formats artifacts into a YAML string.
func (y *YamlArtifactsWriter) FormatArtifacts(artifacts []ArtifactDefinition) string {
	definitions := make([]map[string]interface{}, len(artifacts))
	for i, artifact := range artifacts {
		definitions[i] = artifact.AsDict()
	}
	yamlData, _ := yaml.Marshal(definitions)
	return string(yamlData)
}

// WriteArtifactsFile writes the YAML formatted artifacts to a file.
func (y *YamlArtifactsWriter) WriteArtifactsFile(artifacts []ArtifactDefinition, filename string) {
	data := y.FormatArtifacts(artifacts)
	ioutil.WriteFile(filename, []byte(data), 0644)
}
