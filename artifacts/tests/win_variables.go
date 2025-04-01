package main

import (
	"os"
	"os/user"
)

// Win32Account – структура для WMI-запроса
type Win32Account struct {
	Name string
	SID  string
}

func windowsInitFunc(hv *HostVariables) {
	// Основные переменные
	hv.AddVariable("%%users.appdata%%", os.Getenv("APPDATA"))
	hv.AddVariable("%%users.localappdata%%", os.Getenv("LOCALAPPDATA"))
	hv.AddVariable("%%users.homedir%%", os.Getenv("USERPROFILE"))
	hv.AddVariable("%%environ_systemroot%%", os.Getenv("SystemRoot"))
	hv.AddVariable("%%environ_systemroot%%", os.Getenv("SYSTEMROOT"))
	hv.AddVariable("%%environ_allusersprofile%%", os.Getenv("ALLUSERSPROFILE"))
	hv.AddVariable("%%users.temp%%", os.Getenv("TEMP"))
	hv.AddVariable("%%environ_programdata%%", os.Getenv("ProgramData"))
	hv.AddVariable("%%users.userprofile%%", os.Getenv("USERPROFILE"))

	// Системные пути
	hv.AddVariable("%%environ_systemdrive%%", os.Getenv("SystemDrive"))           // Пример: C:
	hv.AddVariable("%%environ_allusersappdata%%", os.Getenv("ALLUSERSPROFILE"))   // Пример: C:\ProgramData
	hv.AddVariable("%%environ_windir%%", os.Getenv("SystemRoot"))                 // Пример: C:\Windows
	hv.AddVariable("%%environ_programfiles%%", os.Getenv("ProgramFiles"))         // Пример: C:\Program Files
	hv.AddVariable("%%environ_programfilesx86%%", os.Getenv("ProgramFiles(x86)")) // Пример: C:\Program Files (x86)

	// Дополнительные переменные
	hv.AddVariable("%%public%%", os.Getenv("PUBLIC"))
	hv.AddVariable("%%comspec%%", os.Getenv("ComSpec"))

	// Информация о пользователе
	if user, err := user.Current(); err == nil {
		hv.AddVariable("%%users.sid%%", user.Uid)
		hv.AddVariable("%%users.username%%", user.Username)
	}
}

// NewWindowsHostVariables создаёт и инициализирует HostVariables для Windows,
// используя функцию windowsInitFunc для загрузки переменных из реестра и WMI.
func NewWindowsHostVariables() *HostVariables {
	return NewHostVariables(windowsInitFunc)
}
