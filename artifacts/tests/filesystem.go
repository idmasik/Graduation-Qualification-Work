package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// // FileSystem interface
// type FileSystem interface {
//     AddPattern(artifact, pattern, sourceType string)
//     Collect(output CollectorOutput)
//     relativePath(filepath string) string
//     parse(pattern string) []GeneratorFunc
//     baseGenerator() <-chan *PathObject
//     IsDirectory(p *PathObject) bool
//     IsFile(p *PathObject) bool
//     IsSymlink(p *PathObject) bool
//     ListDirectory(p *PathObject) []*PathObject
//     GetPath(parent *PathObject, name string) *PathObject
//     GetFullPath(fullpath string) *PathObject
//     ReadChunks(p *PathObject) ([]byte, error)
//     GetSize(p *PathObject) int64
// }
// // Константы и глобальные переменные
// const CHUNK_SIZE = 5 * 1024 * 1024
// const FILE_INFO_TYPE = "FILE_INFO"

// var TSK_FILESYSTEMS = []string{"NTFS", "ext3", "ext4"}

// var (
// 	// Компилируем регулярное выражение для поиска рекурсии в пути.
// 	// Шаблон: "**" с опциональным числом (либо "-1", либо число) после звёздочек.
// 	pathRecursionRegex = regexp.MustCompile(`\*\*((-1|\d*))`)
// 	pathGlobRegex      = regexp.MustCompile(`\*|\?|\[.+\]`)
// )

// // -------------------------------
// // Вспомогательная структура для хранения паттернов

// type patternEntry struct {
// 	artifact   string
// 	pattern    string
// 	sourceType string
// }

// // GeneratorFunc определяет функцию-генератор, которая принимает исходный канал объектов пути и возвращает новый канал.
// type GeneratorFunc func(source <-chan *PathObject) <-chan *PathObject

// // -------------------------------
// // ArtifactFileSystem – базовая структура для работы с паттернами,
// // реализующая функциональность, аналогичную исходному классу FileSystem на Python.
// // Важно: данное имя отличается от интерфейса FileSystem из path_components.go.
// type ArtifactFileSystem struct {
// 	patterns []patternEntry
// }

// // NewArtifactFileSystem создаёт новый экземпляр ArtifactFileSystem.
// func NewArtifactFileSystem() *ArtifactFileSystem {
// 	return &ArtifactFileSystem{
// 		patterns: make([]patternEntry, 0),
// 	}
// }

// // AddPattern добавляет новый паттерн для указанного артефакта.
// // Если sourceType не указан, по умолчанию используется "FILE".
// func (fs *ArtifactFileSystem) AddPattern(artifact, pattern, sourceType string) {
// 	if sourceType == "" {
// 		sourceType = "FILE"
// 	}
// 	fs.patterns = append(fs.patterns, patternEntry{
// 		artifact:   artifact,
// 		pattern:    pattern,
// 		sourceType: sourceType,
// 	})
// }

// // relativePath нормализует путь относительно точки монтирования.
// // Метод абстрактный – должен быть реализован в конкретном типе файловой системы.
// func (fs *ArtifactFileSystem) relativePath(filepath string) string {
// 	panic("Not implemented: relativePath")
// }

// // parse разбирает строковый паттерн и возвращает последовательность генераторов для компонентов пути.
// func (fs *ArtifactFileSystem) parse(pattern string) []GeneratorFunc {
// 	var generators []GeneratorFunc
// 	items := strings.Split(pattern, "/")
// 	for i, item := range items {
// 		// Если не последний элемент – ожидаем директорию
// 		isDir := i < len(items)-1

