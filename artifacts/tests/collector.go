// collector.go
package main

import (
	"fmt"
	"os"
)

// AbstractCollector задаёт интерфейс для сборщиков артефактов.
type AbstractCollector interface {
	// Collect выполняет сбор данных и записывает результат в output.
	Collect(output *Outputs)
	// RegisterSource пытается зарегистрировать источник для данного определения артефакта.
	// Если источник поддерживается, возвращается true.
	RegisterSource(artifactDefinition *ArtifactDefinition, artifactSource *Source, variables *HostVariables) bool
}

// Collector управляет списком сборщиков (collectors) и переменными, специфичными для хоста.
type Collector struct {
	platform   string
	variables  *HostVariables
	sources    int
	collectors []AbstractCollector
}

// NewCollector создаёт новый Collector для указанной платформы.
// Параметр collectors должен содержать список объектов, реализующих интерфейс AbstractCollector.

// NewCollector создаёт новый Collector для указанной платформы,
// при этом переменные хоста инициализируются независимо от платформы.
// NewCollector создаёт новый Collector для указанной платформы с инициализированными переменными.
func NewCollector(platform string, collectors []AbstractCollector) *Collector {
	hv := NewHostVariables(windowsInitFunc)

	// Initialize required collectors
	fsManager, err := NewFileSystemManager(hv)
	if err != nil {
		logger.Log(LevelCritical, fmt.Sprintf("Failed to create FS manager: %v", err))
		os.Exit(1)
	}

	defaultCollectors := []AbstractCollector{
		NewCommandExecutor(),
		fsManager,
		NewRegistryCollector(),
		NewWMIExecutor(),
	}

	return &Collector{
		platform:   platform,
		variables:  hv,
		collectors: defaultCollectors,
		sources:    0,
	}
}

// RegisterSource пытается зарегистрировать источник артефакта у каждого из внутренних сборщиков.
func (c *Collector) RegisterSource(artifactDefinition *ArtifactDefinition, artifactSource *Source) {
	supported := false

	for _, collector := range c.collectors {
		if collector.RegisterSource(artifactDefinition, artifactSource, c.variables) {
			supported = true
		}
	}

	if supported {
		c.sources++
	} else if artifactSource.TypeIndicator != TYPE_INDICATOR_ARTIFACT_GROUP {
		// Выводим предупреждение, если тип источника не поддерживается
		logger.Log(LevelWarning,
			fmt.Sprintf("Cannot process source for '%s' because type '%s' is not supported",
				artifactDefinition.Name, artifactSource.TypeIndicator))
	}
}

// Collect выполняет сбор артефактов со всех источников и закрывает output.
func (c *Collector) Collect(output *Outputs) {
	logger.Log(LevelProgress,
		fmt.Sprintf("Collecting artifacts from %d sources ...", c.sources))

	for _, collector := range c.collectors {
		collector.Collect(output)
	}

	logger.Log(LevelProgress, "Finished collecting artifacts")
	output.Close()
}
