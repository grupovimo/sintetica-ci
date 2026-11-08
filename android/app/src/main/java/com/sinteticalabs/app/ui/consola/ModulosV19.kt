package com.sinteticalabs.app.ui.consola

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.consola.Acompanar
import com.sinteticalabs.app.consola.Compania
import com.sinteticalabs.app.consola.Lab
import com.sinteticalabs.app.consola.cortarBytes
import com.sinteticalabs.app.consola.largoBytes
import com.sinteticalabs.app.datos.Agenda
import com.sinteticalabs.app.datos.Amigo
import com.sinteticalabs.app.datos.EnVivo
import com.sinteticalabs.app.datos.Entrantes
import com.sinteticalabs.app.datos.ErrorApi
import com.sinteticalabs.app.datos.HabitoVista
import com.sinteticalabs.app.datos.Horarios
import com.sinteticalabs.app.datos.Invitacion
import com.sinteticalabs.app.datos.Musica
import com.sinteticalabs.app.datos.RobotNube
import com.sinteticalabs.app.datos.aceptarAmigo
import com.sinteticalabs.app.datos.agenda
import com.sinteticalabs.app.datos.amigoRecibe
import com.sinteticalabs.app.datos.amigos
import com.sinteticalabs.app.datos.avisoAgenda
import com.sinteticalabs.app.datos.borrarAmigo
import com.sinteticalabs.app.datos.charla
import com.sinteticalabs.app.datos.charlar
import com.sinteticalabs.app.datos.conectarAgenda
import com.sinteticalabs.app.datos.entrantes
import com.sinteticalabs.app.datos.habitos
import com.sinteticalabs.app.datos.horarios
import com.sinteticalabs.app.datos.invitar
import com.sinteticalabs.app.datos.mercadoPago
import com.sinteticalabs.app.datos.musica
import com.sinteticalabs.app.datos.probarEntrante
import com.sinteticalabs.app.datos.regenerarEntrantes
import com.sinteticalabs.app.datos.robots
import com.sinteticalabs.app.datos.sacarAgenda
import com.sinteticalabs.app.datos.sacarSpotify
import com.sinteticalabs.app.datos.tocar
import com.sinteticalabs.app.datos.urlSpotify
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private val SOBRE_PLACA = Color(0xCC1A1714)

private fun copiar(c: Context, t: String) {
    (c.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager).setPrimaryClip(ClipData.newPlainText("Sintética", t))
}

