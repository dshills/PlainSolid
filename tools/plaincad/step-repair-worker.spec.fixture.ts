import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import type { StepRepairProbeReply, StepRepairProbeRequest } from "../src/cad/kernel/plainSolidStepRepairProbe";

test("PlainSolid module workers preserve the STEP repair certificate and strict canonical reopen", async ({ page }) => {
  const workers: string[] = [];
  const wasmLoads: number[] = [];
  const occtRequests: string[] = [];
  page.on("worker", worker => workers.push(worker.url()));
  page.on("response", response => {
    if (new URL(response.url()).pathname === "/assets/plainsolid-kernel.wasm") wasmLoads.push(response.status());
  });
  page.on("request", request => {
    if (/opencascade.*\.(?:wasm|js)/i.test(request.url())) occtRequests.push(request.url());
  });
  await page.goto("/plainsolid-step-repair-worker.html");
  const result = await page.evaluate(async () => {
    function ask(request: StepRepairProbeRequest): Promise<StepRepairProbeReply> {
      const worker = new Worker("/src/cad/kernel/plainSolidStepRepairProbe.ts", { type: "module" });
      return new Promise((resolve, reject) => {
        worker.onmessage = event => { worker.terminate(); resolve(event.data); };
        worker.onerror = event => { worker.terminate(); reject(new Error(event.message)); };
        worker.onmessageerror = () => { worker.terminate(); reject(new Error("Unreadable module worker message")); };
        worker.postMessage(request);
      });
    }
    const response = await fetch("/plainsolid-step-repair-input.step");
    if (!response.ok) throw new Error(`STEP fixture fetch failed (${response.status})`);
    const repair = await ask({ source: await response.text(), repair: true });
    if (!repair.ok) throw new Error(repair.error);
    if (!repair.repair || !repair.evidence?.nativeProofJSON) throw new Error("Missing the repaired worker reply");
    const strict = await ask({ source: new TextDecoder().decode(repair.canonical) });
    if (!strict.ok) throw new Error(strict.error);
    return {
      certificate: repair.evidence.nativeProofJSON,
      certificateFromWasm: Array.from(new Uint8Array(repair.repair.certificateFromWasm)),
      repairEvidence: repair.evidence.repairEvidence,
      strictFailure: repair.repair.strictFailure,
      triangles: repair.repair.triangles,
      positionsInWorldBounds: repair.repair.positionsInWorldBounds,
      before: repair.before,
      after: repair.after,
      strictBefore: strict.before,
      strictAfter: strict.after,
      strictEvidence: strict.evidence,
      canonical: Array.from(new Uint8Array(repair.canonical)),
      canonicalReopened: Array.from(new Uint8Array(strict.canonical)),
    };
  });

  expect(workers).toHaveLength(2);
  expect(wasmLoads.length).toBeGreaterThan(0);
  expect(wasmLoads.every(status => status === 200 || status === 304)).toBe(true);
  expect(occtRequests).toEqual([]);
  expect(result.strictFailure).toMatch(/PlainSolid UNSUPPORTED/);
  expect(result.triangles).toBeGreaterThan(0);
  expect(result.positionsInWorldBounds).toBe(true);

  const source = await readFile("public/plainsolid-step-repair-input.step");
  const digest = createHash("sha256").update(source).digest("hex");
  const pose = [["0", "3/5", "4/5", "8/5"], ["-1", "0", "0", "0"], ["0", "-4/5", "3/5", "6/5"]];
  expect(result.repairEvidence).toMatchObject({ family: "occt-rational-posed-single-pcurve-convex-arc",
    sourceSHA256: digest, maxDistance: 1e-12, pcurveOwners: 24, worldPose: pose });
  expect(Buffer.from(result.certificate, "utf8")).toEqual(Buffer.from(result.certificateFromWasm));
  expect(result.certificate.endsWith("\n")).toBe(true);
  const certificate = JSON.parse(result.certificate);
  expect(certificate).toMatchObject({ Format: "plainsolid-step-rational-posed-pcurve-convex-arc-source", Version: 1,
    Claims: { SourceSHA256: digest, Pose: pose } });
  expect(Buffer.from(certificate.Source, "base64")).toEqual(source);

  expect(result.before).toHaveLength(1);
  const [proof] = result.before;
  expect(proof).toMatchObject({ valid: true, solidCount: 1 });
  expect(proof.volume).toBeCloseTo(29.145041881053842, 10);
  expect(proof.surfaceArea).toBeCloseTo(66.186284791166074, 10);
  for (const [side, expected] of [["min", [1, 0, -.7]], ["max", [7.025, 3, 5]]] as const) {
    proof.bounds[side].forEach((value, axis) => expect(value).toBeCloseTo(expected[axis], 11));
  }
  expect(result.after).toEqual(result.before);
  expect(result.strictBefore).toEqual(result.before);
  expect(result.strictAfter).toEqual(result.before);
  expect(result.strictEvidence).toBeUndefined();
  expect(Buffer.from(result.canonicalReopened)).toEqual(Buffer.from(result.canonical));
});
