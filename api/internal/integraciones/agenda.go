package integraciones

// La agenda: el calendario iCal del dueño (la "dirección secreta" de Google
// Calendar o cualquier .ics). Se baja con las protecciones de los webhooks,
// se lee cada 5 min, se avisa `aviso` minutos antes a los robots conectados
// (marco 0x48, evento SSE "agenda" y el cuando "agenda" de las
// automatizaciones) y a la hora de dormir va el resumen de mañana (0x48 con
// 0xFFFF y, con Telegram, la lista). Ver docs/nube.md, "Agenda".

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"sintetica/api/internal/agenda"
	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

// Límites de la agenda.
const (
	CadaAgenda       = 5 * time.Minute
	MaxICS           = 2 << 20 // bytes del .ics
	AvisoPorDefecto  = 10      // minutos antes
	MaxAviso         = 15      // el robot muestra el globo con 0 a 15 minutos
	VentanaAgenda    = 14 * 24 * time.Hour
	MaxProximos      = 300 // ocurrencias que se guardan
	ProximosVisibles = 10
)

// ErrAgendaInvalida: la dirección no sirve (el motivo se le muestra al usuario).
type ErrAgendaInvalida struct{ Motivo string }

func (e ErrAgendaInvalida) Error() string { return e.Motivo }

// ErrSinAgenda: el usuario no conectó un calendario.
var ErrSinAgenda = errors.New("no conectaste un calendario")