private fun abrir(c: Context, u: String) {
    runCatching { c.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(u)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
}

/** Sin la nube no hay amigos, agenda ni avisos de afuera. */
@Composable
private fun SoloNube() {
    Bandeja {
        Text("EN LA NUBE", style = T.nombre.copy(fontSize = 22.sp), color = C.tinta)
        Text("Sumalo a tu cuenta (Ajustes).", style = T.texto, color = SOBRE_PLACA)
    }
}

// --- Hábitos: hasta 4, la tecla grande lo marca hecho hoy; la racha y 60 días en puntitos ---

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ModHabitos(vm: RobotViewModel) {
    val c by vm.config.collectAsState()
    val guardado by vm.guardado.collectAsState()
    val alcance = rememberCoroutineScope()
    val contexto = LocalContext.current
    val habitos = Compania.habitos(c)
    var vistas by remember { mutableStateOf<List<HabitoVista>>(emptyList()) }
    var aviso by remember { mutableStateOf<String?>(null) }
    val api = vm.apiNube()
    LaunchedEffect(guardado) {
        if (vm.via == Via.NUBE && api != null && guardado != EstadoGuardado.GUARDANDO) runCatching { vistas = api.habitos(vm.idRobot) }
    }
    fun vista(i: Int) = vistas.firstOrNull { it.indice == i && it.nombre == habitos.getOrNull(i)?.nombre }
    val ultimos = remember {
        val hoy = java.time.LocalDate.now()
        (59 downTo 0).map { hoy.minusDays(it.toLong()).toString() }
    }
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        habitos.forEachIndexed { i, h ->
            val v = vista(i)
            Bandeja {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Tecla(null, {
                        alcance.launch {
                            val hecho = v?.hoy != true
                            val (nueva, e) = vm.marcarHabito(i, hecho)
                            if (nueva != null) vistas = vistas.filter { it.indice != i } + nueva
                            aviso = e ?: if (!hecho) "Desmarcado" else if ((nueva?.racha ?: 0) >= 3) "¡${nueva!!.racha} días seguidos!" else "¡Bien ahí!"
                            delay(2500); aviso = null
                        }
                    }, acabado = if (v?.hoy == true) Acabado.NARANJA else Acabado.BLANCA, cuadrada = true, alto = 60.dp, apretada = v?.hoy == true,
                        descripcion = "${h.nombre}: ${if (v?.hoy == true) "hecho hoy" else "marcar hecho"}") {
                        Text(Compania.GLIFO_HABITO[h.icono] ?: "★", style = T.nombre.copy(fontSize = 24.sp))
                    }
                    Visor(Modifier.weight(1f)) {
                        CampoVisor(h.nombre, { t -> vm.cambiar { Compania.cambiarHabito(it, i) { x -> Compania.conNombre(x, t) } } },
                            estilo = T.campoClaro, maxBytes = Compania.MAX_NOMBRE_HABITO)
                    }
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text("${v?.racha ?: 0}", style = T.digitosChicos)
                        Grabado(Compania.textoRacha(v?.racha ?: 0), color = SOBRE_PLACA)
                    }
                    Tecla("✕", { vm.cambiar { Compania.quitarHabito(it, i) } }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Quitar ${h.nombre}")
                }
                FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Compania.ICONOS_HABITO.forEach { ic ->
                        Box(
                            Modifier.size(36.dp).clip(RoundedCornerShape(10.dp)).background(if (h.icono == ic) C.tinta else Color(0x241A1714))
                                .clickable(role = Role.RadioButton) { vm.cambiar { Compania.cambiarHabito(it, i) { x -> x.copy(icono = ic) } } },
                            contentAlignment = Alignment.Center,
                        ) { Text(Compania.GLIFO_HABITO[ic] ?: "★", style = T.dato) }
                    }
                }
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Llave("Pregunta", h.hora != null, { vm.cambiar { Compania.cambiarHabito(it, i) { x -> x.copy(hora = if (x.hora != null) null else "09:00") } } })
                    h.hora?.let { hora ->
                        Text(hora, style = T.digitosChicos.copy(shadow = null, color = C.tinta), modifier = Modifier.clickable {
                            android.app.TimePickerDialog(contexto, { _, hh, mm -> vm.cambiar { Compania.cambiarHabito(it, i) { x -> x.copy(hora = "%02d:%02d".format(hh, mm)) } } },
                                com.sinteticalabs.app.consola.Editar.aMin(hora) / 60, com.sinteticalabs.app.consola.Editar.aMin(hora) % 60, true).show()
                        })
                    }
                }
                FilaDias(h.dias?.ifEmpty { null } ?: "LMXJVSD") { l -> vm.cambiar { Compania.cambiarHabito(it, i) { x -> Compania.alternarDia(x, l) } } }
                // Los últimos 60 días.
                Canvas(Modifier.fillMaxWidth().height(28.dp)) {
                    val porFila = 30
                    val paso = size.width / porFila
                    ultimos.forEachIndexed { k, d ->
                        val x = (k % porFila) * paso + paso / 2
                        val y = (k / porFila) * (size.height / 2) + size.height / 4
                        val si = v?.dias?.contains(d) == true
                        drawCircle(if (si) C.tinta else Color(0x331A1714), if (k == 59) paso * 0.38f else paso * 0.28f, Offset(x, y))
                    }
                }
            }
        }
        if (habitos.size < Compania.MAX_HABITOS) Tecla("+ Hábito", { vm.cambiar { Compania.sumarHabito(it) } }, Modifier.fillMaxWidth(), acabado = Acabado.NEGRA)
        aviso?.let { AvisoLinea(it) }
    }
}

// --- Agenda: tu calendario por iCal; avisa antes y a la noche lo de mañana ---

