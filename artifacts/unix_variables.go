package main

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// initUnixHostVariables инициализирует переменные для Unix-систем.
// Добавлено логирование итоговых значений переменных.
func initUnixHostVariables(hv *HostVariables) {
	// Определяем HOME.
	home := os.Getenv("HOME")
	if home == "" {
		if usr, err := user.Current(); err == nil {
			home = usr.HomeDir
		} else {
			home = "/home/unknown"
		}
	}
	logger.Log(LevelDebug, fmt.Sprintf("UnixHostVariables: определён HOME = %s", home))

	// Основные переменные.
	hv.AddVariable("%%users.homedir%%", home)
	hv.AddVariable("%%users.userprofile%%", home) // Для совместимости.
	// Используем стандартные каталоги для локальных данных и конфигураций.
	localAppData := filepath.Join(home, ".local", "share")
	appData := filepath.Join(home, ".config")
	hv.AddVariable("%%users.localappdata%%", localAppData)
	hv.AddVariable("%%users.appdata%%", appData)

	// Системные пути, типичные для Linux.
	hv.AddVariable("%%environ_programdata%%", "/etc")
	hv.AddVariable("%%environ_systemdrive%%", "/")
	hv.AddVariable("%%environ_programfiles%%", "/usr/local")
	hv.AddVariable("%%environ_programfilesx86%%", "/usr/local")
	hv.AddVariable("%%environ_allusersappdata%%", "/etc")

	logger.Log(LevelDebug, fmt.Sprintf("UnixHostVariables: %%users.homedir%% = %s", home))
	logger.Log(LevelDebug, fmt.Sprintf("UnixHostVariables: %%users.localappdata%% = %s", localAppData))
	logger.Log(LevelDebug, fmt.Sprintf("UnixHostVariables: %%users.appdata%% = %s", appData))
	logger.Log(LevelDebug, "UnixHostVariables: системные переменные для Linux заданы")

	// Дополнительно: Добавляем все домашние каталоги из /etc/passwd, если они отличаются от основного.
	userprofiles := make(map[string]struct{})
	file, err := os.Open("/etc/passwd")
	if err != nil {
		hv.logger.Log(LevelWarning, fmt.Sprintf("UnixHostVariables: Не удалось открыть /etc/passwd: %v", err))
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ":")
		if len(fields) > 5 {
			dir := fields[5]
			if dir != "" {
				userprofiles[dir] = struct{}{}
			}
		}
	}
	for dir := range userprofiles {
		if !strings.EqualFold(dir, home) {
			hv.AddVariable("%%users.homedir%%", dir)
			// При необходимости можно добавить их и как %%users.localappdata%%.
			hv.AddVariable("%%users.localappdata%%", dir)
			logger.Log(LevelDebug, fmt.Sprintf("UnixHostVariables: Добавлен дополнительный домашний каталог: %s", dir))
		}
	}
	logger.Log(LevelInfo, "UnixHostVariables: Инициализация переменных завершена.")
}

// NewUnixHostVariables создаёт и возвращает HostVariables для Unix.
func NewUnixHostVariables() *HostVariables {
	return NewHostVariables(initUnixHostVariables)
}
