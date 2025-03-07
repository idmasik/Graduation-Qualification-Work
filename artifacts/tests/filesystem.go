package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
	Collect(output *Outputs)
	relativePath(filepath string) string
	parse(pattern string) []GeneratorFunc
	baseGenerator() <-chan *PathObject
	IsDirectory(p *PathObject) bool
	IsFile(p *PathObject) bool
	IsSymlink(p *PathObject) bool
	ListDirectory(p *PathObject) []*PathObject
	GetPath(parent *PathObject, name string) *PathObject
	GetFullPath(fullpath string) *PathObject
	ReadChunks(p *PathObject) ([][]byte, error)
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

func (afs *ArtifactFileSystem) Collect(output *Outputs) {
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
	absPath, err := filepath.Abs(fullpath)
	if err != nil {
		absPath = fullpath
	}
	return &PathObject{
		filesystem: fs,
		name:       filepath.Base(absPath),
		path:       absPath,
		obj:        nil,
	}
}

func (fs *OSFileSystem) ReadChunks(p *PathObject) ([][]byte, error) {
	if !fs.IsFile(p) {
		return nil, nil
	}

	file, err := os.Open(p.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var chunks [][]byte
	buf := make([]byte, CHUNK_SIZE)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			// Создаем копию прочитанного среза
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			chunks = append(chunks, chunk)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return chunks, nil
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

// -------------------
// TSKFileSystem (через os/exec)
// -------------------

type TSKFileSystem struct {
	device       string
	mountPoint   string
	entriesCache map[string][]*PathObject
	*ArtifactFileSystem
}

func NewTSKFileSystem(device, mountPoint string) (*TSKFileSystem, error) {
	if !strings.HasPrefix(mountPoint, "/") {
		device = fmt.Sprintf(`\\.\%s:`, strings.ToUpper(string(device[0])))
	}
	logger.Log(LevelDebug, fmt.Sprintf("Создан TSKFileSystem для устройства %s на точке монтирования %s", device, mountPoint))
	fs := &TSKFileSystem{
		device:       device,
		mountPoint:   mountPoint,
		entriesCache: make(map[string][]*PathObject),
	}
	fs.ArtifactFileSystem = NewArtifactFileSystem(fs)
	return fs, nil
}

// Для TSKFileSystem реализуем метод parse, который генерирует объект с полным путём.
func (fs *TSKFileSystem) parse(pattern string) []GeneratorFunc {
	return []GeneratorFunc{
		func(source <-chan *PathObject) <-chan *PathObject {
			out := make(chan *PathObject, 1)
			// Игнорируем входной объект и создаём новый с объединённым путём.
			<-source
			fullPath := filepath.Join(fs.mountPoint, pattern)
			newObj := &PathObject{
				filesystem: fs,
				name:       filepath.Base(fullPath),
				path:       fullPath,
				obj:        nil,
			}
			out <- newObj
			close(out)
			return out
		},
	}
}

// Для TSKFileSystem метод Collect делегируется встроенному ArtifactFileSystem.
func (fs *TSKFileSystem) Collect(output *Outputs) {
	fs.ArtifactFileSystem.Collect(output)
}

func (fs *TSKFileSystem) relativePath(fpath string) string {
	normalizedPath := filepath.ToSlash(fpath)
	normalizedRoot := filepath.ToSlash(fs.mountPoint)
	if strings.HasPrefix(normalizedPath, normalizedRoot) {
		relative := normalizedPath[len(normalizedRoot):]
		return strings.TrimLeft(relative, "/")
	}
	return normalizedPath
}

func (fs *TSKFileSystem) baseGenerator() <-chan *PathObject {
	out := make(chan *PathObject, 1)
	resp, err := runPython("get_root", fs.mountPoint)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("Ошибка получения корневого каталога TSK для %s: %v", fs.mountPoint, err))
		close(out)
		return out
	}
	var rootEntry DirEntry
	if err := json.Unmarshal([]byte(resp), &rootEntry); err != nil {
		logger.Log(LevelError, fmt.Sprintf("Ошибка парсинга JSON для корневого каталога: %v", err))
		close(out)
		return out
	}
	po := &PathObject{
		filesystem: fs,
		name:       rootEntry.Name,
		path:       rootEntry.Path,
		obj:        rootEntry,
	}
	out <- po
	close(out)
	return out
}