@Composable
fun ModAgenda(vm: RobotViewModel) {
    val api = vm.apiNube()
    if (api == null) { SoloNube(); return }
    val contexto = LocalContext.current
    val alcance = rememberCoroutineScope()
    var agenda by remember { mutableStateOf<Agenda?>(null) }
    var ics by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var conectando by remember { mutableStateOf(false) }
    var aviso by remember { mutableStateOf(10.0) }
    suspend fun cargar() { runCatching { agenda = api.agenda(); aviso = agenda!!.aviso.toDouble() }.onFailure { error = it.message } }
    LaunchedEffect(Unit) {
        cargar()
        EnVivo.eventos.collect { if (it.tipo == "agenda") cargar() }
    }
    LaunchedEffect(aviso) {
        val a = agenda ?: return@LaunchedEffect
        if (aviso.toInt() == a.aviso) return@LaunchedEffect
        delay(600)
        runCatching { api.avisoAgenda(aviso.toInt()) }
    }
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        error?.let { AvisoLinea(it, error = true) }
        val a = agenda
        when {
            a == null -> Grabado("Buscando…", color = SOBRE_PLACA)
            !a.conectada -> {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    listOf("⚙" to "Configuración", "▦" to "Tu calendario", "🔒" to "Dirección secreta iCal").forEachIndexed { i, (g, t) ->
                        Bandeja(Modifier.weight(1f)) {
                            Grabado("${i + 1}", color = C.brasa)
                            Text(g, style = T.nombre.copy(fontSize = 20.sp), color = C.tinta)
                            Text(t, style = T.dato.copy(fontSize = 11.sp, color = C.tinta), maxLines = 2)
                        }
                    }
                }
                Tecla("Abrir Google Calendar ↗", { abrir(contexto, "https://calendar.google.com/calendar/r/settings") }, acabado = Acabado.NEGRA, chica = true)
                Visor(Modifier.fillMaxWidth()) {
                    Grabado("Pegala acá")
                    CampoVisor(ics, { ics = it }, placeholder = "https://calendar.google.com/…/basic.ics", estilo = T.dato.copy(color = C.papel),
                        maxBytes = 1000, teclado = androidx.compose.ui.text.input.KeyboardType.Uri, alEscribir = { ics = it })
                    Tecla(if (conectando) "…" else "Conectar", {
                        alcance.launch {
                            conectando = true; error = null
                            runCatching { agenda = api.conectarAgenda(ics.trim()); ics = "" }
                                .onFailure { error = if (it is ErrorApi) it.message + if (it.detalle.isNotEmpty()) ": ${it.detalle}" else "" else "No pude conectarlo." }
                            conectando = false
                        }
                    }, acabado = Acabado.NARANJA, chica = true, habilitada = ics.isNotBlank() && !conectando)
                }
            }
            else -> {
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
                    Bandeja(Modifier.weight(1f)) {
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                            Led(if (a.error == null) C.verde else C.ambar)
                            Grabado(if (a.error == null) "Conectada" else "No la pude leer", color = SOBRE_PLACA)
                        }
                        Text(a.ics, style = T.dato.copy(fontSize = 11.sp, color = C.tinta), maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Tecla("Sacar", { alcance.launch { runCatching { api.sacarAgenda() }; cargar() } }, acabado = Acabado.NEGRA, chica = true)
                    }
                    Bandeja(Modifier.width(120.dp)) {
                        Perilla(aviso, "Avisa", { aviso = it }, Modifier.fillMaxWidth(), min = 0.0, max = 15.0, paso = 1.0, tamano = 56.dp,
                            mostrar = { if (it > 0) "${it.toInt()}′" else "A la hora" })
                    }
                }
                if (a.proximos.isEmpty()) Visor(Modifier.fillMaxWidth()) { Grabado("Nada en dos semanas") }
                a.proximos.forEach { e ->
                    val f = runCatching {
                        if (e.todoElDia) java.time.LocalDate.parse(e.inicio.take(10)).atStartOfDay()
                        else java.time.OffsetDateTime.parse(e.inicio).atZoneSameInstant(java.time.ZoneId.systemDefault()).toLocalDateTime()
                    }.getOrNull()
                    val hoy = f?.toLocalDate() == java.time.LocalDate.now()
                    Row(
                        Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp)).background(C.hundido)
                            .then(if (hoy) Modifier.border(2.dp, C.led, RoundedCornerShape(16.dp)) else Modifier).padding(12.dp),
                        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.width(40.dp)) {
                            Grabado(f?.dayOfWeek?.getDisplayName(java.time.format.TextStyle.SHORT, java.util.Locale.forLanguageTag("es-AR")) ?: "")
                            Text("${f?.dayOfMonth ?: ""}", style = T.digitosChicos)
                        }
                        Text(e.titulo, style = T.dato.copy(color = C.papel), modifier = Modifier.weight(1f), maxLines = 2, overflow = TextOverflow.Ellipsis)
                        Text(if (e.todoElDia) "Todo el día" else f?.let { "%02d:%02d".format(it.hour, it.minute) } ?: "", style = T.dato, color = C.gris)
                    }
                }
            }
        }
    }
}

