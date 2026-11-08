package com.sinteticalabs.app.ui.consola

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shadow
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.ui.Archivo
import com.sinteticalabs.app.ui.ArchivoAncha
import com.sinteticalabs.app.ui.ArchivoAngosta

/** La paleta de la consola (web/src/lib/consola/consola.css). */
object C {
    val fondo = Color(0xFF16140F)
    val fondoAlto = Color(0xFF2A2622)
    val chapa = Color(0xFF211E1A)
    val chapaAlta = Color(0xFF2C2925)
    val chapaBaja = Color(0xFF1C1A17)
    val hundido = Color(0xFF0E0D0B)
    val papel = Color(0xFFF3EEE8)
    val gris = Color(0xFF8F877C)
    val grisClaro = Color(0xFFB9B1A6)
    val grisOscuro = Color(0xFF5A524A)
    val crema = Color(0xFFD9D2C6)
    val naranja = Color(0xFFFF5A1F)
    val brasa = Color(0xFFFF8A5C)
    val led = Color(0xFFFF6A33)
    val verde = Color(0xFF4ADE80)
    val ambar = Color(0xFFFFB347)
    val rojo = Color(0xFFFF4D4D)
    val tinta = Color(0xFF1A1714)
    val ledApagado = Color(0xFF4A443E)

    // La placa naranja anodizada.
    val placaArriba = Color(0xFFFF7039)
    val placaMedio = Color(0xFFF2541A)
    val placaAbajo = Color(0xFFD9450F)
    val sobrePlaca = Color(0xA81A1714)
}

/** Los textos de la consola: títulos grabados, el titular, los dígitos del visor. */
object T {
    val grabado = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(800), fontSize = 11.sp, lineHeight = 13.sp, letterSpacing = 0.14.em, color = C.gris)
    val titular = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(900), fontSize = 48.sp, lineHeight = 44.sp, letterSpacing = (-0.05).em)
    val tecla = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(800), fontSize = 13.sp, lineHeight = 14.sp, letterSpacing = 0.06.em)
    val teclaChica = tecla.copy(fontSize = 11.sp, lineHeight = 12.sp)
    val modulo = TextStyle(fontFamily = ArchivoAngosta, fontWeight = FontWeight(900), fontSize = 11.sp, lineHeight = 12.sp, letterSpacing = 0.01.em)
    private val brillo = Shadow(color = Color(0x73FF6A33), blurRadius = 24f)
    val digitos = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(900), fontSize = 44.sp, lineHeight = 46.sp, color = C.brasa, shadow = brillo)
    val digitosChicos = digitos.copy(fontSize = 22.sp, lineHeight = 24.sp)
    val campo = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(900), fontSize = 24.sp, lineHeight = 28.sp, color = C.brasa, shadow = brillo)
    val campoClaro = TextStyle(fontFamily = Archivo, fontWeight = FontWeight(800), fontSize = 18.sp, lineHeight = 22.sp, color = C.papel)
    val texto = TextStyle(fontFamily = Archivo, fontWeight = FontWeight(600), fontSize = 15.sp, lineHeight = 21.sp, color = C.grisClaro)
    val nombre = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(900), fontSize = 30.sp, lineHeight = 30.sp, letterSpacing = (-0.03).em, color = C.papel)
    val dato = TextStyle(fontFamily = Archivo, fontWeight = FontWeight(700), fontSize = 14.sp, lineHeight = 18.sp, color = C.crema)
    val aviso = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(900), fontSize = 20.sp, lineHeight = 22.sp, color = C.tinta)
}
