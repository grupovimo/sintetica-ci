package integraciones

// Las listas por charla (docs/nube.md, "Listas"): "anotá yerba en la lista
// del súper", "sacá la yerba", "ya compré el pan", "¿qué hay en pendientes?".
// Un analizador chico y determinista que corre ANTES que el motor y la IA en
// toda charla (la consola, Telegram, el escritorio y el robot-celu por MQTT):
// el motor entiende "anotá…" como un recordatorio. Lo que no es de listas
// sigue de largo. Las listas son del usuario (almacen/listas.go).

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"sintetica/api/internal/almacen"
	"sintetica/api/internal/eventos"
	"sintetica/api/internal/protocolo"
)

// ListaPorDefecto: la que se usa cuando no se dice cuál.
const ListaPorDefecto = "súper"

type accionLista int

const (
	listaAgregar accionLista = iota + 1
	listaSacar
	listaTachar
	listaVer
	listaVaciar
	listaLimpiar
	listaCuales
)

var nombresAccionLista = map[accionLista]string{listaAgregar: "agregar", listaSacar: "sacar", listaTachar: "tachar",
	listaVer: "ver", listaVaciar: "vaciar", listaLimpiar: "limpiar", listaCuales: "listas"}

// pedidoLista: lo que se entendió.
type pedidoLista struct {
	accion accionLista
	lista  string // el nombre; "" = no dijo cuál
	items  []string
	// nombrada: dijo "de la lista" (aunque no cuál): si lo que hay que sacar
	// o tachar no está, se contesta igual (si no, sigue al motor).
	nombrada bool
}

// --- El analizador ---

type palabra struct {
	orig string // tal cual, sin los signos de los bordes
	n    string // minúsculas y sin tildes
	coma bool   // terminaba en coma
	dos  bool   // terminaba en dos puntos
}

func palabrasDe(texto string) []palabra {
	texto = separarComas(texto)
	var ps []palabra
	for _, f := range strings.Fields(texto) {
		fin := strings.TrimRight(f, "?!.¿¡…\"'»”)")
		coma := strings.HasSuffix(fin, ",") || strings.HasSuffix(fin, ";")
		dos := strings.HasSuffix(fin, ":")
		o := strings.Trim(f, "¿?¡!.,;:…\"'«»“”()")
		if o == "" {
			if len(ps) > 0 {
				ps[len(ps)-1].coma = ps[len(ps)-1].coma || coma
				ps[len(ps)-1].dos = ps[len(ps)-1].dos || dos
			}
			continue
		}
		ps = append(ps, palabra{orig: o, n: normal(o), coma: coma, dos: dos})
	}
	return ps
}

// separarComas: "leche,pan" → "leche, pan" (pero "1,5 kilos" queda).
func separarComas(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for k, r := range rs {
		b.WriteRune(r)
		if r == ',' && k+1 < len(rs) && !(k > 0 && unicode.IsDigit(rs[k-1]) && unicode.IsDigit(rs[k+1])) {
			b.WriteRune(' ')
		}
	}
	return b.String()
}

func conjunto(l ...string) map[string]bool {
	m := make(map[string]bool, len(l))
	for _, s := range l {
		m[s] = true
	}
	return m
}

