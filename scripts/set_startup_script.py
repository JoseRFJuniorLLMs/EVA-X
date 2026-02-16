import json, urllib.request, urllib.parse, sys, os

CRED_PATH = os.path.expandvars(r'%APPDATA%/gcloud/legacy_credentials/web2ajax@gmail.com/adc.json')

def get_token():
    with open(CRED_PATH) as f:
        creds = json.load(f)
    data = urllib.parse.urlencode({
        'client_id': creds['client_id'],
        'client_secret': creds['client_secret'],
        'refresh_token': creds['refresh_token'],
        'grant_type': 'refresh_token'
    }).encode()
    req = urllib.request.Request('https://oauth2.googleapis.com/token', data=data)
    resp = urllib.request.urlopen(req, timeout=10)
    return json.loads(resp.read())['access_token']

def api(method, path, body=None):
    token = get_token()
    url = f'https://compute.googleapis.com/compute/v1/projects/malaria-487614/{path}'
    req = urllib.request.Request(
        url,
        data=json.dumps(body).encode() if body else None,
        headers={'Authorization': f'Bearer {token}', 'Content-Type': 'application/json'},
        method=method
    )
    try:
        resp = urllib.request.urlopen(req, timeout=15)
        return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        print(f"ERROR {e.code}: {e.read().decode()[:300]}")
        sys.exit(1)

# Read startup script from file
script_path = os.path.join(os.path.dirname(__file__), 'vm_startup.sh')
with open(script_path, 'r', encoding='utf-8') as f:
    startup_script = f.read()

# Get current metadata
vm = api('GET', 'zones/africa-south1-a/instances/malaria-vm')
metadata = vm.get('metadata', {})
fingerprint = metadata.get('fingerprint', '')
existing_items = metadata.get('items', [])

# Keep ssh-keys, replace startup-script
new_items = [item for item in existing_items if item['key'] == 'ssh-keys']
new_items.append({'key': 'startup-script', 'value': startup_script})

result = api('POST', 'zones/africa-south1-a/instances/malaria-vm/setMetadata', {
    'fingerprint': fingerprint,
    'items': new_items
})
print(f"Metadata update: {result.get('status')}")
