package integraciones

// El paquete de voz sin internet: la nube genera UNA vez el mp3 de cada
// frase que el robot dice solo (las de fábrica de shared/formato/vida/
// frases.json según su personalidad, las propias de habla.propias, el
// despertador y los recordatorios, con {dueno} y {robot} ya puestos) y el
// robot se lo baja: así habla con voz real sin wifi y sin costo por frase.
// Las que tienen plantillas que cambian ({clima}, {titulo}, {de}…) no van.
//
// La voz: ElevenLabs si el dueño la conectó (gasta de su cuenta y de su tope:
// el paquete lo pide él, después de ver cuánto va a gastar), si no Edge (con
// su propio tope por día y de a una frase, en segundo plano). Los audios se
// guardan en la base por usuario (voz_clips, clave = sha256 de la voz y el
// texto: si cambia una frase o la voz, se genera solo esa), con un tope de
// 30 MB por cuenta. El robot pide el paquete por MQTT con un tope de bytes y
// recibe una URL firmada que vence en 10 minutos: un tar con indice.json y
// un <hash>.mp3 por frase, primero las más útiles. Ver docs/nube.md,
// "Paquete de voz", y docs/protocolo-ble.md, "Voz".

import (
	"archive/tar"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
	"sintetica/shared/formato"
)

// Lo del paquete.
const (
	TopeBytesPaquetes      = 30 << 20 // por cuenta (todos sus robots y voces)
	MaxTopePaquete         = 64 << 20 // lo más que puede pedir un robot
	TopeEdgePaquete        = 20000    // caracteres por usuario por día con la voz gratis (EDGE_TTS_TOPE_PAQUETE)
	PaquetesALaVez         = 2        // generaciones a la vez en todo el servidor
	VencePaquete           = 10 * time.Minute
	FormatoPaquete         = protocolo.FormatoMP3
	maxFrasesPaquete       = 1000
	estimadoEdgePorLetra   = 500 // bytes de mp3 por letra (48 kbps; medido: "Hola, ¿cómo andás?" son 15 KB)
	estimadoEdgeFijo       = 6000
	estimadoElevenPorLetra = 330 // 32 kbps
	estimadoElevenFijo     = 4000
)

// Estados del paquete.
const (
	PaqueteNuevo     = "nuevo"
	PaqueteGenerando = "generando"
	PaqueteListo     = "listo"
	PaqueteParcial   = "parcial" // faltan frases (el tope del día o el lugar): se sigue pidiendo
	PaqueteError     = "error"
)

// Errores del paquete.
var (
	ErrSinVozPaquete = errors.New("no hay voz para el paquete: prendé la voz gratis o conectá ElevenLabs")
	ErrFirmaPaquete  = errors.New("el link del paquete no es válido o venció: pedí otro")
	ErrVersionPaq    = errors.New("el paquete cambió: pedí el link de nuevo")
)

// ErrConfirmarPaquete: con ElevenLabs, generar gasta de su cuenta; hay que
// pedirlo con confirmar (409 con el estado, que dice cuánto).
type ErrConfirmarPaquete struct{ Estado map[string]any }

func (e ErrConfirmarPaquete) Error() string {
	return "generar el paquete con ElevenLabs gasta de tu cuenta: confirmalo"
}

// FrasePaquete: una frase del paquete. hash = CRC-32 (IEEE) del texto final
// en UTF-8, en hex de 8 letras (lo mismo que el audio tipo 2): el robot lo
// calcula del globo que va a mostrar y busca <hash>.mp3.
type FrasePaquete struct {
	Situacion string `json:"situacion"`
	Preset    string `json:"preset,omitempty"` // "" = de todos; o el preset de esa frase
	Propia    bool   `json:"propia,omitempty"` // del usuario (habla.propias, el despertador o un recordatorio)
	Texto     string `json:"texto"`
	Hash      string `json:"hash"`
	Bytes     int    `json:"bytes,omitempty"`
}

// fraseGuardada: lo que va en voz_paquetes.frases (con la clave del audio).
type fraseGuardada struct {
	FrasePaquete
	Clave string `json:"clave"`
}

// HashFrase: el hash de una frase (CRC-32 del texto en hex).
func HashFrase(texto string) string { return fmt.Sprintf("%08x", protocolo.Crc32([]byte(texto))) }

func claveClip(voz, texto string) string {
	h := sha256.Sum256([]byte(voz + "\x00" + texto))
	return hex.EncodeToString(h[:16])
}

// --- Las frases de fábrica ---

type situacionBase struct {
	nombre string
	prob   float64
	frases map[string][]string // "todos" o el preset → frases
}

var (
	frasesBaseUna sync.Once
	frasesBase    []situacionBase
	presetsRasgos map[string][6]float64
)

