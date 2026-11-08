package integraciones

// Leer en voz alta: lo que la nube le manda a decir a un robot con parlante
// (charla, POST decir, automatizaciones, Telegram, MCP: todo pasa por
// nube.Enviar) sale con su audio, y también los globos que el robot muestra
// solo y la nube conoce (recordatorios y el despertador). La voz, en orden:
// 1) ElevenLabs, si el dueño conectó su cuenta y dejó prendido "leer";
// 2) Edge TTS (gratis, prendida por defecto; edge.go); 3) nada: el robot usa
// su voz propia. El audio va por bicho/{id}/audio (protocolo.TrozosAudio)
// justo ANTES del 0x40, así llegan juntos. Ver docs/nube.md, "Voz".

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"sintetica/api/internal/cifra"
	"sintetica/api/internal/edgetts"
	"sintetica/api/internal/protocolo"
)

// PerfilesConParlante: los robots que pueden hablar (hoy, el robot-celu).
var PerfilesConParlante = map[string]bool{"celu": true}

// Parlante: lo que hace falta para mandar audio (nube.Nube lo cumple).
type Parlante interface {
	PublicarEn(id, sufijo string, datos []byte) error
}

type cacheCuenta struct {
	cuenta *cuentaEleven
	en     time.Time
}

type trabajoVoz struct {
	plan   planVoz
	texto  string
	origen byte
	mandar func() error // nil: un globo propio (no hay marco que mandar)
}

type estadoVoz struct {
	cifra *cifra.Cifrador

	mu      sync.Mutex
	cuentas map[string]cacheCuenta
	usoMin  map[string]usoMinutos      // usuario → segundos de conversación de hoy (conversacion.go)
	colas   map[string]chan trabajoVoz // robot → lo que tiene que leer, en orden
	leidos  map[string]time.Time       // robot/evento → cuándo se leyó (sin repetir)

	// Edge TTS (edge.go); edge es nil si el servidor la apagó.
	edge         *edgetts.Cliente
	edgeSem      chan struct{}
	edgeUsuarios map[string]cacheEdge
	edgeVoces    []edgetts.Voz
	edgeVocesEn  time.Time

	ids atomic.Uint32
}

func (i *Integraciones) iniciarVoz(secreto []byte) {
	i.voz = estadoVoz{cifra: cifra.Nuevo(secreto, "elevenlabs"), cuentas: map[string]cacheCuenta{},
		usoMin: map[string]usoMinutos{}, colas: map[string]chan trabajoVoz{}, leidos: map[string]time.Time{},
		edgeUsuarios: map[string]cacheEdge{}}
	i.iniciarEdge()
}

func (i *Integraciones) parlante() Parlante {
	p, _ := i.robots.(Parlante)
	return p
}

// tieneParlante: el robot está conectado y es de un perfil que habla.
func (i *Integraciones) tieneParlante(bicho string) bool {
	e, ok := i.robots.EstadoDe(bicho)
	return ok && PerfilesConParlante[e.Perfil] && i.parlante() != nil
}

// planVoz: con qué se lee (eleven y/o edge; si eleven falla, edge).
type planVoz struct {
	usuario string
	eleven  *cuentaEleven
	edge    string // la voz de Edge ("" = sin Edge)
}

// planDe: cómo lee en voz alta ese robot (false: no lee; usa su voz propia).
func (i *Integraciones) planDe(ctx context.Context, bicho string) (planVoz, bool) {
	if !i.tieneParlante(bicho) {
		return planVoz{}, false
	}
	u := i.robots.Dueno(bicho)
	if u == "" {
		return planVoz{}, false
	}
	p := planVoz{usuario: u}
	if cu, err := i.cuentaEleven(ctx, u); err == nil && cu.Leer && cu.Voz != "" && cu.TopeCaracteres > 0 {
		p.eleven = cu
	}
	if i.EdgeActivo() {
		if v, err := i.vozEdgeDe(ctx, u); err == nil && v.Prendida {
			p.edge = v.Voz
		}
	}
	return p, p.eleven != nil || p.edge != ""
}

// TomarDecir (nube.Voz): si el robot lee en voz alta, se queda con el 0x40
// y lo manda después de su audio. No bloquea más que una lectura de la base.
func (i *Integraciones) TomarDecir(bicho string, marco []byte, mandar func() error) bool {
	if len(marco) < 3 || int(marco[1])+2 >= len(marco) {
		return false
	}
	texto := string(marco[2+int(marco[1]):])
	ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()
	p, ok := i.planDe(ctx, bicho)
	if !ok {
		return false
	}
	return i.encolarVoz(bicho, trabajoVoz{plan: p, texto: texto, origen: protocolo.OrigenDecir, mandar: mandar})
}

// encolarVoz: a la cola del robot (una goroutine por robot, mientras haya algo).
func (i *Integraciones) encolarVoz(bicho string, t trabajoVoz) bool {
	i.voz.mu.Lock()
	defer i.voz.mu.Unlock()
	c, ok := i.voz.colas[bicho]
	if !ok {
		c = make(chan trabajoVoz, 8)
		i.voz.colas[bicho] = c
		go i.leerCola(bicho, c)
	}
	select {
	case c <- t:
		return true
	default:
		return false // muchos seguidos: este va sin audio
	}
}

func (i *Integraciones) leerCola(bicho string, c chan trabajoVoz) {
	for {
		select {
		case t := <-c:
			i.leer(bicho, t)
		case <-time.After(time.Minute):
			i.voz.mu.Lock()
			if len(c) == 0 {
				delete(i.voz.colas, bicho)
				i.voz.mu.Unlock()
				return
			}
			i.voz.mu.Unlock()
		}
	}
}

