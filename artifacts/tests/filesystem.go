package main

import (
	"encoding/hex"
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

// Константы
const (
	FILE_INFO_TYPE = "FILE_INFO"
	CHUNK_SIZE     = 5 * 1024 * 1024
)

// Поддерживаемые типы ФС для работы через TSK (NTFS и подобное)
var TSK_FILESYSTEMS = map[string]bool{
	"NTFS": true,
	"ext3": true,
	"ext4": true,
}

var (
	pathRecursionRegex = regexp.MustCompile(`\*\*((-1|\d*))`)
	pathGlobRegex      = regexp.MustCompile(`\*|\?|\[.+\]`)
)

// --- Интерфейс FileSystem и вспомогательные структуры --- //

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

		rel := afs.fs.relativePath(pat.pattern)
		gen := afs.fs.baseGenerator()
		for _, gf := range afs.fs.parse(rel) {
			gen = gf(gen)
		}

		for po := range gen {
			//logger.Log(LevelInfo, fmt.Sprintf("Matched '%s' → %s", pat.pattern, po.path))
			if pat.sourceType == FILE_INFO_TYPE {
				output.AddCollectedFileInfo(pat.artifact, po)
			} else {
				output.AddCollectedFile(pat.artifact, po)
			}
		}
	}
}

// ------------------- OSFileSystem (доступ через os) ------------------- //

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
	// Если первый сегмент пути — NTFS‑спецфайл (начинается с '$'),
	// сразу создаём PathObject через TSKFileSystem, минуя list_directory
	parts := strings.Split(pattern, "/")
	if len(parts) > 0 && strings.HasPrefix(parts[0], "$") {
		return []GeneratorFunc{
			func(source <-chan *PathObject) <-chan *PathObject {
				out := make(chan *PathObject, 1)
				// потребляем исходный элемент (корень)
				<-source

				fullPath := filepath.Join(fs.rootPath, filepath.FromSlash(pattern))
				logger.Log(LevelInfo, fmt.Sprintf("TSK‑route: emitting protected file '%s' via TSK", fullPath))

				tskFS, err := NewTSKFileSystem(fs.rootPath, fs.rootPath)
				if err != nil {
					logger.Log(LevelError, fmt.Sprintf("TSK‑route: cannot create TSKFileSystem: %v", err))
					close(out)
					return out
				}

				po := &PathObject{
					filesystem: tskFS,
					name:       filepath.Base(fullPath),
					path:       fullPath,
				}
				out <- po
				close(out)
				return out
			},
		}
	}

	// Обычная логика разбора шаблона:
	var generators []GeneratorFunc
	items := strings.Split(pattern, "/")
	for i, item := range items {
		isDir := i < len(items)-1
		switch {
		case pathRecursionRegex.MatchString(item):
			matches := pathRecursionRegex.FindStringSubmatch(item)
			maxDepth := -1
			if matches[1] != "" {
				if d, err := strconv.Atoi(matches[1]); err == nil {
					maxDepth = d
				}
			}
			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
				return NewRecursionPathComponent(isDir, maxDepth, source).Generate()
			})

		case pathGlobRegex.MatchString(item):
			itemCopy := item
			generators = append(generators, func(source <-chan *PathObject) <-chan *PathObject {
				return NewGlobPathComponent(isDir, itemCopy, source).Generate()
			})

		default:
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

var allowed = map[string]bool{
	"$MFT":     true,
	"$MFTMirr": true,
	"$LogFile": true,
	"$Extend":  true,
	"$UsnJrnl": true,
}

