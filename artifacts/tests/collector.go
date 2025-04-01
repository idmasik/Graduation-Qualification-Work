// collector.go
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
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

func isSourceSupportedOnCurrentOS(artifactDefinition *ArtifactDefinition) bool {
	// Если SupportedOS не задан, считаем, что источник поддерживается на всех ОС
	if len(artifactDefinition.SupportedOS) == 0 {
		return true
	}
	currentOS := runtime.GOOS
	for _, osName := range artifactDefinition.SupportedOS {
		if strings.EqualFold(osName, currentOS) {
			return true
		}
	}
	return false
}

func (c *Collector) RegisterSource(artifactDefinition *ArtifactDefinition, artifactSource *Source) {
	// Если источник не поддерживается на текущей ОС, пропускаем регистрацию
	if !isSourceSupportedOnCurrentOS(artifactDefinition) {
		logger.Log(LevelWarning,
			fmt.Sprintf("Skipping source for '%s' as it is not supported on %s", artifactDefinition.Name, runtime.GOOS))
		return
	}

	supported := false
	for _, collector := range c.collectors {
		if collector.RegisterSource(artifactDefinition, artifactSource, c.variables) {
			supported = true
		}
	}
	if supported {
		c.sources++
	} else if artifactSource.TypeIndicator != TYPE_INDICATOR_ARTIFACT_GROUP {
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
