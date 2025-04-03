//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// ----------------------------------------------------------------------
// RegistryReader – реализация для работы с реестром Windows
// ----------------------------------------------------------------------

type RegistryReader struct {
	hive    string
	pattern string
	keys    map[string]registry.Key
}

// NewRegistryReader создаёт новый объект для чтения реестра.
func NewRegistryReader(hive, pattern string) *RegistryReader {
	return &RegistryReader{
		hive:    hive,
		pattern: pattern,
		keys:    make(map[string]registry.Key),
	}
}

// _key открывает ключ, если он ещё не открыт, и кэширует его.
func (r *RegistryReader) _key(po *PathObject, parent *PathObject) (*PathObject, error) {
	if key, ok := r.keys[po.path]; ok {
		po.obj = key
		return po, nil
	}
	parentKey, ok := parent.obj.(registry.Key)
	if !ok {
		return nil, fmt.Errorf("invalid parent key")
	}
	opened, err := registry.OpenKey(parentKey, po.name, registry.READ|registry.WOW64_64KEY)
	if err != nil {
		return nil, err
	}
	r.keys[po.path] = opened
	po.obj = opened
	return po, nil
}

// baseGenerator возвращает канал с базовым объектом – предопределённым ключом (hive).
func (r *RegistryReader) baseGenerator() <-chan *PathObject {
	out := make(chan *PathObject, 1)
	hKey, err := getHiveKey(r.hive)
	if err != nil {
		close(out)
		return out
	}
	po := &PathObject{
		filesystem: r,
		name:       r.hive,
		path:       r.hive,
		obj:        hKey,
	}
	out <- po
	close(out)
	return out
}

// _parse разбирает шаблон пути и возвращает срез генераторов.
func (r *RegistryReader) _parse(pattern string) []GeneratorFunc {
	parts := strings.Split(pattern, "\\")
	var funcs []GeneratorFunc
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "**" {
			funcs = append(funcs, func(in <-chan *PathObject) <-chan *PathObject {
				return NewRecursionPathComponent(true, -1, in).Generate()
			})
		} else if strings.ContainsAny(part, "*?[") {
			funcs = append(funcs, func(in <-chan *PathObject) <-chan *PathObject {
				return NewGlobPathComponent(true, part, in).Generate()
			})
		} else {
			funcs = append(funcs, func(in <-chan *PathObject) <-chan *PathObject {
				return NewRegularPathComponent(true, part, in).Generate()
			})
		}
	}
	return funcs
}

// keysToCollect строит цепочку генераторов для обхода ключей по шаблону.
func (r *RegistryReader) keysToCollect() <-chan *PathObject {
	gen := r.baseGenerator()
	genFuncs := r._parse(r.pattern)
	for _, gf := range genFuncs {
		gen = gf(gen)
	}
	return gen
}

// ListDirectory перечисляет дочерние ключи заданного объекта.
func (r *RegistryReader) ListDirectory(p *PathObject) []*PathObject {
	var result []*PathObject
	key, ok := p.obj.(registry.Key)
	if !ok {
		return result
	}
	names, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return result
	}
	for _, name := range names {
		keypath := filepath.Join(p.path, name)
		po := &PathObject{
			filesystem: r,
			name:       strings.ToLower(name),
			path:       keypath,
		}
		if child, err := r._key(po, p); err == nil {
			result = append(result, child)
		}
	}
	return result
}

// GetPath получает дочерний ключ по имени.
func (r *RegistryReader) GetPath(parent *PathObject, name string) *PathObject {
	keypath := filepath.Join(parent.path, name)
	po := &PathObject{
		filesystem: r,
		name:       name,
		path:       keypath,
	}
	child, err := r._key(po, parent)
	if err != nil {
		return nil
	}
	return child
}

// IsDirectory проверяет, есть ли у ключа дочерние ключи.
func (r *RegistryReader) IsDirectory(p *PathObject) bool {
	key, ok := p.obj.(registry.Key)
	if !ok {
		return false
	}
	names, err := key.ReadSubKeyNames(1)
	return err == nil && len(names) > 0
}

// IsFile для реестра всегда возвращает true.
func (r *RegistryReader) IsFile(p *PathObject) bool {
	return true
}

// Close закрывает все открытые ключи.
func (r *RegistryReader) Close() {
	for _, key := range r.keys {
		key.Close()
	}
}