// VistaAgenda es lo que ve el usuario (nunca la dirección entera).
type VistaAgenda struct {
	Conectada bool            `json:"conectada"`
	ICS       string          `json:"ics,omitempty"` // la pista: el host y el final
	Aviso     int             `json:"aviso"`
	Proximos  []agenda.Evento `json:"proximos"`
	Leida     *time.Time      `json:"leida,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// normalizarICS: webcal:// → https://; solo https (http en desarrollo), sin
// IPs internas ni el propio servicio.
func (i *Integraciones) normalizarICS(s string) (string, error) {
	s = strings.TrimSpace(s)
	if l := strings.ToLower(s); strings.HasPrefix(l, "webcal://") {
		s = "https://" + s[len("webcal://"):]
	}
	if len(s) > 2000 {
		return "", ErrAgendaInvalida{"la dirección es muy larga"}
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(i.op.Desarrollo && u.Scheme == "http")) {
		return "", ErrAgendaInvalida{"la dirección tiene que ser https:// (o webcal://)"}
	}
	if u.User != nil {
		return "", ErrAgendaInvalida{"la dirección no puede llevar usuario ni clave"}
	}
	if err := i.webhook.ValidarURL(s); err != nil {
		if errors.Is(err, ErrDestinoInterno) {
			return "", ErrAgendaInvalida{"esa dirección es interna: tiene que ser un calendario de internet"}
		}
		return "", ErrAgendaInvalida{err.Error()}
	}
	return s, nil
}

// pistaICS: "calendar.google.com/…/basic.ics" (el secreto de Google va en el medio).
func pistaICS(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return "…"
	}
	partes := strings.Split(strings.Trim(u.Path, "/"), "/")
	ultimo := partes[len(partes)-1]
	if len(ultimo) > 24 {
		ultimo = "…" + ultimo[len(ultimo)-12:]
	}
	if len(partes) <= 1 && len(u.Path) < 30 {
		return u.Hostname() + "/" + ultimo
	}
	return u.Hostname() + "/…/" + ultimo
}

// leerICS baja y lee el calendario.
func (i *Integraciones) leerICS(ctx context.Context, dir string) (*agenda.Calendario, error) {
	b, _, err := i.webhook.bajar(ctx, dir, MaxICS, "text/calendar, text/plain;q=0.5, */*;q=0.1", "Sintetica/1.0 (+agenda)")
	if err != nil {
		return nil, ErrAgendaInvalida{"no pude bajar el calendario: " + err.Error()}
	}
	c, err := agenda.Leer(b, nube.Zona())
	if err != nil {
		return nil, ErrAgendaInvalida{err.Error()}
	}
	return c, nil
}

func proximosDe(c *agenda.Calendario, ahora time.Time) []agenda.Evento {
	l := c.Entre(ahora.Add(-time.Hour), ahora.Add(VentanaAgenda))
	if len(l) > MaxProximos {
		l = l[:MaxProximos]
	}
	if l == nil {
		l = []agenda.Evento{}
	}
	return l
}

// ConectarAgenda guarda (o cambia) el calendario: lo baja ya para ver que sirve.
// aviso nil = el que tenía (o 10).
func (i *Integraciones) ConectarAgenda(ctx context.Context, usuario, ics string, aviso *int) (VistaAgenda, error) {
	dir, err := i.normalizarICS(ics)
	if err != nil {
		return VistaAgenda{}, err
	}
	av := AvisoPorDefecto
	if ag, err := i.alm.AgendaDe(ctx, usuario); err == nil {
		av = ag.Aviso
	}
	if aviso != nil {
		if *aviso < 0 || *aviso > MaxAviso {
			return VistaAgenda{}, ErrAgendaInvalida{fmt.Sprintf("el aviso es de 0 a %d minutos antes", MaxAviso)}
		}
		av = *aviso
	}
	c, err := i.leerICS(ctx, dir)
	if err != nil {
		return VistaAgenda{}, err
	}
	b, _ := json.Marshal(proximosDe(c, i.ahora()))
	if err := i.alm.GuardarAgenda(ctx, usuario, i.v19.cifraAgenda.Cifrar(dir), pistaICS(dir), av, b); err != nil {
		return VistaAgenda{}, err
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "agenda", Datos: map[string]any{"conectada": true}})
	return i.VerAgenda(ctx, usuario)
}

// CambiarAvisoAgenda: solo cuántos minutos antes.
func (i *Integraciones) CambiarAvisoAgenda(ctx context.Context, usuario string, aviso int) (VistaAgenda, error) {
	if aviso < 0 || aviso > MaxAviso {
		return VistaAgenda{}, ErrAgendaInvalida{fmt.Sprintf("el aviso es de 0 a %d minutos antes", MaxAviso)}
	}
	if err := i.alm.CambiarAvisoAgenda(ctx, usuario, aviso); errors.Is(err, almacen.ErrNoEncontrado) {
		return VistaAgenda{}, ErrSinAgenda
	} else if err != nil {
		return VistaAgenda{}, err
	}
	return i.VerAgenda(ctx, usuario)
}

// BorrarAgenda la olvida.
func (i *Integraciones) BorrarAgenda(ctx context.Context, usuario string) error {
	if err := i.alm.BorrarAgenda(ctx, usuario); err != nil {
		return err
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "agenda", Datos: map[string]any{"conectada": false}})
	return nil
}

func leerProximos(ag almacen.Agenda) []agenda.Evento {
	var l []agenda.Evento
	json.Unmarshal(ag.Proximos, &l) //nolint:errcheck
	return l
}

// EventosAgenda: los que no terminaron todavía y empiezan antes de `hasta`.
func (i *Integraciones) EventosAgenda(ctx context.Context, usuario string, hasta time.Time) ([]agenda.Evento, error) {
	ag, err := i.alm.AgendaDe(ctx, usuario)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return nil, ErrSinAgenda
	}
	if err != nil {
		return nil, err
	}
	ahora := i.ahora()
	l := []agenda.Evento{}
	for _, e := range leerProximos(ag) {
		if e.Fin.After(ahora) || (e.Fin.Equal(e.Inicio) && !e.Inicio.Before(ahora)) {
			if e.Inicio.Before(hasta) {
				l = append(l, e)
			}
		}
	}
	return l, nil
}

// VerAgenda: GET /v1/agenda (los próximos 10).
func (i *Integraciones) VerAgenda(ctx context.Context, usuario string) (VistaAgenda, error) {
	ag, err := i.alm.AgendaDe(ctx, usuario)
	if errors.Is(err, almacen.ErrNoEncontrado) {
		return VistaAgenda{Conectada: false, Aviso: AvisoPorDefecto, Proximos: []agenda.Evento{}}, nil
	}
	if err != nil {
		return VistaAgenda{}, err
	}
	l, err := i.EventosAgenda(ctx, usuario, i.ahora().Add(VentanaAgenda))
	if err != nil {
		return VistaAgenda{}, err
	}
	if len(l) > ProximosVisibles {
		l = l[:ProximosVisibles]
	}
	return VistaAgenda{Conectada: true, ICS: ag.Pista, Aviso: ag.Aviso, Proximos: l, Leida: ag.Leida, Error: ag.Error}, nil
}

// --- Cada minuto ---

func (i *Integraciones) revisarAgendas(ctx context.Context, m time.Time) {
	ags, err := i.alm.Agendas(ctx)
	if err != nil {
		i.log.Warn("agenda: no pude leer las agendas", "error", err)
		return
	}
	for _, ag := range ags {
		if ag.Leida == nil || i.ahora().Sub(*ag.Leida) >= CadaAgenda-30*time.Second {
			if l, ok := i.releerAgenda(ctx, ag); ok {
				b, _ := json.Marshal(l)
				ag.Proximos = b
			}
		}
		i.avisarAgenda(ctx, ag, m)
		i.resumenManana(ctx, ag, m)
	}
}

// releerAgenda baja el .ics de nuevo (si falla, quedan los de antes y el error).
func (i *Integraciones) releerAgenda(ctx context.Context, ag almacen.Agenda) ([]agenda.Evento, bool) {
	dir, err := i.v19.cifraAgenda.Descifrar(ag.ICSCifrado)
	if err == nil {
		var c *agenda.Calendario
		ctx, cancelar := context.WithTimeout(ctx, 30*time.Second)
		c, err = i.leerICS(ctx, dir)
		cancelar()
		if err == nil {
			l := proximosDe(c, i.ahora())
			b, _ := json.Marshal(l)
			if err := i.alm.AgendaLeida(ctx, ag.UsuarioID, b, ""); err != nil {
				i.log.Warn("agenda: no pude guardar", "error", err)
			}
			return l, true
		}
	}
	if err.Error() != ag.Error {
		i.alm.AgendaLeida(ctx, ag.UsuarioID, nil, err.Error()) //nolint:errcheck
		i.log.Info("agenda: no la pude leer", "usuario", ag.UsuarioID, "error", err)
	}
	return nil, false
}

// minutosQueFaltan: redondeado para arriba (a las 9:50:10 para las 10:00 faltan 10).
func minutosQueFaltan(ahora, inicio time.Time) int {
	d := inicio.Sub(ahora)
	if d <= 0 {
		return 0
	}
	return int((d + time.Minute - 1) / time.Minute)
}

// avisarAgenda: lo que empieza dentro de `aviso` minutos (una vez por evento).
func (i *Integraciones) avisarAgenda(ctx context.Context, ag almacen.Agenda, m time.Time) {
	ahora := i.ahora()
	for _, e := range leerProximos(ag) {
		if e.TodoElDia || e.Inicio.Before(ahora.Add(-time.Minute)) {
			continue
		}
		faltan := minutosQueFaltan(ahora, e.Inicio)
		if faltan > ag.Aviso {
			continue
		}
		clave := fmt.Sprintf("%s@%d", e.UID, e.Inicio.Unix())
		if ok, err := i.alm.MarcarAvisado(ctx, ag.UsuarioID, clave); err != nil || !ok {
			continue
		}
		i.mandarAgenda(ctx, ag.UsuarioID, e, faltan)
	}
}

func (i *Integraciones) mandarAgenda(ctx context.Context, usuario string, e agenda.Evento, faltan int) {
	marco, err := protocolo.Agenda(faltan, e.Minutos(), e.Titulo)
	if err != nil {
		return
	}
	rs, err := i.robotsDe(ctx, usuario)
	if err != nil {
		return
	}
	var a []string
	for _, r := range rs {
		if r.conectado {
			if err := i.robots.Enviar(r.ID, protocolo.CarComando, marco); err == nil {
				a = append(a, r.ID)
			}
		}
		datos := map[string]any{"texto": e.Titulo, "titulo": e.Titulo, "minutos": faltan}
		if i.tieneAutomatizacionPara(r.ID, "agenda") {
			select {
			case i.cola <- disparo{bicho: r.ID, evento: "agenda", datos: datos}:
			default:
			}
		}
	}
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "agenda", Datos: map[string]any{
		"titulo": e.Titulo, "minutos": faltan, "inicio": e.Inicio, "robots": a}})
}

// horaDelResumenManana: la de dormir del primer robot (horario.duerme, si es
// de 18:00 a 23:59), o las 22:00.
func (i *Integraciones) horaDelResumenManana(ctx context.Context, rs []robot) int {
	for _, r := range rs {
		cfg, err := i.alm.UltimaConfig(ctx, r.ID)
		if err != nil {
			continue
		}
		var c struct {
			Horario struct {
				Duerme string `json:"duerme"`
			} `json:"horario"`
		}
		json.Unmarshal([]byte(cfg.Texto), &c) //nolint:errcheck
		if h := minutosDe(c.Horario.Duerme, -1); h >= 18*60 {
			return h
		}
		break
	}
	return 22 * 60
}

// eventosDeManana: los de mañana (en la zona de los robots), los de día entero primero.
func eventosDeManana(l []agenda.Evento, m time.Time) []agenda.Evento {
	inicio := inicioDelDia(m).AddDate(0, 0, 1)
	fin := inicio.AddDate(0, 0, 1)
	var out []agenda.Evento
	for _, e := range l {
		if !e.Inicio.Before(inicio) && e.Inicio.Before(fin) {
			out = append(out, e)
		}
	}
	return out
}

// textoAgenda: la lista para Telegram ("10:00 Dentista").
func textoAgenda(l []agenda.Evento) string {
	var b strings.Builder
	for _, e := range l {
		if e.TodoElDia {
			b.WriteString("• Todo el día: " + e.Titulo + "\n")
		} else {
			b.WriteString("• " + e.Inicio.In(nube.Zona()).Format("15:04") + " " + e.Titulo + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func (i *Integraciones) resumenManana(ctx context.Context, ag almacen.Agenda, m time.Time) {
	rs, err := i.robotsDe(ctx, ag.UsuarioID)
	if err != nil || len(rs) == 0 {
		return
	}
	hora := i.horaDelResumenManana(ctx, rs)
	if minutos(m) < hora || minutos(m) > hora+30 {
		return
	}
	l := eventosDeManana(leerProximos(ag), m)
	if len(l) == 0 {
		return
	}
	if ok, err := i.alm.MarcarResumen(ctx, ag.UsuarioID, m); err != nil || !ok {
		return
	}
	if marco, err := protocolo.ResumenManana(len(l), l[0].Titulo); err == nil {
		for _, r := range rs {
			if r.conectado {
				i.robots.Enviar(r.ID, protocolo.CarComando, marco) //nolint:errcheck
			}
		}
	}
	if _, err := i.alm.TelegramDe(ctx, ag.UsuarioID); err == nil && i.tg != nil {
		texto := "Mañana tenés " + cantidadDeEventos(len(l)) + ":\n" + textoAgenda(l)
		if err := i.MandarTelegram(ctx, ag.UsuarioID, texto); err != nil {
			i.log.Info("agenda: no pude mandar el resumen por Telegram", "error", err)
		}
	}
	i.hub.Publicar(ag.UsuarioID, eventos.Evento{Tipo: "agenda", Datos: map[string]any{"manana": len(l), "titulo": l[0].Titulo}})
}

func cantidadDeEventos(n int) string {
	if n == 1 {
		return "1 cosa"
	}
	return fmt.Sprintf("%d cosas", n)
}

// TextoAgendaTelegram: lo que contesta /agenda (hoy y mañana).
func (i *Integraciones) TextoAgendaTelegram(ctx context.Context, usuario string) string {
	m := i.ahora().In(nube.Zona())
	l, err := i.EventosAgenda(ctx, usuario, inicioDelDia(m).AddDate(0, 0, 2))
	if errors.Is(err, ErrSinAgenda) {
		return "No conectaste un calendario. En la web, en Agenda, pegá la dirección secreta en formato iCal de tu Google Calendar (o cualquier .ics)."
	}
	if err != nil {
		return "No pude leer tu agenda ahora: probá en un rato."
	}
	var hoy, manana []agenda.Evento
	finHoy := inicioDelDia(m).AddDate(0, 0, 1)
	for _, e := range l {
		if e.Inicio.Before(finHoy) {
			hoy = append(hoy, e)
		} else {
			manana = append(manana, e)
		}
	}
	var b strings.Builder
	if len(hoy) == 0 {
		b.WriteString("Hoy no tenés nada más.")
	} else {
		b.WriteString("Hoy:\n" + textoAgenda(hoy))
	}
	if len(manana) == 0 {
		b.WriteString("\n\nMañana, nada por ahora.")
	} else {
		b.WriteString("\n\nMañana:\n" + textoAgenda(manana))
	}
	return b.String()
}
