#!/usr/bin/env python3
import sys
import json
import pefile
from datetime import datetime

def add_vs_info(parsed_pe):
    # Карта соответствия полей версии.
    VS_INFO_FIELDS = {
        "CompanyName": "company",
        "FileDescription": "description",
        "FileVersion": "file_version",
        "InternalName": "original_file_name",
        "ProductName": "product"
    }
    result = {}
    if hasattr(parsed_pe, "VS_VERSIONINFO") and hasattr(parsed_pe, "FileInfo"):
        for fileinfo in parsed_pe.FileInfo:
            for entry in fileinfo:
                if hasattr(entry, 'StringTable'):
                    for st in entry.StringTable:
                        for key, value in st.entries.items():
                            # Приводим ключ к строке, если он bytes.
                            if isinstance(key, bytes):
                                key = key.decode('utf-8', 'replace')
                            if key in VS_INFO_FIELDS and value:
                                if isinstance(value, bytes):
                                    result[VS_INFO_FIELDS[key]] = value.decode('utf-8', 'replace')
                                else:
                                    result[VS_INFO_FIELDS[key]] = str(value)
    return result

def get_mime_type(file_path):
    """
    Определяет MIME‑тип файла по первым 261 байту с использованием библиотеки filetype.
    Если библиотека не установлена или определить тип не удалось, возвращается "application/octet-stream".
    """
    try:
        import filetype
    except ImportError:
        return "application/octet-stream"
    try:
        with open(file_path, "rb") as f:
            head = f.read(261)
        kind = filetype.guess(head)
        if kind:
            return kind.mime
        else:
            return "application/octet-stream"
    except Exception as e:
        return "application/octet-stream"

def extract_pe_info(file_path):
    try:
        parsed_pe = pefile.PE(file_path)
    except Exception as e:
        return {"error": str(e)}
    
    info = add_vs_info(parsed_pe)
    try:
        info['imphash'] = parsed_pe.get_imphash()
    except Exception as e:
        info['imphash'] = ""
    try:
        compilation_time = datetime.utcfromtimestamp(parsed_pe.FILE_HEADER.TimeDateStamp).isoformat()
    except Exception as e:
        compilation_time = ""
    info['compilation'] = compilation_time
    
    # Получаем MIME‑тип файла
    info['mime'] = get_mime_type(file_path)
    
    return info

def main():
    try:
        if len(sys.argv) < 2:
            #ПОМЕНЯТЬ НАЗВАНИЕ usage: extract_pe_info.py <pe_file> (И ДЛЯ ПЕРЕМЕННОЙ ТОЖЕ extract_pe_info)
            print(json.dumps({"error": "usage: extract_pe_info.py <pe_file>"}))
            sys.exit(0)
        file_path = sys.argv[1]
        info = extract_pe_info(file_path)
        print(json.dumps(info))
    except Exception as e:
        print(json.dumps({"error": str(e)}))
    finally:
        sys.exit(0)

if __name__ == "__main__":
    main()
