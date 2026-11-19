#!/usr/bin/env python3
"""Home-only cutover proof. Run ONLY in the authorized window, from the lead laptop.

Usage: python3 -I ws0054-acceptance.py ARK_READY_BOUNDARY_JSON > native-proof.json
The boundary is an existing absolute path ON ARK, not a local file. Forward and
rollback use their own post-READY boundary. Keys never leave their owning node.
This checks native traffic/usage and 60s stability, NOT account/config/history
preservation or panel assets; those remain separate manual runbook gates.
"""
import datetime
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import re
import shlex
import sqlite3
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

ARK = 'root@100.110.133.6'
DATA = '/mnt/cache/appdata/cpa-home/data/home.db'
NODE_URL = 'http://127.0.0.1:18317'
MANAGEMENT = 'http://100.110.133.6:8327/v8/management'
MINIS = [('moxy', '100.110.240.90'), ('chip', '100.89.64.86'),
         ('lara', '100.71.124.48'), ('hyper', '100.70.69.110')]
MACBOOK_IP = '100.88.81.10'
UUID = r'[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}'


class ProofError(Exception):
    pass


def require(condition, code):
    if not condition:
        raise ProofError(code)


def utc(value):
    parsed = datetime.datetime.fromisoformat(value.replace('Z', '+00:00'))
    return parsed.replace(tzinfo=datetime.timezone.utc) if parsed.tzinfo is None else parsed.astimezone(datetime.timezone.utc)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *_args, **_kwargs):
        return None  # A redirect must never carry a node's key to another host.


def opener():
    # Do not send local node or management requests through an environment proxy.
    return urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())


def request_json(url, key, body=None):
    req = urllib.request.Request(url, data=None if body is None else json.dumps(body).encode(),
        headers={'Authorization': 'Bearer '+key, 'x-api-key': key,
                 'Content-Type': 'application/json', 'anthropic-version': '2023-06-01'})
    try:
        started = time.monotonic()
        with opener().open(req, timeout=90) as response:
            require(response.status == 200, 'unexpected_http_status')
            return json.load(response), response.headers, round((time.monotonic()-started)*1000)
    except urllib.error.HTTPError as error:
        status = error.code
        error.close()  # Never read/log an upstream error body.
        raise ProofError('http_status_'+str(status)) from None
    except (OSError, ValueError):
        raise ProofError('http_or_json_failed') from None


def own_key():
    providers = json.loads((Path.home()/'.openclaw/openclaw.json').read_bytes())['models']['providers']
    keys = set()
    for provider in providers.values():
        url = urllib.parse.urlparse(provider.get('baseUrl', ''))
        if url.port != 18317:
            continue
        value = provider.get('apiKey', '')
        if value.startswith('${') and value.endswith('}'):
            value = os.environ.get(value[2:-1], '')
        require(isinstance(value, str) and bool(value), 'own_key_unavailable')
        keys.add(value)
    require(len(keys) == 1, 'own_key_missing_or_ambiguous')
    return keys.pop()


def node_probe(_payload):
    key = own_key()
    catalog, _, _ = request_json(NODE_URL+'/v1/models', key)
    models = {row['id'] for row in catalog['data']}
    haikus = [m for m in models if re.fullmatch(r'claude-haiku-\d[\d.-]*', m)]
    require(haikus and 'gpt-6.2-sol' in models, 'required_catalog_model_missing')
    haiku = max(haikus, key=lambda m: tuple(map(int, re.findall(r'\d+', m))))
    results = []
    for provider, path, body in [
        ('claude', '/v1/messages', {'model': haiku, 'max_tokens': 8,
          'messages': [{'role': 'user', 'content': 'Say OK.'}]}),
        ('codex', '/v1/responses', {'model': 'gpt-6.2-sol', 'input': 'Say OK.',
          'reasoning': {'effort': 'low'}, 'max_output_tokens': 1024, 'stream': False})]:
        value, headers, latency = request_json(NODE_URL+path, key, body)
        # CPA8.0.69 cpa_trace.go: selected-time + auth-index + context UUID;
        # redisqueue/plugin.go sends that context UUID as Home usage.request_id.
        match = re.fullmatch(r'\d{14}-.+-('+UUID+r')', headers.get('X-CPA-TRACE-ID', ''))
        require(match, 'response_trace_missing_or_invalid')
        response_id = value.get('id', '')
        prefix = 'msg_' if provider == 'claude' else 'resp_'
        require(re.fullmatch(prefix+r'[A-Za-z0-9_-]{1,128}', response_id), 'native_response_id_missing')
        require(not value.get('error'), 'native_response_error')
        if provider == 'claude':
            complete = bool(value.get('stop_reason')) and value['stop_reason'] != 'error'
            blocks = value.get('content', [])
            text_type = 'text'
        else:
            complete = value.get('status') == 'completed'
            blocks = [block for item in value.get('output', []) if item.get('type') == 'message'
                      for block in item.get('content', [])]
            text_type = 'output_text'
        # Inspect text in memory only; never include it in proof, logs or errors.
        require(complete and any(block.get('type') == text_type and
                isinstance(block.get('text'), str) and block['text'].strip() for block in blocks),
                'native_answer_missing_or_unfinished')
        results.append({'provider': provider, 'model': body['model'], 'request_id': match.group(1),
                        'response_id': response_id, 'http_status': 200, 'latency_ms': latency})
    return results


