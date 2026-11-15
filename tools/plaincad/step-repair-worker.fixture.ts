import { OpenCascadeKernel, type NativeStepRepairEvidence } from "./OpenCascadeKernel";
import type { NativeStepExport } from "./nativeStep";

export interface StepRepairProbeRequest {
  source: string;
  repair?: true;
}

export type StepRepairProbeReply = {
  ok: true;
  before: NativeStepExport["before"];
  after: NativeStepExport["after"];
  canonical: ArrayBuffer;
  evidence?: NativeStepRepairEvidence;
  repair?: {
    certificateFromWasm: ArrayBuffer;
    strictFailure: string;
    triangles: number;
    positionsInWorldBounds: boolean;
  };
} | { ok: false; error: string };

const host = globalThis as unknown as {
  plainSolidCall?: (request: string) => string;
  postMessage(reply: StepRepairProbeReply, transfer?: ArrayBuffer[]): void;
  onmessage: (event: MessageEvent<StepRepairProbeRequest>) => void;
};

async function runProbe({ source, repair }: StepRepairProbeRequest) {
  await OpenCascadeKernel.initialize();
  const kernel = new OpenCascadeKernel();
  if (!repair) {
    const shape = kernel.importStep(source);
    try {
      const exported = kernel.exportStep([shape]);
      host.postMessage({ ok: true, before: exported.before, after: exported.after, canonical: exported.bytes,
        evidence: kernel.nativeStepRepairEvidence(shape) }, [exported.bytes]);
    } finally { kernel.disposeShape(shape); }
    return;
  }

  let strictFailure = "";
  try { kernel.disposeShape(kernel.importStep(source)); }
  catch (error) { strictFailure = error instanceof Error ? error.message : String(error); }
  const dispatch = host.plainSolidCall;
  if (!dispatch) throw new Error("Missing the Go WASM dispatcher");
  let certificateFromWire: string | undefined;
  host.plainSolidCall = request => {
    const response = dispatch(request);
    certificateFromWire = JSON.parse(response).result?.nativeProofJSON;
    return response;
  };
  const shape = (() => {
    try { return kernel.importStep(source, { stepRepair: { maxDistance: 1e-12, native: true } }); }
    finally { host.plainSolidCall = dispatch; }
  })();
  try {
    if (!certificateFromWire) throw new Error("Missing the native certificate in the Go WASM response");
    const certificateFromWasm = new TextEncoder().encode(certificateFromWire).buffer;
    const mesh = kernel.tessellate(shape, { linearDeflection: .025, angularDeflection: .2, analyticNormals: true });
    const { min, max } = mesh.bounds;
    const positionsInWorldBounds = Array.from(mesh.positions).every((value, i) => value >= min[i % 3] - 1e-12 && value <= max[i % 3] + 1e-12);
    const exported = kernel.exportStep([shape]);
    host.postMessage({ ok: true, before: exported.before, after: exported.after, canonical: exported.bytes,
      evidence: kernel.nativeStepRepairEvidence(shape),
      repair: { certificateFromWasm, strictFailure, triangles: mesh.indices.length / 3, positionsInWorldBounds },
    }, [exported.bytes, certificateFromWasm]);
  } finally { kernel.disposeShape(shape); }
}

host.onmessage = event => {
  void runProbe(event.data).catch(error => host.postMessage({ ok: false, error: String(error) }));
};
