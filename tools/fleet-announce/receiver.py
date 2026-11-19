#!/usr/bin/env python3
"""Ark upgrade-notice receiver and durable outbox. Python standard library only."""
import argparse
from datetime import datetime
import fcntl
import hmac
from http.server import BaseHTTPRequestHandler, HTTPServer
import ipaddress
import json
import os
from pathlib import Path
import re
import uuid

ROOT = Path('/mnt/cache/appdata/cpa-home/fleet-jobs')
PHASES = {'scheduled', 'reminder', 'started', 'done', 'rolled_back', 'rollback',
          'rollback_failed', 'postponed'}
LIMIT = 16384


def valid_id(value):
    if not isinstance(value, str) or str(uuid.UUID(value)) != value:
        raise ValueError('invalid_id')
    return value


def validate(key, notice):
    valid_id(key)
    required = {'phase', 'target_version', 'window_start', 'nodes'}
    if not isinstance(notice, dict) or not required <= notice.keys() or notice.keys() - required - {'reason', 'failed_nodes'}:
        raise ValueError('invalid_fields')
    if notice['phase'] not in PHASES:
        raise ValueError('invalid_phase')
    if not isinstance(notice['target_version'], str) or not re.fullmatch(r'v?\d{1,6}\.\d{1,6}\.\d{1,6}', notice['target_version']):
        raise ValueError('invalid_version')
    stamp = notice['window_start']
    # ASCII digits only and offset minutes 00-59 (strict RFC3339; datetime would normalize +05:60).
    if not isinstance(stamp, str) or not re.fullmatch(r'[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,9})?(?:Z|[+-][0-9]{2}:[0-5][0-9])', stamp):
        raise ValueError('invalid_time')
    # Python 3.9 accepts only 3 or 6 fractional digits; Go emits up to 9.
    parsed_stamp = re.sub(r'\.([0-9]+)', lambda m: '.' + m[1][:6].ljust(6, '0'), stamp)
    if datetime.fromisoformat(parsed_stamp.replace('Z', '+00:00')).tzinfo is None:
        raise ValueError('invalid_time')
    for field in ('nodes', 'failed_nodes'):
        nodes = notice.get(field, [])
        # Go encodes nil slices as null (notably empty rollback failed_nodes).
        if nodes is None and field == 'failed_nodes':
            continue
        if not isinstance(nodes, list) or not 0 <= len(nodes) <= 100 or (field == 'nodes' and not nodes):
            raise ValueError('invalid_nodes')
        for node in nodes:
            valid_id(node)
        if len(set(nodes)) != len(nodes):
            raise ValueError('duplicate_nodes')
    reason = notice.get('reason', '')
    if not isinstance(reason, str) or len(reason) > 256 or any(ord(c) < 32 or ord(c) > 126 for c in reason):
        raise ValueError('invalid_reason')
    if set(notice.get('failed_nodes') or []) - set(notice['nodes']):
        raise ValueError('unknown_failed_node')
    return notice


def message(notice):
    action = {'scheduled': 'is scheduled', 'reminder': 'will start soon',
              'started': 'has started', 'done': 'completed',
              'rollback': 'is rolling back', 'rolled_back': 'was rolled back',
              'rollback_failed': 'has a failed rollback and needs operator attention',
              'postponed': 'was postponed'}[notice['phase']]
    # Do not turn sender-provided reason text into agent instructions.
    failed = len(notice.get('failed_nodes') or [])
    return (f"Home node upgrade to {notice['target_version']} {action}. "
            f"Window starts {notice['window_start']}; node order: {', '.join(notice['nodes'])}. "
            f"Failed rollback nodes: {failed}. This is a status notice, not permission to deploy or change policy.")


