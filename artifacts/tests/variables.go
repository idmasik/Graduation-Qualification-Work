package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// -------------------------
// HostVariables и Substitution
// -------------------------

type Variable struct {
	Name  string
	Re    *regexp.Regexp
	Value map[string]struct{}
}

type HostVariables struct {
	variables []Variable
	logger    *Logger
	initFunc  func(*HostVariables)
}

func NewHostVariables(initFunc func(*HostVariables)) *HostVariables {
	hv := &HostVariables{
		logger:   logger,
		initFunc: initFunc,
	}
	hv.initFunc(hv)
	hv.ResolveVariables()
	return hv
}

func (hv *HostVariables) AddVariable(name, value string) {
	// Создаем регулярное выражение, которое ищет точное совпадение с переменной.
	re := regexp.MustCompile(regexp.QuoteMeta(name))
	values := make(map[string]struct{})
	values[value] = struct{}{}
	hv.variables = append(hv.variables, Variable{
		Name:  name,
		Re:    re,
		Value: values,
	})
}

func (hv *HostVariables) ResolveVariables() {
	for i := range hv.variables {
		variable := &hv.variables[i]
		resolvedValues := make(map[string]struct{})

		for value := range variable.Value {
			substituted := hv.Substitute(value)
			for s := range substituted {
				resolvedValues[s] = struct{}{}
			}
		}

		variable.Value = resolvedValues
	}
}

// Обновленная функция Substitute, которая ищет шаблоны вида %%имя%% и заменяет их на значение из окружения или из HostVariables.
func (hv *HostVariables) Substitute(value string) map[string]struct{} {
	values := make(map[string]struct{})
	// Регулярное выражение для поиска шаблонов вида %%что-то%%
	re := regexp.MustCompile(`%%([^%]+)%%`)
	substituted := value

	// Цикл для замены (вложенные замены)
	for {
		matches := re.FindAllStringSubmatch(substituted, -1)
		if len(matches) == 0 {
			break
		}
		changed := false
		for _, match := range matches {
			fullMatch := match[0] // например, "%%environ_systemroot%%"
			varName := match[1]   // например, "environ_systemroot"
			// Сначала ищем в переменных хоста
			replaced := ""
			for _, variable := range hv.variables {
				if strings.EqualFold(variable.Name, fullMatch) {
					// Берем первое значение из набора
					for v := range variable.Value {
						replaced = v
						break
					}
				}
			}
			// Если не найдено в hv, пробуем взять из окружения
			if replaced == "" {
				// Пробуем в верхнем регистре
				replaced = os.Getenv(strings.ToUpper(varName))
				if replaced == "" {
					// Пробуем как есть
					replaced = os.Getenv(varName)
				}
			}
			if replaced != "" {
				substituted = strings.ReplaceAll(substituted, fullMatch, replaced)
				changed = true
			} else {
				hv.logger.Log(LevelWarning, fmt.Sprintf("Value '%s' contains unsupported variable '%s'", value, fullMatch))
				// Чтобы не зациклиться, прерываем замену, оставляя шаблон как есть.
				break
			}
		}
		if !changed {
			break
		}
	}
	values[substituted] = struct{}{}
	return values
}

// -------------------------
// Функция инициализации переменных (defaultInitFunc)
// -------------------------

func defaultInitFunc(hv *HostVariables) {
	// Для Windows
	if sysroot := os.Getenv("SYSTEMROOT"); sysroot != "" {
		hv.AddVariable("%%environ_systemroot%%", sysroot)
	}
	if userprofile := os.Getenv("USERPROFILE"); userprofile != "" {
		// Используем USERPROFILE для %%users.homedir%% и %%users.userprofile%%
		hv.AddVariable("%%users.homedir%%", userprofile)
		hv.AddVariable("%%users.userprofile%%", userprofile)
	}
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		hv.AddVariable("%%users.appdata%%", appdata)
	}
	if localappdata := os.Getenv("LOCALAPPDATA"); localappdata != "" {
		hv.AddVariable("%%users.localappdata%%", localappdata)
	}
	if allusers := os.Getenv("ALLUSERSPROFILE"); allusers != "" {
		hv.AddVariable("%%environ_allusersprofile%%", allusers)
	}
	if sysdrive := os.Getenv("SystemDrive"); sysdrive != "" {
		hv.AddVariable("%%environ_systemdrive%%", sysdrive)
	}
	if windir := os.Getenv("WINDIR"); windir != "" {
		hv.AddVariable("%%environ_windir%%", windir)
	}
	if progfiles := os.Getenv("ProgramFiles"); progfiles != "" {
		hv.AddVariable("%%environ_programfiles%%", progfiles)
	}
	if progfilesx86 := os.Getenv("ProgramFiles(x86)"); progfilesx86 != "" {
		hv.AddVariable("%%environ_programfilesx86%%", progfilesx86)
	}

	// Для Unix‑подобных систем (если требуется)
	if home := os.Getenv("HOME"); home != "" {
		hv.AddVariable("%%users.homedir%%", home)
		// Можно также использовать HOME для локальных данных
		hv.AddVariable("%%users.localappdata%%", home)
	}
}
