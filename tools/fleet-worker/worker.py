#!/usr/bin/env python3
"""Execute this node's typed Home tasks over its existing verified mTLS identity."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import socket
import ssl
import subprocess
import tarfile
import threading
import urllib.error
import urllib.request

import catalog_sync

HERE = Path(__file__).resolve().parent


class WorkerError(Exception):
    pass


class Blocked(WorkerError):
    pass


def rpc(settings, command, payload):
    cert = Path(settings['certificate_dir']).expanduser()
    context = ssl.create_default_context(cafile=str(cert/'home-ca-crt.pem'))
    context.load_cert_chain(str(cert/'client-crt.pem'), str(cert/'client-key.pem'))
    args = [command, 'fleet-plan' if command == 'GET' else 'fleet-status', json.dumps(payload)]
    encoded = [arg.encode() for arg in args]
    request = ('*'+str(len(encoded))+'\r\n').encode() + b''.join(
        ('$'+str(len(value))+'\r\n').encode()+value+b'\r\n' for value in encoded)
    with socket.create_connection((settings['host'], settings.get('port', 8327)), timeout=12) as raw:
        with context.wrap_socket(raw, server_hostname=settings['host']) as connection:
            connection.sendall(request)
            stream = connection.makefile('rb')
            try:
                header = stream.readline(4096)
                if not header.endswith(b'\r\n'):
                    raise WorkerError('invalid_home_response')
                if header.startswith(b'-'):
                    raise WorkerError('home_rejected_request')
                if header.startswith(b':'):
                    return int(header[1:-2])
                if not header.startswith(b'$'):
                    raise WorkerError('invalid_home_response')
                size = int(header[1:-2])
                if size < 0 or size > 4*1024*1024:
                    raise WorkerError('invalid_home_payload_size')
                content = stream.read(size)
                if len(content) != size or stream.read(2) != b'\r\n':
                    raise WorkerError('truncated_home_payload')
                return json.loads(content)
            finally:
                stream.close()


def inventory(settings):
    root = Path(settings['service_root']).expanduser()
    path = root/'cli-proxy-api'
    data = path.read_bytes()
    digest = hashlib.sha256(data).hexdigest()
    manifest = json.loads((root/'install-manifest.json').read_bytes())
    if manifest.get('core_sha256') == digest:
        version = manifest['core_version']
    else:
        result = subprocess.run([str(path), '-version'], capture_output=True, text=True, timeout=12)
        match = re.search(r'Version: (\d+\.\d+\.\d+),', result.stdout+result.stderr)
        if not match:
            raise WorkerError('unrecognized_installed_binary')
        version = match[1]
    config=Path.home()/'.openclaw/openclaw.json'
    catalog=False
    if config.exists():
        value=json.loads(config.read_bytes())
        catalog=value.get('models',{}).get('providers',{}).get('cpa-gui',{}).get('baseUrl')=='http://127.0.0.1:18317/v1'
    return {'version': version, 'sha256': digest, 'size': len(data), 'platform': 'darwin/arm64', 'catalog':catalog}


def client_key(settings):
    config = Path.home()/'.openclaw/openclaw.json'
    if config.exists() and json.loads(config.read_bytes()).get('models',{}).get('providers',{}).get('cpa-gui',{}).get('apiKey'):
        value = json.loads(config.read_bytes())['models']['providers']['cpa-gui']['apiKey']
        if value.startswith('${') and value.endswith('}'):
            value = os.environ.get(value[2:-1])
    else:
        value = (Path.home()/'.config/cpa-home/client-key').read_text().strip()
    if not value:
        raise WorkerError('client_key_unavailable')
    return value


def probe(settings):
    key = client_key(settings)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    for path, body in [('/v1/messages', {'model': 'claude-opus-5-5', 'max_tokens': 64,
                        'messages': [{'role': 'user', 'content': 'Reply exactly: home-fleet-ok'}]}),
                       ('/v1/responses', {'model': 'gpt-6.1-sol', 'input': 'Reply exactly: home-fleet-ok',
                        'reasoning': {'effort': 'low'}, 'max_output_tokens': 128, 'stream': False})]:
        request = urllib.request.Request('http://127.0.0.1:18317'+path, data=json.dumps(body).encode(),
                headers={'Authorization': 'Bearer '+key, 'x-api-key': key,
                         'Content-Type': 'application/json', 'anthropic-version': '2023-06-01'})
        try:
            with opener.open(request, timeout=90) as response:
                value = json.load(response)
        except urllib.error.HTTPError as error:
            if error.code == 429:
                raise Blocked('upstream_quota') from None
            raise WorkerError('native_probe_http_'+str(error.code)) from None
        text = (''.join(c.get('text', '') for c in value.get('content', [])) if path.endswith('messages')
                else ''.join(c.get('text', '') for item in value.get('output', [])
                             for c in item.get('content', []) if isinstance(c, dict)))
        if 'home-fleet-ok' not in text:
            raise WorkerError('native_probe_incomplete')


def post_upgrade_probe(settings):
    try:
        probe(settings)
    except Blocked:
        # Once a candidate is installed, quota prevents acceptance and requires rollback.
        raise WorkerError('post_update_quota_unverified') from None


class Lease:
    def __init__(self, settings, job):
        self.settings, self.job = settings, job
        self.stop, self.lost = threading.Event(), threading.Event()
        self.local = settings.get('agent')
        self.env = dict(os.environ, FLEET_LOCK_SESSION='home-fleet-'+job['id'],
                        FLEET_LOCK_ACTOR='cpa-home-fleet-worker', FLEET_LOCK_NONINTERACTIVE='1',
                        FLEET_LOCK_HOSTS_FILE=str(HERE/'fleet-hosts.conf'))
        if self.local:
            self.env.update(FLEET_LOCK_LOCAL_AGENT=self.local,
                            FLEET_LOCK_LOCAL_STATE_DIR=str(Path.home()/'.openclaw'))

    def lock(self, verb):
        args = [str(HERE/'fleet-lock'), verb, self.local]
        if verb == 'acquire':
            args.append('CPA Home scheduled fleet task')
        try:
            result = subprocess.run(args, env=self.env, capture_output=True, text=True, timeout=12)
            if verb == 'acquire':
                return result.returncode == 0 and any(a.startswith('acquired:') for a in result.stdout.splitlines())
            return result.returncode == 0
        except (subprocess.TimeoutExpired, OSError):
            return False

    def check(self):
        if self.lost.is_set() or (self.local and not self.lock('check')):
            raise WorkerError('local_fleet_lock_lost')

    def __enter__(self):
        if self.local and not self.lock('acquire'):
            raise Blocked('local_fleet_lock_busy')
        def renew():
            while not self.stop.wait(30):
                if self.local and not self.lock('heartbeat'):
                    self.lost.set()
                try:
                    rpc(self.settings, 'RPUSH', {'id': self.job['id'], 'token': self.job['token'], 'status': 'running'})
                except Exception:
                    pass  # A finished receipt is retained until Home acknowledges it.
        self.thread = threading.Thread(target=renew, daemon=True)
        self.thread.start()
        return self

    def __exit__(self, *_):
        self.stop.set()
        self.thread.join(timeout=40)
        if self.local:
            self.lock('release')


def install_binary(settings, job, state, lease):
    root = Path(settings['service_root']).expanduser()
    current = inventory(settings)
    release = job['release']
    rollback = job['operation'] == 'rollback'
    if rollback and current['sha256'] == release['sha256']:
        return
    if not rollback and current['sha256'] == release['sha256']:
        manifest=json.loads((root/'install-manifest.json').read_bytes())
        if manifest.get('core_sha256')!=release['sha256'] or manifest.get('core_version')!=release['version']:
            raise WorkerError('installed_binary_needs_reconciliation')
        post_upgrade_probe(settings)
        return
    if not rollback:
        if current['sha256'] != job['before']['sha256']:
            raise WorkerError('binary_changed_since_plan')
        probe(settings)  # Never start an update while the current Claude route cannot be verified.
        url = release['url']
        prefix = 'https://github.com/router-for-me/CLIProxyAPI/releases/download/v'+release['version']+'/'
        if not url.startswith(prefix) or not re.fullmatch(r'[a-f0-9]{64}', release['archive_sha256']):
            raise WorkerError('untrusted_release')
        archive = state/(release['version']+'.tar.gz')
        if not archive.exists():
            with urllib.request.urlopen(url, timeout=90) as response:
                data = response.read(256*1024*1024+1)
            if len(data) > 256*1024*1024:
                raise WorkerError('release_too_large')
            archive.write_bytes(data)
            archive.chmod(0o600)
        if hashlib.sha256(archive.read_bytes()).hexdigest() != release['archive_sha256']:
            raise WorkerError('archive_checksum_mismatch')
        with tarfile.open(archive) as tar:
            members = [m for m in tar.getmembers() if Path(m.name).name == 'cli-proxy-api' and m.isfile()]
            if len(members) != 1 or members[0].size != release['size']:
                raise WorkerError('binary_size_mismatch')
            content = tar.extractfile(members[0]).read()
        if hashlib.sha256(content).hexdigest() != release['sha256']:
            raise WorkerError('binary_checksum_mismatch')
        staged = state/('cli-proxy-api.'+release['version']+'.new')
        staged.write_bytes(content)
        staged.chmod(0o700)
        before, after = job['before'], release
    else:
        before, after = release, current
        staged = state/'unused-rollback-staging'
    values = {'FROM_VERSION': before['version'], 'FROM_SHA256': before['sha256'], 'FROM_SIZE': str(before['size']),
              'TO_VERSION': after['version'], 'TO_SHA256': after['sha256'], 'TO_SIZE': str(after['size']),
              'TO_COMMIT': 'home-verified-release', 'SERVICE_ROOT': str(root), 'NEW_PATH': str(staged),
              'SOURCE': 'cpa-home-fleet', 'REASON': 'home-managed-patch'}
    if rollback:
        values.update(ROLLBACK_ONLY='1', ROLLBACK_TRIGGER='home-managed-rollback')
    lease.check()
    result = subprocess.run(['/bin/sh', str(HERE/'upgrade-node-core.sh')], env=dict(os.environ, **values),
                            capture_output=True, text=True, timeout=230)
    expected = 3 if rollback else 0
    if result.returncode != expected:
        raise WorkerError('binary_operation_failed')
    if not rollback:
        post_upgrade_probe(settings)


def execute_job(settings, job, state):
    with Lease(settings, job) as lease:
        if job['operation'] == 'catalog':
            lease.check()
            try:
                report = catalog_sync.reconcile(Path.home(), apply=True, metadata=job.get('metadata', {}))
            except catalog_sync.SyncError as error:
                raise WorkerError('catalog_'+str(error)) from None
            if report['status'] not in ('updated', 'current'):
                raise WorkerError('catalog_not_verified')
        elif job['operation'] in ('upgrade', 'rollback'):
            install_binary(settings, job, state, lease)
        else:
            raise WorkerError('unsupported_home_operation')
        lease.check()


def run(settings, state):
    plan = rpc(settings, 'GET', inventory(settings))
    accepted=plan.get('accepted_release')
    if accepted and plan.get('verified_node_ids'):
        receipt={'schema':'home-fleet-accepted-core/v1','version':accepted['version'],'sha256':accepted['sha256'],
                 'size':accepted['size'],'verified_node_ids':plan['verified_node_ids']}
        target=state/'accepted-core.json'
        content=json.dumps(receipt,sort_keys=True)
        if not target.exists() or target.read_text()!=content:
            target.write_text(content);target.chmod(0o600)
    job = plan.get('job')
    if not job:
        return {'ok': True, 'status': plan['status']}
    if not re.fullmatch(r'[a-f0-9-]{36}', job['id']):
        raise WorkerError('invalid_job_id')
    receipt = state/(job['id']+'.json')
    if receipt.exists():
        result = json.loads(receipt.read_bytes())
    else:
        result = {'id': job['id'], 'token': job['token']}
        try:
            execute_job(settings, job, state)
            result.update(status='success', inventory=inventory(settings))
        except Blocked as error:
            result.update(status='blocked', error=str(error))
        except Exception as error:
            # Only fixed error vocabulary is allowed in the Home status report.
            result.update(status='failed', error=str(error) if isinstance(error, WorkerError) else type(error).__name__)
        receipt.write_text(json.dumps(result))
        receipt.chmod(0o600)
    rpc(settings, 'RPUSH', result)
    # Never print job tokens, credentials, prompts, upstream bodies or raw exceptions.
    return {'ok': result['status'] in ('success', 'blocked'), 'status': result['status'],
            'operation': job['operation'], 'job_id': job['id'], 'error': result.get('error')}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--settings', type=Path, required=True)
    args = parser.parse_args()
    settings = json.loads(args.settings.read_bytes())
    state = Path(settings['state_dir']).expanduser()
    state.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.umask(0o077)
    with (state/'run.lock').open('a') as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return 0
        try:
            result = run(settings, state)
        except Exception as error:
            result = {'ok': False, 'status': 'unavailable', 'error': str(error) if isinstance(error, WorkerError) else type(error).__name__}
        (state/'latest.json').write_text(json.dumps(result))
        print(json.dumps(result))
        return int(not result['ok'])


if __name__ == '__main__':
    raise SystemExit(main())
