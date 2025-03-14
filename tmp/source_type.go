package main

import (
	"errors"
	"fmt"
)

// Базовый интерфейс SourceType. Он определяет общие методы для всех типов источников,что позволяет обрабатывать разные источники через один тип.
// Определяет два обязательных метода: (Методы TypeIndicator и AsDict нужны для идентификации типа и сериализации данных соответственно.)
// TypeIndicator(): возвращает строковый индикатор типа.
// AsDict(): возвращает представление типа в виде словаря (map), пригодного для сериализации

type SourceType interface {
	TypeIndicator() string
	AsDict() map[string]interface{}
}

// ------------------------------------------------------------------------------------------------------
// Базовая структура BaseSourceType встраивается в конкретные типы,
// Поле TypeIndicatorValue хранит строковый идентификатор типа, а метод TypeIndicator возвращает его.
type BaseSourceType struct {
	TypeIndicatorValue string
}

func (b *BaseSourceType) TypeIndicator() string {
	return b.TypeIndicatorValue
}

// -------------------------------------------------------------------------------------------------------
// Конкретные типы, такие как ArtifactGroupSourceType, CommandSourceType и другие, наследуют BaseSourceType и добавляют свои поля.
// Конструкторы этих типов проверяют входные данные на валидность, что обеспечивает корректное создание объектов.
// Метод AsDict в каждом конкретном типе преобразует структуру в словарь, подходящий для сериализации.
type ArtifactGroupSourceType struct {
	BaseSourceType
	Names []string
}

func NewArtifactGroupSourceType(names []string) (*ArtifactGroupSourceType, error) {
	if len(names) == 0 {
		return nil, errors.New("missing names value")
	}
	return &ArtifactGroupSourceType{
		BaseSourceType: BaseSourceType{TypeIndicatorValue: TYPE_INDICATOR_ARTIFACT_GROUP},
		Names:          names,
	}, nil
}

func (a *ArtifactGroupSourceType) AsDict() map[string]interface{} {
	return map[string]interface{}{"names": a.Names}
}

// CommandSourceType represents a command to be executed.
type CommandSourceType struct {
	BaseSourceType
	Cmd  string
	Args []string
}

func NewCommandSourceType(cmd string, args []string) (*CommandSourceType, error) {
	if cmd == "" || len(args) == 0 {
		return nil, errors.New("missing cmd or args value")
	}
	return &CommandSourceType{
		BaseSourceType: BaseSourceType{TypeIndicatorValue: TYPE_INDICATOR_COMMAND},
		Cmd:            cmd,
		Args:           args,
	}, nil
}

func (c *CommandSourceType) AsDict() map[string]interface{} {
	return map[string]interface{}{
		"cmd":  c.Cmd,
		"args": c.Args,
	}
}

// DirectorySourceType represents a directory source.
type DirectorySourceType struct {
	BaseSourceType
	Paths     []string
	Separator string
}

func NewDirectorySourceType(paths []string, separator string) (*DirectorySourceType, error) {
	if len(paths) == 0 {
		return nil, errors.New("missing paths value")
	}
	if separator == "" {
		separator = "/"
	}
	return &DirectorySourceType{
		BaseSourceType: BaseSourceType{TypeIndicatorValue: TYPE_INDICATOR_DIRECTORY},
		Paths:          paths,
		Separator:      separator,
	}, nil
}

func (d *DirectorySourceType) AsDict() map[string]interface{} {
	result := map[string]interface{}{"paths": d.Paths}
	if d.Separator != "/" {
		result["separator"] = d.Separator
	}
	return result
}

// WindowsRegistryKeySourceType represents Windows Registry keys.
type WindowsRegistryKeySourceType struct {
	BaseSourceType
	Keys []string
}

var validRegistryPrefixes = []string{
	"HKEY_LOCAL_MACHINE",
	"HKEY_USERS",
	"HKEY_CLASSES_ROOT",
	"%%current_control_set%%",
}

