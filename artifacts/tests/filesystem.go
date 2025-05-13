package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	ntfsfs "github.com/forensicanalysis/fslib/ntfs"
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
			if pat.sourceType == FILE_INFO_TYPE || output.sha256 {
				output.AddCollectedFile(pat.artifact, po)
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
	// Сохраняем завершающий разделитель, иначе filepath.Clean снимет его (e.g. "C:\" → "C:")
	cleaned := filepath.Clean(path)
	if !strings.HasSuffix(cleaned, string(os.PathSeparator)) {
		cleaned += string(os.PathSeparator)
	}
	//logger.Log(LevelDebug, fmt.Sprintf("NewOSFileSystem.rootPath = %q", cleaned))
	fs := &OSFileSystem{rootPath: cleaned}
	fs.ArtifactFileSystem = NewArtifactFileSystem(fs)
	return fs
}

func (fs *OSFileSystem) relativePath(fpath string) string {
	normalizedPath := filepath.ToSlash(fpath)
	normalizedRoot := filepath.ToSlash(fs.rootPath)
	//logger.Log(LevelDebug, fmt.Sprintf("relativePath: normalize %q → %q, root %q", fpath, normalizedPath, normalizedRoot))
	if strings.HasPrefix(normalizedPath, normalizedRoot) {
		rel := normalizedPath[len(normalizedRoot):]
		// Убираем оба вида слэшей, чтобы не осталось ни "/" ни "\"
		rel = strings.TrimLeft(rel, "\\/")
		//logger.Log(LevelDebug, fmt.Sprintf("relativePath: trimmed → %q", rel))
		return rel
	}
	//logger.Log(LevelDebug, fmt.Sprintf("relativePath: no trim, return %q", normalizedPath))
	return normalizedPath
}

