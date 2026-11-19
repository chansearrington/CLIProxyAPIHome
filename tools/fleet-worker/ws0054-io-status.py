#!/usr/bin/env python3
"""One read-only Linux IO snapshot; only the isolated build-root packet is written."""
import datetime
import json
import os
import pathlib
import subprocess


NAMES = {'mover', 'mover.old', 'mover.php', 'mover.tuning', 'age_mover', 'move'}
NUMERIC_FIELDS = {
    'sbSynced', 'sbSyncErrs', 'mdNumDisabled', 'mdNumInvalid', 'mdResyncSize',
    'mdResyncCorr', 'mdResync', 'mdResyncPos', 'mdResyncDt', 'mdResyncDb',
}
STATUS_FIELDS = NUMERIC_FIELDS | {'mdState', 'mdResyncAction'}


def snapshot():
    found = []
    unreadable = 0
    try:
        for proc in pathlib.Path('/proc').iterdir():
            if not proc.name.isdigit():
                continue
            try:
                args = (proc / 'cmdline').read_bytes().split(b'\0')
            except FileNotFoundError:
                # A missing cmdline is quiet only when the process is confirmed gone.
                try:
                    proc.stat()
                except FileNotFoundError:
                    continue
                except OSError:
                    pass
                unreadable += 1
                continue
            except OSError:
                unreadable += 1
                continue
            matched = [pathlib.Path(a.decode(errors='replace')).name for a in args
                       if a and pathlib.Path(a.decode(errors='replace')).name in NAMES]
            if matched:
                exe = None
                try:
                    exe = pathlib.Path(os.readlink(proc / 'exe')).name
                except FileNotFoundError:
                    pass  # Optional metadata must not erase the positive argv observation.
                except OSError:
                    unreadable += 1
                found.append({'pid': int(proc.name), 'executable_basename': exe,
                              'script_basenames': matched})
    except OSError:
        unreadable += 1

    pidfile = {'path': '/var/run/mover.pid', 'exists': False, 'pid': None,
               'live': False, 'readable': True}
    try:
        with pathlib.Path(pidfile['path']).open('rb') as stream:
            pidfile['exists'] = True
            raw = stream.read(18)
            data = raw.strip()
        if len(raw) == 18 or not data.isdigit() or len(data) > 16 or not 0 < int(data) <= 2147483647:
            pidfile['readable'] = False
        else:
            pidfile['pid'] = int(data)
            try:
                (pathlib.Path('/proc') / str(pidfile['pid'])).stat()
                pidfile['live'] = True
            except FileNotFoundError:
                pass
    except FileNotFoundError:
        pass
    except OSError:
        pidfile['readable'] = False

    fields = {}
    rc = None
    parity_idle = None
    parity_error = None
    try:
        p = subprocess.run(['/usr/local/sbin/mdcmd', 'status'],
                           capture_output=True, text=True, timeout=15)
        rc = p.returncode
        if rc != 0:
            parity_error = 'nonzero'
        else:
            malformed = False
            for line in p.stdout.splitlines():
                key, sep, value = line.partition('=')
                if not sep or key not in STATUS_FIELDS:
                    continue
                if key in NUMERIC_FIELDS:
                    valid = value.isascii() and value.isdigit() and len(value) <= 20
                elif key == 'mdState':
                    valid = value in {'STARTED', 'STOPPED'}
                else:
                    valid = len(value) <= 64 and all(' ' <= char <= '~' for char in value)
                if not valid or key in fields:
                    malformed = True
                else:
                    fields[key] = value
            if malformed or fields.keys() != STATUS_FIELDS:
                fields = {}
                parity_error = 'malformed_status'
            elif fields['mdState'] == 'STARTED':
                parity_idle = int(fields['mdResync']) == 0
            else:
                parity_error = 'array_not_started'
    except FileNotFoundError:
        parity_error = 'missing'
    except PermissionError:
        parity_error = 'unreadable'
    except subprocess.TimeoutExpired:
        parity_error = 'timeout'
    except UnicodeError:
        parity_error = 'malformed_status'
    except OSError:
        parity_error = 'exec_failed'

    schedules = []
    for path in pathlib.Path('/etc/cron.d').glob('*'):
        try:
            if not path.is_file():
                continue
            for line in path.read_text(errors='replace').splitlines():
                if line.lstrip().startswith('#'):
                    continue
                parts = line.split()
                if len(parts) >= 6 and ('mover' in line or 'parity.check.tuning' in line):
                    schedules.append({'file': path.name, 'schedule_fields': parts[:5],
                                      'kind': 'mover' if 'mover' in line else 'parity-check-tuning'})
        except OSError:
            pass
    return {
        'utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
        'mover_processes': found,
        'mover_idle': not found and not pidfile['live']
                      if unreadable == 0 and pidfile['readable'] else None,
        'mover_pidfile': pidfile,
        'unreadable_process_count': unreadable,
        'process_filter': 'Exact argv basenames mover/mover.old/mover.php/mover.tuning/age_mover/move '
                          'plus mover.pid liveness; no pgrep self-match',
        'mdcmd_path': '/usr/local/sbin/mdcmd',
        'mdcmd_exit': rc,
        'parity_status_fields': fields,
        'parity_idle': parity_idle,
        'parity_error': parity_error,
        'cron_schedules': schedules,
        'production_writes': False,
    }


def main():
    result = snapshot()
    base = pathlib.Path('/mnt/cache/appdata/cpa-home-build/ws0054-20261118T2055Z')
    (base / 'io-status.json').write_text(json.dumps(result, indent=2))
    print(json.dumps(result), flush=True)


if __name__ == '__main__':
    main()
