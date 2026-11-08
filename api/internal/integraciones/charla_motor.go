package integraciones

// La charla sin IA en la nube (v1.10): toda charla (la consola, Telegram, el
// micrófono del robot-celu) pasa primero por el MISMO motor que corre en el
// robot (nucleo.Charlar), con lo que la nube sabe de él (su dueño, su nombre,
// su personalidad, la hora, el clima, la agenda, lo que suena, los hábitos y
// los gustos) y con el hilo de cada robot en la memoria ("¿a qué hora?" →
// "a las 9"). Si entendió:
//
//   - con el robot conectado, se lo manda con 0x4C para que lo charle y lo
//     haga él (su globo, su cara, su voz); lo que hace queda en su RAM y la
//     nube lo guarda cuando llega su 0xCC (charlaDelRobot);
//   - sin el robot, la nube hace la acción en la config (por el mismo camino
//     validado que PUT config, autor "charla") y le dice la frase con 0x40
//     cuando vuelve (si hizo algo).
//
// No gasta el tope de la IA. Solo si no entendió y hay IA, va a la IA. Ver
// docs/nube.md, "Charla".

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"sync"
	"time"

	nucleo "sintetica/firmware/api/cgo"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/nube"
	"sintetica/api/internal/protocolo"
)

const (
	// AutorCharla: el de las versiones de la config que deja la charla.
	AutorCharla = "charla"
	// Lo que se guarda el hilo de un robot sin charlar (el motor, además,
	// lo olvida a los 90 s de la última frase).
	venceHiloCharla = 10 * time.Minute
	// Lo que dura la frase que espera a que el robot se conecte.
	venceDecirPendiente = 30 * time.Minute
	// Lo que queda en la config una alarma o un recordatorio de una vez
	// después de su hora (la alarma suena y se pospone un rato).
	quedaAlarmaUnaVez       = 45 * time.Minute
	quedaRecordatorioUnaVez = 5 * time.Minute
)

// Los modos que dice el motor (el dato de la acción "modo").
var modosCharla = []string{"mascota", "reloj", "pomodoro", "clima"}

type hiloCharla struct {
	estado nucleo.EstadoCharla
	en     time.Time
}

type decirPendiente struct {
	marco []byte
	en    time.Time
}

type estadoCharlaMotor struct {
	mu      sync.Mutex
	hilos   map[string]hiloCharla                     // robot → lo que recuerda entre frases
	esperas map[string]chan protocolo.RespuestaCharla // robot → quien espera su 0xCC
	decir   map[string]decirPendiente                 // robot → la frase para cuando vuelva
	espera  time.Duration                             // lo que se espera el 0xCC
}

func nuevoEstadoCharlaMotor() estadoCharlaMotor {
	return estadoCharlaMotor{hilos: map[string]hiloCharla{}, esperas: map[string]chan protocolo.RespuestaCharla{},
		decir: map[string]decirPendiente{}, espera: 4 * time.Second}
}

// --- Lo que sabe la nube ---