// 		// Проверяем, соответствует ли компонент рекурсивному шаблону "**"
// 		if matches := pathRecursionRegex.FindStringSubmatch(item); len(matches) > 0 {
// 			capturedMaxDepth := matches[1]
// 			var maxDepth int
// 			if capturedMaxDepth != "" {
// 				var err error
// 				maxDepth, err = strconv.Atoi(capturedMaxDepth)
// 				if err != nil {
// 					maxDepth = -1
// 				}
// 			} else {
// 				maxDepth = -1
// 			}
// 			// Создаём копии переменных для корректного замыкания
// 			dir := isDir
// 			md := maxDepth
// 			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
// 				// Используем конструктор из path_components.go
// 				return NewRecursionPathComponent(dir, md, source).Generate()
// 			})
// 		} else if pathGlobRegex.MatchString(item) {
// 			dir := isDir
// 			capturedItem := item
// 			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
// 				return NewGlobPathComponent(dir, capturedItem, source).Generate()
// 			})
// 		} else {
// 			dir := isDir
// 			capturedItem := item
// 			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
// 				return NewRegularPathComponent(dir, capturedItem, source).Generate()
// 			})
// 		}
// 	}
// 	return generators
// }

// // baseGenerator возвращает исходный канал для генерации путей.
// // Метод абстрактный – должен быть реализован в конкретном типе файловой системы.
// func (fs *ArtifactFileSystem) baseGenerator() <-chan *PathObject {
// 	panic("Not implemented: baseGenerator")
// }

// // CollectorOutput определяет интерфейс для вывода собранных артефактов.
// type CollectorOutput interface {
// 	AddCollectedFileInfo(artifact string, path *PathObject) error
// 	AddCollectedFile(artifact string, path *PathObject) error
// }

// // Collect проходит по всем зарегистрированным паттернам, строит цепочку генераторов и передаёт найденные объекты в output.
// func (fs *ArtifactFileSystem) Collect(output CollectorOutput) {
// 	for _, pat := range fs.patterns {
// 		logger.Log(LevelDebug, fmt.Sprintf("Collecting pattern '%s' for artifact '%s'", pat.pattern, pat.artifact))

// 		// Нормализуем паттерн относительно точки монтирования
// 		relativePattern := fs.relativePath(pat.pattern)
// 		genFuncs := fs.parse(relativePattern)

// 		// Получаем базовый генератор
// 		gen := fs.baseGenerator()
// 		// Последовательно оборачиваем базовый генератор генераторами для каждого компонента пути
// 		for _, gf := range genFuncs {
// 			gen = gf(gen)
// 		}
// 		// Обрабатываем найденные объекты пути
// 		for pathObj := range gen {
// 			if pat.sourceType == FILE_INFO_TYPE {
// 				if err := output.AddCollectedFileInfo(pat.artifact, pathObj); err != nil {
// 					logger.Log(LevelError, fmt.Sprintf("Error collecting file '%s': %s", pathObj.path, err))
// 				}
// 			} else {
// 				if err := output.AddCollectedFile(pat.artifact, pathObj); err != nil {
// 					logger.Log(LevelError, fmt.Sprintf("Error collecting file '%s': %s", pathObj.path, err))
// 				}
// 			}
// 		}
// 	}
// }

// // OSFileSystem реализует интерфейс FileSystem из path_components.go
// type OSFileSystem struct {
// 	rootPath string
// }

// // NewOSFileSystem создаёт новый экземпляр OSFileSystem
// func NewOSFileSystem(path string) *OSFileSystem {
// 	return &OSFileSystem{rootPath: filepath.Clean(path)}
// }

// // _relativePath нормализует путь, заменяя разделители на '/' и возвращая путь относительно rootPath.
// func (fs *OSFileSystem) _relativePath(fpath string) string {
// 	normalizedPath := filepath.ToSlash(fpath)
// 	normalizedRoot := filepath.ToSlash(fs.rootPath)
// 	if strings.HasPrefix(normalizedPath, normalizedRoot) {
// 		relative := normalizedPath[len(normalizedRoot):]
// 		return strings.TrimLeft(relative, "/")
// 	}
// 	return normalizedPath
// }

