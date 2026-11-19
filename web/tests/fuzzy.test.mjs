import assert from "node:assert/strict";
import test from "node:test";
import { fuzzyAny } from "../src/lib/fuzzy.ts";

test("a long service description stays searchable by a term near the end", () => {
  const description =
    "Tracks scheduled backups, disk usage, and recent job failures across nodes and namespaces. " +
    "Use the inventory view to find hosts, review their resource allocation, and check the last successful report. " +
    "Reports include historical usage, retention settings, and the current storage owner. " +
    "The detailed view includes a ZFS pool overview.";
  const score = fuzzyAny("zfs", ["Operations", "ops", description]);

  assert.ok(score >= 0, `expected a searchable match, got ${score}`);
  assert.ok(fuzzyAny("zfs", ["ZFS overview"]) > score);
  assert.equal(fuzzyAny("zfs", ["Operations", "ops"]), -1);
});

test("a long field with a matching prefix stays searchable", () => {
  const description = "Router inventory. " + "Tracks the status of each managed network device. ".repeat(22);

  assert.ok(fuzzyAny("router", [description]) >= 0);
});
