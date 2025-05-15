package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"time"

	"debug/pe"

	pefile "github.com/Codehardt/go-pefile"
)

const MAX_PE_SIZE = 50 * 1024 * 1024

// FilePathObject абстрагирует файл на диске
type FilePathObject interface {
	GetSize() int64
	GetPath() string
	ReadChunks() ([][]byte, error)
}

// FileInfo собирает метаданные одного файла
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
	f.md5Hash = md5.New()
	f.sha1Hash = sha1.New()
	f.sha256Hash = sha256.New()

	chunks, err := f.po.ReadChunks()
	if err != nil {
		//logger.Log(LevelError, "ReadChunks error: "+err.Error())
		return nil
	}

	for i, c := range chunks {
		f.md5Hash.Write(c)
		f.sha1Hash.Write(c)
		f.sha256Hash.Write(c)
		if i == 0 {
			f.mimeType = http.DetectContentType(c)
			//logger.Log(LevelDebug, "Detected MIME type: "+f.mimeType)
		}
	}

	// Если MZ и размер подходит — буферизуем для PE
	if len(chunks) > 0 && len(chunks[0]) >= 2 && chunks[0][0] == 'M' && chunks[0][1] == 'Z' {
		//logger.Log(LevelDebug, "PE signature detected, buffering content for PE parsing")
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
			logger.Log(LevelError, "PE parse error: "+err.Error())
		}
	}

	return f.info
}

func (f *FileInfo) parsePE() error {
	//logger.Log(LevelDebug, "Parsing PE headers with debug/pe")
	r := bytes.NewReader(f.content)
	pf, err := pe.NewFile(r)
	if err != nil {
		return fmt.Errorf("debug/pe NewFile: %w", err)
	}
	defer pf.Close()

	// compilation timestamp
	ts := time.Unix(int64(pf.FileHeader.TimeDateStamp), 0).UTC().Format("2006-01-02T15:04:05")
	//logger.Log(LevelInfo, "PE compilation timestamp: "+ts)
	f.addProp("pe", "compilation", ts)

	// ImpHash через github.com/Codehardt/go-pefile
	//logger.Log(LevelDebug, "Computing ImpHash via go-pefile GetImpHash()")
	pf2, err := pefile.NewPEFile(f.po.GetPath())
	if err != nil {
		logger.Log(LevelWarning, fmt.Sprintf("go-pefile NewPEFile failed: %v", err))
	} else {
		defer pf2.Close()
		imp := pf2.GetImpHash()
		//logger.Log(LevelInfo, "PE imphash: "+imp)
		f.addProp("pe", "imphash", imp)
	}

	// version resource через абстрактную функцию
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
