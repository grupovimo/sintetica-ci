package com.sinteticalabs.app.ui.consola

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import com.sinteticalabs.app.consola.cortarBytes

/**
 * Un campo de texto dentro de un visor: se escribe libre (cortado en bytes,
 * como lo cuenta el robot) y se guarda al terminar (Listo o al salir).
 */
@Composable
fun CampoVisor(
    valor: String,
    alTerminar: (String) -> Unit,
    modifier: Modifier = Modifier,
    placeholder: String = "",
    estilo: TextStyle = T.campo,
    maxBytes: Int = 24,
    teclado: KeyboardType = KeyboardType.Text,
    alEscribir: ((String) -> Unit)? = null,
) {
    var texto by remember(valor) { mutableStateOf(valor) }
    var enfocado by remember { mutableStateOf(false) }
    val foco = LocalFocusManager.current
    BasicTextField(
        value = texto,
        onValueChange = { texto = cortarBytes(it, maxBytes); alEscribir?.invoke(texto) },
        singleLine = true,
        textStyle = estilo,
        cursorBrush = SolidColor(C.brasa),
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done, keyboardType = teclado),
        keyboardActions = KeyboardActions(onDone = { foco.clearFocus() }),
        modifier = modifier.fillMaxWidth().onFocusChanged {
            if (enfocado && !it.isFocused && texto != valor) alTerminar(texto)
            enfocado = it.isFocused
        },
        decorationBox = { campo ->
            Box {
                if (texto.isEmpty()) Text(placeholder, style = estilo.copy(color = C.grisOscuro, shadow = null), maxLines = 1)
                campo()
            }
        },
    )
}

/** Un valor que se elige de una lista (se abre un menú). */
@Composable
fun Selector(
    texto: String,
    opciones: List<Pair<String, String>>,
    alElegir: (String) -> Unit,
    modifier: Modifier = Modifier,
    estilo: TextStyle = T.campoClaro,
    habilitado: Boolean = true,
) {
    var abierto by remember { mutableStateOf(false) }
    Box(modifier) {
        Text(
            "$texto ▾", style = estilo, maxLines = 1, overflow = TextOverflow.Ellipsis,
            modifier = Modifier.clickable(enabled = habilitado, role = Role.DropdownList) { abierto = true },
        )
        DropdownMenu(expanded = abierto, onDismissRequest = { abierto = false }, containerColor = C.chapa) {
            opciones.forEach { (k, n) ->
                DropdownMenuItem(text = { Text(n, style = T.dato, color = C.papel) }, onClick = { abierto = false; alElegir(k) })
            }
        }
    }
}
