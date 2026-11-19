import copy
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import catalog_sync


class ClaudeCatalogTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.home = Path(self.directory.name)
        self.path = self.home / '.claude/settings.json'
        self.path.parent.mkdir()
        self.config = {'env': {'ANTHROPIC_BASE_URL': 'http://127.0.0.1:18317'},
                       'model': 'operator-choice', 'permissions': {'allow': ['Read']},
                       'modelPicker': {'options': [{'model': 'gpt-custom', 'label': 'Keep me'}]}}
        self.path.write_text(json.dumps(self.config))
        self.metadata = {'claude-fable-5-2': {'contextWindow': 1_000_000}}
        self.ids = ['claude-fable-5-2', 'claude-sonnet-5-5']

    def test_latest_families_exclude_encoded_aliases_and_choose_stable_ids(self):
        ids = ['claude-fable-5', 'claude-fable-5-1', 'claude-fable-5-2',
               'claude-fable-5-dd-los-1.6-tpg', 'claude-opus-5-5-high',
               'claude-sonnet-5-5-20261001', 'claude-sonnet-5-5', 'copilot/claude-fable-9']
        rows = catalog_sync.claude_picker_options(ids, self.metadata)
        self.assertEqual([r['model'] for r in rows],
                         ['claude-fable-5-2[1m]', 'claude-sonnet-5-5'])
        self.assertEqual(rows[0]['label'], 'Fable 5.2')

    def test_claude_only_node_uses_key_scoped_catalog_without_openclaw_calls(self):
        self.assertTrue(catalog_sync.has_catalog_target(self.home))
        with patch.object(catalog_sync, 'fetch_ids', return_value=self.ids) as fetch, \
                patch.object(catalog_sync, 'cli', side_effect=AssertionError('OpenClaw not installed')):
            result = catalog_sync.reconcile(self.home, True, self.metadata, client_key='fake-key')
        self.assertEqual(result['status'], 'updated')
        self.assertEqual(list(result['targets']), ['claude'])
        self.assertEqual(fetch.call_args.args[0]['models']['providers']['cpa-gui']['apiKey'], 'fake-key')
        after = json.loads(self.path.read_text())
        for field in ['model', 'permissions', 'env']:
            self.assertEqual(after[field], self.config[field])
        self.assertEqual(after['modelPicker']['options'][-1], self.config['modelPicker']['options'][0])
        self.assertEqual(self.path.stat().st_mode & 0o777, 0o600)

    def test_future_model_promotes_managed_row_but_preserves_user_modified_rows(self):
        catalog_sync.reconcile_claude(self.home, self.ids, True, self.metadata)
        after = json.loads(self.path.read_text())
        custom = after['modelPicker']['options'][0]
        custom['label'] = 'My pinned Fable'
        self.path.write_text(json.dumps(after))
        result = catalog_sync.reconcile_claude(self.home, ['claude-fable-5-3'], True)
        current = json.loads(self.path.read_text())
        self.assertEqual(result['models'], ['claude-fable-5-3'])
        self.assertIn(custom, current['modelPicker']['options'])
        self.assertEqual(current['model'], 'operator-choice')
        self.assertFalse(any(r['model'] == 'claude-sonnet-5-5' for r in current['modelPicker']['options']))

    def test_openclaw_failure_does_not_block_claude_but_overall_task_still_fails(self):
        with patch.object(catalog_sync, 'has_openclaw_target', return_value=True), \
                patch.object(catalog_sync, 'fetch_ids', return_value=self.ids), \
                patch.object(catalog_sync, 'reconcile_openclaw',
                             side_effect=catalog_sync.SyncError('openclaw_command_timeout')):
            with self.assertRaisesRegex(catalog_sync.SyncError, 'openclaw_command_timeout'):
                catalog_sync.reconcile(self.home, True, self.metadata, client_key='fake-key')
        after = json.loads(self.path.read_text())
        self.assertEqual(after['modelPicker']['options'][0]['model'], 'claude-fable-5-2[1m]')
        self.assertEqual(after['model'], self.config['model'])

    def test_user_owned_current_model_is_not_relabelled_or_claimed(self):
        self.config['modelPicker']['options'].append({'model': 'claude-fable-5-2[1m]', 'label': 'Pinned'})
        self.path.write_text(json.dumps(self.config))
        result = catalog_sync.reconcile_claude(self.home, self.ids, True, self.metadata)
        self.assertNotIn('claude-fable-5-2[1m]', result['models'])
        after = json.loads(self.path.read_text())
        self.assertIn({'model': 'claude-fable-5-2[1m]', 'label': 'Pinned'}, after['modelPicker']['options'])

    def test_wrong_endpoint_is_not_read_or_modified_by_claude_sync(self):
        self.config['env']['ANTHROPIC_BASE_URL'] = 'https://untrusted.invalid'
        self.path.write_text(json.dumps(self.config))
        original = self.path.read_bytes()
        with patch.object(catalog_sync, 'fetch_ids') as fetch:
            with self.assertRaisesRegex(catalog_sync.SyncError, 'no_catalog_target'):
                catalog_sync.reconcile(self.home, True, self.metadata, client_key='fake-key')
        fetch.assert_not_called()
        self.assertEqual(self.path.read_bytes(), original)

    def test_symlinked_settings_are_not_modified(self):
        target = self.home / 'elsewhere.json'
        self.path.rename(target)
        self.path.symlink_to(target)
        self.assertFalse(catalog_sync.has_claude_target(self.home))

    def test_endpoint_changed_after_target_discovery_is_not_modified(self):
        self.config['env']['ANTHROPIC_BASE_URL'] = 'https://operator-selected.invalid'
        self.path.write_text(json.dumps(self.config))
        original = self.path.read_bytes()
        with patch.object(catalog_sync, 'has_claude_target', return_value=True):
            with self.assertRaisesRegex(catalog_sync.SyncError, 'unexpected_claude_endpoint'):
                catalog_sync.reconcile_claude(self.home, self.ids, True, self.metadata)
        self.assertEqual(self.path.read_bytes(), original)

    def test_concurrent_config_edit_is_preserved(self):
        original_atomic = catalog_sync.atomic_write
        updated = copy.deepcopy(self.config)
        updated['model'] = 'concurrently-selected'
        def atomic(path, content, expected):
            if path == self.path:
                self.path.write_text(json.dumps(updated))
            return original_atomic(path, content, expected)
        with patch.object(catalog_sync, 'atomic_write', side_effect=atomic):
            with self.assertRaisesRegex(catalog_sync.SyncError, 'concurrent_config_change'):
                catalog_sync.reconcile_claude(self.home, self.ids, True, self.metadata)
        self.assertEqual(json.loads(self.path.read_text()), updated)

    def test_ownership_write_failure_rolls_back_only_picker_preserving_newer_settings(self):
        original_atomic = catalog_sync.atomic_write
        def atomic(path, content, expected):
            if path.name == 'claude-picker.json':
                current = json.loads(self.path.read_text())
                current['model'] = 'concurrently-selected'
                self.path.write_text(json.dumps(current))
                raise OSError('fixture disk error')
            return original_atomic(path, content, expected)
        with patch.object(catalog_sync, 'atomic_write', side_effect=atomic):
            with self.assertRaises(OSError):
                catalog_sync.reconcile_claude(self.home, self.ids, True, self.metadata)
        after = json.loads(self.path.read_text())
        self.assertEqual(after['model'], 'concurrently-selected')
        self.assertEqual(after['modelPicker'], self.config['modelPicker'])
        self.assertEqual(after['permissions'], self.config['permissions'])

    def test_idempotent_sync_and_two_version_backup_retention(self):
        first = catalog_sync.reconcile_claude(self.home, self.ids, True, self.metadata)
        original = self.path.read_bytes()
        second = catalog_sync.reconcile_claude(self.home, self.ids, True, self.metadata)
        self.assertEqual(second['status'], 'current')
        self.assertEqual(self.path.read_bytes(), original)
        self.assertIsNone(second['backup'])
        for version in ['3', '4']:
            catalog_sync.reconcile_claude(self.home, ['claude-fable-5-' + version], True)
        backups = list(Path(first['backup']).parent.glob('claude-picker-*'))
        self.assertEqual(len(backups), 2)

    def test_unknown_ownership_schema_and_empty_native_catalog_fail_closed(self):
        with self.assertRaisesRegex(catalog_sync.SyncError, 'no_native_claude_models'):
            catalog_sync.reconcile_claude(self.home, ['gpt-custom'], True)
        state = self.home / '.openclaw/state/cpa-home-fleet-worker/claude-picker.json'
        state.parent.mkdir(parents=True)
        state.write_text('{"schema":99,"options":[]}')
        with self.assertRaisesRegex(catalog_sync.SyncError, 'invalid_claude_picker_state'):
            catalog_sync.reconcile_claude(self.home, self.ids, True)
        self.assertEqual(json.loads(self.path.read_text()), self.config)


class OpenClawRetentionTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.home = Path(self.directory.name)
        self.path = self.home / '.openclaw/openclaw.json'
        self.path.parent.mkdir()
        self.config = {
            'models': {'providers': {'cpa-gui': {
                'baseUrl': 'http://127.0.0.1:18317/v1', 'apiKey': 'fake-key',
                'models': [{'id': 'old', 'name': 'old'}]}}},
            'agents': {'defaults': {'modelPolicy': {'allow': ['cpa-gui/*']},
                                    'models': {'cpa-gui/old': {}}, 'model': 'operator-choice'}}}
        self.path.write_text(json.dumps(self.config))

    def test_success_retains_current_rollback_backup_and_current_sync_is_idempotent(self):
        with patch.object(catalog_sync, 'fetch_ids', return_value=['old', 'new']), \
                patch.object(catalog_sync, 'cli', return_value={'valid': True}), \
                patch.object(catalog_sync, 'verify_views'):
            result = catalog_sync.reconcile_openclaw(self.home, True)
            backup = Path(result['backup'])
            self.assertEqual(json.loads((backup / 'before.json').read_text()), self.config)
            self.assertEqual((backup / 'candidate.json').read_bytes(), self.path.read_bytes())
            self.assertEqual(result['status'], 'updated')
            current = catalog_sync.reconcile_openclaw(self.home, True)
            self.assertEqual(current['status'], 'current')
            self.assertIsNone(current['backup'])
            self.assertTrue(backup.is_dir())

    def test_failed_attempts_retain_two_candidates_and_current_backup(self):
        for failure in ('schema', 'timeout', 'runtime'):
            with self.subTest(failure=failure):
                seen = []
                def validate(args, candidate):
                    backup = candidate.parent
                    seen.append(backup)
                    self.assertEqual(json.loads((backup / 'before.json').read_text()), self.config)
                    self.assertEqual(json.loads(candidate.read_text())['models']['providers']
                                     ['cpa-gui']['models'][-1]['id'], 'new')
                    if failure == 'timeout':
                        raise catalog_sync.SyncError('openclaw_command_timeout')
                    return {'valid': failure != 'schema'}

                def runtime(ids):
                    current = json.loads(self.path.read_text())
                    current['agents']['defaults']['model'] = 'concurrent-choice'
                    self.path.write_text(json.dumps(current))
                    raise catalog_sync.SyncError('gateway_unhealthy')

                expected_error = {'schema': 'schema_validation_failed',
                                  'timeout': 'openclaw_command_timeout',
                                  'runtime': 'runtime_failed_restored_model_fields'}[failure]
                with patch.object(catalog_sync, 'fetch_ids', return_value=['old', 'new']), \
                        patch.object(catalog_sync, 'cli', side_effect=validate), \
                        patch.object(catalog_sync, 'verify_views', side_effect=runtime), \
                        patch.object(catalog_sync.time, 'sleep'):
                    for attempt in range(5):
                        self.path.write_text(json.dumps(self.config))
                        with self.assertRaisesRegex(catalog_sync.SyncError, expected_error):
                            catalog_sync.reconcile_openclaw(self.home, True)
                        current = seen[-1]
                        remaining = set(current.parent.glob('cpa-catalog-*'))
                        self.assertEqual(remaining, set(seen[-2:]))
                        self.assertTrue((current / 'candidate.json').is_file())
                        self.assertEqual(json.loads((current / 'before.json').read_text()), self.config)
                        restored = copy.deepcopy(self.config)
                        if failure == 'runtime':
                            restored['agents']['defaults']['model'] = 'concurrent-choice'
                        self.assertEqual(json.loads(self.path.read_text()), restored)
                        # Older backups can be future-dated after a clock correction.
                        os.utime(current, ns=(2_000_000_000_000_000_000 + attempt,
                                             2_000_000_000_000_000_000 + attempt))
                for backup in seen[-2:]:
                    for child in backup.iterdir():
                        child.unlink()
                    backup.rmdir()


if __name__ == '__main__':
    unittest.main()
