package com.sinteticalabs.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

/** El único botón naranja de cada pantalla: una tecla mecánica naranja. */
@Composable
fun BotonPrimario(texto: String, alTocar: () -> Unit, modifier: Modifier = Modifier, habilitado: Boolean = true) {
    com.sinteticalabs.app.ui.consola.Tecla(texto, alTocar, modifier.fillMaxWidth(), acabado = com.sinteticalabs.app.ui.consola.Acabado.NARANJA, habilitada = habilitado)
}

/** Una tecla negra. */
@Composable
fun BotonSecundario(texto: String, alTocar: () -> Unit, modifier: Modifier = Modifier, habilitado: Boolean = true, @Suppress("UNUSED_PARAMETER") contenido: PaddingValues = PaddingValues()) {
    com.sinteticalabs.app.ui.consola.Tecla(texto, alTocar, modifier, acabado = com.sinteticalabs.app.ui.consola.Acabado.NEGRA, habilitada = habilitado)
}

@Composable
fun Etiqueta(texto: String, modifier: Modifier = Modifier) {
    Text(texto.uppercase(), style = Tipos.etiqueta, color = LocalPaleta.current.tintaSuave, modifier = modifier)
}

/** Una fila de datos en mono, separada por una línea fina (manual: app). */
@Composable
fun FilaDato(etiqueta: String, valor: String) {
    val p = LocalPaleta.current
    Column {
        Row(Modifier.fillMaxWidth().padding(vertical = 12.dp), horizontalArrangement = Arrangement.SpaceBetween, verticalAlignment = Alignment.CenterVertically) {
            Etiqueta(etiqueta)
            Text(valor, style = Tipos.dato, color = p.tinta)
        }
        HorizontalDivider(color = p.linea, thickness = 1.dp)
    }
}

@Composable
fun Seccion(titulo: String, contenido: @Composable () -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(titulo, style = Tipos.titulo, color = LocalPaleta.current.tinta)
        contenido()
    }
}

@Composable
fun Aviso(texto: String, modifier: Modifier = Modifier) {
    val p = LocalPaleta.current
    Box(modifier.fillMaxWidth().background(p.naranjaClaro, RoundedCornerShape(12.dp)).padding(16.dp)) {
        Text(texto, style = Tipos.chico, color = p.tinta)
    }
}
