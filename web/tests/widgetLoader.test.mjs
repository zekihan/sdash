import assert from "node:assert/strict";
import test from "node:test";
import { createWidgetLoader } from "../src/lib/widgetLoader.ts";

test("a late widget response cannot overwrite a newer snapshot", async () => {
  const poll = Promise.withResolvers();
  const refresh = Promise.withResolvers();
  const responses = [poll.promise, refresh.promise];
  const applied = [];
  const load = createWidgetLoader(() => responses.shift(), (snapshot) => applied.push(snapshot));

  const polling = load();
  const refreshing = load();
  refresh.resolve("newer snapshot");
  await refreshing;
  poll.resolve("older snapshot");
  await polling;

  assert.deepEqual(applied, ["newer snapshot"]);
});

test("a widget can update while a newer request is pending or fails", async () => {
  const poll = Promise.withResolvers();
  const refresh = Promise.withResolvers();
  const responses = [poll.promise, refresh.promise, Promise.resolve("recovered snapshot")];
  const applied = [];
  const load = createWidgetLoader(() => responses.shift(), (snapshot) => applied.push(snapshot));

  const polling = load();
  const refreshing = load();
  poll.resolve("initial snapshot");
  await polling;
  assert.deepEqual(applied, ["initial snapshot"]);

  refresh.reject(new Error("widget unavailable"));
  await refreshing;
  assert.deepEqual(applied, ["initial snapshot"]);

  await load();
  assert.deepEqual(applied, ["initial snapshot", "recovered snapshot"]);
});
