"""Offline regression fixtures: no real dependency installs or publication calls."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]


class VerifiedBuildTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / '.github/scripts').mkdir(parents=True)
        (self.root / 'build/tailwind').mkdir(parents=True)
        for name in ('verify_tailwind.sh', 'install_build_deps.sh'):
            shutil.copy(ROOT / '.github/scripts' / name,
                        self.root / '.github/scripts' / name)
        self.asset = 'tailwindcss-linux-x64'
        self.payload = self.root / 'download'
        self.payload.write_bytes(b'fixture: never execute me\n')
        self.digest = hashlib.sha256(self.payload.read_bytes()).hexdigest()
        self.manifest = self.root / 'build/tailwind/checksums.txt'
        self.manifest.write_text(f'{self.digest}  {self.asset}\n')

    def verify(self, asset=None):
        return subprocess.run(['sh', str(self.root / '.github/scripts/verify_tailwind.sh'),
                               asset or self.asset, str(self.payload)],
                              capture_output=True, text=True)

    def test_matching_digest_succeeds(self):
        self.assertEqual(self.verify().returncode, 0)

    def test_tampered_payload_fails(self):
        self.payload.write_bytes(b'compromised delivery')
        result = self.verify()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('checksum mismatch', result.stderr)

    def test_unknown_platform_and_duplicate_digest_fail_closed(self):
        self.assertNotEqual(self.verify('tailwindcss-unknown-x64').returncode, 0)
        self.manifest.write_text(self.manifest.read_text() * 2)
        self.assertNotEqual(self.verify().returncode, 0)

    def test_installer_stops_before_chmod_install_or_execution_on_mismatch(self):
        mocks = self.root / 'mock-bin'
        mocks.mkdir()
        marker = self.root / 'unexpected-execution'
        commands = {
            'uname': 'case "$1" in -m) echo x86_64;; -s) echo Linux;; esac',
            'curl': 'while [ "$1" != "--output" ]; do shift; done; shift; '
                    'printf "compromised binary" > "$1"',
            'npm': 'touch "$MARKER"; exit 99',
            'go': 'touch "$MARKER"; exit 99',
            'chmod': 'touch "$MARKER"; exit 99',
            'mv': 'touch "$MARKER"; exit 99',
            'tailwindcss': 'touch "$MARKER"; exit 99',
        }
        for name, body in commands.items():
            path = mocks / name
            path.write_text('#!/bin/sh\n' + body + '\n')
            path.chmod(0o755)
        # A preexisting executable must not bypass verification either.
        (self.root / 'build/bin').mkdir()
        installed = self.root / 'build/bin/tailwindcss'
        installed.write_text('previous installation')
        result = subprocess.run(
            ['sh', str(self.root / '.github/scripts/install_build_deps.sh')],
            env={**os.environ, 'PATH': str(mocks) + os.pathsep + os.environ['PATH'],
                 'MARKER': str(marker)}, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('checksum mismatch', result.stderr)
        self.assertFalse(marker.exists())
        self.assertEqual(installed.read_text(), 'previous installation')
        self.assertFalse(list((self.root / 'build/bin').glob('.tailwindcss.*')))

    def test_all_format_entrypoints_use_locked_prettier(self):
        for name in ('Makefile', '.goreleaser.yaml'):
            text = (ROOT / name).read_text()
            self.assertNotIn('npx', text)
            self.assertIn('./build/node/node_modules/.bin/prettier', text)
        installer = (ROOT / '.github/scripts/install_build_deps.sh').read_text()
        self.assertIn('npm ci --ignore-scripts --prefix build/node', installer)
        self.assertNotIn('sudo', installer)

    def test_npm_tools_are_exactly_pinned_with_integrity(self):
        package = json.loads((ROOT / 'build/node/package.json').read_text())
        lock = json.loads((ROOT / 'build/node/package-lock.json').read_text())
        self.assertEqual(package['devDependencies'], lock['packages']['']['devDependencies'])
        for name, version in package['devDependencies'].items():
            entry = lock['packages']['node_modules/' + name]
            self.assertEqual(version, entry['version'])
            self.assertTrue(entry['integrity'].startswith('sha512-'))
            self.assertTrue(entry['resolved'].startswith('https://registry.npmjs.org/'))
        self.assertEqual(package['devDependencies']['prettier'], '3.10.1')

    def test_publication_uses_one_pinned_shared_contract(self):
        workflow = (ROOT / '.github/workflows/release.yaml').read_text()
        calls = re.findall(
            r'uses: zekihan/release-workflows/\.github/workflows/(\w+)\.yaml@([0-9a-f]{40})',
            workflow)
        self.assertEqual([name for name, _ in calls], ['decide', 'image', 'release'])
        self.assertEqual(len({pin for _, pin in calls}), 1)
        self.assertNotIn('secrets: inherit', workflow)
        checks = workflow.split('\n  checks:\n', 1)[1].split('\n  publish:\n', 1)[0]
        self.assertIn('uses: ./.github/workflows/pr.yaml', checks)
        self.assertNotIn('secrets', checks)
        self.assertNotIn('contents: write', checks)
        self.assertNotIn('packages: write', checks)
        publish = workflow.split('\n  publish:\n', 1)[1].split('\n  release:\n', 1)[0]
        self.assertIn('      - checks', publish)
        release = workflow.split('\n  release:\n', 1)[1]
        self.assertIn('      - publish', release)
        self.assertIn('image-digest: ${{ needs.publish.outputs.digest }}', release)
        self.assertFalse((ROOT / '.github/workflows/docker_release.yaml').exists())

    def test_native_checks_have_no_publication_credentials(self):
        workflow = (ROOT / '.github/workflows/pr.yaml').read_text()
        for forbidden in ('secrets.', 'contents: write', 'packages: write'):
            self.assertNotIn(forbidden, workflow)
        self.assertIn('persist-credentials: false', workflow)
        self.assertIn('--single-target', workflow)
        dockerfile = (ROOT / 'Dockerfile').read_text()
        self.assertNotIn('npm', dockerfile)
        self.assertNotIn('npx', dockerfile)


if __name__ == '__main__':
    unittest.main()