def usage_rows(connection, boundary, request_ids):
    if not request_ids:
        return []
    require(all(re.fullmatch(UUID, rid) for rid in request_ids), 'invalid_request_uuid')
    sql = '''SELECT u.id,u.timestamp,u.created_at,u.request_id,u.cpa_node_id AS node_id,
        u.provider,u.model,u.failed,a.uuid AS credential_uuid,a.disabled AS auth_disabled,
        a.status AS auth_status,a.deleted_at AS auth_deleted,a.provider AS auth_provider
        FROM usage u LEFT JOIN auth a ON a.uuid=u.quota_credential_id
        WHERE u.id>? AND u.request_id IN ('''+','.join('?' for _ in request_ids)+')'
    return [dict(row) for row in connection.execute(sql, [boundary['id'], *request_ids])]


def ark_read(payload):
    boundary = json.loads(Path(payload['boundary']).read_text())
    boundary = {k: boundary[k] for k in ('id', 'timestamp', 'container_id', 'started_at', 'expected_nodes')}
    require(type(boundary['id']) is int and boundary['id'] >= 0 and boundary['expected_nodes'], 'invalid_boundary')
    utc(boundary['timestamp'])
    info = json.loads(subprocess.check_output(['docker', 'inspect', 'cpa-home', '--format',
        '{"id":{{json .Id}},"image":{{json .Image}},"state":{{json .State}},"restarts":{{.RestartCount}}}'],
        text=True, timeout=30))
    state = info['state']
    require(info['id'] == boundary['container_id'] and state['StartedAt'] == boundary['started_at'], 'container_changed')
    require(state['Running'] and not state['Paused'] and not state.get('Restarting') and info['restarts'] == 0,
            'container_not_stable')
    password = os.environ['MANAGEMENT_PASSWORD']  # Loaded by the sanctioned shell source, never parsed here.
    membership, _, _ = request_json(MANAGEMENT+'/nodes', password)
    fleet, _, _ = request_json(MANAGEMENT+'/fleet', password)
    require(fleet['policy']['patch_updates'] is False and not fleet.get('active'), 'fleet_policy_or_active_task')
    nodes = []
    for node in membership['nodes']:
        nid = node['node_id']
        require(re.fullmatch(r'[A-Za-z0-9_.:-]{1,128}', nid), 'invalid_node_id')
        ip = str(ipaddress.ip_address(node['ip']))
        identity = {k: fleet['nodes'][nid].get(k) for k in ('version', 'sha256', 'size', 'platform', 'catalog')}
        require(node['healthy'] is True and identity['version'] == '8.0.69', 'membership_health_or_version')
        require(isinstance(identity['sha256'], str) and re.fullmatch(r'[a-fA-F0-9]{64}', identity['sha256'])
                and type(identity['size']) is int and identity['size'] > 0
                and identity['platform'] == 'darwin/arm64' and type(identity['catalog']) is bool,
                'invalid_node_artifact')
        nodes.append(dict(node_id=nid, ip=ip, **identity))
    require(len({n['node_id'] for n in nodes}) == len(nodes), 'duplicate_node_membership')
    require(set(boundary['expected_nodes']) <= {n['node_id'] for n in nodes}, 'required_membership_missing')
    connection = sqlite3.connect('file:'+DATA+'?mode=ro', uri=True, timeout=10)
    connection.row_factory = sqlite3.Row
    try:
        rows = usage_rows(connection, boundary, payload.get('request_ids', []))
    finally:
        connection.close()
    return {'boundary': boundary, 'container_id': info['id'], 'started_at': state['StartedAt'],
            'candidate_image': info['image'], 'nodes': sorted(nodes, key=lambda n: n['node_id']), 'usage': rows}


# Source and non-secret input travel over stdin, never a key-bearing process argument or temporary script.
BOOTSTRAP = '''import json,sys
g={}
try:
 d=json.load(sys.stdin);g={'__name__':'acceptance_transport'}
 exec(compile(d['source'],'acceptance_transport','exec'),g)
 print(json.dumps(g[d['function']](d['payload'])))
except Exception as error:
 code=str(error) if isinstance(error,g.get('ProofError',())) else 'remote_failed'
 print(json.dumps({'error':code}));sys.exit(1)
'''


