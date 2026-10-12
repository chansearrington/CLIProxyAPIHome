#!/usr/bin/env python3
"""One-way Home access-token sync. Refresh tokens never leave the SSH host."""

import argparse
import copy
import datetime as dt
import fcntl
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import urllib.error
import urllib.request


class SyncError(Exception):
    pass


REMOTE_READER = r'''
import json, pathlib, sys, urllib.error, urllib.parse, urllib.request
settings = json.loads(SETTINGS_JSON)
try:
    values = {}
    for line in pathlib.Path(settings['env_file']).read_text().splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            values[key.strip()] = value.strip().strip("\"'")
    secret = values['MANAGEMENT_PASSWORD']
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    result = []
    for account in settings['accounts']:
        url = settings['management_url'] + '/credentials/download?' + urllib.parse.urlencode({'id': account['home_id']})
        request = urllib.request.Request(url, headers={'Authorization': 'Bearer ' + secret})
        with opener.open(request, timeout=12) as response:
            credential = json.load(response)
        if credential.get('type') != 'claude' or credential.get('email') != account['email']:
            raise ValueError('identity mismatch')
        # Construct an allowlist response; never serialize the original credential.
        result.append({'home_id': account['home_id'], 'email': credential['email'],
                       'access_token': credential['access_token'], 'expires_at': credential['expired']})
    print(json.dumps({'accounts': result}))
except urllib.error.HTTPError as error:
    print(json.dumps({'error': 'management_http_' + str(error.code)}))
    sys.exit(1)
except Exception:
    print(json.dumps({'error': 'remote_credential_read_failed'}))
    sys.exit(1)
'''

REMOTE_USAGE_READER = r'''
import datetime, json, pathlib, sys, urllib.error, urllib.parse, urllib.request
settings = json.loads(SETTINGS_JSON)
try:
    values = {}
    for line in pathlib.Path(settings['env_file']).read_text().splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            values[key.strip()] = value.strip().strip("\"'")
    follow = settings['follow_usage']
    since = datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(hours=follow.get('max_age_hours', 24))
    query = urllib.parse.urlencode({'provider': 'claude', 'status': 'success', 'limit': 1,
                                   'sort': 'timestamp_desc', 'from': since.isoformat(),
                                   'client_key_id': follow['client_key_id'],
                                   'cpa_node': follow['cpa_node_id']})
    request = urllib.request.Request(settings['management_url'] + '/usage/records?' + query,
                                    headers={'Authorization': 'Bearer ' + values['MANAGEMENT_PASSWORD']})
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(request, timeout=12) as response:
        items = json.load(response)['items']
    selection = {'status': 'idle'}
    if items:
        row = items[0]
        # Project identity only; no masked keys, prompts, payloads, or tokens.
        selection = {'status': 'observed', 'credential_id': row['credential']['credential_id'],
                     'usage_id': row['id'], 'timestamp': row['timestamp'],
                     'client_key_id': row['client']['api_key_id'],
                     'cpa_node_id': row['runtime']['cpa_node_id'], 'provider': row['provider'],
                     'success': row.get('failed') is False and row.get('status') == 'success'}
    print(json.dumps({'selection': selection}))
except urllib.error.HTTPError as error:
    if error.code in (401, 403):
        print(json.dumps({'error': 'management_http_' + str(error.code)}))
        sys.exit(1)
    print(json.dumps({'selection': {'status': 'unavailable'}}))
except Exception:
    print(json.dumps({'selection': {'status': 'unavailable'}}))
'''


def remote_program(settings):
    remote = {key: settings[key] for key in ('env_file', 'management_url', 'accounts')}
    return REMOTE_READER.replace('SETTINGS_JSON', repr(json.dumps(remote)), 1)


def remote_read(settings, program):
    result = subprocess.run(
        ['/usr/bin/ssh', '-T', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes',
         '-o', 'ConnectTimeout=10', '-o', 'ServerAliveInterval=10',
         '-o', 'ServerAliveCountMax=2', settings['ssh_host'], '/usr/bin/python3', '-'],
        input=program, text=True, capture_output=True, timeout=45)
    try:
        payload = json.loads(result.stdout)
    except (ValueError, TypeError):
        raise SyncError('SSH credential read failed; response and stderr suppressed') from None
    if result.returncode or 'error' in payload:
        # Only expose errors from our fixed remote error vocabulary.
        error = payload.get('error', '')
        if error not in ('management_http_401', 'management_http_403'):
            error = 'remote credential read failed'
        raise SyncError(error)
    return payload


def fetch_accounts(settings):
    return remote_read(settings, remote_program(settings))


def validate_follow_settings(settings):
    follow = settings.get('follow_usage')
    if follow is None:
        return None
    if (not isinstance(follow, dict) or type(follow.get('client_key_id')) is not int
            or follow['client_key_id'] <= 0
            or not isinstance(follow.get('cpa_node_id'), str) or not follow['cpa_node_id'].strip()
            or type(follow.get('max_age_hours', 24)) is not int
            or not 1 <= follow.get('max_age_hours', 24) <= 168):
        raise SyncError('invalid follow_usage settings')
    return follow


