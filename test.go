package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	NetworkPrefix  = "192.168.18." // Ajusta a tu red
	ScanTimeout    = 150 * time.Millisecond
	RequestTimeout = 1 * time.Second
	MaxWorkers     = 200
)

var commonPorts = []int{
	8008, 8009, // Chromecast, Android TV
	7676, 8001, 8002, // Samsung
	8060,             // Roku
	3000, 3001, 3002, // LG WebOS
	5555, 5353, // Fire TV, ADB
}

type Device struct {
	IP     string `json:"ip"`
	Name   string `json:"name"`
	Brand  string `json:"brand"`
	Status string `json:"status"`
}

var (
	devices sync.Map
	wg      sync.WaitGroup
	workers = make(chan struct{}, MaxWorkers)
)

func scanPort(ctx context.Context, ip string, port int) {
	defer wg.Done()
	<-workers

	select {
	case <-ctx.Done():
		return
	default:
		address := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", address, ScanTimeout)
		if err != nil {
			return
		}
		conn.Close()

		val, _ := devices.LoadOrStore(ip, &Device{IP: ip})
		dev := val.(*Device)
		devices.Store(ip, dev)
	}
}

func detectTVType(ip string) *Device {
	client := http.Client{Timeout: RequestTimeout}

	// Chromecast
	resp, err := client.Get("http://" + ip + ":8008/setup/eureka_info")
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		var data map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&data)
		name, _ := data["name"].(string)
		return &Device{IP: ip, Brand: "Chromecast", Name: name, Status: "Active"}
	}

	// Roku
	resp, err = client.Get("http://" + ip + ":8060/query/device-info")
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "<device-info") {
			return &Device{IP: ip, Brand: "Roku", Name: "Roku TV", Status: "Active"}
		}
	}

	// LG WebOS
	for _, port := range []int{3000, 3001, 3002} {
		address := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", address, ScanTimeout)
		if err == nil {
			conn.Close()
			return &Device{IP: ip, Brand: "LG WebOS", Name: "LG TV", Status: "Active"}
		}
	}

	// Samsung
	for _, port := range []int{7676, 8001, 8002} {
		address := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", address, ScanTimeout)
		if err == nil {
			conn.Close()
			return &Device{IP: ip, Brand: "Samsung", Name: "Samsung TV", Status: "Active"}
		}
	}

	// Fire TV / Android (con ADB abierto)
	conn, err := net.DialTimeout("tcp", ip+":5555", ScanTimeout)
	if err == nil {
		conn.Close()
		return &Device{IP: ip, Brand: "Android/Fire TV", Name: "ADB Device", Status: "Active"}
	}

	return &Device{IP: ip, Brand: "Unknown", Name: "Smart TV", Status: "Detected"}
}

func processDevices() []*Device {
	var tvs []*Device
	devices.Range(func(_, val any) bool {
		ip := val.(*Device).IP
		tv := detectTVType(ip)
		tvs = append(tvs, tv)
		return true
	})

	sort.Slice(tvs, func(i, j int) bool {
		return tvs[i].IP < tvs[j].IP
	})
	return tvs
}

func exportToJSON(devices []*Device, filename string) error {
	data, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

loop:
	for i := 1; i <= 254; i++ {
		ip := fmt.Sprintf("%s%d", NetworkPrefix, i)
		for _, port := range commonPorts {
			select {
			case <-ctx.Done():
				break loop // Salimos de ambos bucles
			case workers <- struct{}{}:
				wg.Add(1)
				go scanPort(ctx, ip, port)
			}
		}
	}

	wg.Wait()

	tvs := processDevices()
	if err := exportToJSON(tvs, "tvs_detectados.json"); err != nil {
		fmt.Println("Error exportando:", err)
	} else {
		fmt.Printf("✅ TVs detectadas: %d\n", len(tvs))
	}
}