var (
	verbosAgregar = conjunto("anota", "anotame", "anotar", "anotas", "anoto", "anotalo", "anotala", "anotalos", "anotalas",
		"agrega", "agregame", "agregar", "agregas", "agrego", "agregale", "agregalo", "agregala",
		"suma", "sumame", "sumar", "sumas", "sumale", "sumalo", "sumala",
		"añadi", "añade", "añadir", "añadime", "añadile", "pone", "poneme", "poner", "pones", "ponele",
		"mete", "meteme", "meter", "metes", "metele", "inclui", "incluye", "incluime", "incluir")
	verbosSacar = conjunto("saca", "sacame", "sacar", "sacas", "sacale", "sacalo", "sacala", "quita", "quitame", "quitar",
		"quitas", "quitale", "borra", "borrame", "borrar", "borras", "borrale", "borralo", "borrala",
		"elimina", "eliminame", "eliminar", "eliminas")
	verbosTachar = conjunto("tacha", "tachame", "tachar", "tachas", "tache", "tachalo", "tachala", "marca", "marcame", "marcar", "marcas")
	// "ya compré el pan": sin "ya", solo los de comprar.
	verbosYa     = conjunto("compre", "consegui", "traje", "tengo", "hice", "termine", "agarre", "busque", "pague", "compramos", "conseguimos", "trajimos")
	verbosSinYa  = conjunto("compre", "consegui", "traje", "compramos", "conseguimos", "trajimos")
	verbosVaciar = conjunto("vacia", "vaciame", "vaciar", "vacias", "vacies")
	verbosLimpia = conjunto("limpia", "limpiame", "limpiar", "limpias")
	// "¿qué hay en…?", "leeme…".
	pedirVer = [][]string{{"que", "hay"}, {"que", "tengo"}, {"que", "tenes"}, {"que", "tenemos"}, {"que", "me", "falta"},
		{"que", "nos", "falta"}, {"que", "falta"}, {"que", "faltaba"}, {"que", "dice"}, {"que", "me", "queda"}, {"que", "queda"},
		{"leeme"}, {"lee"}, {"leer"}, {"mostrame"}, {"mostra"}, {"decime"}, {"deci"}, {"pasame"}, {"ver"}, {"dame"}, {"repasame"},
		{"cual", "es"}, {"cuales", "son"}, {"como", "esta"}, {"como", "viene"}, {"hay", "algo"}}
	relleno      = conjunto("che", "porfa", "dale", "hola", "ey", "eh", "bueno", "oye", "oiga", "ah")
	antesDeVerbo = conjunto("podes", "podrias", "queres", "me", "nos", "te", "vos", "y", "tambien")
	articulos    = conjunto("el", "la", "los", "las", "mi", "mis", "tu", "tus", "nuestra", "nuestras", "nuestro", "esa", "esta")
	quitarAntes  = conjunto("el", "la", "los", "las", "unos", "unas")
	// Los nombres que valen sin decir "lista": el súper (y sus apodos) y
	// unos de siempre. Los demás, con "lista" ("la lista de regalos") o si ya
	// existe.
	apodosSuper = conjunto("super", "supermercado", "mercado", "chino", "compras", "compra", "almacen")
	conocidas   = conjunto("pendientes", "tareas", "farmacia", "verduleria", "ferreteria", "regalos", "peliculas", "series",
		"libros", "mandados", "cosas para hacer")
)

// analizador: el texto y las listas que ya tiene el usuario.
type analizador struct {
	ps     []palabra
	listas []string
}

// entenderLista: si el texto es de listas, qué pide. listas: los nombres de
// las que ya tiene (valen sin decir "lista").
func entenderLista(texto string, listas []string) (pedidoLista, bool) {
	a := analizador{ps: palabrasDe(texto), listas: listas}
	ps := a.ps
	// Relleno al principio y al final.
	for len(ps) > 0 {
		switch {
		case relleno[ps[0].n]:
			ps = ps[1:]
			continue
		case len(ps) > 1 && ps[0].n == "por" && ps[1].n == "favor":
			ps = ps[2:]
			continue
		}
		break
	}
	for len(ps) > 0 {
		k := len(ps) - 1
		switch {
		case relleno[ps[k].n] || ps[k].n == "gracias":
			ps = ps[:k]
			continue
		case k > 0 && ps[k-1].n == "por" && ps[k].n == "favor":
			ps = ps[:k-1]
			continue
		}
		break
	}
	if len(ps) == 0 || len(ps) > 40 {
		return pedidoLista{}, false
	}
	// "¿qué listas tengo?", "mis listas".
	if p, ok := a.cuales(ps); ok {
		return p, true
	}
	// "la lista del súper" sola.
	if n, ok := a.ref(ps, true); ok {
		return pedidoLista{accion: listaVer, lista: n}, true
	}
	if p, ok := a.ver(ps); ok {
		return p, true
	}
	// Un verbo (con "me", "podés"… antes).
	v := 0
	for v < len(ps)-1 && v < 3 && antesDeVerbo[ps[v].n] && !esVerboLista(ps[v].n) {
		v++
	}
	verbo, resto := ps[v].n, ps[v+1:]
	switch {
	case verbosAgregar[verbo]:
		return a.agregar(resto)
	case verbosVaciar[verbo]:
		if n, ok := a.ref(resto, false); ok {
			return pedidoLista{accion: listaVaciar, lista: n, nombrada: true}, true
		}
	case verbosLimpia[verbo]:
		if n, ok := a.ref(resto, false); ok {
			return pedidoLista{accion: listaLimpiar, lista: n, nombrada: true}, true
		}
		if p, ok := a.tachados(resto); ok {
			return p, true
		}
	case verbosSacar[verbo]:
		return a.sacar(resto)
	case verbosTachar[verbo]:
		return a.cosasYLista(listaTachar, resto, true)
	case verbo == "ya" && len(resto) > 0 && verbosYa[resto[0].n]:
		return a.cosasYLista(listaTachar, resto[1:], false)
	case verbosSinYa[verbo]:
		return a.cosasYLista(listaTachar, resto, false)
	}
	return pedidoLista{}, false
}