// --- Amigos: invitar, aceptar y mandarles toques ---

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ModAmigos(vm: RobotViewModel, modelo: com.sinteticalabs.app.consola.Modelo) {
    val api = vm.apiNube()
    if (api == null) { SoloNube(); return }
    val contexto = LocalContext.current
    val alcance = rememberCoroutineScope()
    var amigos by remember { mutableStateOf<List<Amigo>>(emptyList()) }
    var mios by remember { mutableStateOf<List<RobotNube>>(emptyList()) }
    var invitacion by remember { mutableStateOf<Invitacion?>(null) }
    var codigo by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var aviso by remember { mutableStateOf<String?>(null) }
    val carita = remember { mutableStateMapOf<Long, String>() }
    var mandando by remember { mutableStateOf("") }
    var seguro by remember { mutableStateOf(0L) }
    suspend fun cargar() { runCatching { amigos = api.amigos() }.onFailure { error = it.message } }
    fun decir(t: String) = alcance.launch { aviso = t; delay(2500); aviso = null }
    LaunchedEffect(Unit) {
        cargar()
        runCatching { mios = api.robots() }
        EnVivo.eventos.collect { if (it.tipo == "toque") cargar() }
    }
    val dibujar = Lab.paraDibujar(com.sinteticalabs.app.consola.VACIO)
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        error?.let { AvisoLinea(it, error = true) }
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Bandeja(Modifier.weight(1f)) {
                val inv = invitacion
                if (inv == null) {
                    Text("🤗", style = T.nombre.copy(fontSize = 30.sp))
                    Tecla("Invitar", { alcance.launch { runCatching { invitacion = api.invitar() }.onFailure { error = it.message } } }, Modifier.fillMaxWidth(), acabado = Acabado.NARANJA)
                } else {
                    val horas = inv.vence?.let { maxOf(0, Math.round((it - System.currentTimeMillis()) / 3.6e6).toInt()) } ?: 48
                    Grabado("Tu código · $horas h", color = SOBRE_PLACA)
                    Text("${inv.codigo.take(4)}-${inv.codigo.drop(4)}", style = T.digitosChicos.copy(fontSize = 26.sp))
                    Tecla("Compartir", {
                        runCatching { contexto.startActivity(Intent.createChooser(Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, inv.link), null).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
                    }, Modifier.fillMaxWidth(), acabado = Acabado.NARANJA, chica = true)
                    inv.telegram?.let { t -> Tecla("Telegram", { abrir(contexto, t) }, Modifier.fillMaxWidth(), acabado = Acabado.NEGRA, chica = true) }
                }
            }
            Visor(Modifier.weight(1f)) {
                Grabado("Código de un amigo")
                CampoVisor(codigo, { codigo = it }, placeholder = "ABCD-EFGH", maxBytes = 12, alEscribir = { codigo = it.uppercase() })
                Tecla("Aceptar", {
                    val c = Compania.limpiarCodigo(codigo)
                    alcance.launch {
                        runCatching { api.aceptarAmigo(c) }.onSuccess { codigo = ""; decir("¡Ya son amigos con ${it.nombre}!"); cargar() }
                            .onFailure { error = if (it is ErrorApi) (if (it.estado == 409) "Ya son amigos" else if (it.estado == 400) "Ese código no anda" else it.message) else "No pude." }
                    }
                }, acabado = Acabado.NARANJA, chica = true, habilitada = codigo.isNotBlank())
            }
        }
        aviso?.let { AvisoLinea(it) }
        if (amigos.isEmpty()) Bandeja { Grabado("Sin amigos todavía", color = SOBRE_PLACA) }
        amigos.forEach { a ->
            Bandeja {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Led(if (a.conectado) C.verde else null)
                    Text(a.nombre.uppercase(), style = T.nombre.copy(fontSize = 20.sp), color = C.tinta, modifier = Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Tecla(if (seguro == a.id) "¿Seguro?" else "✕", {
                        if (seguro != a.id) { seguro = a.id; alcance.launch { delay(4000); if (seguro == a.id) seguro = 0 } }
                        else alcance.launch { seguro = 0; runCatching { api.borrarAmigo(a.id) }; cargar() }
                    }, acabado = if (seguro == a.id) Acabado.NARANJA else Acabado.NEGRA, chica = true, descripcion = "Dejar de ser amigos")
                }
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Compania.TIPOS_TOQUE.forEach { (t, n) ->
                        Tecla(null, {
                            if (mandando.isEmpty()) alcance.launch {
                                mandando = "${a.id}:$t"
                                runCatching { api.tocar(a.id, t, if (t == "carita") carita[a.id] ?: "contento" else null, vm.idRobot.takeIf { vm.via == Via.NUBE }) }
                                    .onSuccess { (r, tg) -> decir(if (r > 0) "¡Le llegó!" else if (tg) "Le llegó por Telegram" else "Le llega cuando se conecte") }
                                    .onFailure { decir(if (it is ErrorApi && it.estado == 429) "Muchos toques: esperá un rato" else it.message ?: "No pude") }
                                delay(300); mandando = ""
                            }
                        }, Modifier.weight(1f), apretada = mandando == "${a.id}:$t", alto = 72.dp, descripcion = n) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Text(Compania.GLIFO_TOQUE[t] ?: "", style = T.nombre.copy(fontSize = 22.sp))
                                Text(n.uppercase(), style = T.modulo.copy(fontSize = 9.sp), color = C.tinta)
                            }
                        }
                    }
                }
                androidx.compose.foundation.lazy.LazyRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    items(Compania.CARITAS.size) { k ->
                        val cc = Compania.CARITAS[k]
                        val sel = (carita[a.id] ?: "contento") == cc
                        Box(Modifier.clip(RoundedCornerShape(10.dp)).then(if (sel) Modifier.border(2.dp, C.tinta, RoundedCornerShape(10.dp)) else Modifier)
                            .clickable(role = Role.RadioButton) { carita[a.id] = cc }.padding(3.dp)) {
                            MiniCara(cc, dibujar, com.sinteticalabs.app.protocolo.Pantalla(320, 172, false), 54.dp)
                        }
                    }
                }
                if (mios.isNotEmpty()) FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Grabado("Le llega a", color = SOBRE_PLACA)
                    mios.forEach { r ->
                        Llave(r.nombre.ifEmpty { r.id.takeLast(4) }, a.todos || r.id in a.robots, {
                            alcance.launch { runCatching { api.amigoRecibe(a.id, Compania.recibir(a.todos, a.robots, mios.map { it.id }, r.id)) }; cargar() }
                        })
                    }
                    if (mios.size > 1) Llave("Todos", a.todos, { alcance.launch { runCatching { api.amigoRecibe(a.id, Compania.recibir(a.todos, a.robots, mios.map { it.id }, null)) }; cargar() } })
                }
            }
        }
    }
}

