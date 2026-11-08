#!/usr/bin/env bash
# Deja el celu con lo mínimo para ser un bicho: apaga (no borra) las apps y
# servicios que no hacen falta, para que la batería y la RAM sean del bicho.
#   ./vaciar.sh          apaga la lista
#   ./vaciar.sh --volver las vuelve a prender
# Lo que NO se toca: el sistema (android, systemui, phone, providers), el
# instalador de paquetes, bluetooth, wifi, ajustes y el teclado (para cargar
# una red). El lanzador de Motorola se apaga aparte, cuando el bicho ya es la
# pantalla de inicio (ver README).
set -u
PAQUETES=(
  # Google: apps y servicios (ML Kit va con el modelo adentro, sin Play Services; el TTS de Google queda: lee el globo)
  com.android.chrome com.android.vending com.google.android.gms com.google.android.gsf
  com.google.android.gsf.login com.google.android.googlequicksearchbox com.google.android.youtube
  com.google.android.music com.google.android.videos com.google.android.apps.photos
  com.google.android.apps.docs com.google.android.apps.maps com.google.android.gm
  com.google.android.apps.messaging com.google.android.apps.tachyon com.google.android.calendar
  com.google.android.calculator com.google.android.deskclock com.google.android.gallery3d
  com.google.android.apps.cloudprint com.google.android.apps.wallpaper com.google.android.feedback
  com.google.android.backuptransport com.google.android.configupdater com.google.android.partnersetup
  com.google.android.syncadapters.contacts com.google.android.onetimeinitializer
  com.google.android.marvin.talkback com.google.android.printservice.recommendation
  com.google.android.setupwizard com.google.android.ims
  # Android: apps de celular que el bicho no usa
  com.android.dialer com.android.contacts com.android.calllogbackup com.android.mms.service
  com.android.cellbroadcastreceiver com.android.stk com.android.emergency com.android.facelock
  com.android.egg com.android.dreams.basic com.android.printspooler com.android.bookmarkprovider
  com.android.providers.partnerbookmarks com.android.wallpaper.livepicker com.android.wallpapercropper
  com.android.wallpaperbackup com.android.htmlviewer com.android.documentsui com.android.providers.downloads.ui
  com.android.providers.calendar com.android.providers.userdictionary com.android.sharedstoragebackup
  com.android.backupconfirm com.android.storagemanager com.android.bluetoothmidiservice
  com.android.omadm.service com.android.sdm.plugins.connmo com.android.sdm.plugins.dcmo
  com.android.sdm.plugins.diagmon com.android.sdm.plugins.sprintdm com.android.sprint.hiddenmenuapp
  com.verizon.omadm com.motorola.vzw.pco.extensions.pcoreceiver
  # Motorola y Lenovo: telemetría, actualizaciones, demo, extras
  com.motorola.actions com.motorola.moto com.motorola.motodisplay com.motorola.motodisplay.env
  com.motorola.motocare com.motorola.motocare.internal com.motorola.motocit com.motorola.bug2go
  com.motorola.ccc.checkin com.motorola.ccc.devicemanagement com.motorola.ccc.mainplm
  com.motorola.ccc.notification com.motorola.ccc.ota com.motorola.lifetimedata com.motorola.bach.modemstats
  com.motorola.demo com.motorola.retailconfig com.motorola.genie com.motorola.timeweatherwidget
  com.motorola.storageoptimizer com.motorola.motgeofencesvc com.motorola.fmplayer com.motorola.android.fmradio
  com.motorola.cameraone com.motorola.contacts.preloadcontacts com.motorola.frameworks.singlehand
  com.motorola.motokey com.motorola.programmenu com.motorola.entitlement com.motorola.appdirectedsmsproxy
  com.motorola.android.nativedropboxagent com.motorola.android.providers.chromehomepage
  com.motorola.android.settings.diag_mdlog com.motorola.android.settings.modemdebug
  com.motorola.setup com.motorola.onetimeinitializer com.aura.oobe.ml com.aura.oobe.motorola
  com.lenovo.lsf.user com.lmi.motorola.rescuesecurity
  # El lanzador: el bicho es la pantalla de inicio (dueño del dispositivo)
  com.motorola.launcher3 com.motorola.launcherconfig
  # De terceros
  com.whatsapp
)
accion="disable-user --user 0"
[ "${1:-}" = "--volver" ] && accion="enable"
for p in "${PAQUETES[@]}"; do
  r=$(adb shell pm $accion "$p" 2>&1 | tr -d '\r')
  echo "$p: $r"
done