func esVerboLista(n string) bool {
	return verbosAgregar[n] || verbosSacar[n] || verbosTachar[n] || verbosVaciar[n] || verbosLimpia[n] || verbosSinYa[n] || n == "ya"
}

func normales(ps []palabra) string {
	l := make([]string, len(ps))
	for k, p := range ps {
		l[k] = p.n
	}
	return strings.Join(l, " ")
}

func originales(ps []palabra) string {
	l := make([]string, len(ps))
	for k, p := range ps {
		l[k] = p.orig
	}
	return strings.Join(l, " ")
}

// ref: si las palabras (hasta el final) nombran una lista. conLista: solo si
// dicen "lista" ("la lista", "mi lista de regalos"). "" = "la lista", sin
// decir cuál.
func (a analizador) ref(ps []palabra, conLista bool) (string, bool) {
	k := 0
	for k < len(ps) && articulos[ps[k].n] {
		k++
	}
	if k < len(ps) && (ps[k].n == "lista" || ps[k].n == "listita") {
		k++
		if k < len(ps) && (ps[k].n == "de" || ps[k].n == "del" || ps[k].n == "para" || ps[k].n == "que" && k+2 < len(ps)) {
			if ps[k].n == "que" { // "que se llama x"
				if ps[k+1].n != "se" || ps[k+2].n != "llama" {
					return "", false
				}
				k += 2
			}
			k++
			for k < len(ps) && articulos[ps[k].n] {
				k++
			}
			if k == len(ps) {
				return "", false
			}
		}
		if k == len(ps) {
			return "", true
		}
		return a.resolver(ps[k:], true)
	}
	if conLista || k == len(ps) {
		return "", false
	}
	return a.resolver(ps[k:], false)
}

// resolver: el nombre de la lista (la que ya existe, el súper o uno nuevo).
func (a analizador) resolver(ps []palabra, explicita bool) (string, bool) {
	n := normales(ps)
	for _, l := range a.listas {
		if normal(l) == n {
			return l, true
		}
	}
	if apodosSuper[n] {
		return ListaPorDefecto, true
	}
	nombre := strings.ToLower(originales(ps))
	if (explicita || conocidas[n]) && len(nombre) <= almacen.MaxNombreLista {
		return nombre, true
	}
	return "", false
}

func (a analizador) cuales(ps []palabra) (pedidoLista, bool) {
	t := " " + normales(ps) + " "
	if !strings.Contains(t, " listas ") {
		return pedidoLista{}, false
	}
	for _, s := range []string{"listas", "mis listas", "las listas", "que listas tengo", "que listas hay", "cuales son mis listas",
		"cuales son las listas", "que listas tenes", "mostrame mis listas", "mostrame las listas", "decime mis listas"} {
		if strings.TrimSpace(t) == s {
			return pedidoLista{accion: listaCuales}, true
		}
	}
	return pedidoLista{}, false
}

// ver: "¿qué hay en la lista del súper?", "¿qué me falta comprar?", "leeme pendientes".
func (a analizador) ver(ps []palabra) (pedidoLista, bool) {
	var resto []palabra
	for _, pre := range pedirVer {
		if len(ps) < len(pre) {
			continue
		}
		ok := true
		for k, w := range pre {
			if ps[k].n != w {
				ok = false
				break
			}
		}
		if ok {
			resto = ps[len(pre):]
			break
		}
	}
	if resto == nil {
		return pedidoLista{}, false
	}
	// "leeme la lista del súper", "mostrame pendientes".
	if n, ok := a.ref(resto, false); ok {
		return pedidoLista{accion: listaVer, lista: n}, true
	}
	// "qué hay en pendientes", "qué me falta comprar en el súper".
	for k, p := range resto {
		if p.n == "en" || p.n == "de" || p.n == "del" || p.n == "para" || p.n == "al" {
			if n, ok := a.ref(resto[k+1:], false); ok {
				return pedidoLista{accion: listaVer, lista: n}, true
			}
		}
	}
	t := " " + normales(resto) + " "
	switch {
	case strings.Contains(t, " comprar ") || strings.Contains(t, " compras "):
		return pedidoLista{accion: listaVer, lista: ListaPorDefecto}, true
	case strings.Contains(t, " lista "):
		return pedidoLista{accion: listaVer}, true
	}
	return pedidoLista{}, false
}

