#!/usr/bin/env python3
"""Publish Home-owned Codex access tokens to isolated CodexBar profile homes."""
import argparse
import base64
import datetime as dt
import fcntl
import importlib.util
import json
import os
from pathlib import Path
import sys
import urllib.request

shared_path = Path(__file__).resolve().parent.parent / 'codexbar-claude-sync' / 'sync.py'
spec = importlib.util.spec_from_file_location('claude_sync', shared_path)
shared = importlib.util.module_from_spec(spec)
spec.loader.exec_module(shared)

REMOTE = r'''
import json,pathlib,sys,urllib.request,urllib.parse,urllib.error
s=json.loads(SETTINGS_JSON)
try:
 v={}
 for line in pathlib.Path(s['env_file']).read_text().splitlines():
  if '=' in line and not line.lstrip().startswith('#'):
   k,x=line.split('=',1);v[k.strip()]=x.strip().strip("\"'")
 o=urllib.request.build_opener(urllib.request.ProxyHandler({}))
 result=[]
 for a in s['codex_accounts']:
  q=urllib.parse.urlencode({'id':a['home_id']})
  r=urllib.request.Request(s['management_url']+'/credentials/download?'+q,headers={'Authorization':'Bearer '+v['MANAGEMENT_PASSWORD']})
  with o.open(r,timeout=12) as response:c=json.load(response)
  if c.get('type')!='codex' or c.get('email')!=a['email']:raise ValueError()
  result.append({'home_id':a['home_id'],'email':c['email'],'access_token':c['access_token'],
                 'id_token':c['id_token'],'account_id':c['account_id']})
 print(json.dumps({'accounts':result}))
except urllib.error.HTTPError as e:
 print(json.dumps({'error':'management_http_'+str(e.code)}));sys.exit(1)
except Exception:
 print(json.dumps({'error':'remote_credential_read_failed'}));sys.exit(1)
'''


def claims(token):
    part = token.split('.')[1]
    return json.loads(base64.urlsafe_b64decode(part + '=' * (-len(part) % 4)))


def validate(account, mapping):
    if set(account) != {'home_id', 'email', 'access_token', 'id_token', 'account_id'}:
        raise shared.SyncError('unexpected Codex credential fields')
    if account['home_id'] != mapping['home_id'] or account['email'] != mapping['email']:
        raise shared.SyncError('Codex account identity mismatch')
    access, identity = claims(account['access_token']), claims(account['id_token'])
    owner = identity.get('email')
    scope = access.get('https://api.openai.com/auth', {}).get('chatgpt_account_id')
    if owner != mapping['email'] or scope != account['account_id']:
        raise shared.SyncError('Codex token identity mismatch')
    expiry = dt.datetime.fromtimestamp(access['exp'], dt.timezone.utc)
    if expiry <= dt.datetime.now(dt.timezone.utc) + dt.timedelta(minutes=5):
        raise shared.SyncError('Codex access token needs Home renewal')
    return expiry.isoformat()


def verify_usage(account):
    request = urllib.request.Request('https://chatgpt.com/backend-api/wham/usage', headers={
        'Authorization': 'Bearer ' + account['access_token'],
        'ChatGPT-Account-Id': account['account_id'], 'User-Agent': 'CodexBar'})
    with urllib.request.urlopen(request, timeout=20) as response:
        payload = json.load(response)
    if not isinstance(payload.get('rate_limit'), dict):
        raise shared.SyncError('Codex usage verification failed')


