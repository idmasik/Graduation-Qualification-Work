// collector.go
package main

import (
	"fmt"
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
func NewCollector(platform string, collectors []AbstractCollector) *Collector {
	var hv *HostVariables
	if platform != "Windows" {
		// Для Unix-подобных систем инициализируем переменные хоста
		hv = NewUnixHostVariables()
	}

	// Создаём обязательные коллекторы
	var defaultCollectors []AbstractCollector

	// Инициализация FileSystemManager
	fsManager, err := NewFileSystemManager()
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("Ошибка создания FileSystemManager: %v", err))
	} else {
		defaultCollectors = append(defaultCollectors, fsManager)
	}

	// Инициализация CommandExecutor
	cmdExecutor := NewCommandExecutor()
	defaultCollectors = append(defaultCollectors, cmdExecutor)

	// Объединяем переданные коллекторы с обязательными (если требуется)
	// В данном случае игнорируем переданные collectors, как в Python примере
	allCollectors := defaultCollectors

	return &Collector{
		platform:   platform,
		variables:  hv,
		collectors: allCollectors,
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
