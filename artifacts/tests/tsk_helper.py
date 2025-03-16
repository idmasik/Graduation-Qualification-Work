#!/usr/bin/env python3
import sys
import os
import json
import re
import pytsk3

CHUNK_SIZE = 5 * 1024 * 1024
PATH_RECURSION_REGEX = re.compile(r"\*\*(?P<max_depth>(-1|\d*))")
PATH_GLOB_REGEX = re.compile(r"\*|\?|\[.+\]")

def get_device_and_mountpoint(path):
    """
    Для Windows: если путь начинается с 'C:\', возвращает device вида '\\.\C:' и mountpoint 'C:\'
    Для Unix: возвращает (None, "/")
    """
    if os.name == 'nt':
        drive, _ = os.path.splitdrive(path)
        if not drive:
            raise Exception("Не удалось определить букву диска для пути")
        device = r"\\.\%s:" % drive[0]
        mountpoint = drive + os.sep
        return device, mountpoint
    else:
        return None, "/"

def open_fs(path):
    """
    Открывает файловую систему с использованием pytsk3.
    """
    device, mountpoint = get_device_and_mountpoint(path)
    if os.name == 'nt':
        img_info = pytsk3.Img_Info(device)
    else:
        img_info = pytsk3.Img_Info(path)
    fs_info = pytsk3.FS_Info(img_info)
    return fs_info, mountpoint

def get_root(mountpoint):
    """
    Возвращает корневой каталог.
    """
    # Для простоты возвращаем mountpoint как корневой путь.
    result = {"name": "root", "path": mountpoint}
    print(json.dumps(result))

def list_directory(path):
    fs_info, mountpoint = open_fs(path)
    try:
        directory = fs_info.open_dir(path="/")
    except Exception as e:
        print(json.dumps([]))
        return
    entries = []
    for entry in directory:
        if entry.info.name.name in [b".", b".."]:
            continue
        name = entry.info.name.name.decode("utf-8", errors="replace")
        full_path = os.path.join(path, name)
        meta_type = ""
        if entry.info.meta:
            if entry.info.meta.type == pytsk3.TSK_FS_META_TYPE_DIR:
                meta_type = "DIR"
            elif entry.info.meta.type == pytsk3.TSK_FS_META_TYPE_REG:
                meta_type = "REG"
            elif entry.info.meta.type == pytsk3.TSK_FS_META_TYPE_LNK:
                meta_type = "LNK"
        size = entry.info.meta.size if entry.info.meta else 0
        entries.append({
            "name": name,
            "path": full_path,
            "meta_type": meta_type,
            "size": size
        })
    print(json.dumps(entries))

def is_directory(path):
    fs_info, mountpoint = open_fs(path)
    try:
        directory = fs_info.open_dir(path=path)
    except Exception:
        print("false")
        return
    # Если удалось открыть каталог, считаем его директорией.
    print("true")

def is_file(path):
    fs_info, mountpoint = open_fs(path)
    try:
        file_entry = fs_info.open_dir(path=path)
        # Если open_dir с файлом не выбрасывает исключение, считаем, что это файл.
        print("false")
    except IOError:
        print("true")

def is_symlink(path):
    fs_info, mountpoint = open_fs(path)
    try:
        entry = fs_info.open_dir(path=path)
    except Exception:
        print("false")
        return
    if entry.info.meta and entry.info.meta.type == pytsk3.TSK_FS_META_TYPE_LNK:
        print("true")
    else:
        print("false")

def get_size(path):
    fs_info, mountpoint = open_fs(path)
    try:
        entry = fs_info.open_dir(path=path)
    except Exception:
        print("0")
        return
    size = entry.info.meta.size if entry.info.meta else 0
    print(str(size))

def read_chunks(path, offset_str, chunk_size_str):
    offset = int(offset_str)
    chunk_size = int(chunk_size_str)
    fs_info, mountpoint = open_fs(path)
    try:
        entry = fs_info.open_dir(path=path)
    except Exception:
        print("")
        return
    size = entry.info.meta.size if entry.info.meta else 0
    if offset >= size:
        print("")
        return
    try:
        data = entry.read_random(offset, chunk_size)
    except Exception as e:
        print("")
        return
    # Возвращаем данные в hex-формате
    print(data.hex())

def follow_symlink(parent_path, link_name):
    # Простой вариант: возвращаем составной путь
    result = os.path.join(parent_path, link_name)
    print(result)

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
        get_root(mountpoint)
    elif command == "list_directory":
        if len(sys.argv) < 3:
            print("Missing path")
            sys.exit(1)
        path = sys.argv[2]
        list_directory(path)
    elif command == "is_directory":
        if len(sys.argv) < 3:
            print("Missing path")
            sys.exit(1)
        path = sys.argv[2]
        is_directory(path)
    elif command == "is_file":
        if len(sys.argv) < 3:
            print("Missing path")
            sys.exit(1)
        path = sys.argv[2]
        is_file(path)
    elif command == "is_symlink":
        if len(sys.argv) < 3:
            print("Missing path")
            sys.exit(1)
        path = sys.argv[2]
        is_symlink(path)
    elif command == "get_size":
        if len(sys.argv) < 3:
            print("Missing path")
            sys.exit(1)
        path = sys.argv[2]
        get_size(path)
    elif command == "read_chunks":
        if len(sys.argv) < 5:
            print("Missing arguments for read_chunks")
            sys.exit(1)
        path = sys.argv[2]
        offset_str = sys.argv[3]
        chunk_size_str = sys.argv[4]
        read_chunks(path, offset_str, chunk_size_str)
    elif command == "follow_symlink":
        if len(sys.argv) < 4:
            print("Missing arguments for follow_symlink")
            sys.exit(1)
        parent_path = sys.argv[2]
        link_name = sys.argv[3]
        follow_symlink(parent_path, link_name)
    else:
        print(f"Unknown command: {command}")
        sys.exit(1)

if __name__ == "__main__":
    main()
