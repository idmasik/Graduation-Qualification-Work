package main

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func commandArtifact(name, cmd string, args []string) *ArtifactDefinition {
	return &ArtifactDefinition{
		Name: name,
		Sources: []*Source{
			{
				TypeIndicator: TYPE_INDICATOR_COMMAND,
				Attributes: map[string]interface{}{
					"cmd":  cmd,
					"args": args,
				},
			},
		},
	}
}

func TestCommandExecution(t *testing.T) {
	// Создаём временную директорию
	dir, err := os.MkdirTemp("", "fastir-test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	// Инициализируем outputs
	outputs, err := NewOutputs(dir, "0", false)
	assert.NoError(t, err)
	defer outputs.Close()

	// Платформенно-зависимая настройка команды
	var cmd string
	var args []string
	var expectedOutput string

	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/C", "echo test"}
		expectedOutput = "test\r\n"
	} else {
		cmd = "echo"
		args = []string{"test"}
		expectedOutput = "test\n"
	}

	// Создаём collector и artifact
	collector := NewCommandExecutor()
	artifact := commandArtifact("TestArtifact", cmd, args)

	// Регистрируем источник
	registered := collector.RegisterSource(artifact, artifact.Sources[0], nil)
	assert.True(t, registered)

	// Выполняем сбор данных
	collector.Collect(outputs)

	// Проверяем результаты
	commands := outputs.GetCommands()["TestArtifact"]
	assert.NotNil(t, commands)

	fullCmd := strings.Join(append([]string{cmd}, args...), " ")
	result := commands[fullCmd]

	assert.Contains(t, result, expectedOutput,
		"Command output should contain expected text")
}

func TestUnknownCommand(t *testing.T) {
	// Создаём временную директорию
	dir, err := os.MkdirTemp("", "fastir-test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	// Инициализируем outputs
	outputs, err := NewOutputs(dir, "0", false)
	assert.NoError(t, err)
	defer outputs.Close()

	// Перенаправляем вывод логгера
	var logBuffer bytes.Buffer
	oldLogger := logger
	logger = NewLogger("fastir: ", LevelWarning)
	logger.SetOutput(&logBuffer)
	defer func() { logger = oldLogger }()

	// Создаём collector и artifact
	collector := NewCommandExecutor()
	artifact := commandArtifact("TestArtifact", "nonexistent-command", []string{})

	// Регистрируем источник
	registered := collector.RegisterSource(artifact, artifact.Sources[0], nil)
	assert.True(t, registered)

	// Выполняем сбор данных
	collector.Collect(outputs)

	// Проверяем результаты выполнения команды
	commands := outputs.GetCommands()["TestArtifact"]
	assert.NotNil(t, commands)

	fullCmd := "nonexistent-command"
	result := commands[fullCmd]
	assert.Empty(t, result, "Command output should be empty")

	// Проверяем логи
	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "WARNING", "Log should contain warning")
	assert.Contains(t, logOutput,
		"Command 'nonexistent-command' for artifact 'TestArtifact' could not be found",
		"Log should contain specific warning message")
}

// Дополнительные функции для совместимости с тестами
func (o *Outputs) GetCommands() map[string]map[string]string {
	return o.commands
}

// func (o *Outputs) GetWMI() map[string]map[string]string {
// 	return o.wmi
// }

func (o *Outputs) GetRegistry() map[string]map[string]map[string]interface{} {
	return o.registry
}
