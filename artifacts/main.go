package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// ─── Config и разбор аргументов ───────────────────────────────────────────────

type Config struct {
	Include   string
	Exclude   string
	Directory []string
	Registry  bool
	MaxSize   string
	Output    string
	ApiKey    string
	SHA256    bool
	Analysis  bool
}

func parseArgs() *Config {
	// Получаем текущую рабочую директорию
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Ошибка получения рабочей директории: %v", err)
	}
	configPath := filepath.Join(workDir, "artifacts.ini")
	log.Printf("Поиск конфигурации по пути: %s", configPath)
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Printf("Ошибка конфигурации: %v", err)
	}
	flags := initFlags(cfg)
	flag.Parse()
	return &Config{
		Include:   *flags.include,
		Exclude:   *flags.exclude,
		Directory: splitArgs(*flags.directory),
		Registry:  *flags.registry,
		MaxSize:   *flags.maxsize,
		Output:    *flags.output,
		ApiKey:    *flags.apikey,
		SHA256:    *flags.sha256,
		Analysis:  *flags.analysis,
	}
}

func loadConfig(path string) (*ini.File, error) {
	cfg, err := ini.LoadSources(ini.LoadOptions{
		Insensitive:         true,
		AllowBooleanKeys:    true,
		UnparseableSections: []string{},
	}, path)

	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Файл конфигурации не найден по пути %s", path)
			return ini.Empty(), nil
		}
		return nil, fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}
	return cfg, nil
}

type appFlags struct {
	include   *string
	exclude   *string
	directory *string
	registry  *bool
	maxsize   *string
	apikey    *string
	output    *string
	sha256    *bool
	analysis  *bool
}

func initFlags(cfg *ini.File) *appFlags {
	flags := &appFlags{}
	section := cfg.Section("")

	flags.include = flag.String("include",
		section.Key("include").MustString(""),
		"Артефакты для сбора (через запятую)")

	flags.exclude = flag.String("exclude",
		section.Key("exclude").MustString(""),
		"Артефакты для игнорирования (через запятую)")

	flags.directory = flag.String("directory",
		section.Key("directory").MustString(""),
		"Директории с определениями артефактов (через запятую)")

	flags.registry = flag.Bool("registry",
		section.Key("registry").MustBool(false),
		"Флаг, управляющий сбором реестровых источников (на Windows) ")

	flags.maxsize = flag.String("maxsize",
		section.Key("maxsize").MustString(""),
		"Не собирать файлы размером > n")

	flags.apikey = flag.String("apikey",
		section.Key("apikey").MustString("."),
		"ApiKey платфомы opentip")

	flags.output = flag.String("output",
		section.Key("output").MustString("."),
		"Директория для создания результатов")

	flags.sha256 = flag.Bool("sha256",
		section.Key("sha256").MustBool(false),
		"Вычислять SHA-256 для собранных файлов")

	flags.analysis = flag.Bool("analysis",
		section.Key("analysis").MustBool(false),
		"Флаг, управляющий первичным аналзим артефактов")

	return flags
}

func splitArgs(input string) []string {
	if input == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	for i, s := range parts {
		parts[i] = strings.TrimSpace(s)
	}
	return parts
}

// ReadFromDirectory читает определения артефактов из указанной директории с помощью переданного reader.
func (r *ArtifactDefinitionsRegistry) ReadFromDirectory(reader ArtifactsReaderInterface, path string) error {
	defs, err := reader.ReadDirectory(path, "yaml")
	if err != nil {
		return err
	}
	for _, def := range defs {
		// Приводим имя к нижнему регистру для единообразия
		r.artifactDefinitionsByName[strings.ToLower(def.Name)] = def
	}
	return nil
}

// ─── Реализация функций для загрузки артефактов ───────────────────────────────

// getArtifactsRegistry создаёт реестр, загружая определения как из стандартной библиотеки,
// так и из указанных пользователем директорий.
func getArtifactsRegistry(paths []string) *ArtifactDefinitionsRegistry {
	reader := NewYamlArtifactsReader()
	registry := NewArtifactDefinitionsRegistry()

	// Если не указаны пользовательские пути,
	// читаем артефакты из стандартной директории (например, "<prefix>/share/artifacts").
	if len(paths) == 0 {
		exePath, err := os.Executable()
		if err != nil {
			logger.Log(LevelError, fmt.Sprintf("Ошибка получения пути исполняемого файла: %v", err))
		}
		sharePath := filepath.Join(filepath.Dir(exePath), "data")
		if err := registry.ReadFromDirectory(reader, sharePath); err != nil {
			logger.Log(LevelError, fmt.Sprintf("Ошибка чтения артефактов из %s: %v", sharePath, err))
		}
	}

	// Читаем определения из пользовательских директорий.
	if len(paths) > 0 {
		for _, p := range paths {
			if err := registry.ReadFromDirectory(reader, p); err != nil {
				logger.Log(LevelError, fmt.Sprintf("Ошибка чтения артефактов из %s: %v", p, err))
			}
		}
	}
	return registry
}

