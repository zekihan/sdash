import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { runInNewContext } from "node:vm";

const script = readFileSync(new URL("../public/sw.js", import.meta.url), "utf8");
const origin = "https://dashboard.example";

function serviceWorker() {
  const listeners = new Map();
  const entries = new Map();
  let response;

  runInNewContext(script, {
    URL,
    self: {
      location: { origin },
      addEventListener: (type, listener) => listeners.set(type, listener),
    },
    caches: {
      open: async () => ({ put: async (key, entry) => entries.set(key, entry) }),
      match: async (key) => entries.get(key)?.clone(),
    },
    fetch: async () => {
      if (!response) throw new TypeError("Offline");
      return response;
    },
  });

  return async (path, networkResponse) => {
    response = networkResponse;
    const pending = [];
    let result;
    listeners.get("fetch")({
      request: { url: new URL(path, origin).href, method: "GET", mode: "navigate" },
      respondWith: (promise) => {
        result = promise;
      },
      waitUntil: (promise) => pending.push(promise),
    });
    const navigation = await result;
    await Promise.all(pending);
    return navigation;
  };
}

for (const { name, path, status, type, body } of [
  { name: "an HTTP error", path: "/", status: 503, type: "text/html", body: "Service unavailable" },
  { name: "an asset visit", path: "/favicon.svg", status: 200, type: "image/svg+xml", body: "<svg/>" },
]) {
  test(`offline navigation keeps the last successful page after ${name}`, async () => {
    const navigate = serviceWorker();
    const html = "<!doctype html><title>Dashboard</title>";
    const shell = new Response(html, { headers: { "Content-Type": "text/html; charset=utf-8" } });
    assert.equal(await navigate("/", shell), shell);

    const response = new Response(body, { status, headers: { "Content-Type": type } });
    assert.equal(await navigate(path, response), response);
    assert.equal(await (await navigate("/")).text(), html);
  });
}
