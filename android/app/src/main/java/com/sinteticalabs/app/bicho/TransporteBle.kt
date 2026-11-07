package com.sinteticalabs.app.bicho

import android.annotation.SuppressLint
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothGatt
import android.bluetooth.BluetoothGattCallback
import android.bluetooth.BluetoothGattCharacteristic
import android.bluetooth.BluetoothGattDescriptor
import android.bluetooth.BluetoothManager
import android.bluetooth.BluetoothProfile
import android.bluetooth.le.ScanCallback
import android.bluetooth.le.ScanFilter
import android.bluetooth.le.ScanResult
import android.bluetooth.le.ScanSettings
import android.content.Context
import android.os.Build
import android.os.ParcelUuid
import com.sinteticalabs.app.protocolo.Car
import com.sinteticalabs.app.protocolo.Op
import com.sinteticalabs.app.protocolo.Uuids
import com.sinteticalabs.app.protocolo.idDeAnuncio
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withTimeout

class ErrorBle(mensaje: String) : Exception(mensaje)

/** Un bicho encontrado en el aire. */
data class BichoCerca(val id: String, val nombre: String?, val dispositivo: BluetoothDevice, val rssi: Int)

/**
 * Escanea bichos de SintéticaOS: filtra por el UUID del servicio y lee el id de
 * los datos de fabricante (0xFFFF + MAC). Los permisos los pide la pantalla.
 */
@SuppressLint("MissingPermission")
fun escanearBichos(contexto: Context): Flow<BichoCerca> = callbackFlow {
    val adaptador = contexto.getSystemService(BluetoothManager::class.java)?.adapter
        ?: throw ErrorBle("Este celu no tiene Bluetooth.")
    if (!adaptador.isEnabled) throw ErrorBle("Prendé el Bluetooth.")
    val escaner = adaptador.bluetoothLeScanner ?: throw ErrorBle("Prendé el Bluetooth.")
    val callback = object : ScanCallback() {
        override fun onScanResult(callbackType: Int, r: ScanResult) {
            val datos = r.scanRecord?.getManufacturerSpecificData(0xFFFF) ?: return
            // getManufacturerSpecificData ya saca el id de empresa: reponerlo para leerlo igual que en las otras apps.
            val id = idDeAnuncio(byteArrayOf(0xFF.toByte(), 0xFF.toByte()) + datos) ?: return
            trySend(BichoCerca(id, r.scanRecord?.deviceName, r.device, r.rssi))
        }
        override fun onScanFailed(errorCode: Int) { close(ErrorBle("No pude buscar bichos (error $errorCode).")) }
    }
    val filtro = ScanFilter.Builder().setServiceUuid(ParcelUuid(Uuids.SERVICIO)).build()
    val ajustes = ScanSettings.Builder().setScanMode(ScanSettings.SCAN_MODE_LOW_LATENCY).build()
    escaner.startScan(listOf(filtro), ajustes, callback)
    awaitClose { runCatching { escaner.stopScan(callback) } }
}

/** Busca el bicho con este id (el del QR) durante unos segundos. */
suspend fun buscarBicho(contexto: Context, id: String, ms: Long = 20_000): BichoCerca =
    try {
        withTimeout(ms) { escanearBichos(contexto).first { it.id == id } }
    } catch (e: kotlinx.coroutines.TimeoutCancellationException) {
        throw ErrorBle("No encuentro el bicho. Acercalo al celu y fijate que esté prendido.")
    }

/**
 * BLE con BluetoothGatt, envuelto en corrutinas: una operación GATT por vez
 * (Android no encola), MTU de 185 y notificaciones en Config, Comando y Estado.
 */
@SuppressLint("MissingPermission")
class TransporteBle(private val contexto: Context, private val dispositivo: BluetoothDevice) : Transporte {
    override val notificaciones = MutableSharedFlow<Pair<Car, ByteArray>>(extraBufferCapacity = 256)
    private var gatt: BluetoothGatt? = null
    private val una = Mutex()
    private var pendiente: CompletableDeferred<Any?>? = null
    private val conectado = CompletableDeferred<Unit>()
    private val caracteristicas = mutableMapOf<Car, BluetoothGattCharacteristic>()

