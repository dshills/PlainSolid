#!/usr/bin/env node
// Diagnostic control for Playwright 1.63.0 with large captured WASM assets.
import { lstat, readFile, realpath, rename, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { join, relative, sep } from "node:path";

const [, , option, work, ...extra] = process.argv;
if (option !== "--work" || !work || extra.length) {
  throw new Error("Usage: node tools/plaincad/chromium-response-buffer.mjs --work /isolated/plaincad/stage");
}
const stage = await realpath(work);
const dependencies = join(stage, "node_modules");
if (!(await lstat(dependencies)).isDirectory()) {
  throw new Error("The isolated stage must contain a local dependency directory.");
}
const requested = join(dependencies, "playwright-core", "lib", "coreBundle.js");
const bundle = await realpath(requested);
if (relative(dependencies, bundle).startsWith(`..${sep}`) || bundle !== requested) {
  throw new Error("The Chromium network manager must be a regular staged dependency.");
}
const manifest = JSON.parse(await readFile(join(dependencies, "playwright-core", "package.json"), "utf8"));
if (manifest.name !== "playwright-core" || manifest.version !== "1.63.0") {
  throw new Error("This diagnostic control is pinned to Playwright 1.63.0.");
}

const original = await readFile(bundle, "utf8");
const predecessor = '          session2.send("Network.enable"),\n          this._updateProtocolRequestInterceptionForSession(';
const replacement = '          session2.send("Network.enable", { maxTotalBufferSize: 128 * 1024 * 1024, maxResourceBufferSize: 32 * 1024 * 1024 }),\n          this._updateProtocolRequestInterceptionForSession(';
if (original.split(predecessor).length !== 2) {
  throw new Error("Expected a single unmodified Chromium Network.enable call.");
}
const adapted = original.replace(predecessor, replacement);
const hash = text => createHash("sha256").update(text).digest("hex");
const next = `${bundle}.plainsolid-buffer-${process.pid}`;
// Rename a fresh staged file so even a hard-linked package store stays intact.
await writeFile(next, adapted, { flag: "wx", mode: (await lstat(bundle)).mode & 0o777 });
await rename(next, bundle);
console.log(JSON.stringify({
  playwright: manifest.version,
  chromiumNetworkBufferBytes: 128 * 1024 * 1024,
  chromiumResourceBufferBytes: 32 * 1024 * 1024,
  beforeSha256: hash(original),
  afterSha256: hash(adapted),
}, null, 2));
