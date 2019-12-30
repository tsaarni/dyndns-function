
# Dyndns clone for Google Cloud DNS

This project uses Google Cloud Functions to update the IP address of a given
host in Cloud DNS via a simple HTTP based API.  The update is done by sending
following request:

    https://<YOUR_REGION-YOUR_PROJECT_ID>.cloudfunctions.net/Update?hostname=myserver.example.com

The hostname is given as query parameter `hostname` and the IP address is
automatically taken from the HTTP request.

The function must be configured with proper access control since it has
privileges to update Google Cloud DNS (see Deploy below). The client
authenticates with bearer token `Authorization: Bearer NNN`.


## Deployment

Get the active project name

```
export GCP_PROJECT=$(gcloud config get-value project)
```


We will need two service accounts:

* `dyndns-function` for the function running in Cloud Functions, running
   CloudDNS client
* `dyndns-client` for the external client making dynamic DNS updates in a host
   with dynamically allocated IP address


Create the service accounts

```
gcloud iam service-accounts create dyndns-function \
  --display-name=dyndns-function \
  --project=${GCP_PROJECT}

gcloud iam service-accounts create dyndns-client \
  --display-name=dyndns-client \
  --project=${GCP_PROJECT}
```


Create key for the `dyndns-client`.  The key will be used to create access tokens
for requests towards the REST API:

```
gcloud iam service-accounts keys create gcp-dyndns-client-serviceaccount.json \
  --iam-account=dyndns-client@${GCP_PROJECT}.iam.gserviceaccount.com \
  --project=${GCP_PROJECT}
```


Prepare configuration file

```
cat > configuration.json <<'EOF'
{
    "clouddns_zone": "myzone",
    "allowed_hosts": [
        ".+\\.example.com$"
    ]
}
EOF
```


Deploy the function at `Update` endpoint and set `dyndns-function` service
account for it.  When this function is executed, it will automatically receive
the credentials of `dyndns-function`.

```
gcloud functions deploy Update \
  --runtime go111 --trigger-http \
  --service-account dyndns-function@${GCP_PROJECT}.iam.gserviceaccount.com
```

Aftrer successful deploy, the URL for the Cloud Function is printed.
Currently Cloud Functions are publicly accessible by any unauthenticated client.
This will change in future according to following Google Cloud announcement

> After November 1, 2019, newly created functions will be private-by-default, and will only be invocable by authorized clients unless you set a public IAM policy on the function

For now, to enable authentication remove the `roles/cloudfunctions.invoker`
role from `allUsers` and adding the role to `dyndns-client` instead:

```
gcloud beta functions remove-iam-policy-binding Update \
  --member=allUsers \
  --role=roles/cloudfunctions.invoker

gcloud beta functions add-iam-policy-binding Update \
  --member="serviceAccount:dyndns-client@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role="roles/cloudfunctions.invoker"
```


Verify that access policy for `Update` endpoint was updated

```
gcloud beta functions get-iam-policy Update
```

