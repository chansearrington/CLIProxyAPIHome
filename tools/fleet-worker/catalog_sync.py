#!/usr/bin/env python3
"""Add public CPA Home models to OpenClaw without changing agent selections."""
import argparse
import copy
import datetime
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import urllib.request


class SyncError(Exception):
    pass


def catalog_ids(payload):
    rows = payload.get('data') if isinstance(payload, dict) else None
    if not isinstance(rows, list) or not rows or len(rows) > 10000:
        raise SyncError('invalid_catalog')
    ids = [r.get('id') if isinstance(r, dict) else None for r in rows]
    if any(not isinstance(i, str) or not i or any(c.isspace() or ord(c) < 32 for c in i)
           for i in ids) or len(set(ids)) != len(ids):
        raise SyncError('invalid_catalog')
    # Home's public listing includes private plugin names even for shared-only keys.
    return sorted(i for i in ids if i != 'agent-os-home-canary' and not i.startswith('copilot/'))


def updated_config(config, ids, metadata=None):
    result = copy.deepcopy(config)
    provider = result['models']['providers']['cpa-gui']
    if provider['baseUrl'] != 'http://127.0.0.1:18317/v1':
        raise SyncError('unexpected_endpoint')
    defaults = result['agents']['defaults']
    if defaults.get('modelPolicy', {}).get('allow') != ['cpa-gui/*']:
        raise SyncError('unexpected_model_policy')
    existing = [row['id'] for row in provider['models']]
    if len(existing) != len(set(existing)):
        raise SyncError('duplicate_configured_model')
    added = sorted(set(ids) - set(existing))
    provider['models'].extend({'id': i, 'name': i} for i in added)
    for row in provider['models']:
        info = (metadata or {}).get(row['id'], {})
        for key in ('reasoning', 'contextWindow', 'maxTokens', 'input'):
            if key in info:
                row.setdefault(key, copy.deepcopy(info[key]))
        if row.get('name') == row['id'] and info.get('name'):
            row['name'] = info['name']
    allowed = defaults.setdefault('models', {})
    for model in ids:
        allowed.setdefault('cpa-gui/' + model, {})
    return result, added


def atomic_write(path, content, expected):
    fd, name = tempfile.mkstemp(prefix='.cpa-model-', dir=str(path.parent))
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, 'wb') as output:
            output.write(content)
            output.flush()
            os.fsync(output.fileno())
        if path.read_bytes() != expected:
            raise SyncError('concurrent_config_change')
        os.replace(name, path)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def cli(args, config=None):
    env = dict(os.environ)
    env.pop('OP_SERVICE_ACCOUNT_TOKEN', None)
    if config:
        env['OPENCLAW_CONFIG_PATH'] = str(config)
    try:
        result = subprocess.run(['openclaw'] + args, env=env, capture_output=True, text=True, timeout=12)
    except subprocess.TimeoutExpired:
        raise SyncError('openclaw_command_timeout') from None
    if result.returncode:
        raise SyncError('openclaw_command_failed')
    try:
        return json.loads(result.stdout[result.stdout.index('{'):])
    except (ValueError, TypeError):
        raise SyncError('openclaw_response_invalid') from None


def fetch_ids(config):
    provider = config['models']['providers']['cpa-gui']
    if provider['baseUrl'] != 'http://127.0.0.1:18317/v1':
        raise SyncError('unexpected_endpoint')
    key = provider['apiKey']
    if key.startswith('${') and key.endswith('}'):
        key = os.environ.get(key[2:-1])
    if not key:
        raise SyncError('client_key_unavailable')
    request = urllib.request.Request(provider['baseUrl'] + '/models',
                                     headers={'Authorization': 'Bearer ' + key})
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(request, timeout=10) as response:
        if response.geturl() != request.full_url:
            raise SyncError('unexpected_redirect')
        raw = response.read(2 * 1024 * 1024 + 1)
    if len(raw) > 2 * 1024 * 1024:
        raise SyncError('catalog_too_large')
    return catalog_ids(json.loads(raw))


def verify_views(ids):
    for view in ('provider-config', 'configured'):
        result = cli(['gateway', 'call', 'models.list', '--json', '--timeout', '5000',
                      '--params', json.dumps({'view': view})])
        actual = {r['id'] for r in result.get('models', []) if r.get('provider') == 'cpa-gui'}
        if not set(ids) <= actual:
            raise SyncError('runtime_catalog_mismatch')
    if cli(['gateway', 'call', 'health', '--json', '--timeout', '5000']).get('ok') is not True:
        raise SyncError('gateway_unhealthy')


def reconcile(home, apply=False, metadata=None):
    path = home / '.openclaw/openclaw.json'
    if path.is_symlink():
        raise SyncError('unsafe_config_path')
    original = path.read_bytes()
    before = json.loads(original)
    ids = fetch_ids(before)
    after, added = updated_config(before, ids, metadata)
    report = {'added': added, 'available_count': len(ids), 'defaults_preserved': True,
              'retained_unlisted': sorted({r['id'] for r in before['models']['providers']['cpa-gui']['models']} - set(ids))}
    if not apply:
        return dict(report, status='would-update' if after != before else 'current')
    backup = None
    if after != before:
        directory = home / '.openclaw/backups'
        directory.mkdir(mode=0o700, exist_ok=True)
        backup = Path(tempfile.mkdtemp(prefix='cpa-catalog-', dir=str(directory)))
        os.chmod(backup, 0o700)
        for name, data in [('before.json', original), ('candidate.json', (json.dumps(after, indent=2)+'\n').encode())]:
            target = backup / name
            with target.open('xb') as output:
                os.chmod(target, 0o600)
                output.write(data)
        if cli(['config', 'validate', '--json'], backup/'candidate.json').get('valid') is not True:
            raise SyncError('schema_validation_failed')
        if fetch_ids(before) != ids:
            raise SyncError('catalog_changed_during_validation')
        atomic_write(path, (backup/'candidate.json').read_bytes(), original)
    try:
        # Health alone can go green before the gateway has reloaded its model catalog.
        deadline = time.monotonic() + 60
        for attempt in range(10):
            try:
                verify_views(ids)
                break
            except SyncError:
                if attempt == 9 or time.monotonic() >= deadline:
                    raise
                time.sleep(2)
    except Exception:
        if backup:
            # Restore only our two fields, retaining newer unrelated config edits.
            current_bytes = path.read_bytes()
            current = json.loads(current_bytes)
            if (current['models']['providers']['cpa-gui']['models'] != after['models']['providers']['cpa-gui']['models']
                    or current['agents']['defaults']['models'] != after['agents']['defaults']['models']):
                raise SyncError('runtime_failed_concurrent_model_change') from None
            current['models']['providers']['cpa-gui']['models'] = before['models']['providers']['cpa-gui']['models']
            current['agents']['defaults']['models'] = before['agents']['defaults']['models']
            atomic_write(path, (json.dumps(current, indent=2)+'\n').encode(), current_bytes)
        raise SyncError('runtime_failed_restored_model_fields') from None
    return dict(report, status='updated' if backup else 'current', backup=str(backup) if backup else None)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true')
    args = parser.parse_args()
    try:
        report = reconcile(Path.home(), args.apply, json.loads(os.environ.get('CPA_CATALOG_METADATA', '{}')))
    except Exception as error:
        report = {'status': 'blocked', 'code': str(error) if isinstance(error, SyncError) else 'sync_failed'}
    report['checked_at'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    print(json.dumps(report))
    return int(report['status'] == 'blocked')


if __name__ == '__main__':
    raise SystemExit(main())