// agregar: "anotá yerba y pan en la lista del súper", "sumá pilas a pendientes",
// "anotá en el súper: yerba". Sin decir la lista, no es de listas (el motor
// lo entiende como un recordatorio).
func (a analizador) agregar(ps []palabra) (pedidoLista, bool) {
	for k, p := range ps {
		if p.dos && k+1 < len(ps) { // "anotá en el súper: yerba"
			izq := ps[:k+1]
			if len(izq) > 0 && (izq[0].n == "en" || izq[0].n == "a" || izq[0].n == "al" || izq[0].n == "para") {
				izq = izq[1:]
			}
			if n, ok := a.ref(izq, false); ok {
				if items := separarItems(ps[k+1:]); len(items) > 0 {
					return pedidoLista{accion: listaAgregar, lista: n, items: items}, true
				}
			}
			break
		}
	}
	for k := 1; k < len(ps); k++ {
		switch ps[k].n {
		case "a", "al", "en", "para", "pa":
			if n, ok := a.ref(ps[k+1:], false); ok {
				if items := separarItems(ps[:k]); len(items) > 0 {
					return pedidoLista{accion: listaAgregar, lista: n, items: items}, true
				}
				return pedidoLista{}, false
			}
		}
	}
	return pedidoLista{}, false
}

// sacar: "sacá la yerba de la lista", "borrá la lista del súper" (la vacía),
// "sacá lo tachado".
func (a analizador) sacar(ps []palabra) (pedidoLista, bool) {
	if len(ps) == 0 {
		return pedidoLista{}, false
	}
	if n, ok := a.ref(ps, true); ok {
		return pedidoLista{accion: listaVaciar, lista: n, nombrada: true}, true
	}
	if p, ok := a.tachados(ps); ok {
		return p, true
	}
	p, ok := a.cosasYLista(listaSacar, ps, true)
	if ok && len(p.items) == 1 && normal(p.items[0]) == "todo" {
		// "borrá todo" sin decir de dónde no es de listas.
		return pedidoLista{accion: listaVaciar, lista: p.lista, nombrada: true}, p.nombrada
	}
	return p, ok
}

// tachados: "los tachados (de la lista X)", "lo comprado".
func (a analizador) tachados(ps []palabra) (pedidoLista, bool) {
	k := 0
	for k < len(ps) && (articulos[ps[k].n] || ps[k].n == "lo") {
		k++
	}
	if k == len(ps) {
		return pedidoLista{}, false
	}
	switch ps[k].n {
	case "tachado", "tachados", "tachadas", "comprado", "comprados", "hecho", "hechos", "hechas", "marcados", "marcadas":
	default:
		return pedidoLista{}, false
	}
	resto := ps[k+1:]
	if len(resto) == 0 {
		return pedidoLista{accion: listaLimpiar}, true
	}
	if resto[0].n == "de" || resto[0].n == "del" || resto[0].n == "en" {
		if n, ok := a.ref(resto[1:], false); ok {
			return pedidoLista{accion: listaLimpiar, lista: n}, true
		}
	}
	return pedidoLista{}, false
}

// cosasYLista: "la yerba y el pan [de la lista del súper] [como comprado]".
// Sin decir la lista, las busca en todas al hacerlo (si no están, no es de
// listas).
func (a analizador) cosasYLista(acc accionLista, ps []palabra, conDel bool) (pedidoLista, bool) {
	for k, p := range ps {
		if p.n == "como" && k > 0 { // "marcá la yerba como comprada"
			ps = ps[:k]
			break
		}
	}
	for k := 1; k < len(ps); k++ {
		if ps[k].n == "de" || ps[k].n == "del" || (ps[k].n == "en" && conDel) || ps[k].n == "para" {
			if n, ok := a.ref(ps[k+1:], false); ok {
				items := separarItems(ps[:k])
				return pedidoLista{accion: acc, lista: n, items: items, nombrada: true}, len(items) > 0
			}
		}
	}
	items := separarItems(ps)
	return pedidoLista{accion: acc, items: items}, len(items) > 0
}

// separarItems: "leche, pan y el queso" → leche, pan, queso.
func separarItems(ps []palabra) []string {
	var items []string
	var actual []palabra
	cerrar := func() {
		for len(actual) > 1 && quitarAntes[actual[0].n] {
			actual = actual[1:]
		}
		if len(actual) > 0 {
			t := originales(actual)
			repetido := false
			for _, x := range items {
				repetido = repetido || normal(x) == normal(t)
			}
			if !repetido && len(t) <= almacen.MaxTextoItem {
				items = append(items, t)
			}
		}
		actual = nil
	}
	for _, p := range ps {
		if p.n == "y" || p.n == "e" || p.n == "+" {
			cerrar()
			continue
		}
		actual = append(actual, p)
		if p.coma {
			cerrar()
		}
	}
	cerrar()
	return items
}

