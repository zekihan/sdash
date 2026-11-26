import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { runnerImport } from "vite";

const root = fileURLToPath(new URL("../", import.meta.url));
const { module: { PbsBlock } } = await runnerImport("/src/components/ClusterWidgets.tsx", { root });
const { module: { buildAttentionItems } } = await runnerImport("/src/components/Attention.tsx", { root });

const noBackups = {
  available: true,
  tasksUnavailable: false,
  datastores: [{ name: "backups", used: 400, total: 1000, usedPercent: 40 }],
  used: 400,
  total: 1000,
  lastBackupAt: 0,
  lastBackupOk: false,
  hasBackups: false,
  recentFailures: 0,
  recentTotal: 0,
};

function renderPbs(pbs) {
  return renderToStaticMarkup(createElement(PbsBlock, { pbs, expanded: false, onToggle() {} }));
}

function attentionFor(pbs) {
  return buildAttentionItems({
    pbs,
    services: [],
    kube: null,
    gatusData: null,
    proxmox: null,
    truenas: null,
    actionsEnabled: false,
    onArgoSync() {},
  });
}

test("the PBS widget distinguishes unavailable backup status from empty history", () => {
  const unavailable = renderPbs({ ...noBackups, tasksUnavailable: true });
  assert.match(unavailable, /backup status unavailable/i);
  assert.match(unavailable, /\(40%\)/);
  assert.doesNotMatch(unavailable, /no backups/i);

  const empty = renderPbs(noBackups);
  assert.match(empty, /no backups/i);
  assert.doesNotMatch(empty, /backup status unavailable/i);
});

test("PBS attention flags unreadable history and continues to flag failed backups", () => {
  const unavailable = attentionFor({ ...noBackups, tasksUnavailable: true });
  assert.deepEqual(unavailable.map(({ label }) => label), ["Backup status unavailable"]);
  assert.deepEqual(attentionFor(noBackups), []);
  assert.deepEqual(attentionFor({ ...noBackups, available: false, tasksUnavailable: true }), []);

  const failed = attentionFor({ ...noBackups, hasBackups: true, lastBackupAt: 400 });
  assert.deepEqual(failed.map(({ label }) => label), ["Last backup failed"]);
});
