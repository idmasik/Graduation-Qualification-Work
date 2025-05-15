//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/windows/registry"
)

// helper: split full registry path into hive and subpath
func extractHive(full string) string {
	parts := strings.SplitN(full, "\\", 2)
	return parts[0]
}

func extractSubpath(full string) string {
	parts := strings.SplitN(full, "\\", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// ----------------------------------------------------------------------
// RegistryReader – потокобезопасная реализация FileSystem для реестра
// ----------------------------------------------------------------------

type RegistryReader struct {
	hive     string
	pattern  string
	keys     map[string]registry.Key
	keysLock sync.RWMutex
}

func NewRegistryReader(hive, pattern string) *RegistryReader {
	return &RegistryReader{
		hive:    hive,
		pattern: pattern,
		keys:    make(map[string]registry.Key),
	}
}

// _key открывает ключ, если он ещё не открыт, и кэширует его.
func (r *RegistryReader) _key(po *PathObject, parent *PathObject) (*PathObject, error) {
	// RLock для чтения map
	r.keysLock.RLock()
	if k, ok := r.keys[po.path]; ok {
		r.keysLock.RUnlock()
		po.obj = k
		return po, nil
	}
	r.keysLock.RUnlock()

	// открыть у родителя
	parentKey, ok := parent.obj.(registry.Key)
	if !ok {
		return nil, fmt.Errorf("invalid parent key for %s", po.path)
	}
	opened, err := registry.OpenKey(parentKey, po.name, registry.READ|registry.WOW64_64KEY)
	if err != nil {
		return nil, err
	}

	// Lock для записи
	r.keysLock.Lock()
	r.keys[po.path] = opened
	r.keysLock.Unlock()

	po.obj = opened
	return po, nil
}

func (r *RegistryReader) baseGenerator() <-chan *PathObject {
	out := make(chan *PathObject, 1)
	hKey, err := getHiveKey(r.hive)
	if err != nil {
		close(out)
		return out
	}
	out <- &PathObject{filesystem: r, name: r.hive, path: r.hive, obj: hKey}
	close(out)
	return out
}

// parse pattern into components
func (r *RegistryReader) _parse(pattern string) []GeneratorFunc {
	parts := strings.Split(pattern, "\\")
	var comps []GeneratorFunc
	for _, part := range parts {
		if part == "" {
			continue
		}
		switch {
		case part == "**":
			comps = append(comps, func(in <-chan *PathObject) <-chan *PathObject {
				return NewRecursionPathComponent(true, -1, in).Generate()
			})
		case strings.ContainsAny(part, "*?["):
			p := part
			comps = append(comps, func(in <-chan *PathObject) <-chan *PathObject {
				return NewGlobPathComponent(true, p, in).Generate()
			})
		default:
			p := part
			comps = append(comps, func(in <-chan *PathObject) <-chan *PathObject {
				return NewRegularPathComponent(true, p, in).Generate()
			})
		}
	}
	return comps
}

func (r *RegistryReader) keysToCollect() <-chan *PathObject {
	stream := r.baseGenerator()
	for _, gf := range r._parse(r.pattern) {
		stream = gf(stream)
	}
	return stream
}

// --- FileSystem interface methods for registry ---

func (r *RegistryReader) AddPattern(artifact, pattern, sourceType string) {}
func (r *RegistryReader) parse(pattern string) []GeneratorFunc            { return r._parse(pattern) }
func (r *RegistryReader) GetFullPath(fullpath string) *PathObject         { return nil }
func (r *RegistryReader) relativePath(fp string) string                   { return fp }
func (r *RegistryReader) IsSymlink(p *PathObject) bool                    { return false }
func (r *RegistryReader) ReadChunks(p *PathObject) ([][]byte, error)      { return nil, nil }
func (r *RegistryReader) GetSize(p *PathObject) int64                     { return 0 }

func (r *RegistryReader) IsDirectory(p *PathObject) bool {
	key, ok := p.obj.(registry.Key)
	if !ok {
		return false
	}
	names, err := key.ReadSubKeyNames(1)
	return err == nil && len(names) > 0
}

func (r *RegistryReader) IsFile(p *PathObject) bool { return true }

func (r *RegistryReader) ListDirectory(p *PathObject) []*PathObject {
	var res []*PathObject
	key, ok := p.obj.(registry.Key)
	if !ok {
		return res
	}
	names, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return res
	}
	for _, name := range names {
		child := &PathObject{filesystem: r, name: strings.ToLower(name), path: filepath.Join(p.path, name)}
		if po, err := r._key(child, p); err == nil {
			res = append(res, po)
		}
	}
	return res
}

func (r *RegistryReader) GetPath(parent *PathObject, name string) *PathObject {
	child := &PathObject{filesystem: r, name: name, path: filepath.Join(parent.path, name)}
	if po, err := r._key(child, parent); err == nil {
		return po
	}
	return nil
}

func (r *RegistryReader) Close() {
	r.keysLock.Lock()
	defer r.keysLock.Unlock()
	for _, k := range r.keys {
		k.Close()
	}
}

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
			if s, typ, err := key.GetStringValue(name); err == nil {
				out <- [3]interface{}{name, r.normalizeValue(s), typ}
				continue
			}
			if i, _, err := key.GetIntegerValue(name); err == nil {
				out <- [3]interface{}{name, r.normalizeValue(i), "REG_DWORD"}
				continue
			}
			if b, _, err := key.GetBinaryValue(name); err == nil {
				out <- [3]interface{}{name, r.normalizeValue(b), "REG_BINARY"}
			}
		}
	}()
	return out
}

