package integraciones

// Lo de la v1.9 en la nube que no es de un robot solo sino de su dueño: la
// agenda (agenda.go), la música de Spotify (musica.go) y la charla con IA
// (charla.go). Ver docs/nube.md, "Agenda", "Música" y "Charla".

import (
	"context"
	"sync"
	"time"

	"sintetica/api/internal/cifra"
)

// OpcionesV19 salen del entorno (ver cmd/sintetica-api). Sin las claves de
// Spotify o de Anthropic, esas partes quedan apagadas.
type OpcionesV19 struct {
	// Secreto (SECRETO): de acá sale la clave con la que se cifran la dirección
	// del calendario y el refresh token de Spotify.
	Secreto []byte
	// La URL pública de la API (para la vuelta del OAuth de Spotify) y la de la web
	// (a donde vuelve el usuario después).
	APIURL, WebURL string

	SpotifyID, SpotifySecreto string
	// Vacías, las de Spotify (los tests ponen un servidor falso).
	SpotifyCuentasURL, SpotifyAPIURL string

	AnthropicClave  string
	AnthropicURL    string // vacía, la de siempre (los tests ponen un servidor falso)
	AnthropicModelo string // vacío, claude-haiku-5-5 (con CharlaURL, el modelo de ese servicio)
	// Un servicio compatible con la API de chat de OpenAI (Groq, OpenRouter,
	// Gemini, uno propio): con CharlaURL y un modelo, se usa en vez de Anthropic.
	CharlaURL, CharlaClave string
	// ElevenLabsURL: vacía, la de siempre (los tests ponen un servidor falso).
	// La clave de ElevenLabs es de cada usuario (voz.go).
	ElevenLabsURL string
	// La voz gratis (Edge TTS, edge.go): EdgeApagado la apaga en todo el
	// servidor (EDGE_TTS=0); EdgeTope, caracteres por usuario por día
	// (EDGE_TTS_TOPE, por defecto 3000). Las URLs, para los tests.
	EdgeApagado           bool
	EdgeTope              int
	EdgeURL, EdgeVocesURL string
}

type estadoV19 struct {
	cifraAgenda, cifraSpotify *cifra.Cifrador
	ahora                     func() time.Time

	musicaMu sync.Mutex
	musica   map[string]sonando     // usuario → lo último que se mandó
	tokens   map[string]tokenAcceso // usuario → el access token de Spotify
	bpms     map[string]int         // tema → bpm (0 = no se sabe)

	charlas chan struct{} // cuántas charlas por MQTT a la vez
}

func (i *Integraciones) iniciarV19() {
	sec := i.op.V19.Secreto
	if len(sec) == 0 {
		sec = []byte(aleatorio(32)) // sin SECRETO, lo cifrado no sobrevive a un reinicio
	}
	i.v19 = estadoV19{
		cifraAgenda: cifra.Nuevo(sec, "agenda"), cifraSpotify: cifra.Nuevo(sec, "spotify"), ahora: time.Now,
		musica: map[string]sonando{}, tokens: map[string]tokenAcceso{}, bpms: map[string]int{},
		charlas: make(chan struct{}, 8),
	}
	i.iniciarVoz(sec)
}

func (i *Integraciones) ahora() time.Time { return i.v19.ahora() }

// correrV19: la agenda cada minuto (y la lee cada 5) y, con Spotify, qué
// suena cada 20 s.
func (i *Integraciones) correrV19(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); porMinuto(ctx, func(m time.Time) { i.revisarAgendas(ctx, m) }) }()
	if i.MusicaActiva() {
		wg.Add(1)
		go func() { defer wg.Done(); i.sondearMusica(ctx) }()
		i.log.Info("Spotify prendido")
	}
	if i.CharlaActiva() {
		i.log.Info("charla con IA prendida", "modelo", i.modeloCharla())
	}
	wg.Wait()
}