def fetch_selection(settings):
    remote = {key: settings[key] for key in ('env_file', 'management_url', 'follow_usage')}
    program = REMOTE_USAGE_READER.replace('SETTINGS_JSON', repr(json.dumps(remote)), 1)
    return remote_read(settings, program)


def follow_selection(config, settings, payload, now=None):
    follow = validate_follow_settings(settings)
    if follow is None:
        return {'mode': 'disabled'}, False
    row = payload.get('selection') if isinstance(payload, dict) else None
    if not isinstance(row, dict):
        return {'mode': 'unavailable'}, False
    if row.get('status') in ('idle', 'unavailable'):
        return {'mode': row['status']}, False
    if (row.get('status') != 'observed' or row.get('provider') != 'claude'
            or row.get('success') is not True or type(row.get('client_key_id')) is not int
            or row['client_key_id'] != follow['client_key_id']
            or row.get('cpa_node_id') != follow['cpa_node_id']):
        return {'mode': 'ignored', 'reason': 'scope_or_status_mismatch'}, False
    try:
        timestamp = dt.datetime.fromisoformat(row['timestamp'].replace('Z', '+00:00'))
        now = now or dt.datetime.now(dt.timezone.utc)
        if timestamp.tzinfo is None or timestamp > now + dt.timedelta(seconds=60):
            raise ValueError()
        if timestamp < now - dt.timedelta(hours=follow.get('max_age_hours', 24)):
            return {'mode': 'stale'}, False
    except (KeyError, ValueError, TypeError, AttributeError):
        return {'mode': 'ignored', 'reason': 'invalid_timestamp'}, False
    mapping = next((a for a in settings['accounts'] if a['home_id'] == row.get('credential_id')), None)
    if mapping is None:
        return {'mode': 'unmapped'}, False
    claude = next(p for p in config['providers'] if p['id'] == 'claude')
    stored = claude['tokenAccounts']
    index = next(i for i, a in enumerate(stored['accounts']) if a['id'] == mapping['codexbar_id'])
    changed = stored.get('activeIndex') != index
    stored['activeIndex'] = index
    return {'mode': 'following', 'label': mapping['label'], 'email': mapping['email'],
            'observed_at': row['timestamp'], 'selection_changed': changed}, changed


def validate_accounts(settings, payload, now=None):
    now = now or dt.datetime.now(dt.timezone.utc)
    if set(payload) != {'accounts'} or not isinstance(payload['accounts'], list):
        raise SyncError('unexpected credential response')
    expected = {a['home_id']: a for a in settings['accounts']}
    if len(expected) != len(settings['accounts']):
        raise SyncError('duplicate Home account mapping')
    found = {}
    for item in payload['accounts']:
        if set(item) != {'home_id', 'email', 'access_token', 'expires_at'}:
            raise SyncError('unexpected credential fields')
        account = expected.get(item['home_id'])
        if account is None or item['home_id'] in found or item['email'] != account['email']:
            raise SyncError('Home account identity mismatch')
        token = item['access_token']
        if not isinstance(token, str) or not token.startswith('sk-ant-oat') or any(c.isspace() for c in token):
            raise SyncError('invalid Claude access token')
        try:
            expiry = dt.datetime.fromisoformat(item['expires_at'].replace('Z', '+00:00'))
            if expiry.tzinfo is None or expiry <= now + dt.timedelta(seconds=60):
                raise ValueError()
        except (ValueError, TypeError, AttributeError):
            raise SyncError('Home access token expired or near expiry') from None
        found[item['home_id']] = item
    if set(found) != set(expected):
        raise SyncError('missing Home account')
    return found


def verify_profile(token, email):
    request = urllib.request.Request(
        'https://api.anthropic.com/api/oauth/profile',
        headers={'Authorization': 'Bearer ' + token, 'anthropic-beta': 'oauth-2025-04-20'})
    try:
        with urllib.request.urlopen(request, timeout=15) as response:
            profile = json.load(response)
    except Exception:
        raise SyncError('Anthropic profile verification failed; response suppressed') from None
    if profile.get('account', {}).get('email') != email:
        raise SyncError('Anthropic account identity mismatch')


def patched_config(config, settings, accounts):
    updated = copy.deepcopy(config)
    providers = [p for p in updated.get('providers', []) if p.get('id') == 'claude']
    if len(providers) != 1:
        raise SyncError('expected one Claude provider')
    stored = providers[0].get('tokenAccounts', {}).get('accounts', [])
    changed = []
    seen = set()
    for mapping in settings['accounts']:
        matches = [a for a in stored if a.get('id') == mapping['codexbar_id']]
        if len(matches) != 1 or mapping['codexbar_id'] in seen:
            raise SyncError('CodexBar account mapping missing or duplicated')
        seen.add(mapping['codexbar_id'])
        account = matches[0]
        token = accounts[mapping['home_id']]['access_token']
        if account.get('token') != token:
            account['token'] = token
            changed.append(mapping)
    return updated, changed


