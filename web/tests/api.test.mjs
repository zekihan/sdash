import assert from "node:assert/strict";
import test from "node:test";
import { ApiError, api } from "../src/lib/api.ts";

test("preference sync preserves the newer save when an older request is delayed", async (t) => {
  const olderPrefs = { theme: "light" };
  const newerPrefs = { theme: "dark" };
  const olderStarted = Promise.withResolvers();
  const releaseOlder = Promise.withResolvers();
  let stored;

  t.mock.method(globalThis, "fetch", async (path, request) => {
    assert.equal(path, "/api/preferences");
    assert.equal(request.method, "PUT");
    const prefs = JSON.parse(request.body);
    if (prefs.theme === olderPrefs.theme) {
      olderStarted.resolve();
      await releaseOlder.promise;
    }
    stored = prefs;
    return new Response();
  });

  const older = api.putPreferences(olderPrefs);
  await olderStarted.promise;
  const newer = api.putPreferences(newerPrefs);
  releaseOlder.resolve();
  await Promise.all([older, newer]);

  assert.deepEqual(stored, newerPrefs);
});

test("a failed preference save does not block a queued save", async (t) => {
  const fetch = t.mock.method(globalThis, "fetch", async (_, request) =>
    new Response(null, { status: JSON.parse(request.body).theme === "light" ? 500 : 200 }),
  );

  const older = api.putPreferences({ theme: "light" });
  const newer = api.putPreferences({ theme: "dark" });

  await Promise.all([assert.rejects(older, (error) => error instanceof ApiError && error.status === 500), newer]);
  assert.equal(fetch.mock.callCount(), 2);
});
