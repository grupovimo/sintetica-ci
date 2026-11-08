package com.sinteticalabs.app.ui.consola

import android.graphics.Bitmap
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.FilterQuality
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.consola.Lab
import com.sinteticalabs.app.consola.Modelo
import com.sinteticalabs.app.consola.numero
import com.sinteticalabs.app.consola.obj
import com.sinteticalabs.app.consola.si
import com.sinteticalabs.app.consola.texto
import com.sinteticalabs.app.nucleo.Nucleo
import com.sinteticalabs.app.protocolo.Car
import com.sinteticalabs.app.protocolo.Comando
import com.sinteticalabs.app.protocolo.escriturasDeConfig
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/**
 * Fotos de caras con la instancia (también los dibujos con formas): fija cada
 * una, adelanta el reloj hasta que termina el fundido y saca el cuadro.
 * Después hay que volver a pasarle su config.
 */
private fun fotografiar(config: String, nombres: List<String>): Map<String, ImageBitmap> {
    for (e in escriturasDeConfig(config, 512)) Nucleo.bleEscribir(Car.CONFIG.id, e)
    val w = Nucleo.ancho()
    val h = Nucleo.alto()
    val px = IntArray(w * h)
    val fotos = mutableMapOf<String, ImageBitmap>()
    for (n in nombres) {
        Nucleo.bleEscribir(Car.COMANDO.id, Comando.expresion(n))
        repeat(6) { Nucleo.adelantar(200); Nucleo.tick() }
        Nucleo.pixeles(px)
        fotos[n] = Bitmap.createBitmap(px, w, h, Bitmap.Config.ARGB_8888).asImageBitmap()
    }
    Nucleo.bleEscribir(Car.COMANDO.id, Comando.expresion(""))
    return fotos
}

/**
 * Dibujos: las caras hechas con formas (las 16 del manual y las tuyas). Se
 * ven en vivo, se prueban en el robot, se copian y se borran (o vuelven a la
 * del manual). Como iOS, sin el editor de formas.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun LabDibujos(vm: RobotViewModel, modelo: Modelo) {
    val config by vm.config.collectAsState()
    val robot by vm.robot.collectAsState()
    val llave by vm.vistaPrevia.collectAsState()
    val alcance = rememberCoroutineScope()
    var fotos by remember { mutableStateOf<Map<String, ImageBitmap>>(emptyMap()) }
    var elegido by remember { mutableStateOf("contento") }
    var error by remember { mutableStateOf<String?>(null) }
    var aviso by remember { mutableStateOf<String?>(null) }
    val propios = Lab.propiosDibujos(config)
    val base = VidaBase.dibujos.map { it.first }
    val nombres = base + propios.keys.filter { it !in base }.sorted()
    fun docDe(n: String): JsonObject? = propios[n] ?: VidaBase.dibujos.firstOrNull { it.first == n }?.second
    val delManual = elegido in base

    fun ver(n: String) {
        elegido = n
        vm.pausarEspejo(30.0)
        var c = config
        val d = docDe(n)
        if (n !in propios && d != null) c = Lab.ponerExpresion(c, n, d)
        vm.aplicarEnVistaPrevia(c)
        if (vm.via != Via.DEMO) Nucleo.bleEscribir(Car.COMANDO.id, Comando.expresion(n))
    }

    fun guardar(c: JsonObject, listo: String) = alcance.launch {
        error = vm.guardarAhora(c)
        if (error == null) { aviso = listo; delay(2500); aviso = null }
    }

    LaunchedEffect(config.toString(), llave) {
        if (llave == 0 || vm.via == Via.DEMO) return@LaunchedEffect
        var c = buildJsonObject { put("version", 2) }
        config.obj("cara")?.let { c = JsonObject(c + ("cara" to it)) }
        val e = nombres.take(24).mapNotNull { n -> docDe(n)?.let { n to it } }.toMap()
        c = JsonObject(c + ("expresiones" to JsonObject(e)))
        fotos = fotografiar(c.toString(), nombres.take(24))
        vm.aplicarEnVistaPrevia(config)
        ver(elegido)
    }

    MarcoPantalla(modelo.pantalla) { if (llave > 0) PantallaViva(llave, "Vista previa") }
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(elegido.uppercase(), style = T.nombre.copy(fontSize = 22.sp), maxLines = 1, overflow = TextOverflow.Ellipsis)
        if (elegido in propios) Led(C.led, tamano = 7.dp)
    }
    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Tecla("▶ En el robot", {
            val n = elegido
            alcance.launch { error = vm.comando(expresion = n); delay(4000); vm.comando(expresion = "") }
        }, acabado = Acabado.NEGRA, chica = true, habilitada = robot?.conectado == true)
        Tecla("Copiar", {
            val d = docDe(elegido)
            if (d != null) {
                val n = Lab.libre("mi-$elegido", propios.keys + VidaBase.expresiones.keys)
                guardar(Lab.ponerExpresion(config, n, d), "«$n» está en el robot.")
                elegido = n
            }
        }, chica = true)
        if (elegido in propios) Tecla(if (delManual) "Original" else "Borrar", {
            val n = elegido
            guardar(Lab.ponerExpresion(config, n, null), if (delManual) "Volvió a la del manual." else "Se borró.")
            if (!delManual) elegido = "contento"
        }, acabado = Acabado.NEGRA, chica = true)
    }
    aviso?.let { AvisoLinea(it) }
    error?.let { AvisoLinea(it, error = true) }
    PlacaNaranja(Modifier.fillMaxWidth(), relleno = 14.dp) {
        nombres.chunked(3).forEach { fila ->
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                fila.forEach { n ->
                    val sel = n == elegido
                    Column(
                        Modifier.weight(1f).clip(RoundedCornerShape(12.dp)).background(if (sel) C.tinta else Color(0x1F1A1714)).clickable { ver(n) }.padding(6.dp),
                        horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        Box(
                            Modifier.fillMaxWidth().aspectRatio(if (modelo.redonda) 1f else modelo.pantalla.ancho.toFloat() / modelo.pantalla.alto)
                                .clip(if (modelo.redonda) CircleShape else RoundedCornerShape(8.dp)).background(Color.Black),
                        ) {
                            fotos[n]?.let { Image(it, contentDescription = n, contentScale = ContentScale.Crop, filterQuality = FilterQuality.Low, modifier = Modifier.fillMaxSize()) }
                        }
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                            Text(n, style = T.dato.copy(fontSize = 10.sp), color = if (sel) C.papel else C.tinta, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            if (n in propios) Led(C.led, tamano = 6.dp)
                        }
                    }
                }
                repeat(3 - fila.size) { Spacer(Modifier.weight(1f)) }
            }
        }
    }
}

/**
 * Animaciones: las tuyas y las de ejemplo. Se ven acá, se reproducen en el
 * robot, se suman las de ejemplo y se borran. Como iOS, sin el editor de pistas.
 */
