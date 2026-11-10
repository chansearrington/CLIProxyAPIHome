#!/usr/bin/env python3
"""Reconcile CPA model menus without changing operator model selections."""
import argparse
import copy
import datetime
import json
import os
from pathlib import Path
import re
import shutil
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
        if (path.read_bytes() if path.exists() else None) != expected:
            raise SyncError('concurrent_config_change')
        os.replace(name, path)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def retain_two_backups(directory, prefix):
    backups = sorted(directory.glob(prefix + '*'),
                     key=lambda entry: entry.stat().st_mtime_ns, reverse=True)
    for obsolete in backups[2:]:
        if obsolete.is_dir() and not obsolete.is_symlink():
            shutil.rmtree(obsolete)


def cli(args, config=None):
    env = dict(os.environ)
    env.pop('OP_SERVICE_ACCOUNT_TOKEN', None)
    if config:
        env['OPENCLAW_CONFIG_PATH'] = str(config)
    try:
        result = subprocess.run(['/bin/zsh','-lc','exec openclaw "$@"','--'] + args, env=env, capture_output=True, text=True, timeout=12)
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


def has_openclaw_target(home):
    path = home / '.openclaw/openclaw.json'
    return (path.is_file() and not path.is_symlink()
            and json.loads(path.read_bytes()).get('models', {}).get('providers', {})
            .get('cpa-gui', {}).get('baseUrl') == 'http://127.0.0.1:18317/v1')


def has_claude_target(home):
    path = home / '.claude/settings.json'
    return (path.is_file() and not path.is_symlink()
            and json.loads(path.read_bytes()).get('env', {}).get('ANTHROPIC_BASE_URL')
            in ('http://127.0.0.1:18317', 'http://127.0.0.1:18317/'))


def has_catalog_target(home):
    return has_openclaw_target(home) or has_claude_target(home)


def claude_picker_options(ids, metadata=None):
    """Use the newest native model per family, excluding encoded provider aliases."""
    families = {}
    for identifier in ids:
        match = re.fullmatch(r'claude-(fable|opus|sonnet|haiku)-(\d+(?:-\d+)*)', identifier)
        if not match:
            continue
        family, version = match.groups()
        parts = version.split('-')
        dated = len(parts[-1]) == 8
        normalized = parts[:-1] if dated else parts
        rank = (tuple(int(part) for part in normalized), not dated)
        if family not in families or rank > families[family][0]:
            families[family] = (rank, identifier, normalized)
    result = []
    for family in ('fable', 'opus', 'sonnet', 'haiku'):
        if family not in families:
            continue
        _, identifier, version = families[family]
        info = (metadata or {}).get(identifier, {})
        context = info.get('contextWindow', 0)
        million = isinstance(context, int) and context >= 1_000_000
        result.append({'model': identifier + ('[1m]' if million else ''),
                       'label': family.title() + ' ' + '.'.join(version),
                       'description': 'Claude subscription through CPA'
                       + ('; 1,000,000-token context' if million else '')})
    return result


def updated_claude_config(config, generated, owned):
    result = copy.deepcopy(config)
    picker = result.setdefault('modelPicker', {})
    if not isinstance(picker, dict) or not isinstance(picker.get('options', []), list):
        raise SyncError('invalid_claude_picker')
    options = picker.get('options', [])
    if any(not isinstance(row, dict) or not isinstance(row.get('model'), str) for row in options):
        raise SyncError('invalid_claude_picker')
    # A modified generated row becomes user-owned; never remove or overwrite it.
    custom = [row for row in options if row not in owned]
    existing = {row['model'] for row in custom}
    managed = [row for row in generated if row['model'] not in existing]
    picker['options'] = managed + custom
    return result, managed


