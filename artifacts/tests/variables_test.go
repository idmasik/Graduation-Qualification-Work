package main

import (
	"reflect"
	"testing"
)

func TestVariables(t *testing.T) {
	// Инициализируем HostVariables с тестовыми данными
	hv := NewHostVariables(func(hv *HostVariables) {
		hv.AddVariable("%%users.homedir%%", "%%USERDIR%%")
		hv.AddVariable("%%users.homedir%%", "/tmp/root")
		hv.AddVariable("%%USERDIR%%", "/home/user")
	})

	// Тест 1: Подстановка test%%USERDIR%%test
	t.Run("substitute USERDIR in middle", func(t *testing.T) {
		result := hv.Substitute("test%%USERDIR%%test")
		expected := map[string]struct{}{
			"test/home/usertest": {},
		}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	// Тест 2: Строка без переменных
	t.Run("no variables", func(t *testing.T) {
		result := hv.Substitute("i_dont_have_variables")
		expected := map[string]struct{}{
			"i_dont_have_variables": {},
		}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	// Тест 3: Строка с неподдерживаемой переменной
	t.Run("unsupported variables", func(t *testing.T) {
		result := hv.Substitute("i_contain_%%unsupported%%_variables")
		expected := map[string]struct{}{
			"i_contain_%%unsupported%%_variables": {},
		}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})
}
