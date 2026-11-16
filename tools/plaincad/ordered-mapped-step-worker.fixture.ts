import { OpenCascadeKernel, type NativeStepRepairEvidence, type StepImportOptions } from "./OpenCascadeKernel";
import type { RenderMesh } from "./KernelAdapter";
import type { NativeStepExport } from "./nativeStep";

export type OrderedMappedStepRequest = {
  operation: "repair" | "reopen";
  source: string;
};

export type OrderedMappedStepReply = {
  ok: true;
  evidence: NativeStepRepairEvidence;
  genericFailure: string;
} & (
  {
    operation: "repair";
    legacyFailure: string;
    distanceFailure: string;
    angularFailure: string;
    display: ReturnType<typeof proveClosedDisplay>;
    exported: NativeStepExport;
  } | { operation: "reopen"; exported: NativeStepExport }
) | { ok: false; error: string };

const worker = globalThis as unknown as {
  onmessage: (event: MessageEvent<OrderedMappedStepRequest>) => void;
  postMessage(reply: OrderedMappedStepReply, transfer?: ArrayBuffer[]): void;
};

const repair = {
  stepRepair: { family: "mapped-chain-v13", maxDistance: 1e-12, native: true },
} satisfies StepImportOptions;

function rejected(action: () => void): string {
  try { action(); }
  catch (error) { return error instanceof Error ? error.message : String(error); }
  throw new Error("The unsupported operation succeeded");
}

function proveClosedDisplay(mesh: RenderMesh) {
  const { positions, normals, indices } = mesh;
  if (positions.length !== normals.length || positions.length % 3 || indices.length % 3 ||
      !Array.from(positions).every(Number.isFinite) || !Array.from(normals).every(Number.isFinite)) {
    throw new Error("Invalid display buffer");
  }
  // The renderer duplicates a position at sharp normal changes. Weld exact
  // coordinates before testing the oriented two-owner edge incidence.
  const vertices = new Map<string, number>();
  const welded: number[] = [];
  for (let i = 0; i < positions.length; i += 3) {
    const key = [positions[i], positions[i + 1], positions[i + 2]].join(",");
    if (!vertices.has(key)) vertices.set(key, vertices.size);
    welded.push(vertices.get(key)!);
  }
  if (indices.some(index => !Number.isInteger(index) || index < 0 || index >= welded.length)) {
    throw new Error("Display index is outside the vertex buffer");
  }
  const edges = new Map<string, { uses: number; orientation: number }>();
  for (let i = 0; i < indices.length; i += 3) {
    const triangle = indices.slice(i, i + 3).map(index => welded[index]);
    if (new Set(triangle).size !== 3) throw new Error("Invalid display triangle");
    for (let side = 0; side < 3; side++) {
      const a = triangle[side], b = triangle[(side + 1) % 3];
      const key = [Math.min(a, b), Math.max(a, b)].join(",");
      const incidence = edges.get(key) ?? { uses: 0, orientation: 0 };
      incidence.uses++;
      incidence.orientation += a < b ? 1 : -1;
      edges.set(key, incidence);
    }
  }
  for (const { uses, orientation } of edges.values()) {
    if (uses !== 2 || orientation !== 0) throw new Error("Display edge lacks two opposite triangle uses");
  }
  const triangles = indices.length / 3;
  return {
    triangles, edges: edges.size, eulerCharacteristic: vertices.size - edges.size + triangles,
    source: mesh.geometrySource, operation: mesh.kernelOperation, geometryAssertions: mesh.geometryAssertions,
  };
}

async function run(request: OrderedMappedStepRequest) {
  await OpenCascadeKernel.initialize();
  const kernel = new OpenCascadeKernel();
  const { operation, source } = request;
  const genericFailure = rejected(() => kernel.disposeShape(kernel.importStep(source)));
  const shape = kernel.importStep(source, repair);
  try {
    const evidence = kernel.nativeStepRepairEvidence(shape);
    if (!evidence?.nativeProofJSON || kernel.nativeStepSourceUnit(shape) !== undefined) {
      throw new Error("Missing native receipt or unexpected source unit");
    }
    if (operation === "repair") {
      const legacyFailure = rejected(() => kernel.disposeShape(kernel.importStep(source,
        { stepRepair: { maxDistance: repair.stepRepair.maxDistance } })));
      const distanceFailure = rejected(() => kernel.tessellate(shape, { linearDeflection: .35, angularDeflection: Math.PI }));
      const angularFailure = rejected(() => kernel.tessellate(shape, { linearDeflection: .36, angularDeflection: .2 }));
      const display = proveClosedDisplay(kernel.tessellate(shape,
        { linearDeflection: .36, angularDeflection: Math.PI, analyticNormals: true }));
      const exported = kernel.exportStep([shape]);
      worker.postMessage({ ok: true, operation, evidence, genericFailure, legacyFailure, distanceFailure,
        angularFailure, display, exported }, [exported.bytes]);
    } else {
      const exported = kernel.exportStep([shape]);
      worker.postMessage({ ok: true, operation, evidence, genericFailure, exported }, [exported.bytes]);
    }
  } finally { kernel.disposeShape(shape); }
}

worker.onmessage = event => {
  void run(event.data).catch(error => worker.postMessage({ ok: false, error: String(error) }));
};
