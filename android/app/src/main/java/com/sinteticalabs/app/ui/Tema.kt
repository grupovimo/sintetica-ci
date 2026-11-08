package com.sinteticalabs.app.ui

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.ExperimentalTextApi
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import com.sinteticalabs.app.R

/** Los tokens del manual de marca (siempre el tema oscuro). */
@Immutable
data class Paleta(
    val papel: Color, val papelAlto: Color, val linea: Color, val tinta: Color, val tintaSuave: Color,
    val naranja: Color, val naranjaProfundo: Color, val naranjaClaro: Color, val sobreNaranja: Color,
    val pantalla: Color, val carcasa: Color,
)

val Claro = Paleta(
    papel = Color(0xFFFBFAF7), papelAlto = Color(0xFFFFFFFF), linea = Color(0xFFE6E2DA), tinta = Color(0xFF141210),
    tintaSuave = Color(0xFF6B6660), naranja = Color(0xFFF5501A), naranjaProfundo = Color(0xFFB83C0C),
    naranjaClaro = Color(0xFFFFE3D6), sobreNaranja = Color(0xFF141210), pantalla = Color(0xFF0B0A09), carcasa = Color(0xFF1A1816),
)
val Oscuro = Paleta(
    papel = Color(0xFF16140F), papelAlto = Color(0xFF211E1A), linea = Color(0xFF2C2925), tinta = Color(0xFFF3EEE8),
    tintaSuave = Color(0xFFB9B1A6), naranja = Color(0xFFFF6A33), naranjaProfundo = Color(0xFFFF8A5C),
    naranjaClaro = Color(0xFF3A2216), sobreNaranja = Color(0xFF1A1714), pantalla = Color(0xFF0B0A09), carcasa = Color(0xFF26231F),
)

val LocalPaleta = staticCompositionLocalOf { Oscuro }

/** Archivo (variable, OFL): el peso y el ancho se eligen al pedirla. */
@OptIn(ExperimentalTextApi::class)
fun archivo(peso: Int, ancho: Float = 100f) = Font(
    R.font.archivo, weight = FontWeight(peso),
    variationSettings = FontVariation.Settings(FontVariation.weight(peso), FontVariation.width(ancho)),
)

/** La letra de la marca, en tres anchos (normal, expandida y angosta). */
val Archivo = FontFamily(archivo(500), archivo(600), archivo(700), archivo(800), archivo(900))
val ArchivoAncha = FontFamily(archivo(700, 118f), archivo(800, 118f), archivo(900, 122f))
val ArchivoAngosta = FontFamily(archivo(800, 72f), archivo(900, 70f))

object Tipos {
    val display = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(900), fontSize = 40.sp, lineHeight = 38.sp, letterSpacing = (-0.03).em)
    val titulo = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(800), fontSize = 22.sp, lineHeight = 26.sp)
    val cuerpo = TextStyle(fontFamily = Archivo, fontWeight = FontWeight(600), fontSize = 16.sp, lineHeight = 23.sp)
    val chico = TextStyle(fontFamily = Archivo, fontWeight = FontWeight(600), fontSize = 14.sp, lineHeight = 20.sp)
    val dato = TextStyle(fontFamily = Archivo, fontWeight = FontWeight(700), fontSize = 14.sp, lineHeight = 20.sp)
    val etiqueta = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(800), fontSize = 11.sp, lineHeight = 14.sp, letterSpacing = 0.14.em)
    val boton = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(800), fontSize = 13.sp, letterSpacing = 0.06.em)
    val serie = TextStyle(fontFamily = ArchivoAncha, fontWeight = FontWeight(800), fontSize = 12.sp, letterSpacing = 0.12.em)
}

/** Modo oscuro siempre: la consola es una pieza de hardware oscura. */
@Composable
fun TemaSintetica(contenido: @Composable () -> Unit) {
    val p = Oscuro
    val esquema = darkColorScheme(primary = p.naranja, onPrimary = p.sobreNaranja, background = p.papel, onBackground = p.tinta, surface = p.papelAlto, onSurface = p.tinta, outline = p.linea)
    CompositionLocalProvider(LocalPaleta provides p) {
        MaterialTheme(colorScheme = esquema, typography = androidx.compose.material3.Typography(), content = contenido)
    }
}