// contextoCharla arma lo que sabe la nube del robot y de su dueño.
func (i *Integraciones) contextoCharla(ctx context.Context, usuario, bicho string) nucleo.ContextoCharla {
	c := nucleo.NuevoContextoCharla()
	m := i.ahora().In(nube.Zona())
	c.Robot = i.nombreDe(ctx, bicho)
	c.Minuto = m.Hour()*60 + m.Minute()
	c.Epoch = uint32(protocolo.EpochLocal(m))
	c.Segundos = uint32(m.Unix())
	c.Semilla = rand.Uint32()
	if cfg, err := i.alm.UltimaConfig(ctx, bicho); err == nil {
		var x struct {
			Personalidad struct {
				Preset string `json:"preset"`
			} `json:"personalidad"`
		}
		json.Unmarshal([]byte(cfg.Texto), &x) //nolint:errcheck
		c.Preset = x.Personalidad.Preset
		dc := leerDatosConfig(cfg.Texto)
		c.Dueno = strings.TrimSpace(dc.Dueno.Nombre)
		for _, h := range habitosDe(cfg.Texto) {
			c.Habitos = append(c.Habitos, h.Nombre)
		}
		if dc.Ubicacion != nil {
			cctx, cancelar := context.WithTimeout(ctx, 2*time.Second)
			if _, cl := i.ClimaDe(cctx, bicho); cl != nil {
				c.Clima = cl.Texto()
			}
			cancelar()
		}
	}
	if e, ok := i.robots.EstadoDe(bicho); ok {
		c.AlarmaSonando = strings.HasPrefix(e.Expresion, "despertador")
	}
	if g, _ := i.memoriaDe(ctx, bicho, m); g != nil && g.Dias > 0 {
		c.DiasJuntos = int(g.Dias)
	}
	if gs, err := i.alm.Gustos(ctx, bicho); err == nil {
		c.Gustos = gs[:min(len(gs), 4)]
	}
	if usuario != "" {
		hoy := inicioDelDia(m)
		if evs, err := i.EventosAgenda(ctx, usuario, hoy.AddDate(0, 0, 2)); err == nil {
			for _, e := range evs {
				ini := e.Inicio.In(nube.Zona())
				switch {
				case c.Agenda == "" && ini.Before(hoy.AddDate(0, 0, 1)):
					c.Agenda = textoEvento(e.Titulo, ini, e.TodoElDia)
				case c.AgendaManana == "" && !ini.Before(hoy.AddDate(0, 0, 1)):
					c.AgendaManana = textoEvento(e.Titulo, ini, e.TodoElDia)
				}
			}
		}
		i.v19.musicaMu.Lock()
		s, ok := i.v19.musica[usuario]
		i.v19.musicaMu.Unlock()
		if ok && s.Sonando && s.Titulo != "" {
			c.Musica = s.Titulo
			if s.Artista != "" {
				c.Musica += ", de " + s.Artista
			}
		}
	}
	return c
}

// textoEvento: "Dentista a las 15" (o "a las 15:30"; "Feriado" si es todo el día).
func textoEvento(titulo string, ini time.Time, todoElDia bool) string {
	titulo = protocolo.CortarUTF8(strings.TrimSpace(titulo), 30)
	if todoElDia {
		return titulo
	}
	if ini.Minute() == 0 {
		return fmt.Sprintf("%s a las %d", titulo, ini.Hour())
	}
	return fmt.Sprintf("%s a las %d:%02d", titulo, ini.Hour(), ini.Minute())
}

// entender: lo que dice el motor, con el hilo de ese robot.
func (i *Integraciones) entender(ctx context.Context, usuario, bicho, texto string) nucleo.RespuestaCharla {
	c := i.contextoCharla(ctx, usuario, bicho)
	ahora := i.ahora()
	i.cm.mu.Lock()
	h, ok := i.cm.hilos[bicho]
	if !ok || ahora.Sub(h.en) > venceHiloCharla {
		h = hiloCharla{}
	}
	for k, v := range i.cm.hilos { // los viejos se olvidan
		if ahora.Sub(v.en) > venceHiloCharla {
			delete(i.cm.hilos, k)
		}
	}
	i.cm.mu.Unlock()
	r, err := nucleo.Charlar(protocolo.CortarUTF8(texto, protocolo.MaxTextoCharlar), c, &h.estado)
	if err != nil {
		return nucleo.RespuestaCharla{NoEntendi: true, Accion: "ninguna", Frase: "Mmm, eso no lo entendí.", Dato: -1, Minuto: -1, EnMinutos: -1, Dia: -1}
	}
	h.en = ahora
	i.cm.mu.Lock()
	i.cm.hilos[bicho] = h
	i.cm.mu.Unlock()
	return r
}

// --- Contestar con el motor ---

