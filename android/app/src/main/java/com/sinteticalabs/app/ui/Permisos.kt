package com.sinteticalabs.app.ui

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat

/** Permisos de Bluetooth: Android 12+ pide SCAN/CONNECT; 10 y 11, ubicación. */
val PERMISOS_BLE: Array<String> = if (Build.VERSION.SDK_INT >= 31)
    arrayOf(Manifest.permission.BLUETOOTH_SCAN, Manifest.permission.BLUETOOTH_CONNECT)
else arrayOf(Manifest.permission.ACCESS_FINE_LOCATION)

val PERMISOS_CAMARA = arrayOf(Manifest.permission.CAMERA)

fun tiene(contexto: Context, permisos: Array<String>) =
    permisos.all { ContextCompat.checkSelfPermission(contexto, it) == PackageManager.PERMISSION_GRANTED }

/** Pide los permisos al entrar y dice si están. */
@Composable
fun conPermisos(permisos: Array<String>): Boolean {
    val contexto = LocalContext.current
    var concedidos by remember { mutableStateOf(tiene(contexto, permisos)) }
    val lanzador = rememberLauncherForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { r ->
        concedidos = r.values.all { it }
    }
    LaunchedEffect(Unit) { if (!concedidos) lanzador.launch(permisos) }
    return concedidos
}