def reconcile_claude(home, ids, apply=False, metadata=None):
    path = home / '.claude/settings.json'
    if not has_claude_target(home):
        raise SyncError('unexpected_claude_endpoint')
    original = path.read_bytes()
    before = json.loads(original)
    if before.get('env', {}).get('ANTHROPIC_BASE_URL') not in (
            'http://127.0.0.1:18317', 'http://127.0.0.1:18317/'):
        raise SyncError('unexpected_claude_endpoint')
    state = home / '.openclaw/state/cpa-home-fleet-worker/claude-picker.json'
    if state.is_symlink():
        raise SyncError('unsafe_claude_picker_state')
    old_state = state.read_bytes() if state.exists() else None
    ownership = json.loads(old_state) if old_state is not None else {'schema': 1, 'options': []}
    if (ownership.get('schema') != 1 or not isinstance(ownership.get('options'), list)
            or any(not isinstance(row, dict) or not isinstance(row.get('model'), str)
                   for row in ownership['options'])):
        raise SyncError('invalid_claude_picker_state')
    generated = claude_picker_options(ids, metadata)
    if not generated:
        raise SyncError('no_native_claude_models')
    after, managed = updated_claude_config(before, generated, ownership['options'])
    next_state = (json.dumps({'schema': 1, 'options': managed}, indent=2) + '\n').encode()
    changed = before != after
    report = {'status': 'would-update' if changed else 'current',
              'models': [row['model'] for row in managed], 'defaults_preserved': True}
    if not apply:
        return report
    state.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    backup = None
    if changed:
        directory = state.parent / 'catalog-backups'
        directory.mkdir(mode=0o700, exist_ok=True)
        backup = Path(tempfile.mkdtemp(prefix='claude-picker-', dir=str(directory)))
        for name, content in [('settings-before.json', original),
                              ('ownership-before.json', old_state or b'null\n')]:
            target = backup / name
            target.write_bytes(content)
            target.chmod(0o600)
        candidate = (json.dumps(after, indent=2, ensure_ascii=False) + '\n').encode()
        atomic_write(path, candidate, original)
    try:
        if old_state != next_state:
            atomic_write(state, next_state, old_state)
    except Exception:
        if changed:
            current_bytes = path.read_bytes()
            current = json.loads(current_bytes)
            if current.get('modelPicker', {}).get('options') != after['modelPicker']['options']:
                raise SyncError('claude_picker_rollback_conflict') from None
            if 'options' in before.get('modelPicker', {}):
                current['modelPicker']['options'] = before['modelPicker']['options']
            else:
                current['modelPicker'].pop('options', None)
                if not current['modelPicker'] and 'modelPicker' not in before:
                    current.pop('modelPicker')
            atomic_write(path, (json.dumps(current, indent=2) + '\n').encode(), current_bytes)
        raise
    if changed:
        retain_two_backups(state.parent / 'catalog-backups', 'claude-picker-')
    return dict(report, status='updated' if changed else 'current',
                backup=str(backup) if backup else None)


def reconcile_openclaw(home, apply=False, metadata=None):
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
    if backup:
        retain_two_backups(backup.parent, 'cpa-catalog-')
    return dict(report, status='updated' if backup else 'current', backup=str(backup) if backup else None)


def reconcile(home, apply=False, metadata=None, client_key=None):
    reports = {}
    if has_claude_target(home):
        # Use the original key's local CPA view, never the global metadata alone.
        key = client_key
        if not key:
            key_path = home / '.config/cpa-home/client-key'
            if key_path.is_file():
                key = key_path.read_text().strip()
        if not key:
            raise SyncError('client_key_unavailable')
        config = {'models': {'providers': {'cpa-gui': {
            'baseUrl': 'http://127.0.0.1:18317/v1', 'apiKey': key}}}}
        ids = fetch_ids(config)
        reports['claude'] = reconcile_claude(home, ids, apply, metadata)
    # A slow OpenClaw validation must not prevent the independent Claude picker update.
    # Still fail the overall Home task if either configured target cannot be verified.
    if has_openclaw_target(home):
        reports['openclaw'] = reconcile_openclaw(home, apply, metadata)
    if not reports:
        raise SyncError('no_catalog_target')
    status = ('updated' if any(r['status'] == 'updated' for r in reports.values())
              else 'would-update' if any(r['status'] == 'would-update' for r in reports.values())
              else 'current')
    return {'status': status, 'targets': reports, 'defaults_preserved': True}


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
