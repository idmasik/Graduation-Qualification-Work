package main

import (
	"log"
	"os"
)

// Уровни логирования (аналогичные Python)
const (
	LevelDebug    = 10
	LevelProgress = 25
	LevelInfo     = 30
	LevelWarning  = 40
	LevelError    = 50
	LevelCritical = 60
)

// Logger обёртка для кастомного логгера
type Logger struct {
	*log.Logger
	level int
}

// NewLogger создаёт новый экземпляр логгера
func NewLogger(prefix string, level int) *Logger {
	return &Logger{
		Logger: log.New(os.Stdout, prefix, log.LstdFlags),
		level:  level,
	}
}

// Log выводит сообщение, если его уровень достаточен
func (l *Logger) Log(level int, msg string) {
	if level >= l.level {
		levelName := "UNKNOWN"
		switch level {
		case LevelDebug:
			levelName = "DEBUG"
		case LevelProgress:
			levelName = "PROGRESS"
		case LevelInfo:
			levelName = "INFO"
		case LevelWarning:
			levelName = "WARNING"
		case LevelError:
			levelName = "ERROR"
		case LevelCritical:
			levelName = "CRITICAL"
		}
		l.Printf("[%s] %s", levelName, msg)
	}
}

// Создание глобального логгера
var logger = NewLogger("fastir: ", LevelDebug)

// func main() {
// 	// Пример использования
// 	logger.Log(LevelProgress, "Прогресс выполнения")
// 	logger.Log(LevelDebug, "Отладочная информация")
// }
