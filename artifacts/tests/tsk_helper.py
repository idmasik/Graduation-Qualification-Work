#!/usr/bin/env python3
import sys
import os
import json
import pytsk3

CHUNK_SIZE = 5 * 1024 * 1024

def get_device_and_mountpoint(path):
    # Для Windows: определяем устройство и точку монтирования
    if os.name == 'nt':
        drive, _ = os.path.splitdrive(path)
        if not drive:
            raise Exception("Не удалось определить букву диска для пути")
        device = r"\\.\%s:" % drive[0]
        mountpoint = drive + os.sep
        return device, mountpoint
    else:
        return None, "/"

def open_fs(device, mountpoint):
    # Открываем диск через pytsk3
    if os.name == 'nt':
        img_info = pytsk3.Img_Info(device)
    else:
        img_info = pytsk3.Img_Info(mountpoint)
    fs_info = pytsk3.FS_Info(img_info)
    return fs_info

def get_entry(fs_info, path, mountpoint):
    # Вычисляем относительный путь от точки монтирования
    if path.startswith(mountpoint):
        rel_path = path[len(mountpoint):].strip(os.sep)
    else:
        rel_path = path.strip(os.sep)
    parts = rel_path.split(os.sep) if rel_path else []
    directory = fs_info.open_dir(path="/")
    entry = None
    for part in parts:
        found = False
        for e in directory:
            if e.info.name.name in [b".", b".."]:
                continue
            name = e.info.name.name.decode("utf-8", errors="replace")
            if name.lower() == part.lower():
                entry = e
                found = True
                break
        if not found:
            raise Exception("Запись не найдена: " + part)
        if part != parts[-1]:
            directory = entry.as_directory()
    return entry

def cmd_get_root(mountpoint):
    device, mp = get_device_and_mountpoint(mountpoint)
    try:
        fs_info = open_fs(device, mp)
        # Не нужно выводить весь каталог, достаточно вернуть имя "root" и точку монтирования
        result = {"n": "root", "p": mp}
    except Exception as e:
        result = {"e": str(e)}
    print(json.dumps(result, separators=(',', ':')))

def cmd_list_directory(path):
    device, mp = get_device_and_mountpoint(path)
    try:
        fs_info = open_fs(device, mp)
        entry = get_entry(fs_info, path, mp)
        directory = entry.as_directory()
        entries = []
        for e in directory:
            if e.info.name.name in [b".", b".."]:
                continue
            name = e.info.name.name.decode("utf-8", errors="replace")
            full_path = os.path.join(path, name)
            meta = ""
            if e.info.meta:
                if e.info.meta.type == pytsk3.TSK_FS_META_TYPE_DIR:
                    meta = "D"
                elif e.info.meta.type == pytsk3.TSK_FS_META_TYPE_REG:
                    meta = "R"
                elif e.info.meta.type == pytsk3.TSK_FS_META_TYPE_LNK:
                    meta = "L"
            size = e.info.meta.size if e.info.meta else 0
            entries.append({"n": name, "p": full_path, "m": meta, "s": size})
        print(json.dumps(entries, separators=(',', ':')))
    except Exception as e:
        print("[]")

def cmd_get_size(path):
    device, mp = get_device_and_mountpoint(path)
    try:
        fs_info = open_fs(device, mp)
        entry = get_entry(fs_info, path, mp)
        size = entry.info.meta.size if entry.info.meta else 0
        print(str(size))
    except Exception as e:
        print("0")

def cmd_read_chunks(path, offset_str, chunk_size_str):
    offset = int(offset_str)
    chunk_size = int(chunk_size_str)
    device, mp = get_device_and_mountpoint(path)
    try:
        fs_info = open_fs(device, mp)
        entry = get_entry(fs_info, path, mp)
    except Exception as e:
        print("")
        return
    size = entry.info.meta.size if entry.info.meta else 0
    if offset >= size:
        print("")
        return
    try:
        data = entry.read_random(offset, chunk_size)
        print(data.hex())
    except Exception as e:
        print("")

def cmd_follow_symlink(parent_path, link_name):
    print(os.path.join(parent_path, link_name))

def cmd_batch_collect():
    # Новая команда для пакетного сбора защищённых файлов
    input_data = sys.stdin.read()
    try:
        paths = json.loads(input_data)  # ожидается список путей
    except Exception as e:
        print(json.dumps({"error": str(e)}))
        sys.exit(1)
    results = {}
    for path in paths:
        try:
            device, mp = get_device_and_mountpoint(path)
            fs_info = open_fs(device, mp)
            entry = get_entry(fs_info, path, mp)
            size = entry.info.meta.size if entry.info.meta else 0
            # Читаем весь файл за один вызов (при необходимости можно реализовать чтение по чанкам)
            data = entry.read_random(0, size)
            results[path] = data.hex()
        except Exception as e:
            results[path] = ""
    print(json.dumps(results, separators=(',', ':')))

def main():
    if len(sys.argv) < 2:
        print("No command provided")
        sys.exit(1)
    command = sys.argv[1]
    if command == "get_root":
        if len(sys.argv) < 3:
            print("Missing mountpoint")
            sys.exit(1)
        mountpoint = sys.argv[2]
        cmd_get_root(mountpoint)
    elif command == "list_directory":
        if len(sys.argv) < 3:
            print("Missing path")
            sys.exit(1)
        path = sys.argv[2]
        cmd_list_directory(path)
    elif command == "get_size":
        if len(sys.argv) < 3:
            print("Missing path")
            sys.exit(1)
        path = sys.argv[2]
        cmd_get_size(path)
    elif command == "read_chunks":
        if len(sys.argv) < 5:
            print("Missing arguments for read_chunks")
            sys.exit(1)
        path = sys.argv[2]
        offset_str = sys.argv[3]
        chunk_size_str = sys.argv[4]
        cmd_read_chunks(path, offset_str, chunk_size_str)
    elif command == "follow_symlink":
        if len(sys.argv) < 4:
            print("Missing arguments for follow_symlink")
            sys.exit(1)
        parent_path = sys.argv[2]
        link_name = sys.argv[3]
        cmd_follow_symlink(parent_path, link_name)
    elif command == "batch_collect":
        cmd_batch_collect()
    else:
        print(f"Unknown command: {command}")
        sys.exit(1)

if __name__ == "__main__":
    main()
