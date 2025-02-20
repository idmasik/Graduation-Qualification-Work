package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// MAX_PE_SIZE – 50 МБ
const MAX_PE_SIZE = 50 * 1024 * 1024

// FilePathObject – абстракция для объекта файла.
// Он должен предоставлять методы для получения размера, пути и чтения чанков.
type FilePathObject interface {
	GetSize() int64
	GetPath() string
	// ReadChunks возвращает слайс чанков (байтовых срезов)
	ReadChunks() ([][]byte, error)
}

// FileInfo собирает информацию о файле.
type FileInfo struct {
	pathObject FilePathObject
	size       int64
	info       map[string]interface{}
	content    []byte

	md5Hash    hash.Hash
	sha1Hash   hash.Hash
	sha256Hash hash.Hash
	mimeType   string
}

// NewFileInfo создаёт новый экземпляр FileInfo.
func NewFileInfo(po FilePathObject) *FileInfo {
	return &FileInfo{
		pathObject: po,
		size:       po.GetSize(),
		info:       make(map[string]interface{}),
		content:    []byte{},
	}
}

// getResults формирует результирующую карту с информацией о файле.
func (f *FileInfo) getResults() map[string]interface{} {
	f.info["@timestamp"] = time.Now().UTC().Format(time.RFC3339)
	f.info["file"] = map[string]interface{}{
		"size": f.size,
		"path": f.pathObject.GetPath(),
		"hash": map[string]string{
			"md5":    hex.EncodeToString(f.md5Hash.Sum(nil)),
			"sha1":   hex.EncodeToString(f.sha1Hash.Sum(nil)),
			"sha256": hex.EncodeToString(f.sha256Hash.Sum(nil)),
		},
	}

	if f.mimeType != "" {
		fileMap := f.info["file"].(map[string]interface{})
		fileMap["mime_type"] = f.mimeType
	}

	// Если у нас накоплено содержимое для анализа PE, пытаемся его распарсить.
	if len(f.content) > 0 {
		if err := f.addPEInfo(); err != nil {
			logger.Log(LevelWarning, fmt.Sprintf("Could not parse PE file '%s': '%v'", f.pathObject.GetPath(), err))
		}
	}

	return f.info
}

// addFileProperty добавляет свойство в категорию info["file"].
func (f *FileInfo) addFileProperty(category, field string, value interface{}) {
	fileMap := f.info["file"].(map[string]interface{})
	cat, exists := fileMap[category]
	if !exists {
		cat = make(map[string]interface{})
		fileMap[category] = cat
	}
	cat.(map[string]interface{})[field] = value
}

// Compute вычисляет хэши, определяет MIME‑тип и, если это PE‑файл,
// собирает часть содержимого для дальнейшего анализа.
func (f *FileInfo) Compute() map[string]interface{} {
	f.md5Hash = md5.New()
	f.sha1Hash = sha1.New()
	f.sha256Hash = sha256.New()
	f.mimeType = ""

	chunks, err := f.pathObject.ReadChunks()
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("Error reading chunks: %v", err))
		return nil
	}

	// Обрабатываем все чанки для расчёта хэшей и определения MIME‑типа.
	for i, chunk := range chunks {
		f.md5Hash.Write(chunk)
		f.sha1Hash.Write(chunk)
		f.sha256Hash.Write(chunk)

		if i == 0 {
			// Определяем MIME‑тип с помощью http.DetectContentType.
			guessedMime := http.DetectContentType(chunk)
			f.mimeType = guessedMime
		}
	}

	// Если файл начинается с сигнатуры "MZ", считаем его PE‑файлом.
	if len(chunks) > 0 && len(chunks[0]) >= 2 && chunks[0][0] == 'M' && chunks[0][1] == 'Z' {
		// Переопределяем MIME‑тип для PE‑файлов.
		f.mimeType = "application/x-msdownload"
		// Если размер файла меньше MAX_PE_SIZE, сохраняем содержимое для анализа.
		if f.size < MAX_PE_SIZE {
			for _, chunk := range chunks {
				f.content = append(f.content, chunk...)
			}
		}
	}

	return f.getResults()
}

// addPEInfo анализирует PE‑файл: вызывает Python‑скрипт для версии,
// вычисляет imphash и устанавливает время компиляции.
func (f *FileInfo) addPEInfo() error {
	// Можно открыть PE-файл, чтобы проверить, что он действительно PE.
	r := bytes.NewReader(f.content)
	peFile, err := pe.NewFile(r)
	if err != nil {
		return err
	}
	defer peFile.Close()

	// Вызываем Python‑скрипт для получения всех необходимых данных.
	if err := f.addPEInfoViaPython(); err != nil {
		logger.Log(LevelWarning, fmt.Sprintf("Error extracting PE info via Python: %v", err))
	}
	return nil
}

// addPEInfoViaPython вызывает внешний Python‑скрипт (например, parser.py) для извлечения всей PE‑информации.
func (f *FileInfo) addPEInfoViaPython() error {
	// Записываем содержимое во временный файл.
	tmpFile, err := ioutil.TempFile("", "peinfo_*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(f.content); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// Определяем имя команды для Python в зависимости от ОС.
	pythonCmd := "python3"
	if runtime.GOOS == "windows" {
		pythonCmd = "python"
	}

	// Определяем абсолютный путь до скрипта parser.py (он должен лежать в той же директории, что и исполняемый файл).
	//exePath, err := os.Executable()
	if err != nil {
		return err
	}
	scriptPath := filepath.Join("parser.py")

	// Вызываем Python‑скрипт.
	cmd := exec.Command(pythonCmd, scriptPath, tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("python script error: %s, output: %s", err, output)
	}

	var info map[string]string
	if err := json.Unmarshal(output, &info); err != nil {
		return err
	}
	if errMsg, ok := info["error"]; ok && errMsg != "" {
		return fmt.Errorf("python script error: %s", errMsg)
	}
	// Ожидаемые ключи: company, description, file_version, original_file_name, product, imphash, compilation.
	keys := []string{"company", "description", "file_version", "original_file_name", "product", "imphash", "compilation"}
	for _, key := range keys {
		if val, ok := info[key]; ok && val != "" {
			f.addFileProperty("pe", key, val)
		}
	}
	return nil
}

// addVSInfoWithPython вызывает внешний Python‑скрипт для извлечения версии.
func (f *FileInfo) addVSInfoWithPython() error {
	// Записываем содержимое во временный файл.
	tmpFile, err := ioutil.TempFile("", "peinfo_*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(f.content); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// Определяем имя команды для Python в зависимости от ОС.
	pythonCmd := "python3"
	if runtime.GOOS == "windows" {
		pythonCmd = "python"
	}

	// Определяем абсолютный путь до скрипта extract_pe_info.py (находится в той же директории, что и исполняемый файл).
	//exePath, err := os.Executable()
	if err != nil {
		return err
	}
	scriptPath := filepath.Join("parser.py")

	cmd := exec.Command(pythonCmd, scriptPath, tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("python script error: %s, output: %s", err, output)
	}

	if err != nil {
		return err
	}

	var info map[string]string
	if err := json.Unmarshal(output, &info); err != nil {
		return err
	}
	if errMsg, ok := info["error"]; ok && errMsg != "" {
		return fmt.Errorf("python script error: %s", errMsg)
	}
	// Добавляем извлечённые поля.
	for key, val := range info {
		switch key {
		case "company", "description", "file_version", "original_file_name", "product":
			f.addFileProperty("pe", key, val)
		}
	}
	return nil
}
