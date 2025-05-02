package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"syscall"
	"time"
	"unsafe"

	"debug/pe"

	pefile "github.com/Codehardt/go-pefile" // библиотека для правильного ImpHash
	"golang.org/x/sys/windows"
)

const MAX_PE_SIZE = 50 * 1024 * 1024

var (
	versionDLL                  = windows.NewLazySystemDLL("version.dll")
	procGetFileVersionInfoSizeW = versionDLL.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfoW     = versionDLL.NewProc("GetFileVersionInfoW")
	procVerQueryValueW          = versionDLL.NewProc("VerQueryValueW")
)

// FilePathObject абстрагирует файл на диске
// ... (интерфейс без изменений) ...
type FilePathObject interface {
	GetSize() int64
	GetPath() string
	ReadChunks() ([][]byte, error)
}

// FileInfo собирает метаданные одного файла
// ... (структура без изменений) ...
type FileInfo struct {
	po         FilePathObject
	size       int64
	info       map[string]interface{}
	content    []byte
	md5Hash    hash.Hash
	sha1Hash   hash.Hash
	sha256Hash hash.Hash
	mimeType   string
}

func NewFileInfo(po FilePathObject) *FileInfo {
	return &FileInfo{
		po:      po,
		size:    po.GetSize(),
		info:    make(map[string]interface{}),
		content: nil,
	}
}

func (f *FileInfo) Compute() map[string]interface{} {
	logger.Log(LevelDebug, "Starting Compute() for "+f.po.GetPath())

	f.md5Hash = md5.New()
	f.sha1Hash = sha1.New()
	f.sha256Hash = sha256.New()

	chunks, err := f.po.ReadChunks()
	if err != nil {
		logger.Log(LevelError, "ReadChunks error: "+err.Error())
		return nil
	}

	for i, c := range chunks {
		f.md5Hash.Write(c)
		f.sha1Hash.Write(c)
		f.sha256Hash.Write(c)
		if i == 0 {
			f.mimeType = http.DetectContentType(c)
			logger.Log(LevelDebug, "Detected MIME type: "+f.mimeType)
		}
	}

	// Если MZ и размер подходит — буферизуем для PE
	if len(chunks) > 0 && len(chunks[0]) >= 2 && chunks[0][0] == 'M' && chunks[0][1] == 'Z' {
		logger.Log(LevelDebug, "PE signature detected, buffering content for PE parsing")
		f.mimeType = "application/x-msdownload"
		if f.size < MAX_PE_SIZE {
			f.content = bytes.Join(chunks, nil)
		}
	}

	return f.buildResult()
}

func (f *FileInfo) buildResult() map[string]interface{} {
	f.info["@timestamp"] = time.Now().UTC().Format(time.RFC3339)
	fileMap := map[string]interface{}{
		"size":      f.size,
		"path":      f.po.GetPath(),
		"mime_type": f.mimeType,
		"hash": map[string]string{
			"md5":    hex.EncodeToString(f.md5Hash.Sum(nil)),
			"sha1":   hex.EncodeToString(f.sha1Hash.Sum(nil)),
			"sha256": hex.EncodeToString(f.sha256Hash.Sum(nil)),
		},
	}
	f.info["file"] = fileMap

	if len(f.content) > 0 {
		if err := f.parsePE(); err != nil {
			logger.Log(LevelWarning, "PE parse error: "+err.Error())
		}
	}

	return f.info
}

func (f *FileInfo) parsePE() error {
	logger.Log(LevelDebug, "Parsing PE headers with debug/pe")
	r := bytes.NewReader(f.content)
	pf, err := pe.NewFile(r)
	if err != nil {
		return fmt.Errorf("debug/pe NewFile: %w", err)
	}
	defer pf.Close()

	// compilation timestamp
	ts := time.Unix(int64(pf.FileHeader.TimeDateStamp), 0).UTC().Format("2006-01-02T15:04:05")
	logger.Log(LevelInfo, "PE compilation timestamp: "+ts)
	f.addProp("pe", "compilation", ts)

	// правильный ImpHash через github.com/Codehardt/go-pefile
	logger.Log(LevelDebug, "Computing ImpHash via go-pefile GetImpHash()")
	pf2, err := pefile.NewPEFile(f.po.GetPath())
	if err != nil {
		logger.Log(LevelWarning, fmt.Sprintf("go-pefile NewPEFile failed: %v", err))
	} else {
		defer pf2.Close()
		imp := pf2.GetImpHash()
		logger.Log(LevelInfo, "PE imphash: "+imp)
		f.addProp("pe", "imphash", imp)
	}

	// version resource через Windows API
	verInfo, err := readVersionInfo(f.po.GetPath())
	if err != nil {
		logger.Log(LevelWarning, "VersionInfo failed: "+err.Error())
	} else {
		for k, v := range verInfo {
			f.addProp("pe", k, v)
		}
	}

	return nil
}

func (f *FileInfo) addProp(cat, field string, val interface{}) {
	fileMap := f.info["file"].(map[string]interface{})
	sub, ok := fileMap[cat].(map[string]interface{})
	if !ok {
		sub = make(map[string]interface{})
		fileMap[cat] = sub
	}
	sub[field] = val
}

// readVersionInfo читает StringFileInfo через Windows API
func readVersionInfo(path string) (map[string]string, error) {
	ret := map[string]string{}
	path16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	// размер
	var dummy uint32
	size, _, _ := procGetFileVersionInfoSizeW.Call(
		uintptr(unsafe.Pointer(path16)),
		uintptr(unsafe.Pointer(&dummy)),
	)
	if size == 0 {
		return nil, fmt.Errorf("GetFileVersionInfoSizeW == 0")
	}
	buf := make([]byte, size)
	r1, _, e1 := procGetFileVersionInfoW.Call(
		uintptr(unsafe.Pointer(path16)),
		0,
		size,
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if r1 == 0 {
		return nil, fmt.Errorf("GetFileVersionInfoW failed: %v", e1)
	}
	// translation
	var block *uint16
	var blockLen uint32
	transPath, _ := syscall.UTF16PtrFromString(`\VarFileInfo\Translation`)
	procVerQueryValueW.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(transPath)),
		uintptr(unsafe.Pointer(&block)),
		uintptr(unsafe.Pointer(&blockLen)),
	)
	if blockLen < 4 {
		return ret, nil
	}
	lang := binary.LittleEndian.Uint16((*[2]byte)(unsafe.Pointer(block))[:])
	cp := binary.LittleEndian.Uint16((*[2]byte)(unsafe.Pointer(uintptr(unsafe.Pointer(block)) + 2))[:])
	keys := map[string]string{
		"CompanyName":     "company",
		"FileDescription": "description",
		"FileVersion":     "file_version",
		"InternalName":    "original_file_name",
		"ProductName":     "product",
	}
	for winKey, ourKey := range keys {
		query := fmt.Sprintf(`\StringFileInfo\%04x%04x\%s`, lang, cp, winKey)
		query16, _ := syscall.UTF16PtrFromString(query)
		var valPtr uintptr
		var valLen uint32
		procVerQueryValueW.Call(
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(query16)),
			uintptr(unsafe.Pointer(&valPtr)),
			uintptr(unsafe.Pointer(&valLen)),
		)
		if valLen > 0 {
			str := syscall.UTF16ToString((*[1 << 16]uint16)(unsafe.Pointer(valPtr))[:valLen])
			ret[ourKey] = str
			logger.Log(LevelDebug, fmt.Sprintf("VersionInfo %s=%s", ourKey, str))
		}
	}
	return ret, nil
}
