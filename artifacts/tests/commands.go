package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
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
	return &CommandExecutor{
		commands: make([]Command, 0),
	}
}

func (c *CommandExecutor) AddCommand(artifact, cmd string, args []string) {
	c.commands = append(c.commands, Command{
		Artifact: artifact,
		Cmd:      cmd,
		Args:     args,
	})
}

func (c *CommandExecutor) Collect(output *Outputs) {
	if len(c.commands) == 0 {
		logger.Log(LevelDebug, "No commands to execute")
		return
	}

	logger.Log(LevelInfo, fmt.Sprintf("Executing %d commands...", len(c.commands)))
	for _, cmd := range c.commands {
		logger.Log(LevelDebug, fmt.Sprintf("Executing: %s %v", cmd.Cmd, cmd.Args))
		fullCmd := append([]string{cmd.Cmd}, cmd.Args...)
		fullCmdStr := strings.Join(fullCmd, " ")

		cmdOutput, err := exec.Command(cmd.Cmd, cmd.Args...).CombinedOutput()
		if err != nil {
			var exitErr *exec.ExitError
			var execErr *exec.Error
			if errors.As(err, &exitErr) {
				logger.Log(LevelWarning, fmt.Sprintf("Command '%s' for artifact '%s' returned error code '%d'",
					fullCmdStr, cmd.Artifact, exitErr.ExitCode()))
			} else if errors.As(err, &execErr) && execErr.Err == exec.ErrNotFound {
				logger.Log(LevelWarning, fmt.Sprintf("Command '%s' for artifact '%s' could not be found",
					cmd.Cmd, cmd.Artifact))
				cmdOutput = []byte{}
			} else {
				logger.Log(LevelWarning, fmt.Sprintf("Command '%s' for artifact '%s' failed: %v",
					fullCmdStr, cmd.Artifact, err))
				cmdOutput = []byte{}
			}
		}

		output.AddCollectedCommand(cmd.Artifact, fullCmdStr, cmdOutput)
	}
}

func (c *CommandExecutor) RegisterSource(artifactDefinition *ArtifactDefinition, artifactSource *Source, variables *HostVariables) bool {
	if artifactSource.TypeIndicator == TYPE_INDICATOR_COMMAND {
		cmd, ok1 := artifactSource.Attributes["cmd"].(string)
		var args []string
		switch v := artifactSource.Attributes["args"].(type) {
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					args = append(args, s)
				}
			}
		case []string:
			args = v
		case string:
			// Если аргументы передаются как строка, разделяем по пробелам
			args = strings.Fields(v)
		}
		if ok1 && len(args) > 0 {
			c.AddCommand(artifactDefinition.Name, cmd, args)
			return true
		}
	}
	return false
}