See [here](https://cloud.google.com/functions/docs/securing/managing-access)
for more information about IAM and Cloud Functions.

> **WARNING**: Wait for a moment for the access policy to be applied and double check that anonymous requests are now rejected!

```
export CLOUD_FUNCTION_TRIGGER_URL=$(gcloud functions describe Update --format='value(httpsTrigger.url)')
http "${CLOUD_FUNCTION_TRIGGER_URL}"  # this should result in: 403 Forbidden
```


Next we are granting Cloud DNS admin privileges for the Cloud Function to
operate on resource records Cloud DNS.  Add `dyndns-function` as member of
`roles/dns.admin` role

```
gcloud projects add-iam-policy-binding ${GCP_PROJECT} \
  --member="serviceAccount:dyndns-function@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role=roles/dns.admin
```


Wait for few seconds and verify that `dyndns-function` is included as member
of `roles/dns.admin`

```
gcloud projects get-iam-policy ${GCP_PROJECT}
```


## Using the API to update DNS entries

The requests to the `Update` API endpoint need to be authorized by JWT
access token.  Install [jwt-go](https://github.com/dgrijalva/jwt-go) to
generate JWTs

```
go get -u github.com/dgrijalva/jwt-go/cmd/jwt
```


The instructions are based on
[Service-to-function authentication](https://cloud.google.com/functions/docs/securing/authenticating#service-to-function)

Set the HTTP trigger URL for the Cloud Function into environment variable

```
export CLOUD_FUNCTION_TRIGGER_URL=$(gcloud functions describe Update --format='value(httpsTrigger.url)')
```


Extract `private_key` from service account key file (example uses
[jq](https://stedolan.github.io/jq/) to extract the key)

```
cat gcp-dyndns-client-serviceaccount.json | jq -r .private_key > dyndns-client-key.pem
```


Create self-signed JWT token for authenticating the client towards Google's
`token` endpoint

```
cat <<EOF | jwt -key dyndns-client-key.pem -alg RS256 -sign - > jwt-token
{
    "iss": "dyndns-client@${GCP_PROJECT}.iam.gserviceaccount.com",
    "aud": "https://www.googleapis.com/oauth2/v4/token",
    "target_audience": "${CLOUD_FUNCTION_TRIGGER_URL}",
    "exp": $(($(date +%s) + 60*60)),
    "iat": $(date +%s)
}
EOF
```


Make request for a `id_token` from the `token` endpoint (example uses
[httpie](https://httpie.org/) as client)

```
http -v POST https://www.googleapis.com/oauth2/v4/token \
   grant_type="urn:ietf:params:oauth:grant-type:jwt-bearer" \
   assertion=@jwt-token
```


Finally copy the `id_token` from the response and use it as Bearer token in th
request to `Update` endpoint

```
http "${CLOUD_FUNCTION_TRIGGER_URL}?hostname=myserver.example.com" Authorization:"Bearer <ID_TOKEN>"
```


Both self-signed JWT token and `id_token` are valid for hour.  After that, the
process must be repeated.


Alternatively the `id_token` can be creted with Google Cloud SDK

```
gcloud auth print-identity-token dyndns-client@${GCP_PROJECT}.iam.gserviceaccount.com \
  --audiences="${CLOUD_FUNCTION_TRIGGER_URL}"
```


## Local development and testing

Following instructions are meant for testing the code locally.

> **NOTE**: The local web server will not authenticate the client but it will only accept requests from localhost


Generate key file for `dyndns-function` to let
[Google API client library for Go](https://cloud.google.com/dns/docs/libraries)
to call [Cloud DNS API](https://cloud.google.com/dns/docs/reference/v1/)

```
gcloud iam service-accounts keys create gcp-dyndns-function-serviceaccount.json \
  --iam-account=dyndns-function@${GCP_PROJECT}.iam.gserviceaccount.com \
  --project=${GCP_PROJECT}
```


Set the environment variable

```
export GOOGLE_APPLICATION_CREDENTIALS=gcp-dyndns-function-serviceaccount.json
```


Build and execute the server

```
go build ./cmd/test-server && ./test-server
```


Make request to `Update` endpoint

```
http "http://localhost:8080/Update?hostname=test.example.com"
```


## Python client

File [dyndns-function-client.py](dyndns-function-client.py) implements a client that requests dyndns-function to update a DNS entry.
Following instructions show how to run it periodically from systemd:

1\. Clone this repository `git clone https://github.com/tsaarni/dyndns-function.git`

2\. Generate `dyndns-client-key.pem` according to the instructions in "Using the API to update DNS entries" section above.

3\. Update [client-config.ini](client-config.ini) file to match with your settings.

4\. Create Python "virtual environment" and install required modules

```
python3 -mvenv venv
. venv/bin/activate
pip install -r requirements.txt
```

5\. Create systemd service

```bash
sudo bash -c "cat > /etc/systemd/system/dyndns.service" <<EOF
[Unit]
Description=Register IP address to dyndns-function

[Service]
Type=oneshot
WorkingDirectory=/path/to/dyndns-function/
ExecStart=/path/to/dyndns-function/venv/bin/python dyndns-function-client.py client-config.ini
EOF
```

6\. Create timer for the service

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

7\. Start the timer

```bash
sudo systemctl daemon-reload
sudo systemctl start dyndns.timer
sudo systemctl status dyndns.{service,timer}
```