// // _baseGenerator возвращает канал с единственным PathObject, соответствующим корневой директории.
// func (fs *OSFileSystem) _baseGenerator() <-chan *PathObject {
// 	out := make(chan *PathObject, 1)
// 	out <- &PathObject{
// 		filesystem: fs,
// 		name:       filepath.Base(fs.rootPath),
// 		path:       fs.rootPath,
// 	}
// 	close(out)
// 	return out
// }

// // IsDirectory возвращает true, если p.path указывает на директорию.
// func (fs *OSFileSystem) IsDirectory(p *PathObject) bool {
// 	info, err := os.Stat(p.path)
// 	return err == nil && info.IsDir()
// }

// // IsFile возвращает true, если p.path указывает на файл.
// func (fs *OSFileSystem) IsFile(p *PathObject) bool {
// 	info, err := os.Stat(p.path)
// 	return err == nil && !info.IsDir()
// }

// // IsSymlink возвращает false, так как os.Stat автоматически следует символическим ссылкам.
// func (fs *OSFileSystem) IsSymlink(p *PathObject) bool {
// 	// Используем os.Lstat для получения информации о символической ссылке.
// 	info, err := os.Lstat(p.path)
// 	return err == nil && (info.Mode()&os.ModeSymlink != 0)
// }

// // ListDirectory возвращает срез PathObject для каждого элемента в директории p.path.
// // В случае ошибки выводит сообщение в лог и возвращает пустой срез.
// func (fs *OSFileSystem) ListDirectory(p *PathObject) []*PathObject {
// 	var objects []*PathObject

// 	entries, err := os.ReadDir(p.path)
// 	if err != nil {
// 		logger.Log(LevelError, "Error analyzing directory '"+p.path+"': "+err.Error())
// 		return objects
// 	}

// 	for _, entry := range entries {
// 		objects = append(objects, &PathObject{
// 			filesystem: fs,
// 			name:       entry.Name(),
// 			path:       filepath.Join(p.path, entry.Name()),
// 		})
// 	}

// 	return objects
// }

// // GetPath возвращает новый PathObject для элемента с именем name внутри родительской директории parent.
// func (fs *OSFileSystem) GetPath(parent *PathObject, name string) *PathObject {
// 	return &PathObject{
// 		filesystem: fs,
// 		name:       name,
// 		path:       filepath.Join(parent.path, name),
// 	}
// }

// // GetFullPath возвращает новый PathObject для полного пути fullpath.
// func (fs *OSFileSystem) GetFullPath(fullpath string) *PathObject {
// 	return &PathObject{
// 		filesystem: fs,
// 		name:       filepath.Base(fullpath),
// 		path:       fullpath,
// 	}
// }

// // ReadChunks открывает файл p.path и возвращает один считанный чанк данных (до CHUNK_SIZE байт) и ошибку.
// func (fs *OSFileSystem) ReadChunks(p *PathObject) ([]byte, error) {
// 	// Если p не является файлом, возвращаем ошибку.
// 	if !fs.IsFile(p) {
// 		return nil, nil
// 	}

// 	f, err := os.Open(p.path)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer f.Close()

// 	reader := bufio.NewReader(f)
// 	buf := make([]byte, CHUNK_SIZE)
// 	n, err := reader.Read(buf)
// 	if err != nil && err != io.EOF {
// 		return nil, err
// 	}
// 	return buf[:n], nil
// }

// // GetSize возвращает размер файла p.path.
//
//	func (fs *OSFileSystem) GetSize(p *PathObject) int64 {
//		info, err := os.Lstat(p.path)
//		if err != nil {
//			return 0
//		}
//		return info.Size()
//	}
const (
	FILE_INFO_TYPE = "FILE_INFO"
	CHUNK_SIZE     = 5 * 1024 * 1024
)

