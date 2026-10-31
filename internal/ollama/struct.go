package ollama

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
	"golang.org/x/sync/singleflight"
)

type Client struct {
	baseURL                   string
	model                     string
	singleTranslationPrompt   string
	batchTranslationPrompt    string
	dialogueTranslationPrompt string
	legacyPromptHash          string
	singlePromptHash          string
	batchPromptHash           string
	dialoguePromptHash        string
	allowLegacyCache          bool
	httpClient                *fasthttp.Client
	timeout                   time.Duration
	translationCache          sync.Map
	dialogueCache             sync.Map
	requestGroup              singleflight.Group
	cachePath                 string
	cacheFileMu               sync.Mutex
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatOptions struct {
	Temperature float64 `json:"temperature"`
	NumPredict  int     `json:"num_predict"`
	NumCtx      int     `json:"num_ctx"`
}

type chatRequest struct {
	Model     string          `json:"model"`
	Stream    bool            `json:"stream"`
	Think     bool            `json:"think"`
	KeepAlive string          `json:"keep_alive"`
	Format    json.RawMessage `json:"format"`
	Options   chatOptions     `json:"options"`
	Messages  []message       `json:"messages"`
}

type chatResponse struct {
	Message            message `json:"message"`
	Error              string  `json:"error"`
	TotalDuration      int64   `json:"total_duration"`
	LoadDuration       int64   `json:"load_duration"`
	PromptEvalCount    int     `json:"prompt_eval_count"`
	PromptEvalDuration int64   `json:"prompt_eval_duration"`
	EvalCount          int     `json:"eval_count"`
	EvalDuration       int64   `json:"eval_duration"`
}

type translationResponse struct {
	Translation string `json:"translation"`
}

type translationLinesRequest struct {
	Lines []string `json:"lines"`
}

type translationLinesResponse struct {
	Translations []string `json:"translations"`
}

type persistentCacheEntry struct {
	Version      int      `json:"version"`
	Model        string   `json:"model"`
	PromptMode   string   `json:"prompt_mode,omitempty"`
	PromptHash   string   `json:"prompt_hash,omitempty"`
	Kind         string   `json:"kind"`
	Key          string   `json:"key"`
	Translation  string   `json:"translation,omitempty"`
	Translations []string `json:"translations,omitempty"`
}

type translationValidationError struct {
	attempts int
	err      error
}

type structuredResponseError struct {
	content string
	err     error
}

type batchTextEntry struct {
	text    string
	indexes []int
}
