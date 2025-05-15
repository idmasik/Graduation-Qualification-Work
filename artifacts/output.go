package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// parseHumanSize переводит строковое представление размера (например, "5M")
// в число байт.
func parseHumanSize(size string) (int64, error) {
	if size == "" {
		return 0, nil
	}

	units := map[byte]int64{
		'B': 1,
		'K': 1024,
		'M': 1024 * 1024,
		'G': 1024 * 1024 * 1024,
	}
	last := size[len(size)-1]
	if multiplier, ok := units[last]; ok {
		num, err := strconv.ParseInt(size[:len(size)-1], 10, 64)
		if err != nil {
			return 0, err
		}
		return num * multiplier, nil
	}
	return strconv.ParseInt(size, 10, 64)
}

// normalizeFilepath нормализует путь к файлу.
// Если найден разделитель пути (например, на Windows), удаляет двоеточие после буквы диска.
func normalizeFilepath(pathStr string) string {
	if idx := strings.Index(pathStr, string(os.PathSeparator)); idx > 0 {
		return strings.Replace(pathStr, ":", "", 1)
	}
	return pathStr
}

// Outputs собирает результаты артефактов, такие как файлы, команды, WMI и реестр.
type Outputs struct {
	dirpath    string
	hostname   string
	zipFile    *os.File
	zipWriter  *zip.Writer
	addedFiles map[string]bool

	maxsize int64
	sha256  bool

	commands map[string]map[string]string
	wmi      map[string]map[string]json.RawMessage
	registry map[string]map[string]map[string]interface{}

	fileInfoFile *os.File
	logFile      *os.File

	analysis      bool
	apiKey        string
	analysisQueue *AnalysisQueue
}

// NewOutputs создаёт новый экземпляр Outputs.
// dirpath – путь к каталогу для результатов,
// maxsizeStr – максимально допустимый размер файла (например, "50M"),
// sha256 – вычислять ли SHA-256 для собираемых файлов.
func NewOutputs(dirpath, maxsizeStr string, sha256 bool, analysis bool, apiKey string) (*Outputs, error) {
	maxsize, err := parseHumanSize(maxsizeStr)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format("20060102150405")
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	finalDir := filepath.Join(dirpath, fmt.Sprintf("%s-%s", now, hostname))
	// Создаём каталог с правами 0700.
	if err := os.MkdirAll(finalDir, 0700); err != nil {
		return nil, err
	}
	// Устанавливаем переменную окружения для COMMAND артефактов.
	os.Setenv("FAOUTPUTDIR", finalDir)

	var aq *AnalysisQueue
	if analysis && apiKey != "" {
		client, err := NewClient(apiKey)
		if err != nil {
			return nil, err
		}
		resultsPath := filepath.Join(finalDir, fmt.Sprintf("%s-analyse.jsonl", hostname))
		aq, err = NewQueue(client, 100, 5, resultsPath)
		if err != nil {
			return nil, err
		}
	}

	o := &Outputs{
		dirpath:       finalDir,
		hostname:      hostname,
		maxsize:       maxsize,
		sha256:        sha256,
		addedFiles:    make(map[string]bool),
		commands:      make(map[string]map[string]string),
		wmi:           make(map[string]map[string]json.RawMessage),
		registry:      make(map[string]map[string]map[string]interface{}),
		analysis:      analysis,
		apiKey:        apiKey,
		analysisQueue: aq,
	}

	if err := o.setupLogging(); err != nil {
		return nil, err
	}
	return o, nil
}

// setupLogging настраивает логирование в файл и на консоль.
func (o *Outputs) setupLogging() error {
	logfile := filepath.Join(o.dirpath, fmt.Sprintf("%s-logs.txt", o.hostname))
	f, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	o.logFile = f
	// Логгер из logging.go используется для вывода как на консоль, так и в файл.
	multi := io.MultiWriter(os.Stdout, f)
	logger.SetOutput(multi)
	return nil
}

// AddCollectedFileInfo собирает информацию о файле для указанного артефакта.
// FileInfo берётся из модуля file_info.go.
func (o *Outputs) AddCollectedFileInfo(artifact string, pathObject FilePathObject) error {
	fi := NewFileInfo(pathObject)
	if o.maxsize == 0 || pathObject.GetSize() <= o.maxsize {
		if o.fileInfoFile == nil {
			fileInfoPath := filepath.Join(o.dirpath, fmt.Sprintf("%s-file_info.jsonl", o.hostname))
			f, err := os.OpenFile(fileInfoPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return err
			}
			o.fileInfoFile = f
		}
		fileInfo := fi.Compute()
		if fileInfo == nil {
			return fmt.Errorf("failed to compute file info")
		}
		fileInfo["labels"] = map[string]string{"artifact": artifact}

		b, err := json.Marshal(fileInfo)
		if err != nil {
			return err
		}
		if _, err := o.fileInfoFile.Write(append(b, '\n')); err != nil {
			return err
		}

		// Фильтрация на анализ
		if o.analysisQueue != nil {
			// MIME-тип
			mt, _ := fileInfo["file"].(map[string]interface{})["mime_type"].(string)
			// Расширение файла
			path := fileInfo["file"].(map[string]interface{})["path"].(string)

			ext := strings.ToLower(filepath.Ext(path))

			// Логируем их для отладки
			logger.Log(LevelDebug, fmt.Sprintf("Analyzing candidate: path=%s, mime=%s, ext=%s", path, mt, ext))

			if mt == "application/x-msdownload" || mt == "application/vnd.microsoft.portable-executable" ||
				ext == ".exe" || ext == ".dll" || ext == ".sys" || ext == ".bin" || ext == ".sh" {
				o.analysisQueue.Enqueue(fileInfo)
			}
		}
	}
	return nil
}