// charlaDelMotor: lo entendió el motor (o no hay IA y contesta él igual).
func (i *Integraciones) charlaDelMotor(ctx context.Context, usuario, bicho, texto, desde string, r nucleo.RespuestaCharla, m motorIA, conIA bool) RespuestaCharla {
	res := RespuestaCharla{Texto: r.Frase, Expresion: r.Expresion, Quedan: -1, Origen: OrigenMotor, Intencion: r.Intencion}
	if conIA {
		res.Propia = m.propia
		res.Quedan = i.charlasQuedan(ctx, usuario, m.tope)
	}
	if res.Texto == "" {
		res.Texto = "…"
	}
	a := accionDelMotor(r)
	if a.hace() {
		res.Accion = a.Nombre
	}
	hecho := false
	if i.robots.Conectado(bicho) {
		cc, llego, mandado := i.charlarEnElRobot(ctx, bicho, texto)
		switch {
		case llego && cc.Codigo == protocolo.Ok:
			// Lo charló él: lo que hizo lo guarda charlaDelRobot (su 0xCC).
			res.Enviado, hecho = true, true
			if cc.Frase != "" {
				res.Texto = cc.Frase
			}
		case mandado && !llego:
			// No contestó a tiempo: si lo hizo, su 0xCC llega igual (y se
			// guarda); hacerlo acá también lo haría dos veces.
			res.Enviado, hecho = true, true
		}
		// Si no se pudo mandar o lo rechazó (un firmware sin charla), lo hace
		// la nube y se lo dice con 0x40.
	}
	if !hecho {
		res.Guardado = i.hacerEnLaNube(ctx, usuario, bicho, a, false) && a.hace()
		if md, err := protocolo.Decir(protocolo.CortarUTF8(res.Texto, protocolo.MaxDecir), res.Expresion); err == nil {
			if i.robots.Conectado(bicho) {
				res.Enviado = i.robots.Enviar(bicho, protocolo.CarComando, md) == nil
			} else if a.hace() {
				i.cm.mu.Lock()
				i.cm.decir[bicho] = decirPendiente{marco: md, en: i.ahora()}
				i.cm.mu.Unlock()
			}
		}
	}
	i.alm.AnotarRobotCharla(ctx, usuario, bicho) //nolint:errcheck
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "charla", Bicho: bicho, Datos: map[string]any{
		"texto": res.Texto, "expresion": res.Expresion, "de": desde, "enviado": res.Enviado, "origen": OrigenMotor,
		"intencion": res.Intencion, "accion": res.Accion}})
	return res
}

// charlarEnElRobot le manda el 0x4C y espera su 0xCC. llego: contestó;
// mandado: le llegó el marco.
func (i *Integraciones) charlarEnElRobot(ctx context.Context, bicho, texto string) (cc protocolo.RespuestaCharla, llego, mandado bool) {
	marco, err := protocolo.Charlar(protocolo.CortarUTF8(texto, protocolo.MaxTextoCharlar))
	if err != nil {
		return cc, false, false
	}
	ch := make(chan protocolo.RespuestaCharla, 1)
	i.cm.mu.Lock()
	i.cm.esperas[bicho] = ch
	espera := i.cm.espera
	i.cm.mu.Unlock()
	defer func() {
		i.cm.mu.Lock()
		if i.cm.esperas[bicho] == ch {
			delete(i.cm.esperas, bicho)
		}
		i.cm.mu.Unlock()
	}()
	if i.robots.Enviar(bicho, protocolo.CarComando, marco) != nil {
		return cc, false, false
	}
	t := time.NewTimer(espera)
	defer t.Stop()
	select {
	case cc = <-ch:
		return cc, true, true
	case <-t.C:
	case <-ctx.Done():
	}
	return cc, false, true
}

