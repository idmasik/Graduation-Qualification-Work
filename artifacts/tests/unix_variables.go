package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type UnixHostVariables struct {
	*HostVariables
}

func initUnixHostVariables(hv *HostVariables) {
	userprofiles := make(map[string]struct{})

	// Чтение информации о пользователях из /etc/passwd
	file, err := os.Open("/etc/passwd")
	if err != nil {
		hv.logger.Log(LevelWarning, fmt.Sprintf("Failed to open passwd file: %v", err))
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

	// Добавление каждого пути как отдельного значения переменной
	for dir := range userprofiles {
		hv.AddVariable("%%users.homedir%%", dir)
	}
}

// Пример создания экземпляра
func NewUnixHostVariables() *HostVariables {
	return NewHostVariables(initUnixHostVariables)
}