// Los presets en el orden del núcleo (habla.cpp, PRESETS) y los rasgos en el
// suyo (energia, sociabilidad, picardia, timidez, curiosidad, malGenio).
var (
	ordenPresets = []string{"equilibrado", "timido", "picaro", "dormilon", "carinoso", "curioso", "grunon", "electrico"}
	ordenRasgos  = []string{"energia", "sociabilidad", "picardia", "timidez", "curiosidad", "malGenio"}
)

// situacionesFrecuentes: lo primero que entra en un paquete chico.
var situacionesFrecuentes = []string{"despertar", "saludo", "buenas-noches", "volviste", "caricia", "como-estas",
	"te-extrane", "aburrido", "sueno", "despertador", "posponer", "bateria-baja", "cargando"}

func cargarFrasesBase() {
	frasesBaseUna.Do(func() {
		var f struct {
			Situaciones map[string]struct {
				Probabilidad float64             `json:"probabilidad"`
				Frases       map[string][]string `json:"frases"`
			} `json:"situaciones"`
		}
		json.Unmarshal(formato.Frases(), &f) //nolint:errcheck
		for n, s := range f.Situaciones {
			frasesBase = append(frasesBase, situacionBase{nombre: n, prob: s.Probabilidad, frases: s.Frases})
		}
		// Las frecuentes primero (en su orden); el resto, las más probables primero.
		prio := func(n string) int {
			for k, x := range situacionesFrecuentes {
				if x == n {
					return k
				}
			}
			return len(situacionesFrecuentes)
		}
		sort.SliceStable(frasesBase, func(a, b int) bool {
			pa, pb := prio(frasesBase[a].nombre), prio(frasesBase[b].nombre)
			if pa != pb {
				return pa < pb
			}
			if frasesBase[a].prob != frasesBase[b].prob {
				return frasesBase[a].prob > frasesBase[b].prob
			}
			return frasesBase[a].nombre < frasesBase[b].nombre
		})
		var p map[string]map[string]float64
		json.Unmarshal(formato.Personalidades(), &p) //nolint:errcheck
		presetsRasgos = map[string][6]float64{}
		for n, r := range p {
			var v [6]float64
			for k, x := range ordenRasgos {
				v[k] = r[x]
			}
			presetsRasgos[n] = v
		}
	})
}

// presetCercano: como el del núcleo (habla.cpp): el preset más parecido a
// los rasgos (el preset de la config con los que vengan encima).
func presetCercano(preset string, rasgos map[string]float64) string {
	cargarFrasesBase()
	base, ok := presetsRasgos[preset]
	if !ok {
		base = presetsRasgos["equilibrado"]
	}
	for k, x := range ordenRasgos {
		if v, ok := rasgos[x]; ok {
			base[k] = v
		}
	}
	mejor, dMejor := ordenPresets[0], 1e9
	for _, n := range ordenPresets {
		q, ok := presetsRasgos[n]
		if !ok {
			continue
		}
		d := 0.0
		for k := range q {
			d += (base[k] - q[k]) * (base[k] - q[k])
		}
		if d < dMejor {
			mejor, dMejor = n, d
		}
	}
	return mejor
}

// completarFrase: como la del núcleo (habla.cpp, completarFrase) con solo
// {dueno} y {robot}: sin dueño se lleva lo de antes ("¡Hola, {dueno}!" →
// "¡Hola!") o, al principio, lo de después; se corta a 60 bytes sin partir
// letras. false: no va en el paquete (empieza con {dueno} sin dueño, o tiene
// otra plantilla que cambia cada vez).
func completarFrase(p, dueno, robot string) (string, bool) {
	if dueno == "" && strings.HasPrefix(p, "{dueno}") {
		return "", false
	}
	var b []byte
	for k := 0; k < len(p); {
		if p[k] == '{' {
			var valor string
			lk := 0
			switch {
			case strings.HasPrefix(p[k:], "{dueno}"):
				valor, lk = dueno, 7
			case strings.HasPrefix(p[k:], "{robot}"):
				valor, lk = robot, 7
			default:
				if j := strings.IndexByte(p[k:], '}'); j > 1 {
					return "", false // {clima}, {titulo}, {de}…: cambia cada vez
				}
			}
			if lk > 0 {
				if valor != "" {
					b = append(b, valor...)
				} else {
					for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == ',' || b[len(b)-1] == ':') {
						b = b[:len(b)-1]
					}
					if len(b) == 0 {
						for k+lk < len(p) && (p[k+lk] == ',' || p[k+lk] == ' ' || p[k+lk] == ':') {
							k++
						}
					}
				}
				k += lk
				continue
			}
		}
		b = append(b, p[k])
		k++
	}
	t := protocolo.CortarUTF8(string(b), protocolo.MaxDecir)
	if strings.TrimSpace(t) == "" || !utf8.ValidString(t) {
		return "", false
	}
	return t, true
}

// configHabla: lo de la config que hace falta para las frases.
type configHabla struct {
	Nombre       string `json:"nombre"`
	Personalidad struct {
		Preset string `json:"preset"`
	} `json:"personalidad"`
	Habla struct {
		Propias map[string][]string `json:"propias"`
	} `json:"habla"`
	Despertador struct {
		Texto string `json:"texto"`
	} `json:"despertador"`
}