def run(settings, state):
    remote = {k: settings[k] for k in ('env_file', 'management_url', 'codex_accounts')}
    payload = shared.remote_read(settings, REMOTE.replace('SETTINGS_JSON', repr(json.dumps(remote)), 1))
    mappings = settings['codex_accounts']
    accounts = payload['accounts']
    if len(accounts) != len(mappings):
        raise shared.SyncError('Codex account count mismatch')
    pending, report = [], []
    for account, mapping in zip(accounts, mappings):
        expiry = validate(account, mapping)
        home = Path(mapping['profile_home']).expanduser()
        home.mkdir(parents=True, exist_ok=True, mode=0o700)
        if home.is_symlink():
            raise shared.SyncError('Codex profile home must not be a symlink')
        os.chmod(home, 0o700)
        path = home / 'auth.json'
        if path.is_symlink():
            raise shared.SyncError('Codex profile auth must not be a symlink')
        original = path.read_bytes() if path.exists() else None
        # CodexBar requires a nonempty field to parse native auth. This fixed marker
        # is not a credential; Home remains the only refresh-token owner.
        auth = {'auth_mode': 'chatgpt', 'tokens': {k: account[k] for k in
                ('access_token', 'id_token', 'account_id')}}
        auth['tokens']['refresh_token'] = 'cpa-home-owned-no-local-refresh'
        content = shared.encoded(auth)
        if original != content:
            verify_usage(account)
            pending.append((path, content, original))
        report.append({'email': mapping['email'], 'expires_at': expiry, 'changed': original != content})
    for path, content, original in pending:
        shared.atomic_write(path, content, expected=original)
    # Follow the same Mac/key scope as the Claude helper, using Codex attribution.
    follow_remote = {k: settings[k] for k in ('env_file', 'management_url', 'follow_usage')}
    program = shared.REMOTE_USAGE_READER.replace("'claude'", "'codex'")
    selection = shared.remote_read(settings, program.replace('SETTINGS_JSON', repr(json.dumps(follow_remote)), 1))['selection']
    config_path = Path(settings['codexbar_config'])
    with Path(str(config_path) + '.lock').open('a') as config_lock:
        os.chmod(config_lock.name, 0o600)
        fcntl.flock(config_lock, fcntl.LOCK_EX)
        original = config_path.read_bytes()
        config = json.loads(original)
        provider = next(p for p in config['providers'] if p['id'] == 'codex')
        paths = provider.setdefault('codexProfileHomePaths', [])
        for mapping in mappings:
            if not mapping.get('codexbar_id') and mapping['profile_home'] not in paths:
                paths.append(mapping['profile_home'])
        if (selection.get('status') == 'observed' and selection.get('success') is True
                and selection.get('provider') == 'codex'
                and selection.get('client_key_id') == settings['follow_usage']['client_key_id']
                and selection.get('cpa_node_id') == settings['follow_usage']['cpa_node_id']):
            mapping = next((a for a in mappings if a['home_id'] == selection.get('credential_id')), None)
            if mapping:
                provider['codexActiveSource'] = (
                    {'kind': 'managedAccount', 'accountID': mapping['codexbar_id']} if mapping.get('codexbar_id')
                    else {'kind': 'liveSystem', 'homePath': mapping['profile_home']})
            elif selection.get('credential_id') == settings['codex_system_home_id']:
                provider['codexActiveSource'] = {'kind': 'liveSystem'}
        if config != json.loads(original):
            shared.atomic_write(config_path, shared.encoded(config), expected=original)
    result = {'ok': True, 'checked_at': dt.datetime.now(dt.timezone.utc).isoformat(), 'accounts': report,
              'selection': selection}
    shared.atomic_write(state / 'status.json', shared.encoded(result))
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--settings', required=True)
    args = parser.parse_args()
    settings = json.loads(Path(args.settings).read_bytes())
    state = Path(settings['codex_state_dir'])
    state.mkdir(parents=True, exist_ok=True, mode=0o700)
    with (Path(settings['state_dir']) / 'sync.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        blocked = state / 'auth-blocked'
        if blocked.exists():
            return 1
        try:
            print(json.dumps(run(settings, state)))
            return 0
        except Exception as error:
            message = str(error) if isinstance(error, shared.SyncError) else 'Codex sync failed; raw error suppressed'
            if message in ('management_http_401', 'management_http_403'):
                shared.atomic_write(blocked, b'Management authentication needs attention.\n')
            shared.atomic_write(state / 'status.json', shared.encoded({'ok': False, 'error': message}))
            print(json.dumps({'ok': False, 'error': message}))
            return 1


if __name__ == '__main__':
    sys.exit(main())
