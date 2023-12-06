// Package dyndns provides functionality for updating DNS records on Google Cloud DNS.
// It uses the Google Cloud DNS API to perform these updates.
// The package is configured via environment variables and a configuration file.
// It is used as a Cloud Function and it is invoked via HTTP requests.

package dyndns

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"google.golang.org/api/dns/v1"
)

type configuration struct {
	Zone         string   `json:"clouddns_zone"`
	AllowedHosts []string `json:"allowed_hosts"`
}

type Response struct {
	Hostname string `json:"hostname"`
	Address  string `json:"address"`
}

func init() {
	functions.HTTP("Update", Update)
}

func readConfiguration(filename string) (*configuration, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	var conf configuration
	err = json.NewDecoder(f).Decode(&conf)
	if err != nil {
		return nil, err
	}

	return &conf, nil
}

// Update Google Cloud DNS with given hostname and IP address of originating request.
func Update(w http.ResponseWriter, r *http.Request) {
	configFile := os.Getenv("CONFIGURATION")
	if configFile == "" {
		configFile = "configuration.json"
	}

	project := os.Getenv("GCP_PROJECT")
	if project == "" {
		slog.Error("Error: missing environment variable: 'GCP_PROJECT'")
		http.Error(w, "Error: missing configuration", http.StatusInternalServerError)
		return
	}

	conf, err := readConfiguration(configFile)
	if err != nil {
		slog.Error("Error reading configuration file", "error", err)
		http.Error(w, "Error: cannot read configuration file", http.StatusInternalServerError)
		return
	}

	// Get the caller's IP address.
	addr := r.Header.Get("X-Forwarded-For")
	if addr == "" {
		addr, _, _ = net.SplitHostPort(r.RemoteAddr)
	}

	hostname := r.URL.Query().Get("hostname")

	if hostname == "" {
		http.Error(w, "Error: missing query parameter: 'hostname'", http.StatusBadRequest)
		return
	}

	slog.Info("Update request", "hostname", hostname, "address", addr)

	matched := false
	for _, pattern := range conf.AllowedHosts {
		matched, err = regexp.MatchString(pattern, hostname)
		if err != nil {
			slog.Error("Error matching hostname", "error", err)
			http.Error(w, "Error: error matching hostname", http.StatusInternalServerError)
			return
		}
		if matched {
			break
		}
	}

	if !matched {
		slog.Error("Hostname does not match required domain", "hostname", hostname)
		http.Error(w, "Error: hostname does not match required domain", http.StatusBadRequest)
		return
	}

	// Add final period to make fully qualified domain name.
	fqdn := hostname + "."

	ctx := context.Background()
	dnsService, err := dns.NewService(ctx)
	if err != nil {
		slog.Error("Failed to initialize DNS client", "error", err)
		http.Error(w, "Error: failed to initialize DNS client", http.StatusInternalServerError)
		return
	}

	// Get existing resource record to be updated,
	// result may be empty if record is being created for first time.
	rrs, err := dnsService.ResourceRecordSets.List(project, conf.Zone).Name(fqdn).Do()
	if err != nil {
		slog.Error("Failed to fetch existing resource record", "fqdn", fqdn, "error", err)
		http.Error(w, "Error: failed to fetch existing resource record", http.StatusInternalServerError)
		return
	}

	// Update resource record with the new address.
	rb := &dns.Change{
		Deletions: rrs.Rrsets,
		Additions: []*dns.ResourceRecordSet{
			{
				Name:    fqdn,
				Rrdatas: []string{addr},
				Ttl:     300,
				Type:    "A",
			}},
	}

	_, err = dnsService.Changes.Create(project, conf.Zone, rb).Context(ctx).Do()
	if err != nil {
		slog.Error("Failed to update resource record", "error", err)
		http.Error(w, "Error: failed to update resource record", http.StatusInternalServerError)
		return
	}

	resp := &Response{
		Hostname: hostname,
		Address:  addr,
	}
	jsonResp, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResp)
}