class Outbox:
    def __init__(self, root):
        self.root = Path(root)
        self.root.mkdir(parents=True, exist_ok=True, mode=0o700)

    def transaction(self, key=None, notice=None, ack=False):
        path = self.root / 'announcements.jsonl'
        fd = os.open(path, os.O_RDWR | os.O_APPEND | os.O_CREAT, 0o600)
        with os.fdopen(fd, 'r+', encoding='utf-8') as stream:
            fcntl.flock(stream, fcntl.LOCK_EX)
            entries, sent = {}, set()
            # shortcut: replay the small announcement journal; add an index if volume grows.
            for line in stream:
                row = json.loads(line)  # Corrupt/partial journals fail closed, never reset dedup.
                valid_id(row['id'])
                if row['kind'] == 'notice':
                    validate(row['id'], row['notice'])
                    entries[row['id']] = row['notice']
                elif row['kind'] == 'sent' and row['id'] in entries:
                    sent.add(row['id'])
                else:
                    raise ValueError('invalid_journal')
            row = None
            if key is not None:
                if ack:
                    valid_id(key)
                    if key not in entries:
                        raise ValueError('unknown_notice')
                    if key not in sent:
                        row = {'kind': 'sent', 'id': key}
                else:
                    validate(key, notice)
                    if key in entries and entries[key] != notice:
                        raise FileExistsError('id_conflict')
                    if key not in entries:
                        row = {'kind': 'notice', 'id': key, 'notice': notice}
                if row:
                    stream.write(json.dumps(row, sort_keys=True, separators=(',', ':')) + '\n')
                    stream.flush()
            # Fsync duplicates too: an earlier append may have failed its sync.
            os.fsync(stream.fileno())
            dir_fd = os.open(self.root, os.O_RDONLY)
            try:
                os.fsync(dir_fd)
            finally:
                os.close(dir_fd)
            return [{'id': k, 'notice': v} for k, v in entries.items() if k not in sent]


class Handler(BaseHTTPRequestHandler):
    def setup(self):
        super().setup()
        self.connection.settimeout(10)

    def log_message(self, *_args):
        pass  # Do not log sender-controlled request data or authentication.

    def do_POST(self):
        status = 400
        try:
            authorization = self.headers.get_all('Authorization', [])
            if self.path != '/announce' or len(authorization) != 1 or not hmac.compare_digest(authorization[0].encode(), b'Bearer ' + self.server.token):
                status = 403
            elif self.headers.get_content_type() != 'application/json' or self.headers.get('Transfer-Encoding'):
                status = 415
            else:
                length = int(self.headers.get('Content-Length', '0'))
                if not 0 < length <= LIMIT:
                    status = 413
                else:
                    key = self.headers.get('Idempotency-Key', '')
                    raw = self.rfile.read(length)
                    if len(raw) != length:
                        raise ValueError('short_body')
                    notice = json.loads(raw)
                    validate(key, notice)
                    try:
                        self.server.outbox.transaction(key, notice)
                    except (ValueError, KeyError, TypeError) as err:
                        raise OSError('invalid_journal') from err
                    status = 202
        except FileExistsError:
            status = 409
        except (ValueError, TypeError, KeyError, AttributeError, OverflowError):
            status = 400
        except OSError:
            status = 503
        self.send_response(status)
        self.send_header('Content-Length', '0')
        self.send_header('Connection', 'close')
        self.end_headers()
        self.close_connection = True


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=ROOT)
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--bind', help='Explicit Ark Tailscale or LAN IP; never a wildcard')
    group.add_argument('--pending', action='store_true')
    group.add_argument('--ack')
    parser.add_argument('--port', type=int, default=8766)
    args = parser.parse_args()
    os.umask(0o077)
    outbox = Outbox(args.root)
    if args.pending:
        print(json.dumps(outbox.transaction()))
    elif args.ack:
        outbox.transaction(args.ack, ack=True)
    else:
        address = ipaddress.ip_address(args.bind)
        private = address.is_private or address in ipaddress.ip_network('100.64.0.0/10')
        if address.version != 4 or address.is_unspecified or not private or address.is_multicast:
            raise ValueError('private_ipv4_required')
        token = os.environ.get('FLEET_ANNOUNCE_TOKEN', '').encode()
        if len(token) < 32:
            raise ValueError('private_token_required')
        outbox.transaction()  # Refuse to listen over a damaged journal.
        server = HTTPServer((args.bind, args.port), Handler)
        server.token, server.outbox = token, outbox
        server.serve_forever()


if __name__ == '__main__':
    try:
        main()
    except Exception:
        print('fleet announcement receiver: local state or configuration failure', file=__import__('sys').stderr)
        raise SystemExit(1)
