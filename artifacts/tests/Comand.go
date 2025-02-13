package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type commandEntry struct {
	artifact string
	cmd      string
	args     []string
}

type CommandExecutor struct {
	commands []commandEntry
}

func NewCommandExecutor() *CommandExecutor {
	return &CommandExecutor{
		commands: make([]commandEntry, 0),
	}
}

func (c *CommandExecutor) AddCommand(artifact, cmd string, args []string) {
	c.commands = append(c.commands, commandEntry{
		artifact: artifact,
		cmd:      cmd,
		args:     args,
	})
}

func (c *CommandExecutor) Collect(output io.WriteCloser) {
	for _, entry := range c.commands {
		fullCmdArgs := append([]string{entry.cmd}, entry.args...)
		fullCmdStr := strings.Join(fullCmdArgs, " ")

		cmd := exec.Command(fullCmdArgs[0], fullCmdArgs[1:]...)
		outputBytes, err := cmd.CombinedOutput()

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				logger.Log(LevelWarning, fmt.Sprintf("Command '%s' for artifact '%s' returned error code '%d'",
					fullCmdStr, entry.artifact, exitErr.ExitCode()))
			} else if os.IsNotExist(err) {
				logger.Log(LevelWarning, fmt.Sprintf("Command '%s' for artifact '%s' could not be found",
					entry.cmd, entry.artifact))
				outputBytes = []byte{}
			} else {
				logger.Log(LevelWarning, fmt.Sprintf("Error executing command '%s': %v", fullCmdStr, err))
				outputBytes = []byte{}
			}
		}

		outputLine := fmt.Sprintf("Artifact: %s\nCommand: %s\nOutput:\n%s\n\n",
			entry.artifact, fullCmdStr, string(outputBytes))
		output.Write([]byte(outputLine))
	}
}

////пока сыро

// func (c *CommandExecutor) RegisterSource(artifactDefinition *ArtifactDefinition, artifactSource *Source, variables *HostVariables) bool {
// 	if artifactSource.TypeIndicator == TYPE_INDICATOR_COMMAND {
// 		c.AddCommand(artifactDefinition.Name, artifactSource.Cmd, artifactSource.Args)
// 		return true
// 	}
// 	return false
// }
