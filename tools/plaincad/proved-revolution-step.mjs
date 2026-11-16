import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const [wasm, runtime] = process.argv.slice(2);
if (!wasm || !runtime) throw new Error("usage: node proved-revolution-step.mjs PLAINCAD_WASM WASM_EXEC_JS");
await import(pathToFileURL(resolve(runtime)).href);
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(await readFile(wasm), go.importObject);
void go.run(instance);

const fixture = mode => readFile(new URL(
  `../../exchange/step/testdata/occt-eccentric-annular-chamfer-${mode}.step`, import.meta.url,
), "utf8");
const ask = req => JSON.parse(globalThis.plainSolidCall(JSON.stringify(req)));
const invoke = req => {
  const reply = ask(req);
  assert(reply.ok, `${req.op}: ${JSON.stringify(reply.error)}`);
  return reply.result;
};

const source = await fixture("exact");
assert.equal(ask({ op: "importStep", source: await fixture("rounded") }).error.code, "UNSUPPORTED");
const solid = invoke({ op: "importStep", source });
assert.equal(solid.geometryAssertions.valid, true);
assert.equal(solid.geometryAssertions.solidCount, 1);
const volume = solid.geometryAssertions.volume;
assert(Math.abs(volume - 726.98826535488854) < 1e-5);
const display = invoke({ op: "mesh", handle: solid.handle, linearDeflection: .5, angularDeflection: .2 });
assert.deepEqual(display.geometryAssertions, solid.geometryAssertions);
assert(display.positions.every(Number.isFinite));
assert(display.normals.every(Number.isFinite));
const vertices = new Map();
const welded = [];
for (let i = 0; i < display.positions.length; i += 3) {
  const key = display.positions.slice(i, i + 3).join(",");
  if (!vertices.has(key)) vertices.set(key, vertices.size);
  welded.push(vertices.get(key));
}
const edges = new Map();
for (let i = 0; i < display.indices.length; i += 3) {
  const t = display.indices.slice(i, i + 3).map(v => welded[v]);
  assert.equal(new Set(t).size, 3);
  for (let s = 0; s < 3; s++) {
    const a = t[s], b = t[(s + 1) % 3];
    const key = a < b ? `${a},${b}` : `${b},${a}`;
    const uses = edges.get(key) ?? [0, 0];
    uses[0]++;
    uses[1] += a < b ? 1 : -1;
    edges.set(key, uses);
  }
}
for (const uses of edges.values()) assert.deepEqual(uses, [2, 0]);
const triangles = display.indices.length / 3;
assert.equal(vertices.size - edges.size + triangles, 0);

const neutral = invoke({ op: "exportStep", handles: [solid.handle] });
assert.equal(neutral.units, "mm");
assert.deepEqual(neutral.before, neutral.after);
const raw = Buffer.from(neutral.base64, "base64");
assert.equal(raw.toString("utf8"), source);
const replay = invoke({ op: "importStep", source: raw.toString("utf8") });
assert.deepEqual(replay.geometryAssertions, solid.geometryAssertions);
assert.deepEqual(replay.bounds, solid.bounds);
invoke({ op: "dispose", handle: replay.handle });
invoke({ op: "dispose", handle: solid.handle });
console.log(JSON.stringify({ triangles, vertices: vertices.size, edges: edges.size, eulerCharacteristic: 0,
  volume, originalSHA256: createHash("sha256").update(raw).digest("hex") }));
