package integraciones

// La charla con cualquier servicio compatible con la API de chat de OpenAI
// (Groq, OpenRouter, Gemini, Cloudflare, un Ollama propio…): CHARLA_URL es la
// base (por ejemplo https://api.groq.com/openai/v1), CHARLA_CLAVE la clave (puede
// ir vacía para uno propio) y CHARLA_MODELO el modelo. Con CHARLA_URL se usa
// esto en vez de Anthropic. Mismo prompt, mismo tope y misma limpieza. Un
// usuario puede poner el suyo (ia.go): entonces va con su URL, su clave y su
// modelo, por el cliente con las protecciones de los webhooks.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// charlaCompatible: hay un servicio compatible configurado.
func (i *Integraciones) charlaCompatible() bool {
	return i.op.V19.CharlaURL != "" && i.op.V19.AnthropicModelo != ""
}

// preguntarCompatible: POST {base}/chat/completions pidiendo JSON. No todos
// aceptan json_schema, así que se pide json_object y el esquema va en el prompt.
func (i *Integraciones) preguntarCompatible(ctx context.Context, m motorIA, d datosCharla, texto string) (string, string, error) {
	esquema, _ := json.Marshal(esquemaCharla(d.Expresiones))
	sistema := promptCharla(d) + "\nContestá solo con un JSON que cumpla este esquema: " + string(esquema)
	cuerpo, _ := json.Marshal(map[string]any{
		"model": m.modelo,
		"messages": []map[string]string{
			{"role": "system", "content": sistema},
			{"role": "user", "content": texto},
		},
		"max_tokens":      300,
		"temperature":     0.8,
		"response_format": map[string]string{"type": "json_object"},
	})
	ctx, cancelar := context.WithTimeout(ctx, TimeoutCharla)
	defer cancelar()
	url := strings.TrimRight(m.url, "/") + "/chat/completions"
	pedido, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(cuerpo))
	if err != nil {
		return "", "", errors.New("no me pude comunicar con la IA")
	}
	pedido.Header.Set("Content-Type", "application/json")
	if c := m.clave; c != "" {
		pedido.Header.Set("Authorization", "Bearer "+c)
	}
	cli := &http.Client{Timeout: TimeoutCharla + 5*time.Second}
	if m.propia {
		cli = i.ia.http // la URL la eligió el usuario: sin IPs internas
	}
	r, err := cli.Do(pedido)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", "", errors.New("la IA no respondió a tiempo")
		}
		return "", "", errors.New("no me pude comunicar con la IA")
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("la IA respondió %d", r.StatusCode)
	}
	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&res); err != nil || len(res.Choices) == 0 {
		return "", "", errors.New("la IA contestó algo raro")
	}
	return leerRespuestaJSON(res.Choices[0].Message.Content)
}

// leerRespuestaJSON: {"texto","expresion"}; algunos modelos lo envuelven en
// ```json … ```, y si no hay JSON va el texto tal cual.
func leerRespuestaJSON(crudo string) (string, string, error) {
	crudo = strings.TrimSpace(crudo)
	if a, b := strings.Index(crudo, "{"), strings.LastIndex(crudo, "}"); a >= 0 && b > a {
		var res struct {
			Texto     string `json:"texto"`
			Expresion string `json:"expresion"`
		}
		if json.Unmarshal([]byte(crudo[a:b+1]), &res) == nil && res.Texto != "" {
			return res.Texto, res.Expresion, nil
		}
	}
	return crudo, "", nil
}
