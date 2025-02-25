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

	"github.com/shirou/gopsutil/disk"
)

const (
	FILE_INFO_TYPE = "FILE_INFO"
	CHUNK_SIZE     = 5 * 1024 * 1024
)

// var TSK_FILESYSTEMS = []string{"NTFS", "ext3", "ext4"}
var TSK_FILESYSTEMS = map[string]bool{
	"NTFS": true,
	"ext3": true,
	"ext4": true,
}
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

//------------------------------------------------------------------------------
// Реализация TSKFileSystem с использованием пакета github.com/dutchcoders/go-tsk

// TSKFileSystem использует pytsk3 для доступа к файловой системе образа.
type TSKFileSystem struct {
	device     string
	mountPoint string
}

func NewTSKFileSystem(device, mountPoint string) (*TSKFileSystem, error) {
	logger.Log(LevelDebug, fmt.Sprintf("Создана заглушка TSKFileSystem для устройства %s на точке монтирования %s", device, mountPoint))
	return &TSKFileSystem{
		device:     device,
		mountPoint: mountPoint,
	}, nil
}

func (fs *TSKFileSystem) AddPattern(artifact, pattern, sourceType string) {
	logger.Log(LevelDebug, fmt.Sprintf("TSKFileSystem.AddPattern (заглушка): artifact=%s, pattern=%s, sourceType=%s", artifact, pattern, sourceType))
}

func (fs *TSKFileSystem) Collect(output CollectorOutput) {
	logger.Log(LevelDebug, "TSKFileSystem.Collect (заглушка) вызвана")
}

func (fs *TSKFileSystem) relativePath(fpath string) string {
	return fpath
}

func (fs *TSKFileSystem) parse(pattern string) []GeneratorFunc {
	return nil
}

func (fs *TSKFileSystem) baseGenerator() <-chan *PathObject {
	out := make(chan *PathObject)
	close(out)
	return out
}

func (fs *TSKFileSystem) IsDirectory(p *PathObject) bool            { return false }
func (fs *TSKFileSystem) IsFile(p *PathObject) bool                 { return false }
func (fs *TSKFileSystem) IsSymlink(p *PathObject) bool              { return false }
func (fs *TSKFileSystem) ListDirectory(p *PathObject) []*PathObject { return nil }
func (fs *TSKFileSystem) GetPath(parent *PathObject, name string) *PathObject {
	return nil
}
func (fs *TSKFileSystem) GetFullPath(fullpath string) *PathObject {
	return &PathObject{
		filesystem: fs,
		name:       filepath.Base(fullpath),
		path:       fullpath,
	}
}
func (fs *TSKFileSystem) ReadChunks(p *PathObject) ([]byte, error) { return nil, nil }
func (fs *TSKFileSystem) GetSize(p *PathObject) int64              { return 0 }

// -----------------------------------------------------------------------------
// FileSystemManager – менеджер файловых систем и сборки артефактов.
// -----------------------------------------------------------------------------

// FileSystemManager управляет набором файловых систем, найденных по точкам монтирования,
// и реализует интерфейс AbstractCollector.
type FileSystemManager struct {
	filesystems map[string]FileSystem // Ключ – точка монтирования
	mountPoints []disk.PartitionStat  // Список точек монтирования
}

// NewFileSystemManager создаёт новый менеджер, получая список точек монтирования с помощью gopsutil.
func NewFileSystemManager() (*FileSystemManager, error) {
	partitions, err := disk.Partitions(true)
	if err != nil {
		return nil, err
	}
	return &FileSystemManager{
		filesystems: make(map[string]FileSystem),
		mountPoints: partitions,
	}, nil
}

// getMountPoint находит наиболее подходящую точку монтирования для указанного пути.
func (fsm *FileSystemManager) getMountPoint(filepath string) (*disk.PartitionStat, error) {
	var best *disk.PartitionStat
	bestLength := 0
	for _, mp := range fsm.mountPoints {
		if strings.HasPrefix(filepath, mp.Mountpoint) {
			if len(mp.Mountpoint) > bestLength {
				best = &mp
				bestLength = len(mp.Mountpoint)
			}
		}
	}
	if best == nil {
		return nil, fmt.Errorf("не найдена точка монтирования для пути %s", filepath)
	}
	return best, nil
}