// --- Avisos de afuera: una dirección secreta para GitHub, Mercado Pago o lo que quieras ---

@Composable
fun ModAvisos(vm: RobotViewModel) {
    val api = vm.apiNube()
    if (api == null) { SoloNube(); return }
    val contexto = LocalContext.current
    val alcance = rememberCoroutineScope()
    var e by remember { mutableStateOf<Entrantes?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var copiado by remember { mutableStateOf("") }
    var seguro by remember { mutableStateOf(false) }
    var mp by remember { mutableStateOf("") }
    var ultimo by remember { mutableStateOf<String?>(null) }
    val probado = remember { mutableStateMapOf<String, String>() }
    suspend fun cargar() { runCatching { e = api.entrantes() }.onFailure { error = it.message } }
    fun copiarlo(t: String?, que: String) { if (t == null) return; copiar(contexto, t); alcance.launch { copiado = que; delay(1800); copiado = "" } }
    LaunchedEffect(Unit) {
        cargar()
        EnVivo.eventos.collect { x -> if (x.tipo == "entrante") ultimo = x.datos["tipo"]?.let { (it as? kotlinx.serialization.json.JsonPrimitive)?.content } }
    }
    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        error?.let { AvisoLinea(it, error = true) }
        Bandeja {
            Grabado("Tu dirección secreta", color = SOBRE_PLACA)
            val x = e
            if (x?.url != null) Row(
                Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(C.hundido).clickable { copiarlo(x.url, "url") }.padding(12.dp),
                verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Text(x.url, style = T.dato.copy(fontSize = 11.sp, color = C.papel), modifier = Modifier.weight(1f), maxLines = 2, overflow = TextOverflow.Ellipsis)
                Grabado(if (copiado == "url") "Copiada" else "Copiar", color = C.brasa)
            } else if (x?.regenerar == true) AvisoLinea("Hay que hacer una nueva")
            Tecla(if (seguro) "¿Seguro? La vieja deja de andar" else "Regenerar", {
                if (!seguro) { seguro = true; alcance.launch { delay(4000); seguro = false } }
                else alcance.launch { seguro = false; runCatching { e = api.regenerarEntrantes() }.onFailure { error = it.message } }
            }, acabado = if (seguro) Acabado.NARANJA else Acabado.NEGRA, chica = true)
        }
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Bandeja(Modifier.weight(1f)) {
                Text("GitHub", style = T.nombre.copy(fontSize = 18.sp), color = C.tinta)
                Grabado("Webhooks · application/json", color = SOBRE_PLACA)
                Tecla(if (copiado == "token") "Copiado" else "Secret", { copiarlo(e?.token, "token") }, chica = true, habilitada = e?.token != null)
            }
            Bandeja(Modifier.weight(1f)) {
                Text("Mercado Pago", style = T.nombre.copy(fontSize = 18.sp), color = C.tinta, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Grabado("Webhooks · Pagos", color = SOBRE_PLACA)
                if (e?.mercadoPago == true) Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Led(C.verde); Grabado("Con monto", color = SOBRE_PLACA)
                    Tecla("Sacar", { alcance.launch { runCatching { api.mercadoPago(null) }; cargar() } }, acabado = Acabado.NEGRA, chica = true)
                } else {
                    Visor(Modifier.fillMaxWidth()) {
                        CampoVisor(mp, { mp = it }, placeholder = "Access token", estilo = T.dato.copy(color = C.papel), maxBytes = 200,
                            teclado = androidx.compose.ui.text.input.KeyboardType.Password, alEscribir = { mp = it })
                    }
                    Tecla("Guardar", { alcance.launch { runCatching { api.mercadoPago(mp.trim()); mp = "" }.onFailure { error = it.message }; cargar() } }, chica = true, habilitada = mp.isNotBlank())
                }
            }
        }
        Compania.TIPOS_ENTRANTE.chunked(3).forEach { fila ->
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                fila.forEach { (t, n) ->
                    Column(
                        Modifier.weight(1f).clip(RoundedCornerShape(16.dp)).background(C.hundido)
                            .then(if (ultimo == t) Modifier.border(2.dp, C.led, RoundedCornerShape(16.dp)) else Modifier).padding(10.dp),
                        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        val ej = Compania.EJEMPLO_ENTRANTE[t]!!
                        Box(Modifier.fillMaxWidth().height(40.dp).clip(RoundedCornerShape(10.dp)).background(Color.Black), contentAlignment = Alignment.Center) {
                            Text(if (t == "pago") "🤩" else "“${ej.first}”", style = T.dato.copy(fontSize = 11.sp, color = C.papel), maxLines = 1)
                        }
                        Grabado(n)
                        Tecla(probado[t] ?: "Probar", {
                            val tk = e?.token ?: return@Tecla
                            alcance.launch {
                                probado[t] = "…"
                                probado[t] = runCatching { if (api.probarEntrante(tk, t, ej.first, ej.second, vm.idRobot) > 0) "¡Llegó!" else "Desconectado" }.getOrElse { it.message ?: "No pude" }
                                delay(2500); probado.remove(t)
                            }
                        }, chica = true, habilitada = e?.token != null)
                    }
                }
                repeat(3 - fila.size) { Spacer(Modifier.weight(1f)) }
            }
        }
    }
}