// SepararItems: lo que se escribe en la web o el escritorio ("leche, pan y
// queso") en cosas sueltas.
func SepararItems(texto string) []string { return separarItems(palabrasDe(texto)) }

// --- Hacerlo ---

// resultadoLista: lo que se contesta (corto: en el globo del robot, hasta 60
// bytes; largo: en Telegram, si hay más para mostrar).
type resultadoLista struct {
	corto, largo, expresion string
	cambio                  bool
	accion                  string
}

func (r resultadoLista) paraTelegram() string {
	if r.largo != "" {
		return r.largo
	}
	return r.corto
}

// listaPara: la lista con ese nombre o, sin nombre, la del súper (o la única
// que tenga).
func listaPara(ls []almacen.Lista, nombre string) *almacen.Lista {
	if nombre == "" {
		nombre = ListaPorDefecto
		if len(ls) == 1 && !esSuper(ls[0].Nombre) {
			return &ls[0]
		}
	}
	for k := range ls {
		if normal(ls[k].Nombre) == normal(nombre) {
			return &ls[k]
		}
	}
	return nil
}

func esSuper(n string) bool { return normal(n) == normal(ListaPorDefecto) }

// Mayuscula: la primera letra en mayúscula ("súper" → "Súper").
func Mayuscula(s string) string {
	for k, r := range s {
		return strings.ToUpper(string(r)) + s[k+len(string(r)):]
	}
	return s
}

// juntarY: "a, b y c".
func juntarY(l []string) string {
	if len(l) <= 1 {
		return strings.Join(l, "")
	}
	return strings.Join(l[:len(l)-1], ", ") + " y " + l[len(l)-1]
}

// enumerar: "Súper: yerba, pan, queso y 2 más" en hasta max bytes.
func enumerar(prefijo string, cosas []string, max int) string {
	for n := len(cosas); n >= 1; n-- {
		s := prefijo + juntarY(cosas[:n])
		if n < len(cosas) {
			s = prefijo + strings.Join(cosas[:n], ", ") + fmt.Sprintf(" y %d más", len(cosas)-n)
		}
		if len(s) <= max {
			return s
		}
	}
	if len(cosas) == 0 {
		return strings.TrimSuffix(strings.TrimSpace(prefijo), ":")
	}
	return protocolo.CortarUTF8(prefijo+cosas[0], max)
}

func textos(items []almacen.ItemLista) []string {
	l := make([]string, len(items))
	for k, it := range items {
		l[k] = it.Texto
	}
	return l
}

// buscarItem: la cosa que se parece a t (igual sin tildes ni mayúsculas, en
// singular o plural, o una sola que la contenga como palabras enteras).
func buscarItem(items []almacen.ItemLista, t string, soloPendientes bool) (almacen.ItemLista, bool) {
	nt := normal(t)
	singular := func(s string) string {
		ws := strings.Fields(s)
		for k, w := range ws {
			w = strings.TrimSuffix(w, "s")
			ws[k] = strings.TrimSuffix(w, "e")
		}
		return strings.Join(ws, " ")
	}
	var parecidos []almacen.ItemLista
	for _, pasada := range []int{0, 1} {
		for _, it := range items {
			if soloPendientes && it.Hecho {
				continue
			}
			ni := normal(it.Texto)
			if pasada == 0 && (ni == nt || singular(ni) == singular(nt)) {
				return it, true
			}
			if pasada == 1 && (strings.Contains(" "+ni+" ", " "+nt+" ") || strings.Contains(" "+nt+" ", " "+ni+" ")) {
				parecidos = append(parecidos, it)
			}
		}
	}
	if len(parecidos) == 1 {
		return parecidos[0], true
	}
	return almacen.ItemLista{}, false
}

// errorLista: el error para el usuario (los de los topes, tal cual).
func errorLista(err error) (string, bool) {
	for _, e := range []error{almacen.ErrTopeItems, almacen.ErrTopeListas, almacen.ErrTextoItem, almacen.ErrNombreLista} {
		if errors.Is(err, e) {
			return Mayuscula(e.Error()), true
		}
	}
	return "", false
}

