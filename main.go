package main

import (
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type SubnetResult struct {
	IPAddress        string
	SubnetMask       string
	NetworkAddress   string
	BroadcastAddress string
	MinHostAddress   string
	MaxHostAddress   string
	UsableHosts      string
	Error            string
}

// loadTemplate loads and parses the HTML template from file
func loadTemplate() (*template.Template, error) {
	templateData, err := os.ReadFile("index.html")
	if err != nil {
		return nil, fmt.Errorf("failed to read index.html: %v", err)
	}

	tmpl, err := template.New("subnet").Parse(string(templateData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %v", err)
	}

	return tmpl, nil
}

// isValidSubnetMask validates that the IP mask has contiguous 1s followed by contiguous 0s
func isValidSubnetMask(mask net.IPMask) bool {
	// Convert mask to 32-bit integer
	maskInt := uint32(mask[0])<<24 | uint32(mask[1])<<16 | uint32(mask[2])<<8 | uint32(mask[3])

	// Find the number of leading 1s
	leadingOnes := 0
	for i := 31; i >= 0; i-- {
		if maskInt&(1<<uint(i)) != 0 {
			leadingOnes++
		} else {
			break
		}
	}

	// Check if remaining bits are all 0s
	expectedMask := uint32(0xFFFFFFFF) << uint(32-leadingOnes)
	return maskInt == expectedMask
}

// parseSubnetMask parses subnet mask in either dotted decimal or CIDR notation
func parseSubnetMask(mask string) (net.IPMask, error) {
	mask = strings.TrimSpace(mask)

	// Handle CIDR notation (e.g., /24)
	if strings.HasPrefix(mask, "/") {
		cidr, err := strconv.Atoi(mask[1:])
		if err != nil || cidr < 0 || cidr > 32 {
			return nil, fmt.Errorf("invalid CIDR notation: %s", mask)
		}
		return net.CIDRMask(cidr, 32), nil
	}

	// Handle dotted decimal notation (e.g., 255.255.255.0)
	ip := net.ParseIP(mask)
	if ip == nil {
		return nil, fmt.Errorf("invalid subnet mask format: %s", mask)
	}

	ipv4 := ip.To4()
	if ipv4 == nil {
		return nil, fmt.Errorf("not a valid IPv4 mask: %s", mask)
	}

	subnetMask := net.IPMask(ipv4)

	// Validate that it's a proper subnet mask (contiguous 1s followed by 0s)
	if !isValidSubnetMask(subnetMask) {
		return nil, fmt.Errorf("invalid subnet mask: %s (must have contiguous 1s followed by 0s)", mask)
	}

	return subnetMask, nil
}

// calculateSubnet performs the subnet calculations
func calculateSubnet(ipStr, maskStr string) (*SubnetResult, error) {
	// Parse IP address
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	ipv4 := ip.To4()
	if ipv4 == nil {
		return nil, fmt.Errorf("not a valid IPv4 address: %s", ipStr)
	}

	// Parse subnet mask
	mask, err := parseSubnetMask(maskStr)
	if err != nil {
		return nil, err
	}

	// Get CIDR prefix length for corner case handling
	prefixLen, _ := mask.Size()

	// Create network
	network := &net.IPNet{
		IP:   ipv4.Mask(mask),
		Mask: mask,
	}

	// Calculate network address (first IP in subnet)
	networkAddr := network.IP

	// Calculate broadcast address (last IP in subnet)
	broadcastAddr := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcastAddr[i] = networkAddr[i] | ^mask[i]
	}

	result := &SubnetResult{
		NetworkAddress:   networkAddr.String(),
		BroadcastAddress: broadcastAddr.String(),
	}

	// Handle corner cases based on prefix length
	switch prefixLen {
	case 32:
		// /32: Single host, network = broadcast = entered IP
		// No usable host addresses
		result.NetworkAddress = ipv4.String()
		result.BroadcastAddress = ipv4.String()
		result.MinHostAddress = "N/A"
		result.MaxHostAddress = "N/A"
		result.UsableHosts = "0"

	case 31:
		// /31: Point-to-point link (RFC 3021)
		// No usable host addresses in traditional sense
		result.MinHostAddress = "N/A"
		result.MaxHostAddress = "N/A"
		result.UsableHosts = "0"

	default:
		// Normal subnets: calculate min/max host addresses
		// Calculate min host address (network + 1)
		minHostAddr := make(net.IP, 4)
		copy(minHostAddr, networkAddr)
		// Add 1 to the network address
		for i := 3; i >= 0; i-- {
			if minHostAddr[i] < 255 {
				minHostAddr[i]++
				break
			}
			minHostAddr[i] = 0
		}

		// Calculate max host address (broadcast - 1)
		maxHostAddr := make(net.IP, 4)
		copy(maxHostAddr, broadcastAddr)
		// Subtract 1 from the broadcast address
		for i := 3; i >= 0; i-- {
			if maxHostAddr[i] > 0 {
				maxHostAddr[i]--
				break
			}
			maxHostAddr[i] = 255
		}

		result.MinHostAddress = minHostAddr.String()
		result.MaxHostAddress = maxHostAddr.String()

		// Calculate number of usable hosts
		// Total hosts in subnet = 2^(32-prefix) - 2 (network and broadcast)
		totalHosts := 1 << uint(32-prefixLen)
		usableHosts := totalHosts - 2
		if usableHosts < 0 {
			usableHosts = 0
		}
		result.UsableHosts = fmt.Sprintf("%d", usableHosts)
	}

	return result, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := loadTemplate()
	if err != nil {
		log.Printf("Template loading error: %v", err)
		http.Error(w, "Template loading error", http.StatusInternalServerError)
		return
	}

	result := &SubnetResult{}

	if r.Method == http.MethodPost {
		ip := strings.TrimSpace(r.FormValue("ip"))
		mask := strings.TrimSpace(r.FormValue("mask"))

		result.IPAddress = ip
		result.SubnetMask = mask

		if ip != "" && mask != "" {
			calcResult, err := calculateSubnet(ip, mask)
			if err != nil {
				result.Error = err.Error()
			} else {
				result.NetworkAddress = calcResult.NetworkAddress
				result.BroadcastAddress = calcResult.BroadcastAddress
				result.MinHostAddress = calcResult.MinHostAddress
				result.MaxHostAddress = calcResult.MaxHostAddress
				result.UsableHosts = calcResult.UsableHosts
			}
		}
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, result); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Template execution error", http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/", handler)

	// Get port from environment variable, default to 8080
	port := os.Getenv("GO_SUBNET_CALCULATOR_PORT")
	if port == "" {
		port = "8080"
	}

	// Validate port is numeric
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatalf("Invalid port number: %s", port)
	}

	address := ":" + port
	fmt.Printf("IPv4 Subnet Calculator starting on http://localhost:%s\n", port)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
