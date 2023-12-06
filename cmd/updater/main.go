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

const serviceAccount = "dyndns-client"

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
