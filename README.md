
# Dyndns clone for Google Cloud DNS

This project uses Google Cloud Functions to update the IP address of a given
host in Cloud DNS.  The update is done by sending following request:

    https://<YOUR_REGION-YOUR_PROJECT_ID>.cloudfunctions.net/Update?hostname=myserver.example.com

The hostname is given as query parameter `hostname` and the IP address is
automatically taken from the HTTP request.

The function must be configured with proper access control since it has
privileges to update Google Cloud DNS (see Deploy below). The client
authenticates with bearer token `Authorization: Bearer NNN`.


## Deploy

Prepare two service accounts: `dyndns-function` for the function running in
Cloud Functions and `dyndns-client` for the client making dynamic DNS updates.

    # get the active project name
    export GCP_PROJECT=$(gcloud config get-value project)

    # create service account for cloud function
    gcloud iam service-accounts create dyndns-function \
      --display-name=dyndns-function \
      --project=${GCP_PROJECT}

    # create service account for client
    gcloud iam service-accounts create dyndns-client \
      --display-name=dyndns-client \
      --project=${GCP_PROJECT}
    gcloud iam service-accounts keys create gcp-dyndns-client-serviceaccount.json \
      --iam-account=dyndns-client@${GCP_PROJECT}.iam.gserviceaccount.com \
      --project=${GCP_PROJECT}
    gcloud auth activate-service-account dyndns-client@${GCP_PROJECT}.iam.gserviceaccount.com \
      --key-file gcp-dyndns-client-serviceaccount.json


Prepare environment variable file

    cat > .env.yaml <<EOF
    CLOUDDNS_DOMAIN: example.com.
    CLOUDDNS_ZONE: myzone
    EOF

Deploy the function

    # set dyndns-function service account for the function
    gcloud functions deploy Update \
      --env-vars-file .env.yaml \
      --runtime go111 --trigger-http \
      --service-account dyndns-function@${GCP_PROJECT}.iam.gserviceaccount.com \
      --source functions


Require authentication from clients when they make request to the function

    gcloud beta functions remove-iam-policy-binding Update \
      --member=allUsers \
      --role=roles/cloudfunctions.invoker
    gcloud beta functions add-iam-policy-binding Update \
      --member="serviceAccount:dyndns-client@${GCP_PROJECT}.iam.gserviceaccount.com" \
      --role="roles/cloudfunctions.invoker"

    # verify that access policy was updated
    gcloud beta functions get-iam-policy Update


> **WARNING**: Next we are granting Cloud DNS admin privileges for the function.  Wait for a moment for the policy to be applied and double check that anonymous requests are now rejected!


Add privileges for `dyndns-function` to update Cloud DNS

    gcloud projects add-iam-policy-binding ${GCP_PROJECT} \
      --member=serviceAccount:dyndns-function@${GCP_PROJECT}.iam.gserviceaccount.com \
      --role=roles/dns.admin



Create identity token for client to make requests

    gcloud auth print-identity-token dyndns-client@${GCP_PROJECT}.iam.gserviceaccount.com \
      --audiences="<FUNCTION URL>"


Alternatively if you prefer avoiding dependency to Google Cloud SDK, you can
create self-signed JWT and exchange that to Google-signed ID token
(based on [Service-to-function authentication](https://cloud.google.com/functions/docs/securing/authenticating#service-to-function))

    # install jwt-go for generating JWT
    go get -u github.com/dgrijalva/jwt-go/cmd/jwt

    # extract private key from service account
    cat gcp-dyndns-client-serviceaccount.json | jq -r .private_key > dyndns-client-key.pem

    # create self-signed JWT token
    cat <<EOF | jwt -key dyndns-client-key.pem -alg RS256 -sign - > jwt-token
    {
        "iss": "dyndns-client@${GCP_PROJECT}.iam.gserviceaccount.com",
        "aud": "https://www.googleapis.com/oauth2/v4/token",
        "target_audience": "https://<YOUR_REGION-YOUR_PROJECT_ID>.cloudfunctions.net/Update",
        "exp": $(date -d "now 1 hour" +%s),
        "iat": $(date +%s)
    }
    EOF

    # use self-signed JWT to make request for id token (example uses httpie as client)
    http -v POST https://www.googleapis.com/oauth2/v4/token \
       grant_type="urn:ietf:params:oauth:grant-type:jwt-bearer" \
       assertion=@jwt-token

Finally copy the `id_token` from the response and use it as Bearer token in the request to Update function.


## Local development and testing

    # generate credentials file to update Cloud DNS
    gcloud iam service-accounts keys create gcp-dyndns-function-serviceaccount.json \
      --iam-account=dyndns-function@${GCP_PROJECT}.iam.gserviceaccount.com \
      --project=${GCP_PROJECT}

    export GOOGLE_APPLICATION_CREDENTIALS=gcp-dyndns-function-serviceaccount.json
    export CLOUDDNS_DOMAIN=example.com.
    export CLOUDDNS_ZONE=myzone

    go build && ./dyndns-function