var (
	pathRecursionRegex = regexp.MustCompile(`\*\*((-1|\d*))`)
	pathGlobRegex      = regexp.MustCompile(`\*|\?|\[.+\]`)
)

type FileSystem interface {
	AddPattern(artifact, pattern, sourceType string)
	Collect(output CollectorOutput)
	relativePath(filepath string) string
	parse(pattern string) []GeneratorFunc
	baseGenerator() <-chan *PathObject
	IsDirectory(p *PathObject) bool
	IsFile(p *PathObject) bool
	IsSymlink(p *PathObject) bool
	ListDirectory(p *PathObject) []*PathObject
	GetPath(parent *PathObject, name string) *PathObject
	GetFullPath(fullpath string) *PathObject
	ReadChunks(p *PathObject) ([]byte, error)
	GetSize(p *PathObject) int64
}

type patternEntry struct {
	artifact   string
	pattern    string
	sourceType string
}

type ArtifactFileSystem struct {
	patterns []patternEntry
	fs       FileSystem
}

func NewArtifactFileSystem(fs FileSystem) *ArtifactFileSystem {
	return &ArtifactFileSystem{
		patterns: make([]patternEntry, 0),
		fs:       fs,
	}
}

func (afs *ArtifactFileSystem) AddPattern(artifact, pattern, sourceType string) {
	if sourceType == "" {
		sourceType = "FILE"
	}
	afs.patterns = append(afs.patterns, patternEntry{
		artifact:   artifact,
		pattern:    pattern,
		sourceType: sourceType,
	})
}

func (afs *ArtifactFileSystem) Collect(output CollectorOutput) {
	for _, pat := range afs.patterns {
		logger.Log(LevelDebug, fmt.Sprintf("Collecting pattern '%s' for artifact '%s'", pat.pattern, pat.artifact))

		relativePattern := afs.fs.relativePath(pat.pattern)
		genFuncs := afs.fs.parse(relativePattern)

		gen := afs.fs.baseGenerator()
		for _, gf := range genFuncs {
			gen = gf(gen)
		}

		for pathObj := range gen {
			if pat.sourceType == FILE_INFO_TYPE {
				output.AddCollectedFileInfo(pat.artifact, pathObj)
			} else {
				output.AddCollectedFile(pat.artifact, pathObj)
			}
		}
	}
}

type OSFileSystem struct {
	rootPath string
	*ArtifactFileSystem
}

func NewOSFileSystem(path string) *OSFileSystem {
	fs := &OSFileSystem{
		rootPath: filepath.Clean(path),
	}
	fs.ArtifactFileSystem = NewArtifactFileSystem(fs)
	return fs
}

func (fs *OSFileSystem) relativePath(fpath string) string {
	normalizedPath := filepath.ToSlash(fpath)
	normalizedRoot := filepath.ToSlash(fs.rootPath)
	if strings.HasPrefix(normalizedPath, normalizedRoot) {
		relative := normalizedPath[len(normalizedRoot):]
		return strings.TrimLeft(relative, "/")
	}
	return normalizedPath
}

// func (fs *OSFileSystem) parse(pattern string) []GeneratorFunc {
// 	var generators []GeneratorFunc
// 	items := strings.Split(pattern, "/")
// 	for i, item := range items {
// 		isDir := i < len(items)-1

// 		if matches := pathRecursionRegex.FindStringSubmatch(item); len(matches) > 0 {
// 			maxDepth := -1
// 			if matches[1] != "" {
// 				if d, err := strconv.Atoi(matches[1]); err == nil {
// 					maxDepth = d
// 				}
// 			}
// 			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
// 				return NewRecursionPathComponent(isDir, maxDepth, source).Generate()
// 			})
// 		} else if pathGlobRegex.MatchString(item) {
// 			itemCopy := item
// 			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
// 				return NewGlobPathComponent(isDir, itemCopy, source).Generate()
// 			})
// 		} else {
// 			itemCopy := item
// 			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
// 				return NewRegularPathComponent(isDir, itemCopy, source).Generate()
// 			})
// 		}
// 	}
// 	return generators
// }