@Composable
fun LabAnimaciones(vm: RobotViewModel, modelo: Modelo) {
    val config by vm.config.collectAsState()
    val robot by vm.robot.collectAsState()
    val llave by vm.vistaPrevia.collectAsState()
    val alcance = rememberCoroutineScope()
    var error by remember { mutableStateOf<String?>(null) }
    var aviso by remember { mutableStateOf<String?>(null) }
    val propias = config.obj("animaciones").orEmpty().mapNotNull { (k, v) -> (v as? JsonObject)?.let { k to it } }.toMap()
    val dibujar = Lab.paraDibujar(config)

    fun guardar(c: JsonObject, listo: String) = alcance.launch {
        error = vm.guardarAhora(c)
        if (error == null) { aviso = listo; delay(2500); aviso = null }
    }

    @Composable
    fun Fila(n: String, doc: JsonObject, propia: Boolean) {
        Bandeja {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                MiniCara(doc.texto("sobre") ?: "neutral", dibujar, modelo.pantalla, 64.dp)
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text(n.uppercase(), style = T.nombre.copy(fontSize = 16.sp), color = C.tinta, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Grabado("%.1f s".format((doc.numero("duracion") ?: 0.0) / 1000) + if (doc.si("repetir") == true) " · en bucle" else "", color = C.sobrePlaca)
                }
                Tecla("◉", {
                    val c = Lab.ponerAnimacion(config, n, doc)
                    val e = Nucleo.validarConfig(c.toString())
                    if (e != null) error = e
                    else {
                        error = null
                        vm.pausarEspejo(8.0)
                        vm.aplicarEnVistaPrevia(c)
                        Nucleo.bleEscribir(Car.COMANDO.id, Comando.reproducir(n))
                    }
                }, acabado = Acabado.NARANJA, cuadrada = true, descripcion = "Verla acá")
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                if (propia) {
                    Tecla("▶ En el robot", { alcance.launch { error = vm.comando(reproducir = n) } }, acabado = Acabado.NEGRA, chica = true, habilitada = robot?.conectado == true)
                    Tecla("✕", { guardar(Lab.ponerAnimacion(config, n, null), "Se borró.") }, acabado = Acabado.NEGRA, cuadrada = true, descripcion = "Borrar")
                } else Tecla("+ Sumar", {
                    val libre = Lab.libre(n, propias.keys)
                    guardar(Lab.ponerAnimacion(config, libre, doc), "«$libre» está en el robot.")
                }, chica = true)
            }
        }
    }

    MarcoPantalla(modelo.pantalla) { if (llave > 0) PantallaViva(llave, "Vista previa") }
    aviso?.let { AvisoLinea(it) }
    error?.let { AvisoLinea(it, error = true) }
    PlacaNaranja(Modifier.fillMaxWidth(), relleno = 14.dp) {
        Grabado("Tuyas", color = C.sobrePlaca)
        if (propias.isEmpty()) Text("Ninguna todavía.", style = T.texto, color = Color(0xCC1A1714))
        propias.keys.sorted().forEach { n -> Fila(n, propias.getValue(n), true) }
        Grabado("Ejemplos", color = C.sobrePlaca)
        VidaBase.animaciones.forEach { (n, d) -> Fila(n, d, false) }
    }
}