func (fs *OSFileSystem) parse(pattern string) []GeneratorFunc {
	parts := strings.Split(pattern, "/")
	if len(parts) > 0 && strings.HasPrefix(parts[0], "$") {
		return []GeneratorFunc{func(src <-chan *PathObject) <-chan *PathObject {
			out := make(chan *PathObject, 1)
			<-src
			full := filepath.Join(fs.rootPath, filepath.FromSlash(pattern))
			//logger.Log(LevelInfo, fmt.Sprintf("parse: NTFS‑route fullPath = %q", full))
			ntfsFS, err := NewNTFSFileSystem(
				fmt.Sprintf("\\\\.\\%s", filepath.VolumeName(fs.rootPath)))
			if err != nil {
				//	logger.Log(LevelError, fmt.Sprintf("parse: NewNTFSFileSystem error: %v", err))
				close(out)
				return out
			}
			po := &PathObject{filesystem: ntfsFS, name: filepath.Base(full), path: full}
			//logger.Log(LevelInfo, fmt.Sprintf("parse: emitting PathObject {%q, %q}", po.name, po.path))
			out <- po
			close(out)
			return out
		}}
	}

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

// OSFileSystem.ListDirectory
func (fsys *OSFileSystem) ListDirectory(p *PathObject) []*PathObject {
	// 1) Пробуем стандартный способ
	entries, err := os.ReadDir(p.path)
	if err == nil {
		var objs []*PathObject
		for _, e := range entries {
			name := e.Name()
			full := filepath.Join(p.path, name)
			if strings.HasPrefix(name, "~$") {
				logger.Log(LevelDebug, fmt.Sprintf("Skipping temp file: %s", full))
				continue
			}
			objs = append(objs, &PathObject{filesystem: fsys, name: name, path: full})
		}
		return objs
	}

	// 2) Логируем ошибку и падаем на NTFS-FS
	// Лог ошибки os.ReadDir…
	vol := filepath.VolumeName(p.path)
	if vol == "" {
		return nil
	}
	device := `\\.\` + strings.TrimSuffix(vol, `\`)
	ntfsFS, nerr := NewNTFSFileSystem(device)
	if nerr != nil {
		logger.Log(LevelError, fmt.Sprintf("NTFSFS init failed: %v", nerr))
		return nil
	}

	rel := ntfsFS.relativePath(p.path)
	ntfsEntries, rerr := fs.ReadDir(ntfsFS.fs, rel)
	if rerr != nil {
		logger.Log(LevelError, fmt.Sprintf("NTFSFS ReadDir error on %q: %v", rel, rerr))
		return nil
	}
	var objs []*PathObject
	for _, info := range ntfsEntries {
		full := filepath.Join(p.path, info.Name())
		objs = append(objs, &PathObject{filesystem: ntfsFS, name: info.Name(), path: full})
	}
	return objs
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
	//logger.Log(LevelInfo, fmt.Sprintf("OSFS.ReadChunks start for %q", p.path))
	// hive‑файлы реестра определяются по имени
	if strings.HasPrefix(p.name, "$") || isRegistryFile(p.name) {
		//	logger.Log(LevelInfo, fmt.Sprintf(	"OSFS.ReadChunks: protected → use NTFSFS for %q", p.path))
		ntfsFS, err := NewNTFSFileSystem(
			fmt.Sprintf(`\\.\%s`, filepath.VolumeName(fs.rootPath))) // UNC‑доступ к raw‑томe :contentReference[oaicite:6]{index=6}
		if err != nil {
			//logger.Log(LevelError, fmt.Sprintf(	"OSFS.ReadChunks: NewNTFSFileSystem error: %v", err))
			return nil, err
		}
		return ntfsFS.ReadChunks(p)
	}
	if !fs.IsFile(p) {
		//logger.Log(LevelDebug, fmt.Sprintf("OSFS.ReadChunks: skipping non-file %q", p.path))
		return nil, nil
	}
	file, err := os.Open(p.path)
	if err != nil {
		//logger.Log(LevelError, fmt.Sprintf(	"OSFS.ReadChunks: os.Open error: %v", err))
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
		if err == io.EOF {
			break
		}
		if err != nil {
			//logger.Log(LevelError, fmt.Sprintf(	"OSFS.ReadChunks: read error: %v", err))
			return nil, err
		}
	}
	//logger.Log(LevelInfo, fmt.Sprintf("OSFS.ReadChunks: read %d chunks for %q", len(chunks), p.path))
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
func (fsm *FileSystemManager) getFilesystem(path string) (FileSystem, error) {
	// Normalize to forward slashes
	p := filepath.ToSlash(path)
	// Skip Unix-style absolute patterns on Windows
	if runtime.GOOS == "windows" && strings.HasPrefix(p, "/") {
		//logger.Log(LevelDebug, fmt.Sprintf("Skipping Unix-style pattern on Windows: %s", p))
		return nil, nil
	}
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
			// Используем чисто Go‑NTFS для защищённых потоков
			volPath := fmt.Sprintf("\\\\.\\%s", volume) // UNC‑путь к томe :contentReference[oaicite:6]{index=6}
			ntfsFS, err := NewNTFSFileSystem(volPath)
			if err != nil {
				logger.Log(LevelError, fmt.Sprintf("NTFSFS error for %s: %v", volPath, err))
			} else {
				logger.Log(LevelInfo, fmt.Sprintf("NTFSFS selected for protected path %s", resolvedPath))
				chosenFS = ntfsFS
			}
		}
	}

	// По умолчанию — OSFileSystem
	if chosenFS == nil {
		osfs := NewOSFileSystem(volume + string(filepath.Separator))
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

// NTFSFileSystem реализует FileSystem через forensicanalysis/fslib/ntfs
type NTFSFileSystem struct {
	volHandle *os.File
	fs        *ntfsfs.FS
	*ArtifactFileSystem
}

// NewNTFSFileSystem открывает том (например "\\\\.\\C:") и парсит его как NTFS FS
func NewNTFSFileSystem(volumePath string) (*NTFSFileSystem, error) {
	// Открываем raw‑том для чтения (CreateFileA под капотом) :contentReference[oaicite:1]{index=1}
	vol, err := os.OpenFile(volumePath, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open volume %s: %w", volumePath, err)
	}
	// Парсим NTFS
	fsys, err := ntfsfs.New(vol)
	if err != nil {
		vol.Close()
		return nil, fmt.Errorf("ntfsfs.New: %w", err)
	}
	nts := &NTFSFileSystem{volHandle: vol, fs: fsys}
	nts.ArtifactFileSystem = NewArtifactFileSystem(nts)
	return nts, nil
}

// baseGenerator даёт стартовый PathObject (игнорируется, нужен для цепочки генераторов)
func (nts *NTFSFileSystem) baseGenerator() <-chan *PathObject {
	ch := make(chan *PathObject, 1)
	ch <- &PathObject{filesystem: nts, name: "", path: ""}
	close(ch)
	return ch
}

// parse всегда выдаёт ровно один компонент — наш защищённый файл
func (nts *NTFSFileSystem) parse(pattern string) []GeneratorFunc {
	rel := nts.relativePath(pattern)
	return []GeneratorFunc{
		func(source <-chan *PathObject) <-chan *PathObject {
			out := make(chan *PathObject, 1)
			<-source
			out <- &PathObject{filesystem: nts, name: filepath.Base(rel), path: rel}
			close(out)
			return out
		},
	}
}

func (nts *NTFSFileSystem) ListDirectory(p *PathObject) []*PathObject {
	// Перечисляем через io/fs ReadDir: поддержка любых директорий NTFS :contentReference[oaicite:2]{index=2}
	entries, err := fs.ReadDir(nts.fs, p.path)
	if err != nil {
		return nil
	}
	var res []*PathObject
	for _, e := range entries {
		res = append(res, &PathObject{
			filesystem: nts,
			name:       e.Name(),
			path:       filepath.Join(p.path, e.Name()),
		})
	}
	return res
}

func (nts *NTFSFileSystem) IsDirectory(p *PathObject) bool {
	info, err := nts.fs.Open(p.path)
	if err != nil {
		return false
	}
	st, err := info.Stat()
	return err == nil && st.IsDir()
}

func (nts *NTFSFileSystem) IsFile(p *PathObject) bool {
	info, err := nts.fs.Open(p.path)
	if err != nil {
		return false
	}
	st, err := info.Stat()
	return err == nil && !st.IsDir()
}

func (nts *NTFSFileSystem) IsSymlink(p *PathObject) bool {
	// NTFS потоки не дают symlink‑ов в этой логике
	return false
}

// relativePath убирает префикс тома и все ведущие слэши, затем делает ToSlash
func (nts *NTFSFileSystem) relativePath(p string) string {
	// Убираем "C:" или "\\host\share"
	noVol := strings.TrimPrefix(p, filepath.VolumeName(p)) // :contentReference[oaicite:2]{index=2}
	// Убираем любые ведущие '\' или '/'
	trimmed := strings.TrimLeft(noVol, `\/`)
	// Конвертируем '\' → '/'
	rel := filepath.ToSlash(trimmed) // :contentReference[oaicite:3]{index=3}
	//logger.Log(LevelDebug, fmt.Sprintf("NTFSFS.relativePath: %q → %q", p, rel))
	return rel
}

func (nts *NTFSFileSystem) ReadChunks(p *PathObject) ([][]byte, error) {
	logger.Log(LevelInfo, fmt.Sprintf("NTFSFS.ReadChunks start for %q", p.path))
	// 1) Получаем относительный путь внутри NTFS
	rel := nts.relativePath(p.path) // "$MFT" или "Windows/System32/..." :contentReference[oaicite:4]{index=4}
	// 2) Гарантируем, что путь в формате FS‑пути
	fsPath := rel
	// Альтернатива: использовать ToFSPath напрямую:
	// fsPath, err := fslib.ToFSPath(p.path)
	// if err != nil { … }
	//logger.Log(LevelDebug, fmt.Sprintf("NTFSFS.ReadChunks: opening fsPath %q", fsPath))
	// 3) Открываем через ntfsfs
	file, err := nts.fs.Open(fsPath) // корректно "Windows/System32/config/SAM" :contentReference[oaicite:5]{index=5}
	if err != nil {
		//logger.Log(LevelError, fmt.Sprintf(	"NTFSFS.ReadChunks: ntfsfs.Open(%q) error: %v", fsPath, err))
		return nil, err
	}
	defer file.Close()
	// 4) Читаем чанками
	var chunks [][]byte
	buf := make([]byte, CHUNK_SIZE)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			chunks = append(chunks, chunk)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Log(LevelError, fmt.Sprintf("NTFSFS.ReadChunks: read error: %v", err))
			return nil, err
		}
	}
	//logger.Log(LevelInfo, fmt.Sprintf("NTFSFS.ReadChunks: read %d chunks for %q", len(chunks), p.path))
	return chunks, nil
}

func (nts *NTFSFileSystem) GetSize(p *PathObject) int64 {
	file, err := nts.fs.Open(p.path)
	if err != nil {
		return 0
	}
	st, err := file.Stat()
	if err != nil {
		return 0
	}
	return st.Size()
}

// GetPath возвращает вложенный объект внутри NTFSFileSystem.
// Это нужно для навигации по каталогам.
func (nts *NTFSFileSystem) GetPath(parent *PathObject, name string) *PathObject {
	// строим новый PathObject с тем же FS и корректным относительным путём
	childPath := filepath.Join(parent.path, name)
	return &PathObject{
		filesystem: nts,
		name:       name,
		path:       childPath,
	}
}

// GetFullPath создаёт PathObject по абсолютному или относительному пути внутри тома NTFS.
// Интерфейс требует именно такую сигнатуру.
func (nts *NTFSFileSystem) GetFullPath(fullpath string) *PathObject {
	// приводим системный путь к виду, который понимает ntfsfs (отбрасываем имя тома)
	rel := nts.relativePath(fullpath)
	return &PathObject{
		filesystem: nts,
		name:       filepath.Base(rel),
		path:       rel,
	}
}
