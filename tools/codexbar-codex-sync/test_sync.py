import base64
import importlib.util
import json
from pathlib import Path
import time
import tempfile
from unittest.mock import patch
import unittest

spec = importlib.util.spec_from_file_location('codex_sync', Path(__file__).with_name('sync.py'))
sync = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sync)


def jwt(payload):
    return 'fixture.' + base64.urlsafe_b64encode(json.dumps(payload).encode()).decode().rstrip('=') + '.fake'


class IdentityTests(unittest.TestCase):
    def setUp(self):
        self.mapping = {'home_id': 'fixture-id', 'email': 'test@example.com'}
        self.account = dict(self.mapping, account_id='fixture-account',
                            access_token=jwt({'exp': time.time()+3600, 'https://api.openai.com/auth':
                                              {'chatgpt_account_id': 'fixture-account'}}),
                            id_token=jwt({'email': 'test@example.com'}))

    def test_valid_identity(self):
        self.assertIn('+00:00', sync.validate(self.account, self.mapping))

    def test_refresh_token_rejected(self):
        self.account['refresh_token'] = 'fake-forbidden-token'
        with self.assertRaises(sync.shared.SyncError):
            sync.validate(self.account, self.mapping)

    def test_wrong_owner_rejected(self):
        self.account['id_token'] = jwt({'email': 'other@example.com'})
        with self.assertRaises(sync.shared.SyncError):
            sync.validate(self.account, self.mapping)

    def test_wrong_workspace_rejected(self):
        self.account['account_id'] = 'wrong-account'
        with self.assertRaises(sync.shared.SyncError):
            sync.validate(self.account, self.mapping)

    def test_expired_rejected(self):
        self.account['access_token'] = jwt({'exp': time.time()-1, 'https://api.openai.com/auth':
                                           {'chatgpt_account_id': 'fixture-account'}})
        with self.assertRaises(sync.shared.SyncError):
            sync.validate(self.account, self.mapping)


class PublicationTests(unittest.TestCase):
    setUp = IdentityTests.setUp
    def test_access_only_publication_and_preserved_settings(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = root / 'config.json'
            original = {'providers': [{'id': 'codex', 'cookieSource': 'off'},
                                      {'id': 'claude', 'untouched': 'fixture'}], 'version': 1}
            config.write_text(json.dumps(original))
            mapping = dict(self.mapping, profile_home=str(root / 'profile'))
            settings = {'env_file': 'fake', 'management_url': 'fake',
                        'codex_accounts': [mapping], 'codexbar_config': str(config),
                        'codex_system_home_id': 'other',
                        'follow_usage': {'client_key_id': 6, 'cpa_node_id': 'fixture-node'}}
            selection = {'status': 'observed', 'success': True, 'provider': 'codex',
                         'credential_id': mapping['home_id'], 'client_key_id': 6,
                         'cpa_node_id': 'fixture-node'}
            with patch.object(sync.shared, 'remote_read', side_effect=[
                {'accounts': [self.account]}, {'selection': selection}]), patch.object(sync, 'verify_usage'):
                sync.run(settings, root)
            auth = json.loads((root / 'profile/auth.json').read_text())
            self.assertEqual(auth['tokens']['refresh_token'], 'cpa-home-owned-no-local-refresh')
            updated = json.loads(config.read_text())
            self.assertEqual(updated['providers'][1], original['providers'][1])
            self.assertEqual(updated['providers'][0]['cookieSource'], 'off')
            self.assertEqual(updated['providers'][0]['codexActiveSource']['homePath'], mapping['profile_home'])
            self.assertEqual((root / 'profile/auth.json').stat().st_mode & 0o777, 0o600)
            self.assertNotIn(self.account['access_token'], (root / 'status.json').read_text())

    def test_provider_rejection_preserves_existing_auth(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            profile = root / 'profile'
            profile.mkdir()
            auth = profile / 'auth.json'
            auth.write_bytes(b'original-private-fixture')
            mapping = dict(self.mapping, profile_home=str(profile))
            settings = {'env_file': 'fake', 'management_url': 'fake', 'codex_accounts': [mapping]}
            with patch.object(sync.shared, 'remote_read', return_value={'accounts': [self.account]}), \
                    patch.object(sync, 'verify_usage', side_effect=sync.shared.SyncError('fixture rejection')):
                with self.assertRaises(sync.shared.SyncError):
                    sync.run(settings, root)
            self.assertEqual(auth.read_bytes(), b'original-private-fixture')


if __name__ == '__main__':
    unittest.main()
