package com.sinteticalabs.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.sinteticalabs.app.datos.Vinculados
import com.sinteticalabs.app.protocolo.DatosQr
import com.sinteticalabs.app.ui.BichoViewModel
import com.sinteticalabs.app.ui.ID_DEMO
import com.sinteticalabs.app.ui.PantallaBicho
import com.sinteticalabs.app.ui.PantallaEscanear
import com.sinteticalabs.app.ui.PantallaInicio
import com.sinteticalabs.app.ui.PantallaVinculando
import com.sinteticalabs.app.ui.TemaSintetica

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent { TemaSintetica { App() } }
    }
}

@androidx.compose.runtime.Composable
private fun App() {
    val nav = rememberNavController()
    val contexto = LocalContext.current
    val vinculados = remember { Vinculados(contexto) }
    val bichos by vinculados.todos.collectAsState(initial = emptyList())
    NavHost(nav, startDestination = "inicio") {
        composable("inicio") {
            PantallaInicio(
                bichos,
                alElegir = { nav.navigate("bicho/$it") },
                alVincular = { nav.navigate("escanear") },
                alProbar = { nav.navigate("bicho/$ID_DEMO") },
            )
        }
        composable("escanear") {
            PantallaEscanear(
                alLeer = { qr -> nav.navigate("vinculando/${qr.id}/${qr.codigo}") { popUpTo("inicio") } },
                alVolver = { nav.popBackStack() },
            )
        }
        composable("vinculando/{id}/{codigo}") { e ->
            val qr = DatosQr(e.arguments?.getString("id").orEmpty(), e.arguments?.getString("codigo").orEmpty())
            PantallaVinculando(qr, alTerminar = { id -> nav.navigate("bicho/$id") { popUpTo("inicio") } }, alVolver = { nav.popBackStack() })
        }
        composable("bicho/{id}") { e ->
            val vm: BichoViewModel = viewModel()
            vm.abrir(e.arguments?.getString("id").orEmpty())
            PantallaBicho(vm, alVolver = { nav.popBackStack("inicio", false) }, alRevincular = { nav.navigate("escanear") { popUpTo("inicio") } })
        }
    }
}
