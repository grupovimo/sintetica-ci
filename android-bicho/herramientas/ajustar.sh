#!/usr/bin/env bash
# Ajustes del celu para que sea solo un bicho (sin chip, enchufado, 24 h).
# Todo reversible desde Ajustes o con `settings put` de nuevo. Lo que tiene que
# sostener el dueño del dispositivo (hora automática, wifi siempre prendido,
# sin actualizaciones del sistema) lo hace además la app (Kiosco.kt).
set -u
g() { adb shell settings put global "$1" "$2"; }
s() { adb shell settings put secure "$1" "$2"; }
y() { adb shell settings put system "$1" "$2"; }

# Radio celular afuera: modo avión que no toca el wifi (no hay chip).
g airplane_mode_radios "cell,bluetooth,nfc,wimax"
g airplane_mode_toggleable_radios "bluetooth,wifi,nfc"
g airplane_mode_on 1
adb shell am broadcast -a android.intent.action.AIRPLANE_MODE --ez state true >/dev/null 2>&1
adb shell svc data disable

# Wifi siempre prendido, aunque la pantalla se apague; sin buscar redes de fondo.
g wifi_sleep_policy 2
g wifi_scan_always_enabled 0
g wifi_networks_available_notification_on 0

# Sin animaciones del sistema.
g window_animation_scale 0
g transition_animation_scale 0
g animator_duration_scale 0

# Ubicación apagada.
s location_mode 0
s location_providers_allowed -gps
s location_providers_allowed -network

# Silencio: solo suena el bicho (el despertador va por el canal de alarma).
# El silencio de timbre, notificaciones y sistema lo pone la app (Kiosco.kt).
y sound_effects_enabled 0
y haptic_feedback_enabled 0
y lockscreen_sounds_enabled 0
y dtmf_tone 0
g charging_sounds_enabled 0
s charging_sounds_enabled 0

# Hora y zona automáticas (el ritmo diario depende de la hora).
g auto_time 1
g auto_time_zone 1

# Sin ahorro de batería de Android para el bicho; prendido mientras esté enchufado.
adb shell dumpsys deviceidle whitelist +com.sinteticalabs.bicho >/dev/null
g stay_on_while_plugged_in 7

# Sin el "¿actualizar?" de la Play ni verificaciones de apps (no hay Play).
g package_verifier_enable 0
echo listo