func (fs *OSFileSystem) parse(pattern string) []GeneratorFunc {
	var generators []GeneratorFunc
	items := strings.Split(pattern, "/")
	for i, item := range items {
		isDir := i < len(items)-1

		if matches := pathRecursionRegex.FindStringSubmatch(item); len(matches) > 0 {
			var maxDepth int
			if matches[1] != "" {
				if d, err := strconv.Atoi(matches[1]); err == nil {
					maxDepth = d
				} else {
					maxDepth = -1
				}
			} else {
				// Если глубина не задана, по умолчанию всегда используем 3
				maxDepth = 3
			}
			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
				return NewRecursionPathComponent(isDir, maxDepth, source).Generate()
			})
		} else if pathGlobRegex.MatchString(item) {
			itemCopy := item
			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
				return NewGlobPathComponent(isDir, itemCopy, source).Generate()
			})
		} else {
			itemCopy := item
			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
				return NewRegularPathComponent(isDir, itemCopy, source).Generate()
			})
		}
	}
	return generators
}

func (fs *OSFileSystem) baseGenerator() <-chan *PathObject {
	out := make(chan *PathObject, 1)
	out <- &PathObject{
		filesystem: fs,
		name:       filepath.Base(fs.rootPath),
		path:       fs.rootPath,
	}
	close(out)
	return out
}

func (fs *OSFileSystem) IsDirectory(p *PathObject) bool {
	info, err := os.Stat(p.path)
	return err == nil && info.IsDir()
}

func (fs *OSFileSystem) IsFile(p *PathObject) bool {
	info, err := os.Stat(p.path)
	return err == nil && !info.IsDir()
}

func (fs *OSFileSystem) IsSymlink(p *PathObject) bool {
	info, err := os.Lstat(p.path)
	return err == nil && (info.Mode()&os.ModeSymlink != 0)
}

func (fs *OSFileSystem) ListDirectory(p *PathObject) []*PathObject {
	var objects []*PathObject
	entries, err := os.ReadDir(p.path)
	if err != nil {
		logger.Log(LevelError, "Error reading directory: "+p.path)
		return objects
	}

	for _, entry := range entries {
		objects = append(objects, &PathObject{
			filesystem: fs,
			name:       entry.Name(),
			path:       filepath.Join(p.path, entry.Name()),
		})
	}
	return objects
}

func (fs *OSFileSystem) GetPath(parent *PathObject, name string) *PathObject {
	return &PathObject{
		filesystem: fs,
		name:       name,
		path:       filepath.Join(parent.path, name),
	}
}

func (fs *OSFileSystem) GetFullPath(fullpath string) *PathObject {
	return &PathObject{
		filesystem: fs,
		name:       filepath.Base(fullpath),
		path:       fullpath,
	}
}

func (fs *OSFileSystem) ReadChunks(p *PathObject) ([]byte, error) {
	if !fs.IsFile(p) {
		return nil, nil
	}

	file, err := os.Open(p.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	buf := make([]byte, CHUNK_SIZE)
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf[:n], nil
}

func (fs *OSFileSystem) GetSize(p *PathObject) int64 {
	info, err := os.Lstat(p.path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// // GeneratorFunc определяет функцию-генератор, которая принимает исходный канал объектов пути и возвращает новый канал.
type GeneratorFunc func(source <-chan *PathObject) <-chan *PathObject

// /ТУТ ВОЗМОЖНО НАДО ПЕРДЕЛАТЬ НА ДРУГОЙ ИНТЕРФЕЙС ИЗ OUTPUT
type CollectorOutput interface {
	AddCollectedFileInfo(artifact string, path *PathObject) error
	AddCollectedFile(artifact string, path *PathObject) error
}
