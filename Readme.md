# fast_dfar

**Fast DFAR** (Digital Forensics Artifact Retriever) — кроссплатформенный инструмент для сбора артефактов с хоста и интеграции с Kaspersky OpenTIP. Поддерживает сбор файлов, реестра, команд и WMI-запросов с последующим анализом угроз.

## Возможности
- поддержка гибкой настройки через CLI и конфигурационных файлов: позволяет выбирать артефакты для сбора или исключения, ограничивать размер файлов, вычислять хеш-суммы файлов, использовать пользовательские YAML-определения артефактов;
- интеграция с Digital Forensics Artifact Repository: использует общедоступные YAML-определения артефактов из репозитория ForensicArtifacts, что позволяет автоматически обновлять набор собираемых данных;
- cбор артефактов: инструмент фокусируется на сборе данных, необходимых для расследования инцидента, таких как системные файлы, ключи реестра, результаты команд и WMI-запросов;
- поддержка кроссплатформенности: работает на Windows, Unix;
- формирование ZIP-архива с артефактами, сохраняющего исходную структуру файловой системы;
- вычисление хеш-сумм собираемых артефактов;
- предоставление первичного анализа: инструмент анализирует исполняемые файлы (MIME-типы application/x-msdownload), сверяя их хеш-суммы в облачой системе Kaspersky Threat Intelligence Portal, предоставляющий доступ к информации о киберугрозах, что позволяет оперативно определить, являются ли они вредоносными, и получить информацию о классификации и дате последнего обнаружения;
- использование собственной системы журналирования: осуществляет журналирование в текстовом формате для фиксации ошибок и контроля хода выполнения работы программы.



## Требования

- Go 1.23+`


## Установка

1. Клонировать репозиторий:
   ```bash
   git clone https://.../fast_dfar.git
   cd fast_dfar
   ```
2. Сбилдить бинарь:
   ```bash
   go build -o fast_dfar .
   ```


## Быстрый запуск

В репозитории уже имеются готовые исполняемые файлы для Windows (fast_dfar_win.exe) и Linux (fast_dfar_unix). 
Перед запуском следует отредактировать файл конфигурации artifacts.ini.
> **Важно:** запускайте от имени администратора (root) для доступа ко всем артефактам.

или

 ```bash
 cd artifacts
 go run .
 ```

## Запуск

```bash
./fast_dfar \
  -include "Artifact1,Artifact2" \
  -exclude "UnwantedArtifact" \
  -directory "/path/to/custom/artifacts" \
  -registry true \
  -maxsize "50M" \
  -output ./results \
  -analysis true \
  -apikey "YOUR_KASPERSKY_API_KEY" \
  -sha256 true
```

- `-include` — список артефактов или групп через запятую
- `-exclude` — исключаемые артефакты
- `-directory` — директории с вашими YAML/JSON определениями
- `-maxsize` — не собирать файлы больше этого размера
- `-analysis`— активировать анализ через Kaspersky OpenTIP
- `-apikey`— API-ключ для Kaspersky Threat Intelligence
- `-output` — папка для результатов
- `-sha256` — вычислять SHA-256 хеши в архиве

Результаты будут в папке: `<timestamp>-<hostname>`:
- `*-files.zip` — архив c собраными артефактами
- `*-file_info.jsonl` — метаданные файлов
- `*-commands.json`, `*-registry.json`, `*-wmi.json`
- `*-logs.txt` - журнал событий работы программы
- `*-analysis.jsonl` - результаты проверки хешей

## Структура проекта

- `main.go` — инициализация, разбор CLI-аргументов, координация модулей
- `reader.go` — чтение определений артефактов
- `artifacts.go` —  определение структуры артефактов 
- `registry.go` — логика регистрации определений артефактов
- `collector.go` — определение и запуск сборщиков
- `filesystem.go` — абстракция FS + OS/NTFS реализации
- `output.go` — упаковка результатов
- `path_components.go` — генераторы путей (glob, recursion)
- `*_variables.go` — подстановка переменных для путей
- `analysis.go` — взаимодействие с Kaspersky OpenTIP
- `commands.go` — выполнение системных команд
- `defenition.go` — константы и определения типов
- `file_info.go` и `file_info_win.go` — сбор метаданных файлов
- `helper.go` — вспомогательные функции
- `logging.go` — система логирования
- `source_type.go` — фабрика типов источников
- `wmi.go` — сбор данных через WMI (Windows)
- `win_registry.go` — работа с реестром Windows


##  YAML-определения артефактов

В каталоге `data` находяться YAML-определениям артефактов.

Пример:
```yaml
name: System32 Metadata
doc: Metadata about dll and exe files in System32.
sources:
- type: FILE_INFO
  attributes:
    paths:
    - '%%environ_systemroot%%\System32\*.dll'
    - '%%environ_systemroot%%\System32\*.exe'
    - '%%environ_systemroot%%\System32\**\*.dll'
    - '%%environ_systemroot%%\System32\**\*.exe'
    separator: '\'
supported_os: [Windows]
```