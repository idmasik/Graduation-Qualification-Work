//go:build !windows
// +build !windows

package main

// Заглушка для windowsInitFunc для Unix: принимает *HostVariables и возвращает nil.
func windowsInitFunc(*HostVariables) {
	// Ничего не делаем.
}

// Заглушка для NewRegistryCollector, возвращающая dummyCollector.
func NewRegistryCollector() AbstractCollector {
	return dummyCollector{}
}

// Заглушка для NewWMIExecutor, возвращающая dummyCollector.
func NewWMIExecutor() AbstractCollector {
	return dummyCollector{}
}

type dummyCollector struct{}

func (d dummyCollector) Collect(output *Outputs) {}
func (d dummyCollector) RegisterSource(artifactDefinition *ArtifactDefinition, artifactSource *Source, variables *HostVariables) bool {
	return false
}

// readVersionInfo-заглушка для non-Windows
func readVersionInfo(path string) (map[string]string, error) {
	return map[string]string{}, nil
}