// GetKeyValues перечисляет все значения ключа.
// Возвращает канал, в который отправляются срезы: [name, normalizedValue, type].
// GetKeyValues перечисляет все значения ключа.
// Здесь получаем список имён значений и для каждого пытаемся прочитать данные.
func (r *RegistryReader) GetKeyValues(p *PathObject) <-chan [3]interface{} {
	out := make(chan [3]interface{})
	go func() {
		defer close(out)
		key, ok := p.obj.(registry.Key)
		if !ok {
			return
		}
		names, err := key.ReadValueNames(-1)
		if err != nil {
			return
		}
		for _, name := range names {
			// Попытка получить строковое значение
			if s, typ, err := key.GetStringValue(name); err == nil {
				out <- [3]interface{}{name, r.normalizeValue(s), typ}
				continue
			}
			// Попытка получить целочисленное значение
			if i, _, err := key.GetIntegerValue(name); err == nil {
				out <- [3]interface{}{name, r.normalizeValue(i), "REG_DWORD"}
				continue
			}
			// Попытка получить бинарное значение
			if b, _, err := key.GetBinaryValue(name); err == nil {
				out <- [3]interface{}{name, r.normalizeValue(b), "REG_BINARY"}
				continue
			}
		}
	}()
	return out
}

// GetKeyValue получает конкретное значение из ключа.
// GetKeyValue получает конкретное значение из ключа.
func (r *RegistryReader) GetKeyValue(p *PathObject, valueName string) map[string]interface{} {
	key, ok := p.obj.(registry.Key)
	if !ok {
		return nil
	}
	// Попытка получить строковое значение
	if s, typ, err := key.GetStringValue(valueName); err == nil {
		return map[string]interface{}{
			"value": r.normalizeValue(s),
			"type":  typ,
		}
	}
	// Попытка получить целочисленное значение
	if i, _, err := key.GetIntegerValue(valueName); err == nil {
		return map[string]interface{}{
			"value": r.normalizeValue(i),
			"type":  "REG_DWORD",
		}
	}
	// Попытка получить бинарное значение
	if b, _, err := key.GetBinaryValue(valueName); err == nil {
		return map[string]interface{}{
			"value": r.normalizeValue(b),
			"type":  "REG_BINARY",
		}
	}
	return nil
}

// normalizeValue пытается сериализовать значение в JSON, если не удаётся – возвращает строковое представление.
func (r *RegistryReader) normalizeValue(val interface{}) interface{} {
	_, err := json.Marshal(val)
	if err != nil {
		return fmt.Sprintf("%v", val)
	}
	return val
}

// getHiveKey возвращает соответствующий ключ реестра по названию hive.
func getHiveKey(hive string) (registry.Key, error) {
	switch hive {
	case "HKEY_LOCAL_MACHINE":
		return registry.LOCAL_MACHINE, nil
	case "HKEY_CURRENT_USER":
		return registry.CURRENT_USER, nil
	case "HKEY_CLASSES_ROOT":
		return registry.CLASSES_ROOT, nil
	case "HKEY_USERS":
		return registry.USERS, nil
	case "HKEY_CURRENT_CONFIG":
		return registry.CURRENT_CONFIG, nil
	default:
		return 0, fmt.Errorf("unknown hive: %s", hive)
	}
}

// ============================================================================
// RegistryCollector реализует сбор данных реестра и использует RegistryReader.
// ============================================================================
// Добавляем метод AddPattern — в данном случае он может быть пустым, если не нужен.
func (r *RegistryReader) AddPattern(artifact, pattern, sourceType string) {
	// Заглушка: для работы с реестром этот метод может быть не нужен.
}

// Заглушки для остальных методов, требуемых интерфейсом FileSystem:
func (r *RegistryReader) Collect(output *Outputs) {
	// Заглушка
}

func (r *RegistryReader) relativePath(fp string) string {
	return fp
}

func (r *RegistryReader) parse(pattern string) []GeneratorFunc {
	return nil
}

func (r *RegistryReader) IsSymlink(p *PathObject) bool {
	return false
}

func (r *RegistryReader) GetFullPath(fullpath string) *PathObject {
	return nil
}

func (r *RegistryReader) ReadChunks(p *PathObject) ([][]byte, error) {
	return nil, nil
}

func (r *RegistryReader) GetSize(p *PathObject) int64 {
	return 0
}

// RegistryKeyEntry описывает запрос для сбора всего ключа.
type RegistryKeyEntry struct {
	Artifact string
	Hive     string
	Key      string
}

// RegistryValueEntry описывает запрос для сбора конкретного значения.
type RegistryValueEntry struct {
	Artifact string
	Hive     string
	Key      string
	Value    string
}

