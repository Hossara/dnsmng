package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

type Config struct {
	DNS map[string][]string `yaml:"dns"`
}

func readConfig(filePath string) (*Config, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func setDNS(dnsIPs []string) error {
	// Create the content for /etc/resolv.conf with multiple nameservers
	var content strings.Builder
	for _, ip := range dnsIPs {
		content.WriteString(fmt.Sprintf("nameserver %s\n", ip))
	}

	// Overwrite /etc/resolv.conf
	return ioutil.WriteFile("/etc/resolv.conf", []byte(content.String()), 0644)
}

func main() {
	configPath := flag.String("config", "/etc/dnsmng/config.yaml", "Path to the config file")
	domain := flag.String("set", "local", "DNS name to set (e.g. google, cloudflare)")
	flag.Parse()

	// Read the configuration
	config, err := readConfig(*configPath)
	if err != nil {
		fmt.Printf("Error reading config file: %s\n", err)
		os.Exit(1)
	}

	// Get the DNS IPs for the specified domain
	dnsIPs, exists := config.DNS[*domain]
	if !exists {
		fmt.Printf("DNS entry for '%s' not found in config\n", *domain)
		os.Exit(1)
	}

	// Set the DNS in /etc/resolv.conf
	err = setDNS(dnsIPs)
	if err != nil {
		fmt.Printf("Error setting DNS: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("DNS set to %v for domain '%s'\n", dnsIPs, *domain)
}