// FrasesDelRobot: las que el robot dice solo, en orden de prioridad: las de
// las situaciones más frecuentes, después las propias del usuario (habla,
// despertador y recordatorios) y después el resto. En cada situación, las de
// su preset primero. Si el usuario tiene propias para una situación, las de
// fábrica de esa no van (el robot dice solo las suyas). Sin repetir textos
// ni hashes.
func FrasesDelRobot(cfg, robot string) []FrasePaquete {
	cargarFrasesBase()
	var c configHabla
	json.Unmarshal([]byte(cfg), &c) //nolint:errcheck
	var r struct {
		Personalidad map[string]any `json:"personalidad"`
	}
	json.Unmarshal([]byte(cfg), &r) //nolint:errcheck
	rasgos := map[string]float64{}
	for k, v := range r.Personalidad {
		if f, ok := v.(float64); ok {
			rasgos[k] = f
		}
	}
	if n := strings.TrimSpace(c.Nombre); n != "" {
		robot = n
	}
	dueno := strings.TrimSpace(leerDatosConfig(cfg).Dueno.Nombre)
	cercano := presetCercano(c.Personalidad.Preset, rasgos)
	presets := []string{cercano}
	if p := c.Personalidad.Preset; p != "" && p != cercano {
		presets = append(presets, p)
	}

	var out []FrasePaquete
	vistos := map[string]bool{}
	agregar := func(f FrasePaquete, plantilla string) {
		t, ok := completarFrase(plantilla, dueno, robot)
		if !ok || len(out) >= maxFrasesPaquete {
			return
		}
		f.Texto, f.Hash = t, HashFrase(t)
		if vistos[f.Hash] {
			return
		}
		vistos[f.Hash] = true
		out = append(out, f)
	}
	deFabrica := func(s situacionBase) {
		if len(c.Habla.Propias[s.nombre]) > 0 {
			return
		}
		for _, p := range presets {
			for _, t := range s.frases[p] {
				agregar(FrasePaquete{Situacion: s.nombre, Preset: p}, t)
			}
		}
		for _, t := range s.frases["todos"] {
			agregar(FrasePaquete{Situacion: s.nombre}, t)
		}
	}
	frecuente := func(n string) bool {
		for _, x := range situacionesFrecuentes {
			if x == n {
				return true
			}
		}
		return false
	}
	for _, s := range frasesBase {
		if frecuente(s.nombre) {
			deFabrica(s)
		}
	}
	// Las propias, en el orden de las situaciones.
	nombres := make([]string, 0, len(c.Habla.Propias))
	for n := range c.Habla.Propias {
		nombres = append(nombres, n)
	}
	orden := map[string]int{}
	for k, s := range frasesBase {
		orden[s.nombre] = k
	}
	sort.SliceStable(nombres, func(a, b int) bool {
		oa, okA := orden[nombres[a]]
		ob, okB := orden[nombres[b]]
		if okA != okB {
			return okA
		}
		if oa != ob {
			return oa < ob
		}
		return nombres[a] < nombres[b]
	})
	for _, n := range nombres {
		for _, t := range c.Habla.Propias[n] {
			agregar(FrasePaquete{Situacion: n, Propia: true}, t)
		}
	}
	if t := strings.TrimSpace(c.Despertador.Texto); t != "" {
		agregar(FrasePaquete{Situacion: "despertador", Propia: true}, t)
	}
	for _, rc := range recordatoriosDe(cfg) {
		if t := strings.TrimSpace(rc.Texto); t != "" {
			agregar(FrasePaquete{Situacion: "recordatorio", Propia: true}, t)
		}
	}
	for _, s := range frasesBase {
		if !frecuente(s.nombre) {
			deFabrica(s)
		}
	}
	return out
}

// --- La voz y el plan ---

// vozPaquete: con qué se genera (la misma que lee en voz alta).
type vozPaquete struct {
	proveedor string // "elevenlabs" | "edge"
	id        string
	eleven    *cuentaEleven
}

func (v vozPaquete) nombre() string { return v.proveedor + ":" + v.id }

func (i *Integraciones) vozDelPaquete(ctx context.Context, usuario string) (vozPaquete, error) {
	if cu, err := i.cuentaEleven(ctx, usuario); err == nil && cu.Leer && cu.Voz != "" && cu.TopeCaracteres > 0 {
		return vozPaquete{proveedor: "elevenlabs", id: cu.Voz, eleven: cu}, nil
	}
	if i.EdgeActivo() {
		if v, err := i.vozEdgeDe(ctx, usuario); err == nil && v.Prendida {
			return vozPaquete{proveedor: "edge", id: v.Voz}, nil
		}
	}
	return vozPaquete{}, ErrSinVozPaquete
}

