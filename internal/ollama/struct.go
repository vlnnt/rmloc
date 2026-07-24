package ollama

import (
	"encoding/json"
	"time"

	"github.com/valyala/fasthttp"
)

type Client struct {
	baseURL    string
	model      string
	httpClient *fasthttp.Client
	timeout    time.Duration
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatOptions struct {
	Temperature float64 `json:"temperature"`
	NumPredict  int     `json:"num_predict"`
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
