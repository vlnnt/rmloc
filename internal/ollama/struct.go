package ollama

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

type Client struct {
	baseURL    string
	model      string
	prompt     string
	promptHash string

	httpClient *fasthttp.Client
	timeout    time.Duration

	translationCache sync.Map
	dialogueCache    sync.Map

	cachePath   string
	cacheFileMu sync.Mutex
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
	Message message `json:"message"`
	Error   string  `json:"error"`
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
	Model        string   `json:"model"`
	PromptHash   string   `json:"prompt_hash"`
	Kind         string   `json:"kind"`
	Key          string   `json:"key"`
	Translation  string   `json:"translation,omitempty"`
	Translations []string `json:"translations,omitempty"`
}
