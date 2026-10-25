"""Offline regression fixtures: no real dependency installs or publication calls."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import textwrap
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
        self.assertEqual(package['devDependencies']['prettier'], '3.9.10')

    def test_publication_credentials_are_isolated_from_build_jobs(self):
        for name in ('release.yaml', 'docker_release.yaml'):
            workflow = (ROOT / '.github/workflows' / name).read_text()
            build, publish = workflow.split('\n  publish:\n', 1)
            self.assertNotIn('secrets.', build)
            self.assertNotIn('contents: write', build)
            self.assertNotIn('packages: write', build)
            self.assertIn('persist-credentials: false', build)
            self.assertIn('needs: build', publish)
            self.assertNotIn('actions/checkout', publish)
            for command in ('npm ', 'goreleaser ', 'make ', 'install_build_deps', 'templ fmt'):
                self.assertNotIn(command, publish)
        dockerfile = (ROOT / 'Dockerfile').read_text()
        self.assertNotIn('npm', dockerfile)
        self.assertNotIn('npx', dockerfile)

    def publish_fixture(self, tag, tamper=False):
        dist = self.root / 'dist'
        dist.mkdir()
        archive = dist / 'sdash.tar.gz'
        archive.write_bytes(b'prepared archive fixture')
        checksum = hashlib.sha256(archive.read_bytes()).hexdigest()
        (dist / 'checksums.txt').write_text(f'{checksum}  sdash.tar.gz\n')
        (dist / 'CHANGELOG.md').write_text('Fixture release notes')
        if tamper:
            archive.write_bytes(b'modified after build')
        mocks = self.root / 'mock-bin'
        mocks.mkdir()
        gh = mocks / 'gh'
        gh.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > "$CALLS"\n')
        gh.chmod(0o755)
        calls = self.root / 'calls'
        workflow = (ROOT / '.github/workflows/release.yaml').read_text()
        script = textwrap.dedent(workflow.rsplit('        run: |\n', 1)[1])
        result = subprocess.run(['bash', '-e', '-o', 'pipefail', '-c', script],
            cwd=self.root, capture_output=True, text=True,
            env={**os.environ, 'PATH': str(mocks) + os.pathsep + os.environ['PATH'],
                 'CALLS': str(calls), 'RELEASE_TAG': tag,
                 'GH_TOKEN': 'synthetic-fixture-token', 'GH_REPO': 'fixture/repository'})
        return result, calls

    def test_release_publishes_only_prepared_verified_archives(self):
        result, calls = self.publish_fixture('v1.2.3')
        self.assertEqual(result.returncode, 0, result.stderr)
        arguments = calls.read_text().splitlines()
        self.assertEqual(arguments[:3], ['release', 'create', 'v1.2.3'])
        self.assertIn('--verify-tag', arguments)
        self.assertIn('./sdash.tar.gz', arguments)
        self.assertNotIn('./*.zip', arguments)
        self.assertNotIn('--prerelease', arguments)

    def test_prerelease_flag_is_preserved(self):
        result, calls = self.publish_fixture('v1.2.3-rc.1')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('--prerelease', calls.read_text().splitlines())

    def test_release_checksum_mismatch_prevents_publication(self):
        result, calls = self.publish_fixture('v1.2.3', tamper=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(calls.exists())


if __name__ == '__main__':
    unittest.main()
