# Dyndns clone for Google Cloud DNS

This project uses Google Cloud Functions to update the IP address of a given host in Cloud DNS via a simple HTTP based API.
The update is done by sending following request:

```
https://<YOUR_REGION-YOUR_PROJECT_ID>.cloudfunctions.net/Update?hostname=myserver.example.com
```

The hostname is given as query parameter `hostname` and the IP address is automatically taken from the HTTP request.

The function must be configured with proper access control since it has privileges to update Google Cloud DNS (see below).
The client authenticates with bearer token `Authorization: Bearer NNN`.

## Deployment

Get the active project name and default region

```bash
export GCP_PROJECT=$(gcloud config get-value project)
export GCP_REGION=$(gcloud config get-value compute/region) # Uses default region
gcloud config set functions/region $GCP_REGION # Set default region for Cloud Functions
```

We will create two service accounts:

- `dyndns-function` for the function running in Cloud Functions, acting as Cloud DNS client and updating the DNS records.
- `dyndns-client` for the external client making dynamic DNS updates in a host with dynamically allocated IP address.

Create the service accounts:

```bash
gcloud iam service-accounts create dyndns-function \
  --display-name=dyndns-function \
  --project=${GCP_PROJECT}

gcloud iam service-accounts create dyndns-client \
  --display-name=dyndns-client \
  --project=${GCP_PROJECT}
```

Prepare configuration file for the cloud function:

```bash
cat > configuration.json <<'EOF'
{
    "clouddns_zone": "my-dns-zone",
    "allowed_hosts": [
        ".+\\.example.com$"
    ]
}
EOF
```

Deploy the function at `Update` endpoint and set `dyndns-function` service account for it.
When this function is executed, it will automatically receive the credentials of `dyndns-function`.

```bash
gcloud functions deploy Update \
  --gen2 \
  --runtime=go121 \
  --region=$GCP_REGION \
  --source=. \
  --entry-point=Update \
  --trigger-http \
  --service-account dyndns-function@${GCP_PROJECT}.iam.gserviceaccount.com \
  --set-env-vars CONFIGURATION=serverless_function_source_code/configuration.json,GCP_PROJECT=${GCP_PROJECT}
```

Answer `n` to question "Allow unauthenticated invocations of new function".

After successful deployment, the URL for the Cloud Function is printed.

Grant `dyndns-client` service account access to the `Update` endpoint:

```bash
gcloud functions add-invoker-policy-binding Update \
  --member="serviceAccount:dyndns-client@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --region="${GCP_REGION}"
```

Verify that access policy for `Update` endpoint was updated:

```bash
gcloud run services get-iam-policy projects/$GCP_PROJECT/locations/$GCP_REGION/services/update
```

Try to make unauthenticated request to the `Update` endpoint.
This should result in `403 Forbidden` response.

```bash
http $(gcloud functions describe Update --format='value(url)')
```

Next grant Cloud DNS admin privileges for the Cloud Function to operate on resource records Cloud DNS.
Add `dyndns-function` as member of `roles/dns.admin` role:

```bash
gcloud projects add-iam-policy-binding ${GCP_PROJECT} \
  --member="serviceAccount:dyndns-function@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role=roles/dns.admin
```

Wait for few seconds and verify that `dyndns-function` is included as member of `roles/dns.admin`:

```bash
gcloud projects get-iam-policy ${GCP_PROJECT}
```

## Using the updater client to set DNS entries

Compile the client:

```bash
go build ./cmd/updater/
```

Create key for the `dyndns-client` service account.

```bash
gcloud iam service-accounts keys create gcp-dyndns-client-serviceaccount.json \
  --iam-account=dyndns-client@${GCP_PROJECT}.iam.gserviceaccount.com \
  --project=${GCP_PROJECT}
```

The key will be used by the client to create short-lived access tokens for requests towards the REST API.

Run the client:

```bash
./updater --hostname=myserver.example.com --key-file=gcp-dyndns-client-serviceaccount.json --function-url=<YOUR_REGION-YOUR_PROJECT_ID>.cloudfunctions.net/Update
```

where `<YOUR_REGION-YOUR_PROJECT_ID>` is the region and project ID where the Cloud Function was deployed.
Run following command to get the URL:

```bash
gcloud functions describe Update --format='value(url)'
```

### Run updater client periodically as systemd service

1\. Create systemd service

```bash
sudo bash -c "cat > /etc/systemd/system/dyndns.service" <<EOF
[Unit]
Description=Register IP address to dyndns-function

[Service]
Type=oneshot
ExecStart=/path/to/updater --hostname=myserver.example.com --key-file=/path/to/gcp-dyndns-client-serviceaccount.json --function-url=<YOUR_REGION-YOUR_PROJECT_ID>.cloudfunctions.net/Update
EOF
```

2\. Create timer for the service

```bash
sudo bash -c "cat > /etc/systemd/system/dyndns.timer" <<EOF
[Unit]
Description=Daily update of IP address to dyndns-function

[Timer]
OnBootSec=15min
OnCalendar=daily
AccuracySec=1h
Persistent=true

[Install]
WantedBy=multi-user.target
EOF
```

3\. Start the timer

```bash
sudo systemctl daemon-reload
sudo systemctl start dyndns.timer
sudo systemctl status dyndns.{service,timer}
```

### Build container image for the updater client

Build the container image:

```bash
podman build -t quay.io/tsaarni/dyndns-updater:latest .
```

When running the updater as container, the key file must be mounted as volume.

To keep the container running on the background to update the DNS record periodically, start the updater with `--update-every=24h` option.

## Local development and testing

Following instructions are meant for testing the code locally.

> **NOTE**: The local web server will not authenticate the client but it will only accept requests from localhost

Generate key file for `dyndns-function` to let [Google API client library for Go](https://cloud.google.com/dns/docs/libraries) to call [Cloud DNS API](https://cloud.google.com/dns/docs/reference/v1/).

```bash
gcloud iam service-accounts keys create gcp-dyndns-function-serviceaccount.json \
 --iam-account=dyndns-function@${GCP_PROJECT}.iam.gserviceaccount.com \
  --project=${GCP_PROJECT}
```

Set the environment variable:

```bash
export GOOGLE_APPLICATION_CREDENTIALS=gcp-dyndns-function-serviceaccount.json
```

Make sure that current directory has `configuration.json` configuration file (created above).
Run the server:

```bash
go run ./cmd/test-server/
```

Make request to `Update` endpoint

```
http "http://localhost:8080/Update?hostname=test.example.com"
```
