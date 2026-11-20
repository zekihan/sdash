import assert from "node:assert/strict";
import test from "node:test";
import { defaultPrefs, sanitizePrefs } from "../src/lib/prefs.ts";

test("dashboard defaults use a supported accent", () => {
  assert.equal(defaultPrefs({ accent: "purple" }).accent, "lime");
  const configured = defaultPrefs({ accent: "cyan" });
  assert.equal(configured.accent, "cyan");
  assert.equal(sanitizePrefs({ accent: "purple" }, configured).accent, "cyan");
});
