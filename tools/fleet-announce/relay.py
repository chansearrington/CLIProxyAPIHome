#!/usr/bin/env python3
"""MacBook outbox relay, using ws-0056's existing hc Orchestrator delivery."""
import fcntl
import importlib.util
import json
from pathlib import Path
import subprocess
import sys

# Import sibling code under -I without modifying the search path.
spec = importlib.util.spec_from_file_location('fleet_receiver', Path(__file__).with_name('receiver.py'))
receiver = importlib.util.module_from_spec(spec)
spec.loader.exec_module(receiver)
SSH = ['/usr/bin/ssh', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=10',
       '-o', 'IdentitiesOnly=yes', '-o', 'IdentityAgent=none', '-i',
       str(Path.home() / '.ssh/cronos_ark'), 'root@100.110.133.6',
       '/usr/bin/python3', '-I', '/mnt/cache/appdata/cpa-home/fleet-announce/receiver.py']


def relay(rows, send, ack):
    for row in rows:
        receiver.validate(row['id'], row['notice'])
        if not send('orchestrator', receiver.message(row['notice'])):
            return False
        # shortcut: a crash after hc accepts but before ack may repeat one notice.
        ack(row['id'])
    return True


def remote(*args):
    return subprocess.run(SSH + list(args), capture_output=True, timeout=30, check=True).stdout


def main():
    root = Path.home() / 'Library/Application Support/cpa-fleet-announce'
    root.mkdir(parents=True, exist_ok=True, mode=0o700)
    with (root / 'relay.lock').open('a') as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return 0
        path = Path.home() / 'Library/Application Support/cpa-model-alert/alert.py'
        spec = importlib.util.spec_from_file_location('model_alert', path)
        alert = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(alert)
        rows = json.loads(remote('--pending'))
        ok = relay(rows, alert.send, lambda key: remote('--ack', key))
        print('fleet announcement relay: ' + ('accepted' if ok else 'delivery pending'))
        return 0 if ok else 1


if __name__ == '__main__':
    try:
        raise SystemExit(main())
    except Exception:
        print('fleet announcement relay: delivery or local state failure', file=sys.stderr)
        raise SystemExit(1)
