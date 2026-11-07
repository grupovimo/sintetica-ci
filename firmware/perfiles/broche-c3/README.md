# Broche (perfil `broche-c3`)

ESP32-C3 SuperMini + pantalla redonda GC9A01 de 1,28" (240 × 240) + LiPo
602030 de 300 mAh + cargador TP4056 + interruptor deslizante + pulsador.

## Cableado

| Desde | Hasta | Cable |
| --- | --- | --- |
| Batería + | TP4056 B+ | Rojo |
| Batería − | TP4056 B− | Negro |
| TP4056 OUT+ | Interruptor → C3 5V | Rojo |
| TP4056 OUT− | C3 GND | Negro |
| C3 3V3 | Pantalla VCC | Rojo |
| C3 GND | Pantalla GND | Negro |
| C3 GPIO4 | Pantalla SCL | Señal |
| C3 GPIO6 | Pantalla SDA | Señal |
| C3 GPIO10 | Pantalla DC | Señal |
| C3 GPIO7 | Pantalla CS | Señal |
| C3 GPIO3 | Pantalla RST | Señal |
| C3 GPIO1 | Pantalla BLK (luz de fondo, PWM) | Señal |
| C3 GPIO5 | Pulsador → GND (pull-up interno) | Señal |
| **TP4056 B+** | **100 kΩ → C3 GPIO0** | **Señal (nuevo: batería)** |
| **C3 GPIO0** | **100 kΩ → GND** | **Señal (nuevo: batería)** |

El divisor deja la mitad de la tensión de la batería en el GPIO0 (ADC). Gasta
unos 20 µA siempre; si hace falta bajar el consumo dormido, se corta con un pin
más (queda para cuando se mida).

## Grabar

```sh
cd firmware
pio run -e broche-c3 -t upload -t monitor
```

Por el monitor serie: `serie 001` graba el número de unidad.

## Uso

- **Toque:** lo siguiente de la lista (sin lista, recorre las 16 caras).
- **Mantener 2 s:** se duerme (pantalla apagada y sueño profundo).
- **Un toque dormido:** despierta (arranca de nuevo, sigue donde estaba).
- **Mantener 8 s:** muestra el QR para vincular otro celu.

## Memoria

Sin PSRAM, la pantalla se dibuja de a franjas de 40 filas (19 KB). La RAM
estática usada es el 68% (223 KB de 327 KB); quedan unos 100 KB para BLE y
LittleFS. La flash tiene 1,9 MB para el programa y 2 MB para imágenes
(`particiones.csv`).

## Falta verificar con la placa armada

- [ ] Rotación e inversión de colores del GC9A01 (`invert = true`).
- [ ] Que el pulsador despierte del sueño profundo.
- [ ] Consumo dormido < 0,3 mA (sacarle a la SuperMini el LED rojo de encendido).
- [ ] La lectura de la batería con el divisor.
- [ ] Que BLE + LittleFS entren en la RAM que queda.
