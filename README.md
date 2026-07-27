# rmloc

`rmloc` is a command-line tool for translating game files with AI.

The project is designed to support multiple game engines and AI providers in the future. Currently, it supports RPG Maker MZ and uses an Ollama-compatible API for translation.

Translation behavior is controlled through an external JSON prompt file. This allows users to translate between different languages without rebuilding the application.

## Requirements

* Go
* Ollama or another compatible Ollama API server
* A compatible translation model

Example:

```bash
ollama pull qwen3:4b
```

Start Ollama:

```bash
ollama serve
```

Optional parallel processing on Linux or macOS:

```bash
OLLAMA_NUM_PARALLEL=2 ollama serve
```

The default local Ollama API address is commonly:

```text
http://localhost:11434
```

The API URL must still be passed explicitly through the `-url` argument.

## Build

### Linux

```bash
go build -o rmloc ./cmd/rmloc
```

### Windows x64

From Linux or macOS:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
go build -o rmloc.exe ./cmd/rmloc
```

### Windows ARM64

From Linux or macOS:

```bash
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 \
go build -o rmloc-arm64.exe ./cmd/rmloc
```

### macOS Intel

```bash
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 \
go build -o rmloc-macos-amd64 ./cmd/rmloc
```

### macOS Apple Silicon

```bash
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
go build -o rmloc-macos-arm64 ./cmd/rmloc
```

## Command-line arguments

The following arguments are required:

```text
-url
-model
-prompt
```

The game directory must be provided as the final positional argument.

```text
rmloc -url <API URL> -model <model> -prompt <prompt.json> <game directory>
```

### Arguments

`-url`

The base URL of the Ollama-compatible API server.

Example:

```text
http://localhost:11434
```

`-model`

The model name sent to the API.

Example:

```text
qwen3:4b
```

`-prompt`

The path to a JSON file containing the translation prompts.

Example:

```text
./prompts/en-ru.json
```

## Usage

### Linux

```bash
./rmloc \
  -url http://localhost:11434 \
  -model qwen3:4b \
  -prompt ./prompts/en-ru.json \
  /path/to/game
```

### macOS

```bash
./rmloc-macos-arm64 \
  -url http://localhost:11434 \
  -model qwen3:4b \
  -prompt ./prompts/en-ru.json \
  /path/to/game
```

Use `rmloc-macos-amd64` instead on an Intel Mac.

### Windows PowerShell

```powershell
.\rmloc.exe `
  -url http://localhost:11434 `
  -model qwen3:4b `
  -prompt .\prompts\en-ru.json `
  "C:\Path\To\Game"
```

A single-line Windows command is also supported:

```powershell
.\rmloc.exe -url http://localhost:11434 -model qwen3:4b -prompt .\prompts\en-ru.json "C:\Path\To\Game"
```

The selected game must currently be an RPG Maker MZ project containing a `data` or `www/data` directory.

Options must be specified before the game directory.

## Prompt configuration

Translation instructions are loaded from a JSON file.

Example:

```json
{
  "name": "English to Russian",
  "single_translation": [
    "You are a professional English-to-Russian translator.",
    "Translate the input into natural Russian.",
    "Return only the required JSON object."
  ],
  "dialogue_translation": [
    "You are a professional English-to-Russian video game translator.",
    "Translate every element while preserving its position in the array.",
    "Use surrounding dialogue lines as context.",
    "Return only the required JSON object."
  ]
}
```

The configuration contains three fields:

`name`

An optional human-readable name printed when the program starts.

`single_translation`

The system prompt used for individual strings such as names, descriptions, and standalone messages.

`dialogue_translation`

The system prompt used for grouped dialogue lines, scrolling text, and player choices.

Both prompt fields may be written as an array of strings:

```json
{
  "single_translation": [
    "First instruction.",
    "Second instruction."
  ]
}
```

They may also be written as a single JSON string:

```json
{
  "single_translation": "First instruction.\nSecond instruction."
}
```

Both `single_translation` and `dialogue_translation` are required and must not be empty.

To translate another language pair, copy an existing prompt file and replace its translation instructions.

Examples:

```text
prompts/en-ru.json
prompts/ja-en.json
prompts/zh-de.json
prompts/ru-es.json
```

The program detects letters from different writing systems, including Latin, Cyrillic, Japanese, Chinese, Korean, Arabic, and others.

## Structured responses

The single-string prompt must instruct the model to return:

```json
{
  "translation": "Translated text"
}
```

The dialogue prompt must instruct the model to return:

```json
{
  "translations": [
    "First translated line",
    "Second translated line"
  ]
}
```

For grouped translations, the returned array must contain exactly the same number of elements as the input array.

## Cache

Translations are stored inside the selected game directory:

```text
.rmloc/translation-cache.jsonl
```

Do not delete this file when restarting an interrupted translation. Existing results will be loaded and reused.

Cache entries are separated by:

* model name;
* prompt fingerprint;
* translation type;
* source text or dialogue block.

Changing the model or changing the contents of the prompt file creates a different cache context. Translations produced using another model or another prompt will not be reused accidentally.

The cache also stores in-memory aliases for translated text. This prevents already translated game files from being sent to the model again when the program is restarted.

Example:

```text
Source:      Goblin
Translation: Гоблин
```

The program can reuse the cached result when it encounters either the original text or the already translated text during the same cache context.

When a cache file is loaded, the program prints information similar to:

```text
Loaded cache entries: 5251; translated-text aliases: 5036
```

## Backups

Original files are moved into an `old` directory before translated files replace them.

Example:

```text
data/old/Actors_20260727_153000.json
data/old/Map001_20260727_153015.json
```

A timestamp is added to each backup filename.

## Safety

RPG Maker control codes and placeholders are protected before text is sent to the model.

Examples:

```text
\N[1]
\V[2]
%1
{name}
${variable}
<color>
```

Protected values are temporarily replaced with markers such as:

```text
⟦RMLOC_0000⟧
```

The model must preserve each marker exactly once.

If validation fails, the program retries the translation with stricter instructions.

If a grouped translation repeatedly fails validation, its lines are translated individually.

If an individual string still cannot be translated safely, the original text is kept and processing continues.

## Output statistics

After processing, `rmloc` prints statistics including:

```text
Translation statistics:
  Processed text entries
  Translated by model
  Left unchanged after protection errors
  Loaded from cache
  Skipped without translation
  HTTP requests
  Successful HTTP requests
  Failed HTTP attempts
  Retry attempts
  Prompt tokens
  Generated tokens
  Ollama processing time
  HTTP wait time
  Model loading time
  Prompt processing speed
  Generation speed
```

## Example

```bash
ollama pull qwen3:4b
ollama serve
```

In another terminal:

```bash
./rmloc \
  -url http://localhost:11434 \
  -model qwen3:4b \
  -prompt ./prompts/en-ru.json \
  "/home/user/Games/My RPG Maker Game"
```
