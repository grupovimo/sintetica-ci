package com.sinteticalabs.app.datos

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map

/** La sesión en la nube: a qué servidor, con qué email y el token de la sesión. */
data class Sesion(val servidor: String, val email: String, val token: String) {
    val activa get() = token.isNotEmpty()
}

private val Context.cuenta by preferencesDataStore(name = "cuenta")
private val SERVIDOR = stringPreferencesKey("servidor")
private val EMAIL = stringPreferencesKey("email")
private val TOKEN = stringPreferencesKey("token")

/** En local, la API corre en la Mac: http://<la IP de la Mac>:8080. */
const val SERVIDOR_POR_DEFECTO = "http://192.168.1.10:8080"

class Cuenta(private val contexto: Context) {
    val sesion: Flow<Sesion> = contexto.cuenta.data.map { p ->
        Sesion(p[SERVIDOR] ?: SERVIDOR_POR_DEFECTO, p[EMAIL].orEmpty(), p[TOKEN].orEmpty())
    }

    suspend fun actual(): Sesion = sesion.first()

    suspend fun guardarServidor(url: String) = contexto.cuenta.edit { it[SERVIDOR] = url.trim().trimEnd('/') }
    suspend fun guardarEmail(email: String) = contexto.cuenta.edit { it[EMAIL] = email.trim() }
    suspend fun entrar(token: String) = contexto.cuenta.edit { it[TOKEN] = token }
    suspend fun salir() = contexto.cuenta.edit { it.remove(TOKEN) }

    fun api(s: Sesion) = ApiSintetica(s.servidor, s.token)
}
