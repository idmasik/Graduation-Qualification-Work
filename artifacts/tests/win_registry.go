package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// RegistryReader реализует доступ к реестру Windows, действуя как файловая система.
type RegistryReader struct {
	hive    string // например, "HKEY_LOCAL_MACHINE"
	pattern string // шаблон пути (например, "Software\\Microsoft\\Windows")
	keys    map[string]registry.Key
}

// NewRegistryReader создаёт новый RegistryReader.
func NewRegistryReader(hive, pattern string) *RegistryReader {
	return &RegistryReader{
		hive:    hive,
		pattern: pattern,
		keys:    make(map[string]registry.Key),
	}
}

// getHiveKey возвращает предопределённый ключ реестра по строке.
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
		return 0, fmt.Errorf("неизвестный hive: %s", hive)
	}
}

// openKey открывает и кэширует ключ, аналог метода _key в Python.
func (r *RegistryReader) openKey(po *PathObject, parent *PathObject) (*PathObject, error) {
	if key, ok := r.keys[po.path]; ok {
		po.obj = key
		return po, nil
	}
	// parent.obj должен быть registry.Key
	parentKey, ok := parent.obj.(registry.Key)
	if !ok {
		return nil, fmt.Errorf("недопустимый родительский ключ")
	}
	// Открываем дочерний ключ с правами чтения и флагом WOW64_64KEY.
	opened, err := registry.OpenKey(parentKey, po.name, registry.READ|registry.WOW64_64KEY)
	if err != nil {
		return nil, err
	}
	r.keys[po.path] = opened
	po.obj = opened
	return po, nil
}

// baseGenerator возвращает канал с базовым объектом – предопределённым ключом hive.
func (r *RegistryReader) baseGenerator() <-chan *PathObject {
	out := make(chan *PathObject, 1)
	hKey, err := getHiveKey(r.hive)
	if err != nil {
		// Логируем ошибку и возвращаем пустой канал
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

// keysToCollect реализует последовательное применение компонентов пути.
// Здесь для простоты шаблон разбивается по "\" и последовательно вызывается GetPath.
func (r *RegistryReader) keysToCollect() (<-chan *PathObject, error) {
	parts := strings.Split(r.pattern, "\\")
	gen := r.baseGenerator()
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		gen = r.applyRegularComponent(gen, part)
	}
	return gen, nil
}

// applyRegularComponent реализует переход по обычному компоненту пути.
func (r *RegistryReader) applyRegularComponent(source <-chan *PathObject, part string) <-chan *PathObject {
	out := make(chan *PathObject)
	go func() {
		defer close(out)
		for parent := range source {
			child := r.GetPath(parent, part)
			if child != nil {
				out <- child
			}
		}
	}()
	return out
}

// ListDirectory перечисляет дочерние ключи (поддиректории) заданного PathObject.
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
		if child, err := r.openKey(po, p); err == nil {
			result = append(result, child)
		}
	}
	return result
}

// GetPath получает дочерний ключ по имени от родительского.
func (r *RegistryReader) GetPath(parent *PathObject, name string) *PathObject {
	keypath := filepath.Join(parent.path, name)
	po := &PathObject{
		filesystem: r,
		name:       name,
		path:       keypath,
	}
	child, err := r.openKey(po, parent)
	if err != nil {
		return nil
	}
	return child
}

// IsDirectory проверяет, можно ли перечислить дочерние ключи.
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

// RegistryValue – структура для хранения значений ключа.
type RegistryValue struct {
	Name  string
	Value interface{}
	Type  interface{}
}

// GetKeyValues перечисляет все значения для заданного ключа.
func (r *RegistryReader) GetKeyValues(p *PathObject) ([]RegistryValue, error) {
	var values []RegistryValue
	key, ok := p.obj.(registry.Key)
	if !ok {
		return values, fmt.Errorf("недопустимый ключ реестра")
	}
	names, err := key.ReadValueNames(-1)
	if err != nil {
		return values, err
	}
	for _, name := range names {
		// Пробуем получить строковое значение.
		if s, typ, err := key.GetStringValue(name); err == nil {
			values = append(values, RegistryValue{Name: name, Value: r.normalizeValue(s), Type: typ})
			continue
		}
		// Пробуем получить целочисленное значение.
		if n, _, err := key.GetIntegerValue(name); err == nil {
			values = append(values, RegistryValue{Name: name, Value: n, Type: "REG_DWORD"})
		}

		// Пробуем получить бинарное значение.
		if b, _, err := key.GetBinaryValue(name); err == nil {
			values = append(values, RegistryValue{Name: name, Value: r.normalizeValue(b), Type: "REG_BINARY"})
		}

	}
	return values, nil
}

// GetKeyValue получает конкретное значение по его имени.
func (r *RegistryReader) GetKeyValue(p *PathObject, valueName string) (*RegistryValue, error) {
	key, ok := p.obj.(registry.Key)
	if !ok {
		return nil, fmt.Errorf("недопустимый ключ реестра")
	}
	if s, typ, err := key.GetStringValue(valueName); err == nil {
		return &RegistryValue{Name: valueName, Value: r.normalizeValue(s), Type: typ}, nil
	}
	if n, _, err := key.GetIntegerValue(valueName); err == nil {
		return &RegistryValue{Name: valueName, Value: n, Type: "REG_DWORD"}, nil
	}
	if b, _, err := key.GetBinaryValue(valueName); err == nil {
		return &RegistryValue{Name: valueName, Value: r.normalizeValue(b), Type: "REG_BINARY"}, nil
	}
	return nil, fmt.Errorf("значение '%s' не найдено", valueName)
}

// normalizeValue пытается сериализовать значение в JSON – если не удаётся, возвращает строковое представление.
func (r *RegistryReader) normalizeValue(val interface{}) interface{} {
	_, err := json.Marshal(val)
	if err != nil {
		return fmt.Sprintf("%v", val)
	}
	return val
}

// ============================================================================
// RegistryCollector реализует сбор данных реестра и использует RegistryReader.
// ============================================================================
// Добавляем метод AddPattern — в данном случае он может быть пустым, если не нужен.
func (r *RegistryReader) AddPattern(artifact, pattern, sourceType string) {
	// Заглушка: для работы с реестром этот метод может быть не нужен.
}

// Заглушки для остальных методов, требуемых интерфейсом FileSystem:
func (r *RegistryReader) Collect(output CollectorOutput) {
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

func (r *RegistryReader) ReadChunks(p *PathObject) ([]byte, error) {
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
