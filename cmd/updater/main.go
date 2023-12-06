// This package implements updater command-line tool to update the IP address
// associated with a given hostname. It calls the REST API defined by the
// github.com/tsaarni/dyndns package.
//
// By default the tool updates the IP address once and exits. It can also be
// used to update the IP address periodically.
//
// The tool uses the Google Cloud Identity Token API to authenticate the
// HTTP request to the cloud function. The authentication is performed using
// the provided key file.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
)

type configuration struct {
	URL         string
	KeyFile     string
	Hostname    string
	UpdateEvery time.Duration
}

type Response struct {
	Hostname string `json:"hostname"`
	Address  string `json:"address"`
}

func update(conf *configuration) error {
	ctx := context.Background()
	client, err := idtoken.NewClient(ctx, conf.URL, option.WithCredentialsFile(conf.KeyFile))
	if err != nil {
		return err
	}

	resp, err := client.Get(fmt.Sprintf("%s?hostname=%s", conf.URL, conf.Hostname))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error calling cloud function: %s", resp.Status)
	}

	var r Response
	err = json.NewDecoder(resp.Body).Decode(&r)
	if err != nil {
		return err
	}

	fmt.Println("Updated", r.Hostname, "to", r.Address)

	return nil
}

func main() {
	conf := &configuration{}

	flag.StringVar(&conf.Hostname, "hostname", "", "Hostname")
	flag.StringVar(&conf.KeyFile, "key-file", "", "Key file")
	flag.StringVar(&conf.URL, "url", "", "URL of the cloud function to call")
	flag.DurationVar(&conf.UpdateEvery, "update-every", 0, "Update periodically. Duration is e.g. 240m, 24h (default: update once and exit)")

	flag.Parse()

	if conf.Hostname == "" || conf.KeyFile == "" || conf.URL == "" {
		fmt.Fprintln(os.Stderr, "Missing required arguments")
		flag.Usage()
		os.Exit(1)
	}

	if conf.UpdateEvery == 0 {
		err := update(conf)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		for {
			err := update(conf)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}

			fmt.Println("Sleeping for", conf.UpdateEvery)
			time.Sleep(conf.UpdateEvery)
		}
	}

}