// --- Su día: la presencia (los horarios que aprendió la nube y qué te dice al irte) ---

@Composable
fun SeccionPresencia(vm: RobotViewModel) {
    val c by vm.config.collectAsState()
    val api = vm.apiNube()
    var horarios by remember { mutableStateOf<Horarios?>(null) }
    LaunchedEffect(Unit) { if (vm.via == Via.NUBE && api != null) runCatching { horarios = api.horarios(vm.idRobot) } }
    val p = Compania.presencia(c)
    Bandeja {
        Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            listOf("Salís" to horarios?.salis, "Llegás" to horarios?.llegas, "Dormís" to horarios?.dormis).forEach { (t, h) ->
                Visor(Modifier.weight(1f)) {
                    Grabado(t)
                    Text(h ?: "--:--", style = T.digitosChicos.copy(fontSize = 18.sp, color = if (h == null) C.grisOscuro else C.brasa), maxLines = 1)
                }
            }
        }
        val segmentos = Math.round((horarios?.seguridad ?: 0.0) * 5).toInt()
        Row(Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(C.hundido).padding(10.dp), verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Grabado("Seguro")
            Canvas(Modifier.weight(1f).height(10.dp)) {
                val w = size.width / 5
                for (i in 0 until 5) drawRoundRect(if (i < segmentos) C.led else Color(0xFF2A2622), Offset(i * w + 2, 0f), Size(w - 4, size.height), CornerRadius(3f))
            }
            Grabado("${horarios?.dias ?: 0} d")
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Llave("Al irte", p.preguntar, { vm.cambiar { Compania.ponerPresencia(it) { x -> x.copy(preguntar = !x.preguntar) } } })
            Visor(Modifier.weight(1f)) {
                CampoVisor(p.texto, { t -> vm.cambiar { Compania.ponerPresencia(it) { x -> x.copy(texto = cortarBytes(t.trim(), Compania.MAX_TEXTO_PRESENCIA)) } } },
                    placeholder = "¿Llevás las llaves?", estilo = T.dato.copy(color = C.papel), maxBytes = Compania.MAX_TEXTO_PRESENCIA)
            }
        }
        DesdeEsteCelu(vm)
    }
}

// --- Integraciones: Spotify y la música del celu ---

@Composable
fun SeccionMusica(vm: RobotViewModel) {
    val api = vm.apiNube()
    val contexto = LocalContext.current
    val alcance = rememberCoroutineScope()
    var musica by remember { mutableStateOf<Musica?>(null) }
    suspend fun cargar() { musica = runCatching { api?.musica() }.getOrNull() }
    LaunchedEffect(Unit) {
        cargar()
        EnVivo.eventos.collect { e -> if (e.tipo == "musica" && e.datos["conectada"] != null) cargar() }
    }
    val suena by vm.suena.collectAsState()
    val m = musica
    if (m?.disponible == true) Bandeja {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Box(Modifier.size(48.dp).clip(CircleShape).background(Color(0xFF1ED760)), contentAlignment = Alignment.Center) { Text("♫", style = T.nombre.copy(fontSize = 22.sp), color = C.hundido) }
            Column(Modifier.weight(1f)) {
                Grabado("Spotify" + if (m.nombre.isNotEmpty()) " · ${m.nombre}" else "", color = SOBRE_PLACA)
                val s = suena ?: m.suena?.takeIf { it.sonando }
                Text(if (!m.conectada) "QUE BAILE TU MÚSICA" else s?.titulo ?: "CONECTADO", style = T.nombre.copy(fontSize = 16.sp), color = C.tinta, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (m.conectada && s != null) Text(s.artista, style = T.dato, color = SOBRE_PLACA, maxLines = 1)
            }
            if (m.conectada) Tecla("Desconectar", { alcance.launch { runCatching { api?.sacarSpotify() }; cargar() } }, acabado = Acabado.NEGRA, chica = true)
            else Tecla("Conectar", { alcance.launch { runCatching { api?.urlSpotify() }.getOrNull()?.takeIf { it.isNotEmpty() }?.let { abrir(contexto, it) } } }, acabado = Acabado.NARANJA, chica = true)
        }
    }
    MusicaDelCelu(vm)
}