// getFilesystem возвращает экземпляр FileSystem для указанного пути.
// Если файловая система ещё не создана, производится выбор между TSKFileSystem (если тип поддерживается)
// и OSFileSystem.
// func (fsm *FileSystemManager) getFilesystem(path string) (FileSystem, error) {
// 	mp, err := fsm.getMountPoint(path)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if fs, ok := fsm.filesystems[mp.Mountpoint]; ok {
// 		return fs, nil
// 	}
// 	// Если тип файловой системы входит в TSK_FILESYSTEMS, пытаемся создать TSKFileSystem.
// 	if TSK_FILESYSTEMS[mp.Fstype] {
// 		tsk, err := NewTSKFileSystem(mp.Device, mp.Mountpoint)
// 		if err == nil {
// 			fsm.filesystems[mp.Mountpoint] = tsk
// 			return tsk, nil
// 		}
// 		// При ошибке – переходим к OSFileSystem.
// 	}
// 	// Создаём OSFileSystem как запасной вариант.
// 	osfs := NewOSFileSystem(mp.Mountpoint)
// 	fsm.filesystems[mp.Mountpoint] = osfs
// 	return osfs, nil
// }

// GetPathObject возвращает объект пути, используя метод GetFullPath файловой системы.
func (fsm *FileSystemManager) GetPathObject(path string) (*PathObject, error) {
	fs, err := fsm.getFilesystem(path)
	if err != nil {
		return nil, err
	}
	return fs.GetFullPath(path), nil
}

// AddPattern регистрирует шаблон для указанного артефакта.
// Если шаблон начинается с "\", он применяется ко всем точкам монтирования, для которых тип ФС поддерживается.
func (fsm *FileSystemManager) AddPattern(artifact, pattern, sourceType string) {
	pattern = filepath.Clean(pattern)
	if strings.HasPrefix(pattern, "\\") {
		trimmed := strings.TrimPrefix(pattern, "\\")
		for _, mp := range fsm.mountPoints {
			if TSK_FILESYSTEMS[mp.Fstype] {
				extendedPattern := filepath.Join(mp.Mountpoint, trimmed)
				fs, err := fsm.getFilesystem(extendedPattern)
				if err == nil {
					fs.AddPattern(artifact, extendedPattern, sourceType)
				} else {
					logger.Log(LevelError, fmt.Sprintf("Ошибка получения файловой системы для шаблона %s: %v", extendedPattern, err))
				}
			}
		}
	} else {
		fs, err := fsm.getFilesystem(pattern)
		if err == nil {
			fs.AddPattern(artifact, pattern, sourceType)
		} else {
			logger.Log(LevelError, fmt.Sprintf("Ошибка получения файловой системы для шаблона %s: %v", pattern, err))
		}
	}
}

// Collect проходит по всем файловым системам и вызывает их Collect для сбора артефактов.
func (fsm *FileSystemManager) Collect(output CollectorOutput) {
	for mount, fs := range fsm.filesystems {
		logger.Log(LevelDebug, fmt.Sprintf("Начало сбора для '%s'", mount))
		fs.Collect(output)
	}
}

// // RegisterSource регистрирует источник артефакта. Если тип источника соответствует
// // TYPE_INDICATOR_FILE, TYPE_INDICATOR_PATH или FILE_INFO_TYPE, для каждого пути выполняется
// // подстановка переменных (variables) и добавление шаблона.
// func (fsm *FileSystemManager) RegisterSource(artifactDefinition *ArtifactDefinition, artifactSource *Source, variables *HostVariables) bool {
// 	supported := false
// 	if artifactSource.TypeIndicator == TYPE_INDICATOR_FILE ||
// 		artifactSource.TypeIndicator == TYPE_INDICATOR_PATH ||
// 		artifactSource.TypeIndicator == FILE_INFO_TYPE {

// 		supported = true