// charlaDelRobot: llegó un 0xCC (la respuesta a un 0x4C o lo que charló él
// desde adentro): se lo pasa a quien espera y guarda lo que haga falta.
func (i *Integraciones) charlaDelRobot(bicho string, datos map[string]any) {
	marco, _ := datos["marco"].([]byte)
	cc, err := protocolo.LeerCharla(marco)
	if err != nil {
		i.log.Info("charla: 0xCC mal armado", "bicho", bicho, "error", err)
		return
	}
	i.cm.mu.Lock()
	ch := i.cm.esperas[bicho]
	i.cm.mu.Unlock()
	if ch != nil {
		select {
		case ch <- cc:
		default:
		}
	}
	if cc.Codigo != protocolo.Ok || cc.NoEntendi || cc.Falta || !cc.HizoAlgo {
		return
	}
	usuario := i.robots.Dueno(bicho)
	if usuario == "" {
		return
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelar()
	i.hacerEnLaNube(ctx, usuario, bicho, accionCharla{Nombre: cc.Accion, Dato: cc.Dato, Minuto: cc.Minuto,
		EnMinutos: cc.EnMinutos, Dia: cc.Dia, Dias: cc.Dias, Texto: cc.Texto}, true)
}

// decirPendiente: al conectarse, la frase de lo que se hizo sin él.
func (i *Integraciones) decirPendiente(bicho string) {
	i.cm.mu.Lock()
	p, ok := i.cm.decir[bicho]
	delete(i.cm.decir, bicho)
	for k, v := range i.cm.decir {
		if i.ahora().Sub(v.en) > venceDecirPendiente {
			delete(i.cm.decir, k)
		}
	}
	i.cm.mu.Unlock()
	if ok && i.ahora().Sub(p.en) <= venceDecirPendiente {
		i.robots.Enviar(bicho, protocolo.CarComando, p.marco) //nolint:errcheck
	}
}

// --- Las acciones ---

// accionCharla: lo que hay que hacer (del motor de la nube o del 0xCC).
type accionCharla struct {
	Nombre                       string
	Dato, Minuto, EnMinutos, Dia int
	Dias                         int // bit 0 lunes ... bit 6 domingo (0 = una vez)
	Texto                        string
}

func (a accionCharla) hace() bool { return a.Nombre != "" && a.Nombre != "ninguna" }

func accionDelMotor(r nucleo.RespuestaCharla) accionCharla {
	if r.NoEntendi || r.Falta {
		return accionCharla{Nombre: "ninguna"}
	}
	return accionCharla{Nombre: r.Accion, Dato: r.Dato, Minuto: r.Minuto, EnMinutos: r.EnMinutos, Dia: r.Dia, Dias: r.Dias, Texto: r.Texto}
}

// letrasDias: los bits de días como en la config ("LMXJV"; todos: "").
func letrasDias(dias int) string {
	if dias&0x7F == 0x7F {
		return ""
	}
	var b strings.Builder
	for k, l := range "LMXJVSD" {
		if dias&(1<<k) != 0 {
			b.WriteRune(l)
		}
	}
	return b.String()
}

func horaCharla(minuto int) string { return fmt.Sprintf("%02d:%02d", minuto/60%24, minuto%60) }

// cuandoCharla: la próxima vez de una hora dicha una vez (como Sistema::hacerAccion).
func (i *Integraciones) cuandoCharla(a accionCharla) (time.Time, bool) {
	ahora := i.ahora().In(nube.Zona())
	if a.Minuto < 0 {
		if a.EnMinutos < 0 {
			return time.Time{}, false
		}
		// Redondeado al minuto que viene (la config va de a minutos).
		return ahora.Add(time.Duration(a.EnMinutos)*time.Minute + time.Minute - 1).Truncate(time.Minute), true
	}
	t := inicioDelDia(ahora).AddDate(0, 0, max(a.Dia, 0)).Add(time.Duration(a.Minuto) * time.Minute)
	if !t.After(ahora) {
		t = t.AddDate(0, 0, 1)
	}
	return t, true
}

// hacerEnLaNube hace lo que se guarda en la nube. delRobot: lo hizo el robot
// (lo que vive en su RAM, como una alarma de una vez, ya está). Devuelve si
// hizo algo.
func (i *Integraciones) hacerEnLaNube(ctx context.Context, usuario, bicho string, a accionCharla, delRobot bool) bool {
	if !a.hace() {
		return false
	}
	var err error
	hizo := true
	switch a.Nombre {
	case "alarma":
		switch {
		case a.Dias != 0 && a.Minuto >= 0:
			err = i.ponerDespertador(ctx, usuario, bicho, map[string]any{"hora": horaCharla(a.Minuto), "dias": letrasDias(a.Dias), "activo": true}, false, time.Time{})
		case delRobot:
			hizo = false
		default:
			t, ok := i.cuandoCharla(a)
			if !ok {
				return false
			}
			err = i.ponerDespertador(ctx, usuario, bicho, map[string]any{"hora": t.Format("15:04"), "dias": string(letraDia[t.Weekday()]), "activo": true}, true, t)
		}
	case "cancelar-alarma":
		err = i.sacarAlarmasUnaVez(ctx, usuario, bicho)
	case "recordatorio":
		texto := strings.TrimSpace(a.Texto)
		if texto == "" {
			texto = "¡Tiempo!"
		}
		texto = protocolo.CortarUTF8(texto, 40)
		switch {
		case a.Dias != 0 && a.Minuto >= 0:
			rec := map[string]any{"hora": horaCharla(a.Minuto), "texto": texto}
			if d := letrasDias(a.Dias); d != "" {
				rec["dias"] = d
			}
			err = i.sumarRecordatorio(ctx, usuario, bicho, rec, time.Time{})
		case delRobot:
			hizo = false
		default:
			t, ok := i.cuandoCharla(a)
			if !ok {
				return false
			}
			err = i.sumarRecordatorio(ctx, usuario, bicho, map[string]any{"hora": t.Format("15:04"), "dias": string(letraDia[t.Weekday()]), "texto": texto}, t)
		}
	case "habito":
		if delRobot || a.Dato < 0 {
			return false // el robot lo avisa con su 0x9E
		}
		_, _, err = i.MarcarHabito(ctx, bicho, a.Dato, true, false)
	case "gusto":
		err = i.alm.SumarGusto(ctx, bicho, protocolo.CortarUTF8(a.Texto, 24))
	case "olvidar-gusto":
		err = i.alm.SacarGusto(ctx, bicho, a.Texto)
	case "toque":
		err = i.toqueDeCharla(ctx, usuario, bicho, a)
	case "dueno":
		nombre := protocolo.CortarUTF8(strings.TrimSpace(a.Texto), 24)
		if nombre == "" {
			return false
		}
		err = i.cambiarObjeto(ctx, usuario, bicho, "dueno", func(o map[string]any) { o["nombre"] = nombre })
	case "volumen":
		err = i.cambiarObjeto(ctx, usuario, bicho, "voz", func(o map[string]any) {
			v, ok := o["volumen"].(float64)
			if !ok {
				v = 0.7
			}
			if a.Dato > 0 {
				v += 0.2
			} else {
				v -= 0.2
			}
			o["volumen"] = float64(int(min(1, max(0.1, v))*100+0.5)) / 100
		})
	case "modo":
		if a.Dato < 0 || a.Dato >= len(modosCharla) {
			return false
		}
		err = i.cambiar(ctx, usuario, bicho, map[string]any{"modo": modosCharla[a.Dato]})
	default:
		// Jugar, dormir, despertar, callar, ánimo, posponer y apagar: solo
		// en el robot. Música: todavía no.
		return false
	}
	if err != nil {
		i.log.Warn("charla: no pude guardar lo que dijo", "bicho", bicho, "accion", a.Nombre, "delRobot", delRobot, "error", err)
		return false
	}
	return hizo
}

// cambiar: claves de arriba de todo de la config (nil = sacarla).
func (i *Integraciones) cambiar(ctx context.Context, usuario, bicho string, claves map[string]any) error {
	if i.cambiarConfig == nil {
		return fmt.Errorf("no hay cómo guardar la config")
	}
	return i.cambiarConfig(ctx, bicho, usuario, AutorCharla, claves)
}

// configCruda: la última config como mapa de claves (vacía si no hay).
func (i *Integraciones) configCruda(ctx context.Context, bicho string) (map[string]json.RawMessage, error) {
	c, err := i.alm.UltimaConfig(ctx, bicho)
	if err == almacen.ErrNoEncontrado {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(c.Texto), &m) != nil || m == nil {
		m = map[string]json.RawMessage{}
	}
	return m, nil
}

// cambiarObjeto cambia campos de un objeto de la config (dueno, voz...).
func (i *Integraciones) cambiarObjeto(ctx context.Context, usuario, bicho, clave string, f func(o map[string]any)) error {
	m, err := i.configCruda(ctx, bicho)
	if err != nil {
		return err
	}
	o := map[string]any{}
	json.Unmarshal(m[clave], &o) //nolint:errcheck
	if o == nil {
		o = map[string]any{}
	}
	f(o)
	return i.cambiar(ctx, usuario, bicho, map[string]any{clave: o})
}

// ponerDespertador pone el despertador (con lo demás que tenía: texto,
// sonido, posponer). unaVez: hasta `t` (después vuelve el que había).
func (i *Integraciones) ponerDespertador(ctx context.Context, usuario, bicho string, nuevo map[string]any, unaVez bool, t time.Time) error {
	m, err := i.configCruda(ctx, bicho)
	if err != nil {
		return err
	}
	antes := m["despertador"]
	// Si había otra de una vez, se olvida (y si esta es de una vez, el de
	// antes es el de esa).
	if l, err := i.alm.UnasVez(ctx, bicho, "alarma", time.Now().AddDate(10, 0, 0)); err == nil {
		for k, u := range l {
			if k == 0 && unaVez {
				antes = u.Antes
			}
			i.alm.BorrarUnaVez(ctx, u.ID) //nolint:errcheck
		}
	}
	d := map[string]any{}
	json.Unmarshal(antes, &d) //nolint:errcheck
	if d == nil {
		d = map[string]any{}
	}
	for k, v := range nuevo {
		d[k] = v
	}
	if d["dias"] == "" {
		delete(d, "dias")
	}
	if err := i.cambiar(ctx, usuario, bicho, map[string]any{"despertador": d}); err != nil {
		return err
	}
	if !unaVez {
		return nil
	}
	puesto, _ := json.Marshal(d)
	return i.alm.AnotarUnaVez(ctx, almacen.UnaVez{Bicho: bicho, Tipo: "alarma", Vence: t.Add(quedaAlarmaUnaVez), Puesto: puesto, Antes: antes})
}

// sumarRecordatorio lo suma a "recordatorios" (si entra). unaVez (t no
// cero): se saca después de su hora.
func (i *Integraciones) sumarRecordatorio(ctx context.Context, usuario, bicho string, rec map[string]any, t time.Time) error {
	m, err := i.configCruda(ctx, bicho)
	if err != nil {
		return err
	}
	var l []any
	json.Unmarshal(m["recordatorios"], &l) //nolint:errcheck
	if len(l) >= 12 {
		return fmt.Errorf("ya tiene 12 recordatorios")
	}
	l = append(l, rec)
	if err := i.cambiar(ctx, usuario, bicho, map[string]any{"recordatorios": l}); err != nil {
		return err
	}
	if t.IsZero() {
		return nil
	}
	puesto, _ := json.Marshal(rec)
	return i.alm.AnotarUnaVez(ctx, almacen.UnaVez{Bicho: bicho, Tipo: "recordatorio", Vence: t.Add(quedaRecordatorioUnaVez), Puesto: puesto})
}

// iguales: dos valores JSON iguales (sin importar el orden de las claves).
func iguales(a json.RawMessage, b any) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil {
		return false
	}
	bb, _ := json.Marshal(b)
	json.Unmarshal(bb, &y) //nolint:errcheck
	return reflect.DeepEqual(x, y)
}

