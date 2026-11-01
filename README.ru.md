# RMLOC

**Язык:** [English](README.md) | Русский

Консольный переводчик игр на **RPG Maker MZ**, использующий локальные модели искусственного интеллекта через [Ollama](https://ollama.com/).

## Требования

- Go 1.26+
- Ollama
- Любая совместимая модель Ollama, например:

```bash
ollama pull qwen3:4b-instruct
ollama serve
```

## Сборка

Linux/macOS:

```bash
go build -o rmloc ./cmd/rmloc
```

Windows:

```powershell
go build -o rmloc.exe ./cmd/rmloc
```

Кросс-компиляция для Windows из Linux/macOS:

```bash
GOOS=windows GOARCH=amd64 go build -o rmloc.exe ./cmd/rmloc
```

## Запуск

Linux/macOS:

```bash
./rmloc
```

Windows:

```powershell
.\rmloc.exe
```

В интерфейсе программы укажите папку с игрой, адрес сервера Ollama, модель, файл промпта и количество параллельных обработчиков (Workers).

Оригинальные файлы перемещаются в каталог `old`. Кеш переводов сохраняется в файле `.rmloc/translation-cache.jsonl`.

## Лицензия

Apache License 2.0