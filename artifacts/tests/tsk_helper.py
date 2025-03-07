#!/usr/bin/env python3
import sys
import json
import pytsk3
import os
import base64

def open_fs(image_path):
    try:
        img_info = pytsk3.Img_Info(image_path)
        fs_info = pytsk3.FS_Info(img_info)
        return fs_info
    except Exception as e:
        sys.stderr.write("Error opening FS: " + str(e))
        sys.exit(1)

def command_get_root(image_path):
    fs_info = open_fs(image_path)
    try:
        # Открываем корневой каталог (относительный путь пустой)
        root = fs_info.open_dir("")
    except Exception as e:
        sys.stderr.write("Error opening root directory: " + str(e))
        sys.exit(1)
    # Возвращаем JSON с информацией о корне (имя – "root", внутренний путь пустой)
    result = {"name": "root", "path": ""}
    print(json.dumps(result))

def command_is_directory(image_path, internal_path):
    fs_info = open_fs(image_path)
    try:
        entry = fs_info.open_dir(internal_path)
        meta_type = entry.info.meta.type
        # Сравниваем с типами каталога
        if meta_type in (pytsk3.TSK_FS_META_TYPE_DIR, pytsk3.TSK_FS_META_TYPE_VIRT_DIR):
            print("true")
        else:
            print("false")
    except Exception:
        print("false")

def command_is_file(image_path, internal_path):
    fs_info = open_fs(image_path)
    try:
        entry = fs_info.open(internal_path)
        meta_type = entry.info.meta.type
        if meta_type == pytsk3.TSK_FS_META_TYPE_REG:
            print("true")
        else:
            print("false")
    except Exception:
        print("false")

def command_is_symlink(image_path, internal_path):
    fs_info = open_fs(image_path)
    try:
        entry = fs_info.open(internal_path)
        meta_type = entry.info.meta.type
        if meta_type == pytsk3.TSK_FS_META_TYPE_LNK:
            print("true")
        else:
            print("false")
    except Exception:
        print("false")

def command_list_directory(image_path, internal_path):
    fs_info = open_fs(image_path)
    try:
        directory = fs_info.open_dir(internal_path)
    except Exception as e:
        sys.stderr.write("Error opening directory: " + str(e))
        sys.exit(1)
    entries = []
    for entry in directory:
        try:
            name = entry.info.name.name.decode("utf-8", errors="replace")
        except Exception:
            continue
        if name in [".", ".."]:
            continue
        # Формируем внутренний путь для дочернего элемента
        child_path = os.path.join(internal_path, name) if internal_path != "" else name
        meta_type = ""
        size = 0
        try:
            mtype = entry.info.meta.type
            if mtype == pytsk3.TSK_FS_META_TYPE_DIR:
                meta_type = "DIR"
            elif mtype == pytsk3.TSK_FS_META_TYPE_REG:
                meta_type = "REG"
            elif mtype == pytsk3.TSK_FS_META_TYPE_LNK:
                meta_type = "LNK"
            elif mtype == pytsk3.TSK_FS_META_TYPE_VIRT_DIR:
                meta_type = "VIRT_DIR"
            size = entry.info.meta.size
        except Exception:
            pass
        entries.append({
            "name": name,
            "path": child_path,
            "meta_type": meta_type,
            "size": size
        })
    print(json.dumps(entries))

def command_follow_symlink(image_path, internal_path, name):
    # Пока реализуем как заглушку – возвращаем пустую строку
    print("")

def command_get_size(image_path, internal_path):
    fs_info = open_fs(image_path)
    try:
        entry = fs_info.open(internal_path)
        size = entry.info.meta.size
        print(str(size))
    except Exception as e:
        sys.stderr.write("Error getting size: " + str(e))
        sys.exit(1)

def command_read_chunks(image_path, internal_path, offset_str, chunk_size_str):
    offset = int(offset_str)
    chunk_size = int(chunk_size_str)
    fs_info = open_fs(image_path)
    try:
        entry = fs_info.open(internal_path)
    except Exception as e:
        sys.stderr.write("Error opening file: " + str(e))
        sys.exit(1)
    try:
        data = entry.read_random(offset, chunk_size)
        # Кодируем данные в base64 для безопасной передачи
        encoded = base64.b64encode(data).decode("utf-8")
        print(encoded)
    except Exception as e:
        sys.stderr.write("Error reading chunks: " + str(e))
        sys.exit(1)

def main():
    if len(sys.argv) < 3:
        sys.stderr.write("Usage: tsk_helper.py <command> <image_file> [<internal_path> ...]\n")
        sys.exit(1)
    command = sys.argv[1]
    if command == "get_root":
        # Usage: tsk_helper.py get_root <image_file>
        command_get_root(sys.argv[2])
    elif command == "is_directory":
        # Usage: tsk_helper.py is_directory <image_file> <internal_path>
        if len(sys.argv) < 4:
            sys.stderr.write("Usage: is_directory <image_file> <internal_path>\n")
            sys.exit(1)
        command_is_directory(sys.argv[2], sys.argv[3])
    elif command == "is_file":
        if len(sys.argv) < 4:
            sys.stderr.write("Usage: is_file <image_file> <internal_path>\n")
            sys.exit(1)
        command_is_file(sys.argv[2], sys.argv[3])
    elif command == "is_symlink":
        if len(sys.argv) < 4:
            sys.stderr.write("Usage: is_symlink <image_file> <internal_path>\n")
            sys.exit(1)
        command_is_symlink(sys.argv[2], sys.argv[3])
    elif command == "list_directory":
        if len(sys.argv) < 4:
            sys.stderr.write("Usage: list_directory <image_file> <internal_path>\n")
            sys.exit(1)
        command_list_directory(sys.argv[2], sys.argv[3])
    elif command == "follow_symlink":
        if len(sys.argv) < 5:
            sys.stderr.write("Usage: follow_symlink <image_file> <internal_path> <name>\n")
            sys.exit(1)
        command_follow_symlink(sys.argv[2], sys.argv[3], sys.argv[4])
    elif command == "get_size":
        if len(sys.argv) < 4:
            sys.stderr.write("Usage: get_size <image_file> <internal_path>\n")
            sys.exit(1)
        command_get_size(sys.argv[2], sys.argv[3])
    elif command == "read_chunks":
        if len(sys.argv) < 6:
            sys.stderr.write("Usage: read_chunks <image_file> <internal_path> <offset> <chunk_size>\n")
            sys.exit(1)
        command_read_chunks(sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5])
    else:
        sys.stderr.write("Unknown command: " + command)
        sys.exit(1)

if __name__ == "__main__":
    main()