// AddCollectedFile собирает содержимое файла для указанного артефакта.
// Если файл не превышает максимально допустимый размер, он добавляется в zip-архив.
func (o *Outputs) AddCollectedFile(artifact string, pathObject FilePathObject) error {
	filePath := pathObject.GetPath()

	// Проверка существования файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logger.Log(LevelWarning, fmt.Sprintf("File not found: %s", filePath))
		return nil
	}

	// Проверка размера
	if o.maxsize > 0 && pathObject.GetSize() > o.maxsize {
		logger.Log(LevelWarning,
			fmt.Sprintf("Skipping large file: %s (%d bytes)",
				filePath, pathObject.GetSize()))
		return nil
	}

	// Создание архива при первом использовании
	if o.zipWriter == nil {
		zipPath := filepath.Join(o.dirpath, fmt.Sprintf("%s-files.zip", o.hostname))
		f, err := os.Create(zipPath)
		if err != nil {
			return fmt.Errorf("failed to create zip: %v", err)
		}
		o.zipFile = f
		o.zipWriter = zip.NewWriter(f)
	}

	// Нормализация пути
	filename := normalizeFilepath(filePath)
	if _, exists := o.addedFiles[filename]; exists {
		return nil
	}

	// Добавление в архив
	header := &zip.FileHeader{
		Name:   filename,
		Method: zip.Deflate,
	}
	writer, err := o.zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	// Чтение и запись содержимого
	chunks, err := pathObject.ReadChunks()
	if err != nil {
		return err
	}

	var hash256 hash.Hash
	if o.sha256 {
		hash256 = sha256.New()
	}

	for _, chunk := range chunks {
		if _, err := writer.Write(chunk); err != nil {
			return err
		}
		if o.sha256 {
			hash256.Write(chunk)
		}
	}

	o.addedFiles[filename] = true
	logger.Log(LevelInfo,
		fmt.Sprintf("Added %s (%d bytes) to archive",
			filename, pathObject.GetSize()))

	return nil
}

// AddCollectedCommand собирает результат выполнения команды для указанного артефакта.
func (o *Outputs) AddCollectedCommand(artifact, command string, output []byte) {
	logger.Log(LevelInfo, fmt.Sprintf("Collecting command '%s' for artifact '%s'", command, artifact))
	if o.commands[artifact] == nil {
		o.commands[artifact] = make(map[string]string)
	}
	o.commands[artifact][command] = string(output)
}

// AddCollectedWMI собирает результат WMI-запроса для указанного артефакта.
func (o *Outputs) AddCollectedWMI(artifact, query string, output json.RawMessage) {
	logger.Log(LevelInfo, fmt.Sprintf("Collecting WMI query '%s' for artifact '%s'", query, artifact))
	if o.wmi[artifact] == nil {
		o.wmi[artifact] = make(map[string]json.RawMessage)
	}
	o.wmi[artifact][query] = output
}

// AddCollectedRegistryValue собирает значение реестра для указанного артефакта.
func (o *Outputs) AddCollectedRegistryValue(artifact, key, name string, value interface{}, type_ string) {
	logger.Log(LevelInfo, fmt.Sprintf("Collecting Reg value '%s' from '%s' for artifact '%s'", name, key, artifact))
	if o.registry[artifact] == nil {
		o.registry[artifact] = make(map[string]map[string]interface{})
	}
	if o.registry[artifact][key] == nil {
		o.registry[artifact][key] = make(map[string]interface{})
	}
	o.registry[artifact][key][name] = map[string]interface{}{
		"value": value,
		"type":  type_,
	}
}

// Close завершает работу Outputs: закрывает zip-архив, записывает файлы JSON и закрывает открытые дескрипторы.
func (o *Outputs) Close() error {
	var err error
	if o.zipWriter != nil {
		if e := o.zipWriter.Close(); e != nil {
			err = e
		}
		if e := o.zipFile.Close(); e != nil {
			err = e
		}
	}
	if len(o.commands) > 0 {
		path := filepath.Join(o.dirpath, fmt.Sprintf("%s-commands.json", o.hostname))
		f, e := os.Create(path)
		if e != nil {
			err = e
		} else {
			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			if e := enc.Encode(o.commands); e != nil {
				err = e
			}
			f.Close()
		}
	}
	if len(o.wmi) > 0 {
		path := filepath.Join(o.dirpath, fmt.Sprintf("%s-wmi.json", o.hostname))
		f, e := os.Create(path)
		if e != nil {
			err = e
		} else {
			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			if e := enc.Encode(o.wmi); e != nil {
				err = e
			}
			f.Close()
		}
	}
	if len(o.registry) > 0 {
		path := filepath.Join(o.dirpath, fmt.Sprintf("%s-registry.json", o.hostname))
		f, e := os.Create(path)
		if e != nil {
			err = e
		} else {
			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			if e := enc.Encode(o.registry); e != nil {
				err = e
			}
			f.Close()
		}
	}
	if o.fileInfoFile != nil {
		if e := o.fileInfoFile.Close(); e != nil {
			err = e
		}
	}
	if o.logFile != nil {
		if e := o.logFile.Close(); e != nil {
			err = e
		}
	}
	return err
}
