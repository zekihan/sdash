import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const versions = new Set();
for (const [workflow, count] of [['pr.yaml', 1], ['release.yaml', 2]]) {
  const path = new URL(`../workflows/${workflow}`, import.meta.url);
  const inputs = readFileSync(path, 'utf8').split(/\r?\n/)
    .filter(line => /^\s*node-version\s*:/.test(line));
  assert.equal(inputs.length, count, `${workflow} must declare ${count} Node version input(s)`);

  for (const input of inputs) {
    const pin = /^\s*node-version:\s+(['"]?)(\d+\.\d+\.\d+)\1\s+# renovate: depName=node datasource=node-version versioning=node\s*$/.exec(input);
    assert.ok(pin, `${workflow} must use a Renovate-tracked exact Node version: ${input.trim()}`);
    versions.add(pin[2]);
  }
}

assert.equal(versions.size, 1, `PR checks and Publish must use the same Node version: ${[...versions].join(', ')}`);
