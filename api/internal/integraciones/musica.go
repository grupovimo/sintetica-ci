package integraciones

// La música: Spotify con OAuth (SPOTIFY_CLIENT_ID y SPOTIFY_CLIENT_SECRET;
// sin ellas, apagado). Cada 20 s, para los usuarios con algún robot
// conectado, mira qué suena y, si cambió el tema o se pausó, les manda el
// 0x4B a sus robots conectados (y el evento SSE "musica"). El refresh token se
// guarda cifrado; el access token vive en memoria. El celu manda lo suyo con
// POST /v1/bichos/{id}/musica (MandarMusica). Ver docs/nube.md, "Música".

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// Spotify.
const (
	CadaMusica      = 20 * time.Second
	alcancesSpotify = "user-read-currently-playing user-read-playback-state"
	venceEstado     = 10 * time.Minute
)

// ErrMusicaApagada: no hay claves de Spotify en este servidor.
var ErrMusicaApagada = errors.New("la música de Spotify no está prendida en este servidor")

// ErrSinSpotify: el usuario no conectó Spotify (o hay que volver a conectarlo).
var ErrSinSpotify = errors.New("no conectaste Spotify")

type sonando struct {
	ID      string `json:"-"`
	Sonando bool   `json:"sonando"`
	Titulo  string `json:"titulo"`
	Artista string `json:"artista"`
	BPM     int    `json:"bpm"`
	en      time.Time
}

type tokenAcceso struct {
	token string
	vence time.Time
}

// MusicaActiva: hay claves de Spotify.
func (i *Integraciones) MusicaActiva() bool {
	return i.op.V19.SpotifyID != "" && i.op.V19.SpotifySecreto != ""
}

func (i *Integraciones) urlCuentas() string {
	if u := i.op.V19.SpotifyCuentasURL; u != "" {
		return strings.TrimRight(u, "/")
	}
	return "https://accounts.spotify.com"
}

func (i *Integraciones) urlSpotify() string {
	if u := i.op.V19.SpotifyAPIURL; u != "" {
		return strings.TrimRight(u, "/")
	}
	return "https://api.spotify.com"
}

func (i *Integraciones) vueltaSpotify() string {
	return strings.TrimRight(i.op.V19.APIURL, "/") + "/v1/musica/spotify/vuelta"
}

var clienteSpotify = &http.Client{Timeout: 10 * time.Second}

// --- OAuth ---

func (i *Integraciones) firmaEstado(s string) string {
	m := hmac.New(sha256.New, append([]byte("spotify-estado/"), i.op.V19.Secreto...))
	m.Write([]byte(s))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// EnlaceSpotify: a dónde mandar al usuario para que autorice (el state lleva
// su id, vence en 10 min y va firmado).
func (i *Integraciones) EnlaceSpotify(usuario string) (string, error) {
	if !i.MusicaActiva() {
		return "", ErrMusicaApagada
	}
	cuerpo := usuario + "|" + strconv.FormatInt(i.ahora().Add(venceEstado).Unix(), 10)
	estado := base64.RawURLEncoding.EncodeToString([]byte(cuerpo)) + "." + i.firmaEstado(cuerpo)
	q := url.Values{"client_id": {i.op.V19.SpotifyID}, "response_type": {"code"}, "redirect_uri": {i.vueltaSpotify()},
		"scope": {alcancesSpotify}, "state": {estado}}
	return i.urlCuentas() + "/authorize?" + q.Encode(), nil
}

// usuarioDelEstado valida el state que volvió.
func (i *Integraciones) usuarioDelEstado(estado string) (string, bool) {
	c, f, ok := strings.Cut(estado, ".")
	if !ok {
		return "", false
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil || !hmac.Equal([]byte(i.firmaEstado(string(b))), []byte(f)) {
		return "", false
	}
	u, v, ok := strings.Cut(string(b), "|")
	vence, err := strconv.ParseInt(v, 10, 64)
	if !ok || err != nil || i.ahora().Unix() > vence {
		return "", false
	}
	return u, true
}

type respuestaToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

func (i *Integraciones) pedirToken(ctx context.Context, form url.Values) (respuestaToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, i.urlCuentas()+"/api/token", strings.NewReader(form.Encode()))
	if err != nil {
		return respuestaToken{}, err
	}
	req.SetBasicAuth(i.op.V19.SpotifyID, i.op.V19.SpotifySecreto)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r, err := clienteSpotify.Do(req)
	if err != nil {
		return respuestaToken{}, sinURL(err)
	}
	defer r.Body.Close()
	var t respuestaToken
	json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&t) //nolint:errcheck
	if r.StatusCode == http.StatusBadRequest && t.Error == "invalid_grant" {
		return t, ErrSinSpotify
	}
	if r.StatusCode != http.StatusOK || t.AccessToken == "" {
		return t, fmt.Errorf("Spotify respondió %d", r.StatusCode)
	}
	return t, nil
}

