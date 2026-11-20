import assert from "node:assert/strict";
import test from "node:test";
import { defaultPrefs, sanitizePrefs } from "../src/lib/prefs.ts";

test("dashboard defaults use a supported accent", () => {
  assert.equal(defaultPrefs({ accent: "purple" }).accent, "lime");
  const configured = defaultPrefs({ accent: "cyan" });
  assert.equal(configured.accent, "cyan");
  assert.equal(sanitizePrefs({ accent: "purple" }, configured).accent, "cyan");
});

test("partial synced preferences preserve local favorites and recents", () => {
  const local = { ...defaultPrefs(), pinned: ["photos"], recents: ["docs"] };

  assert.deepEqual(sanitizePrefs({ theme: "light" }, local), { ...local, theme: "light" });
  assert.deepEqual(sanitizePrefs({ pinned: "photos", recents: null }, local), local);
});

test("synced preference lists can replace or clear local lists", () => {
  const local = { ...defaultPrefs(), pinned: ["photos"], recents: ["docs"] };
  const synced = sanitizePrefs({ pinned: [], recents: ["files", 42] }, local);

  assert.deepEqual(synced, { ...local, pinned: [], recents: ["files"] });
});
