#!/usr/bin/env python3
"""Suma el bicho-celu a una cuenta de una nube (por defecto, producción).

El Moto no tiene BLE para la app: la Mac hace de app. Entra con Telegram,
reclama el robot y le pasa las credenciales MQTT por su puerta HTTP (:8090),
como la API local con los bichos virtuales.

  python3 herramientas/sumar.py                      # producción, el celu en 192.168.100.20
  python3 herramientas/sumar.py --celu 192.168.1.50 --api http://localhost:8080 --token <sesión>
"""
import argparse
import base64
import json
import re
import sys
import time
import urllib.error
import urllib.request

PRODUCCION = "https://api-sintetica.grupovimo.com.ar"


def pedir(metodo, url, cuerpo=None, cabeceras=None, espera=40):
    datos = json.dumps(cuerpo).encode() if cuerpo is not None else None
    req = urllib.request.Request(url, data=datos, method=metodo,
                                 headers={"Content-Type": "application/json", **(cabeceras or {})})
    try:
        with urllib.request.urlopen(req, timeout=espera) as r:
            texto = r.read().decode()
            return r.status, json.loads(texto) if texto else {}
    except urllib.error.HTTPError as e:
        texto = e.read().decode()
        try:
            return e.code, json.loads(texto)
        except ValueError:
            return e.code, {"error": texto}


def entrar_con_telegram(api):
    estado, e = pedir("POST", api + "/v1/sesion/telegram", {"cliente": "app"})
    if estado != 201:
        sys.exit(f"No pude pedir la entrada con Telegram: {e}")
    print(f"\n1. Abrí este link en el celu y tocá Start en el bot:\n\n   {e['enlace']}\n")
    cab = {"X-Sintetica-Secreto": e["secreto"]}
    hasta = time.time() + 10 * 60
    while time.time() < hasta:
        estado, r = pedir("GET", f"{api}/v1/sesion/telegram/{e['token']}?esperar=25", cabeceras=cab)
        if estado == 200 and r.get("token"):
            print("   Listo, entraste.")
            return r["token"]
        if estado == 200 and r.get("estado") == "confirmar":
            codigo = input("   El bot te mandó un código: escribilo acá: ").strip()
            pedir("POST", f"{api}/v1/sesion/telegram/{e['token']}/codigo", {"codigo": codigo}, cab)
            continue
        if estado not in (200,):
            sys.exit(f"La entrada con Telegram falló: {r}")
    sys.exit("Se venció la entrada con Telegram (10 min): corré el script de nuevo.")


def corto(s):
    b = s.encode()
    return bytes([len(b)]) + b


def main():
    a = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    a.add_argument("--api", default=PRODUCCION)
    a.add_argument("--celu", default="192.168.100.20", help="la IP del Moto en la red de casa")
    a.add_argument("--token", help="una sesión ya abierta (si no, entra con Telegram)")
    o = a.parse_args()
    api, puerta = o.api.rstrip("/"), f"http://{o.celu}:8090"

    estado, info = pedir("GET", puerta + "/info", espera=5)
    if estado != 200:
        sys.exit(f"No encuentro al bicho en {puerta}: ¿está prendido y en el mismo wifi?")
    bid, perfil = info["id"], info.get("perfil", "celu")
    print(f"El bicho: {bid} (perfil {perfil}).")

    token = o.token or entrar_con_telegram(api)
    sesion = {"Authorization": "Bearer " + token}

    estado, r = pedir("POST", api + "/v1/bichos/reclamar", {"id": bid, "perfil": perfil}, sesion)
    if estado != 200:
        sys.exit(f"La nube no me dio las credenciales: {r}")
    m = r["mqtt"]

    # El código del QR solo vale mientras lo muestra: se lo pedimos y se vincula con él.
    _, q = pedir("POST", puerta + "/mostrar-qr", {}, espera=10)
    qr = re.match(r"SINTETICA:V1/([0-9A-F]{12})/(\w+)", q.get("qr", ""))
    if not qr:
        sys.exit(f"El bicho no mostró el QR: {q}")
    escrituras = [
        {"car": 2, "datos": base64.b64encode(b"\x13" + qr.group(2).encode()).decode()},
        {"car": 2, "datos": base64.b64encode(b"\x17" + corto(m["url"]) + corto(m["usuario"]) + corto(m["clave"])).decode()},
    ]
    estado, r = pedir("POST", puerta + "/ble", {"escrituras": escrituras}, espera=15)
    if estado != 200:
        sys.exit(f"El bicho no aceptó las credenciales: {r}")
    for n in r.get("notificaciones", []):
        d = base64.b64decode(n.get("datos", ""))
        if len(d) >= 3 and d[0] in (0x93, 0x97) and d[1] != 0:
            sys.exit(f"El bicho dijo que no ({d[0]:#x}, error {d[1]:#x}).")

    print("2. Le pasé las credenciales; espero que se conecte a la nube…")
    estado, r = pedir("GET", f"{api}/v1/bichos/{bid}/reclamo?esperar=45", cabeceras=sesion, espera=60)
    if estado == 200 and r.get("sumado"):
        print(f"\n¡Listo! {bid} está en tu cuenta de {api}. Abrí la app y aparece en Mis robots.")
    else:
        sys.exit("Todavía no se conectó. Mirá el registro del celu (adb logcat -s Bicho) y probá de nuevo.")


if __name__ == "__main__":
    main()