    private val callback = object : BluetoothGattCallback() {
        override fun onConnectionStateChange(g: BluetoothGatt, status: Int, nuevo: Int) {
            if (nuevo == BluetoothProfile.STATE_CONNECTED) g.discoverServices()
            else if (nuevo == BluetoothProfile.STATE_DISCONNECTED) {
                val e = ErrorBle("Se cortó la conexión con el bicho.")
                conectado.completeExceptionally(e)
                pendiente?.completeExceptionally(e)
            }
        }
        override fun onServicesDiscovered(g: BluetoothGatt, status: Int) {
            val s = g.getService(Uuids.SERVICIO)
            if (s == null) { conectado.completeExceptionally(ErrorBle("Ese dispositivo no es un bicho.")); return }
            Car.entries.forEach { c -> s.getCharacteristic(c.uuid)?.let { caracteristicas[c] = it } }
            conectado.complete(Unit)
        }
        override fun onMtuChanged(g: BluetoothGatt, mtu: Int, status: Int) { pendiente?.complete(mtu) }
        override fun onDescriptorWrite(g: BluetoothGatt, d: BluetoothGattDescriptor, status: Int) { pendiente?.complete(status) }
        override fun onCharacteristicWrite(g: BluetoothGatt, c: BluetoothGattCharacteristic, status: Int) { pendiente?.complete(status) }
        @Deprecated("API < 33")
        override fun onCharacteristicRead(g: BluetoothGatt, c: BluetoothGattCharacteristic, status: Int) {
            @Suppress("DEPRECATION") pendiente?.complete(c.value)
        }
        override fun onCharacteristicRead(g: BluetoothGatt, c: BluetoothGattCharacteristic, valor: ByteArray, status: Int) { pendiente?.complete(valor) }
        @Deprecated("API < 33")
        override fun onCharacteristicChanged(g: BluetoothGatt, c: BluetoothGattCharacteristic) {
            @Suppress("DEPRECATION") val v = c.value ?: return
            Car.de(c.uuid)?.let { notificaciones.tryEmit(it to v.copyOf()) }
        }
        override fun onCharacteristicChanged(g: BluetoothGatt, c: BluetoothGattCharacteristic, valor: ByteArray) {
            Car.de(c.uuid)?.let { notificaciones.tryEmit(it to valor) }
        }
    }

    private suspend fun <T> operacion(lanzar: () -> Boolean): T = una.withLock {
        val d = CompletableDeferred<Any?>()
        pendiente = d
        if (!lanzar()) throw ErrorBle("El celu no aceptó la operación Bluetooth.")
        @Suppress("UNCHECKED_CAST")
        withTimeout(5_000) { d.await() } as T
    }

    override suspend fun conectar() {
        gatt = dispositivo.connectGatt(contexto, false, callback, BluetoothDevice.TRANSPORT_LE)
        withTimeout(15_000) { conectado.await() }
        val g = gatt!!
        operacion<Int> { g.requestMtu(185) }
        for (c in listOf(Car.CONFIG, Car.COMANDO, Car.ESTADO, Car.IMAGEN)) {
            // Imagen es de la v1.2: un firmware viejo no la tiene.
            val car = caracteristicas[c] ?: if (c == Car.IMAGEN) continue else throw ErrorBle("Al bicho le falta la característica ${c.name}.")
            g.setCharacteristicNotification(car, true)
            val d = car.getDescriptor(Uuids.CCCD) ?: continue
            operacion<Int> {
                if (Build.VERSION.SDK_INT >= 33) g.writeDescriptor(d, BluetoothGattDescriptor.ENABLE_NOTIFICATION_VALUE) == BluetoothGatt.GATT_SUCCESS
                else @Suppress("DEPRECATION") run { d.value = BluetoothGattDescriptor.ENABLE_NOTIFICATION_VALUE; g.writeDescriptor(d) }
            }
        }
    }

    override suspend fun escribir(car: Car, datos: ByteArray) {
        val g = gatt ?: throw ErrorBle("No hay conexión.")
        val c = caracteristicas[car] ?: throw ErrorBle("Al bicho le falta la característica ${car.name}.")
        // Los bloques de config e imagen van sin respuesta (mucho más rápido); el resto, con.
        val bloque = (car == Car.CONFIG && datos[0].toInt() == Op.CONFIG_BLOQUE) || (car == Car.IMAGEN && datos[0].toInt() == Op.IMG_BLOQUE)
        val tipo = if (bloque) BluetoothGattCharacteristic.WRITE_TYPE_NO_RESPONSE else BluetoothGattCharacteristic.WRITE_TYPE_DEFAULT
        val status = operacion<Int> {
            if (Build.VERSION.SDK_INT >= 33) g.writeCharacteristic(c, datos, tipo) == BluetoothGatt.GATT_SUCCESS
            else @Suppress("DEPRECATION") run { c.writeType = tipo; c.value = datos; g.writeCharacteristic(c) }
        }
        if (status != BluetoothGatt.GATT_SUCCESS) throw ErrorBle("El bicho no aceptó la escritura (GATT $status).")
    }

    override suspend fun leerEstado(): ByteArray {
        val g = gatt ?: throw ErrorBle("No hay conexión.")
        val c = caracteristicas[Car.ESTADO] ?: throw ErrorBle("Al bicho le falta la característica de estado.")
        return operacion { g.readCharacteristic(c) }
    }

    override fun cerrar() {
        gatt?.disconnect()
        gatt?.close()
        gatt = null
    }
}