// 		pathsInterface, exists := artifactSource.Attributes["paths"]
// 		if !exists {
// 			logger.Log(LevelError, "Нет атрибута 'paths' у источника")
// 			return false
// 		}
// 		pathsSlice, ok := convertToStringSlice(pathsInterface)
// 		if !ok || len(pathsSlice) == 0 {
// 			logger.Log(LevelError, "Неверный или пустой список путей в источнике")
// 			return false
// 		}
// 		for _, p := range pathsSlice {
// 			substituted := variables.Substitute(p)
// 			// Перебираем ключи, а не значения
// 			for sp := range substituted {
// 				if artifactSource.TypeIndicator == TYPE_INDICATOR_PATH && !strings.HasSuffix(sp, "*") {
// 					sp = sp + string(filepath.Separator) + "**-1"
// 				}
// 				fsm.AddPattern(artifactDefinition.Name, sp, artifactSource.TypeIndicator)
// 			}
// 		}

// 	}
// 	return supported
// }

func (fsm *FileSystemManager) getFilesystem(path string) (FileSystem, error) {
	mp, err := fsm.getMountPoint(path)
	if err != nil {
		return nil, err
	}
	if fs, ok := fsm.filesystems[mp.Mountpoint]; ok {
		return fs, nil
	}
	// Приводим тип файловой системы к верхнему регистру для корректного сравнения.
	if TSK_FILESYSTEMS[strings.ToUpper(mp.Fstype)] {
		tsk, err := NewTSKFileSystem(mp.Device, mp.Mountpoint)
		if err == nil {
			fsm.filesystems[mp.Mountpoint] = tsk
			return tsk, nil
		}
		// При ошибке – переходим к OSFileSystem.
	}
	// Создаём OSFileSystem как запасной вариант.
	osfs := NewOSFileSystem(mp.Mountpoint)
	fsm.filesystems[mp.Mountpoint] = osfs
	return osfs, nil
}

// RegisterSource регистрирует источник артефакта и добавляет шаблоны.
// Если шаблон начинается с "\", он применяется ко всем точкам монтирования с поддержкой TSK.
func (fsm *FileSystemManager) RegisterSource(artifactDefinition *ArtifactDefinition, artifactSource *Source, variables *HostVariables) bool {
	supported := false
	if artifactSource.TypeIndicator == TYPE_INDICATOR_FILE ||
		artifactSource.TypeIndicator == TYPE_INDICATOR_PATH ||
		artifactSource.TypeIndicator == FILE_INFO_TYPE {

		supported = true

		pathsInterface, exists := artifactSource.Attributes["paths"]
		if !exists {
			logger.Log(LevelError, "Нет атрибута 'paths' у источника")
			return false
		}
		pathsSlice, ok := convertToStringSlice(pathsInterface)
		if !ok || len(pathsSlice) == 0 {
			logger.Log(LevelError, "Неверный или пустой список путей в источнике")
			return false
		}
		for _, p := range pathsSlice {
			substituted := variables.Substitute(p)
			// Перебираем ключи, а не значения
			for sp := range substituted {
				// Для источников типа PATH добавляем рекурсию, если отсутствует "*"
				if artifactSource.TypeIndicator == TYPE_INDICATOR_PATH && !strings.HasSuffix(sp, "*") {
					sp = sp + string(filepath.Separator) + "**-1"
				}
				// Если шаблон начинается с "\", применяем его ко всем точкам монтирования с поддержкой TSK.
				if strings.HasPrefix(sp, "\\") {
					trimmed := strings.TrimPrefix(sp, "\\")
					for _, mp := range fsm.mountPoints {
						if TSK_FILESYSTEMS[strings.ToUpper(mp.Fstype)] {
							extendedPattern := filepath.Join(mp.Mountpoint, trimmed)
							fs, err := fsm.getFilesystem(extendedPattern)
							if err == nil {
								fs.AddPattern(artifactDefinition.Name, extendedPattern, artifactSource.TypeIndicator)
							} else {
								logger.Log(LevelError, fmt.Sprintf("Ошибка получения файловой системы для шаблона %s: %v", extendedPattern, err))
							}
						}
					}
				} else {
					fs, err := fsm.getFilesystem(sp)
					if err == nil {
						fs.AddPattern(artifactDefinition.Name, sp, artifactSource.TypeIndicator)
					} else {
						logger.Log(LevelError, fmt.Sprintf("Ошибка получения файловой системы для шаблона %s: %v", sp, err))
					}
				}
			}
		}

	}
	return supported
}