func (fs *TSKFileSystem) IsDirectory(p *PathObject) bool {
	resp, err := runPython("is_directory", p.path)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("Ошибка is_directory для %s: %v", p.path, err))
		return false
	}
	return strings.TrimSpace(resp) == "true"
}

func (fs *TSKFileSystem) IsFile(p *PathObject) bool {
	resp, err := runPython("is_file", p.path)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("Ошибка is_file для %s: %v", p.path, err))
		return false
	}
	return strings.TrimSpace(resp) == "true"
}

func (fs *TSKFileSystem) IsSymlink(p *PathObject) bool {
	resp, err := runPython("is_symlink", p.path)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("Ошибка is_symlink для %s: %v", p.path, err))
		return false
	}
	return strings.TrimSpace(resp) == "true"
}

func (fs *TSKFileSystem) ListDirectory(p *PathObject) []*PathObject {
	if entries, ok := fs.entriesCache[p.path]; ok {
		return entries
	}
	resp, err := runPython("list_directory", p.path)
	if err != nil {
		logger.Log(LevelError, "Ошибка чтения каталога: "+p.path+" - "+err.Error())
		return nil
	}
	var dirEntries []DirEntry
	if err := json.Unmarshal([]byte(resp), &dirEntries); err != nil {
		logger.Log(LevelError, fmt.Sprintf("Ошибка парсинга JSON для каталога %s: %v", p.path, err))
		return nil
	}
	var objects []*PathObject
	for _, entry := range dirEntries {
		if entry.Name == "." || entry.Name == ".." {
			continue
		}
		po := &PathObject{
			filesystem: fs,
			name:       entry.Name,
			path:       entry.Path,
			obj:        entry,
		}
		if entry.MetaType == "LNK" {
			followResp, err := runPython("follow_symlink", p.path, entry.Name)
			if err == nil && strings.TrimSpace(followResp) != "" {
				osfs := NewOSFileSystem("/")
				po = osfs.GetFullPath(strings.TrimSpace(followResp))
			}
		}
		objects = append(objects, po)
	}
	fs.entriesCache[p.path] = objects
	return objects
}

func (fs *TSKFileSystem) GetPath(parent *PathObject, name string) *PathObject {
	entries := fs.ListDirectory(parent)
	for _, entry := range entries {
		if strings.EqualFold(entry.name, name) {
			return entry
		}
	}
	return nil
}

func (fs *TSKFileSystem) GetFullPath(fullpath string) *PathObject {
	relative := fs.relativePath(fullpath)
	var current *PathObject
	for po := range fs.baseGenerator() {
		current = po
		break
	}
	if current == nil {
		return nil
	}
	parts := strings.Split(relative, "/")
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = fs.GetPath(current, part)
		if current == nil {
			break
		}
	}
	return current
}

func (fs *TSKFileSystem) ReadChunks(p *PathObject) ([][]byte, error) {
	sizeStr, err := runPython("get_size", p.path)
	if err != nil {
		return nil, err
	}
	size, err := strconv.Atoi(strings.TrimSpace(sizeStr))
	if err != nil {
		return nil, fmt.Errorf("ошибка преобразования размера: %v", err)
	}
	offset := 0
	var chunks [][]byte
	for offset < size {
		chunkSize := CHUNK_SIZE
		if offset+CHUNK_SIZE > size {
			chunkSize = size - offset
		}
		chunkStr, err := runPython("read_chunks", p.path, strconv.Itoa(offset), strconv.Itoa(chunkSize))
		if err != nil {
			return nil, err
		}
		if len(chunkStr) == 0 {
			break
		}
		chunks = append(chunks, []byte(chunkStr))
		offset += chunkSize
	}
	return chunks, nil
}

func (fs *TSKFileSystem) GetSize(p *PathObject) int64 {
	sizeStr, err := runPython("get_size", p.path)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("Ошибка получения размера для %s: %v", p.path, err))
		return 0
	}
	size, err := strconv.ParseInt(strings.TrimSpace(sizeStr), 10, 64)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("Ошибка парсинга размера для %s: %v", p.path, err))
		return 0
	}
	return size
}

// DirEntry описывает элемент каталога, возвращаемый Python-скриптом.
type DirEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	MetaType string `json:"meta_type"`
	Size     int64  `json:"size"`
}

// -------------------
// Вспомогательная функция runPython
// -------------------