// hacerLista: hace lo pedido. ok = false: no era de listas (las cosas que
// tachar o sacar no están en ninguna).
func (i *Integraciones) hacerLista(ctx context.Context, usuario string, p pedidoLista, ls []almacen.Lista) (r resultadoLista, ok bool, err error) {
	r.accion = nombresAccionLista[p.accion]
	nombreDe := func(l *almacen.Lista) string { return l.Nombre }
	noTiene := func() resultadoLista {
		return resultadoLista{corto: protocolo.CortarUTF8("No tenés la lista "+p.lista, protocolo.MaxDecir), expresion: "pensando", accion: r.accion}
	}
	switch p.accion {
	case listaCuales:
		if len(ls) == 0 {
			r.corto, r.expresion = "No tenés listas todavía", "pensando"
			r.largo = "No tenés listas todavía. Escribime «anotá yerba en el súper» y la armo."
			return r, true, nil
		}
		var l []string
		for _, x := range ls {
			l = append(l, fmt.Sprintf("%s (%d)", x.Nombre, len(x.Pendientes())))
		}
		r.corto, r.expresion = enumerar("Listas: ", l, protocolo.MaxDecir), "contento"
		r.largo = "Tus listas: " + strings.Join(l, ", ")
		return r, true, nil

	case listaVer:
		l := listaPara(ls, p.lista)
		if l == nil && p.lista == "" && len(ls) > 0 {
			// "la lista", sin súper y con varias: cuáles hay.
			return i.hacerLista(ctx, usuario, pedidoLista{accion: listaCuales}, ls)
		}
		if l == nil {
			if p.lista != "" && !esSuper(p.lista) {
				return noTiene(), true, nil
			}
			n := ListaPorDefecto
			r.corto, r.largo, r.expresion = Mayuscula(n)+": no falta nada", Mayuscula(n)+": no falta nada.", "contento"
			return r, true, nil
		}
		r.corto, r.largo, r.expresion = textoCortoLista(*l), textoLargoLista(*l), "pensando"
		return r, true, nil

	case listaAgregar:
		nombre := p.lista
		if nombre == "" {
			if l := listaPara(ls, ""); l != nil {
				nombre = l.Nombre
			} else {
				nombre = ListaPorDefecto
			}
		}
		l, _, err := i.alm.AsegurarLista(ctx, usuario, nombre)
		if err != nil {
			return r, true, err
		}
		nuevos, ya, err := i.alm.AgregarItems(ctx, usuario, l.ID, p.items)
		if err != nil {
			return r, true, err
		}
		r.cambio = true
		if len(nuevos) == 0 {
			r.corto, r.expresion = enumerar("Ya estaba: ", textos(ya), protocolo.MaxDecir), "guino"
			return r, true, nil
		}
		prefijo := "Anotado: "
		if !esSuper(l.Nombre) {
			prefijo = "Anotado en " + l.Nombre + ": "
			if len(prefijo) > 30 {
				prefijo = "Anotado: "
			}
		}
		r.corto, r.expresion = enumerar(prefijo, textos(nuevos), protocolo.MaxDecir), "contento"
		r.largo = strings.TrimSpace(prefijo + juntarY(textos(nuevos)) + " (" + l.Nombre + ").")
		return r, true, nil

	case listaSacar, listaTachar:
		var donde []*almacen.Lista
		if p.lista != "" {
			l := listaPara(ls, p.lista)
			if l == nil {
				return noTiene(), true, nil
			}
			donde = append(donde, l)
		} else {
			// Primero la del súper (o la que diga listaPara), después las demás.
			if l := listaPara(ls, ""); l != nil {
				donde = append(donde, l)
			}
			for k := range ls {
				if len(donde) == 0 || donde[0].ID != ls[k].ID {
					donde = append(donde, &ls[k])
				}
			}
		}
		var hechos, faltan []string
		for _, t := range p.items {
			encontrado := false
			for _, l := range donde {
				it, ok := buscarItem(l.Items, t, true)
				if !ok {
					it, ok = buscarItem(l.Items, t, false)
				}
				if !ok {
					continue
				}
				encontrado = true
				if p.accion == listaSacar {
					err = i.alm.BorrarItem(ctx, usuario, l.ID, it.ID)
				} else if !it.Hecho {
					si := true
					_, err = i.alm.CambiarItem(ctx, usuario, l.ID, it.ID, &si, nil)
				}
				if err != nil {
					return r, true, err
				}
				hechos = append(hechos, it.Texto)
				break
			}
			if !encontrado {
				faltan = append(faltan, t)
			}
		}
		if len(hechos) == 0 {
			if !p.nombrada {
				return r, false, nil
			}
			r.corto, r.expresion = enumerar("No estaba: ", faltan, protocolo.MaxDecir), "pensando"
			return r, true, nil
		}
		r.cambio = true
		prefijo, cara := "Saqué: ", "neutral"
		if p.accion == listaTachar {
			prefijo, cara = "Tachado: ", "contento"
		}
		r.corto, r.expresion = enumerar(prefijo, hechos, protocolo.MaxDecir), cara
		r.largo = prefijo + juntarY(hechos) + "."
		if len(faltan) > 0 {
			r.largo += " No encontré: " + juntarY(faltan) + "."
		}
		return r, true, nil

	case listaVaciar, listaLimpiar:
		l := listaPara(ls, p.lista)
		if l == nil {
			if p.lista != "" && !esSuper(p.lista) {
				return noTiene(), true, nil
			}
			r.corto, r.expresion = "No hay nada para sacar", "neutral"
			return r, true, nil
		}
		n, err := i.alm.LimpiarLista(ctx, usuario, l.ID, p.accion == listaVaciar)
		if err != nil {
			return r, true, err
		}
		r.cambio = n > 0
		switch {
		case p.accion == listaVaciar:
			r.corto, r.expresion = protocolo.CortarUTF8("Vacié la lista "+nombreDe(l), protocolo.MaxDecir), "guino"
		case n == 0:
			r.corto, r.expresion = "No había nada tachado", "neutral"
		default:
			r.corto, r.expresion = protocolo.CortarUTF8(fmt.Sprintf("Saqué %d tachadas de %s", n, nombreDe(l)), protocolo.MaxDecir), "guino"
		}
		return r, true, nil
	}
	return r, false, nil
}

