package com.sinteticalabs.app.ui.consola

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.consola.Automatizacion
import com.sinteticalabs.app.consola.Automatizaciones
import com.sinteticalabs.app.consola.Ejecucion
import com.sinteticalabs.app.consola.hace
import com.sinteticalabs.app.datos.ClienteMcp
import com.sinteticalabs.app.datos.EnVivo
import com.sinteticalabs.app.datos.ErrorApi
import com.sinteticalabs.app.datos.EstadoTelegram
import com.sinteticalabs.app.datos.RobotNube
import com.sinteticalabs.app.datos.TokenMcp
import com.sinteticalabs.app.datos.automatizaciones
import com.sinteticalabs.app.datos.borrarTokenMcp
import com.sinteticalabs.app.datos.clientesMcp
import com.sinteticalabs.app.datos.crearTokenMcp
import com.sinteticalabs.app.datos.desconectarMcp
import com.sinteticalabs.app.datos.desvincularTelegram
import com.sinteticalabs.app.datos.fechaMs
import com.sinteticalabs.app.datos.guardarAutomatizaciones
import com.sinteticalabs.app.datos.probarAutomatizacion
import com.sinteticalabs.app.datos.robots
import com.sinteticalabs.app.datos.secretoWebhooks
import com.sinteticalabs.app.datos.telegram
import com.sinteticalabs.app.datos.tokensMcp
import com.sinteticalabs.app.datos.vincularTelegram
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private fun copiar(contexto: Context, texto: String) {
    (contexto.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager).setPrimaryClip(ClipData.newPlainText("Sintética", texto))
}

private fun textoError(e: Throwable, por: String) = if (e is ErrorApi) e.message + if (e.detalle.isNotEmpty()) ": ${e.detalle}" else "" else por