// OSFileSystem.ListDirectory — теперь явно разрешает системные файлы и логгирует фильтрацию
func (fs *OSFileSystem) ListDirectory(p *PathObject) []*PathObject {
	var objects []*PathObject
	entries, err := os.ReadDir(p.path)
	if err != nil {
		if os.IsPermission(err) {
			logger.Log(LevelWarning, fmt.Sprintf("Skipping directory due to permissions: %s", p.path))
			return nil
		}
		logger.Log(LevelError, fmt.Sprintf("Error reading directory: %s - %v", p.path, err))
		return nil
	}
	for _, entry := range entries {
		name := entry.Name()
		full := filepath.Join(p.path, name)

		// Явно включаем NTFS‑потоки
		switch name {
		case "$MFT", "$MFTMirr", "$LogFile", "$Extend", "$UsnJrnl":
			logger.Log(LevelDebug, fmt.Sprintf("Including NTFS system stream: %s", full))
			objects = append(objects, &PathObject{filesystem: fs, name: name, path: full})
			continue
		}

		// Пропускаем временные Office-файлы
		if strings.HasPrefix(name, "~$") {
			logger.Log(LevelDebug, fmt.Sprintf("Skipping temp file: %s", full))
			continue
		}

		// Всё прочее
		objects = append(objects, &PathObject{filesystem: fs, name: name, path: full})
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

// OSFileSystem.ReadChunks — сначала ловим все защищённые файлы по "$" или реестру, затем уже проверяем IsFile
func (fs *OSFileSystem) ReadChunks(p *PathObject) ([][]byte, error) {
	// 1) Любые NTFS-потоки и hive-файлы реестра — через TSK
	if strings.HasPrefix(p.name, "$") || isRegistryFile(p.name) {
		logger.Log(LevelInfo, fmt.Sprintf("Protected file detected: %s, switching to TSK", p.path))
		tskFS, err := NewTSKFileSystem(fs.rootPath, fs.rootPath)
		if err != nil {
			logger.Log(LevelError, fmt.Sprintf("Failed to create TSKFileSystem for %s: %v", fs.rootPath, err))
			return nil, err
		}
		return tskFS.ReadChunks(p)
	}

	// 2) Если не файл — пропускаем
	if !fs.IsFile(p) {
		logger.Log(LevelDebug, fmt.Sprintf("Skipping non-file: %s", p.path))
		return nil, nil
	}

	// 3) Обычное чтение через os
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

func isRegistryFile(name string) bool {
	registryFiles := []string{"SYSTEM", "SOFTWARE", "SAM", "SECURITY", "DEFAULT", "NTUSER.DAT"}
	for _, reg := range registryFiles {
		if strings.EqualFold(name, reg) {
			return true
		}
	}
	return false
}

type GeneratorFunc func(source <-chan *PathObject) <-chan *PathObject

// ------------------- TSKFileSystem (используется только для MFT и реестра) -------------------

// DirEntry – структура, получаемая из Python-скрипта
type DirEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	MetaType string `json:"meta_type"`
	Size     int64  `json:"size"`
}

// TSKFileSystem – используется только для MFT и файлов реестра
type TSKFileSystem struct {
	device       string
	mountPoint   string
	entriesCache map[string][]*PathObject
	sizeCache    map[string]int // кэш размеров файлов
	rootCache    *DirEntry      // кэш корневого каталога
	*ArtifactFileSystem
}

// NewTSKFileSystem создаёт новый TSKFileSystem.
func NewTSKFileSystem(device, mountPoint string) (*TSKFileSystem, error) {
	// На Windows приводим mountPoint к Unix‑стилю с завершающим слешем.
	if runtime.GOOS == "windows" {
		mountPoint = filepath.ToSlash(mountPoint)
		if !strings.HasSuffix(mountPoint, "/") {
			mountPoint += "/"
		}
	}
	logger.Log(LevelDebug, fmt.Sprintf("Создан TSKFileSystem для устройства %s на точке монтирования %s", device, mountPoint))
	tskFS := &TSKFileSystem{
		device:       device,
		mountPoint:   mountPoint,
		entriesCache: make(map[string][]*PathObject),
		sizeCache:    make(map[string]int),
	}
	tskFS.ArtifactFileSystem = NewArtifactFileSystem(tskFS)
	return tskFS, nil
}

func (fs *TSKFileSystem) parse(pattern string) []GeneratorFunc {
	return []GeneratorFunc{
		func(source <-chan *PathObject) <-chan *PathObject {
			out := make(chan *PathObject, 1)
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

func (fs *TSKFileSystem) Collect(output *Outputs) {
	fs.ArtifactFileSystem.Collect(output)
}

func (fs *TSKFileSystem) relativePath(fpath string) string {
	normalizedPath := filepath.ToSlash(fpath)
	normalizedRoot := filepath.ToSlash(fs.mountPoint)
	if !strings.HasSuffix(normalizedRoot, "/") {
		normalizedRoot += "/"
	}
	if strings.HasPrefix(normalizedPath, normalizedRoot) {
		return normalizedPath[len(normalizedRoot):]
	}
	return normalizedPath
}

func (fs *TSKFileSystem) baseGenerator() <-chan *PathObject {
	out := make(chan *PathObject, 1)
	// Используем кэшированный корневой каталог, если он уже получен
	if fs.rootCache != nil {
		po := &PathObject{
			filesystem: fs,
			name:       fs.rootCache.Name,
			path:       fs.rootCache.Path,
			obj:        fs.rootCache,
		}
		out <- po
		close(out)
		return out
	}
	logger.Log(LevelDebug, fmt.Sprintf("TSK: Вызов команды get_root для точки монтирования: %s", fs.mountPoint))
	resp, err := runPython("get_root", fs.mountPoint)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("TSK: Ошибка получения корневого каталога для %s: %v", fs.mountPoint, err))
		close(out)
		return out
	}
	logger.Log(LevelDebug, fmt.Sprintf("TSK: Ответ get_root: %s", resp))
	var rootEntry DirEntry
	if err := json.Unmarshal([]byte(resp), &rootEntry); err != nil {
		logger.Log(LevelError, fmt.Sprintf("TSK: Ошибка парсинга JSON для корневого каталога: %v", err))
		close(out)
		return out
	}
	fs.rootCache = &rootEntry
	logger.Log(LevelInfo, fmt.Sprintf("TSK: Корневой каталог: %s", rootEntry.Path))
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
		logger.Log(LevelError, fmt.Sprintf("TSK: Ошибка is_directory для %s: %v", p.path, err))
		return false
	}
	return strings.TrimSpace(resp) == "true"
}

func (fs *TSKFileSystem) IsFile(p *PathObject) bool {
	resp, err := runPython("is_file", p.path)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("TSK: Ошибка is_file для %s: %v", p.path, err))
		return false
	}
	return strings.TrimSpace(resp) == "true"
}

