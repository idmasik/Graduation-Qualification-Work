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
