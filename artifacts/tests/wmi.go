//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

func wmiQueryPS(query, namespace string) (string, error) {
	var ps string
	query = strings.TrimSpace(query)

	if strings.HasPrefix(strings.ToUpper(query), "SELECT ") {
		ps = fmt.Sprintf(
			"Get-CimInstance -Namespace '%s' -Query \"%s\" | ConvertTo-Json -Depth 3",
			namespace, query,
		)
	} else {
		ps = fmt.Sprintf(
			"Get-CimInstance -Namespace '%s' -ClassName '%s' | ConvertTo-Json -Depth 3",
			namespace, query,
		)
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command", ps)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("powershell failed: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

type WMIQuery struct {
	Artifact, Query, BaseObject string
}

type WMIExecutor struct {
	queries []WMIQuery
}

func NewWMIExecutor() *WMIExecutor {
	return &WMIExecutor{queries: make([]WMIQuery, 0)}
}

func (w *WMIExecutor) addQuery(artifact, query, baseObject string) {
	w.queries = append(w.queries, WMIQuery{artifact, query, baseObject})
}

func (w *WMIExecutor) RegisterSource(def *ArtifactDefinition, src *Source, vars *HostVariables) bool {
	if strings.ToUpper(src.TypeIndicator) != TYPE_INDICATOR_WMI {
		return false
	}
	qRaw, ok := src.Attributes["query"].(string)
	if !ok {
		return false
	}
	qRaw = strings.TrimSpace(qRaw)
	if qRaw == "" || strings.HasPrefix(qRaw, "{") {
		logger.Log(LevelWarning,
			fmt.Sprintf("Invalid or empty WMI query '%s' for artifact '%s', skipping", qRaw, def.Name))
		return true
	}

	base := ""
	if bo, ok := src.Attributes["base_object"].(string); ok {
		base = bo
	}

	if strings.Contains(qRaw, "%%") {
		for _, sub := range vars.Substitute(qRaw) {
			w.addQuery(def.Name, fmt.Sprintf("%v", sub), base)
		}
	} else {
		w.addQuery(def.Name, qRaw, base)
	}
	return true
}

func (w *WMIExecutor) Collect(output *Outputs) {
	for _, q := range w.queries {
		ns := "root\\cimv2"

		if q.BaseObject != "" {
			baseObj := q.BaseObject
			baseObj = strings.TrimPrefix(baseObj, "winmgmts:")
			baseObj = strings.TrimPrefix(baseObj, `\\.\`)
			baseObj = strings.Trim(baseObj, `\ `)
			if baseObj != "" {
				ns = baseObj
			}
		}

		query := strings.TrimSpace(q.Query)
		if query == "" || query == "{}" {
			logger.Log(LevelWarning,
				fmt.Sprintf("Empty or invalid WMI query for artifact '%s', skipping", q.Artifact))
			continue
		}

		execQuery := func(namespace string) (string, error) {
			logger.Log(LevelDebug,
				fmt.Sprintf("Executing WMI query for '%s' in namespace '%s': %s",
					q.Artifact, namespace, query))
			return wmiQueryPS(query, namespace)
		}

		raw, err := execQuery(ns)

		// Повторная попытка, если ошибка пространства имен и класс MSFT_*
		if err != nil && strings.Contains(query, "MSFT_") {
			retryNs := "root\\StandardCimv2"
			logger.Log(LevelInfo,
				fmt.Sprintf("Retrying WMI query for '%s' in namespace '%s'", q.Artifact, retryNs))
			raw, err = execQuery(retryNs)
		}

		if err != nil {
			logger.Log(LevelWarning,
				fmt.Sprintf("WMI PS query ultimately failed for artifact '%s': %v", q.Artifact, err))
			continue
		}

		if !json.Valid([]byte(raw)) {
			logger.Log(LevelWarning,
				fmt.Sprintf("WMI PS output is not valid JSON for artifact '%s', skipping", q.Artifact))
			continue
		}
		if !strings.HasPrefix(raw, "[") {
			raw = "[" + raw + "]"
		}

		output.AddCollectedWMI(q.Artifact, query, json.RawMessage(raw))
	}
}