func (v vozPaquete) estimar(texto string) int {
	n := utf8.RuneCountInString(texto)
	if v.proveedor == "elevenlabs" {
		return estimadoElevenFijo + n*estimadoElevenPorLetra
	}
	return estimadoEdgeFijo + n*estimadoEdgePorLetra
}

// planPaquete: lo que habría que tener para ese robot con esa voz y ese tope.
type planPaquete struct {
	voz        vozPaquete
	frases     []fraseGuardada // todas, con Bytes si ya tienen audio
	tienen     map[string]int  // clave → bytes
	faltan     []fraseGuardada // las que entran en el tope y no tienen audio
	caracteres int             // los de faltan
	estimados  int             // bytes del paquete con todo lo que entra
}

func (i *Integraciones) planDelPaquete(ctx context.Context, usuario, bicho string, tope int) (planPaquete, error) {
	v, err := i.vozDelPaquete(ctx, usuario)
	if err != nil {
		return planPaquete{}, err
	}
	cfg := ""
	if c, err := i.alm.UltimaConfig(ctx, bicho); err == nil {
		cfg = c.Texto
	} else if !errors.Is(err, almacen.ErrNoEncontrado) {
		return planPaquete{}, err
	}
	p := planPaquete{voz: v}
	claves := []string{}
	for _, f := range FrasesDelRobot(cfg, i.nombreDe(ctx, bicho)) {
		g := fraseGuardada{FrasePaquete: f, Clave: claveClip(v.nombre(), f.Texto)}
		p.frases = append(p.frases, g)
		claves = append(claves, g.Clave)
	}
	if p.tienen, err = i.alm.TamanosClips(ctx, usuario, claves); err != nil {
		return planPaquete{}, err
	}
	for k := range p.frases {
		f := &p.frases[k]
		n, ya := p.tienen[f.Clave]
		if ya {
			f.Bytes = n
		} else {
			n = v.estimar(f.Texto)
		}
		if tope > 0 && p.estimados+n > tope {
			continue
		}
		p.estimados += n
		if !ya {
			p.faltan = append(p.faltan, *f)
			p.caracteres += utf8.RuneCountInString(f.Texto)
		}
	}
	return p, nil
}

// --- El estado ---

type progresoPaquete struct {
	hechas, de int
}

type estadoPaquete struct {
	mu      sync.Mutex
	enCurso map[string]*progresoPaquete // robot → lo que va
	avisar  map[string]bool             // robot → pidió por MQTT: mandarle la URL al terminar
	sem     chan struct{}
	firma   []byte
	pausa   time.Duration // entre frase y frase (Edge: sin saturar)
}

func (i *Integraciones) iniciarPaquete(secreto []byte) {
	k := sha256.Sum256(append([]byte("sintetica/voz-paquete/v1/"), secreto...))
	i.paq = estadoPaquete{enCurso: map[string]*progresoPaquete{}, avisar: map[string]bool{},
		sem: make(chan struct{}, PaquetesALaVez), firma: k[:], pausa: 300 * time.Millisecond}
}

func (i *Integraciones) topeEdgePaquete() int {
	if t := i.op.V19.EdgeTopePaquete; t > 0 {
		return t
	}
	return TopeEdgePaquete
}

// paqueteDe: el guardado (vacío si es de otro dueño o no hay).
func (i *Integraciones) paqueteDe(ctx context.Context, usuario, bicho string) (almacen.PaqueteVoz, []fraseGuardada, error) {
	p, err := i.alm.PaqueteVozDe(ctx, bicho)
	if errors.Is(err, almacen.ErrNoEncontrado) || (err == nil && p.UsuarioID != usuario) {
		return almacen.PaqueteVoz{BichoID: bicho, UsuarioID: usuario, Estado: PaqueteNuevo}, nil, nil
	}
	if err != nil {
		return p, nil, err
	}
	var fs []fraseGuardada
	json.Unmarshal(p.Frases, &fs) //nolint:errcheck
	return p, fs, nil
}