// --- El héroe: "Decile algo" o "Charlar" (con IA, si la nube la tiene) y lo que suena ---

@Composable
fun DecileOCharla(vm: RobotViewModel, conectado: Boolean) {
    val api = vm.apiNube()
    val alcance = rememberCoroutineScope()
    var charla by remember { mutableStateOf<com.sinteticalabs.app.datos.Charla?>(null) }
    var charlando by remember { mutableStateOf(false) }
    var frase by remember { mutableStateOf("") }
    var respuesta by remember { mutableStateOf<Pair<String, String>?>(null) }
    var aviso by remember { mutableStateOf<String?>(null) }
    var diciendo by remember { mutableStateOf(false) }
    val suena by vm.suena.collectAsState()
    LaunchedEffect(Unit) {
        if (vm.via == Via.NUBE && api != null) {
            charla = runCatching { api.charla() }.getOrNull()?.takeIf { it.disponible }
            runCatching { api.musica() }.getOrNull()?.suena?.takeIf { it.sonando }?.let { if (vm.suena.value == null) vm.suena.value = it }
        }
    }
    val max = if (charlando) Compania.MAX_CHARLA else Acompanar.MAX_TEXTO
    suena?.let { s ->
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Text("♫", style = T.tecla, color = C.brasa)
            Text(s.titulo, style = T.dato.copy(color = C.papel), maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (s.artista.isNotEmpty()) Text(s.artista, style = T.dato, color = C.gris, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
    Visor(Modifier.fillMaxWidth()) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            val ch = charla
            if (ch != null) {
                Tecla("Decile algo", { charlando = false; frase = cortarBytes(frase, Acompanar.MAX_TEXTO) }, acabado = if (!charlando) Acabado.NARANJA else Acabado.NEGRA, chica = true, alto = 32.dp)
                Tecla("Charlar", { charlando = true }, acabado = if (charlando) Acabado.NARANJA else Acabado.NEGRA, chica = true, alto = 32.dp)
                Spacer(Modifier.weight(1f))
                if (charlando) Grabado("${ch.quedan}/${ch.porDia} hoy") else Grabado("${largoBytes(frase)}/$max")
            } else {
                Grabado("Decile algo", Modifier.weight(1f))
                Grabado("${largoBytes(frase)}/$max", color = if (largoBytes(frase) >= max) C.brasa else C.grisOscuro)
            }
        }
        if (charlando) respuesta?.let { (yo, el) ->
            Text(yo, style = T.dato, color = C.gris, maxLines = 2)
            Text("“$el”", style = T.campoClaro.copy(color = C.brasa))
        }
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Box(Modifier.weight(1f)) {
                CampoVisor(frase, { frase = it }, placeholder = if (charlando) "¿Cómo andás?" else if (conectado) "¡Hola!" else "Desconectado", estilo = T.campoClaro,
                    maxBytes = max, alEscribir = { frase = it })
            }
            Tecla(if (diciendo) "…" else if (charlando) "Charlar" else "Decir", {
                val t = frase.trim()
                alcance.launch {
                    diciendo = true
                    if (charlando) {
                        runCatching { api!!.charlar(vm.idRobot, t) }
                            .onSuccess { r -> respuesta = t to r.texto; charla = charla?.copy(quedan = r.quedan); frase = "" }
                            .onFailure { x ->
                                aviso = if (x is ErrorApi) (if (x.estado == 429) "Por hoy ya charlaron mucho" else if (x.estado == 503) "Sin charla en esta nube" else x.message) else "No contestó"
                            }
                    } else {
                        val e = vm.decir(t)
                        if (e == null) frase = ""
                        aviso = e ?: "Lo dijo"
                    }
                    diciendo = false
                    delay(2500); aviso = null
                }
            }, acabado = Acabado.NARANJA, chica = true,
                habilitada = (conectado || charlando) && !diciendo && frase.isNotBlank() && (!charlando || (charla?.quedan ?: 0) > 0))
        }
        aviso?.let { Grabado(it, color = C.brasa) }
    }
}

/** La clave de este robot para lo que manda el celu: su id (nube) o "cerca:<id>" (BLE). */
private fun claveCelu(vm: RobotViewModel) = if (vm.via == Via.NUBE) vm.idRobot else com.sinteticalabs.app.datos.AvisosCelu.claveCerca(vm.idRobot)

/**
 * La presencia desde este celu: el sistema avisa cuando el robot está cerca
 * por Bluetooth (CompanionDeviceManager, casi sin batería) y, de respaldo, una
 * geocerca de casa. Apagado por defecto; cada permiso se pide con su porqué.
 */
