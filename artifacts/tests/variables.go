package main

import (
	"fmt"
	"regexp"
	"strings"
)

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
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(name))
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

func (hv *HostVariables) Substitute(value string) map[string]struct{} {
	values := make(map[string]struct{})

	if strings.Count(value, "%") < 2 {
		values[value] = struct{}{}
		return values
	}

	hasSubstitution := false

	for _, variable := range hv.variables {
		for variableValue := range variable.Value {
			escapedValue := strings.ReplaceAll(variableValue, `\`, `\\`)
			replaced := variable.Re.ReplaceAllString(value, escapedValue)
			if replaced != value {
				hasSubstitution = true
				substituted := hv.Substitute(replaced)
				for s := range substituted {
					values[s] = struct{}{}
				}
			}
		}
	}

	if !hasSubstitution {
		hv.logger.Log(LevelWarning, fmt.Sprintf("Value '%s' contains unsupported variables", value))
		values[value] = struct{}{}
	}

	return values
}
