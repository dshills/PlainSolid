import { OpenCascadeKernel, type NativeStepRepairEvidence, type StepSourceUnit } from "./OpenCascadeKernel";
import type { KernelShape } from "./KernelAdapter";
import type { NativeStepExport } from "./nativeStep";

export type StepRepairProbeRequest = { source: string; units?: StepSourceUnit } & (
  { operation: "repair"; otherSource?: string }
  | { operation: "reopen"; receipt: { certificate: string; fromWasm: ArrayBuffer } }
);

export type StepRepairProbeReply = {
  ok: true;
  before: NativeStepExport["before"];
  after: NativeStepExport["after"];
  canonical: ArrayBuffer;
  units: "mm" | StepSourceUnit;
  proofUnits?: "mm";
  sourceUnit?: StepSourceUnit;
  evidence?: NativeStepRepairEvidence;
  repair?: {
    certificateFromWasm: ArrayBuffer;
    strictFailure: string;
    triangles: number;
    positionsInWorldBounds: boolean;
    wrongImport?: string;
    omittedExport?: string;
    wrongExport?: string;
    mixedExport?: string;
  };
  reopen?: {
    reprovedCertificate: string;
    omittedImport?: string;
    wrongImport?: string;
  };
} | { ok: false; error: string };

const host = globalThis as unknown as {
  plainSolidCall?: (request: string) => string;
  postMessage(reply: StepRepairProbeReply, transfer?: ArrayBuffer[]): void;
  onmessage: (event: MessageEvent<StepRepairProbeRequest>) => void;
};

function failure(action: () => void): string {
  try { action(); }
  catch (error) { return error instanceof Error ? error.message : String(error); }
  return "";
}

function exportOccurrence(kernel: OpenCascadeKernel, shape: KernelShape, units?: StepSourceUnit) {
  const exported = units ? kernel.exportStep([shape], { units }) : kernel.exportStep([shape]);
  return { before: exported.before, after: exported.after, canonical: exported.bytes, units: exported.units,
    proofUnits: "proofUnits" in exported ? exported.proofUnits : undefined,
    sourceUnit: kernel.nativeStepSourceUnit(shape), evidence: kernel.nativeStepRepairEvidence(shape) };
}

async function runProbe(request: StepRepairProbeRequest) {
  await OpenCascadeKernel.initialize();
  const kernel = new OpenCascadeKernel();
  const { source, units } = request;
  const otherUnit = units === "m" ? "in" : "m";
  if (request.operation === "reopen") {
    const omittedImport = units ? failure(() => kernel.disposeShape(kernel.importStep(source))) : undefined;
    const wrongImport = units ? failure(() => kernel.disposeShape(kernel.importStep(source, { units: otherUnit }))) : undefined;
    const shape = kernel.importStep(source, { units });
    try {
      const { certificate, fromWasm } = request.receipt;
      const decoder = new TextDecoder("utf-8", { fatal: true });
      if (decoder.decode(fromWasm) !== certificate) throw new Error("The transported STEP certificate bytes changed");
      const { Source, MaxDistance } = JSON.parse(certificate);
      const original = decoder.decode(Uint8Array.from(atob(Source), character => character.charCodeAt(0)));
      const reproved = kernel.importStep(original, { units, stepRepair: { maxDistance: MaxDistance, native: true } });
      let reprovedCertificate: string | undefined;
      try { reprovedCertificate = kernel.nativeStepRepairEvidence(reproved)?.nativeProofJSON; }
      finally { kernel.disposeShape(reproved); }
      if (reprovedCertificate !== certificate) throw new Error("A fresh worker derived a different exact STEP certificate");

      const result = exportOccurrence(kernel, shape, units);
      host.postMessage({ ok: true, ...result, reopen: { reprovedCertificate, omittedImport, wrongImport } }, [result.canonical]);
    } finally { kernel.disposeShape(shape); }
    return;
  }

  const strictFailure = failure(() => kernel.disposeShape(kernel.importStep(source, { units })));
  const wrongImport = units ? failure(() => kernel.disposeShape(kernel.importStep(source,
    { units: otherUnit, stepRepair: { maxDistance: 1e-12 } }))) : undefined;
  const dispatch = host.plainSolidCall;
  if (!dispatch) throw new Error("Missing the Go WASM dispatcher");
  let certificateFromWire: string | undefined;
  host.plainSolidCall = request => {
    const response = dispatch(request);
    certificateFromWire = JSON.parse(response).result?.nativeProofJSON;
    return response;
  };
  const shape = (() => {
    try { return kernel.importStep(source, { units, stepRepair: { maxDistance: 1e-12, native: true } }); }
    finally { host.plainSolidCall = dispatch; }
  })();
  try {
    if (!certificateFromWire) throw new Error("Missing the native certificate in the Go WASM response");
    const certificateFromWasm = new TextEncoder().encode(certificateFromWire).buffer;
    const millimetres = units === "m" ? 1000 : units === "in" ? 25.4 : 1;
    const mesh = kernel.tessellate(shape, { linearDeflection: .025 * millimetres, angularDeflection: .2, analyticNormals: true });
    const { min, max } = mesh.bounds;
    const margin = 1e-12 * millimetres;
    const positionsInWorldBounds = Array.from(mesh.positions).every((value, i) => value >= min[i % 3] - margin && value <= max[i % 3] + margin);
    let omittedExport: string | undefined, wrongExport: string | undefined, mixedExport: string | undefined;
    if (units) {
      if (!request.otherSource) throw new Error("Missing the independently authored opposite-unit STEP fixture");
      omittedExport = failure(() => kernel.exportStep([shape]));
      wrongExport = failure(() => kernel.exportStep([shape], { units: otherUnit }));
      const other = kernel.importStep(request.otherSource, { units: otherUnit, stepRepair: { maxDistance: 1e-12 } });
      try { mixedExport = failure(() => kernel.exportStep([shape, other], { units })); }
      finally { kernel.disposeShape(other); }
    }
    const result = exportOccurrence(kernel, shape, units);
    host.postMessage({ ok: true, ...result, repair: { certificateFromWasm, strictFailure, triangles: mesh.indices.length / 3,
      positionsInWorldBounds, wrongImport, omittedExport, wrongExport, mixedExport } }, [result.canonical, certificateFromWasm]);
  } finally { kernel.disposeShape(shape); }
}

host.onmessage = event => {
  void runProbe(event.data).catch(error => host.postMessage({ ok: false, error: String(error) }));
};
