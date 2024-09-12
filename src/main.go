package main

import (
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type Config struct {
	DNS map[string][]string `yaml:"dns"`
}

const (
	lastDNSFilePath = "/var/lib/dnsmng/last_dns" // File to store the last set DNS
	resolvConfPath  = "/etc/resolv.conf"         // Path to resolv.conf
)

func readConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
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
	return os.WriteFile("/etc/resolv.conf", []byte(content.String()), 0644)
}

// Read the last saved DNS name from a file
func readLastDNS() (string, error) {
	data, err := os.ReadFile(lastDNSFilePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Save the last selected DNS name to a file
func saveLastDNS(dnsName string) error {
	dir := filepath.Dir(lastDNSFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(lastDNSFilePath, []byte(dnsName), 0644)
}

// Watch /etc/resolv.conf for changes and restore the DNS settings if modified
func watchResolvConf(dnsIPs []string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("Detected change in /etc/resolv.conf, restoring DNS settings")
					err = setDNS(dnsIPs)
					if err != nil {
						log.Printf("Error restoring DNS: %s\n", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("Error:", err)
			}
		}
	}()

	err = watcher.Add(resolvConfPath)
	if err != nil {
		log.Fatal(err)
	}

	<-done
}

func main() {
	configPath := flag.String("config", "/etc/dnsmng/config.yaml", "Path to the config file")
	domain := flag.String("set", "", "DNS name to set (e.g. google, cloudflare)")
	flag.Parse()

	// Read the configuration
	config, err := readConfig(*configPath)
	if err != nil {
		fmt.Printf("Error reading config file: %s\n", err)
		os.Exit(1)
	}

	// If a DNS domain is specified, set it and save it as the last DNS
	if *domain != "" {
		dnsIPs, exists := config.DNS[*domain]
		if !exists {
			log.Fatalf("DNS entry for '%s' not found in config\n", *domain)
		}

		// Set the DNS in resolv.conf
		err = setDNS(dnsIPs)
		if err != nil {
			log.Fatalf("Error setting DNS: %s\n", err)
		}

		// Save the last set DNS name
		err = saveLastDNS(*domain)
		if err != nil {
			log.Fatalf("Error saving last DNS: %s\n", err)
		}

		log.Printf("DNS set to %v for domain '%s'\n", dnsIPs, *domain)
	} else {
		// If no domain is specified, read the last DNS and set it on startup
		lastDNS, err := readLastDNS()
		if err != nil || lastDNS == "" {
			log.Println("No previous DNS set or file missing, defaulting to 'local' DNS")

			// Set 'local' as the default DNS
			lastDNS = "local"
		}

		// Set the last used DNS
		dnsIPs, exists := config.DNS[lastDNS]
		if !exists {
			log.Fatalf("DNS entry for '%s' not found in config\n", lastDNS)
		}

		err = setDNS(dnsIPs)
		if err != nil {
			log.Fatalf("Error setting DNS: %s\n", err)
		}

		log.Printf("Restored last DNS: %s\n", lastDNS)
	}

	// Watch /etc/resolv.conf for changes
	dnsIPs, exists := config.DNS[*domain]
	if !exists {
		lastDNS, _ := readLastDNS()
		dnsIPs = config.DNS[lastDNS]
	}
	watchResolvConf(dnsIPs)
}
