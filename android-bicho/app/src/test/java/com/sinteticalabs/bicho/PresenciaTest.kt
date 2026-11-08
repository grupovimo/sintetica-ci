package com.sinteticalabs.bicho

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PresenciaTest {
    private val min = 60_000L

    @Test fun alArrancarNoAvisa() {
        val p = Presente()
        assertNull(p.tick(0))
        assertNull(p.visto(min))  // ya estaba
        assertNull(p.tick(10 * min))
    }

    @Test fun seVaYVuelve() {
        val p = Presente()
        p.tick(0); p.visto(0)
        assertNull(p.tick(14 * min))
        assertEquals(false, p.tick(16 * min))
        assertNull(p.tick(30 * min))  // una sola vez
        assertEquals(true, p.visto(40 * min))
        assertNull(p.visto(41 * min))
    }

    @Test fun siNoLoVeAlArrancarNoEsta() {
        val p = Presente()
        p.tick(0)
        assertNull(p.tick(16 * min))  // no estaba: no se "fue"
        assertEquals(true, p.visto(20 * min))
    }

    @Test fun tablaArp() {
        val t = """IP address       HW type     Flags       HW address            Mask     Device
192.168.100.1    0x1         0x2         aa:bb:cc:dd:ee:01     *        wlan0
192.168.100.7    0x1         0x0         aa:bb:cc:dd:ee:07     *        wlan0
"""
        assertTrue(Presente.enArp(t, "AA:BB:CC:DD:EE:01"))
        assertFalse(Presente.enArp(t, "aa:bb:cc:dd:ee:07"))
        assertFalse(Presente.enArp(t, "aa:bb:cc:dd:ee:99"))
    }
}