// VueltaSpotify canjea el código y guarda la cuenta. Devuelve el usuario.
func (i *Integraciones) VueltaSpotify(ctx context.Context, estado, codigo string) (string, error) {
	if !i.MusicaActiva() {
		return "", ErrMusicaApagada
	}
	usuario, ok := i.usuarioDelEstado(estado)
	if !ok || codigo == "" {
		return "", errors.New("el link venció o no es válido: probá conectar de nuevo")
	}
	t, err := i.pedirToken(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {codigo}, "redirect_uri": {i.vueltaSpotify()}})
	if err != nil {
		return usuario, errors.New("Spotify no aceptó el código: probá de nuevo")
	}
	if t.RefreshToken == "" {
		return usuario, errors.New("Spotify no devolvió un refresh token")
	}
	var yo struct {
		DisplayName string `json:"display_name"`
		ID          string `json:"id"`
	}
	if err := i.getSpotify(ctx, t.AccessToken, "/v1/me", &yo); err != nil {
		i.log.Info("spotify: no pude leer el nombre", "error", err)
	}
	nombre := yo.DisplayName
	if nombre == "" {
		nombre = yo.ID
	}
	if err := i.alm.GuardarSpotify(ctx, usuario, i.v19.cifraSpotify.Cifrar(t.RefreshToken), protocolo.CortarUTF8(nombre, 60)); err != nil {
		return usuario, err
	}
	i.v19.musicaMu.Lock()
	i.v19.tokens[usuario] = tokenAcceso{token: t.AccessToken, vence: i.ahora().Add(time.Duration(t.ExpiresIn-60) * time.Second)}
	delete(i.v19.musica, usuario)
	i.v19.musicaMu.Unlock()
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "musica", Datos: map[string]any{"conectada": true, "nombre": nombre}})
	return usuario, nil
}

// DesconectarSpotify olvida la cuenta.
func (i *Integraciones) DesconectarSpotify(ctx context.Context, usuario string) error {
	i.v19.musicaMu.Lock()
	delete(i.v19.tokens, usuario)
	delete(i.v19.musica, usuario)
	i.v19.musicaMu.Unlock()
	if err := i.alm.BorrarSpotify(ctx, usuario); err != nil {
		return err
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "musica", Datos: map[string]any{"conectada": false}})
	return nil
}