// textoCortoLista: "Súper: yerba, pan, queso y 2 más".
func textoCortoLista(l almacen.Lista) string {
	p := l.Pendientes()
	if len(p) == 0 {
		return protocolo.CortarUTF8(Mayuscula(l.Nombre)+": no falta nada", protocolo.MaxDecir)
	}
	return enumerar(protocolo.CortarUTF8(Mayuscula(l.Nombre), 30)+": ", textos(p), protocolo.MaxDecir)
}

// textoLargoLista: la lista numerada (las tachadas, solo cuántas).
func textoLargoLista(l almacen.Lista) string {
	var b strings.Builder
	b.WriteString(Mayuscula(l.Nombre))
	p := l.Pendientes()
	if len(p) == 0 {
		b.WriteString(": no falta nada.")
	} else {
		b.WriteString(":")
		for k, it := range p {
			fmt.Fprintf(&b, "\n%d. %s", k+1, it.Texto)
		}
	}
	if t := len(l.Items) - len(p); t == 1 {
		b.WriteString("\n\n(1 tachada)")
	} else if t > 1 {
		fmt.Fprintf(&b, "\n\n(%d tachadas)", t)
	}
	return b.String()
}

// ListaDesdeTexto: si el texto es de listas, lo hace y dice qué contestar.
func (i *Integraciones) ListaDesdeTexto(ctx context.Context, usuario, texto string) (resultadoLista, bool) {
	ls, err := i.alm.Listas(ctx, usuario)
	if err != nil {
		i.log.Warn("listas: no pude leerlas", "error", err)
		return resultadoLista{}, false
	}
	nombres := make([]string, len(ls))
	for k, l := range ls {
		nombres[k] = l.Nombre
	}
	p, ok := entenderLista(texto, nombres)
	if !ok {
		return resultadoLista{}, false
	}
	r, ok, err := i.hacerLista(ctx, usuario, p, ls)
	if err != nil {
		if t, ok := errorLista(err); ok {
			return resultadoLista{corto: protocolo.CortarUTF8(t, protocolo.MaxDecir), largo: t + ".", expresion: "pensando", accion: r.accion}, true
		}
		i.log.Warn("listas: no pude", "error", err)
		return resultadoLista{corto: "Uy, no pude con la lista", expresion: "mareado", accion: r.accion}, true
	}
	if r.cambio {
		i.PublicarListas(usuario)
	}
	return r, ok
}

// PublicarListas avisa a la web y al escritorio que cambiaron las listas.
func (i *Integraciones) PublicarListas(usuario string) {
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "listas"})
}