// EstadoPaqueteVoz: GET /v1/bichos/{id}/voz/paquete[?tope=bytes].
func (i *Integraciones) EstadoPaqueteVoz(ctx context.Context, usuario, bicho string, tope int) (map[string]any, error) {
	if tope < 0 || tope > MaxTopePaquete {
		return nil, ErrCambioVoz{fmt.Sprintf("el tope es de 0 (sin tope) a %d bytes", MaxTopePaquete)}
	}
	g, fs, err := i.paqueteDe(ctx, usuario, bicho)
	if err != nil {
		return nil, err
	}
	if tope == 0 {
		tope = g.TopeBytes
	}
	usados, err := i.alm.BytesClips(ctx, usuario)
	if err != nil {
		return nil, err
	}
	bytes := 0
	for _, f := range elegirFrases(fs, tope) {
		bytes += f.Bytes
	}
	r := map[string]any{"estado": g.Estado, "version": g.Version, "voz": g.Voz, "frases": len(elegirFrases(fs, tope)),
		"bytes": bytes, "tope": tope, "usados_bytes": usados, "tope_bytes": TopeBytesPaquetes}
	if g.Error != "" {
		r["error"] = g.Error
	}
	if g.Version > 0 {
		r["actualizado"] = g.Actualizado
	}
	i.paq.mu.Lock()
	if pr, ok := i.paq.enCurso[bicho]; ok {
		r["estado"] = PaqueteGenerando
		r["progreso"] = map[string]int{"hechas": pr.hechas, "de": pr.de}
	}
	i.paq.mu.Unlock()
	p, err := i.planDelPaquete(ctx, usuario, bicho, tope)
	if errors.Is(err, ErrSinVozPaquete) {
		r["disponible"] = false
		r["motivo"] = err.Error()
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	r["disponible"] = true
	r["proveedor"] = p.voz.proveedor
	r["voz_nueva"] = p.voz.nombre()
	r["total"] = len(p.frases)
	r["faltan"] = len(p.faltan)
	r["caracteres"] = p.caracteres     // lo que va a leer (con ElevenLabs, lo que gasta)
	r["bytes_estimados"] = p.estimados // el paquete con todo lo que entra en el tope
	hoy := i.ahora().In(nube.Zona())
	if p.voz.eleven != nil {
		n, _ := i.alm.CaracteresDeHoy(ctx, usuario, hoy)
		r["gasta"] = p.caracteres // caracteres de tu cuenta de ElevenLabs
		r["tope_dia"] = p.voz.eleven.TopeCaracteres
		r["quedan_hoy"] = max(0, p.voz.eleven.TopeCaracteres-n)
	} else {
		n, _ := i.alm.CaracteresPaqueteDeHoy(ctx, usuario, hoy)
		r["gasta"] = 0
		r["tope_dia"] = i.topeEdgePaquete()
		r["quedan_hoy"] = max(0, i.topeEdgePaquete()-n)
	}
	return r, nil
}

// elegirFrases: las que entran en el tope (en orden; una grande que no
// entra no corta: sigue con las que sí).
func elegirFrases(fs []fraseGuardada, tope int) []fraseGuardada {
	var out []fraseGuardada
	n := 0
	for _, f := range fs {
		if tope > 0 && n+f.Bytes > tope {
			continue
		}
		n += f.Bytes
		out = append(out, f)
	}
	return out
}

// GenerarPaqueteVoz: POST /v1/bichos/{id}/voz/paquete {confirmar?, tope?}.
// Arranca en segundo plano y devuelve el estado. Con ElevenLabs, si gasta
// algo y no vino confirmar, ErrConfirmarPaquete (con lo que gastaría).
func (i *Integraciones) GenerarPaqueteVoz(ctx context.Context, usuario, bicho string, confirmar bool, tope int) (map[string]any, error) {
	if tope < 0 || tope > MaxTopePaquete {
		return nil, ErrCambioVoz{fmt.Sprintf("el tope es de 0 (sin tope) a %d bytes", MaxTopePaquete)}
	}
	if _, err := i.alm.BichoDe(ctx, bicho, usuario); err != nil {
		return nil, err
	}
	p, err := i.planDelPaquete(ctx, usuario, bicho, tope)
	if err != nil {
		return nil, err
	}
	if p.voz.eleven != nil && p.caracteres > 0 && !confirmar {
		e, err := i.EstadoPaqueteVoz(ctx, usuario, bicho, tope)
		if err != nil {
			return nil, err
		}
		return nil, ErrConfirmarPaquete{e}
	}
	i.arrancarPaquete(usuario, bicho, tope, false)
	return i.EstadoPaqueteVoz(ctx, usuario, bicho, tope)
}

// arrancarPaquete: genera en segundo plano (si ya está generando, nada).
func (i *Integraciones) arrancarPaquete(usuario, bicho string, tope int, avisar bool) {
	i.paq.mu.Lock()
	if avisar {
		i.paq.avisar[bicho] = true
	}
	if _, ya := i.paq.enCurso[bicho]; ya {
		i.paq.mu.Unlock()
		return
	}
	pr := &progresoPaquete{}
	i.paq.enCurso[bicho] = pr
	i.paq.mu.Unlock()
	go func() {
		defer func() {
			i.paq.mu.Lock()
			delete(i.paq.enCurso, bicho)
			aviso := i.paq.avisar[bicho]
			delete(i.paq.avisar, bicho)
			i.paq.mu.Unlock()
			if aviso {
				i.mandarURLPaquete(bicho, usuario, tope)
			}
		}()
		i.paq.sem <- struct{}{}
		defer func() { <-i.paq.sem }()
		i.generarPaquete(usuario, bicho, tope, pr)
	}()
}

// generarPaquete: lo que falta, de a una frase, y guarda el paquete.
func (i *Integraciones) generarPaquete(usuario, bicho string, tope int, pr *progresoPaquete) {
	ctx, cancelar := context.WithTimeout(context.Background(), time.Hour)
	defer cancelar()
	i.alm.PodarClips(ctx, usuario) //nolint:errcheck
	p, err := i.planDelPaquete(ctx, usuario, bicho, tope)
	g, _, _ := i.paqueteDe(ctx, usuario, bicho)
	if err != nil {
		g.Estado, g.Error = PaqueteError, MensajeVoz(err)
		if errors.Is(err, ErrSinVozPaquete) {
			g.Error = err.Error()
		}
		i.alm.GuardarPaqueteVoz(ctx, g) //nolint:errcheck
		return
	}
	i.paq.mu.Lock()
	pr.de = len(p.faltan)
	i.paq.mu.Unlock()
	motivo := ""
	fallas := 0
	for k, f := range p.faltan {
		if k > 0 && i.paq.pausa > 0 {
			time.Sleep(i.paq.pausa)
		}
		audio, err := i.audioPaquete(ctx, usuario, p.voz, f.Texto)
		if err != nil && !errors.Is(err, ErrTopeCaracteres) && !errors.Is(err, ErrTopeEdge) {
			fallas++
			time.Sleep(3 * i.paq.pausa)
			audio, err = i.audioPaquete(ctx, usuario, p.voz, f.Texto)
		}
		if errors.Is(err, ErrTopeCaracteres) || errors.Is(err, ErrTopeEdge) {
			motivo = "por hoy se llenó el tope de la voz: mañana sigue (pedilo de nuevo)"
			break
		}
		if err != nil {
			if fallas >= 3 {
				motivo = "la voz no contestó: probá en un rato"
				i.log.Info("paquete de voz: la voz no contestó", "bicho", bicho, "voz", p.voz.proveedor, "error", err)
				break
			}
			continue
		}
		ok, err := i.alm.GuardarClip(ctx, usuario, f.Clave, p.voz.nombre(), f.Texto, audio, TopeBytesPaquetes)
		if err != nil {
			motivo = "no pude guardar el audio"
			break
		}
		if !ok {
			motivo = fmt.Sprintf("no hay más lugar para audios (%d MB por cuenta)", TopeBytesPaquetes>>20)
			break
		}
		p.tienen[f.Clave] = len(audio)
		i.paq.mu.Lock()
		pr.hechas++
		i.paq.mu.Unlock()
	}
	// Lo que quedó: todas las que tienen audio, en orden.
	var fs []fraseGuardada
	var claves []string
	for _, f := range p.frases {
		if n, ok := p.tienen[f.Clave]; ok {
			f.Bytes = n
			fs = append(fs, f)
			claves = append(claves, f.Clave)
		}
	}
	h := sha256.Sum256([]byte(p.voz.nombre() + "\n" + strings.Join(claves, "\n")))
	huella := hex.EncodeToString(h[:16])
	if huella != g.Huella {
		g.Version++
	}
	b, _ := json.Marshal(fs)
	g.UsuarioID, g.Voz, g.Huella, g.Frases, g.TopeBytes, g.Error = usuario, p.voz.nombre(), huella, b, tope, motivo
	g.Estado = PaqueteListo
	if motivo != "" {
		g.Estado = PaqueteParcial
	}
	if err := i.alm.GuardarPaqueteVoz(ctx, g); err != nil {
		i.log.Warn("paquete de voz: no lo pude guardar", "bicho", bicho, "error", err)
		return
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "voz-paquete", Bicho: bicho, Datos: map[string]any{
		"estado": g.Estado, "version": g.Version, "frases": len(fs)}})
}