func (fs *TSKFileSystem) IsSymlink(p *PathObject) bool {
	resp, err := runPython("is_symlink", p.path)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("TSK: Ошибка is_symlink для %s: %v", p.path, err))
		return false
	}
	return strings.TrimSpace(resp) == "true"
}

func (fs *TSKFileSystem) ListDirectory(p *PathObject) []*PathObject {
	logger.Log(LevelDebug, fmt.Sprintf("TSK: Вызов команды list_directory для %s", p.path))
	if entries, ok := fs.entriesCache[p.path]; ok {
		logger.Log(LevelDebug, fmt.Sprintf("TSK: Используем кэш для %s", p.path))
		return entries
	}
	resp, err := runPython("list_directory", p.path)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("TSK: Ошибка чтения каталога %s: %v", p.path, err))
		return nil
	}
	logger.Log(LevelDebug, fmt.Sprintf("TSK: Ответ list_directory для %s: %s", p.path, resp))
	var dirEntries []DirEntry
	if err := json.Unmarshal([]byte(resp), &dirEntries); err != nil {
		logger.Log(LevelError, fmt.Sprintf("TSK: Ошибка парсинга JSON для каталога %s: %v", p.path, err))
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
			logger.Log(LevelDebug, fmt.Sprintf("TSK: Обработка символической ссылки %s в каталоге %s", entry.Name, p.path))
			followResp, err := runPython("follow_symlink", p.path, entry.Name)
			if err != nil {
				logger.Log(LevelWarning, fmt.Sprintf("TSK: Ошибка follow_symlink для %s: %v", entry.Name, err))
			} else if trimmed := strings.TrimSpace(followResp); trimmed != "" {
				logger.Log(LevelDebug, fmt.Sprintf("TSK: Символическая ссылка %s указывает на %s", entry.Name, trimmed))
				osfs := NewOSFileSystem("/") // fallback через OSFileSystem для симлинков
				po = osfs.GetFullPath(trimmed)
			}
		}
		objects = append(objects, po)
	}
	fs.entriesCache[p.path] = objects
	logger.Log(LevelDebug, fmt.Sprintf("TSK: Найдено %d объектов в %s", len(objects), p.path))
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
	// Для одного файла вызываем BatchReadChunks с единственным элементом.
	resultMap, err := fs.BatchReadChunks([]*PathObject{p})
	if err != nil {
		return nil, err
	}
	chunks, exists := resultMap[p.GetPath()]
	if !exists {
		return nil, fmt.Errorf("Нет результата для %s", p.GetPath())
	}
	return chunks, nil
}

