package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"time"
	"unicode/utf16"
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

// Compute вычисляет хэши, определяет MIME-тип и, если это PE-файл,
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

	for i, chunk := range chunks {
		f.md5Hash.Write(chunk)
		f.sha1Hash.Write(chunk)
		f.sha256Hash.Write(chunk)

		if i == 0 {
			// Используем http.DetectContentType для определения MIME-типа.
			guessedMime := http.DetectContentType(chunk)
			if guessedMime != "" {
				f.mimeType = guessedMime
			}
		}

		// Если MIME-тип равен "application/x-msdownload" и размер файла меньше MAX_PE_SIZE,
		// сохраняем содержимое для дальнейшего анализа PE.
		if f.mimeType == "application/x-msdownload" && f.size < MAX_PE_SIZE {
			f.content = append(f.content, chunk...)
		}
	}

	return f.getResults()
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

// addVSInfo пытается извлечь информацию из VS_VERSIONINFO.
// Здесь реализован простой парсер, который ищет в ресурсе строки из блока StringFileInfo.
func (f *FileInfo) addVSInfo(peFile *pe.File) {
	data, err := extractVersionInfo(peFile)
	if err != nil {
		logger.Log(LevelWarning, fmt.Sprintf("Error extracting version info: %v", err))
		return
	}
	root, _, err := parseVersionBlock(data, 0)
	if err != nil {
		logger.Log(LevelWarning, fmt.Sprintf("Error parsing version info: %v", err))
		return
	}
	props := extractStringProperties(root)
	vsInfoFields := map[string]string{
		"CompanyName":     "company",
		"FileDescription": "description",
		"FileVersion":     "file_version",
		"InternalName":    "original_file_name",
		"ProductName":     "product",
	}
	for key, field := range vsInfoFields {
		if val, ok := props[key]; ok && val != "" {
			f.addFileProperty("pe", field, val)
		}
	}
}

// addPEInfo анализирует PE-структуру и дополняет info.
func (f *FileInfo) addPEInfo() error {
	r := bytes.NewReader(f.content)
	peFile, err := pe.NewFile(r)
	if err != nil {
		return err
	}
	defer peFile.Close()

	f.addVSInfo(peFile)
	// Заглушка для imphash – стандартная библиотека не предоставляет его расчёт.
	f.addFileProperty("pe", "imphash", "<imphash>")
	compilationTime := time.Unix(int64(peFile.FileHeader.TimeDateStamp), 0).UTC().Format(time.RFC3339)
	f.addFileProperty("pe", "compilation", compilationTime)

	return nil
}

// --- Helper функции для парсинга VS_VERSIONINFO ---

// align выравнивает offset до заданного кратного значения.
func align(offset int, alignment int) int {
	if offset%alignment == 0 {
		return offset
	}
	return offset + (alignment - (offset % alignment))
}

// decodeUTF16LE декодирует байты в строку UTF-16LE.
func decodeUTF16LE(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u16s := make([]uint16, len(b)/2)
	for i := 0; i < len(u16s); i++ {
		u16s[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
	}
	// Удаляем завершающий нул, если есть
	if len(u16s) > 0 && u16s[len(u16s)-1] == 0 {
		u16s = u16s[:len(u16s)-1]
	}
	return string(utf16.Decode(u16s))
}

// encodeUTF16LE кодирует строку в UTF-16LE.
func encodeUTF16LE(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	buf := make([]byte, len(u16)*2)
	for i, v := range u16 {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	return buf
}

// versionBlock представляет собой структуру VERSIONINFO.
type versionBlock struct {
	Length      uint16
	ValueLength uint16
	Type        uint16
	Key         string
	Value       string
	Children    []versionBlock
}

// parseVersionBlock рекурсивно парсит блок VERSIONINFO, начиная с offset.
func parseVersionBlock(data []byte, offset int) (versionBlock, int, error) {
	if offset+6 > len(data) {
		return versionBlock{}, offset, fmt.Errorf("insufficient data for header")
	}
	length := binary.LittleEndian.Uint16(data[offset : offset+2])
	valueLength := binary.LittleEndian.Uint16(data[offset+2 : offset+4])
	vType := binary.LittleEndian.Uint16(data[offset+4 : offset+6])
	block := versionBlock{
		Length:      length,
		ValueLength: valueLength,
		Type:        vType,
	}
	startBlock := offset
	offset += 6

	// Читаем ключ (null-терминированная строка в UTF-16LE)
	keyStart := offset
	for {
		if offset+2 > len(data) {
			break
		}
		if data[offset] == 0 && data[offset+1] == 0 {
			break
		}
		offset += 2
	}
	if offset+2 > len(data) {
		return block, offset, fmt.Errorf("unexpected end while reading key")
	}
	keyBytes := data[keyStart:offset]
	block.Key = decodeUTF16LE(keyBytes)
	offset += 2 // пропускаем завершающий нул
	offset = align(offset, 4)

	// Читаем значение, если оно присутствует
	if valueLength > 0 {
		if block.Type == 1 { // текстовое значение
			valByteLen := int(valueLength) * 2
			if offset+valByteLen > len(data) {
				return block, offset, fmt.Errorf("insufficient data for value")
			}
			valueBytes := data[offset : offset+valByteLen]
			block.Value = decodeUTF16LE(valueBytes)
			offset += valByteLen
		} else {
			if offset+int(valueLength) > len(data) {
				return block, offset, fmt.Errorf("insufficient data for binary value")
			}
			offset += int(valueLength)
		}
		offset = align(offset, 4)
	}

	// Рекурсивно парсим дочерние блоки до конца текущего блока.
	endBlock := int(startBlock) + int(length)
	for offset < endBlock {
		if offset+2 > endBlock {
			break
		}
		child, newOffset, err := parseVersionBlock(data, offset)
		if err != nil {
			break
		}
		block.Children = append(block.Children, child)
		offset = newOffset
	}
	return block, int(startBlock) + int(length), nil
}

// extractVersionInfo пытается найти ресурс версии (VS_VERSIONINFO) в секции .rsrc.
func extractVersionInfo(peFile *pe.File) ([]byte, error) {
	for _, section := range peFile.Sections {
		if section.Name == ".rsrc" {
			data, err := section.Data()
			if err != nil {
				return nil, err
			}
			// Ищем метку "VS_VERSIONINFO" в виде UTF-16LE
			key := "VS_VERSIONINFO"
			keyBytes := encodeUTF16LE(key)
			idx := bytes.Index(data, keyBytes)
			if idx >= 0 {
				start := idx - 6 // предполагаем, что заголовок находится за 6 байт до найденного ключа
				if start < 0 {
					start = 0
				}
				if start+2 > len(data) {
					return nil, fmt.Errorf("not enough data")
				}
				length := int(binary.LittleEndian.Uint16(data[start : start+2]))
				if start+length > len(data) {
					return nil, fmt.Errorf("version info length out of bounds")
				}
				return data[start : start+length], nil
			}
		}
	}
	return nil, fmt.Errorf("version info not found")
}

// extractStringProperties обходит дерево блоков и извлекает свойства из StringFileInfo.
func extractStringProperties(root versionBlock) map[string]string {
	props := make(map[string]string)
	for _, child := range root.Children {
		if child.Key == "StringFileInfo" {
			for _, stringTable := range child.Children {
				for _, strBlock := range stringTable.Children {
					if strBlock.Key != "" && strBlock.Value != "" {
						props[strBlock.Key] = strBlock.Value
					}
				}
			}
		}
	}
	return props
}
