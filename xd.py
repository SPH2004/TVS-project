import subprocess
import re
import socket
import pychromecast
import netifaces

def get_local_ip():
    for iface in netifaces.interfaces():
        addrs = netifaces.ifaddresses(iface)
        if netifaces.AF_INET in addrs:
            for addr in addrs[netifaces.AF_INET]:
                ip = addr['addr']
                if ip.startswith("192.168.") or ip.startswith("10.") or ip.startswith("172."):
                    return ip
    return None

def scan_network():
    print("[*] Escaneando la red local...")
    try:
        output = subprocess.check_output(['sudo', 'arp-scan', '--localnet']).decode()
    except Exception as e:
        print(f"Error al ejecutar arp-scan: {e}")
        return []

    devices = []
    for line in output.splitlines():
        match = re.match(r"(\d+\.\d+\.\d+\.\d+)\s+([0-9A-F:]+)\s+(.+)", line)
        if match:
            ip, mac, vendor = match.groups()
            devices.append({'ip': ip, 'mac': mac, 'vendor': vendor})
    return devices

def detect_hostname(ip):
    try:
        return socket.gethostbyaddr(ip)[0]
    except:
        return None

def detect_chromecast():
    chromecasts, _ = pychromecast.get_chromecasts()
    tvs = []
    for cc in chromecasts:
        info = {
            'ip': cc.host,
            'marca': 'Google',
            'modelo': cc.device.friendly_name,
            'os': 'Chromecast (Cast OS)',
            'tipo': 'Smart TV / Dongle'
        }
        tvs.append(info)
    return tvs

def main():
    print("🎯 Detectando televisores conectados...\n")
    local_ip = get_local_ip()
    if not local_ip:
        print("No se pudo determinar la IP local.")
        return

    tvs_detected = []

    # Parte 1: Chromecast
    chromecast_tvs = detect_chromecast()
    tvs_detected.extend(chromecast_tvs)

    # Parte 2: Escaneo general
    network_devices = scan_network()
    for device in network_devices:
        ip = device['ip']
        vendor = device['vendor']
        mac = device['mac']
        hostname = detect_hostname(ip)

        # Heurística: si el vendor contiene marcas conocidas
        known_brands = ["LG", "Samsung", "Sony", "Philips", "Panasonic", "TCL", "Hisense", "Sharp", "Roku"]
        if any(brand in vendor for brand in known_brands):
            so = "Desconocido"
            if "LG" in vendor:
                so = "webOS"
            elif "Samsung" in vendor:
                so = "Tizen"
            elif "Sony" in vendor or "Philips" in vendor:
                so = "Android TV"
            elif "Roku" in vendor:
                so = "Roku OS"

            tvs_detected.append({
                'ip': ip,
                'marca': vendor,
                'modelo': hostname or "Sin nombre",
                'os': so,
                'tipo': 'Smart TV'
            })

    # Mostrar resultados
    if tvs_detected:
        print("\n📺 Televisores detectados en la red:\n")
        for i, tv in enumerate(tvs_detected, 1):
            print(f"{i}. IP: {tv['ip']}")
            print(f"   Marca: {tv['marca']}")
            print(f"   Modelo: {tv['modelo']}")
            print(f"   Sistema Operativo: {tv['os']}")
            print(f"   Tipo: {tv['tipo']}\n")
    else:
        print("No se detectaron televisores.")

if __name__ == "__main__":
    main()