def atomic_write(path, data, expected=None):
    path = Path(path)
    descriptor, temporary = tempfile.mkstemp(prefix='.' + path.name + '.', dir=str(path.parent))
    try:
        os.fchmod(descriptor, 0o600)
        with os.fdopen(descriptor, 'wb') as output:
            output.write(data)
            output.flush()
            os.fsync(output.fileno())
        if expected is not None and path.read_bytes() != expected:
            raise SyncError('config changed concurrently; retry on next sync')
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def encoded(value):
    return (json.dumps(value, indent=2, ensure_ascii=False) + '\n').encode()


def sync(settings, state, stage=None, check=False):
    follow = validate_follow_settings(settings)
    payload = fetch_accounts(settings)
    accounts = validate_accounts(settings, payload)
    path = Path(settings['codexbar_config']).expanduser()
    if path.is_symlink():
        raise SyncError('CodexBar config must be a regular file')
    original = path.read_bytes()
    config = json.loads(original)
    updated, changed = patched_config(config, settings, accounts)
    selection_payload = None
    if follow is not None:
        try:
            selection_payload = fetch_selection(settings)
        except SyncError as error:
            if str(error) in ('management_http_401', 'management_http_403'):
                raise
        except (OSError, subprocess.SubprocessError):
            pass
    selection, selection_changed = follow_selection(updated, settings, selection_payload)
    for mapping in changed:
        verify_profile(accounts[mapping['home_id']]['access_token'], mapping['email'])
    if stage:
        atomic_write(stage, encoded(updated))
    elif not check and (changed or selection_changed):
        backup = state / 'original-tokens.json'
        if changed and not backup.exists():
            claude = next(p for p in config['providers'] if p['id'] == 'claude')
            managed = {a['codexbar_id'] for a in settings['accounts']}
            originals = {a['id']: a['token'] for a in claude['tokenAccounts']['accounts'] if a['id'] in managed}
            atomic_write(backup, encoded(originals))
        atomic_write(path, encoded(updated), expected=original)
    # No credentials, fingerprints, or raw exceptions in status/logs.
    report = {'ok': True, 'mode': 'stage' if stage else 'check' if check else 'sync',
              'checked_at': dt.datetime.now(dt.timezone.utc).isoformat(),
              'changed_accounts': [a['label'] for a in changed],
              'selection': selection,
              'accounts': [{'label': a['label'], 'email': a['email'],
                            'expires_at': accounts[a['home_id']]['expires_at']} for a in settings['accounts']]}
    atomic_write(state / 'status.json', encoded(report))
    return report


def restore(settings, state):
    originals = json.loads((state / 'original-tokens.json').read_bytes())
    path = Path(settings['codexbar_config']).expanduser()
    original = path.read_bytes()
    config = json.loads(original)
    claude = next(p for p in config['providers'] if p['id'] == 'claude')
    restored = set()
    for account in claude['tokenAccounts']['accounts']:
        if account['id'] in originals:
            account['token'] = originals[account['id']]
            restored.add(account['id'])
    if restored != set(originals):
        raise SyncError('rollback account mapping missing')
    atomic_write(path, encoded(config), expected=original)
    return {'ok': True, 'mode': 'restore', 'accounts_restored': len(restored)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--settings', required=True)
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument('--check', action='store_true')
    modes.add_argument('--stage', type=Path)
    modes.add_argument('--restore', action='store_true')
    args = parser.parse_args()
    settings = json.loads(Path(args.settings).read_bytes())
    state = Path(settings['state_dir']).expanduser()
    state.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(state, 0o700)
    with (state / 'sync.lock').open('a') as lock:
        os.chmod(lock.name, 0o600)
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return 0
        blocked = state / 'auth-blocked'
        if blocked.exists() and not args.restore:
            print(json.dumps({'ok': False, 'error': 'management authentication blocked; fix key then remove auth-blocked'}))
            return 1
        try:
            report = restore(settings, state) if args.restore else sync(settings, state, args.stage, args.check)
            print(json.dumps(report))
            return 0
        except Exception as error:
            message = str(error) if isinstance(error, SyncError) else 'sync failed; raw error suppressed'
            if message in ('management_http_401', 'management_http_403'):
                atomic_write(blocked, b'Management authentication needs attention.\n')
            report = {'ok': False, 'error': message,
                      'checked_at': dt.datetime.now(dt.timezone.utc).isoformat()}
            atomic_write(state / 'status.json', encoded(report))
            print(json.dumps(report))
            return 1


if __name__ == '__main__':
    sys.exit(main())