func (fs *TSKFileSystem) GetSize(p *PathObject) int64 {
	logger.Log(LevelDebug, fmt.Sprintf("TSK: Вызов GetSize для %s", p.path))
	resp, err := runPython("get_size", p.path)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("TSK: Ошибка получения размера для %s: %v", p.path, err))
		return 0
	}
	trimmed := strings.TrimSpace(resp)
	logger.Log(LevelDebug, fmt.Sprintf("TSK: Ответ get_size для %s: '%s'", p.path, trimmed))
	size, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("TSK: Ошибка парсинга размера для %s: %v", p.path, err))
		return 0
	}
	return size
}

func runPython(command string, args ...string) (string, error) {
	pythonCmd := "python3"
	if runtime.GOOS == "windows" {
		pythonCmd = "python"
	}
	pyArgs := append([]string{"tsk_helper.py", command}, args...)
	cmd := exec.Command(pythonCmd, pyArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения python-команды: %v, вывод: %s", err, output)
	}
	return string(output), nil
}

// ------------------- FileSystemManager -------------------

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

// getMountPoint выбирает лучшую точку монтирования для указанного пути, с учетом особенностей ОС.
func (fsm *FileSystemManager) getMountPoint(path string) (*disk.PartitionStat, error) {
	if runtime.GOOS == "windows" {
		// Для Windows используем ToSlash для нормализации пути.
		path = filepath.ToSlash(path)
		var best *disk.PartitionStat
		bestLength := 0
		for _, mp := range fsm.mountPoints {
			mpt := filepath.ToSlash(mp.Mountpoint)
			if strings.HasPrefix(path, mpt) && len(mpt) > bestLength {
				best = &mp
				bestLength = len(mpt)
			}
		}
		if best == nil {
			return nil, fmt.Errorf("не найдена точка монтирования для пути %s", path)
		}
		return best, nil
	} else {
		// Для Unix нормализуем путь через Clean.
		path = filepath.Clean(path)
		var best *disk.PartitionStat
		bestLength := 0
		for _, mp := range fsm.mountPoints {
			mpt := filepath.Clean(mp.Mountpoint)
			if strings.HasPrefix(path, mpt) && len(mpt) > bestLength {
				best = &mp
				bestLength = len(mpt)
			}
		}
		if best == nil {
			return nil, fmt.Errorf("mount point not found for path: %s", path)
		}
		return best, nil
	}
}

// GetPathObject возвращает объект пути, полученный из файловой системы.
func (fsm *FileSystemManager) GetPathObject(path string) (*PathObject, error) {
	fs, err := fsm.getFilesystem(path)
	if err != nil {
		return nil, err
	}
	return fs.GetFullPath(path), nil
}

