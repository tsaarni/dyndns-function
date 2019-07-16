package dyndns

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"os"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
)

var project = os.Getenv("GCP_PROJECT")
var domain = os.Getenv("CLOUDDNS_DOMAIN")
var zone = os.Getenv("CLOUDDNS_ZONE")

// Update Google Cloud DNS with given hostname and IP address of originating request
func Update(w http.ResponseWriter, r *http.Request) {

	// get the caller's IP address
	addr := r.Header.Get("X-Forwarded-For")
	if addr == "" {
		addr, _, _ = net.SplitHostPort(r.RemoteAddr)
	}

	hostname := r.URL.Query().Get("hostname") + "."
	if hostname == "." {
		http.Error(w, "Error: missing query parameter: 'hostname'", http.StatusInternalServerError)
		return
	}

	if !strings.HasSuffix(hostname, domain) {
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

	dnsService, err := dns.New(c)
	if err != nil {
		http.Error(w, "Error: failed to initialize DNS client", http.StatusInternalServerError)
		return
	}

	// get existing resource record to be updated, result may be empty
	// if record is being created for first time
	rrs, err := dnsService.ResourceRecordSets.List(project, zone).Name(hostname).Do()
	if err != nil {
		http.Error(w, "Error: failed to fetch existing resource record", http.StatusInternalServerError)
		return
	}

	// update resource record with the new address
	rb := &dns.Change{
		Deletions: rrs.Rrsets,
		Additions: []*dns.ResourceRecordSet{
			&dns.ResourceRecordSet{
				Name:    hostname,
				Rrdatas: []string{addr},
				Ttl:     300,
				Type:    "A",
			}},
	}

	_, err = dnsService.Changes.Create(project, zone, rb).Context(ctx).Do()
	if err != nil {
		http.Error(w, "Error: failed to update resource record", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{\n    \"hostname\": \"%s\",\n    \"address\": \"%s\"\n}\n",
		strings.TrimSuffix(hostname, "."),
		addr)
}
