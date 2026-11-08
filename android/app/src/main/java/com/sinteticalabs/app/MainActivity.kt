package com.sinteticalabs.app

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.sinteticalabs.app.protocolo.DatosQr
import com.sinteticalabs.app.ui.CuentaViewModel
import com.sinteticalabs.app.ui.PantallaCuenta
import com.sinteticalabs.app.ui.PantallaEscanear
import com.sinteticalabs.app.ui.PantallaVinculando
import com.sinteticalabs.app.ui.TemaSintetica
import com.sinteticalabs.app.ui.consola.PantallaConsola
import com.sinteticalabs.app.ui.consola.PantallaMisRobots
import com.sinteticalabs.app.ui.consola.RobotViewModel
import com.sinteticalabs.app.ui.consola.RobotsViewModel

class MainActivity : ComponentActivity() {
    /** El token del link mágico, si la app se abrió con sintetica://entrar?t=… */
    private val linkMagico = kotlinx.coroutines.flow.MutableStateFlow<String?>(null)
    /** Un link de la web que abrió la app: ?amigo=CODIGO o ?spotify=ok|error|cancelado. */
    private val linkWeb = kotlinx.coroutines.flow.MutableStateFlow<Pair<String, String>?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        leerLink(intent)
        setContent { TemaSintetica { App(linkMagico, linkWeb) } }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        leerLink(intent)
    }

    private fun leerLink(i: Intent?) {
        val d = i?.data ?: return
        if (d.scheme == "sintetica" && d.host == "entrar") linkMagico.value = d.getQueryParameter("t")
        com.sinteticalabs.app.consola.LinksWeb.leer(d.scheme, d.host, d.path, d.getQueryParameter("amigo"), d.getQueryParameter("spotify"))?.let { linkWeb.value = it }
    }
}

@androidx.compose.runtime.Composable
private fun App(linkMagico: kotlinx.coroutines.flow.MutableStateFlow<String?>, linkWeb: kotlinx.coroutines.flow.MutableStateFlow<Pair<String, String>?>) {
    val nav = rememberNavController()
    val cuenta: CuentaViewModel = viewModel()
    val link by linkMagico.collectAsState()
    androidx.compose.runtime.LaunchedEffect(link) {
        link?.let { cuenta.entrarConLink(it); linkMagico.value = null; nav.navigate("cuenta") }
    }
    val web by linkWeb.collectAsState()
    web?.let { (que, valor) -> com.sinteticalabs.app.ui.consola.LinkDeLaWeb(que, valor) { linkWeb.value = null } }
    NavHost(nav, startDestination = "robots") {
        composable("robots") {
            val vm: RobotsViewModel = viewModel()
            PantallaMisRobots(
                vm,
                alAbrir = { nav.navigate("robot/" + android.net.Uri.encode(it)) },
                alEscanear = { nav.navigate("escanear") },
                alCuenta = { nav.navigate("cuenta") },
            )
        }
        composable("cuenta") { PantallaCuenta(cuenta, alVolver = { nav.popBackStack("robots", false) }) }
        composable("escanear") {
            PantallaEscanear(
                alLeer = { qr -> nav.navigate("vinculando/${qr.id}/${qr.codigo}") { popUpTo("robots") } },
                alVolver = { nav.popBackStack() },
            )
        }
        composable("vinculando/{id}/{codigo}") { e ->
            val qr = DatosQr(e.arguments?.getString("id").orEmpty(), e.arguments?.getString("codigo").orEmpty())
            PantallaVinculando(qr, alTerminar = { id -> nav.navigate("robot/$id") { popUpTo("robots") } }, alVolver = { nav.popBackStack() })
        }
        composable("robot/{id}") { e ->
            val vm: RobotViewModel = viewModel()
            vm.abrir(e.arguments?.getString("id").orEmpty())
            PantallaConsola(vm, alVolver = { nav.popBackStack("robots", false) })
        }
    }
}
