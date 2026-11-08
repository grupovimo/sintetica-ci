# android-bicho

Un celu Android viejo convertido en un bicho de escritorio: la cara a pantalla
completa las 24 h, los sensores del celu como sentidos y el celu entero
dedicado al bicho (kiosco como dueño del dispositivo).

La cara y el motor de vida **no** están acá: son el núcleo C++ de
`firmware/nucleo/` (el mismo de la placa, la web y el simulador), compilado
con el NDK. Esta app solo pone el anfitrión:

| Archivo | Qué hace |
|---|---|
| `app/src/main/cpp/puente.cpp` | JNI ↔ API C (`firmware/api/sintetica_c.h`), el hal (hora, guardar, batería) y la subida del cuadro a GL |
| `Lienzo.kt` | GL ES 2.0: el núcleo dibuja a 640×360 (RGB565), se sube como textura y se estira a 1280×720; tope de 30 fps |
| `Sentidos.kt` | Acelerómetro (en g, ejes de la pantalla), batería y cargador, luz → brillo y `oscuridad`, proximidad → `mano-cerca`, ojos → la mirada y `cara-detectada`, oído |
| `Voz.kt` | La voz de criatura: los sonidos del núcleo (`despertador:1..5`, `dado`, `ronquido`…) sintetizados, sin archivos; el tono según `voz.tipo` |
| `Avisos.kt` | Las notificaciones del propio celu como comando Aviso (`0x44`): la app y quién, nada del texto |
| `Oido.kt` | Del micrófono solo el nivel (dBFS): voz, ruido, música, silencio largo. No guarda audio |
| `Kiosco.kt` | Dueño del dispositivo: pantalla de inicio, lock task, sin barra ni bloqueo, prendido enchufado |
| `Secreto.kt` | La salida secreta |
| `Depuracion.kt`, `Datos.kt` | Eventos y tiempo por adb, y la superposición de datos |
| `Guardado.kt` | Lo que guarda el núcleo (config, claves, memoria de la relación) en archivos, con escritura atómica |

## Compilar e instalar

Necesita el Android SDK con el NDK 27.2.12479018 y CMake 3.22.1
(`sdkmanager "ndk;27.2.12479018" "cmake;3.22.1"`) y Java 17.

```sh
cd android-bicho
./gradlew assembleDebug testDebugUnitTest
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

Por wifi (para dejarlo en el cargador de pared): con el cable puesto,
`adb tcpip 5555` y después `adb connect <ip>:5555`.

## Preparar el celu (una vez)

1. Sacar todas las cuentas (Ajustes → Cuentas): con cuentas no se puede ser
   dueño del dispositivo.
2. `adb shell dpm set-device-owner com.sinteticalabs.bicho/.Admin`
3. `./herramientas/vaciar.sh`: apaga (no borra) todo lo que no hace falta,
   lanzador incluido. `./herramientas/vaciar.sh --volver` lo prende de nuevo.
4. `./herramientas/ajustar.sh`: celu dedicado (sin chip): modo avión con wifi,
   wifi siempre prendido, sin animaciones, sin ubicación ni sonidos del
   sistema, hora automática, el bicho fuera del ahorro de batería. La app
   además sostiene la hora automática, el wifi, el bluetooth apagado, el
   silencio y posterga las actualizaciones del sistema.
5. Vincularlo: `adb shell am broadcast -a sintetica.EVENT --es tipo vincular`
   (provisorio, hasta que hable con la nube).
6. Los avisos de notificaciones: `adb shell settings put secure
   enabled_notification_listeners com.sinteticalabs.bicho/com.sinteticalabs.bicho.Avisos`.

7. La presencia (si estás en casa): el celu del dueño por wifi y/o Bluetooth.
   `adb shell am broadcast -a sintetica.EVENT --es tipo duenio --es wifi
   <MAC del celu en el wifi> --es bt <MAC de Bluetooth>`. Con `bt`, el
   bluetooth del bicho queda prendido; para emparejarlos, la primera vez, desde
   los ajustes (salida secreta). Si se va 15 min, "te fuiste"; al volver, "llegaste".

Con el celu enchufado las 24 h, conviene un enchufe con horario (que cargue
unas horas y descanse otras): sin root no se puede limitar la carga, y una
batería vieja siempre al 100 % se gasta y se puede hinchar.

## Salida secreta

5 toques en la esquina de arriba a la izquierda y después mantener apretado
2 s ahí mismo. Abre un menú: volver, ver datos, salir del kiosco (abre los
ajustes; al abrir el bicho de nuevo vuelve el kiosco) y soltar el celu (deja
de ser dueño del dispositivo; para devolverlo a la normalidad, después
`vaciar.sh --volver`).

## Depurar

```sh
adb shell am broadcast -a sintetica.EVENT --es tipo sacudida-fuerte  # cualquier evento por su nombre del JSON
adb shell am broadcast -a sintetica.EVENT --es tipo depuracion        # datos en pantalla
adb shell am broadcast -a sintetica.EVENT --es tipo tiempo --ei x 720 # un día en 2 minutos (x 1 = normal)
adb shell am broadcast -a sintetica.EVENT --es tipo comando --es hex 4202  # un comando (acá, sonar el despertador; 4200 lo apaga, 4303 el dado)
adb logcat -s Bicho SinteticaOS
adb exec-out screencap -p > captura.png
```

## Privacidad

El micrófono se lee en tramos de 50 ms de los que solo se mide el nivel; el
tramo se borra enseguida. Nada se graba, se guarda ni sale del celu. De la
cámara solo sale dónde está la cara (para mirarte); de las notificaciones,
solo la app y el título.
