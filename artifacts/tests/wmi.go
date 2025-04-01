package main

import (
	"encoding/json"
	"fmt"
)

// ---------------------------------------------------------------------
// Реализация WMI-запроса и WMIExecutor (аналог Python-версии)
// ---------------------------------------------------------------------

// wmiQuery выполняет WMI-запрос и возвращает результат в виде JSON-строки.
// Если baseObject пустой, то используется значение по умолчанию.
// В реальной реализации здесь следует использовать вызовы COM или специализированную библиотеку.
func wmiQuery(query string, baseObject string) (string, error) {
	if baseObject == "" {
		baseObject = `winmgmts:\root\cimv2`
	}

	// Здесь должна быть логика выполнения WMI-запроса.
	// В Python-версии результаты перебираются, при этом пропуская COM-объекты.
	// В данном примере возвращается пустой JSON-массив.
	dummyResult := []map[string]interface{}{}
	resultBytes, err := json.Marshal(dummyResult)
	if err != nil {
		return "", err
	}
	return string(resultBytes), nil
}

// WMIQuery хранит параметры отдельного WMI-запроса.
type WMIQuery struct {
	Artifact   string
	Query      string
	BaseObject string
}

// WMIExecutor реализует интерфейс AbstractCollector для сбора данных WMI.
type WMIExecutor struct {
	queries []WMIQuery
}

func NewWMIExecutor() *WMIExecutor {
	return &WMIExecutor{
		queries: make([]WMIQuery, 0),
	}
}

func (w *WMIExecutor) Collect(output *Outputs) {
	for _, q := range w.queries {
		result, err := wmiQuery(q.Query, q.BaseObject)
		if err != nil {
			logger.Log(LevelWarning, fmt.Sprintf("WMI query failed: %s", q.Query))
			continue
		}
		output.AddCollectedWMI(q.Artifact, q.Query, result)
	}
}

// addQuery добавляет новый WMI-запрос в список.
func (w *WMIExecutor) addQuery(artifact, query, baseObject string) {
	w.queries = append(w.queries, WMIQuery{
		Artifact:   artifact,
		Query:      query,
		BaseObject: baseObject,
	})
}

// RegisterSource пытается зарегистрировать источник данных для артефакта.
// Если тип источника соответствует WMI-запросу, производится подстановка переменных
// и добавление запроса в список, после чего возвращается true.
// RegisterSource пытается зарегистрировать источник данных для артефакта.
// Если тип источника соответствует WMI-запросу, выполняется подстановка переменных
// и запрос добавляется в список.
func (w *WMIExecutor) RegisterSource(artifactDefinition *ArtifactDefinition, artifactSource *Source, variables *HostVariables) bool {
	if artifactSource.TypeIndicator == TYPE_INDICATOR_WMI_QUERY {
		// Извлекаем атрибут query из Attributes.
		queryAttr, ok := artifactSource.Attributes["query"]
		if !ok {
			logger.Log(LevelError, "WMI query attribute not found")
			return false
		}
		// Извлекаем base_object, если указан.
		baseObject := ""
		if bo, ok := artifactSource.Attributes["base_object"]; ok {
			if s, ok := bo.(string); ok {
				baseObject = s
			}
		}

		// Обрабатываем значение атрибута query, которое может быть строкой или срезом.
		switch q := queryAttr.(type) {
		case string:
			// Применяем подстановку переменных.
			for _, sub := range variables.Substitute(q) {
				// Преобразуем sub в строку без type assertion.
				w.addQuery(artifactDefinition.Name, fmt.Sprintf("%v", sub), baseObject)
			}
		case []interface{}:
			for _, qi := range q {
				if qs, ok := qi.(string); ok {
					for _, sub := range variables.Substitute(qs) {
						w.addQuery(artifactDefinition.Name, fmt.Sprintf("%v", sub), baseObject)
					}
				} else {
					logger.Log(LevelWarning, "WMI query attribute contains a non-string value")
				}
			}
		default:
			logger.Log(LevelError, "WMI query attribute has unsupported type")
			return false
		}
		return true
	}
	return false
}
