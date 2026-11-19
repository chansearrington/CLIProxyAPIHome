"""Synthetic only: no SSH, fleet keys, production files or inference."""
from contextlib import closing
import importlib.util
import json
from pathlib import Path
import sqlite3
import subprocess
import sys
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location('acceptance', Path(__file__).with_name('ws0054-acceptance.py'))
proof = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(proof)
REQUESTS = ['01990a00-0000-7000-8000-000000000001', '01990a00-0000-7000-8000-000000000002']
BOUNDARY = {'id': 10, 'timestamp': '2026-11-20 08:00:00', 'container_id': 'a'*64,
            'started_at': '2026-11-20T07:59:00Z', 'expected_nodes': ['node-1']}


class AcceptanceTest(unittest.TestCase):
    def test_native_http_and_no_key_in_result(self):
        seen = []
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass
            def do_GET(self):
                self.send_response(200); self.end_headers()
                self.wfile.write(json.dumps({'data': [{'id': 'claude-haiku-4-5-20251001'},
                    {'id': 'claude-haiku-5-5'}, {'id': 'gpt-6.2-sol'},
                    {'id': 'encoded/claude-haiku-99-99'}]}).encode())
            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                seen.append((self.path, body, self.headers['Authorization']))
                i = len(seen)-1
                self.send_response(200)
                self.send_header('X-CPA-TRACE-ID', '20261120080001-fake-auth-index-'+REQUESTS[i])
                self.end_headers()
                value = ({'id': 'msg_synthetic', 'stop_reason': 'end_turn',
                          'content': [{'type': 'text', 'text': 'DO_NOT_SAVE'}]} if i == 0 else
                         {'id': 'resp_synthetic', 'status': 'completed', 'output': [
                             {'type': 'message', 'content': [{'type': 'output_text', 'text': 'DO_NOT_SAVE'}]}]})
                self.wfile.write(json.dumps(value).encode())
        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True); thread.start()
        try:
            with tempfile.TemporaryDirectory() as directory:
                home = Path(directory); (home/'.openclaw').mkdir()
                (home/'.openclaw/openclaw.json').write_text(json.dumps({'models': {'providers': {
                    'renamed-provider': {'baseUrl': 'http://127.0.0.1:18317/v1', 'apiKey': 'fake-test-only-key'}}}}))
                with patch.object(proof.Path, 'home', return_value=home), patch.object(proof, 'NODE_URL',
                        'http://127.0.0.1:'+str(server.server_port)):
                    result = proof.node_probe({})
            self.assertEqual([p['request_id'] for p in result], REQUESTS)
            self.assertEqual([p[0] for p in seen], ['/v1/messages', '/v1/responses'])
            self.assertEqual(seen[0][1]['model'], 'claude-haiku-5-5')
            self.assertEqual(seen[0][1]['max_tokens'], 8)
            self.assertEqual(seen[1][1]['model'], 'gpt-6.2-sol')
            self.assertEqual(seen[1][1]['reasoning'], {'effort': 'low'})
            self.assertEqual(seen[1][1]['max_output_tokens'], 1024)
            self.assertNotIn('fake-test-only-key', json.dumps(result))
            self.assertNotIn('DO_NOT_SAVE', json.dumps(result))
        finally:
            server.shutdown(); server.server_close(); thread.join()

    def test_rejects_empty_unfinished_or_id_only_native_answers(self):
        catalog = {'data': [{'id': 'claude-haiku-5-5'}, {'id': 'gpt-6.2-sol'}]}
        claude = {'id': 'msg_synthetic', 'stop_reason': 'end_turn',
                  'content': [{'type': 'text', 'text': 'DO_NOT_SAVE'}]}
        codex = {'id': 'resp_synthetic', 'status': 'completed', 'output': [
            {'type': 'message', 'content': [{'type': 'output_text', 'text': 'DO_NOT_SAVE'}]}]}
        invalid = [
            ('claude', {'id': 'msg_synthetic'}),
            ('claude', dict(claude, content=[])),
            ('claude', dict(claude, content=[{'type': 'thinking', 'thinking': 'DO_NOT_SAVE'}])),
            ('claude', dict(claude, content=[{'type': 'text', 'text': '  '}])),
            ('claude', dict(claude, stop_reason=None)),
            ('claude', dict(claude, stop_reason='error')),
            ('codex', {'id': 'resp_synthetic'}),
            ('codex', dict(codex, status='incomplete', output=[])),
            ('codex', dict(codex, status='incomplete')),
            ('codex', dict(codex, output=[])),
            ('codex', dict(codex, output=[{'type': 'reasoning', 'summary': []}])),
            ('codex', dict(codex, output=[{'type': 'message', 'content': [
                {'type': 'output_text', 'text': ''}]}])),
        ]
        for provider, value in invalid:
            replies = [(catalog, {}, 1)]
            answers = [value] if provider == 'claude' else [claude, value]
            replies += [(answer, {'X-CPA-TRACE-ID': '20261120080001-fake-'+REQUESTS[i]}, 1)
                        for i, answer in enumerate(answers)]
            with self.subTest(provider=provider, value=value), \
                    patch.object(proof, 'own_key', return_value='fake-test-only-key'), \
                    patch.object(proof, 'request_json', side_effect=replies), \
                    self.assertRaisesRegex(proof.ProofError, '^native_answer_missing_or_unfinished$'):
                proof.node_probe({})

    def rows(self):
        c = sqlite3.connect(':memory:'); c.row_factory = sqlite3.Row
        c.executescript('''CREATE TABLE auth(uuid TEXT, disabled INTEGER, status TEXT, deleted_at TEXT, provider TEXT);
            CREATE TABLE usage(id INTEGER, timestamp TEXT, created_at TEXT, request_id TEXT,
              cpa_node_id TEXT, provider TEXT, model TEXT, failed INTEGER, quota_credential_id TEXT);
            INSERT INTO auth VALUES ('fake-credential-uuid',0,'active',NULL,'claude');''')
        c.execute('INSERT INTO usage VALUES (?,?,?,?,?,?,?,?,?)', (11, '2026-11-20 08:00:01.1',
            '2026-11-20 08:00:02', REQUESTS[0], 'node-1', 'claude', 'claude-haiku-5-5', 0, 'fake-credential-uuid'))
        c.execute('INSERT INTO usage VALUES (?,?,?,?,?,?,?,?,?)', (99, '2026-11-20 08:00:03',
            '2026-11-20 08:00:03', REQUESTS[1], 'ambient-node', 'codex', 'gpt-6.2-sol', 0, 'fake-credential-uuid'))
        rows = proof.usage_rows(c, BOUNDARY, [REQUESTS[0]])
        c.close()
        return rows

    def probe(self):
        return {'request_id': REQUESTS[0], 'node_id': 'node-1', 'provider': 'claude',
                'model': 'claude-haiku-5-5', 'http_status': 200, 'latency_ms': 1, 'response_id': 'msg_synthetic'}

    def test_exact_usage_join(self):
        rows = self.rows()
        self.assertEqual(len(rows), 1)
        result = proof.verify_usage([self.probe()], rows, BOUNDARY)
        self.assertEqual(result[0]['usage_id'], 11)
        self.assertEqual(len(result[0]['credential_label']), 6)
        self.assertNotIn('credential_uuid', result[0])

    def test_rejects_old_misattributed_disabled_and_duplicate_usage(self):
        row = self.rows()[0]
        for field, value in [('id', 10), ('timestamp', '2026-11-20 07:59:59'),
                             ('created_at', '2026-11-20 07:59:59'), ('node_id', 'wrong-node'),
                             ('provider', 'codex'), ('model', 'wrong-model'), ('failed', 1),
                             ('auth_disabled', 1), ('auth_status', 'error'), ('auth_deleted', 'now'),
                             ('auth_provider', 'codex'), ('credential_uuid', None)]:
            with self.subTest(field=field), self.assertRaises(proof.ProofError):
                proof.verify_usage([self.probe()], [dict(row, **{field: value})], BOUNDARY)
        with self.assertRaises(proof.ProofError):
            proof.verify_usage([self.probe()], [row, row], BOUNDARY)
        with self.assertRaises(proof.ProofError):
            proof.verify_usage([self.probe()], [], BOUNDARY)

    def test_transport_uses_stdin_not_secrets_or_source_in_argv(self):
        def fake_run(args, **kw):
            self.assertNotIn('def node_probe', ' '.join(args))
            self.assertNotIn('fake-test-only-key', ' '.join(args))
            payload = json.loads(kw['input'])
            self.assertEqual(payload['function'], 'node_probe')
            self.assertEqual(payload['payload'], {})
            self.assertIn('def node_probe', payload['source'])
            return subprocess.CompletedProcess(args, 0, '[{"synthetic":true}]', '')
        with patch.object(proof.subprocess, 'run', side_effect=fake_run):
            self.assertEqual(proof.remote('moxy@100.110.240.90', 'node_probe', {}), [{'synthetic': True}])

    def test_remote_failure_is_sanitized(self):
        failed = subprocess.CompletedProcess([], 1, 'fake-secret-out', 'fake-secret-err')
        with patch.object(proof.subprocess, 'run', return_value=failed):
            with self.assertRaisesRegex(proof.ProofError, '^transport_failed$'):
                proof.remote('moxy@100.110.240.90', 'node_probe', {})
        failed.stdout = '{"error":"http_status_429"}'
        with patch.object(proof.subprocess, 'run', return_value=failed):
            with self.assertRaisesRegex(proof.ProofError, '^node_probe_http_status_429$'):
                proof.remote('moxy@100.110.240.90', 'node_probe', {})

    def test_redirect_does_not_forward_key(self):
        seen = []
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass
            def do_GET(self):
                seen.append(self.path)
                self.send_response(302)
                self.send_header('Location', '/must-not-follow')
                self.end_headers()
                self.wfile.write(b'fake-private-error-body')
        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True); thread.start()
        try:
            with self.assertRaisesRegex(proof.ProofError, '^http_status_302$'):
                proof.request_json('http://127.0.0.1:'+str(server.server_port)+'/redirect', 'fake-key')
            self.assertEqual(seen, ['/redirect'])
        finally:
            server.shutdown(); server.server_close(); thread.join()

    def test_missing_trace_is_not_ambient_success(self):
        catalog = {'data': [{'id': 'claude-haiku-5-5'}, {'id': 'gpt-6.2-sol'}]}
        with patch.object(proof, 'own_key', return_value='fake-test-only-key'), patch.object(proof, 'request_json',
                side_effect=[(catalog, {}, 1), ({'id': 'msg_synthetic'}, {}, 1)]):
            with self.assertRaisesRegex(proof.ProofError, 'response_trace_missing_or_invalid'):
                proof.node_probe({})

    def test_ark_uses_readonly_direct_path_and_sanitizes_management(self):
        with tempfile.TemporaryDirectory() as directory:
            db = Path(directory)/'synthetic.db'
            with closing(sqlite3.connect(db)) as c:
                c.executescript('CREATE TABLE usage(id INTEGER); CREATE TABLE auth(uuid TEXT);')
            boundary = Path(directory)/'ready-boundary.json'; boundary.write_text(json.dumps(BOUNDARY))
            container = {'id': BOUNDARY['container_id'], 'image': 'sha256:'+'b'*64, 'restarts': 0,
                         'state': {'StartedAt': BOUNDARY['started_at'], 'Running': True, 'Paused': False}}
            membership = {'nodes': [{'node_id': 'node-1', 'ip': '100.110.240.90', 'healthy': True,
                                      'node_name': 'DO_NOT_SAVE'}]}
            inventory = {'version': '8.0.69', 'sha256': 'c'*64, 'size': 123,
                         'platform': 'darwin/arm64', 'catalog': True}
            fleet = {'policy': {'patch_updates': False}, 'active': None,
                     'nodes': {'node-1': dict(inventory, error='DO_NOT_SAVE')}}
            real_connect = sqlite3.connect
            def connect(database, **kw):
                self.assertEqual(database, 'file:'+str(db)+'?mode=ro')
                self.assertTrue(kw['uri'])
                return real_connect(database, **kw)
            with patch.object(proof, 'DATA', str(db)), patch.object(proof.subprocess, 'check_output',
                    return_value=json.dumps(container)), patch.object(proof, 'request_json',
                    side_effect=[(membership, {}, 0), (fleet, {}, 0)]), patch.dict(proof.os.environ,
                    {'MANAGEMENT_PASSWORD': 'fake-management-secret'}), patch.object(proof.sqlite3, 'connect', side_effect=connect):
                result = proof.ark_read({'boundary': str(boundary)})
            self.assertNotIn('DO_NOT_SAVE', json.dumps(result))
            self.assertNotIn('fake-management-secret', json.dumps(result))
            self.assertEqual(result['nodes'], [dict(inventory, node_id='node-1', ip='100.110.240.90')])
            for field, value in [('sha256', None), ('sha256', 'g'*64), ('sha256', 'c'*63), ('sha256', 123),
                                 ('size', None), ('size', 0), ('size', -1), ('size', True), ('size', 1.5), ('size', '123'),
                                 ('platform', None), ('platform', 'darwin/amd64'), ('platform', {}),
                                 ('catalog', None), ('catalog', 1), ('catalog', 'true')]:
                invalid = inventory.copy()
                if value is None:
                    invalid.pop(field)
                else:
                    invalid[field] = value
                fleet['nodes']['node-1'] = invalid
                with self.subTest(field=field, value=value), patch.object(proof, 'DATA', str(db)), \
                        patch.object(proof.subprocess, 'check_output', return_value=json.dumps(container)), patch.object(proof, 'request_json',
                        side_effect=[(membership, {}, 0), (fleet, {}, 0)]), patch.dict(proof.os.environ,
                        {'MANAGEMENT_PASSWORD': 'fake-management-secret'}):
                    with self.assertRaisesRegex(proof.ProofError, '^invalid_node_artifact$'):
                        proof.ark_read({'boundary': str(boundary)})
            container['state']['StartedAt'] = 'changed'
            with patch.object(proof.subprocess, 'check_output', return_value=json.dumps(container)):
                with self.assertRaisesRegex(proof.ProofError, 'container_changed'):
                    proof.ark_read({'boundary': str(boundary)})

    def test_hash_only_inventory_change_rejects_readiness_and_stability(self):
        with tempfile.TemporaryDirectory() as directory:
            db = Path(directory)/'synthetic.db'; sqlite3.connect(db).close()
            boundary = Path(directory)/'ready-boundary.json'; boundary.write_text(json.dumps(BOUNDARY))
            container = {'id': BOUNDARY['container_id'], 'image': 'sha256:'+'b'*64, 'restarts': 0,
                         'state': {'StartedAt': BOUNDARY['started_at'], 'Running': True, 'Paused': False}}
            membership = {'nodes': [{'node_id': 'node-1', 'ip': '100.110.240.90', 'healthy': True}]}
            fleet = {'policy': {'patch_updates': False}, 'active': None,
                     'nodes': {'node-1': {'version': '8.0.69', 'sha256': 'c'*64, 'size': 123,
                                         'platform': 'darwin/arm64', 'catalog': True}}}
            with patch.object(proof, 'DATA', str(db)), patch.object(proof.subprocess, 'check_output',
                    return_value=json.dumps(container)), patch.dict(proof.os.environ,
                    {'MANAGEMENT_PASSWORD': 'fake-management-secret'}):
                with patch.object(proof, 'request_json', side_effect=[(membership, {}, 0), (fleet, {}, 0)]):
                    before = proof.ark_read({'boundary': str(boundary)})
                fleet['nodes']['node-1']['sha256'] = 'd'*64
                with patch.object(proof, 'request_json', side_effect=[(membership, {}, 0), (fleet, {}, 0)]):
                    changed = proof.ark_read({'boundary': str(boundary)})
        for replies, error in [([changed, changed], 'readiness_changed'), ([before, changed], 'stability_changed')]:
            with self.subTest(error=error), patch.object(proof, 'remote', side_effect=replies), \
                    patch.object(proof.time, 'sleep'), self.assertRaisesRegex(proof.ProofError, '^'+error+'$'):
                proof.finish('/synthetic/ready-boundary.json', before, [])

    def test_bootstrap_runs_stdin_source_without_importing_local_files(self):
        source = Path(proof.__file__).read_text()
        data = {'source': source, 'function': 'require', 'payload': {}}
        # A deliberately wrong call fails closed without printing source/exception detail.
        result = subprocess.run([sys.executable, '-I', '-B', '-c', proof.BOOTSTRAP],
                                input=json.dumps(data), text=True, capture_output=True)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(json.loads(result.stdout), {'error': 'remote_failed'})
        self.assertEqual(result.stderr, '')

    def test_all_four_minis_both_providers_and_optional_mac_skip(self):
        nodes = [{'node_id': 'synthetic-'+user, 'ip': ip, 'version': '8.0.69'} for user, ip in proof.MINIS]
        def remote(host, function, payload):
            if host == proof.ARK:
                return {'nodes': nodes}
            return [{'provider': 'claude'}, {'provider': 'codex'}]
        with patch.object(proof, 'remote', side_effect=remote), patch.object(proof, 'finish', return_value={'ok': True}) as finish:
            result = proof.run('/synthetic/ready-boundary.json')
            self.assertEqual(result['macbook'], 'not_connected_optional_skip')
            self.assertEqual(len(finish.call_args.args[2]), 8)
            nodes.append({'node_id': 'synthetic-mac', 'ip': proof.MACBOOK_IP, 'version': '8.0.69'})
            with patch.object(proof, 'node_probe', return_value=[{'provider': 'claude'}, {'provider': 'codex'}]):
                self.assertEqual(proof.run('/synthetic/ready-boundary.json')['macbook'], 'tested')
                self.assertEqual(len(finish.call_args.args[2]), 10)
            nodes.pop()
        def wrong_remote(host, function, payload):
            if host == proof.ARK:
                return {'nodes': nodes}
            return [{'provider': 'claude'}, {'provider': 'claude'}]
        with patch.object(proof, 'remote', side_effect=wrong_remote):
            with self.assertRaisesRegex(proof.ProofError, 'native_coverage_missing'):
                proof.run('/synthetic/ready-boundary.json')

    def test_stability_reuses_boundary_and_exact_request_ids(self):
        first = {'boundary': BOUNDARY, 'container_id': BOUNDARY['container_id'],
                 'started_at': BOUNDARY['started_at'], 'candidate_image': 'sha256:'+'b'*64,
                 'nodes': [{'node_id': 'node-1', 'ip': '100.110.240.90', 'version': '8.0.69'}], 'usage': self.rows()}
        calls, sleeps = [], []
        def remote(host, function, payload):
            calls.append((host, function, payload.copy()))
            return first
        with patch.object(proof, 'remote', side_effect=remote), patch.object(proof.time, 'sleep', side_effect=sleeps.append):
            proof.finish('/synthetic/ready-boundary.json', first, [self.probe()])
        self.assertEqual(sleeps, [60])
        self.assertEqual(len(calls), 2)
        self.assertEqual(calls[0][2], calls[1][2])
        self.assertEqual(calls[0][2]['request_ids'], [REQUESTS[0]])
        changed = dict(first, container_id='different')
        with patch.object(proof, 'remote', side_effect=[first, changed]), patch.object(proof.time, 'sleep'), \
                self.assertRaisesRegex(proof.ProofError, 'stability_changed'):
            proof.finish('/synthetic/ready-boundary.json', first, [self.probe()])

    def test_own_key_skips_unset_env_reference_but_keeps_literal(self):
        with tempfile.TemporaryDirectory() as home:
            (Path(home)/'.openclaw').mkdir()
            providers = {'gui': {'baseUrl': 'http://127.0.0.1:18317', 'apiKey': 'fake-literal'},
                         'lite': {'baseUrl': 'http://127.0.0.1:18317/v1', 'apiKey': '${WS54_UNSET_TEST}'},
                         'other': {'baseUrl': 'http://127.0.0.1:9999', 'apiKey': 'ignored'}}
            (Path(home)/'.openclaw/openclaw.json').write_text(json.dumps({'models': {'providers': providers}}))
            with patch.object(proof.Path, 'home', return_value=Path(home)), patch.dict(proof.os.environ, {}, clear=False):
                proof.os.environ.pop('WS54_UNSET_TEST', None)
                self.assertEqual(proof.own_key(), 'fake-literal')
                providers['gui']['apiKey'] = '${WS54_UNSET_TEST}'
                (Path(home)/'.openclaw/openclaw.json').write_text(json.dumps({'models': {'providers': providers}}))
                with self.assertRaises(proof.ProofError):
                    proof.own_key()
            (Path(home)/'.openclaw/openclaw.json').write_text(json.dumps({'gateway': {}}))
            (Path(home)/'.config/cpa-home').mkdir(parents=True)
            cache = Path(home)/'.config/cpa-home/client-key'
            cache.write_text('k'*64+'\n'); cache.chmod(0o600)
            with patch.object(proof.Path, 'home', return_value=Path(home)):
                self.assertEqual(proof.own_key(), 'k'*64)
                cache.chmod(0o644)
                with self.assertRaises(proof.ProofError):
                    proof.own_key()


if __name__ == '__main__':
    unittest.main()
