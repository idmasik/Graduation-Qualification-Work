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
	Library   bool
	MaxSize   string
	Output    string
	SHA256    bool
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
		Library:   *flags.library,
		MaxSize:   *flags.maxsize,
		Output:    *flags.output,
		SHA256:    *flags.sha256,
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
	library   *bool
	maxsize   *string
	output    *string
	sha256    *bool
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

	flags.library = flag.Bool("library",
		section.Key("library").MustBool(false),
		"Загружать определения из стандартной библиотеки артефактов (в дополнение к custom-директориям)")

	flags.maxsize = flag.String("maxsize",
		section.Key("maxsize").MustString(""),
		"Не собирать файлы размером > n")

	flags.output = flag.String("output",
		section.Key("output").MustString("."),
		"Директория для создания результатов")

	flags.sha256 = flag.Bool("sha256",
		section.Key("sha256").MustBool(false),
		"Вычислять SHA-256 для собранных файлов")

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
func getArtifactsRegistry(useLibrary bool, paths []string) *ArtifactDefinitionsRegistry {
	reader := NewYamlArtifactsReader()
	registry := NewArtifactDefinitionsRegistry()

	// Если не указаны пользовательские пути или требуется использовать библиотеку,
	// читаем артефакты из стандартной директории (например, "<prefix>/share/artifacts").
	if len(paths) == 0 || useLibrary {
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
	"WMILoginUsers":         true,
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
	// // Включаем привилегию SeBackupPrivilege.
	// if err := enableBackupPrivilege(); err != nil {
	// 	logger.Log(LevelWarning, fmt.Sprintf("Не удалось включить привилегию резервного копирования: %v", err))
	// 	// Обратите внимание: для корректного доступа к защищённым файлам может потребоваться запуск от SYSTEM.
	// }

	config := parseArgs()
	fmt.Printf("Config: %#v\n", config)

	platform, err := getOperatingSystem()
	if err != nil {
		logger.Log(LevelCritical, err.Error())
		os.Exit(1)
	}

	logger.Log(LevelProgress, "Загрузка артефактов ...")

	output, err := NewOutputs(config.Output, config.MaxSize, config.SHA256)
	if err != nil {
		logger.Log(LevelCritical, fmt.Sprintf("Не удалось инициализировать вывод: %v", err))
		os.Exit(1)
	}

	// Создаём коллектор. В конструктор передаётся платформа.
	collector := NewCollector(platform, nil)

	// Загружаем определения артефактов
	registry := getArtifactsRegistry(config.Library, config.Directory)

	// Разворачиваем группы (если заданы)
	includeArtifacts := resolveArtifactGroups(registry, config.Include)
	excludeArtifacts := resolveArtifactGroups(registry, config.Exclude)

	// Флаг, управляющий сбором реестровых источников (аналогично Python‑логике)
	collectRegistry := false
	if config.Include != "" || (len(config.Directory) > 0 && !config.Library) {
		collectRegistry = true
	}

	// Фильтруем артефакты и регистрируем источники в коллекторе.
	for _, pair := range getArtifactsToCollect(registry, includeArtifacts, excludeArtifacts, platform, collectRegistry) {
		collector.RegisterSource(pair.definition, pair.source)
	}

	// Запускаем сбор артефактов и закрываем вывод.
	collector.Collect(output)
}
