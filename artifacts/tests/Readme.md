# fast_dfar

**Fast DFAR** (Digital Forensics Artifact Retriever) — кроссплатформенный инструмент для сбора артефактов с хоста (файлы, реестр, команды, WMI) и упаковки их в удобный формат.

## Возможности

- Чтение определений артефактов из YAML/JSON
- Фильтрация по Include/Exclude и поддерживаемым ОС
- Сбор файловой системы (включая NTFS‑потоки через TSK)
- Сбор значений реестра и WMI-запросов на Windows
- Вычисление хэшей (MD5, SHA1, SHA256)
- Ограничение по максимальному размеру файлов
- Вывод результатов в ZIP, JSONL и лог-файл

## Требования

- Go 1.20+
- Для TSK-подхода: Python 3, библиотека `pytsk3`
- Права администратора (для доступа к реестру и системным файлам)

## Установка

1. Клонировать репозиторий:
   ```bash
   git clone https://.../fast_dfar.git
   cd fast_dfar
   ```
2. Установить зависимости:
   ```bash
   go mod download
   ```
3. Сбилдить бинарь:
   ```bash
   go build -o fast_dfar .
   ```

## Запуск

> **Важно:** запускайте от имени администратора/root для доступа ко всем артефактам.

```bash
./fast_dfar \
  -include "Artifact1,Artifact2" \
  -exclude "UnwantedArtifact" \
  -directory "/path/to/custom/artifacts" \
  -maxsize 50M \
  -output ./results \
  -sha256
```

- `-include` — список артефактов или групп через запятую
- `-exclude` — исключаемые артефакты
- `-directory` — директории с вашими YAML/JSON определениями
- `-maxsize` — не собирать файлы больше этого размера (пример: `100M`)
- `-output` — папка для результатов
- `-sha256` — вычислять SHA-256 хеши в архиве

Результаты будут в папке: `<timestamp>-<hostname>`:
- `*-files.zip` — архив файлов
- `*-file_info.jsonl` — метаданные файлов
- `*-commands.json`, `*-registry.json`, `*-wmi.json`
- `*-logs.txt`

## Структура проекта

- `main.go` — разбор аргументов, инициация
- `reader.go` — чтение определений артефактов
- `artifacts.go` — модель ArtifactDefinition
- `collector.go` — логика регистрации и запуска Collectors
- `filesystem.go` — абстракция FS + OS/TSK реализации
- `output.go` — упаковка результатов
- `path_components.go` — генераторы путей (glob, recursion)
- `*_variables.go` — подстановка переменных для путей
- `tsk_helper.py` — Python‑скрипт для TSK

## Примеры

В каталоге `tests/data` есть пример YAML с определениями артефактов.

```yaml
- name: Hostname
  doc: Собрать имя хоста
  sources:
    - type: COMMAND
      attributes:
        cmd: hostname
        args: []
```

Запуск для тестовых данных:
```bash
./fast_dfar -directory ./tests/data -output ./out
```

## Лицензия

MIT © Your Name