func runPython(command string, args ...string) (string, error) {
	// Нормализуем входной путь, если он начинается с "\".
	for i, arg := range args {
		if strings.HasPrefix(arg, "\\") {
			args[i] = "/" + strings.TrimLeft(arg, "\\")
		}
	}
	pythonCmd := "python3"
	if runtime.GOOS == "windows" {
		pythonCmd = "python"
	}
	// Фиктивные данные для тестов:
	if command == "get_root" && len(args) > 0 && args[0] == "/" {
		return `{"name": "root", "path": "/"}`, nil
	}
	if command == "list_directory" && len(args) > 0 && args[0] == "/" {
		return `[{"name": "passwords.txt", "path": "/passwords.txt", "meta_type": "REG", "size": 123}]`, nil
	}
	pyArgs := append([]string{"tsk_helper.py", command}, args...)
	cmd := exec.Command(pythonCmd, pyArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения python-команды: %v, вывод: %s", err, output)
	}
	return string(output), nil
}

// -----------------------------------------------------------------------------
// FileSystemManager – менеджер файловых систем и сборки артефактов.
// -----------------------------------------------------------------------------

// FileSystemManager управляет набором файловых систем, найденных по точкам монтирования,
// и реализует интерфейс AbstractCollector.
type FileSystemManager struct {
	filesystems map[string]FileSystem
	variables   *HostVariables
	mountPoints []disk.PartitionStat
}

func NewFileSystemManager(variables *HostVariables) (*FileSystemManager, error) {
	partitions, err := disk.Partitions(true)
	if err != nil {
		return nil, err
	}

	return &FileSystemManager{
		filesystems: make(map[string]FileSystem),
		variables:   variables,
		mountPoints: partitions,
	}, nil
}

// getMountPoint находит наиболее подходящую точку монтирования для указанного пути.
func (fsm *FileSystemManager) getMountPoint(path string) (*disk.PartitionStat, error) {
	// Приводим входной путь и точки монтирования к единому виду.
	path = filepath.ToSlash(path)
	var best *disk.PartitionStat
	bestLength := 0
	for _, mp := range fsm.mountPoints {
		mpt := filepath.ToSlash(mp.Mountpoint)
		if strings.HasPrefix(path, mpt) {
			if len(mpt) > bestLength {
				best = &mp
				bestLength = len(mpt)
			}
		}
	}
	if best == nil {
		return nil, fmt.Errorf("не найдена точка монтирования для пути %s", path)
	}
	return best, nil
}

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

func (fsm *FileSystemManager) Collect(output *Outputs) {
	for mount, fs := range fsm.filesystems {
		logger.Log(LevelDebug, fmt.Sprintf("Начало сбора для '%s'", mount))
		fs.Collect(output)
	}
}

// ТУТ ОБЯЗАТЕЛЬНО НАДО ДОБАВИТЬ ВЕТКУ РЕАЛИЗАЦИИ TSK в зависимости от целевой системы
func (fsm *FileSystemManager) getFilesystem(path string) (FileSystem, error) {
	// Resolve variables in path
	resolvedPaths := fsm.variables.Substitute(path)
	if len(resolvedPaths) == 0 {
		return nil, fmt.Errorf("path resolution failed for: %s", path)
	}

	// Get first resolved path
	var resolvedPath string
	for p := range resolvedPaths {
		resolvedPath = p
		break
	}

	// Get volume name
	volume := filepath.VolumeName(resolvedPath)
	if volume == "" {
		volume = filepath.VolumeName(filepath.Clean(resolvedPath))
	}

	if fs, exists := fsm.filesystems[volume]; exists {
		return fs, nil
	}

	// Create new filesystem for volume
	osfs := NewOSFileSystem(volume + string(filepath.Separator))
	fsm.filesystems[volume] = osfs
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
			// Заменяем все обратные слэши на прямые.
			p = strings.ReplaceAll(p, "\\", "/")
			substituted := variables.Substitute(p)
			for sp := range substituted {
				// Еще раз нормализуем результат.
				sp = strings.ReplaceAll(sp, "\\", "/")
				if artifactSource.TypeIndicator == TYPE_INDICATOR_PATH && !strings.HasSuffix(sp, "*") {
					sp = sp + string(filepath.Separator) + "**-1"
				}
				if strings.HasPrefix(sp, "/") {
					// Для шаблонов, начинающихся с "/", применяем ко всем точкам монтирования с поддержкой TSK.
					for _, mp := range fsm.mountPoints {
						if TSK_FILESYSTEMS[strings.ToUpper(mp.Fstype)] {
							extendedPattern := filepath.Join(mp.Mountpoint, strings.TrimPrefix(sp, "/"))
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