def remote(host, function, payload):
    args = ['ssh', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=10', '-o', 'IdentitiesOnly=yes',
            '-o', 'IdentityAgent=none']
    command = 'python3 -I -c '+shlex.quote(BOOTSTRAP)
    if host == ARK:
        args += ['-i', str(Path.home()/'.ssh/cronos_ark')]
        command = 'set +x; set -a; . /mnt/user/appdata/cpa-home/.env; set +a; '+command
    args += [host, command]
    try:
        result = subprocess.run(args, input=json.dumps({'source': Path(__file__).read_text(),
            'function': function, 'payload': payload}), text=True, capture_output=True, timeout=240)
        value = json.loads(result.stdout)
        if result.returncode != 0:
            code = value.get('error', '') if isinstance(value, dict) else ''
            require(re.fullmatch(r'[a-z_0-9]{1,80}', code), 'transport_failed')
            raise ProofError(function+'_'+code)
        return value
    except (OSError, ValueError, subprocess.TimeoutExpired):
        raise ProofError('transport_failed') from None


def verify_usage(probes, rows, boundary):
    require(len({p['request_id'] for p in probes}) == len(probes), 'duplicate_probe_uuid')
    verified = []
    for probe in probes:
        matching = [r for r in rows if r['request_id'] == probe['request_id']]
        require(matching, 'usage_pending')
        require(len(matching) == 1, 'duplicate_usage')
        row = matching[0]
        require(row['id'] > boundary['id'] and utc(row['timestamp']) > utc(boundary['timestamp'])
                and utc(row['created_at']) > utc(boundary['timestamp']), 'usage_before_boundary')
        require(all(row[k] == probe[k] for k in ('node_id', 'provider', 'model')), 'usage_attribution_mismatch')
        require(not row['failed'] and row['credential_uuid'] and row['auth_disabled'] == 0
                and row['auth_status'] == 'active' and row['auth_deleted'] is None
                and row['auth_provider'] == probe['provider'], 'usage_auth_not_enabled')
        # Hash the identity, not an email/personal label; retain only six characters.
        label = hashlib.sha256(row['credential_uuid'].encode()).hexdigest()[:6]
        verified.append(dict(probe, usage_id=row['id'], credential_label=label))
    return verified


def finish(boundary_path, before, probes):
    payload = {'boundary': boundary_path, 'request_ids': [p['request_id'] for p in probes]}
    deadline = time.monotonic()+90
    while True:
        after = remote(ARK, 'ark_read', payload)
        require(all(after[k] == before[k] for k in ('boundary', 'container_id', 'started_at', 'candidate_image', 'nodes')),
                'readiness_changed')
        try:
            verified = verify_usage(probes, after['usage'], before['boundary'])
            break
        except ProofError as error:
            if str(error) != 'usage_pending' or time.monotonic() >= deadline:
                raise
            time.sleep(2)
    time.sleep(60)
    stable = remote(ARK, 'ark_read', payload)
    require(all(stable[k] == after[k] for k in ('boundary', 'container_id', 'started_at', 'candidate_image', 'nodes')),
            'stability_changed')
    require(verify_usage(probes, stable['usage'], before['boundary']) == verified, 'usage_changed')
    return {'ok': True, 'candidate_image': stable['candidate_image'], 'boundary': before['boundary'],
            'nodes': stable['nodes'], 'native_requests': verified, 'stability_seconds': 60,
            'scope': 'native_usage_and_membership_only; panel_and_preservation_are_separate_gates'}


def run(boundary_path):
    require(Path(boundary_path).is_absolute(), 'boundary_must_be_absolute_ark_path')
    before = remote(ARK, 'ark_read', {'boundary': boundary_path, 'request_ids': []})
    by_ip = {n['ip']: n['node_id'] for n in before['nodes']}
    require(len(by_ip) == len(before['nodes']), 'ambiguous_node_ip')
    probes = []
    for user, ip in MINIS:
        require(ip in by_ip, 'mini_membership_missing')
        probes += [dict(p, node_id=by_ip[ip]) for p in remote(user+'@'+ip, 'node_probe', {})]
    macbook = 'not_connected_optional_skip'
    if MACBOOK_IP in by_ip:
        probes += [dict(p, node_id=by_ip[MACBOOK_IP]) for p in node_probe({})]
        macbook = 'tested'
    tested_ips = [ip for _, ip in MINIS]+([MACBOOK_IP] if macbook == 'tested' else [])
    expected = {(by_ip[ip], provider) for ip in tested_ips for provider in ('claude', 'codex')}
    require(len(probes) == len(expected) and {(p['node_id'], p['provider']) for p in probes} == expected,
            'native_coverage_missing')
    result = finish(boundary_path, before, probes)
    result['macbook'] = macbook
    return result


if __name__ == '__main__':
    try:
        require(len(sys.argv) == 2, 'usage_requires_ark_ready_boundary_path')
        print(json.dumps(run(sys.argv[1]), sort_keys=True))
    except ProofError as error:
        print(json.dumps({'ok': False, 'error': str(error)})); sys.exit(1)
    except Exception:
        print(json.dumps({'ok': False, 'error': 'acceptance_failed'})); sys.exit(1)