// audioPaquete: el mp3 de una frase con la voz del paquete.
func (i *Integraciones) audioPaquete(ctx context.Context, usuario string, v vozPaquete, texto string) ([]byte, error) {
	if v.eleven != nil {
		c, cancelar := context.WithTimeout(ctx, 20*time.Second)
		defer cancelar()
		return i.leerConTope(c, v.eleven, texto)
	}
	if i.voz.edge == nil {
		return nil, ErrEdgeApagado
	}
	n := utf8.RuneCountInString(texto)
	if _, ok, err := i.alm.UsarCaracteresPaquete(ctx, usuario, i.ahora().In(nube.Zona()), n, i.topeEdgePaquete()); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrTopeEdge
	}
	c, cancelar := context.WithTimeout(ctx, 20*time.Second)
	defer cancelar()
	select {
	case i.voz.edgeSem <- struct{}{}:
		defer func() { <-i.voz.edgeSem }()
	case <-c.Done():
		return nil, c.Err()
	}
	return i.voz.edge.Hablar(c, v.id, texto)
}

// --- El robot lo pide por MQTT ---

// PaqueteVozMQTT: el robot publicó en bicho/{id}/voz/paquete {"pedir": true,
// "tope"?: bytes}. Contesta en bicho/{id}/voz/paquete/url con la URL (o
// {"estado": "generando"}, y la URL cuando termina; {"estado":
// "falta-pedir"} si con ElevenLabs lo tiene que pedir el dueño; o
// {"error"}). No bloquea.
func (i *Integraciones) PaqueteVozMQTT(bicho string, datos []byte) {
	var q struct {
		Pedir bool `json:"pedir"`
		Tope  int  `json:"tope"`
	}
	if len(datos) > 1024 || json.Unmarshal(datos, &q) != nil || !q.Pedir {
		return
	}
	q.Tope = min(max(q.Tope, 0), MaxTopePaquete)
	usuario := i.robots.Dueno(bicho)
	if usuario == "" || !i.limite.permitir("paquete-mqtt/"+bicho, 5*time.Second, 20, time.Hour) {
		return
	}
	go func() {
		ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancelar()
		contestar := func(v any) {
			b, _ := json.Marshal(v)
			if pa := i.parlante(); pa != nil {
				pa.PublicarEn(bicho, "voz/paquete/url", b) //nolint:errcheck
			}
		}
		i.paq.mu.Lock()
		_, generando := i.paq.enCurso[bicho]
		if generando {
			i.paq.avisar[bicho] = true
		}
		i.paq.mu.Unlock()
		if generando {
			contestar(map[string]any{"estado": PaqueteGenerando})
			return
		}
		p, err := i.planDelPaquete(ctx, usuario, bicho, q.Tope)
		if err != nil {
			contestar(map[string]any{"error": MensajeVozPaquete(err)})
			return
		}
		g, _, err := i.paqueteDe(ctx, usuario, bicho)
		if err != nil {
			contestar(map[string]any{"error": "no pude leer el paquete"})
			return
		}
		mismo := g.Version > 0 && g.Voz == p.voz.nombre()
		switch {
		case len(p.faltan) == 0 && mismo:
			i.mandarURLPaquete(bicho, usuario, q.Tope)
		case p.voz.eleven == nil:
			contestar(map[string]any{"estado": PaqueteGenerando, "faltan": len(p.faltan)})
			i.arrancarPaquete(usuario, bicho, q.Tope, true)
		case mismo:
			// Con ElevenLabs, lo que hay (el índice dice qué entró); lo
			// que falta lo pide el dueño.
			i.mandarURLPaquete(bicho, usuario, q.Tope)
		default:
			contestar(map[string]any{"estado": "falta-pedir", "gasta": p.caracteres})
		}
	}()
}

