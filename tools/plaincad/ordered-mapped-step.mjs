import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const [wasm, runtime] = process.argv.slice(2);
if (!wasm || !runtime) throw new Error("usage: node ordered-mapped-step.mjs PLAINCAD_WASM WASM_EXEC_JS");

await import(pathToFileURL(resolve(runtime)).href);
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(await readFile(wasm), go.importObject);
void go.run(instance);
assert.equal(typeof globalThis.plainSolidCall, "function");

const fixture = name => readFile(new URL(
  "../../exchange/step/testdata/occt_mapped_chain_ordered_joined_" + name + ".step",
  import.meta.url,
), "utf8");
const sha256 = bytes => createHash("sha256").update(bytes).digest("hex");
const ask = request => JSON.parse(globalThis.plainSolidCall(JSON.stringify(request)));
const invoke = request => {
  const reply = ask(request);
  assert.equal(reply.ok, true, request.op + " failed " + JSON.stringify(reply.error));
  return reply.result;
};
const request = source => ({
  op: "importStep", source,
  stepRepair: { family: "mapped-chain-v13", maxDistance: 1e-12, nativeText: true },
});

function verifyClosedDisplay(display) {
  assert.equal(display.indices.length, 3 * 2822);
  assert.equal(display.positions.length, display.normals.length);
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
    const triangle = display.indices.slice(i, i + 3).map(index => welded[index]);
    assert.equal(new Set(triangle).size, 3);
    for (let side = 0; side < 3; side++) {
      const a = triangle[side];
      const b = triangle[(side + 1) % 3];
      const key = a < b ? a + "," + b : b + "," + a;
      const count = edges.get(key) ?? { uses: 0, direction: 0 };
      count.uses++;
      count.direction += a < b ? 1 : -1;
      edges.set(key, count);
    }
  }
  for (const count of edges.values()) {
    assert.deepEqual(count, { uses: 2, direction: 0 });
  }
}

const source = await fixture("independent_roofs");
const moved = await fixture("moved_joint");
assert.equal(ask({ op: "importStep", source }).error.code, "UNSUPPORTED");
assert.equal(ask({ op: "importStep", source, stepRepair: { maxDistance: 1e-12 } }).error.code, "UNSUPPORTED");
assert.equal(ask(request(moved)).error.code, "UNSUPPORTED");
const admitted = invoke(request(source));
console.log("Authored four-cell STEP imported with the explicit repair family");
assert.equal(admitted.repairEvidence.family, "mapped-chain-v13");
assert.equal(admitted.repairEvidence.sourceSHA256, sha256(source));
assert.equal(admitted.repairEvidence.pcurveOwners, 52);
assert.equal(admitted.repairEvidence.mappedChain.boundaryVersion, 13);
assert.equal(admitted.repairEvidence.mappedChain.originalEdges.length, 4);
assert.equal(admitted.repairEvidence.mappedChain.originalFaces.length, 7);
assert.deepEqual(admitted.repairEvidence.mappedChain.supportEvents.map(event => [
  event.modelStation, event.ownersChangingCells, event.originalJoint,
]), [
  ["1/12", [false, true], false], ["1/4", [true, true], true],
  ["3/10", [true, false], false], ["1/2", [false, true], true],
  ["19/36", [false, true], false], ["3/4", [true, false], true],
  ["13/17", [true, false], false],
]);
assert.equal(admitted.geometryAssertions.valid, true);
assert.equal(admitted.geometryAssertions.solidCount, 1);
assert(Math.abs(admitted.geometryAssertions.volume - 0.6448671346810797) < 1e-8);
assert(Math.abs(admitted.geometryAssertions.surfaceArea - 4.6513347531185012) < 1e-8);
assert.equal(sha256(admitted.nativeProofJSON), "c705f9dbb44f21d4e14ae38dea64fa6c00fe207e4c96dddb95ad28b9b663eaa0");

const display = invoke({ op: "mesh", handle: admitted.handle, linearDeflection: .36, angularDeflection: Math.PI });
verifyClosedDisplay(display);
assert.deepEqual(display.geometryAssertions, admitted.geometryAssertions);
console.log("WASM display returned 2,822 triangles with closed oriented edge incidence");

const output = invoke({ op: "exportStep", handles: [admitted.handle] });
assert.equal(output.units, "mm");
assert.deepEqual(output.before, output.after);
const canonical = Buffer.from(output.base64, "base64");
assert.equal(sha256(canonical), "db0329b3ff7c4b9b04770aff59ce97f8a19d26acf91a7facb60dca3deef356e8");
assert.equal(ask({ op: "importStep", source: canonical.toString("utf8") }).error.code, "UNSUPPORTED");
const strict = invoke(request(canonical.toString("utf8")));
assert.deepEqual(strict.geometryAssertions, admitted.geometryAssertions);
assert.equal(strict.nativeProofJSON, admitted.nativeProofJSON);
invoke({ op: "dispose", handle: strict.handle });
invoke({ op: "dispose", handle: admitted.handle });
console.log("Canonical STEP strictly reopened with the same exact native receipt");
