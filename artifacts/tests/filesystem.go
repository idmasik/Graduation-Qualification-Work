package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Константы и глобальные переменные
const CHUNK_SIZE = 5 * 1024 * 1024
const FILE_INFO_TYPE = "FILE_INFO"

var TSK_FILESYSTEMS = []string{"NTFS", "ext3", "ext4"}

var (
	// Компилируем регулярное выражение для поиска рекурсии в пути.
	// Шаблон: "**" с опциональным числом (либо "-1", либо число) после звёздочек.
	pathRecursionRegex = regexp.MustCompile(`\*\*((-1|\d*))`)
	// Шаблон для поиска символов подстановки: *, ? или группы в квадратных скобках.
	pathGlobRegex = regexp.MustCompile(`\*|\?|\[.+\]`)
)

// -------------------------------
// Вспомогательная структура для хранения паттернов

type patternEntry struct {
	artifact   string
	pattern    string
	sourceType string
}

// GeneratorFunc определяет функцию-генератор, которая принимает исходный канал объектов пути и возвращает новый канал.
type GeneratorFunc func(source <-chan *PathObject) <-chan *PathObject

// -------------------------------
// ArtifactFileSystem – базовая структура для работы с паттернами,
// реализующая функциональность, аналогичную исходному классу FileSystem на Python.
// Важно: данное имя отличается от интерфейса FileSystem из path_components.go.
type ArtifactFileSystem struct {
	patterns []patternEntry
}

// NewArtifactFileSystem создаёт новый экземпляр ArtifactFileSystem.
func NewArtifactFileSystem() *ArtifactFileSystem {
	return &ArtifactFileSystem{
		patterns: make([]patternEntry, 0),
	}
}

// AddPattern добавляет новый паттерн для указанного артефакта.
// Если sourceType не указан, по умолчанию используется "FILE".
func (fs *ArtifactFileSystem) AddPattern(artifact, pattern, sourceType string) {
	if sourceType == "" {
		sourceType = "FILE"
	}
	fs.patterns = append(fs.patterns, patternEntry{
		artifact:   artifact,
		pattern:    pattern,
		sourceType: sourceType,
	})
}

// relativePath нормализует путь относительно точки монтирования.
// Метод абстрактный – должен быть реализован в конкретном типе файловой системы.
func (fs *ArtifactFileSystem) relativePath(filepath string) string {
	panic("Not implemented: relativePath")
}

// parse разбирает строковый паттерн и возвращает последовательность генераторов для компонентов пути.
func (fs *ArtifactFileSystem) parse(pattern string) []GeneratorFunc {
	var generators []GeneratorFunc
	items := strings.Split(pattern, "/")
	for i, item := range items {
		// Если не последний элемент – ожидаем директорию
		isDir := i < len(items)-1

		// Проверяем, соответствует ли компонент рекурсивному шаблону "**"
		if matches := pathRecursionRegex.FindStringSubmatch(item); len(matches) > 0 {
			capturedMaxDepth := matches[1]
			var maxDepth int
			if capturedMaxDepth != "" {
				var err error
				maxDepth, err = strconv.Atoi(capturedMaxDepth)
				if err != nil {
					maxDepth = -1
				}
			} else {
				maxDepth = -1
			}
			// Создаём копии переменных для корректного замыкания
			dir := isDir
			md := maxDepth
			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
				// Используем конструктор из path_components.go
				return NewRecursionPathComponent(dir, md, source).Generate()
			})
		} else if pathGlobRegex.MatchString(item) {
			dir := isDir
			capturedItem := item
			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
				return NewGlobPathComponent(dir, capturedItem, source).Generate()
			})
		} else {
			dir := isDir
			capturedItem := item
			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
				return NewRegularPathComponent(dir, capturedItem, source).Generate()
			})
		}
	}
	return generators
}

// baseGenerator возвращает исходный канал для генерации путей.
// Метод абстрактный – должен быть реализован в конкретном типе файловой системы.
func (fs *ArtifactFileSystem) baseGenerator() <-chan *PathObject {
	panic("Not implemented: baseGenerator")
}

// CollectorOutput определяет интерфейс для вывода собранных артефактов.
type CollectorOutput interface {
	AddCollectedFileInfo(artifact string, path *PathObject) error
	AddCollectedFile(artifact string, path *PathObject) error
}

// Collect проходит по всем зарегистрированным паттернам, строит цепочку генераторов и передаёт найденные объекты в output.
func (fs *ArtifactFileSystem) Collect(output CollectorOutput) {
	for _, pat := range fs.patterns {
		logger.Log(LevelDebug, fmt.Sprintf("Collecting pattern '%s' for artifact '%s'", pat.pattern, pat.artifact))

		// Нормализуем паттерн относительно точки монтирования
		relativePattern := fs.relativePath(pat.pattern)
		genFuncs := fs.parse(relativePattern)

		// Получаем базовый генератор
		gen := fs.baseGenerator()
		// Последовательно оборачиваем базовый генератор генераторами для каждого компонента пути
		for _, gf := range genFuncs {
			gen = gf(gen)
		}
		// Обрабатываем найденные объекты пути
		for pathObj := range gen {
			if pat.sourceType == FILE_INFO_TYPE {
				if err := output.AddCollectedFileInfo(pat.artifact, pathObj); err != nil {
					logger.Log(LevelError, fmt.Sprintf("Error collecting file '%s': %s", pathObj.path, err))
				}
			} else {
				if err := output.AddCollectedFile(pat.artifact, pathObj); err != nil {
					logger.Log(LevelError, fmt.Sprintf("Error collecting file '%s': %s", pathObj.path, err))
				}
			}
		}
	}
}
