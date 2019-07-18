package dyndns

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
)

var project = os.Getenv("GCP_PROJECT")

type configuration struct {
	Zone         string   `json:"clouddns_zone"`
	AllowedHosts []string `json:"allowed_hosts"`
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

// Update Google Cloud DNS with given hostname and IP address of originating request
func Update(w http.ResponseWriter, r *http.Request) {

	conf, err := readConfiguration("configuration.json")
	if err != nil {
		http.Error(w, "Error: cannot read configuration file", http.StatusInternalServerError)
		return
	}

	// get the caller's IP address
	addr := r.Header.Get("X-Forwarded-For")
	if addr == "" {
		addr, _, _ = net.SplitHostPort(r.RemoteAddr)
	}

	hostname := r.URL.Query().Get("hostname")

	if hostname == "" {
		http.Error(w, "Error: missing query parameter: 'hostname'", http.StatusInternalServerError)
		return
	}

	matched := false
	for _, pattern := range conf.AllowedHosts {
		matched, err = regexp.MatchString(pattern, hostname)
		if matched == true {
			break
		}
	}

	if matched == false {
		http.Error(w, "Error: hostname does not match required domain", http.StatusInternalServerError)
		return
	}

	// create client with default service account credentials set by the runtime environment
	ctx := context.Background()

	c, err := google.DefaultClient(ctx, dns.CloudPlatformScope)
	if err != nil {
		http.Error(w, "Error: failed to initialize client", http.StatusInternalServerError)
		return
	}

	// add final period to make fully qualified domain name
	fqdn := hostname + "."

	dnsService, err := dns.New(c)
	if err != nil {
		http.Error(w, "Error: failed to initialize DNS client", http.StatusInternalServerError)
		return
	}

	// get existing resource record to be updated,
	// result may be empty if record is being created for first time
	rrs, err := dnsService.ResourceRecordSets.List(project, conf.Zone).Name(fqdn).Do()
	if err != nil {
		http.Error(w, "Error: failed to fetch existing resource record", http.StatusInternalServerError)
		return
	}

	// update resource record with the new address
	rb := &dns.Change{
		Deletions: rrs.Rrsets,
		Additions: []*dns.ResourceRecordSet{
			&dns.ResourceRecordSet{
				Name:    fqdn,
				Rrdatas: []string{addr},
				Ttl:     300,
				Type:    "A",
			}},
	}

	_, err = dnsService.Changes.Create(project, conf.Zone, rb).Context(ctx).Do()
	if err != nil {
		http.Error(w, "Error: failed to update resource record", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{\n    \"hostname\": \"%s\",\n    \"address\": \"%s\"\n}\n",
		hostname,
		addr)
}
