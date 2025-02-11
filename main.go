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

func main() {
	// Example usage of errors
	fmt.Println(CodeStyleError{"Code style issue"})
	fmt.Println(FormatError{"Invalid format"})
	fmt.Println(MissingDependencyError{"Missing dependency"})

	// Example usage
	factory := NewSourceTypeFactory()

	// Register ArtifactGroupSourceType
	factory.RegisterSourceType(TYPE_INDICATOR_ARTIFACT_GROUP, func(attributes map[string]interface{}) (SourceType, error) {
		names, ok := attributes["names"].([]string)
		if !ok {
			return nil, &FormatError{"invalid or missing 'names' attribute"}
		}
		return NewArtifactGroupSourceType(names)
	})

	// Create an ArtifactGroupSourceType
	source, err := factory.CreateSourceType(TYPE_INDICATOR_ARTIFACT_GROUP, map[string]interface{}{
		"names": []string{"example_artifact"},
	})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Source Type:", source.AsDict())

	artifact := NewArtifactDefinition("ExampleArtifact", []string{"ex_art"}, "Пример определения артефакта")
	attrs := map[string]interface{}{"path": "/var/log/example.log"}
	if _, err := artifact.AppendSource("FILE", attrs); err != nil {
		fmt.Println("Ошибка добавления источника:", err)
	}
	fmt.Printf("Artifact as dict: %#v\n", artifact.AsDict())

	// Пример чтения JSON‑артефактов.
	jsonFile := "artifacts.json"
	jReader := NewJsonArtifactsReader()
	artifacts, err := jReader.ReadFile(jsonFile)
	if err != nil {
		fmt.Printf("Ошибка чтения JSON: %s\n", err.Error())
	} else {
		fmt.Printf("Прочитано %d артефактов из JSON.\n", len(artifacts))
	}

	// Пример чтения YAML‑артефактов.
	yamlFile := "D:\\Projects\\GolangProjects\\Graduation Qualifying Work\\data\\antivirus.yaml"
	yReader := NewYamlArtifactsReader()
	artifacts, err = yReader.ReadFile(yamlFile)
	if err != nil {
		fmt.Printf("Ошибка чтения YAML: %s\n", err.Error())
	} else {
		fmt.Printf("Прочитано %d артефактов из YAML.\n", len(artifacts))
	}

	//////////////////////////////////////////////////////////////////////////////////////
	config := parseArgs()
	fmt.Printf("Config: %#v\n", config)

	// Определение ОС
	platform, err := getOperatingSystem()
	if err != nil {
		logger.Log(LevelCritical, err.Error())
		os.Exit(1)
	}

	fmt.Print(platform)

	// Инициализация коллектора
	//collector := NewCollector(platform)

}

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
		log.Fatalf("Error getting working directory: %v", err)
	}
	configPath := filepath.Join(workDir, "artifacts.ini")

	log.Printf("Looking for config at: %s", configPath)
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Printf("Config error: %v", err)
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
			log.Printf("Config file not found at %s", path)
			return ini.Empty(), nil
		}
		return nil, fmt.Errorf("config load error: %w", err)
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
		"Artifacts to collect (comma-separated)")

	flags.exclude = flag.String("exclude",
		section.Key("exclude").MustString(""),
		"Artifacts to ignore (comma-separated)")

	flags.directory = flag.String("directory",
		section.Key("directory").MustString(""),
		"Directories containing artifacts definitions (comma-separated)")

	flags.library = flag.Bool("library",
		section.Key("library").MustBool(false),
		"Keep loading Artifacts definitions from the ForensicArtifacts library (in addition to customdirectories)")

	flags.maxsize = flag.String("maxsize",
		section.Key("maxsize").MustString(""),
		"Do not collect file with size > n")

	flags.output = flag.String("output",
		section.Key("output").MustString("."),
		"Directory where the results are created")

	flags.sha256 = flag.Bool("sha256",
		section.Key("sha256").MustBool(false),
		"Calculate SHA-256 hashes")

	return flags
}

func splitArgs(input string) []string {
	if input == "" {
		return nil
	}
	return strings.Split(input, ",")
}
