import { OpenCascadeKernel } from "./OpenCascadeKernel";
import type { BoundingBox, RenderMesh } from "./KernelAdapter";
import type { NativeStepExport } from "./nativeStep";

export type ConicalBoreWorkerRequest = {
  operation: "importAndMesh";
  source: string;
  rounded: string;
} | { operation: "reopen"; source: string };

type SourceProof = {
  bounds: BoundingBox;
  exported: NativeStepExport;
  importMs: number;
  exportMs: number;
};

export type ConicalBoreWorkerReply =
  | ({ ok: true; operation: "importAndMesh"; roundedRejection: string;
    meshMs: number; meshSHA256: string; display: ReturnType<typeof proveClosedDisplay> } & SourceProof)
  | ({ ok: true; operation: "reopen" } & SourceProof)
  | { ok: false; error: string };

const worker = globalThis as unknown as {
  onmessage: (event: MessageEvent<ConicalBoreWorkerRequest>) => void;
  postMessage(reply: ConicalBoreWorkerReply, transfer?: ArrayBuffer[]): void;
};

function proveClosedDisplay(mesh: RenderMesh) {
  const { positions, normals, indices, geometryAssertions } = mesh;
  if (!positions.length || positions.length !== normals.length || positions.length % 3 ||
      !indices.length || indices.length % 3 || !Array.from(positions).every(Number.isFinite) ||
      !Array.from(normals).every(Number.isFinite) || !geometryAssertions?.valid) {
    throw new Error("Invalid native display");
  }
  const vertices = new Map<string, number>();
  const welded: number[] = [];
  for (let i = 0; i < positions.length; i += 3) {
    // The renderer can use multiple vertices at one exact point when a
    // cap and the bore wall have distinct normals.
    const key = [positions[i], positions[i + 1], positions[i + 2]].join(",");
    if (!vertices.has(key)) vertices.set(key, vertices.size);
    welded.push(vertices.get(key)!);
  }
  if (indices.some(i => !Number.isInteger(i) || i < 0 || i >= welded.length)) {
    throw new Error("Display index is outside the vertex buffer");
  }
  const edges = new Map<string, { uses: number; orientation: number }>();
  let signedVolume = 0;
  for (let i = 0; i < indices.length; i += 3) {
    const offsets = indices.slice(i, i + 3).map(index => index * 3);
    const [a, b, c] = offsets.map(offset => [positions[offset], positions[offset + 1], positions[offset + 2]]);
    signedVolume += (a[0] * (b[1] * c[2] - b[2] * c[1]) + a[1] * (b[2] * c[0] - b[0] * c[2]) +
      a[2] * (b[0] * c[1] - b[1] * c[0])) / 6;
    const triangle = indices.slice(i, i + 3).map(index => welded[index]);
    if (new Set(triangle).size !== 3) throw new Error("Degenerate display triangle");
    for (let side = 0; side < 3; side++) {
      const first = triangle[side], next = triangle[(side + 1) % 3];
      const key = [Math.min(first, next), Math.max(first, next)].join(",");
      const incidence = edges.get(key) ?? { uses: 0, orientation: 0 };
      incidence.uses++;
      incidence.orientation += first < next ? 1 : -1;
      edges.set(key, incidence);
    }
  }
  if (Array.from(edges.values()).some(edge => edge.uses !== 2 || edge.orientation !== 0)) {
    throw new Error("The display is missing an oppositely oriented triangle use");
  }
  const triangles = indices.length / 3;
  return { triangles, vertices: vertices.size, edges: edges.size, eulerCharacteristic: vertices.size - edges.size + triangles,
    signedVolume, source: mesh.geometrySource, operation: mesh.kernelOperation, geometryAssertions };
}

function rejectRounded(kernel: OpenCascadeKernel, rounded: string) {
  try {
    kernel.disposeShape(kernel.importStep(rounded));
  } catch (error) {
    return error instanceof Error ? error.message : String(error);
  }
  throw new Error("The rounded conical source unexpectedly imported");
}

async function run(request: ConicalBoreWorkerRequest): Promise<ConicalBoreWorkerReply> {
  await OpenCascadeKernel.initialize();
  const kernel = new OpenCascadeKernel();
  const started = performance.now();
  const shape = kernel.importStep(request.source);
  const importMs = performance.now() - started;
  try {
    if (kernel.nativeStepRepairEvidence(shape)) throw new Error("Ordinary import unexpectedly reported a repair");
    const bounds = kernel.nativeBounds(shape);
    const exportSource = () => {
      const started = performance.now();
      const exported = kernel.exportStep([shape]);
      return { bounds, exported, importMs, exportMs: performance.now() - started };
    };
    if (request.operation === "reopen") return { ok: true, operation: "reopen", ...exportSource() };

    const roundedRejection = rejectRounded(kernel, request.rounded);
    const meshStart = performance.now();
    const mesh = kernel.tessellate(shape, { linearDeflection: .05, angularDeflection: .2, analyticNormals: true });
    const meshMs = performance.now() - meshStart;
    const display = proveClosedDisplay(mesh);
    // Match the dispatcher payload before the typed adapter adds its body ID
    // and source labels. That lets the browser verify the same Node/WASM hash.
    const { positions, normals, indices, bounds: meshBounds, geometryAssertions } = mesh;
    const raw = JSON.stringify({ positions, normals, indices, bounds: meshBounds, geometryAssertions });
    const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(raw));
    const meshSHA256 = Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, "0")).join("");
    return { ok: true, operation: "importAndMesh", ...exportSource(), roundedRejection, display, meshMs, meshSHA256 };
  } finally {
    kernel.disposeShape(shape);
  }
}

worker.onmessage = event => {
  void run(event.data).then(reply => {
    worker.postMessage(reply, reply.ok ? [reply.exported.bytes] : undefined);
  }, error => worker.postMessage({ ok: false, error: String(error) }));
};