func (r *RegistryReader) GetKeyValue(p *PathObject, valueName string) map[string]interface{} {
	key, ok := p.obj.(registry.Key)
	if !ok {
		return nil
	}
	if s, typ, err := key.GetStringValue(valueName); err == nil {
		return map[string]interface{}{"value": r.normalizeValue(s), "type": typ}
	}
	if i, _, err := key.GetIntegerValue(valueName); err == nil {
		return map[string]interface{}{"value": r.normalizeValue(i), "type": "REG_DWORD"}
	}
	if b, _, err := key.GetBinaryValue(valueName); err == nil {
		return map[string]interface{}{"value": r.normalizeValue(b), "type": "REG_BINARY"}
	}
	return nil
}

func (r *RegistryReader) normalizeValue(val interface{}) interface{} {
	if _, err := json.Marshal(val); err != nil {
		return fmt.Sprintf("%v", val)
	}
	return val
}

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
	}
	return 0, fmt.Errorf("unknown hive: %s", hive)
}

// ----------------------------------------------------------------------
// RegistryCollector – сбор данных реестра
// ----------------------------------------------------------------------

type RegistryCollector struct {
	keys   []map[string]string
	values []map[string]string
}

func NewRegistryCollector() *RegistryCollector {
	return &RegistryCollector{keys: make([]map[string]string, 0), values: make([]map[string]string, 0)}
}

func (rc *RegistryCollector) add_key(artifact, fullKey string) {
	h := extractHive(fullKey)
	sub := extractSubpath(fullKey)
	rc.keys = append(rc.keys, map[string]string{"artifact": artifact, "hive": h, "key": sub})
}

func (rc *RegistryCollector) add_value(artifact, fullKey, valName string) {
	h := extractHive(fullKey)
	sub := extractSubpath(fullKey)
	rc.values = append(rc.values, map[string]string{"artifact": artifact, "hive": h, "key": sub, "value": valName})
}

func (rc *RegistryCollector) RegisterSource(def *ArtifactDefinition, src *Source, vars *HostVariables) bool {
	supported := false
	if src.TypeIndicator == TYPE_INDICATOR_WINDOWS_REGISTRY_KEY {
		supported = true
		if raw, ok := src.Attributes["keys"].([]interface{}); ok {
			for _, item := range raw {
				if ks, ok := item.(string); ok {
					subs := vars.Substitute(ks)
					if len(subs) == 0 {
						rc.add_key(def.Name, ks)
					}
					for sub := range subs {
						rc.add_key(def.Name, sub)
					}
				}
			}
		}
	} else if src.TypeIndicator == TYPE_INDICATOR_WINDOWS_REGISTRY_VALUE {
		supported = true
		if raw, ok := src.Attributes["key_value_pairs"].([]interface{}); ok {
			for _, item := range raw {
				if kv, ok := item.(map[string]interface{}); ok {
					if keyStr, ok := kv["key"].(string); ok {
						valStr, _ := kv["value"].(string)
						subs := vars.Substitute(keyStr)
						if len(subs) == 0 {
							rc.add_value(def.Name, keyStr, valStr)
						}
						for sub := range subs {
							rc.add_value(def.Name, sub, valStr)
						}
					}
				}
			}
		}
	}
	return supported
}

func (rc *RegistryCollector) Collect(output *Outputs) {
	// ключи
	for _, e := range rc.keys {
		reader := NewRegistryReader(e["hive"], e["key"])
		for po := range reader.keysToCollect() {
			for triple := range reader.GetKeyValues(po) {
				name := triple[0].(string)
				val := triple[1]
				typStr := fmt.Sprintf("%v", triple[2])
				output.AddCollectedRegistryValue(e["artifact"], po.path, name, val, typStr)
			}
		}
		reader.Close()
	}
	// значения
	for _, e := range rc.values {
		reader := NewRegistryReader(e["hive"], e["key"])
		for po := range reader.keysToCollect() {
			if kv := reader.GetKeyValue(po, e["value"]); kv != nil {
				val := kv["value"]
				typStr := fmt.Sprintf("%v", kv["type"])
				output.AddCollectedRegistryValue(e["artifact"], po.path, e["value"], val, typStr)
			}
		}
		reader.Close()
	}
}

// ensure RegistryCollector implements AbstractCollector
var _ AbstractCollector = (*RegistryCollector)(nil)

func (r *RegistryReader) Collect(output *Outputs) {
	// Ничего не делаем для реестра специально
	_ = output // Чтобы избежать предупреждений о неиспользованной переменной
}
