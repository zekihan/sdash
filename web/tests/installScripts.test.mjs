import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { copyFileSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { delimiter, dirname, join } from "node:path";
import test from "node:test";

function fixture(t, paths) {
  const root = mkdtempSync(join(tmpdir(), "sdash-install-scripts-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));

  for (const path of paths) {
    const destination = join(root, path);
    mkdirSync(dirname(destination), { recursive: true });
    copyFileSync(new URL(`../../${path}`, import.meta.url), destination);
  }
  return root;
}

function writePolicy(root, packages, allowScripts) {
  writeFileSync(join(root, "web/package.json"), JSON.stringify({ allowScripts }));
  writeFileSync(join(root, "web/package-lock.json"), JSON.stringify({ lockfileVersion: 3, packages }));
}

test("the installer and make web disable npm install scripts", (t) => {
  const installer = ".github/scripts/install_build_deps.sh";
  const root = fixture(t, ["Makefile", installer]);
  const bin = join(root, "bin");
  mkdirSync(bin);
  writeFileSync(join(bin, "npm"), "#!/bin/sh\nprintf '%s\\n' npm \"$@\"\n", { mode: 0o755 });
  const options = {
    cwd: root,
    encoding: "utf8",
    env: { ...process.env, PATH: bin + delimiter + process.env.PATH },
  };
  const install = ["npm", "ci", "--prefix", "web", "--ignore-scripts"];
  assert.deepEqual(execFileSync("sh", [installer], options).trimEnd().split("\n"), install);
  assert.deepEqual(execFileSync("make", ["-s", "web"], options).trimEnd().split("\n"), [
    ...install,
    "npm", "--prefix", "web", "run", "build",
  ]);
});

const checker = "web/scripts/check-install-scripts.mjs";
const script = { hasInstallScript: true, version: "1.0.0" };

test("the policy accepts exact approvals and optional packages excluded from Linux", (t) => {
  const root = fixture(t, [checker]);
  writePolicy(root, {
    "node_modules/ordinary": script,
    "node_modules/@outer/pkg/node_modules/@inner/tool": script,
    "node_modules/alias": { ...script, name: "@real/tool" },
    "node_modules/mac-only": { ...script, optional: true, os: ["darwin"] },
    "node_modules/no-linux": { ...script, optional: true, os: ["!linux"] },
  }, { "ordinary@1.0.0": true, "@inner/tool@1.0.0": true, "@real/tool@1.0.0": true });

  const result = spawnSync(process.execPath, [checker], { cwd: root, encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
});

test("make lint rejects unapproved Linux install scripts and required packages", (t) => {
  const root = fixture(t, ["Makefile", checker]);
  writePolicy(root, {
    "node_modules/parent/node_modules/@scope/version-changed": { ...script, version: "1.0.1" },
    "node_modules/linux": { ...script, optional: true, os: ["linux", "darwin"] },
    "node_modules/not-mac": { ...script, optional: true, os: ["!darwin"] },
    "node_modules/any": { ...script, optional: true, os: ["any"] },
    "node_modules/required": { ...script, os: ["darwin"] },
  }, { "@scope/version-changed@1.0.0": true, "linux@1.0.0": false, "not-mac": true });

  const result = spawnSync("make", ["-o", "tidy", "lint"], { cwd: root, encoding: "utf8" });
  assert.equal(result.status, 2, result.stderr);
  for (const approval of ["@scope/version-changed@1.0.1", "linux@1.0.0", "not-mac@1.0.0", "any@1.0.0", "required@1.0.0"]) {
    assert.ok(result.stderr.includes(`Unapproved install script: ${approval} (`), result.stderr);
  }
});
