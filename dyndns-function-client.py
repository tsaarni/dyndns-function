#!/usr/bin/env python3
#
# Client for cloud function for dynamically updating DNS entries
#
# Usage:
#   dyndns-function-client.py client-config.ini
#

import sys
import datetime
import configparser
import jwt
import requests

# read configuration from ini file
p = configparser.ConfigParser()
p.read(sys.argv[1])
config = p['client_config']

# read private key
f = open(config['private_key_file'], 'r')
key = f.read()

# create JWT
claims = {
    'iss': f'dyndns-client@{config["gcp_project"]}.iam.gserviceaccount.com',
    'aud': 'https://www.googleapis.com/oauth2/v4/token',
    'target_audience': f'{config["cloud_function_trigger_url"]}',
    'exp': datetime.datetime.utcnow() + datetime.timedelta(seconds=30),
    'iat': datetime.datetime.utcnow()
}

token = jwt.encode(claims, key, algorithm='RS256')

# request for id_token
r = requests.post('https://www.googleapis.com/oauth2/v4/token', data={
    'grant_type': 'urn:ietf:params:oauth:grant-type:jwt-bearer',
    'assertion': token.decode("utf-8")})
r.raise_for_status()
resp = r.json()
id_token = resp['id_token']

# request for DNS update
r = requests.get(f'{config["cloud_function_trigger_url"]}?hostname={config["hostname"]}',
                 headers={'Authorization': f'Bearer {id_token}'})
r.raise_for_status()
print(f'DNS for {config["hostname"]} successfully updated')
