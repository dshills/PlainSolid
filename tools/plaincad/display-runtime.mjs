import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";

export async function installDisplayRuntime(work, hash) {
  const detailClient = "src/cad/worker/displayMeshClient.ts";
  const client = await readFile(join(work, detailClient), "utf8").catch(error => {
    if (error.code === "ENOENT") return undefined;
    throw error;
  });
  if (client === undefined) return [];

  const changes = [];
  async function install(file, match, replacement) {
    const path = join(work, file);
    const before = await readFile(path, "utf8");
    if (before.split(match).length !== 2) throw new Error(`PlainCAD display worker boundary changed in ${file}.`);
    const after = before.replace(match, replacement);
    await writeFile(path, after);
    changes.push({
      file, purpose: "Tessellate detail from the accepted geometry worker's native feature cache.",
      beforeSha256: hash(before), afterSha256: hash(after),
    });
  }

  await install(detailClient,
    "  return backgroundJob<DisplayMeshRequest, DisplayMesh[]>(PreviewWorker, request, signal, undefined, 20000);",
    `  const isolated = () => backgroundJob<DisplayMeshRequest, DisplayMesh[]>(PreviewWorker, request, signal, undefined, 20000);
  // Small rebuilds use the independently cancellable worker. Retained
  // geometry avoids repeating the modeling work for longer feature graphs.
  const minRetainedFeatures = 8;
  if (request.document.features.length < minRetainedFeatures) return isolated();
  return import("../../state/useCadStore").then(({ requestRetainedDisplayMeshes }) =>
    requestRetainedDisplayMeshes(request, signal) ?? isolated(),
  );`);

  await install("src/state/useCadStore.ts",
    "function getGeometryWorker(): Worker | undefined {",
    `export function requestRetainedDisplayMeshes(
  request: import("../cad/kernel/displayMesh").DisplayMeshRequest, signal: AbortSignal,
): Promise<import("../cad/kernel/displayMesh").DisplayMesh[]> | undefined {
  const state = useCadStore.getState(), worker = geometryWorker, epoch = workerEpoch;
  if (!worker || !kernelInitialized || state.history.present !== request.document ||
      state.rebuild.status !== "succeeded" || !state.rebuild.result?.success) return;
  const requestId = nextRequestId(), result = state.rebuild.result, session = state.documentSession;
  return new Promise((resolve, reject) => {
    if (signal.aborted) { reject(new Error("Operation cancelled.")); return; }
    const finish = () => {
      clearTimeout(timer);
      signal.removeEventListener("abort", cancel);
      worker.removeEventListener("message", message);
      worker.removeEventListener("error", workerError);
      worker.removeEventListener("messageerror", workerError);
    };
    const cancel = () => { finish(); reject(new Error("Operation cancelled.")); };
    const workerError = () => { finish(); reject(new Error("Geometry worker failed.")); };
    const message = (event: MessageEvent) => {
      const reply = event.data;
      if (!reply || reply.type !== "plainSolidDisplayResult" || reply.requestId !== requestId || reply.epoch !== epoch) return;
      finish();
      const current = useCadStore.getState();
      if (geometryWorker !== worker || current.history.present !== request.document ||
          current.documentSession !== session || current.rebuild.result !== result) {
        reject(new Error("The display geometry is no longer current."));
      } else if (reply.error || !Array.isArray(reply.meshes)) {
        reject(new Error(reply.error ?? "Geometry worker returned malformed display detail."));
      } else resolve(reply.meshes);
    };
    const timer = setTimeout(() => { finish(); reject(new Error("Operation exceeded its resource/time limit. Simplify the project and retry.")); }, 20000);
    signal.addEventListener("abort", cancel, { once: true });
    worker.addEventListener("message", message);
    worker.addEventListener("error", workerError);
    worker.addEventListener("messageerror", workerError);
    try {
      worker.postMessage({ type: "plainSolidDisplay", requestId, epoch, request });
    } catch (error) {
      finish();
      reject(error);
    }
  });
}

function getGeometryWorker(): Worker | undefined {`);

  await install("src/cad/worker/geometryWorker.ts",
    `self.onmessage = (event: MessageEvent<WorkerRequest>) => {
  const request = event.data;`,
    `type DisplayWork = {
  type: "plainSolidDisplay"; requestId: number; epoch: number;
  request: import("../kernel/displayMesh").DisplayMeshRequest;
};

self.onmessage = (event: MessageEvent<WorkerRequest | DisplayWork>) => {
  const request = event.data;
  if (request.type === "plainSolidDisplay") {
    const job = request;
    requestQueue = requestQueue.then(() => {
      const reply = { type: "plainSolidDisplayResult", requestId: job.requestId, epoch: job.epoch };
      try {
        const result = rebuildDocument(job.request.document, { epoch: job.epoch, displayMeshRequest: job.request.bodies });
        if (!result.success || !result.displayMeshes) throw new Error(result.errors[0]?.message ?? "Close-up detail did not rebuild.");
        const meshes = result.displayMeshes;
        self.postMessage({ ...reply, meshes }, { transfer: meshes.flatMap(mesh => [
          mesh.positions.buffer, mesh.normals.buffer, mesh.indices.buffer,
          ...(mesh.nativeEdgeSegments ? [mesh.nativeEdgeSegments.buffer] : []),
        ]) });
      } catch (error) {
        self.postMessage({ ...reply, error: error instanceof Error ? error.message : String(error) });
      }
    }).catch(recoverQueue);
    return;
  }`);
  return changes;
}
