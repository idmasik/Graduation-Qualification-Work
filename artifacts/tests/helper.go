package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func getOperatingSystem() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return "Linux", nil
	case "darwin":
		return "Darwin", nil
	case "windows":
		return "Windows", nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// func main() {
// 	os, err := getOperatingSystem()
// 	if err != nil {
// 		fmt.Println(err)
// 	} else {
// 		fmt.Println("Operating System:", os)
// 	}
// }

// enableBackupPrivilege запрашивает привилегию SeBackupPrivilege для текущего процесса.
func enableBackupPrivilege() error {
	var hToken windows.Token
	currentProcess, err := windows.GetCurrentProcess()
	if err != nil {
		return fmt.Errorf("не удалось получить текущий процесс: %v", err)
	}
	err = windows.OpenProcessToken(currentProcess, windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &hToken)
	if err != nil {
		return fmt.Errorf("не удалось открыть токен процесса: %v", err)
	}
	defer hToken.Close()

	var luid windows.LUID
	err = windows.LookupPrivilegeValue(nil, syscall.StringToUTF16Ptr("SeBackupPrivilege"), &luid)
	if err != nil {
		return fmt.Errorf("не удалось найти значение привилегии SeBackupPrivilege: %v", err)
	}

	tp := windows.Tokenprivileges{
		PrivilegeCount: 1,
		Privileges: [1]windows.LUIDAndAttributes{
			{
				Luid:       luid,
				Attributes: windows.SE_PRIVILEGE_ENABLED,
			},
		},
	}

	err = windows.AdjustTokenPrivileges(hToken, false, &tp, uint32(unsafe.Sizeof(tp)), nil, nil)
	if err != nil {
		return fmt.Errorf("не удалось изменить привилегии токена: %v", err)
	}

	// Отладка: проверьте, включилась ли привилегия
	logger.Log(LevelDebug, "SeBackupPrivilege успешно включена")
	return nil
}

// openFileWithBackupPrivilege открывает файл с правами резервного копирования,
// позволяя получить доступ к системным защищённым файлам.
func openFileWithBackupPrivilege(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		p,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS, // Позволяет открывать системные файлы/каталоги
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

var (
	modkernel32    = windows.NewLazySystemDLL("kernel32.dll")
	procBackupRead = modkernel32.NewProc("BackupRead")
	procBackupSeek = modkernel32.NewProc("BackupSeek")
)

// backupReadFile использует Windows‑API BackupRead для чтения содержимого защищённых файлов.
func backupReadFile(path string) ([]byte, error) {
	file, err := openFileWithBackupPrivilege(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var context uintptr = 0
	var total []byte
	buf := make([]byte, CHUNK_SIZE)

	for {
		var bytesRead uint32 = 0
		ret, _, callErr := procBackupRead.Call(
			file.Fd(), // HANDLE
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
			uintptr(unsafe.Pointer(&bytesRead)),
			0, // bAbort = FALSE
			0, // bProcessSecurity = FALSE
			uintptr(unsafe.Pointer(&context)),
		)
		if ret == 0 {
			// Если произошла ошибка, возвращаем её
			return nil, fmt.Errorf("BackupRead failed: %v", callErr)
		}
		if bytesRead == 0 {
			break
		}
		total = append(total, buf[:bytesRead]...)
	}

	// Завершаем чтение – вызываем BackupRead с флагом bAbort = TRUE.
	var dummy uint32
	procBackupRead.Call(
		file.Fd(),
		0,
		0,
		uintptr(unsafe.Pointer(&dummy)),
		1, // bAbort = TRUE
		0,
		uintptr(unsafe.Pointer(&context)),
	)

	return total, nil
}

// createShadowCopy с помощью DiskShadow создаёт теневую копию для указанного тома (например, "C:")
// и возвращает букву смонтированного тома (например, "X:").
// Обратите внимание, что для работы DiskShadow необходимо, чтобы утилита была доступна, и чтобы ее запуск выполнялся от имени администратора.
func createShadowCopy(volume string) (string, error) {
	// Создаем временный скрипт для DiskShadow
	scriptContent := fmt.Sprintf(`set context persistent nowriters
add volume %s alias MyShadow
create
expose %s X:`, volume, volume)

	tmpScript, err := os.CreateTemp("", "diskshadow_*.txt")
	if err != nil {
		return "", fmt.Errorf("не удалось создать временный файл для DiskShadow: %v", err)
	}
	scriptPath := tmpScript.Name()
	_, err = tmpScript.WriteString(scriptContent)
	tmpScript.Close()
	if err != nil {
		return "", fmt.Errorf("не удалось записать скрипт для DiskShadow: %v", err)
	}
	defer os.Remove(scriptPath)

	// Запускаем DiskShadow с созданным скриптом
	cmd := exec.Command("diskshadow.exe", "/s", scriptPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("diskshadow failed: %v, output: %s", err, out.String())
	}

	// Пример строки в выводе (на английском):
	// "Shadow copy exposed as X:"
	// Регулярное выражение ищет эту строку (без учета регистра)
	re := regexp.MustCompile(`(?i)Shadow copy exposed as ([A-Z]:)`)
	matches := re.FindStringSubmatch(out.String())
	if len(matches) < 2 {
		return "", fmt.Errorf("не удалось распарсить букву тома из вывода DiskShadow: %s", out.String())
	}
	exposedDrive := strings.TrimSpace(matches[1])
	return exposedDrive, nil
}
