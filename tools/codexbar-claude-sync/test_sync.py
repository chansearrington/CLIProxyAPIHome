import contextlib
import datetime as dt
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import sync


class SyncTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.path = self.root / 'config.json'
        self.settings = {
            'ssh_host': 'test-host', 'env_file': '/fake/.env',
            'management_url': 'http://localhost/v8/management',
            'codexbar_config': str(self.path), 'state_dir': str(self.root),
            'accounts': [{'label': 'Test', 'home_id': 'home-1',
                          'codexbar_id': 'bar-1', 'email': 'test@example.com'}]}
        self.config = {'providers': [
            {'id': 'codex', 'enabled': True, 'unchanged': 'keep'},
            {'id': 'claude', 'source': 'web', 'cookieSource': 'manual',
             'tokenAccounts': {'activeIndex': 1, 'accounts': [
                 {'id': 'bar-1', 'label': 'Test', 'addedAt': 123, 'token': 'old-cookie'},
                 {'id': 'unmanaged', 'label': 'Other', 'token': 'unmanaged-cookie'}]}}]}
        self.path.write_bytes(sync.encoded(self.config))
        self.now = dt.datetime(2026, 10, 8, tzinfo=dt.timezone.utc)
        self.payload = {'accounts': [{'home_id': 'home-1', 'email': 'test@example.com',
                                     'access_token': 'sk-ant-oat-fake-token',
                                     'expires_at': '2099-01-01T00:00:00Z'}]}

    def test_only_mapped_token_changes_and_rollback_preserves_other_changes(self):
        with patch.object(sync, 'fetch_accounts', return_value=self.payload), \
             patch.object(sync, 'verify_profile') as profile:
            report = sync.sync(self.settings, self.root)
            profile.assert_called_once_with('sk-ant-oat-fake-token', 'test@example.com')
        expected = json.loads(json.dumps(self.config))
        expected['providers'][1]['tokenAccounts']['accounts'][0]['token'] = 'sk-ant-oat-fake-token'
        self.assertEqual(json.loads(self.path.read_bytes()), expected)
        self.assertEqual(self.path.stat().st_mode & 0o777, 0o600)
        backup = self.root / 'original-tokens.json'
        self.assertEqual(backup.stat().st_mode & 0o777, 0o600)
        self.assertNotIn('sk-ant-oat', json.dumps(report))
        expected['providers'][0]['unchanged'] = 'new unrelated change'
        self.path.write_bytes(sync.encoded(expected))
        sync.restore(self.settings, self.root)
        self.assertEqual(json.loads(self.path.read_bytes())['providers'][0]['unchanged'], 'new unrelated change')
        self.assertEqual(json.loads(self.path.read_bytes())['providers'][1], self.config['providers'][1])

    def test_rotation_updates_token_without_replacing_original_backup(self):
        with patch.object(sync, 'fetch_accounts', return_value=self.payload), patch.object(sync, 'verify_profile'):
            sync.sync(self.settings, self.root)
            self.payload['accounts'][0]['access_token'] = 'sk-ant-oat-rotated-fake'
            sync.sync(self.settings, self.root)
        token = json.loads(self.path.read_bytes())['providers'][1]['tokenAccounts']['accounts'][0]['token']
        self.assertEqual(token, 'sk-ant-oat-rotated-fake')
        self.assertEqual(json.loads((self.root / 'original-tokens.json').read_bytes()), {'bar-1': 'old-cookie'})

    def test_no_change_avoids_profile_and_config_write(self):
        with patch.object(sync, 'fetch_accounts', return_value=self.payload), patch.object(sync, 'verify_profile'):
            sync.sync(self.settings, self.root)
        with patch.object(sync, 'fetch_accounts', return_value=self.payload), \
             patch.object(sync, 'verify_profile') as profile:
            before = self.path.stat().st_mtime_ns
            report = sync.sync(self.settings, self.root)
            self.assertEqual(before, self.path.stat().st_mtime_ns)
            self.assertEqual(report['changed_accounts'], [])
            profile.assert_not_called()

    def test_rejects_refresh_tokens_wrong_identity_duplicates_and_expiry(self):
        for change in [
            {'refresh_token': 'secret'}, {'email': 'wrong@example.com'},
            {'access_token': 'cookie'}, {'expires_at': '2026-10-08T00:00:30Z'},
            {'expires_at': '2099-01-01T00:00:00'}, {'expires_at': 'malformed'}]:
            with self.subTest(change=change):
                payload = json.loads(json.dumps(self.payload))
                payload['accounts'][0].update(change)
                with self.assertRaises(sync.SyncError):
                    sync.validate_accounts(self.settings, payload, self.now)
        self.payload['accounts'].append(self.payload['accounts'][0])
        with self.assertRaises(sync.SyncError):
            sync.validate_accounts(self.settings, self.payload, self.now)

    def test_profile_failure_leaves_config_and_backup_untouched(self):
        original = self.path.read_bytes()
        with patch.object(sync, 'fetch_accounts', return_value=self.payload), \
             patch.object(sync, 'verify_profile', side_effect=sync.SyncError('profile mismatch')):
            with self.assertRaises(sync.SyncError):
                sync.sync(self.settings, self.root)
        self.assertEqual(self.path.read_bytes(), original)
        self.assertFalse((self.root / 'original-tokens.json').exists())

    def test_concurrent_edit_is_not_overwritten(self):
        original = self.path.read_bytes()
        self.path.write_text('concurrent edit')
        with self.assertRaises(sync.SyncError):
            sync.atomic_write(self.path, b'replacement', expected=original)
        self.assertEqual(self.path.read_text(), 'concurrent edit')

    def test_remote_reader_only_exports_access_token_fields(self):
        class Response(io.StringIO):
            pass

        class Opener:
            def open(inner, request, timeout):
                return Response(json.dumps({'type': 'claude', 'email': 'test@example.com',
                                            'access_token': 'sk-ant-oat-fake-token',
                                            'expired': '2099-01-01T00:00:00Z',
                                            'refresh_token': 'never-export-this-refresh-secret'}))

        output = io.StringIO()
        with patch('pathlib.Path.read_text', return_value='MANAGEMENT_PASSWORD=never-export-management-secret'), \
             patch('urllib.request.build_opener', return_value=Opener()), contextlib.redirect_stdout(output):
            exec(sync.remote_program(self.settings), {})
        serialized = output.getvalue()
        self.assertNotIn('never-export', serialized)
        self.assertEqual(sync.validate_accounts(self.settings, json.loads(serialized), self.now)['home-1']['email'],
                         'test@example.com')

    def test_ssh_failure_never_exposes_stderr(self):
        result = subprocess.CompletedProcess([], 1, 'not json', 'secret raw server output')
        with patch('subprocess.run', return_value=result):
            with self.assertRaises(sync.SyncError) as error:
                sync.fetch_accounts(self.settings)
        self.assertNotIn('secret raw', str(error.exception))

    def test_auth_failure_latches_and_stops_further_requests(self):
        settings_path = self.root / 'settings.json'
        settings_path.write_bytes(sync.encoded(self.settings))
        args = ['sync.py', '--settings', str(settings_path)]
        with patch('sys.argv', args), contextlib.redirect_stdout(io.StringIO()), \
             patch.object(sync, 'fetch_accounts', side_effect=sync.SyncError('management_http_401')) as fetch:
            self.assertEqual(sync.main(), 1)
            self.assertEqual(sync.main(), 1)
            fetch.assert_called_once()
        self.assertTrue((self.root / 'auth-blocked').exists())
        self.assertEqual(json.loads(self.path.read_bytes()), self.config)


if __name__ == '__main__':
    unittest.main()
