// Package main provides the entry point for the updater command-line tool.
// This tool is used to update the IP address associated with a given hostname
// by calling a cloud function.
//
// The tool reads the configuration from command-line flags and makes an HTTP
// request to the specified cloud function URL, passing the hostname as a query
// parameter. The response from the cloud function is then decoded and printed
// to the console.
//
// The configuration is expected to be provided through the following command-line
// flags:
//
//	-hostname: The hostname to update.
//	-key-file: The path to the key file used for authentication.
//	-function-url: The URL of the cloud function to call.
//
// If any of the required flags are missing, an error message is printed to
// the standard error stream and the tool exits with a non-zero status code.
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

	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
)

type configuration struct {
	FunctionURL string `json:"function_url"`
	KeyFile     string `json:"key_file"`
	Hostname    string `json:"hostname"`
}

type Response struct {
	Hostname string `json:"hostname"`
	Address  string `json:"address"`
}

func main() {
	conf := &configuration{}

	flag.StringVar(&conf.Hostname, "hostname", "", "Hostname")
	flag.StringVar(&conf.KeyFile, "key-file", "", "Key file")
	flag.StringVar(&conf.FunctionURL, "function-url", "", "Function URL")

	flag.Parse()

	if conf.Hostname == "" || conf.KeyFile == "" || conf.FunctionURL == "" {
		fmt.Fprintln(os.Stderr, "Missing required arguments")
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()
	client, err := idtoken.NewClient(ctx, conf.FunctionURL, option.WithCredentialsFile(conf.KeyFile))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating client", err)
		os.Exit(1)
	}

	resp, err := client.Get(fmt.Sprintf("%s?hostname=%s", conf.FunctionURL, conf.Hostname))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error calling cloud function", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "Error calling cloud function", resp.Status)
		os.Exit(1)
	}

	var r Response
	err = json.NewDecoder(resp.Body).Decode(&r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error decoding response", err)
		os.Exit(1)
	}

	fmt.Println("Updated", r.Hostname, "to", r.Address)
}
