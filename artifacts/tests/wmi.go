//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// wmiQueryPS выполняет WMI‑запрос через PowerShell и возвращает JSON‑строку.
func wmiQueryPS(query, namespace string) (string, error) {
	ps := fmt.Sprintf(
		"Get-CimInstance -Namespace '%s' -Query \"%s\" | ConvertTo-Json -Depth 3",
		namespace, query,
	)
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
		if bo := strings.TrimPrefix(q.BaseObject, "winmgmts:"); bo != "" {
			ns = strings.Trim(bo, `\ `)
		}

		raw, err := wmiQueryPS(q.Query, ns)
		if err != nil {
			logger.Log(LevelWarning, fmt.Sprintf("WMI PS query failed: %v", err))
			continue
		}

		// Если PowerShell вернул не массив, обернём в [ ... ]
		if !json.Valid([]byte(raw)) {
			logger.Log(LevelWarning, "WMI PS output is not valid JSON, skipping")
			continue
		}
		if !strings.HasPrefix(raw, "[") {
			raw = "[" + raw + "]"
		}

		// Сохраняем настоящий JSON‑массив
		output.AddCollectedWMI(q.Artifact, q.Query, json.RawMessage(raw))
	}
}