@Composable
fun DesdeEsteCelu(vm: RobotViewModel) {
    if (vm.via == Via.DEMO) return
    val contexto = LocalContext.current
    val alcance = rememberCoroutineScope()
    val clave = claveCelu(vm)
    var bluetooth by remember { mutableStateOf(com.sinteticalabs.app.datos.Celu.asociadoA(contexto, clave)) }
    var geocerca by remember { mutableStateOf(com.sinteticalabs.app.datos.Celu.conGeocerca(contexto) && com.sinteticalabs.app.datos.Celu.quiere(contexto, "presencia", clave)) }
    var explicando by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    val dialogo = androidx.activity.compose.rememberLauncherForActivityResult(androidx.activity.result.contract.ActivityResultContracts.StartIntentSenderForResult()) { r ->
        if (r.resultCode == android.app.Activity.RESULT_OK) { com.sinteticalabs.app.datos.Celu.asociado(contexto); bluetooth = true }
    }
    val fondo = androidx.activity.compose.rememberLauncherForActivityResult(androidx.activity.result.contract.ActivityResultContracts.RequestPermission()) { si ->
        if (si) alcance.launch {
            error = com.sinteticalabs.app.datos.Celu.ponerGeocerca(contexto)
            if (error == null) { com.sinteticalabs.app.datos.Celu.poner(contexto, "presencia", clave, true); geocerca = true }
        } else error = "Sin ubicación en segundo plano"
    }
    val ubicacion = androidx.activity.compose.rememberLauncherForActivityResult(androidx.activity.result.contract.ActivityResultContracts.RequestPermission()) { si ->
        if (si) fondo.launch(android.Manifest.permission.ACCESS_BACKGROUND_LOCATION) else error = "Sin ubicación"
    }
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Grabado("Este celu", color = SOBRE_PLACA)
        Llave("Bluetooth", bluetooth, {
            if (bluetooth) { com.sinteticalabs.app.datos.Celu.desasociar(contexto, clave); bluetooth = false }
            else explicando = "bluetooth"
        }, habilitada = com.sinteticalabs.app.datos.Celu.disponibleBluetooth())
        Llave("Casa", geocerca, {
            if (geocerca) { com.sinteticalabs.app.datos.Celu.sacarGeocerca(contexto); geocerca = false }
            else explicando = "casa"
        })
    }
    error?.let { AvisoLinea(it, error = true) }
    explicando?.let { que ->
        androidx.compose.ui.window.Dialog(onDismissRequest = { explicando = null }) {
            Chapa(Modifier.fillMaxWidth(), radio = 28.dp, relleno = 22.dp) {
                Grabado(if (que == "bluetooth") "Llegás y te vas" else "Tu casa", color = C.brasa)
                if (que == "bluetooth") {
                    Text("Cuando el celu lo ve cerca, llegaste.", style = T.texto.copy(color = C.papel))
                    Text("Lo avisa el sistema: casi no gasta batería.", style = T.texto)
                } else {
                    Text("Tu casa es donde estás ahora.", style = T.texto.copy(color = C.papel))
                    Text("Pide la ubicación también en segundo plano.", style = T.texto)
                }
                Tecla("Dale", {
                    explicando = null
                    error = null
                    if (que == "bluetooth") com.sinteticalabs.app.datos.Celu.asociar(contexto, clave, vm.idRobot,
                        { s -> dialogo.launch(androidx.activity.result.IntentSenderRequest.Builder(s).build()) }, { error = it })
                    else ubicacion.launch(android.Manifest.permission.ACCESS_FINE_LOCATION)
                }, Modifier.fillMaxWidth(), acabado = Acabado.NARANJA)
                Tecla("Ahora no", { explicando = null }, Modifier.fillMaxWidth(), acabado = Acabado.NEGRA)
            }
        }
    }
}

/** La música del celu (la sesión de medios, con el mismo permiso que los avisos). Apagada por defecto. */
@Composable
fun MusicaDelCelu(vm: RobotViewModel) {
    if (vm.via == Via.DEMO) return
    val contexto = LocalContext.current
    val clave = claveCelu(vm)
    var prendida by remember { mutableStateOf(com.sinteticalabs.app.datos.Celu.quiere(contexto, "musica", clave)) }
    var permiso by remember { mutableStateOf(com.sinteticalabs.app.datos.AvisosCelu.permiso(contexto)) }
    val dueno = androidx.compose.ui.platform.LocalLifecycleOwner.current
    androidx.compose.runtime.DisposableEffect(dueno) {
        val o = androidx.lifecycle.LifecycleEventObserver { _, e -> if (e == androidx.lifecycle.Lifecycle.Event.ON_RESUME) permiso = com.sinteticalabs.app.datos.AvisosCelu.permiso(contexto) }
        dueno.lifecycle.addObserver(o)
        onDispose { dueno.lifecycle.removeObserver(o) }
    }
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Llave("Música del celu", prendida && permiso, {
            prendida = !prendida
            com.sinteticalabs.app.datos.Celu.poner(contexto, "musica", clave, prendida)
            if (prendida && !permiso) runCatching {
                contexto.startActivity(Intent(android.provider.Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            }
        }, Modifier.weight(1f))
        if (prendida && !permiso) Grabado("Sin permiso", color = SOBRE_PLACA)
    }
}