// MensajeVozPaquete: el error para el robot o el usuario.
func MensajeVozPaquete(err error) string {
	if errors.Is(err, ErrSinVozPaquete) {
		return err.Error()
	}
	return MensajeVoz(err)
}

// mandarURLPaquete: la URL firmada por bicho/{id}/voz/paquete/url.
func (i *Integraciones) mandarURLPaquete(bicho, usuario string, tope int) {
	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	pa := i.parlante()
	if pa == nil {
		return
	}
	u, info, err := i.URLPaqueteVoz(ctx, usuario, bicho, tope)
	var b []byte
	if err != nil {
		b, _ = json.Marshal(map[string]any{"error": MensajeVozPaquete(err)})
	} else {
		info["url"] = u
		b, _ = json.Marshal(info)
	}
	pa.PublicarEn(bicho, "voz/paquete/url", b) //nolint:errcheck
}

// URLPaqueteVoz: la URL firmada (vence en 10 min) y {version, voz, formato,
// frases, bytes, vence}. Sin paquete, ErrNoEncontrado.
func (i *Integraciones) URLPaqueteVoz(ctx context.Context, usuario, bicho string, tope int) (string, map[string]any, error) {
	g, fs, err := i.paqueteDe(ctx, usuario, bicho)
	if err != nil {
		return "", nil, err
	}
	if g.Version == 0 {
		return "", nil, almacen.ErrNoEncontrado
	}
	el := elegirFrases(fs, tope)
	bytes := 0
	for _, f := range el {
		bytes += f.Bytes
	}
	vence := i.ahora().Add(VencePaquete).Unix()
	q := url.Values{"v": {strconv.Itoa(g.Version)}, "tope": {strconv.Itoa(tope)}, "vence": {strconv.FormatInt(vence, 10)},
		"firma": {i.firmaPaquete(bicho, g.Version, tope, vence)}}
	u := strings.TrimRight(i.op.V19.APIURL, "/") + "/v1/voz/paquete/" + bicho + "?" + q.Encode()
	return u, map[string]any{"version": g.Version, "voz": g.Voz, "formato": FormatoPaquete, "frases": len(el), "bytes": bytes,
		"vence": vence, "estado": g.Estado}, nil
}

func (i *Integraciones) firmaPaquete(bicho string, version, tope int, vence int64) string {
	m := hmac.New(sha256.New, i.paq.firma)
	fmt.Fprintf(m, "%s.%d.%d.%d", bicho, version, tope, vence)
	return hex.EncodeToString(m.Sum(nil))[:32]
}