/**
 * Integraciones: el robot conectado al mundo desde la nube. Tu IA (MCP),
 * Telegram (le escribís y te avisa) y automatizaciones: cuando pasa algo,
 * hace cosas (web: modulos/Integraciones.svelte).
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ModIntegraciones(vm: RobotViewModel) {
    val api = vm.apiNube()
    if (vm.via != Via.NUBE || api == null) {
        Bandeja {
            Text("EN LA NUBE", style = T.nombre.copy(fontSize = 22.sp), color = C.tinta)
            Text("Sumalo a tu cuenta (Ajustes).", style = T.texto, color = Color(0xCC1A1714))
        }
        return
    }
    val contexto = LocalContext.current
    val alcance = rememberCoroutineScope()
    val id = vm.idRobot
    var telegram by remember { mutableStateOf<EstadoTelegram?>(null) }
    var enlace by remember { mutableStateOf<String?>(null) }
    var autos by remember { mutableStateOf<List<Automatizacion>>(emptyList()) }
    var historial by remember { mutableStateOf<List<Ejecucion>>(emptyList()) }
    var cuandos by remember { mutableStateOf(Automatizaciones.CUANDOS_DE_FABRICA) }
    var otros by remember { mutableStateOf<List<RobotNube>>(emptyList()) }
    var clientes by remember { mutableStateOf<List<ClienteMcp>>(emptyList()) }
    var tokens by remember { mutableStateOf<List<TokenMcp>>(emptyList()) }
    var url by remember { mutableStateOf("") }
    var tokenNuevo by remember { mutableStateOf("") }
    var copiado by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var sinNube by remember { mutableStateOf(false) }
    var guardando by remember { mutableStateOf("") }
    val probado = remember { mutableStateMapOf<Int, String>() }
    var reloj by remember { mutableStateOf<Job?>(null) }
    var secreto by remember { mutableStateOf<String?>(null) }
    var rotarSeguro by remember { mutableStateOf(false) }

    suspend fun cargar() {
        try {
            telegram = api.telegram()
            val r = api.automatizaciones(id)
            autos = r.lista
            historial = r.historial
            cuandos = r.cuandos
            sinNube = false
        } catch (e: ErrorApi) {
            if (e.estado == 404) sinNube = true else error = textoError(e, "No pude cargarlas.")
        } catch (e: Exception) {
            error = e.message
        }
        runCatching { otros = api.robots().filter { it.id != id } }
    }
    suspend fun cargarIA() {
        runCatching {
            val (l, u) = api.clientesMcp()
            clientes = l
            url = u
            tokens = api.tokensMcp()
        }
    }
    fun guardar(nuevas: List<Automatizacion>) {
        autos = nuevas
        guardando = "Guardando…"
        reloj?.cancel()
        reloj = alcance.launch {
            delay(700)
            try {
                api.guardarAutomatizaciones(id, autos)
                guardando = "Guardadas"
                error = null
            } catch (e: Exception) {
                guardando = ""
                error = textoError(e, "No pude guardar.")
            }
        }
    }
    fun cambiar(i: Int, f: (Automatizacion) -> Automatizacion) = guardar(autos.mapIndexed { k, a -> if (k == i) f(a) else a })
    fun aviso(que: String) = alcance.launch { copiado = que; delay(1800); copiado = "" }

    LaunchedEffect(id) {
        cargar()
        cargarIA()
        EnVivo.eventos.collect { e ->
            if (e.tipo == "telegram" || (e.tipo == "automatizacion" && e.robot == id)) cargar()
            if (e.tipo == "mcp") cargarIA()
        }
    }
    // Hasta que llegue el evento, se fija cada tanto si ya se vinculó.
    LaunchedEffect(enlace) {
        if (enlace == null) return@LaunchedEffect
        repeat(300) {
            delay(3000)
            runCatching { telegram = api.telegram() }
            if (telegram?.vinculado == true) { enlace = null; return@LaunchedEffect }
        }
    }

    val nombreDe = { rid: String -> otros.firstOrNull { it.id == rid }?.nombre?.ifEmpty { rid } ?: rid }

    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
        error?.let { AvisoLinea(it, error = true) }
        if (sinNube) {
            AvisoLinea("Todavía no están en esta nube.")
            return@Column
        }

        // Tu IA: Claude o ChatGPT, por MCP.
        Bandeja {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Logo("✦")
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Grabado("Tu IA", color = C.sobrePlaca)
                    Text(if (clientes.isEmpty()) "CONECTALA" else "${clientes.size} CONECTADA${if (clientes.size > 1) "S" else ""}", style = T.nombre.copy(fontSize = 20.sp), color = C.tinta)
                }
                Tecla("+ Token", { alcance.launch {
                    runCatching { tokenNuevo = api.crearTokenMcp("Claude Code"); cargarIA() }.onFailure { error = textoError(it, "No pude crearlo.") }
                } }, acabado = Acabado.NEGRA, chica = true)
            }
            if (url.isNotEmpty()) Fila(url, if (copiado == "url") "Copiada" else "Copiar") { copiar(contexto, url); aviso("url") }
            if (tokenNuevo.isNotEmpty()) {
                val cmd = "claude mcp add --transport http sintetica $url --header \"Authorization: Bearer $tokenNuevo\""
                Grabado("Se ve una sola vez", color = C.sobrePlaca)
                Fila(cmd, if (copiado == "cmd") "Copiado" else "Copiar") { copiar(contexto, cmd); aviso("cmd") }
            }
            clientes.forEach { c ->
                Conectada(c.cliente, c.usada, true) { alcance.launch { runCatching { api.desconectarMcp(c.id) }; cargarIA() } }
            }
            tokens.forEach { t ->
                Conectada(t.nombre, t.usado, false) { alcance.launch { runCatching { api.borrarTokenMcp(t.id) }; cargarIA() } }
            }
        }

        // Telegram: te avisa y le escribís.
        Bandeja {
            val t = telegram
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Logo("➤")
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Grabado("Telegram" + (t?.bot?.takeIf { it.isNotEmpty() }?.let { " · @$it" } ?: ""), color = C.sobrePlaca)
                    Text(
                        when {
                            t == null -> "…"
                            !t.activo -> "APAGADO"
                            t.vinculado -> "VINCULADO"
                            else -> "SIN VINCULAR"
                        },
                        style = T.nombre.copy(fontSize = 20.sp), color = C.tinta,
                    )
                    if (t?.vinculado == true && t.nombre.isNotEmpty()) Text(t.nombre, style = T.texto, color = Color(0xCC1A1714))
                }
            }
            if (t?.activo == true) {
                when {
                    t.vinculado -> Tecla("Desvincular", { alcance.launch { runCatching { api.desvincularTelegram() }; cargar() } }, Modifier.fillMaxWidth(), acabado = Acabado.NEGRA, chica = true)
                    enlace != null -> Tecla("Abrir Telegram", { abrir(contexto, enlace!!) }, Modifier.fillMaxWidth(), acabado = Acabado.NARANJA)
                    else -> Tecla("Vincular", {
                        alcance.launch {
                            runCatching { api.vincularTelegram() }.onSuccess { l -> enlace = l; l?.let { abrir(contexto, it) } }.onFailure { error = textoError(it, "No pude.") }
                        }
                    }, Modifier.fillMaxWidth(), acabado = Acabado.NARANJA)
                }
            }
        }

        // Automatizaciones: cuando pasa algo → hace cosas.
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.SpaceBetween) {
            Grabado("Cuando… → hace" + if (guardando.isNotEmpty()) " · ${guardando.lowercase()}" else "", color = C.sobrePlaca)
            Tecla("+ Nueva", { guardar(autos + Automatizaciones.nueva(System.currentTimeMillis())) }, acabado = Acabado.NEGRA, chica = true,
                habilitada = autos.size < Automatizaciones.MAXIMO)
        }
        if (autos.isEmpty()) Bandeja { Text("Recordatorio → Telegram", style = T.texto, color = Color(0xCC1A1714)) }
        autos.forEachIndexed { i, a ->
            Bandeja {
                Visor(Modifier.fillMaxWidth()) {
                    Grabado("Cuando")
                    Selector(Automatizaciones.nombreCuando(a.cuando), Automatizaciones.cuandos(cuandos).map { it to Automatizaciones.nombreCuando(it) },
                        { k -> cambiar(i) { Automatizaciones.conCuando(it, k) } })
                    if (a.cuando == "expresion") Selector(a.valor ?: "cualquiera", listOf("" to "cualquiera") + Automatizaciones.CARAS.map { it to it },
                        { k -> cambiar(i) { it.copy(valor = k.ifEmpty { null }) } }, estilo = T.dato.copy(color = C.brasa))
                    if (a.cuando == "hora") {
                        val h = a.hora ?: "08:00"
                        Text(h, style = T.digitosChicos, modifier = Modifier.clickable {
                            android.app.TimePickerDialog(contexto, { _, hh, mm -> cambiar(i) { it.copy(hora = "%02d:%02d".format(hh, mm)) } },
                                com.sinteticalabs.app.consola.Editar.aMin(h) / 60, com.sinteticalabs.app.consola.Editar.aMin(h) % 60, true).show()
                        })
                        FilaDias(a.dias ?: "LMXJVSD") { l -> cambiar(i) { Automatizaciones.alternarDia(it, l) } }
                    }
                }
                a.hacer.forEachIndexed { j, h ->
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Text("→", style = T.nombre.copy(fontSize = 20.sp), color = C.tinta)
                        Visor(Modifier.weight(1f)) {
                            when (h.tipo) {
                                "telegram" -> {
                                    Grabado("Telegram")
                                    CampoVisor(h.texto.orEmpty(), { v -> cambiar(i) { x -> x.copy(hacer = x.hacer.mapIndexed { k, y -> if (k == j) y.copy(texto = v) else y }) } },
                                        estilo = T.dato.copy(color = C.papel), maxBytes = 300)
                                }
                                "decir" -> {
                                    Grabado("Dice")
                                    CampoVisor(h.texto.orEmpty(), { v ->
                                        val t = com.sinteticalabs.app.consola.cortarBytes(v.trim(), 60).ifEmpty { "¡Hola!" }
                                        cambiar(i) { x -> x.copy(hacer = x.hacer.mapIndexed { k, y -> if (k == j) y.copy(texto = t) else y }) }
                                    }, estilo = T.dato.copy(color = C.papel), maxBytes = 60)
                                    Selector(h.expresion ?: "su cara", listOf("" to "su cara") + Automatizaciones.CARAS.map { it to it },
                                        { v -> cambiar(i) { x -> x.copy(hacer = x.hacer.mapIndexed { k, y -> if (k == j) y.copy(expresion = v.ifEmpty { null }) else y }) } },
                                        estilo = T.dato.copy(color = C.brasa))
                                }
                                "cara" -> {
                                    Grabado("Pone la cara")
                                    Selector(h.expresion ?: "contento", Automatizaciones.CARAS.map { it to it },
                                        { v -> cambiar(i) { x -> x.copy(hacer = x.hacer.mapIndexed { k, y -> if (k == j) y.copy(expresion = v) else y }) } },
                                        estilo = T.dato.copy(color = C.papel))
                                }
                                "modo" -> {
                                    Grabado("Pasa a")
                                    Selector(Automatizaciones.MODOS.firstOrNull { it.first == (h.modo ?: "reloj") }?.second ?: "Reloj", Automatizaciones.MODOS,
                                        { v -> cambiar(i) { x -> x.copy(hacer = x.hacer.mapIndexed { k, y -> if (k == j) y.copy(modo = v) else y }) } },
                                        estilo = T.dato.copy(color = C.papel))
                                }
                                "despertar" -> {
                                    Grabado("Lo despierta")
                                    Text("¡Buen día!", style = T.dato.copy(color = C.papel))
                                }
                                "jugar" -> {
                                    Grabado("Te invita a")
                                    Selector(Automatizaciones.JUEGOS.firstOrNull { it.first == (h.juego ?: "dado") }?.second ?: "Dado", Automatizaciones.JUEGOS,
                                        { v -> cambiar(i) { x -> x.copy(hacer = x.hacer.mapIndexed { k, y -> if (k == j) y.copy(juego = v) else y }) } },
                                        estilo = T.dato.copy(color = C.papel))
                                }
                                "webhook" -> {
                                    Grabado("URL")
                                    CampoVisor(h.url.orEmpty(), { v -> cambiar(i) { x -> x.copy(hacer = x.hacer.mapIndexed { k, y -> if (k == j) y.copy(url = v.trim()) else y }) } },
                                        placeholder = "https://", estilo = T.dato.copy(color = C.papel), maxBytes = 500, teclado = androidx.compose.ui.text.input.KeyboardType.Uri)
                                }
                                else -> {
                                    Grabado("Otro robot")
                                    Selector(h.robot?.let(nombreDe) ?: "—", otros.map { it.id to nombreDe(it.id) },
                                        { v -> cambiar(i) { x -> x.copy(hacer = x.hacer.mapIndexed { k, y -> if (k == j) y.copy(robot = v) else y }) } }, estilo = T.dato.copy(color = C.papel))
                                    Selector(h.expresion ?: "contento", Automatizaciones.CARAS.map { it to it },
                                        { v -> cambiar(i) { x -> x.copy(hacer = x.hacer.mapIndexed { k, y -> if (k == j) y.copy(expresion = v) else y }) } }, estilo = T.dato.copy(color = C.brasa))
                                }
                            }
                        }
                        Tecla("✕", { cambiar(i) { Automatizaciones.quitarAccion(it, j) } }, acabado = Acabado.NEGRA, cuadrada = true, habilitada = a.hacer.size > 1, descripcion = "Quitar")
                    }
                }
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    Automatizaciones.ACCIONES.forEach { (k, n) ->
                        Tecla("+ $n", { cambiar(i) { Automatizaciones.sumarAccion(it, k, otros.firstOrNull()?.id) } }, chica = true,
                            habilitada = a.hacer.size < Automatizaciones.MAX_ACCIONES && (k != "robot" || otros.isNotEmpty()))
                    }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                    Llave(if (a.activa) "Activa" else "Apagada", a.activa, { cambiar(i) { it.copy(activa = !it.activa) } })
                    Tecla(probado[i] ?: "Probar", {
                        alcance.launch {
                            probado[i] = "…"
                            probado[i] = runCatching { api.probarAutomatizacion(id, i); "Listo" }.getOrElse { textoError(it, "Falló") }
                            delay(4000)
                            probado.remove(i)
                        }
                    }, chica = true)
                    Tecla("✕", { guardar(autos.filterIndexed { k, _ -> k != i }) }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Borrar")
                }
            }
        }
        Text(Automatizaciones.PLANTILLAS.joinToString(" "), style = T.dato.copy(fontSize = 12.sp), color = Color(0xB31A1714))

        // El secreto de los webhooks: cada llamada va firmada (X-Sintetica-Firma: sha256=…, HMAC del cuerpo).
        Bandeja {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Grabado("Webhooks · firma", color = C.sobrePlaca, modifier = Modifier.weight(1f))
                Tecla(if (secreto == null) "Ver" else "Ocultar", {
                    if (secreto != null) secreto = null
                    else alcance.launch { runCatching { secreto = api.secretoWebhooks(false) }.onFailure { error = textoError(it, "No pude traerlo.") } }
                }, chica = true)
                Tecla(if (rotarSeguro) "¿Seguro?" else "Rotar", {
                    if (!rotarSeguro) { rotarSeguro = true; alcance.launch { delay(5000); rotarSeguro = false } }
                    else {
                        rotarSeguro = false
                        alcance.launch { runCatching { secreto = api.secretoWebhooks(true) }.onFailure { error = textoError(it, "No pude rotarlo.") } }
                    }
                }, acabado = if (rotarSeguro) Acabado.NARANJA else Acabado.NEGRA, chica = true)
            }
            Fila(secreto ?: "••••••••••••••••••••••••", if (secreto == null) "" else if (copiado == "secreto") "Copiado" else "Copiar") {
                secreto?.let { copiar(contexto, it); aviso("secreto") }
            }
        }
        if (historial.isNotEmpty()) Column(Modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp)).background(C.hundido).padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Grabado("Últimas")
            historial.take(6).forEach { h ->
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Led(if (h.ok) C.verde else C.led, tamano = 7.dp)
                    Text(hace(fechaMs(h.en)), style = T.dato.copy(fontSize = 12.sp), color = C.gris)
                    Text("${Automatizaciones.nombreCuando(h.evento)} → ${h.accion}", style = T.dato.copy(fontSize = 12.sp), color = C.crema, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
        }
    }
}

private fun abrir(contexto: Context, enlace: String) {
    runCatching { contexto.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(enlace)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
}

@Composable
private fun Logo(glifo: String) {
    Box(Modifier.size(48.dp).clip(RoundedCornerShape(14.dp)).background(C.tinta), contentAlignment = Alignment.Center) {
        Text(glifo, style = T.nombre.copy(fontSize = 22.sp), color = C.brasa)
    }
}

/** Un texto para copiar (la URL del conector, el comando). */
@Composable
private fun Fila(texto: String, accion: String, alTocar: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(C.hundido).clickable(onClick = alTocar).padding(horizontal = 12.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Text(texto, style = T.dato.copy(fontSize = 12.sp, color = C.papel), modifier = Modifier.weight(1f), maxLines = 2, overflow = TextOverflow.Ellipsis)
        Grabado(accion, color = C.brasa)
    }
}

@Composable
private fun Conectada(nombre: String, usada: Long?, oauth: Boolean, alQuitar: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clip(RoundedCornerShape(12.dp)).background(C.hundido).padding(start = 12.dp, end = 6.dp, top = 6.dp, bottom = 6.dp),
        verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Led(if (oauth) C.verde else C.led, tamano = 7.dp)
        Text(nombre, style = T.dato.copy(color = C.papel), modifier = Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(if (usada != null) hace(usada) else "sin usar", style = T.dato.copy(fontSize = 12.sp), color = C.gris)
        Tecla("✕", alQuitar, acabado = Acabado.NEGRA, cuadrada = true, descripcion = if (oauth) "Desconectar" else "Borrar")
    }
}
