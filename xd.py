import json
import logging
import re
import subprocess
from datetime import datetime
import pychromecast
from pychromecast.controllers.media import MediaController
import requests

# Configuración
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)
TIMEOUT = 10  # segundos

class ChromecastScanner:
    def __init__(self, ip, name):
        self.ip = ip
        self.name = name
        self.data = {
            "name": name,
            "ip": ip,
            "timestamp": datetime.now().isoformat(),
            "system": {},
            "network": {},
            "media": {},
            "volume": {},
            "android": {},
            "raw": {}
        }

    def scan_with_pychromecast(self):
        """Extrae datos via pychromecast"""
        try:
            cast = pychromecast.Chromecast(self.ip, timeout=TIMEOUT)
            cast.wait()

            # Información básica
            self.data["system"].update({
                "model": cast.model_name,
                "manufacturer": cast.cast_info.manufacturer,
                "uuid": str(cast.uuid),
                "firmware": cast.cast_info.cast_type,
                "status": cast.status.status_text
            })

            # Estado multimedia
            if cast.media_controller.status:
                self.data["media"] = {
                    "content_id": cast.media_controller.status.content_id,
                    "title": cast.media_controller.status.title,
                    "duration": cast.media_controller.status.duration,
                    "player_state": cast.media_controller.status.player_state
                }

            # Volumen
            self.data["volume"] = {
                "level": cast.status.volume_level,
                "muted": cast.status.volume_muted
            }

            # Red
            self.data["network"] = {
                "connected": cast.socket_client.is_connected,
                "ssl": cast.socket_client.ssl_enabled
            }

            cast.disconnect()
        except Exception as e:
            self.data["error_pychromecast"] = str(e)

    def scan_with_adb(self):
        """Extrae datos de Android TV via ADB"""
        try:
            # Comandos ADB (requiere adb conectado previamente)
            adb_commands = {
                "android_version": "adb shell getprop ro.build.version.release",
                "model": "adb shell getprop ro.product.model",
                "installed_apps": "adb shell pm list packages"
            }

            for key, cmd in adb_commands.items():
                result = subprocess.run(cmd, shell=True, capture_output=True, text=True)
                self.data["android"][key] = result.stdout.strip()

        except Exception as e:
            self.data["error_adb"] = f"ADB no disponible: {str(e)}"

    def scan_eureka_api(self):
        """Extrae datos de la API Eureka"""
        try:
            response = requests.get(f"http://{self.ip}:8008/setup/eureka_info", timeout=TIMEOUT)
            if response.status_code == 200:
                self.data["raw"]["eureka"] = response.json()

                # Procesar datos útiles de Eureka
                eureka = response.json()
                self.data["system"].update({
                    "build_version": eureka.get("build_version"),
                    "uptime": eureka.get("uptime"),
                    "hotspot": bool(eureka.get("hotspot_bssid"))
                })
        except requests.exceptions.RequestException as e:
            self.data["error_eureka"] = str(e)

    def get_all_data(self):
        """Ejecuta todos los scanners"""
        self.scan_with_pychromecast()
        self.scan_with_adb()
        self.scan_eureka_api()
        return self.data

def main():
    # Cargar dispositivos desde JSON
    with open("tvs_detectados.json") as f:
        devices = [d for d in json.load(f) if d.get("brand", "").lower() in ["chromecast", "androidtv", "googletv"]]

    results = []
    for device in devices:
        logger.info(f"Escaneando {device['name']} ({device['ip']})...")
        scanner = ChromecastScanner(device["ip"], device["name"])
        results.append(scanner.get_all_data())

    # Guardar reporte completo
    with open("chromecast_full_report.json", "w") as f:
        json.dump(results, f, indent=2, ensure_ascii=False)

    logger.info("¡Escaneo completado! Ver 'chromecast_full_report.json'")

if __name__ == "__main__":
    main()