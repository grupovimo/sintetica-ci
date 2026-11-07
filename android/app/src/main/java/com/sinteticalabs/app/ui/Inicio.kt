package com.sinteticalabs.app.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.sinteticalabs.app.datos.Vinculado

@Composable
fun PantallaInicio(bichos: List<Vinculado>, alElegir: (String) -> Unit, alVincular: () -> Unit, alProbar: () -> Unit) {
    val p = LocalPaleta.current
    Surface(color = p.papel, modifier = Modifier.fillMaxSize()) {
        Column(Modifier.safeDrawingPadding().padding(24.dp), verticalArrangement = Arrangement.spacedBy(24.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Etiqueta("Sintética Labs")
                Text(if (bichos.isEmpty()) "Hola." else "Tus bichos", style = Tipos.display, color = p.tinta)
            }
            if (bichos.isEmpty()) {
                Text("Prendé tu bicho: en la pantalla va a aparecer un QR. Escanealo para vincularlo con este celu.", style = Tipos.cuerpo, color = p.tintaSuave)
            }
            LazyColumn(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                items(bichos, key = { it.id }) { b ->
                    Surface(
                        color = p.papelAlto, shape = RoundedCornerShape(24.dp), border = BorderStroke(1.dp, p.linea),
                        modifier = Modifier.fillMaxWidth().clickable { alElegir(b.id) },
                    ) {
                        Column(Modifier.padding(20.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Text(if (b.serie.isNotEmpty()) "UNIDAD #${b.serie}" else "UNIDAD #—", style = Tipos.serie, color = p.naranjaProfundo)
                            Text(b.nombre, style = Tipos.titulo, color = p.tinta)
                            Text(b.id, style = Tipos.dato, color = p.tintaSuave)
                        }
                    }
                }
            }
            BotonPrimario("Vincular un bicho", alVincular)
            BotonSecundario("Probar sin bicho", alProbar, Modifier.fillMaxWidth())
            Spacer(Modifier.padding(4.dp))
        }
    }
}