// sacarUnaVez saca de la config lo que puso la charla para una vez (si
// sigue como lo dejó: si el dueño lo cambió, queda).
func (i *Integraciones) sacarUnaVez(ctx context.Context, usuario string, u almacen.UnaVez) error {
	m, err := i.configCruda(ctx, u.Bicho)
	if err != nil {
		return err
	}
	switch u.Tipo {
	case "alarma":
		if !iguales(u.Puesto, m["despertador"]) {
			return nil
		}
		var antes any
		if len(u.Antes) > 0 {
			antes = u.Antes
		}
		return i.cambiar(ctx, usuario, u.Bicho, map[string]any{"despertador": antes})
	case "recordatorio":
		var l []json.RawMessage
		json.Unmarshal(m["recordatorios"], &l) //nolint:errcheck
		for k, r := range l {
			if iguales(u.Puesto, r) {
				l = append(l[:k], l[k+1:]...)
				return i.cambiar(ctx, usuario, u.Bicho, map[string]any{"recordatorios": l})
			}
		}
	}
	return nil
}

// sacarAlarmasUnaVez: "cancelá la alarma" (vuelve el despertador de antes).
func (i *Integraciones) sacarAlarmasUnaVez(ctx context.Context, usuario, bicho string) error {
	l, err := i.alm.UnasVez(ctx, bicho, "alarma", time.Now().AddDate(10, 0, 0))
	if err != nil {
		return err
	}
	for k, u := range l {
		if k == 0 {
			if err := i.sacarUnaVez(ctx, usuario, u); err != nil {
				return err
			}
		}
		i.alm.BorrarUnaVez(ctx, u.ID) //nolint:errcheck
	}
	return nil
}