// tokenDe: el access token del usuario (lo renueva con el refresh token).
func (i *Integraciones) tokenDe(ctx context.Context, usuario string) (string, error) {
	i.v19.musicaMu.Lock()
	t, ok := i.v19.tokens[usuario]
	i.v19.musicaMu.Unlock()
	if ok && i.ahora().Before(t.vence) {
		return t.token, nil
	}
	c, err := i.alm.SpotifyDe(ctx, usuario)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return "", ErrSinSpotify
	}
	if err != nil {
		return "", err
	}
	refresh, err := i.v19.cifraSpotify.Descifrar(c.RefreshCifrado)
	if err != nil {
		return "", ErrSinSpotify
	}
	r, err := i.pedirToken(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
	if errors.Is(err, ErrSinSpotify) {
		// Lo revocaron desde Spotify: se olvida.
		i.log.Info("spotify: el refresh token ya no vale", "usuario", usuario)
		i.DesconectarSpotify(ctx, usuario) //nolint:errcheck
		return "", ErrSinSpotify
	}
	if err != nil {
		return "", err
	}
	if r.RefreshToken != "" && r.RefreshToken != refresh {
		i.alm.GuardarSpotify(ctx, usuario, i.v19.cifraSpotify.Cifrar(r.RefreshToken), c.Nombre) //nolint:errcheck
	}
	i.v19.musicaMu.Lock()
	i.v19.tokens[usuario] = tokenAcceso{token: r.AccessToken, vence: i.ahora().Add(time.Duration(r.ExpiresIn-60) * time.Second)}
	i.v19.musicaMu.Unlock()
	return r.AccessToken, nil
}

// errSinContenido: 204 (no suena nada).
var errSinContenido = errors.New("sin contenido")

func (i *Integraciones) getSpotify(ctx context.Context, token, camino string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.urlSpotify()+camino, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	r, err := clienteSpotify.Do(req)
	if err != nil {
		return sinURL(err)
	}
	defer r.Body.Close()
	switch {
	case r.StatusCode == http.StatusNoContent:
		return errSinContenido
	case r.StatusCode == http.StatusUnauthorized:
		return ErrSinSpotify
	case r.StatusCode != http.StatusOK:
		return fmt.Errorf("Spotify respondió %d", r.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}

// queSuena pregunta a Spotify.
func (i *Integraciones) queSuena(ctx context.Context, usuario string) (sonando, error) {
	token, err := i.tokenDe(ctx, usuario)
	if err != nil {
		return sonando{}, err
	}
	var r struct {
		IsPlaying bool `json:"is_playing"`
		Item      *struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
			Show *struct {
				Name string `json:"name"`
			} `json:"show"`
		} `json:"item"`
	}
	err = i.getSpotify(ctx, token, "/v1/me/player/currently-playing", &r)
	if errors.Is(err, errSinContenido) {
		return sonando{en: i.ahora()}, nil
	}
	if errors.Is(err, ErrSinSpotify) {
		// El access token venció antes de tiempo: la próxima vez se renueva.
		i.v19.musicaMu.Lock()
		delete(i.v19.tokens, usuario)
		i.v19.musicaMu.Unlock()
		return sonando{}, err
	}
	if err != nil {
		return sonando{}, err
	}
	s := sonando{Sonando: r.IsPlaying && r.Item != nil, en: i.ahora()}
	if r.Item != nil {
		s.ID, s.Titulo = r.Item.ID, r.Item.Name
		var as []string
		for _, a := range r.Item.Artists {
			as = append(as, a.Name)
		}
		s.Artista = strings.Join(as, ", ")
		if s.Artista == "" && r.Item.Show != nil {
			s.Artista = r.Item.Show.Name // un podcast
		}
		s.BPM = i.bpmDe(ctx, token, s.ID)
	}
	return s, nil
}

// bpmDe: el tempo del tema (de la API de audio de Spotify, si la app tiene
// acceso; si no, 0). Se pregunta una vez por tema.
func (i *Integraciones) bpmDe(ctx context.Context, token, id string) int {
	if id == "" {
		return 0
	}
	i.v19.musicaMu.Lock()
	b, ok := i.v19.bpms[id]
	i.v19.musicaMu.Unlock()
	if ok {
		return b
	}
	var f struct {
		Tempo float64 `json:"tempo"`
	}
	if err := i.getSpotify(ctx, token, "/v1/audio-features/"+url.PathEscape(id), &f); err == nil && f.Tempo > 0 {
		b = int(f.Tempo + 0.5)
	}
	i.v19.musicaMu.Lock()
	if len(i.v19.bpms) > 5000 {
		i.v19.bpms = map[string]int{}
	}
	i.v19.bpms[id] = b
	i.v19.musicaMu.Unlock()
	return b
}

// --- El sondeo ---

func (i *Integraciones) sondearMusica(ctx context.Context) {
	t := time.NewTicker(CadaMusica)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		i.revisarMusica(ctx)
	}
}