// AddPattern добавляет шаблон для артефакта в соответствующую файловую систему.
func (fsm *FileSystemManager) AddPattern(artifact, pattern, sourceType string) {
	// Осуществляем нормализацию пути.
	pattern = filepath.Clean(pattern)
	if len(pattern) > 0 && pattern[0] == '\\' {
		for _, mp := range fsm.mountPoints {
			// Используем TSK для тех файловых систем, которые поддерживаются.
			if TSK_FILESYSTEMS[mp.Fstype] {
				extendedPattern := filepath.Join(mp.Mountpoint, pattern[1:])
				filesystem := fsm.getFilesystemOrError(extendedPattern)
				if filesystem != nil {
					filesystem.AddPattern(artifact, extendedPattern, sourceType)
				}
			}
		}
	} else {
		filesystem := fsm.getFilesystemOrError(pattern)
		if filesystem != nil {
			filesystem.AddPattern(artifact, pattern, sourceType)
		}
	}
}

// getFilesystemOrError возвращает файловую систему для указанного пути или логирует ошибку.
func (fsm *FileSystemManager) getFilesystemOrError(path string) FileSystem {
	fs, err := fsm.getFilesystem(path)
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("Ошибка получения файловой системы для шаблона %s: %v", path, err))
		return nil
	}
	return fs
}

// Collect вызывает сбор артефактов для каждой файловой системы.
func (fsm *FileSystemManager) Collect(output *Outputs) {
	for mount, fs := range fsm.filesystems {
		logger.Log(LevelDebug, fmt.Sprintf("Начало сбора для '%s'", mount))
		fs.Collect(output)
	}
}

// getFilesystem определяет, какую файловую систему использовать для указанного пути.
// FileSystemManager.getFilesystem — теперь логгирует выбор TSK для любых защищённых файлов
// FileSystemManager.getFilesystem — теперь логируем факт выбора TSK для любых защищённых путей с "$"
func (fsm *FileSystemManager) getFilesystem(path string) (FileSystem, error) {
	// Разрешаем переменные
	resolvedMap := fsm.variables.Substitute(path)
	if len(resolvedMap) == 0 {
		return nil, fmt.Errorf("path resolution failed for: %s", path)
	}
	var resolvedPath string
	for p := range resolvedMap {
		resolvedPath = p
		break
	}
	resolvedPath = filepath.Clean(resolvedPath)

	// Определяем volume
	volume := filepath.VolumeName(resolvedPath)
	if volume == "" {
		volume = filepath.VolumeName(filepath.Clean(resolvedPath))
	}
	if fs, ok := fsm.filesystems[volume]; ok {
		return fs, nil
	}

	// Ищем точку монтирования
	mp, err := fsm.getMountPoint(resolvedPath)
	if err != nil {
		return nil, err
	}

	var chosenFS FileSystem
	up := strings.ToUpper(resolvedPath)

	// Любые пути с "$" или системные hive-файлы — через TSK
	// Принудительно через TSK для любых потоков NTFS и hive-файлов
	if strings.Contains(up, "$LOGFILE") || strings.Contains(up, "$EXTEND") ||
		strings.Contains(up, "$MFT") || strings.Contains(up, "SYSTEM32/CONFIG") {
		supported := false
		if runtime.GOOS == "windows" {
			supported = TSK_FILESYSTEMS[mp.Fstype]
		} else {
			supported = TSK_FILESYSTEMS[strings.ToLower(mp.Fstype)]
		}
		if supported {
			tskFS, err := NewTSKFileSystem(mp.Device, mp.Mountpoint)
			if err != nil {
				logger.Log(LevelError, fmt.Sprintf("Cannot create TSKFileSystem for %s: %v", mp.Mountpoint, err))
			} else {
				logger.Log(LevelInfo, fmt.Sprintf("TSK selected for protected path %s on %s", resolvedPath, mp.Mountpoint))
				chosenFS = tskFS
			}
		}
	}

	// По умолчанию — OSFileSystem
	if chosenFS == nil {
		osfs := NewOSFileSystem(volume + string(filepath.Separator))
		logger.Log(LevelInfo, fmt.Sprintf("Using OSFileSystem for %s", resolvedPath))
		chosenFS = osfs
	}

	fsm.filesystems[volume] = chosenFS
	return chosenFS, nil
}

