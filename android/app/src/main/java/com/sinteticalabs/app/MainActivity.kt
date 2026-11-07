package com.sinteticalabs.app

import android.content.Intent
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
import com.sinteticalabs.app.ui.CuentaViewModel
import com.sinteticalabs.app.ui.PREFIJO_NUBE
import com.sinteticalabs.app.ui.PantallaCuenta
import com.sinteticalabs.app.ui.ID_DEMO
import com.sinteticalabs.app.ui.ID_DEMO_BROCHE
import com.sinteticalabs.app.ui.PantallaBicho
import com.sinteticalabs.app.ui.PantallaEscanear
import com.sinteticalabs.app.ui.PantallaInicio
import com.sinteticalabs.app.ui.PantallaVinculando
import com.sinteticalabs.app.ui.TemaSintetica

class MainActivity : ComponentActivity() {
    /** El token del link mágico, si la app se abrió con sintetica://entrar?t=… */
    private val linkMagico = kotlinx.coroutines.flow.MutableStateFlow<String?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        leerLink(intent)
        setContent { TemaSintetica { App(linkMagico) } }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        leerLink(intent)
    }

    private fun leerLink(i: Intent?) {
        val d = i?.data ?: return
        if (d.scheme == "sintetica" && d.host == "entrar") linkMagico.value = d.getQueryParameter("t")
    }
}

@androidx.compose.runtime.Composable
private fun App(linkMagico: kotlinx.coroutines.flow.MutableStateFlow<String?>) {
    val nav = rememberNavController()
    val contexto = LocalContext.current
    val vinculados = remember { Vinculados(contexto) }
    val bichos by vinculados.todos.collectAsState(initial = emptyList())
    val cuenta: CuentaViewModel = viewModel()
    val sesion by cuenta.sesion.collectAsState()
    val enLaNube by cuenta.bichos.collectAsState()
    val link by linkMagico.collectAsState()
    androidx.compose.runtime.LaunchedEffect(link) {
        link?.let { cuenta.entrarConLink(it); linkMagico.value = null; nav.navigate("cuenta") }
    }
    androidx.compose.runtime.LaunchedEffect(sesion?.activa) { if (sesion?.activa == true) cuenta.cargarBichos() }
    NavHost(nav, startDestination = "inicio") {
        composable("inicio") {
            androidx.compose.runtime.LaunchedEffect(Unit) { cuenta.cargarBichos() }
            PantallaInicio(
                bichos,
                alElegir = { nav.navigate("bicho/$it") },
                alVincular = { nav.navigate("escanear") },
                alProbar = { broche -> nav.navigate("bicho/" + if (broche) ID_DEMO_BROCHE else ID_DEMO) },
                enLaNube = enLaNube,
                conCuenta = sesion?.activa == true,
                alCuenta = { nav.navigate("cuenta") },
                alElegirNube = { nav.navigate("bicho/$PREFIJO_NUBE$it") },
            )
        }
        composable("cuenta") { PantallaCuenta(cuenta, alVolver = { nav.popBackStack("inicio", false) }) }
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