// IndicePaquete: lo que va en indice.json (y en GET …?indice=1).
type IndicePaquete struct {
	Version int            `json:"version"`
	Voz     string         `json:"voz"`
	Formato int            `json:"formato"` // 1 = mp3 mono (el formato 1 del audio tipo 2)
	Tope    int            `json:"tope"`    // 0 = sin tope
	Bytes   int            `json:"bytes"`   // la suma de los mp3
	Frases  []FrasePaquete `json:"frases"`  // las que entraron (hay un <hash>.mp3 por cada una), en orden de prioridad
	Fuera   []FrasePaquete `json:"fuera"`   // las que dice solo y no entraron: pedirlas a la nube o con su voz propia
}

// PaqueteBajado: lo que se manda al robot.
type PaqueteBajado struct {
	Indice IndicePaquete
	audios map[string][]byte // hash → mp3
}

// AbrirPaqueteVoz: la bajada sin sesión (GET /v1/voz/paquete/{id}?v&tope&vence&firma).
func (i *Integraciones) AbrirPaqueteVoz(ctx context.Context, bicho string, q url.Values) (*PaqueteBajado, error) {
	version, err1 := strconv.Atoi(q.Get("v"))
	tope, err2 := strconv.Atoi(q.Get("tope"))
	vence, err3 := strconv.ParseInt(q.Get("vence"), 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || !protocolo.IDValido(bicho) || tope < 0 || tope > MaxTopePaquete ||
		i.ahora().Unix() > vence || vence > i.ahora().Add(VencePaquete+time.Minute).Unix() ||
		!hmac.Equal([]byte(i.firmaPaquete(bicho, version, tope, vence)), []byte(q.Get("firma"))) {
		return nil, ErrFirmaPaquete
	}
	g, err := i.alm.PaqueteVozDe(ctx, bicho)
	if err != nil {
		return nil, err
	}
	if g.Version != version {
		return nil, ErrVersionPaq
	}
	var fs []fraseGuardada
	json.Unmarshal(g.Frases, &fs) //nolint:errcheck
	el := elegirFrases(fs, tope)
	claves := make([]string, len(el))
	for k, f := range el {
		claves[k] = f.Clave
	}
	clips, err := i.alm.LeerClips(ctx, g.UsuarioID, claves)
	if err != nil {
		return nil, err
	}
	b := &PaqueteBajado{Indice: IndicePaquete{Version: g.Version, Voz: g.Voz, Formato: FormatoPaquete, Tope: tope,
		Frases: []FrasePaquete{}, Fuera: []FrasePaquete{}}, audios: map[string][]byte{}}
	adentro := map[string]bool{}
	for _, f := range el {
		a, ok := clips[f.Clave]
		if !ok {
			continue
		}
		f.Bytes = len(a)
		b.Indice.Frases = append(b.Indice.Frases, f.FrasePaquete)
		b.Indice.Bytes += len(a)
		b.audios[f.Hash] = a
		adentro[f.Hash] = true
	}
	// Las que no entraron: las de su config de ahora que no están.
	cfg := ""
	if c, err := i.alm.UltimaConfig(ctx, bicho); err == nil {
		cfg = c.Texto
	}
	for _, f := range FrasesDelRobot(cfg, i.nombreDe(ctx, bicho)) {
		if !adentro[f.Hash] {
			b.Indice.Fuera = append(b.Indice.Fuera, f)
		}
	}
	return b, nil
}

// EscribirTar: indice.json primero y después un <hash>.mp3 por frase, en
// el orden del índice (un tar ustar común).
func (b *PaqueteBajado) EscribirTar(w io.Writer) error {
	t := tar.NewWriter(w)
	ind, _ := json.Marshal(b.Indice)
	ahora := time.Unix(0, 0)
	if err := t.WriteHeader(&tar.Header{Name: "indice.json", Mode: 0o644, Size: int64(len(ind)), ModTime: ahora, Format: tar.FormatUSTAR}); err != nil {
		return err
	}
	if _, err := t.Write(ind); err != nil {
		return err
	}
	for _, f := range b.Indice.Frases {
		a := b.audios[f.Hash]
		if err := t.WriteHeader(&tar.Header{Name: f.Hash + ".mp3", Mode: 0o644, Size: int64(len(a)), ModTime: ahora, Format: tar.FormatUSTAR}); err != nil {
			return err
		}
		if _, err := t.Write(a); err != nil {
			return err
		}
	}
	return t.Close()
}

// Tamano: lo que pesa el tar (para el Content-Length).
func (b *PaqueteBajado) Tamano() int64 {
	ind, _ := json.Marshal(b.Indice)
	bloque := func(n int) int64 { return 512 + int64((n+511)/512*512) }
	total := bloque(len(ind))
	for _, f := range b.Indice.Frases {
		total += bloque(len(b.audios[f.Hash]))
	}
	return total + 1024 // los dos bloques vacíos del final
}
