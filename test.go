package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

func getLocalNetwork() (net.IP, *net.IPNet, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			var network *net.IPNet

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
				network = v
			case *net.IPAddr:
				ip = v.IP
				network = &net.IPNet{IP: v.IP, Mask: v.IP.DefaultMask()}
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			ip = ip.To4()
			if ip == nil {
				continue // no es IPv4
			}

			return ip, network, nil
		}
	}

	return nil, nil, fmt.Errorf("no se pudo determinar la red local")
}

func getIPsInNetwork(network *net.IPNet) []net.IP {
	var ips []net.IP
	ip := network.IP.Mask(network.Mask)

	for ip := ip.Mask(network.Mask); network.Contains(ip); inc(ip) {
		ips = append(ips, net.IP{ip[0], ip[1], ip[2], ip[3]})
	}

	// Eliminar dirección de red y broadcast
	if len(ips) > 2 {
		return ips[1 : len(ips)-1]
	}
	return ips
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func scanARP(ip net.IP) (net.HardwareAddr, error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("arping", "-c", "1", "-I", "eth0", ip.String())
	case "darwin":
		cmd = exec.Command("arping", "-c", "1", "-I", "en0", ip.String())
	case "windows":
		cmd = exec.Command("arp", "-a", ip.String())
	default:
		return nil, fmt.Errorf("sistema operativo no soportado")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	if runtime.GOOS == "windows" {
		// Parsear salida de ARP en Windows
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, ip.String()) {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					mac, err := net.ParseMAC(parts[1])
					if err == nil {
						return mac, nil
					}
				}
			}
		}
	} else {
		// Parsear salida de arping en Linux/macOS
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Unicast reply from") {
				parts := strings.Fields(line)
				for _, part := range parts {
					if mac, err := net.ParseMAC(part); err == nil {
						return mac, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("no se encontró dirección MAC")
}

func main() {
	fmt.Println("Escaneando dispositivos en la red local...")

	localIP, network, err := getLocalNetwork()
	if err != nil {
		fmt.Printf("Error al obtener la red local: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Tu IP: %s\nRed local: %s\nMáscara: %s\n\n", 
		localIP, network.IP, net.IP(network.Mask))

	ips := getIPsInNetwork(network)
	fmt.Printf("Escaneando %d direcciones IP...\n", len(ips))

	var wg sync.WaitGroup
	var mutex sync.Mutex
	activeDevices := make(map[string]string)

	concurrencyLimit := 50
	semaphore := make(chan struct{}, concurrencyLimit)

	for _, ip := range ips {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(ip net.IP) {
			defer wg.Done()
			defer func() { <-semaphore }()

			mac, err := scanARP(ip)
			if err == nil {
				mutex.Lock()
				activeDevices[ip.String()] = mac.String()
				mutex.Unlock()
			}
		}(ip)
	}

	wg.Wait()

	fmt.Println("\nDispositivos encontrados en la red:")
	fmt.Println("IP\t\tMAC Address")
	for ip, mac := range activeDevices {
		fmt.Printf("%s\t%s\n", ip, mac)
	}
}