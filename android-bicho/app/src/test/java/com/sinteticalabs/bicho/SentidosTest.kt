package com.sinteticalabs.bicho

import android.view.Surface
import org.junit.Assert.assertEquals
import org.junit.Test

/** El acelerómetro del celu (vertical) pasado a la pantalla tal como se ve. */
class SentidosTest {
    private fun ejes(x: Float, y: Float, rot: Int) = Sentidos.aPantalla(x, y, rot)
    private fun assertEquals(esperado: Pair<Float, Float>, real: Pair<Float, Float>) {
        assertEquals(esperado.first, real.first, 1e-4f)
        assertEquals(esperado.second, real.second, 1e-4f)
    }

    @Test fun verticalQuedaIgual() = assertEquals(1f to 2f, ejes(1f, 2f, Surface.ROTATION_0))

    @Test fun apaisadoParadoLaGravedadVaHaciaAbajoDeLaPantalla() {
        // Apaisado (rotación 90) y parado sobre su lado: el sensor mide +g en x;
        // en la pantalla eso es "arriba" (la reacción a la gravedad), y = +g.
        assertEquals(0f to 9.8f, ejes(9.8f, 0f, Surface.ROTATION_90))
    }

    @Test fun inclinadoALaDerechaDeLaPantalla() {
        // Apaisado: la derecha de la pantalla es el -y del sensor.
        assertEquals(9.8f to 0f, ejes(0f, -9.8f, Surface.ROTATION_90))
    }

    @Test fun lasCuatroRotacionesSonGiros() {
        for (r in listOf(Surface.ROTATION_0, Surface.ROTATION_90, Surface.ROTATION_180, Surface.ROTATION_270)) {
            val (a, b) = ejes(3f, 4f, r)
            assertEquals(25f, a * a + b * b, 1e-4f)
        }
    }
}