// charlaDeListas: la charla con un robot, si es de listas: lo dice en su
// pantalla (si está conectado) sin pasar por el motor ni por la IA.
func (i *Integraciones) charlaDeListas(ctx context.Context, usuario, bicho, texto, desde string) (RespuestaCharla, bool) {
	r, ok := i.ListaDesdeTexto(ctx, usuario, texto)
	if !ok {
		return RespuestaCharla{}, false
	}
	res := RespuestaCharla{Texto: r.corto, Expresion: r.expresion, Quedan: -1, Origen: OrigenMotor, Intencion: "lista"}
	if r.cambio {
		res.Accion = "lista"
	}
	if m, conIA := i.motorDe(ctx, usuario); conIA {
		res.Propia = m.propia
		res.Quedan = i.charlasQuedan(ctx, usuario, m.tope)
	}
	if i.robots.Conectado(bicho) {
		if md, err := protocolo.Decir(protocolo.CortarUTF8(res.Texto, protocolo.MaxDecir), res.Expresion); err == nil {
			res.Enviado = i.robots.Enviar(bicho, protocolo.CarComando, md) == nil
		}
	}
	i.alm.AnotarRobotCharla(ctx, usuario, bicho) //nolint:errcheck
	i.hub.Publicar(usuario, eventos.Evento{Tipo: "charla", Bicho: bicho, Datos: map[string]any{
		"texto": res.Texto, "expresion": res.Expresion, "de": desde, "enviado": res.Enviado, "origen": OrigenMotor,
		"intencion": res.Intencion, "accion": res.Accion}})
	return res, true
}

// resumenListas: para la IA ("súper: yerba, pan y 3 más; pendientes: nada").
func (i *Integraciones) resumenListas(ctx context.Context, usuario string) string {
	ls, err := i.alm.Listas(ctx, usuario)
	if err != nil || len(ls) == 0 {
		return ""
	}
	var l []string
	for k, x := range ls {
		if k == 6 {
			l = append(l, fmt.Sprintf("y %d listas más", len(ls)-k))
			break
		}
		p := textos(x.Pendientes())
		switch {
		case len(p) == 0:
			l = append(l, x.Nombre+": no falta nada")
		case len(p) > 6:
			l = append(l, fmt.Sprintf("%s: %s y %d más", x.Nombre, strings.Join(p[:6], ", "), len(p)-6))
		default:
			l = append(l, x.Nombre+": "+strings.Join(p, ", "))
		}
	}
	return protocolo.CortarUTF8(strings.Join(l, "; "), 800)
}

// TextoListaTelegram: /lista [nombre].
func (i *Integraciones) TextoListaTelegram(ctx context.Context, usuario, nombre string) string {
	ls, err := i.alm.Listas(ctx, usuario)
	if err != nil {
		return "No pude leer tus listas ahora: probá en un rato."
	}
	if len(ls) == 0 {
		return "No tenés listas todavía. Escribime «anotá yerba en el súper» y la armo."
	}
	nombre = strings.TrimSpace(nombre)
	if nombre != "" {
		a := analizador{listas: textosListas(ls)}
		if n, ok := a.ref(palabrasDe(nombre), false); ok {
			nombre = n
		}
	}
	l := listaPara(ls, nombre)
	if l == nil {
		return "No tenés la lista «" + nombre + "». Tus listas: " + strings.Join(textosListas(ls), ", ") + "."
	}
	t := textoLargoLista(*l)
	var otras []string
	for _, x := range ls {
		if x.ID != l.ID {
			otras = append(otras, x.Nombre)
		}
	}
	if len(otras) > 0 {
		t += "\n\nOtras: " + strings.Join(otras, ", ") + " (/lista nombre)."
	}
	return t
}

func textosListas(ls []almacen.Lista) []string {
	l := make([]string, len(ls))
	for k, x := range ls {
		l[k] = x.Nombre
	}
	return l
}

// --- Para el MCP ---

// ResolverLista: el nombre de la lista a partir de lo que escribieron
// ("super" → "súper"; una que ya existe, con su nombre; "" → la del súper o
// la única).
func ResolverLista(nombre string, ls []almacen.Lista) string {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		if l := listaPara(ls, ""); l != nil {
			return l.Nombre
		}
		return ListaPorDefecto
	}
	a := analizador{listas: textosListas(ls)}
	ps := palabrasDe(nombre)
	if n, ok := a.ref(ps, false); ok && n != "" {
		return n
	}
	if n, ok := a.resolver(ps, true); ok {
		return n
	}
	return nombre
}

// ListaLlamada: la lista con ese nombre (sin mirar tildes ni mayúsculas).
func ListaLlamada(ls []almacen.Lista, nombre string) *almacen.Lista {
	if nombre == "" {
		return nil
	}
	return listaPara(ls, nombre)
}

// BuscarItem: la cosa de la lista que se parece a t (primero las pendientes).
func BuscarItem(l almacen.Lista, t string) (almacen.ItemLista, bool) {
	if it, ok := buscarItem(l.Items, t, true); ok {
		return it, true
	}
	return buscarItem(l.Items, t, false)
}
