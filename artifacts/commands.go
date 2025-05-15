package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type Command struct {
	Artifact string
	Cmd      string
	Args     []string
}

type CommandExecutor struct {
	commands []Command
}

func NewCommandExecutor() *CommandExecutor {
	return &CommandExecutor{commands: make([]Command, 0)}
}

func (c *CommandExecutor) AddCommand(artifact, cmd string, args []string) {
	c.commands = append(c.commands, Command{Artifact: artifact, Cmd: cmd, Args: args})
}

func (c *CommandExecutor) Collect(output *Outputs) {
	if len(c.commands) == 0 {
		logger.Log(LevelDebug, "No commands to execute")
		return
	}
	logger.Log(LevelInfo, fmt.Sprintf("Executing %d commands...", len(c.commands)))

	for _, cm := range c.commands {
		full := append([]string{cm.Cmd}, cm.Args...)
		fullCmdStr := strings.Join(full, " ")
		// Если на Windows и команда выглядит как Unix-путь — пропускаем
		if runtime.GOOS == "windows" && strings.Contains(cm.Cmd, "/") {
			logger.Log(LevelDebug,
				fmt.Sprintf("Skipping Unix-style command on Windows: %s", fullCmdStr))
			continue
		}

		logger.Log(LevelDebug, "Executing: "+fullCmdStr)
		var raw []byte
		var err error
		if runtime.GOOS == "windows" {
			// через cmd.exe, чтобы поддержать внутренние команды
			ps := "chcp 65001>nul && " + fullCmdStr
			raw, err = exec.Command("cmd", "/C", ps).CombinedOutput()
		} else {
			raw, err = exec.Command(cm.Cmd, cm.Args...).CombinedOutput()
		}

		if err != nil {
			var exitErr *exec.ExitError
			var execErr *exec.Error
			if errors.As(err, &exitErr) {
				logger.Log(LevelWarning, fmt.Sprintf(
					"Command '%s' for artifact '%s' returned code %d",
					fullCmdStr, cm.Artifact, exitErr.ExitCode(),
				))
			} else if errors.As(err, &execErr) && execErr.Err == exec.ErrNotFound {
				logger.Log(LevelWarning, fmt.Sprintf(
					"Command '%s' for artifact '%s' not found",
					cm.Cmd, cm.Artifact,
				))
			} else {
				logger.Log(LevelWarning, fmt.Sprintf(
					"Command '%s' for artifact '%s' failed: %v",
					fullCmdStr, cm.Artifact, err,
				))
			}
		}

		decoded := decodeOutput(raw)
		if strings.TrimSpace(decoded) == "" {
			continue
		}
		output.AddCollectedCommand(cm.Artifact, fullCmdStr, []byte(decoded))
	}
}

// decodeOutput приводит байты к UTF-8
func decodeOutput(raw []byte) string {
	if len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE {
		dec := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder()
		rdr := transform.NewReader(bytes.NewReader(raw), dec)
		if out, err := io.ReadAll(rdr); err == nil {
			return string(out)
		}
	}
	// OEM → UTF-8 на Windows
	if runtime.GOOS == "windows" {
		if out, err := charmap.CodePage866.NewDecoder().Bytes(raw); err == nil {
			return string(out)
		}
		if out, err := charmap.CodePage437.NewDecoder().Bytes(raw); err == nil {
			return string(out)
		}
	}
	return string(raw)
}

func (c *CommandExecutor) RegisterSource(def *ArtifactDefinition, src *Source, vars *HostVariables) bool {
	if src.TypeIndicator != TYPE_INDICATOR_COMMAND {
		return false
	}
	// проверим supported_os у source
	if v, ok := src.Attributes["supported_os"].([]interface{}); ok {
		supported := false
		for _, o := range v {
			if osStr, ok := o.(string); ok && strings.EqualFold(osStr, runtime.GOOS) {
				supported = true
				break
			}
		}
		if !supported {
			logger.Log(LevelDebug,
				fmt.Sprintf("Skipping command for artifact '%s': OS %s not in %v",
					def.Name, runtime.GOOS, v))
			return true
		}
	}

	cmd, ok := src.Attributes["cmd"].(string)
	if !ok {
		return false
	}
	var args []string
	switch v := src.Attributes["args"].(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				args = append(args, s)
			}
		}
	case []string:
		args = v
	case string:
		args = strings.Fields(v)
	}
	if len(args) == 0 {
		return false
	}
	c.AddCommand(def.Name, cmd, args)
	return true
}
