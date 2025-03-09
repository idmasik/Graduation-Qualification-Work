package main

import (
	"os"
	"os/user"
	"strings"

	"golang.org/x/sys/windows/registry"

	"github.com/StackExchange/wmi"
)

// reg выполняет запрос к реестру: открывает ключ, пытается получить значение valueName.
// Если значение не найдено и задан альтернативный параметр, то пробует alternativeValue.
func reg(hive registry.Key, keyPath string, valueName string, alternative ...string) (string, error) {
	// Используем флаги READ и WOW64_64KEY
	k, err := registry.OpenKey(hive, keyPath, registry.READ|registry.WOW64_64KEY)
	if err != nil {
		return "", err
	}
	defer k.Close()

	val, _, err := k.GetStringValue(valueName)
	if err != nil && len(alternative) > 0 {
		// пробуем альтернативное значение
		val, _, err = k.GetStringValue(alternative[0])
	}
	return val, err
}

// Win32Account – структура для WMI-запроса
type Win32Account struct {
	Name string
	SID  string
}

// getLocalUsers выполняет WMI-запрос для получения локальных пользователей
func getLocalUsers() ([]Win32Account, error) {
	var accounts []Win32Account
	query := "SELECT Name, SID FROM Win32_Account WHERE SidType = 1 AND LocalAccount = True"
	err := wmi.Query(query, &accounts)
	return accounts, err
}

// getExtraSids возвращает список SID из HKEY_USERS, отфильтровывая _Classes и .DEFAULT.
func getExtraSids() ([]string, error) {
	k := registry.USERS
	names, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}
	var sids []string
	for _, name := range names {
		if strings.Contains(name, "_Classes") || name == ".DEFAULT" {
			continue
		}
		sids = append(sids, name)
	}
	return sids, nil
}

// getUserProfiles получает пути профилей пользователей из реестра
func getUserProfiles() ([]string, error) {
	keyPath := `SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList`
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.READ)
	if err != nil {
		return nil, err
	}
	defer k.Close()

	subkeys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}
	var profiles []string
	for _, subkey := range subkeys {
		sk, err := registry.OpenKey(k, subkey, registry.READ)
		if err != nil {
			continue
		}
		profile, _, err := sk.GetStringValue("ProfileImagePath")
		sk.Close()
		if err == nil {
			profiles = append(profiles, profile)
		}
	}
	return profiles, nil
}

func windowsInitFunc(hv *HostVariables) {
	// Основные переменные
	hv.AddVariable("%%users.appdata%%", os.Getenv("APPDATA"))
	hv.AddVariable("%%users.localappdata%%", os.Getenv("LOCALAPPDATA"))
	hv.AddVariable("%%users.homedir%%", os.Getenv("USERPROFILE"))
	hv.AddVariable("%%environ_systemroot%%", os.Getenv("SystemRoot"))
	hv.AddVariable("%%environ_systemroot%%", os.Getenv("SYSTEMROOT"))
	hv.AddVariable("%%environ_allusersprofile%%", os.Getenv("ALLUSERSPROFILE"))
	hv.AddVariable("%%users.temp%%", os.Getenv("TEMP"))
	hv.AddVariable("%%environ_programdata%%", os.Getenv("ProgramData"))
	hv.AddVariable("%%users.userprofile%%", os.Getenv("USERPROFILE"))

	// Системные пути
	hv.AddVariable("%%environ_systemdrive%%", os.Getenv("SystemDrive"))           // Пример: C:
	hv.AddVariable("%%environ_allusersappdata%%", os.Getenv("ALLUSERSPROFILE"))   // Пример: C:\ProgramData
	hv.AddVariable("%%environ_windir%%", os.Getenv("SystemRoot"))                 // Пример: C:\Windows
	hv.AddVariable("%%environ_programfiles%%", os.Getenv("ProgramFiles"))         // Пример: C:\Program Files
	hv.AddVariable("%%environ_programfilesx86%%", os.Getenv("ProgramFiles(x86)")) // Пример: C:\Program Files (x86)

	// Дополнительные переменные
	hv.AddVariable("%%public%%", os.Getenv("PUBLIC"))
	hv.AddVariable("%%comspec%%", os.Getenv("ComSpec"))

	// Информация о пользователе
	if user, err := user.Current(); err == nil {
		hv.AddVariable("%%users.sid%%", user.Uid)
		hv.AddVariable("%%users.username%%", user.Username)
	}
}

// NewWindowsHostVariables создаёт и инициализирует HostVariables для Windows,
// используя функцию windowsInitFunc для загрузки переменных из реестра и WMI.
func NewWindowsHostVariables() *HostVariables {
	return NewHostVariables(windowsInitFunc)
}

// func main() {
// 	hv := NewHostVariables(windowsInitFunc)

// 	// Демонстрационный вывод переменных
// 	for _, variable := range hv.variables {
// 		for value := range variable.Value {
// 			fmt.Printf("%s = %s\n", variable.Name, value)
// 		}
// 	}
// }
