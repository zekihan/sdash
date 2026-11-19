import assert from "node:assert/strict";
import test from "node:test";
import { humanizeHealthError } from "../src/lib/format.ts";

test("health probes that receive a response identify keyword check failures", () => {
  assert.deepEqual(
    [humanizeHealthError("keyword not found", 200), humanizeHealthError("failed to read body", 200)],
    ["Expected text missing", "Could not read response"],
  );
});