// resolveArtifactGroups разворачивает группы артефактов и возвращает множество имён.
func resolveArtifactGroups(registry *ArtifactDefinitionsRegistry, artifactNames string) map[string]bool {
	resolved := make(map[string]bool)
	if artifactNames == "" {
		return resolved
	}
	names := strings.Split(artifactNames, ",")
	for _, name := range names {
		name = strings.TrimSpace(name)
		def := registry.GetDefinitionByName(name)
		if def != nil {
			resolved[name] = true
			for _, source := range def.Sources {
				if source.TypeIndicator == TYPE_INDICATOR_ARTIFACT_GROUP {
					if namesAttr, ok := source.Attributes["names"].([]string); ok {
						for _, subName := range namesAttr {
							resolved[subName] = true
						}
					}
				}
			}
		}
	}
	return resolved
}

// ArtifactSourcePair связывает определение артефакта и один из его источников.
type ArtifactSourcePair struct {
	definition *ArtifactDefinition
	source     *Source
}

// getArtifactsToCollect фильтрует определения, учитывая списки include/exclude, поддерживаемую ОС и тип источника.
func getArtifactsToCollect(registry *ArtifactDefinitionsRegistry, include map[string]bool, exclude map[string]bool, platform string, collectRegistry bool) []ArtifactSourcePair {
	var result []ArtifactSourcePair
	for _, def := range registry.GetDefinitions() {
		// Пропускаем артефакты из чёрного списка, если они не указаны явно.
		if BLACKLIST[def.Name] && !include[def.Name] {
			continue
		}
		// Если задан список include, то пропускаем неуказанные.
		if len(include) > 0 && !include[def.Name] {
			continue
		}
		// Пропускаем, если определение попало в exclude.
		if exclude[def.Name] {
			continue
		}
		// Проверяем поддерживаемые ОС для определения артефакта.
		if len(def.SupportedOS) > 0 && !contains(def.SupportedOS, platform) {
			continue
		}
		for _, source := range def.Sources {
			// Если в атрибутах источника задан список поддерживаемых ОС, проверяем его.
			if v, ok := source.Attributes["supported_os"]; ok {
				if supportedOS, ok := v.([]string); ok && len(supportedOS) > 0 && !contains(supportedOS, platform) {
					continue
				}
			}
			// Если не требуется собирать реестр и источник относится к нему, пропускаем.
			if !collectRegistry && (source.TypeIndicator == TYPE_INDICATOR_WINDOWS_REGISTRY_KEY || source.TypeIndicator == TYPE_INDICATOR_WINDOWS_REGISTRY_VALUE) {
				continue
			}
			result = append(result, ArtifactSourcePair{definition: def, source: source})
		}
	}
	return result
}

// ─── Константы и переменные ─────────────────────────────────────────────────────

var BLACKLIST = map[string]bool{
	"WMILoginUsers":         false,
	"WMIUsers":              true,
	"WMIVolumeShadowCopies": true,
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ─── Основная функция ─────────────────────────────────────────────────────────

func main() {
	config := parseArgs()

	platform, err := getOperatingSystem()
	if err != nil {
		logger.Log(LevelCritical, err.Error())
		os.Exit(1)
	}

	output, err := NewOutputs(config.Output, config.MaxSize, config.SHA256, config.Analysis, config.ApiKey)
	if err != nil {
		logger.Log(LevelCritical, fmt.Sprintf("Не удалось инициализировать вывод: %v", err))
		os.Exit(1)
	}

	logger.Log(LevelInfo, fmt.Sprintf("Config: %#v\n", config))

	// Создаём коллектор. В конструктор передаётся платформа.
	collector := NewCollector(platform, nil)

	// Загружаем определения артефактов
	logger.Log(LevelProgress, "Загрузка артефактов ...")
	registry := getArtifactsRegistry(config.Directory)

	// Разворачиваем группы (если заданы)
	includeArtifacts := resolveArtifactGroups(registry, config.Include)
	excludeArtifacts := resolveArtifactGroups(registry, config.Exclude)

	// Флаг, управляющий сбором реестровых источников
	if (platform == "Windows") && (config.Registry) {
		logger.Log(LevelInfo, "Сбор реестровых источников активирован")
	} else {
		logger.Log(LevelInfo, "Сбор реестровых источников отключен: флаг Registry не задан")
	}

	// Фильтруем артефакты и регистрируем источники в коллекторе.
	for _, pair := range getArtifactsToCollect(registry, includeArtifacts, excludeArtifacts, platform, config.Registry) {
		collector.RegisterSource(pair.definition, pair.source)
	}

	// Запускаем сбор артефактов и закрываем вывод.
	logger.Log(LevelProgress, fmt.Sprintf("Collecting artifacts from %d sources ...", collector.sources))
	collector.Collect(output)
}