// limpiarUnaVez: cada minuto, saca lo de una vez que ya pasó.
func (i *Integraciones) limpiarUnaVez(ctx context.Context) {
	l, err := i.alm.UnasVez(ctx, "", "", i.ahora())
	if err != nil {
		i.log.Warn("charla: no pude leer lo de una vez", "error", err)
		return
	}
	for _, u := range l {
		usuario := i.robots.Dueno(u.Bicho)
		if usuario == "" {
			if b, err := i.alm.Bicho(ctx, u.Bicho); err == nil && b.UsuarioID != nil {
				usuario = *b.UsuarioID
			}
		}
		if usuario != "" {
			if err := i.sacarUnaVez(ctx, usuario, u); err != nil {
				i.log.Warn("charla: no pude sacar lo de una vez", "bicho", u.Bicho, "tipo", u.Tipo, "error", err)
				continue
			}
		}
		i.alm.BorrarUnaVez(ctx, u.ID) //nolint:errcheck
	}
}

// toqueDeCharla: "mandale un abrazo a Juan" (el amigo por su nombre).
func (i *Integraciones) toqueDeCharla(ctx context.Context, usuario, bicho string, a accionCharla) error {
	quien := normalizarNombre(a.Texto)
	if quien == "" {
		return nil
	}
	l, err := i.Amigos(ctx, usuario)
	if err != nil {
		return err
	}
	for _, am := range l {
		n := normalizarNombre(am.Nombre)
		if n == quien || strings.HasPrefix(n, quien+" ") {
			tipo := "toque"
			if a.Dato >= 0 && a.Dato < len(protocolo.TiposToque) {
				tipo = protocolo.TiposToque[a.Dato]
			}
			expresion := ""
			if tipo == "carita" {
				expresion = "contento"
			}
			_, err := i.Tocar(ctx, usuario, am.ID, tipo, expresion, i.nombreDe(ctx, bicho))
			return err
		}
	}
	return nil // no es un amigo: el robot ya contestó
}

func normalizarNombre(s string) string {
	r := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")
	return strings.Join(strings.Fields(r.Replace(strings.ToLower(s))), " ")
}
