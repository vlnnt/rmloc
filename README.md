# RMLOC

**Language:** English | [Русский](README.ru.md)

Terminal translator for **RPG Maker MZ** games using local AI models through [Ollama](https://ollama.com/).

## Requirements

- Go 1.26+
- Ollama
- Any compatible Ollama model, for example:

```bash
ollama pull qwen3:4b-instruct
ollama serve
```

## Build

Linux/macOS:

```bash
go build -o rmloc ./cmd/rmloc
```

Windows:

```powershell
go build -o rmloc.exe ./cmd/rmloc
```

Cross-build for Windows from Linux/macOS:

```bash
GOOS=windows GOARCH=amd64 go build -o rmloc.exe ./cmd/rmloc
```

## Run

Linux/macOS:

```bash
./rmloc
```

Windows:

```powershell
.\rmloc.exe
```

Select the game directory, Ollama URL, model, prompt file and worker count in the interface.

Original files are moved to the `old` directory. Translation cache is stored in `.rmloc/translation-cache.jsonl`.

## License

Apache License 2.0