func NewWindowsRegistryKeySourceType(keys []string) (*WindowsRegistryKeySourceType, error) {
	if len(keys) == 0 {
		return nil, errors.New("missing keys value")
	}
	for _, key := range keys {
		if !isValidRegistryKey(key) {
			return nil, fmt.Errorf("unsupported registry key: %s", key)
		}
	}
	return &WindowsRegistryKeySourceType{
		BaseSourceType: BaseSourceType{TypeIndicatorValue: TYPE_INDICATOR_WINDOWS_REGISTRY_KEY},
		Keys:           keys,
	}, nil
}

// Функции валидации, например isValidRegistryKey, обеспечивают корректность данных для специфических типов.
func isValidRegistryKey(key string) bool {
	for _, prefix := range validRegistryPrefixes {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func (w *WindowsRegistryKeySourceType) AsDict() map[string]interface{} {
	return map[string]interface{}{"keys": w.Keys}
}

type WMIQuerySourceType struct {
	BaseSourceType
	BaseObject string
	Query      string
}

func NewWMIQuerySourceType(baseObject string, query string) (*WMIQuerySourceType, error) {
	if query == "" {
		return nil, errors.New("missing query value")
	}
	return &WMIQuerySourceType{
		BaseSourceType: BaseSourceType{TypeIndicatorValue: TYPE_INDICATOR_WMI_QUERY},
		BaseObject:     baseObject,
		Query:          query,
	}, nil
}

func (w *WMIQuerySourceType) AsDict() map[string]interface{} {
	result := map[string]interface{}{
		"query": w.Query,
	}
	if w.BaseObject != "" {
		result["base_object"] = w.BaseObject
	}
	return result
}

type WindowsRegistryValueSourceType struct {
	BaseSourceType
	KeyValuePairs []map[string]string
}

func NewWindowsRegistryValueSourceType(keyValuePairs []map[string]string) (*WindowsRegistryValueSourceType, error) {
	if len(keyValuePairs) == 0 {
		return nil, errors.New("missing key value pairs")
	}
	for _, pair := range keyValuePairs {
		if _, ok := pair["key"]; !ok {
			return nil, errors.New("missing 'key' in key-value pair")
		}
		if _, ok := pair["value"]; !ok {
			return nil, errors.New("missing 'value' in key-value pair")
		}
	}
	return &WindowsRegistryValueSourceType{
		BaseSourceType: BaseSourceType{TypeIndicatorValue: TYPE_INDICATOR_WINDOWS_REGISTRY_VALUE},
		KeyValuePairs:  keyValuePairs,
	}, nil
}

func (w *WindowsRegistryValueSourceType) AsDict() map[string]interface{} {
	return map[string]interface{}{"key_value_pairs": w.KeyValuePairs}
}

// FileSourceType represents a file source.
type FileSourceType struct {
	BaseSourceType
	Paths     []string
	Separator string
}

func NewFileSourceType(paths []string, separator string) (*FileSourceType, error) {
	if len(paths) == 0 {
		return nil, errors.New("missing paths value")
	}
	if separator == "" {
		separator = "/"
	}
	return &FileSourceType{
		BaseSourceType: BaseSourceType{TypeIndicatorValue: TYPE_INDICATOR_FILE},
		Paths:          paths,
		Separator:      separator,
	}, nil
}

func (f *FileSourceType) AsDict() map[string]interface{} {
	result := map[string]interface{}{"paths": f.Paths}
	if f.Separator != "/" {
		result["separator"] = f.Separator
	}
	return result
}

// PathSourceType represents a path source.
type PathSourceType struct {
	BaseSourceType
	Paths     []string
	Separator string
}

func NewPathSourceType(paths []string, separator string) (*PathSourceType, error) {
	if len(paths) == 0 {
		return nil, errors.New("missing paths value")
	}
	if separator == "" {
		separator = "/"
	}
	return &PathSourceType{
		BaseSourceType: BaseSourceType{TypeIndicatorValue: TYPE_INDICATOR_PATH},
		Paths:          paths,
		Separator:      separator,
	}, nil
}

func (p *PathSourceType) AsDict() map[string]interface{} {
	result := map[string]interface{}{"paths": p.Paths}
	if p.Separator != "/" {
		result["separator"] = p.Separator
	}
	return result
}

//-----------------------------------------------------------------------------------------------------------------------------------------------------

// SourceTypeFactory is a factory for creating source types.
// Фабрика использует зарегистрированные конструкторы для создания объектов на основе типа.

//////////////////////////////////////// НЕ УВЕРЕН ЧТО ОТСУСТВИЕ ДАННОЙ ЧАСТИ В GO НЕ
// _source_type_classes = {
// 	definitions.TYPE_INDICATOR_ARTIFACT_GROUP: ArtifactGroupSourceType,
// 	definitions.TYPE_INDICATOR_COMMAND: CommandSourceType,
// 	definitions.TYPE_INDICATOR_DIRECTORY: DirectorySourceType,
// 	definitions.TYPE_INDICATOR_FILE: FileSourceType,
// 	definitions.TYPE_INDICATOR_PATH: PathSourceType,
// 	definitions.TYPE_INDICATOR_WINDOWS_REGISTRY_KEY:
// 		WindowsRegistryKeySourceType,
// 	definitions.TYPE_INDICATOR_WINDOWS_REGISTRY_VALUE:
// 		WindowsRegistryValueSourceType,
// 	definitions.TYPE_INDICATOR_WMI_QUERY: WMIQuerySourceType,
// }

// Структура Хранит зарегистрированные типы источников в виде словаря.
type SourceTypeFactory struct {
	sourceTypeConstructors map[string]func(map[string]interface{}) (SourceType, error)
}

func NewSourceTypeFactory() *SourceTypeFactory {
	return &SourceTypeFactory{
		sourceTypeConstructors: make(map[string]func(map[string]interface{}) (SourceType, error)),
	}
}

// Регистрирует новый тип источника в фабрике.
func (f *SourceTypeFactory) RegisterSourceType(typeIndicator string, constructor func(map[string]interface{}) (SourceType, error)) {
	f.sourceTypeConstructors[typeIndicator] = constructor
}

// Массовая регистрация типов.
func (f *SourceTypeFactory) RegisterSourceTypes(sourceTypes map[string]func(map[string]interface{}) (SourceType, error)) {
	for typeIndicator, constructor := range sourceTypes {
		f.RegisterSourceType(typeIndicator, constructor)
	}
}

// Удаляет тип из фабрики.
func (f *SourceTypeFactory) DeregisterSourceType(typeIndicator string) error {
	if _, exists := f.sourceTypeConstructors[typeIndicator]; !exists {
		return fmt.Errorf("source type not set for type: %s", typeIndicator)
	}
	delete(f.sourceTypeConstructors, typeIndicator)
	return nil
}

// Возвращает список всех зарегистрированных конструкторов
func (f *SourceTypeFactory) GetSourceTypes() []func(map[string]interface{}) (SourceType, error) {
	sourceTypes := []func(map[string]interface{}) (SourceType, error){}
	for _, constructor := range f.sourceTypeConstructors {
		sourceTypes = append(sourceTypes, constructor)
	}
	return sourceTypes
}

// Возвращает список идентификаторов зарегистрированных типов.
func (f *SourceTypeFactory) GetSourceTypeIndicators() []string {
	indicators := []string{}
	for indicator := range f.sourceTypeConstructors {
		indicators = append(indicators, indicator)
	}
	return indicators
}

// Создает объект источника данных на основе типа и атрибутов.
func (f *SourceTypeFactory) CreateSourceType(typeIndicator string, attributes map[string]interface{}) (SourceType, error) {
	constructor, exists := f.sourceTypeConstructors[typeIndicator]
	if !exists {
		return nil, fmt.Errorf("unsupported type indicator: %s", typeIndicator)
	}
	return constructor(attributes)
}
