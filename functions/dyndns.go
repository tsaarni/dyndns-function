package functions

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
)

var project = os.Getenv("GCP_PROJECT")
var domain = os.Getenv("CLOUDDNS_DOMAIN")
var zone = os.Getenv("CLOUDDNS_ZONE")

// Update Google Cloud DNS with given hostname and IP address of originating request
func Update(w http.ResponseWriter, r *http.Request) {

	addr := r.Header.Get("X-Forwarded-For")
	if addr == "" {
		addr, _, _ = net.SplitHostPort(r.RemoteAddr)
	}

	hostname := r.URL.Query().Get("hostname") + "."
	if hostname == "." {
		http.Error(w, "Error: query parameter 'hostname' missing", http.StatusInternalServerError)
		return
	}

	if !strings.HasSuffix(hostname, domain) {
		http.Error(w, "Error: hostname does not match required domain", http.StatusInternalServerError)
		return
	}

	ctx := context.Background()

	c, err := google.DefaultClient(ctx, dns.CloudPlatformScope)
	if err != nil {
		http.Error(w, "Error initializing service account", http.StatusInternalServerError)
		return
	}

	dnsService, err := dns.New(c)
	if err != nil {
		http.Error(w, "Error initializing client", http.StatusInternalServerError)
		return
	}

	// get existing resource record (to be updated, may be empty if record is being created for first time
	rrs, err := dnsService.ResourceRecordSets.List(project, zone).Name(hostname).Do()
	if err != nil {
		http.Error(w, "Error fetching existing records", http.StatusInternalServerError)
		return
	}

	// update resource record
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
		http.Error(w, "Error updaing records", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Updated DNS with hostname=%s and IP=%s", hostname, addr)
}
