package main

import (
	"errors"
	"fmt"
	"os"
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
	for _, cmd := range c.commands {
		fullCmd := append([]string{cmd.Cmd}, cmd.Args...)
		fullCmdStr := strings.Join(fullCmd, " ")

		command := exec.Command(cmd.Cmd, cmd.Args...)
		cmdOutput, err := command.CombinedOutput()
		if err != nil {
			var exitErr *exec.ExitError
			var execErr *exec.Error

			switch {
			case errors.As(err, &exitErr):
				logger.Log(LevelWarning, fmt.Sprintf("Command '%s' for artifact '%s' returned error code '%d'",
					fullCmdStr, cmd.Artifact, exitErr.ExitCode()))
			case errors.As(err, &execErr) && execErr.Err == os.ErrNotExist:
				logger.Log(LevelWarning, fmt.Sprintf("Command '%s' for artifact '%s' could not be found",
					cmd.Cmd, cmd.Artifact))
				cmdOutput = []byte{}
			default:
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
		args, ok2 := artifactSource.Attributes["args"].([]string)
		if ok1 && ok2 {
			c.AddCommand(artifactDefinition.Name, cmd, args)
			return true
		}
	}
	return false
}