// RegisterSource регистрирует источник артефакта в файловой системе, если он поддерживается.
// FileSystemManager.RegisterSource — обновлённая версия для единоразового определения ФС по абсолютному пути
func (fsm *FileSystemManager) RegisterSource(
	artifactDefinition *ArtifactDefinition,
	artifactSource *Source,
	variables *HostVariables,
) bool {
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

		for _, patternStr := range pathsSlice {
			// нормализуем разделители
			patternStr = strings.ReplaceAll(patternStr, "\\", "/")
			substitutedMap := variables.Substitute(patternStr)
			for resPath := range substitutedMap {
				resolvedPath := strings.ReplaceAll(resPath, "\\", "/")

				// для TYPE_INDICATOR_PATH добавляем рекурсивный суффикс
				if artifactSource.TypeIndicator == TYPE_INDICATOR_PATH && !strings.HasSuffix(resolvedPath, "*") {
					resolvedPath = resolvedPath + string(filepath.Separator) + "**-1"
				}

				// единоразово выбираем файловую систему по пути
				fs := fsm.getFilesystemOrError(resolvedPath)
				if fs == nil {
					continue
				}

				// логируем факт регистрации абсолютного пути
				if strings.HasPrefix(resolvedPath, string(filepath.Separator)) {
					logger.Log(LevelDebug,
						fmt.Sprintf("Registering absolute path '%s' on FS %T for artifact '%s'",
							resolvedPath, fs, artifactDefinition.Name))
				}

				// добавляем шаблон на найденную ФС
				fs.AddPattern(artifactDefinition.Name, resolvedPath, artifactSource.TypeIndicator)
			}
		}
	}
	return supported
}

// isValidWindowsPattern проверяет корректность шаблона пути для Windows.
func isValidWindowsPattern(pattern string) bool {
	if runtime.GOOS != "windows" {
		return true
	}
	matched, _ := regexp.MatchString(`^[A-Za-z]:[\\/].+`, pattern)
	return matched
}

// BatchReadChunks принимает список путей (для одного тома) и возвращает для каждого путь список чанков.
func (fs *TSKFileSystem) BatchReadChunks(paths []*PathObject) (map[string][][]byte, error) {
	var pathList []string
	for _, p := range paths {
		pathList = append(pathList, p.GetPath())
	}
	inputJSON, err := json.Marshal(pathList)
	if err != nil {
		return nil, err
	}
	output, err := runPythonWithInput("batch_collect", string(inputJSON))
	if err != nil {
		return nil, err
	}
	var results map[string]string
	err = json.Unmarshal([]byte(output), &results)
	if err != nil {
		return nil, err
	}
	resultChunks := make(map[string][][]byte)
	for path, hexStr := range results {
		if hexStr == "" {
			resultChunks[path] = nil
		} else {
			data, err := hex.DecodeString(hexStr)
			if err != nil {
				return nil, err
			}
			// Разбиваем данные на чанки, если они превышают CHUNK_SIZE.
			chunks := splitIntoChunks(data)
			resultChunks[path] = chunks
		}
	}
	return resultChunks, nil
}

// runPythonWithInput выполняет вызов python‑скрипта с передачей данных через stdin.
func runPythonWithInput(command string, input string) (string, error) {
	pythonCmd := "python3"
	if runtime.GOOS == "windows" {
		pythonCmd = "python"
	}
	cmd := exec.Command(pythonCmd, "tsk_helper.py", command)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, input)
	}()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения python-команды: %v, вывод: %s", err, output)
	}
	return string(output), nil
}

// splitIntoChunks разбивает данные на чанки по CHUNK_SIZE.
func splitIntoChunks(data []byte) [][]byte {
	var chunks [][]byte
	for i := 0; i < len(data); i += CHUNK_SIZE {
		end := i + CHUNK_SIZE
		if end > len(data) {
			end = len(data)
		}
		chunk := make([]byte, end-i)
		copy(chunk, data[i:end])
		chunks = append(chunks, chunk)
	}
	return chunks
}
