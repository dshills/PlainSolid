import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const [wasm, runtime, family = "annular"] = process.argv.slice(2);
if (!wasm || !runtime || !["annular", "conical", "annular-repair"].includes(family))
  throw new Error("usage: node proved-revolution-step.mjs PLAINCAD_WASM WASM_EXEC_JS [annular|conical|annular-repair]");
await import(pathToFileURL(resolve(runtime)).href);
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(await readFile(wasm), go.importObject);
void go.run(instance);

const conical = family === "conical";
const fixtureName = conical ? "occt-conical-double-rim-eccentric-bore" : "occt-eccentric-annular-chamfer";
const fixture = mode => readFile(new URL(`../../exchange/step/testdata/${fixtureName}-${mode}.step`, import.meta.url), "utf8");
const ask = req => JSON.parse(globalThis.plainSolidCall(JSON.stringify(req)));
const invoke = req => {
  const reply = ask(req);
  assert(reply.ok, `${req.op}: ${JSON.stringify(reply.error)}`);
  return reply.result;
};

const repaired = family === "annular-repair";
const rounded = await fixture("rounded");
const source = repaired ? rounded : await fixture("exact");
assert.equal(ask({ op: "importStep", source: rounded }).error.code, "UNSUPPORTED");
const repair = { family: "occt-eccentric-annular-chamfer", maxDistance: 2e-12, nativeText: true };
if (repaired) assert.equal(ask({ op: "importStep", source, stepRepair: { ...repair, maxDistance: 1.6e-12 } }).error.code, "UNSUPPORTED");
const solid = invoke({ op: "importStep", source, ...(repaired && { stepRepair: repair }) });
assert.equal(solid.geometryAssertions.valid, true);
assert.equal(solid.geometryAssertions.solidCount, 1);
const volume = solid.geometryAssertions.volume;
assert(Math.abs(volume - (conical ? 2282.4014040828897 : 726.98826535488854)) < 1e-5);
const start = performance.now();
const display = invoke({ op: "mesh", handle: solid.handle, linearDeflection: conical ? .05 : .5, angularDeflection: .2, analyticNormals: true });
const meshMs = performance.now() - start;
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
let signedVolume = 0;
for (let i = 0; i < display.indices.length; i += 3) {
  const t = display.indices.slice(i, i + 3).map(v => welded[v]);
  assert.equal(new Set(t).size, 3);
  const [a, b, c] = display.indices.slice(i, i + 3).map(v => display.positions.slice(3 * v, 3 * v + 3));
  signedVolume += (a[0] * (b[1] * c[2] - b[2] * c[1]) + a[1] * (b[2] * c[0] - b[0] * c[2]) + a[2] * (b[0] * c[1] - b[1] * c[0])) / 6;
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
assert(Math.abs(signedVolume / volume - 1) < .015);
if (conical) assert.equal(triangles, 10_496);

const neutral = invoke({ op: "exportStep", handles: [solid.handle] });
assert.equal(neutral.units, "mm");
assert.deepEqual(neutral.before, neutral.after);
const raw = Buffer.from(neutral.base64, "base64");
const sourceSHA256 = createHash("sha256").update(source).digest("hex");
const exportSHA256 = createHash("sha256").update(raw).digest("hex");
if (repaired) {
  assert.notEqual(exportSHA256, sourceSHA256);
  assert.equal(solid.repairEvidence.family, repair.family);
  assert.equal(solid.repairEvidence.sourceSHA256, sourceSHA256);
  assert.equal(solid.repairEvidence.annularChamfer.correctedSHA256, exportSHA256);
  assert.equal(solid.repairEvidence.annularChamfer.walls.length, 6);
  assert.equal(JSON.parse(solid.nativeProofJSON).Source, Buffer.from(source).toString("base64"));
} else {
  assert.equal(raw.toString("utf8"), source);
}
const replay = invoke({ op: "importStep", source: raw.toString("utf8") });
assert.deepEqual(replay.geometryAssertions, solid.geometryAssertions);
assert.deepEqual(replay.bounds, solid.bounds);
if (repaired) {
  assert.equal(replay.repairEvidence, undefined);
  assert.equal(invoke({ op: "exportStep", handles: [replay.handle] }).base64, neutral.base64);
}
invoke({ op: "dispose", handle: replay.handle });
invoke({ op: "dispose", handle: solid.handle });
console.log(JSON.stringify({ family, triangles, vertices: vertices.size, edges: edges.size, eulerCharacteristic: 0,
  volume, signedVolume, meshMs, originalSHA256: sourceSHA256, ...(repaired && { exportSHA256 }) }));
