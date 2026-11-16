import { createHash } from "node:crypto";
import { expect, test } from "@playwright/test";
import type { ConicalBoreWorkerReply, ConicalBoreWorkerRequest } from "../src/cad/kernel/plainSolidConicalBoreProbe";

test("PlainSolid module workers preserve the ordinary conical bore STEP and signed angular display", async ({ page }, info) => {
  test.setTimeout(180_000);
  const workers: string[] = [];
  const wasmLoads: number[] = [];
  const occtRequests: string[] = [];
  page.on("worker", worker => workers.push(new URL(worker.url()).pathname));
  page.on("response", response => {
    if (new URL(response.url()).pathname === "/assets/plainsolid-kernel.wasm") wasmLoads.push(response.status());
  });
  page.on("request", request => {
    if (/(?:opencascade|occt)[^/?]*\.(?:js|wasm)(?:\?|$)/i.test(request.url())) occtRequests.push(request.url());
  });

  await page.goto("/plainsolid-conical-bore-worker.html");
  const result = await page.evaluate(async () => {
    const start = performance.now();
    const decoder = new TextDecoder("utf-8", { fatal: true });
    async function fetchSource(path: string) {
      const response = await fetch(path);
      if (!response.ok) throw new Error(`Conical STEP fixture fetch failed (${response.status})`);
      return response.arrayBuffer();
    }
    function ask(request: ConicalBoreWorkerRequest): Promise<ConicalBoreWorkerReply> {
      const worker = new Worker("/src/cad/kernel/plainSolidConicalBoreProbe.ts", { type: "module" });
      return new Promise((resolve, reject) => {
        worker.onmessage = event => { worker.terminate(); resolve(event.data); };
        worker.onerror = event => { worker.terminate(); reject(new Error(event.message)); };
        worker.onmessageerror = () => { worker.terminate(); reject(new Error("Unreadable conical worker message")); };
        worker.postMessage(request);
      });
    }
    const [source, rounded] = await Promise.all([
      fetchSource("/plainsolid-conical-bore-exact.step"), fetchSource("/plainsolid-conical-bore-rounded.step"),
    ]);
    const original = await ask({ operation: "importAndMesh", source: decoder.decode(source), rounded: decoder.decode(rounded) });
    if (!original.ok) throw new Error(original.error);
    if (original.operation !== "importAndMesh") throw new Error("Unexpected conical import reply");
    const reopened = await ask({ operation: "reopen", source: decoder.decode(original.exported.bytes) });
    if (!reopened.ok) throw new Error(reopened.error);
    if (reopened.operation !== "reopen") throw new Error("Unexpected conical reopen reply");
    const workflowMs = performance.now() - start;
    const capture = ({ bytes, ...proof }: typeof original.exported) => ({ ...proof, bytes: Array.from(new Uint8Array(bytes)) });
    return { original: { ...original, exported: capture(original.exported) },
      reopened: { ...reopened, exported: capture(reopened.exported) }, source: Array.from(new Uint8Array(source)), workflowMs };
  });
  expect(workers).toEqual(Array(2).fill("/src/cad/kernel/plainSolidConicalBoreProbe.ts"));
  expect(wasmLoads).toHaveLength(2);
  expect(wasmLoads).toContain(200);
  expect(wasmLoads.every(status => status === 200 || status === 304)).toBe(true);
  expect(occtRequests).toEqual([]);

  const { original, reopened, source, workflowMs } = result;
  expect(original.roundedRejection).toMatch(/^PlainSolid UNSUPPORTED:/);
  expect(original.display).toMatchObject({ triangles: 10496, vertices: 5248, edges: 15744, eulerCharacteristic: 0,
    source: "plainsolid", operation: "stepImport", geometryAssertions: { valid: true, solidCount: 1 } });
  expect(original.display.signedVolume).toBeCloseTo(2277.843268907193, 7);
  expect(original.meshSHA256).toBe("1958b4d1365d8c2fa1d09f838601e4dc4139aeb5f18fe83d77676414892d1edb");
  expect(reopened.bounds).toEqual(original.bounds);
  expect(original.exported.units).toBe("mm");
  expect(reopened.exported.units).toBe("mm");
  expect(original.exported.before).toHaveLength(1);
  const { bounds, ...material } = original.exported.before[0];
  expect(bounds).toEqual(original.bounds);
  expect(material).toEqual(original.display.geometryAssertions);
  expect(material.volume).toBeCloseTo(2282.401400976394, 7);
  expect(material.surfaceArea).toBeCloseTo(1182.6440976399522, 7);
  expect(original.exported.after).toEqual(original.exported.before);
  expect(reopened.exported.before).toEqual(original.exported.before);
  expect(reopened.exported.after).toEqual(original.exported.before);

  const originalBytes = Buffer.from(source);
  expect(Buffer.from(original.exported.bytes)).toEqual(originalBytes);
  expect(Buffer.from(reopened.exported.bytes)).toEqual(originalBytes);
  const sourceSHA256 = createHash("sha256").update(originalBytes).digest("hex");
  expect(sourceSHA256).toBe("af7b2221489a2edef6c4535233c8786826519787e9e326639e984bea0bd871a9");

  await info.attach("conical-bore-worker-proof", { contentType: "application/json", body: JSON.stringify({
    sourceSHA256, meshSHA256: original.meshSHA256, linearDeflection: .05, angularDeflection: .2,
    workers, wasmLoads, occtRequests, display: original.display, originalBytes: originalBytes.length,
    nativeProof: original.exported.before[0], roundedRejection: original.roundedRejection,
    meshMs: original.meshMs, importMs: original.importMs, exportMs: original.exportMs,
    reopenMs: reopened.importMs, reopenedExportMs: reopened.exportMs, workflowMs,
  }, null, 2) });
});
