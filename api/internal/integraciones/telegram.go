package integraciones

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

// Telegram es un cliente mínimo de la Bot API (solo net/http). El token va
// en el camino de cada pedido: nunca se loguea una URL ni un error con ella.
type Telegram struct {
	base   string
	token  string
	espera int // segundos de long polling
	cli    *http.Client
}

// NuevoTelegram arma el cliente. base vacía = https://api.telegram.org.
func NuevoTelegram(base, token string, espera int) *Telegram {
	if base == "" {
		base = "https://api.telegram.org"
	}
	return &Telegram{base: strings.TrimRight(base, "/"), token: token, espera: espera,
		cli: &http.Client{Timeout: time.Duration(espera+15) * time.Second}}
}

// ErrTelegram es un error que devolvió la API de Telegram.
type ErrTelegram struct {
	Codigo      int
	Descripcion string
	Reintentar  int // segundos (429)
}

func (e ErrTelegram) Error() string { return fmt.Sprintf("Telegram: %d %s", e.Codigo, e.Descripcion) }

// llamar hace un método de la Bot API con parámetros JSON.
func (t *Telegram) llamar(ctx context.Context, metodo string, params any, salida any) error {
	b, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.base+"/bot"+t.token+"/"+metodo, bytes.NewReader(b))
	if err != nil {
		return errors.New("telegram: no pude armar el pedido")
	}
	req.Header.Set("Content-Type", "application/json")
	r, err := t.cli.Do(req)
	if err != nil {
		return fmt.Errorf("telegram %s: %w", metodo, sinURL(err))
	}
	defer r.Body.Close()
	var resp struct {
		Ok          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&resp); err != nil {
		return fmt.Errorf("telegram %s: respuesta inválida (%d)", metodo, r.StatusCode)
	}
	if !resp.Ok {
		return ErrTelegram{Codigo: resp.ErrorCode, Descripcion: resp.Description, Reintentar: resp.Parameters.RetryAfter}
	}
	if salida != nil {
		return json.Unmarshal(resp.Result, salida)
	}
	return nil
}

// Tipos de la Bot API (lo que se usa).
type update struct {
	UpdateID      int64     `json:"update_id"`
	Message       *mensaje  `json:"message"`
	CallbackQuery *callback `json:"callback_query"`
}

type mensaje struct {
	MessageID int64      `json:"message_id"`
	From      *usuarioTG `json:"from"`
	Chat      chatTG     `json:"chat"`
	Text      string     `json:"text"`
}

type usuarioTG struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type chatTG struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type callback struct {
	ID      string    `json:"id"`
	From    usuarioTG `json:"from"`
	Message *mensaje  `json:"message"`
	Data    string    `json:"data"`
}

// Boton es un botón de un teclado inline.
type Boton struct {
	Texto string `json:"text"`
	Datos string `json:"callback_data"`
}

func (t *Telegram) mandar(ctx context.Context, chat int64, texto string, teclado [][]Boton) error {
	p := map[string]any{"chat_id": chat, "text": texto, "disable_web_page_preview": true}
	if len(teclado) > 0 {
		p["reply_markup"] = map[string]any{"inline_keyboard": teclado}
	}
	return t.llamar(ctx, "sendMessage", p, nil)
}

// MandarTelegram le escribe al usuario (si vinculó su chat).
func (i *Integraciones) MandarTelegram(ctx context.Context, usuario, texto string) error {
	if i.tg == nil {
		return errors.New("Telegram está apagado en este servidor")
	}
	v, err := i.alm.TelegramDe(ctx, usuario)
	if err != nil {
		return errors.New("no vinculaste Telegram")
	}
	if len(texto) > 4000 {
		texto = texto[:4000]
	}
	return i.tg.mandar(ctx, v.ChatID, texto, nil)
}

// AvisarReserva manda una reserva de la landing al chat de las reservas (HTML).
// Sin Telegram o sin chat, no hace nada.
func (i *Integraciones) AvisarReserva(ctx context.Context, html string) error {
	if i.tg == nil || i.op.ChatReservas == 0 {
		return nil
	}
	return i.tg.llamar(ctx, "sendMessage", map[string]any{
		"chat_id": i.op.ChatReservas, "text": html, "parse_mode": "HTML", "disable_web_page_preview": true,
	}, nil)
}

// escucharTelegram es el trabajador del bot: long polling con getUpdates (no
// hace falta exponer nada para recibir), con backoff si algo falla.
func (i *Integraciones) escucharTelegram(ctx context.Context) {
	offset, err := i.alm.OffsetTelegram(ctx)
	if err != nil {
		i.log.Warn("telegram: no pude leer el offset", "error", err)
	}
	// Un webhook configurado haría fallar getUpdates (409).
	i.tg.llamar(ctx, "deleteWebhook", map[string]any{}, nil)                        //nolint:errcheck
	i.tg.llamar(ctx, "setMyCommands", map[string]any{"commands": comandosBot}, nil) //nolint:errcheck
	espera := time.Second
	for ctx.Err() == nil {
		var ups []update
		err := i.tg.llamar(ctx, "getUpdates", map[string]any{
			"offset": offset, "timeout": i.tg.espera, "allowed_updates": []string{"message", "callback_query"},
		}, &ups)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			pausa := espera
			var et ErrTelegram
			if errors.As(err, &et) && et.Reintentar > 0 {
				pausa = time.Duration(et.Reintentar) * time.Second
			}
			i.log.Warn("telegram: getUpdates falló (reintento)", "error", err, "en", pausa.String())
			select {
			case <-ctx.Done():
				return
			case <-time.After(pausa):
			}
			espera = min(espera*2, time.Minute)
			continue
		}
		espera = time.Second
		for _, u := range ups {
			i.atender(ctx, u)
			offset = u.UpdateID + 1
		}
		if len(ups) > 0 {
			if err := i.alm.GuardarOffsetTelegram(ctx, offset); err != nil && ctx.Err() == nil {
				i.log.Warn("telegram: no pude guardar el offset", "error", err)
			}
		}
	}
}

func (i *Integraciones) atender(ctx context.Context, u update) {
	defer func() {
		if p := recover(); p != nil {
			i.log.Error("telegram: pánico atendiendo un mensaje", "error", p)
		}
	}()
	ctx, cancelar := context.WithTimeout(ctx, 20*time.Second)
	defer cancelar()
	switch {
	case u.Message != nil:
		i.atenderMensaje(ctx, u.Message)
	case u.CallbackQuery != nil:
		i.atenderBoton(ctx, u.CallbackQuery)
	}
}
