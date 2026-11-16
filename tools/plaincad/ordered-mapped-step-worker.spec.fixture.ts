import { createHash } from "node:crypto";
import { expect, test } from "@playwright/test";
import type { OrderedMappedStepReply, OrderedMappedStepRequest } from "../src/cad/kernel/plainSolidOrderedMappedStepProbe";

const sha256 = (bytes: Buffer | string) => createHash("sha256").update(bytes).digest("hex");

test("PlainSolid module workers preserve the ordered v13 STEP receipt and distance-certified closed display", async ({ page }, info) => {
  test.setTimeout(240_000);
  const workers: string[] = [];
  const wasmLoads: number[] = [];
  const occtRequests: string[] = [];
  page.on("worker", worker => workers.push(new URL(worker.url()).pathname));
  page.on("response", response => {
    if (new URL(response.url()).pathname === "/assets/plainsolid-kernel.wasm") wasmLoads.push(response.status());
  });
  page.on("request", request => {
    if (/opencascade.*\.(?:wasm|js)/i.test(request.url())) occtRequests.push(request.url());
  });
  await page.goto("/plainsolid-ordered-mapped-step-worker.html");
  const result = await page.evaluate(async () => {
    function ask(request: OrderedMappedStepRequest): Promise<OrderedMappedStepReply> {
      const worker = new Worker("/src/cad/kernel/plainSolidOrderedMappedStepProbe.ts", { type: "module" });
      return new Promise((resolve, reject) => {
        worker.onmessage = event => { worker.terminate(); resolve(event.data); };
        worker.onerror = event => { worker.terminate(); reject(new Error(event.message)); };
        worker.onmessageerror = () => { worker.terminate(); reject(new Error("Unreadable module worker message")); };
        worker.postMessage(request);
      });
    }
    const response = await fetch("/plainsolid-ordered-mapped-step-input.step");
    if (!response.ok) throw new Error(`Authored STEP fixture fetch failed (${response.status})`);
    const decoder = new TextDecoder("utf-8", { fatal: true });
    const bytes = await response.arrayBuffer();
    const sha = await crypto.subtle.digest("SHA-256", bytes);
    const original = await ask({ operation: "repair", source: decoder.decode(bytes) });
    if (!original.ok) throw new Error(original.error);
    if (original.operation !== "repair") throw new Error("Unexpected repair reply");
    const strict = await ask({ operation: "reopen", source: decoder.decode(original.exported.bytes) });
    if (!strict.ok) throw new Error(strict.error);
    if (strict.operation !== "reopen") throw new Error("Unexpected strict reopen reply");
    const captured = ({ bytes, ...details }: typeof original.exported) => ({ ...details, canonical: Array.from(new Uint8Array(bytes)) });
    return { sourceSha256: Array.from(new Uint8Array(sha), byte => byte.toString(16).padStart(2, "0")).join(""),
      original: { ...original, exported: captured(original.exported) }, strict: { ...strict, exported: captured(strict.exported) } };
  });
  expect(workers).toEqual(Array(2).fill("/src/cad/kernel/plainSolidOrderedMappedStepProbe.ts"));
  expect(wasmLoads).toContain(200);
  expect(occtRequests).toEqual([]);

  const { original, strict, sourceSha256 } = result;
  expect(original.genericFailure).toMatch(/^PlainSolid UNSUPPORTED:/);
  expect(original.legacyFailure).toMatch(/^PlainSolid UNSUPPORTED:/);
  expect(strict.genericFailure).toMatch(/^PlainSolid UNSUPPORTED:/);
  expect(original.distanceFailure).toMatch(/^PlainSolid UNSUPPORTED: .*at least 0.36 mm/);
  expect(original.angularFailure).toMatch(/^PlainSolid UNSUPPORTED: .*certifies linear distance and requires angular deflection pi/);
  const { repairEvidence, nativeProofJSON } = original.evidence;
  expect(repairEvidence).toMatchObject({ family: "mapped-chain-v13", sourceSHA256: sourceSha256, lengthUnit: .001, pcurveOwners: 52 });
  expect(repairEvidence.mappedChain?.boundaryVersion).toBe(13);
  expect(repairEvidence.mappedChain?.originalEdges).toHaveLength(4);
  expect(repairEvidence.mappedChain?.originalFaces).toHaveLength(7);
  expect(repairEvidence.mappedChain?.supportEvents.map(event => [event.modelStation, event.ownersChangingCells, event.originalJoint])).toEqual([
    ["1/12", [false, true], false], ["1/4", [true, true], true],
    ["3/10", [true, false], false], ["1/2", [false, true], true],
    ["19/36", [false, true], false], ["3/4", [true, false], true],
    ["13/17", [true, false], false],
  ]);

  expect(original.display).toMatchObject({
    triangles: 2822, edges: 4233, eulerCharacteristic: 2, source: "plainsolid", operation: "stepImport",
    geometryAssertions: { valid: true, solidCount: 1 },
  });
  const proof = original.exported.before;
  expect(proof).toHaveLength(1);
  const { valid, solidCount, volume, surfaceArea } = proof[0];
  expect(original.display.geometryAssertions).toEqual({ valid, solidCount, volume, surfaceArea });
  expect(volume).toBeCloseTo(0.6448671346810797, 8);
  expect(surfaceArea).toBeCloseTo(4.6513347531185012, 8);
  expect(original.exported.units).toBe("mm");
  expect(strict.exported.units).toBe("mm");
  expect(original.exported.after).toEqual(proof);
  expect(strict.exported.before).toEqual(proof);
  expect(strict.exported.after).toEqual(proof);
  expect(strict.evidence.nativeProofJSON).toBe(nativeProofJSON);
  const certificateHash = sha256(nativeProofJSON!);
  expect(certificateHash).toBe("c705f9dbb44f21d4e14ae38dea64fa6c00fe207e4c96dddb95ad28b9b663eaa0");
  const canonical = Buffer.from(original.exported.canonical);
  expect(Buffer.from(strict.exported.canonical)).toEqual(canonical);
  const canonicalSha256 = sha256(canonical);
  expect(canonicalSha256).toBe("db0329b3ff7c4b9b04770aff59ce97f8a19d26acf91a7facb60dca3deef356e8");
  expect(strict.evidence.repairEvidence.sourceSHA256).toBe(canonicalSha256);

  await info.attach("ordered-mapped-step-worker-proof", { contentType: "application/json", body: JSON.stringify({
    sourceSha256, certificateSha256: certificateHash, canonicalSha256, workers, wasmLoads, occtRequests,
    display: original.display, linearDeflection: .36, angularInput: Math.PI,
    genericFailure: original.genericFailure, legacyFailure: original.legacyFailure, strictFailure: strict.genericFailure,
    distanceFailure: original.distanceFailure, angularFailure: original.angularFailure, proof,
  }, null, 2) });
});