// audioDe: ElevenLabs y, si no está o falla, Edge. Nil si ninguna.
func (i *Integraciones) audioDe(ctx context.Context, bicho string, p planVoz, texto string) []byte {
	if p.eleven != nil {
		a, err := i.leerConTope(ctx, p.eleven, texto)
		if err == nil {
			return a
		}
		if !errors.Is(err, ErrTopeCaracteres) {
			i.log.Info("voz: ElevenLabs no leyó", "bicho", bicho, "error", err)
		}
	}
	if p.edge != "" {
		a, err := i.hablarEdge(ctx, p.usuario, p.edge, texto)
		if err == nil {
			return a
		}
		if !errors.Is(err, ErrTopeEdge) {
			// No es oficial: puede cambiar. El robot usa su voz propia.
			i.log.Info("voz: Edge TTS no leyó", "bicho", bicho, "error", err)
		}
	}
	return nil
}

// leer: el audio (si se puede) y después el marco.
func (i *Integraciones) leer(bicho string, t trabajoVoz) {
	defer func() {
		if t.mandar != nil {
			if err := t.mandar(); err != nil {
				i.log.Info("voz: no pude mandar el globo", "bicho", bicho, "error", err)
			}
		}
	}()
	ctx, cancelar := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancelar()
	audio := i.audioDe(ctx, bicho, t.plan, t.texto)
	if audio == nil {
		return
	}
	trozos, err := protocolo.TrozosAudio(uint16(i.voz.ids.Add(1)), protocolo.FormatoMP3, t.origen, protocolo.Crc32([]byte(t.texto)), audio)
	if err != nil {
		return
	}
	for _, p := range trozos {
		if err := i.parlante().PublicarEn(bicho, "audio", p); err != nil {
			i.log.Info("voz: no pude mandar el audio", "bicho", bicho, "error", err)
			return
		}
	}
}

// leerGloboPropio: un recordatorio o el despertador que el robot ya está
// mostrando (avisó con el 0x9E): la nube sabe su texto por la config.
func (i *Integraciones) leerGloboPropio(bicho, evento string, datos map[string]any) {
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	p, ok := i.planDe(ctx, bicho)
	if !ok {
		return
	}
	cfg, err := i.alm.UltimaConfig(ctx, bicho)
	if err != nil {
		return
	}
	var texto, clave string
	origen := byte(protocolo.OrigenRecordatorio)
	switch evento {
	case "recordatorio":
		n, ok := datos["indice"].(int)
		rs := recordatoriosDe(cfg.Texto)
		if !ok || n < 0 || n >= len(rs) {
			return
		}
		texto, clave = rs[n].Texto, bicho+"/recordatorio/"+rs[n].Texto
	case "despertador":
		var c struct {
			Despertador struct {
				Texto string `json:"texto"`
			} `json:"despertador"`
		}
		json.Unmarshal([]byte(cfg.Texto), &c) //nolint:errcheck
		texto, clave, origen = c.Despertador.Texto, bicho+"/despertador", protocolo.OrigenDespertador
	default:
		return
	}
	dc := leerDatosConfig(cfg.Texto)
	texto = strings.NewReplacer("{dueno}", strings.TrimSpace(dc.Dueno.Nombre), "{robot}", i.nombreDe(ctx, bicho)).Replace(texto)
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return
	}
	// Sin repetir: el mismo globo, una vez cada 3 minutos.
	i.voz.mu.Lock()
	if en, ya := i.voz.leidos[clave]; ya && i.ahora().Sub(en) < 3*time.Minute {
		i.voz.mu.Unlock()
		return
	}
	if len(i.voz.leidos) > 10000 {
		i.voz.leidos = map[string]time.Time{}
	}
	i.voz.leidos[clave] = i.ahora()
	i.voz.mu.Unlock()
	i.encolarVoz(bicho, trabajoVoz{plan: p, texto: texto, origen: origen})
}

// avisarVoz: le dice al robot (paquete AudioVoz) si la nube lee sus globos,
// para que el robot-celu espere el audio de los suyos en vez de usar su voz.
func (i *Integraciones) avisarVoz(bicho string) {
	if !i.tieneParlante(bicho) {
		return
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelar()
	_, lee := i.planDe(ctx, bicho)
	i.parlante().PublicarEn(bicho, "audio", protocolo.AvisoVoz(lee)) //nolint:errcheck
}

// avisarVozATodos: cambió la cuenta; a todos sus robots conectados.
func (i *Integraciones) avisarVozATodos(ctx context.Context, usuario string) {
	rs, err := i.robotsDe(ctx, usuario)
	if err != nil {
		return
	}
	for _, r := range rs {
		if r.conectado {
			go i.avisarVoz(r.ID)
		}
	}
}

// DecirConVoz (MCP "voz", probar): que el robot lo diga con su voz. Devuelve
// con qué voz sale ("elevenlabs", "edge" o "" si con la suya).
func (i *Integraciones) DecirConVoz(ctx context.Context, bicho, texto string) (string, error) {
	m, err := protocolo.Decir(texto, "")
	if err != nil {
		return "", err
	}
	voz := ""
	if p, ok := i.planDe(ctx, bicho); ok {
		voz = "edge"
		if p.eleven != nil {
			voz = "elevenlabs"
		}
	}
	return voz, i.mandarAlBicho(bicho, m)
}
