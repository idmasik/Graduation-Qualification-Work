//go:build windows
// +build windows

package main

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	versionDLL                  = windows.NewLazySystemDLL("version.dll")
	procGetFileVersionInfoSizeW = versionDLL.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfoW     = versionDLL.NewProc("GetFileVersionInfoW")
	procVerQueryValueW          = versionDLL.NewProc("VerQueryValueW")
)

// readVersionInfo читает StringFileInfo через Windows API
func readVersionInfo(path string) (map[string]string, error) {
	ret := map[string]string{}
	path16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	// размер буфера
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
	// получение Translation
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
		}
	}
	return ret, nil
}
