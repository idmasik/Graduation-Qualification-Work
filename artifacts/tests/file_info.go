// file_info.go
package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"time"
)

// MAX_PE_SIZE is the maximum size (50 MB) we will buffer to parse PE internals.
const MAX_PE_SIZE = 50 * 1024 * 1024

// FilePathObject abstracts a file on disk (size, path, chunked read).
type FilePathObject interface {
	GetSize() int64
	GetPath() string
	ReadChunks() ([][]byte, error)
}

// FileInfo collects and computes metadata for one file.
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

// NewFileInfo constructs a FileInfo for the given path object.
func NewFileInfo(po FilePathObject) *FileInfo {
	return &FileInfo{
		po:   po,
		size: po.GetSize(),
		info: make(map[string]interface{}),
	}
}

// Compute drives the entire process and returns a JSON-serializable map.
func (f *FileInfo) Compute() map[string]interface{} {
	logger.Log(LevelDebug, fmt.Sprintf("Starting Compute() for %s", f.po.GetPath()))

	// Initialize hash.Hash objects
	f.md5Hash = md5.New()
	f.sha1Hash = sha1.New()
	f.sha256Hash = sha256.New()

	chunks, err := f.po.ReadChunks()
	if err != nil {
		logger.Log(LevelError, fmt.Sprintf("ReadChunks error: %v", err))
		return nil
	}

	// Process each chunk: feed hashes, detect MIME on first chunk, buffer PE content if needed
	for i, chunk := range chunks {
		f.md5Hash.Write(chunk)
		f.sha1Hash.Write(chunk)
		f.sha256Hash.Write(chunk)

		if i == 0 {
			f.mimeType = http.DetectContentType(chunk)
			logger.Log(LevelDebug, fmt.Sprintf("Detected MIME type: %s", f.mimeType))
		}
	}

	// If it’s a PE (“MZ” header) and under MAX_PE_SIZE, buffer for deeper parsing
	if len(chunks) > 0 && len(chunks[0]) >= 2 && chunks[0][0] == 'M' && chunks[0][1] == 'Z' {
		f.mimeType = "application/x-msdownload"
		logger.Log(LevelDebug, "PE signature detected, buffering content for PE parsing")
		if f.size < MAX_PE_SIZE {
			f.content = bytes.Join(chunks, nil)
		} else {
			logger.Log(LevelWarning, "PE file exceeds buffer threshold, skipping PE parse")
		}
	}

	return f.buildResult()
}

// buildResult assembles the final map with timestamps, hashes, MIME, and PE info if present.
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

	// If we buffered PE content, parse it
	if len(f.content) > 0 {
		if err := f.parsePE(); err != nil {
			logger.Log(LevelWarning, fmt.Sprintf("PE parse error: %v", err))
		}
	}

	return f.info
}

// parsePE uses debug/pe to extract the compilation timestamp.
func (f *FileInfo) parsePE() error {
	logger.Log(LevelDebug, "Parsing PE headers with debug/pe")
	r := bytes.NewReader(f.content)
	pf, err := pe.NewFile(r)
	if err != nil {
		return fmt.Errorf("debug/pe NewFile: %w", err)
	}
	defer pf.Close()

	ts := time.Unix(int64(pf.FileHeader.TimeDateStamp), 0).UTC()
	logger.Log(LevelInfo, fmt.Sprintf("PE compilation timestamp: %s", ts.Format(time.RFC3339)))
	f.addProp("pe", "compilation", ts.Format(time.RFC3339))
	return nil
}

// addProp adds a property under file → category → field.
func (f *FileInfo) addProp(category, field string, val interface{}) {
	fileMap := f.info["file"].(map[string]interface{})
	catMap, ok := fileMap[category].(map[string]interface{})
	if !ok {
		catMap = make(map[string]interface{})
		fileMap[category] = catMap
	}
	catMap[field] = val
}
