package main

import (
	"log"
	"path/filepath"
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

// windowsInitFunc инициализирует переменные хоста, выполняя запросы к реестру и WMI.
func windowsInitFunc(hv *HostVariables) {
	// Получаем SystemRoot
	systemroot, err := reg(registry.LOCAL_MACHINE, `Software\Microsoft\Windows NT\CurrentVersion`, "SystemRoot")
	if err != nil {
		log.Fatalf("Не удалось получить SystemRoot: %v", err)
	}
	hv.AddVariable("%systemroot%", systemroot)
	hv.AddVariable("%%environ_systemroot%%", systemroot)

	if len(systemroot) < 2 {
		log.Fatalf("Некорректное значение SystemRoot: %s", systemroot)
	}
	systemdrive := systemroot[:2]
	hv.AddVariable("%systemdrive%", systemdrive)
	hv.AddVariable("%%environ_systemdrive%%", systemdrive)

	// windir
	if windir, err := reg(registry.LOCAL_MACHINE, `System\CurrentControlSet\Control\Session Manager\Environment`, "windir"); err == nil {
		hv.AddVariable("%%environ_windir%%", windir)
	} else {
		log.Printf("Не удалось получить windir: %v", err)
	}

	// allusersappdata
	if allusersappdata, err := reg(registry.LOCAL_MACHINE, `Software\Microsoft\Windows NT\CurrentVersion\ProfileList`, "ProgramData"); err == nil {
		hv.AddVariable("%%environ_allusersappdata%%", allusersappdata)
	} else {
		log.Printf("Не удалось получить allusersappdata: %v", err)
	}

	// programfiles
	if programfiles, err := reg(registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion`, "ProgramFilesDir"); err == nil {
		hv.AddVariable("%%environ_programfiles%%", programfiles)
	} else {
		log.Printf("Не удалось получить programfiles: %v", err)
	}

	// programfilesx86 с альтернативным значением
	if programfilesx86, err := reg(registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion`, "ProgramFilesDir (x86)", "ProgramFilesDir"); err == nil {
		hv.AddVariable("%%environ_programfilesx86%%", programfilesx86)
	} else {
		log.Printf("Не удалось получить programfilesx86: %v", err)
	}

	// allusersprofile
	if allusersprofile, err := reg(registry.LOCAL_MACHINE, `Software\Microsoft\Windows NT\CurrentVersion\ProfileList`, "AllUsersProfile", "ProgramData"); err == nil {
		hv.AddVariable("%%environ_allusersprofile%%", allusersprofile)
	} else {
		log.Printf("Не удалось получить allusersprofile: %v", err)
	}

	// Local AppData и AppData из профиля .DEFAULT
	if localAppData, err := reg(registry.USERS, `.DEFAULT\Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`, "Local AppData"); err == nil {
		hv.AddVariable("%%users.localappdata%%", localAppData)
	} else {
		log.Printf("Не удалось получить Local AppData: %v", err)
	}
	if appData, err := reg(registry.USERS, `.DEFAULT\Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`, "AppData"); err == nil {
		hv.AddVariable("%%users.appdata%%", appData)
	} else {
		log.Printf("Не удалось получить AppData: %v", err)
	}

	// TEMP
	if temp, err := reg(registry.USERS, `.DEFAULT\Environment`, "TEMP"); err == nil {
		hv.AddVariable("%%users.temp%%", temp)
	} else {
		log.Printf("Не удалось получить TEMP: %v", err)
	}

	// LocalAppData Low – составляем путь, присоединяя RelativePath к %USERPROFILE%
	if localAppDataLowRel, err := reg(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\FolderDescriptions\{A520A1A4-1780-4FF6-BD18-167343C5AF16}`, "RelativePath"); err == nil {
		localAppDataLow := filepath.Join("%USERPROFILE%", localAppDataLowRel)
		hv.AddVariable("%%users.localappdata_low%%", localAppDataLow)
	} else {
		log.Printf("Не удалось получить RelativePath для LocalAppData Low: %v", err)
	}

	// Профили пользователей
	if userProfiles, err := getUserProfiles(); err == nil && len(userProfiles) > 0 {
		// Если несколько – объединяем через точку с запятой
		profilesJoined := strings.Join(userProfiles, ";")
		hv.AddVariable("%USERPROFILE%", profilesJoined)
		hv.AddVariable("%%users.homedir%%", profilesJoined)
		hv.AddVariable("%%users.userprofile%%", profilesJoined)
	} else {
		log.Printf("Не удалось получить профили пользователей: %v", err)
	}

	// Локальные пользователи и SID
	if users, err := getLocalUsers(); err == nil {
		var usernames []string
		var sids []string
		for _, user := range users {
			usernames = append(usernames, user.Name)
			sids = append(sids, user.SID)
		}
		// Добавляем имена пользователей
		hv.AddVariable("%%users.username%%", strings.Join(usernames, ";"))
		// Дополняем SID дополнительными SID
		if extraSids, err := getExtraSids(); err == nil {
			sids = append(sids, extraSids...)
		} else {
			log.Printf("Не удалось получить extra SIDs: %v", err)
		}
		hv.AddVariable("%%users.sid%%", strings.Join(sids, ";"))
	} else {
		log.Printf("Не удалось выполнить WMI-запрос локальных пользователей: %v", err)
	}
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