// ----------------------------------------------------------------------
// RegistryCollector – сбор реестровых данных
// ----------------------------------------------------------------------
// RegistryCollector собирает ключи и значения реестра.
type RegistryCollector struct {
	keys   []map[string]string // каждый элемент: artifact, hive, key
	values []map[string]string // каждый элемент: artifact, hive, key, value
}

func NewRegistryCollector() *RegistryCollector {
	return &RegistryCollector{
		keys:   make([]map[string]string, 0),
		values: make([]map[string]string, 0),
	}
}

func (rc *RegistryCollector) add_key(artifact, key string) {
	keyParts := strings.Split(key, "\\")
	if len(keyParts) < 1 {
		return
	}
	hive := keyParts[0]
	subKey := ""
	if len(keyParts) > 1 {
		subKey = strings.Join(keyParts[1:], "/")
	}
	rc.keys = append(rc.keys, map[string]string{
		"artifact": artifact,
		"hive":     hive,
		"key":      subKey,
	})
}

func (rc *RegistryCollector) add_value(artifact, key, value string) {
	keyParts := strings.Split(key, "\\")
	if len(keyParts) < 1 {
		return
	}
	hive := keyParts[0]
	subKey := ""
	if len(keyParts) > 1 {
		subKey = strings.Join(keyParts[1:], "/")
	}
	rc.values = append(rc.values, map[string]string{
		"artifact": artifact,
		"hive":     hive,
		"key":      subKey,
		"value":    value,
	})
}

// Collect проходит по всем зарегистрированным ключам и значениям и передаёт данные в output.
func (rc *RegistryCollector) Collect(output *Outputs) {
	// Обработка ключей
	for _, keyEntry := range rc.keys {
		reader := NewRegistryReader(keyEntry["hive"], keyEntry["key"])
		for po := range reader.keysToCollect() {
			for triple := range reader.GetKeyValues(po) {
				name, value, typ := triple[0].(string), triple[1], triple[2]
				output.AddCollectedRegistryValue(
					keyEntry["artifact"],
					po.path,
					name,
					value,
					fmt.Sprintf("%v", typ))
			}
		}
		reader.Close()
	}

	// Обработка значений
	for _, keyValue := range rc.values {
		reader := NewRegistryReader(keyValue["hive"], keyValue["key"])
		for po := range reader.keysToCollect() {
			regVal := reader.GetKeyValue(po, keyValue["value"])
			if regVal != nil {
				output.AddCollectedRegistryValue(
					keyValue["artifact"],
					po.path,
					keyValue["value"],
					regVal["value"],
					fmt.Sprintf("%v", regVal["type"]))
			}
		}
		reader.Close()
	}
}

func (rc *RegistryCollector) RegisterSource(artifactDefinition *ArtifactDefinition, artifactSource *Source, variables *HostVariables) bool {
	supported := false
	if artifactSource.TypeIndicator == TYPE_INDICATOR_WINDOWS_REGISTRY_KEY {
		supported = true
		// Извлекаем ключи из Attributes["keys"]
		if keysAttr, ok := artifactSource.Attributes["keys"]; ok {
			if keysList, ok := keysAttr.([]interface{}); ok {
				for _, keyIface := range keysList {
					if keyStr, ok := keyIface.(string); ok {
						// Используем Substitute для подстановки переменных
						for _, sub := range variables.Substitute(keyStr) {
							// Преобразуем sub в строку
							subKey := fmt.Sprintf("%v", sub)
							rc.add_key(artifactDefinition.Name, subKey)
						}
					}
				}
			}
		}
	} else if artifactSource.TypeIndicator == TYPE_INDICATOR_WINDOWS_REGISTRY_VALUE {
		supported = true
		// Извлекаем пары ключ-значение из Attributes["key_value_pairs"]
		if kvAttr, ok := artifactSource.Attributes["key_value_pairs"]; ok {
			if kvList, ok := kvAttr.([]interface{}); ok {
				for _, kvIface := range kvList {
					if kvMap, ok := kvIface.(map[string]interface{}); ok {
						// Извлекаем значение ключа
						if keyVal, exists := kvMap["key"]; exists {
							if keyStr, ok := keyVal.(string); ok {
								var valueStr string
								if v, exists := kvMap["value"]; exists {
									if s, ok := v.(string); ok {
										valueStr = s
									}
								}
								for _, sub := range variables.Substitute(keyStr) {
									subKey := fmt.Sprintf("%v", sub)
									rc.add_value(artifactDefinition.Name, subKey, valueStr)
								}
							}
						}
					}
				}
			}
		}
	}
	return supported
}
