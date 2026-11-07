package com.sinteticalabs.app.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
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

/** Los tokens del manual de marca (temas Claro y Pantalla). */
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
    papel = Color(0xFF0E0D0C), papelAlto = Color(0xFF1A1816), linea = Color(0xFF2C2925), tinta = Color(0xFFF5F2EC),
    tintaSuave = Color(0xFFA39E96), naranja = Color(0xFFFF6A33), naranjaProfundo = Color(0xFFFF8A5C),
    naranjaClaro = Color(0xFF4A2416), sobreNaranja = Color(0xFF141210), pantalla = Color(0xFF0B0A09), carcasa = Color(0xFF26231F),
)

val LocalPaleta = staticCompositionLocalOf { Claro }

@OptIn(ExperimentalTextApi::class)
private fun fraunces(peso: Int) = Font(
    R.font.fraunces, weight = FontWeight(peso),
    variationSettings = FontVariation.Settings(FontVariation.weight(peso), FontVariation.Setting("SOFT", 100f), FontVariation.Setting("WONK", 0f)),
)

@OptIn(ExperimentalTextApi::class)
private fun mono(peso: Int) = Font(R.font.jetbrains_mono, weight = FontWeight(peso), variationSettings = FontVariation.Settings(FontVariation.weight(peso)))

val Fraunces = FontFamily(fraunces(400), fraunces(700), fraunces(800))
val Mono = FontFamily(mono(400), mono(700))

object Tipos {
    val display = TextStyle(fontFamily = Fraunces, fontWeight = FontWeight(800), fontSize = 40.sp, lineHeight = 40.sp, letterSpacing = (-0.02).em)
    val titulo = TextStyle(fontFamily = Fraunces, fontWeight = FontWeight(700), fontSize = 24.sp, lineHeight = 30.sp)
    val cuerpo = TextStyle(fontFamily = Fraunces, fontWeight = FontWeight(400), fontSize = 17.sp, lineHeight = 26.sp)
    val chico = TextStyle(fontFamily = Fraunces, fontWeight = FontWeight(400), fontSize = 14.sp, lineHeight = 20.sp)
    val dato = TextStyle(fontFamily = Mono, fontWeight = FontWeight(400), fontSize = 14.sp, lineHeight = 20.sp)
    val etiqueta = TextStyle(fontFamily = Mono, fontWeight = FontWeight(700), fontSize = 11.sp, lineHeight = 16.sp, letterSpacing = 0.12.em)
    val boton = TextStyle(fontFamily = Mono, fontWeight = FontWeight(700), fontSize = 13.sp, letterSpacing = 0.08.em)
    val serie = TextStyle(fontFamily = Mono, fontWeight = FontWeight(700), fontSize = 13.sp, letterSpacing = 0.1.em)
}

@Composable
fun TemaSintetica(oscuro: Boolean = isSystemInDarkTheme(), contenido: @Composable () -> Unit) {
    val p = if (oscuro) Oscuro else Claro
    val esquema = if (oscuro) darkColorScheme(primary = p.naranja, onPrimary = p.sobreNaranja, background = p.papel, onBackground = p.tinta, surface = p.papelAlto, onSurface = p.tinta, outline = p.linea)
    else lightColorScheme(primary = p.naranja, onPrimary = p.sobreNaranja, background = p.papel, onBackground = p.tinta, surface = p.papelAlto, onSurface = p.tinta, outline = p.linea)
    CompositionLocalProvider(LocalPaleta provides p) {
        MaterialTheme(colorScheme = esquema, content = contenido)
    }
}
