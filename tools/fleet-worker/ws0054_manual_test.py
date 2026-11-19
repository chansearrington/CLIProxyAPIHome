"""Source-only manual checks: synthetic shell clock, never Home/Docker/SSH."""
import ast
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
MANUAL = ROOT/'docs/fleet/ws-0054-cutover-steps.txt'
ANNEX = ROOT/'docs/fleet/ws-0054-upgrade-1110-8076.md'


class ManualTest(unittest.TestCase):
    def test_every_manual_shell_and_python_block_parses(self):
        shells = pythons = 0
        for path in (MANUAL, ANNEX):
            for shell in re.findall(r'```sh\n(.*?)\n```', path.read_text(), re.S):
                result = subprocess.run(['bash', '-n'], input=shell, text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                shells += 1
                for python in re.findall(r"<<'PY'\n(.*?)\nPY(?:\n|$)", shell, re.S):
                    ast.parse(python, filename=str(path))
                    pythons += 1
        self.assertGreater(shells, 0)
        self.assertGreater(pythons, 0)
        print('MANUAL_SYNTAX shell_blocks=%d Python_blocks=%d PASS' % (shells, pythons))

    def test_hot_time_contract_pins_nov20_and_rejects_stale_nov19(self):
        text = ANNEX.read_text().split('### HOT —', 1)[1]
        python = text.split("<<'PY'\n", 1)[1].split('\nPY\n', 1)[0]
        tree = ast.parse(python)
        # Run the exact time-contract prefix; physical/artifact checks have the HOT suite.
        contract = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == 'contract')
        end = next(i for i, node in enumerate(contract.body) if isinstance(node, ast.Assert)
                   and isinstance(node.msg, ast.Constant) and node.msg.value == 'stale_or_late_invocation')
        contract.body = contract.body[:end+1]
        definitions = [node for node in tree.body if isinstance(node, (ast.Import, ast.Assign))
                       or isinstance(node, ast.FunctionDef) and node.name in ('stamp', 'contract')]
        namespace = {}
        exec(compile(ast.Module(body=definitions, type_ignores=[]), str(ANNEX), 'exec'), namespace)
        self.assertEqual(namespace['WINDOW_DAY'], '2026-11-20')
        base = Path('/synthetic/window')
        def check(day, completed='05:01:00+00:00'):
            start = day+'T05:00:00+00:00'
            finish = day+'T'+completed
            namespace['contract']({'started_utc': start, 'completed_utc': finish},
                {'base': str(base), 'helper_sha256': namespace['PIN'], 'started_utc': start},
                {'exit': 0, 'completed_utc': finish}, {}, base, namespace['DAY']+namespace['datetime'].timedelta(hours=8))
        check('2026-11-20')
        for day, completed in [('2026-11-19', '05:01:00+00:00'),
                               ('2026-11-20', '07:00:00+00:00')]:
            with self.subTest(day=day, completed=completed), self.assertRaisesRegex(AssertionError, 'stale_or_late'):
                check(day, completed)

    def test_readiness_deadline_routes_late_success_to_recovery(self):
        section = MANUAL.read_text().split('6 — ONE ACCEPTANCE CONTRACT', 1)[1]
        shell = re.search(r'```sh\n(.*?)\n```', section, re.S).group(1)
        # Exercise the actual caller/latch, omitting only private environment loading.
        caller = shell[shell.index('if test "$NEXT" = ACCEPT'):]
        caller = '\n'.join(line for line in caller.splitlines()
                           if line not in ('set -a; . /mnt/user/appdata/cpa-home/.env; set +a',
                                           'export MANAGEMENT_PASSWORD'))
        mocks = r'''
set +e
date() { read -r now < "$CLOCK"; printf '%s\n' "$now"; }
verify_home() {
  test "$1" = ready || return 99
  read -r now < "$CLOCK"
  printf '%s\n' "$((now+DURATION))" > "$CLOCK"
}
timeout() {
  test "$1" = --signal=KILL && test "$2" = 10 || return 98
  if test "$TIMED_OUT" = 1; then printf '110\n' > "$CLOCK"; return 137; fi
  shift 2
  "$@"
}
docker() { printf '%s\n' '0 false'; }
sleep() { return 97; }
export -f date verify_home timeout docker sleep
NEXT=ACCEPT
export ACCEPT_SECONDS=10 CLOCK DURATION TIMED_OUT
'''
        with tempfile.TemporaryDirectory() as directory:
            clock = Path(directory)/'clock'
            for duration, timed_out, expected in [(9, 0, 'NATIVE'), (10, 0, 'RECOVERY'),
                                                   (11, 0, 'RECOVERY'), (0, 1, 'RECOVERY')]:
                clock.write_text('100\n')
                script = ('CLOCK='+str(clock)+'\nDURATION='+str(duration)+'\nTIMED_OUT='+str(timed_out)
                          +'\n'+mocks+'\n'+caller+'\nprintf "LATCH=%s\\n" "$NEXT"\n')
                with self.subTest(duration=duration, timed_out=timed_out):
                    result = subprocess.run(['bash', '-eu'], input=script, text=True, capture_output=True)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertIn('LATCH='+expected, result.stdout)
        panel = ANNEX.read_text()
        self.assertIn('for name in sorted(served):', panel)
        self.assertIn("time.time()<int(os.environ['READINESS_DEADLINE'])", panel)
        print('READINESS_CLOCK controls=4 PASS on_time=NATIVE late/exact/timeout=RECOVERY; full_panel_retained')


if __name__ == '__main__':
    unittest.main()