// revisarMusica: una vuelta (los tests la llaman directo).
func (i *Integraciones) revisarMusica(ctx context.Context) {
	cs, err := i.alm.CuentasSpotify(ctx)
	if err != nil {
		i.log.Warn("spotify: no pude leer las cuentas", "error", err)
		return
	}
	for _, usuario := range cs {
		rs, err := i.robotsDe(ctx, usuario)
		if err != nil {
			continue
		}
		var conectados []string
		for _, r := range rs {
			if r.conectado {
				conectados = append(conectados, r.ID)
			}
		}
		if len(conectados) == 0 {
			continue
		}
		c, cancelar := context.WithTimeout(ctx, 15*time.Second)
		s, err := i.queSuena(c, usuario)
		cancelar()
		if err != nil {
			if !errors.Is(err, ErrSinSpotify) {
				i.log.Info("spotify: no pude ver qué suena", "usuario", usuario, "error", err)
			}
			continue
		}
		i.v19.musicaMu.Lock()
		antes, habia := i.v19.musica[usuario]
		i.v19.musica[usuario] = s
		i.v19.musicaMu.Unlock()
		if habia && antes.ID == s.ID && antes.Sonando == s.Sonando {
			continue
		}
		if !habia && !s.Sonando {
			continue // no sonaba nada y sigue sin sonar
		}
		marco := protocolo.Musica(s.BPM, s.Sonando, s.Titulo, s.Artista)
		for _, id := range conectados {
			i.robots.Enviar(id, protocolo.CarComando, marco) //nolint:errcheck
		}
		i.hub.Publicar(usuario, eventos.Evento{Tipo: "musica", Datos: map[string]any{
			"sonando": s.Sonando, "titulo": s.Titulo, "artista": s.Artista, "bpm": s.BPM, "de": "spotify"}})
	}
}

// musicaYa: un robot se conectó; la próxima vuelta le manda lo que suena.
func (i *Integraciones) musicaYa(bicho string) {
	u := i.robots.Dueno(bicho)
	if u == "" {
		return
	}
	i.v19.musicaMu.Lock()
	delete(i.v19.musica, u)
	i.v19.musicaMu.Unlock()
}

// EstadoMusica: GET /v1/musica.
func (i *Integraciones) EstadoMusica(ctx context.Context, usuario string) (map[string]any, error) {
	if !i.MusicaActiva() {
		return map[string]any{"disponible": false}, nil
	}
	c, err := i.alm.SpotifyDe(ctx, usuario)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return map[string]any{"disponible": true, "conectada": false}, nil
	}
	if err != nil {
		return nil, err
	}
	r := map[string]any{"disponible": true, "conectada": true, "nombre": c.Nombre, "suena": nil}
	i.v19.musicaMu.Lock()
	s, ok := i.v19.musica[usuario]
	i.v19.musicaMu.Unlock()
	if !ok || i.ahora().Sub(s.en) > time.Minute {
		ctx, cancelar := context.WithTimeout(ctx, 8*time.Second)
		defer cancelar()
		if s2, err := i.queSuena(ctx, usuario); err == nil {
			s, ok = s2, true
		}
	}
	if ok && s.Titulo != "" {
		r["suena"] = s
	}
	return r, nil
}

// MandarMusica: lo que suena en el celu, a un robot (POST /v1/bichos/{id}/musica).
func (i *Integraciones) MandarMusica(usuario, bicho string, s bool, titulo, artista string, bpm int) error {
	if err := i.mandarAlBicho(bicho, protocolo.Musica(bpm, s, titulo, artista)); err != nil {
		return err
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "musica", Bicho: bicho, Datos: map[string]any{
		"sonando": s, "titulo": titulo, "artista": artista, "bpm": bpm, "de": "celu"}})
	return nil
}
