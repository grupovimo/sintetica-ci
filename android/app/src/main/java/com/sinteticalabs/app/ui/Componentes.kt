package com.sinteticalabs.app.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

/** El único botón naranja de cada pantalla (manual: un solo primario por pantalla). */
@Composable
fun BotonPrimario(texto: String, alTocar: () -> Unit, modifier: Modifier = Modifier, habilitado: Boolean = true) {
    val p = LocalPaleta.current
    Button(
        onClick = alTocar, enabled = habilitado, modifier = modifier.fillMaxWidth().height(52.dp),
        shape = RoundedCornerShape(12.dp),
        colors = ButtonDefaults.buttonColors(containerColor = p.naranja, contentColor = p.sobreNaranja, disabledContainerColor = p.linea, disabledContentColor = p.tintaSuave),
    ) { Text(texto.uppercase(), style = Tipos.boton) }
}

@Composable
fun BotonSecundario(texto: String, alTocar: () -> Unit, modifier: Modifier = Modifier, habilitado: Boolean = true, contenido: PaddingValues = ButtonDefaults.ContentPadding) {
    val p = LocalPaleta.current
    OutlinedButton(
        onClick = alTocar, enabled = habilitado, modifier = modifier, shape = RoundedCornerShape(12.dp),
        border = BorderStroke(1.dp, p.linea), contentPadding = contenido,
        colors = ButtonDefaults.outlinedButtonColors(contentColor = p.tinta),
    ) { Text(texto.uppercase(), style = Tipos.boton) }
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
