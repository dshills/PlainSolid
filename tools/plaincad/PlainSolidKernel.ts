// Installed as OpenCascadeKernel.ts only in the isolated acceptance workspace.
// The class name preserves the application's import boundary; all geometry is Go.
import { rectangularShellDimensions, requireShellOpening } from "../features/shellFeature";
import type { NativeStepExport } from "./nativeStep";
import type { ComponentPlacement } from "../document/schema";
import { placementTransform } from "../document/componentPlacement";
import { createId } from "../document/ids";
import type { KernelAdapter, KernelShape, RenderMesh, GeometryAssertions, BoundingBox, TessellationOptions } from "./KernelAdapter";
import type { SketchProfile, ProfileLoop } from "../sketch/profileDetection";
import { transformPoint, type SketchPlaneTransform } from "../sketch/planes";

type Proof = { handle: string; geometryAssertions: GeometryAssertions; bounds: BoundingBox };
type Handle = { proof: Proof; operation: RenderMesh["kernelOperation"]; extrusion?: { profile: SketchProfile; distance: number } };
type Runtime = typeof globalThis & {
  Go?: new () => { importObject: WebAssembly.Imports; run(instance: WebAssembly.Instance): Promise<void> };
  plainSolidCall?: (request: string) => string;
};
const runtime = globalThis as Runtime;
let initialization: Promise<void> | undefined;

function request<T>(args: Record<string, unknown>): T {
  if (!runtime.plainSolidCall) throw new Error("PlainSolid WebAssembly is not initialized.");
  const response = JSON.parse(runtime.plainSolidCall(JSON.stringify(args)));
  if (!response.ok) throw new Error(`PlainSolid ${response.error.code}: ${response.error.message}`);
  return response.result as T;
}
function unsupported(..._args: unknown[]): never {
  throw new Error("This operation is not yet supported by the PlainSolid acceptance adapter.");
}
function handle(shape: KernelShape): Handle { return shape.kernelHandle as Handle; }
function owned(proof: Proof, operation: RenderMesh["kernelOperation"], id = createId("shape")): KernelShape {
  return { id, kernelHandle: { proof, operation } satisfies Handle };
}
const xyz = (point: { x: number; y: number; z: number }) => [point.x, point.y, point.z];
const defaultPlane: SketchPlaneTransform = { origin: { x: 0, y: 0, z: 0 }, u: { x: 1, y: 0, z: 0 }, v: { x: 0, y: 1, z: 0 }, normal: { x: 0, y: 0, z: 1 } };

export class OpenCascadeKernel implements KernelAdapter {
  static async initialize(): Promise<void> {
    initialization ??= (async () => {
      // Dynamic module import works in both module workers and the window.
      await import("./wasm_exec.js");
      if (!runtime.Go) throw new Error("Go WebAssembly runtime did not initialize.");
      const go = new runtime.Go();
      const response = await fetch("/plainsolid/kernel.wasm");
      if (!response.ok) throw new Error(`PlainSolid WASM fetch failed: ${response.status}`);
      const compiled = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
      void go.run(compiled.instance);
      if (!runtime.plainSolidCall) throw new Error("PlainSolid WASM did not install its dispatcher.");
      request({ op: "capabilities" });
    })();
    return initialization;
  }
  static isInitialized() { return typeof runtime.plainSolidCall === "function"; }
  static stepExportDiagnostic() { return "PlainSolid STEP adapter is not implemented."; }
  static stepImportDiagnostic() { return "PlainSolid STEP adapter is not implemented."; }
  static shellAvailable() { return this.isInitialized(); }
  static threadsAvailable() { return false; }

  createBox(width: number, height: number, depth: number): KernelShape {
    return owned(request({ op: "box", min: [0, 0, 0], max: [width, height, depth] }), "box");
  }
  extrudeProfile(profile: SketchProfile, distance: number, transform = defaultPlane): KernelShape {
    const loop = (boundary: ProfileLoop) => {
      if (boundary.type !== "polygon" || !boundary.segments?.length || boundary.segments.some(segment => segment.type !== "line"))
        throw new Error("PlainSolid adapter currently requires exact straight polygon profiles; circular and arc profiles are unsupported.");
      return boundary.segments.map(segment => xyz(transformPoint(transform, segment.start.x, segment.start.y)));
    };
    const shape = owned(request({ op: "prism", outer: loop(profile.outerLoop), holes: profile.innerLoops.map(loop), offset: xyz(transform.normal).map(value => value * distance) }), "extrusion");
    handle(shape).extrusion = { profile: structuredClone(profile), distance };
    return shape;
  }
  cloneShape(shape: KernelShape): KernelShape {
    const clone = owned(request({ op: "clone", handle: handle(shape).proof.handle }), handle(shape).operation, shape.id);
    if (handle(shape).extrusion) handle(clone).extrusion = structuredClone(handle(shape).extrusion);
    return clone;
  }
  placeShape(shape: KernelShape, placement: ComponentPlacement): KernelShape {
    const frame = placementTransform(placement);
    const matrix = (["x", "y", "z"] as const).map(axis => [frame.u[axis], frame.v[axis], frame.normal[axis], frame.origin[axis]]);
    return owned(request({ op: "transform", handle: handle(shape).proof.handle, matrix }), handle(shape).operation, shape.id);
  }
  nativeBounds(shape: KernelShape): BoundingBox { return request<Proof>({ op: "inspect", handle: handle(shape).proof.handle }).bounds; }
  private boolean(base: KernelShape, tool: KernelShape, op: "cut" | "fuse", exportUnion = false): KernelShape {
    const a = request<Proof>({ op: "inspect", handle: handle(base).proof.handle });
    const b = request<Proof>({ op: "inspect", handle: handle(tool).proof.handle });
    const result = request<Proof>({ op, a: a.handle, b: b.handle });
    try {
      const before = a.geometryAssertions.volume, after = result.geometryAssertions;
      const tolerance = Math.max(1e-9, Math.abs(before) * 1e-10);
      if (!after.valid || !Number.isFinite(after.volume) || after.volume <= 1e-12 || after.solidCount < 1)
        throw new Error("Boolean operation produced no solid volume.");
      if (op === "cut" && before - after.volume <= tolerance)
        throw new Error("Boolean cut removed no volume. Move the tool into the target body.");
      if (op === "fuse" && !exportUnion && after.solidCount !== 1)
        throw new Error("Boolean join requires connected solids.");
      if (op === "fuse" && !exportUnion && after.volume - before <= tolerance)
        throw new Error("Boolean join added no volume. The tool is already contained in the target body.");
      if (op === "fuse" && (after.volume > before + b.geometryAssertions.volume + tolerance || after.volume < Math.max(before, b.geometryAssertions.volume) - tolerance))
        throw new Error("Boolean union produced an inconsistent solid volume.");
      return owned(result, op, base.id);
    } catch (error) { request({ op: "dispose", handle: result.handle }); throw error; }
  }
  cut(base: KernelShape, tool: KernelShape): KernelShape { return this.boolean(base, tool, "cut"); }
  fuse(base: KernelShape, tool: KernelShape): KernelShape { return this.boolean(base, tool, "fuse"); }
  unionForExport(base: KernelShape, tool: KernelShape): KernelShape { return this.boolean(base, tool, "fuse", true); }
  cutAll(base: KernelShape, tools: KernelShape[]): KernelShape {
    let current = this.cloneShape(base);
    try { for (const tool of tools) { const next = this.cut(current, tool); this.disposeShape(current); current = next; } return current; }
    catch (error) { this.disposeShape(current); throw error; }
  }
  tessellate(shape: KernelShape, _options: TessellationOptions): RenderMesh {
    const mesh = request<Omit<RenderMesh, "id" | "bodyId">>({ op: "mesh", handle: handle(shape).proof.handle });
    return { ...mesh, id: shape.id, bodyId: shape.id, geometrySource: "plainsolid", kernelOperation: handle(shape).operation };
  }
  exportStl(shape: KernelShape): ArrayBuffer {
    const { base64 } = request<{ base64: string }>({ op: "stl", handle: handle(shape).proof.handle });
    return Uint8Array.from(atob(base64), character => character.charCodeAt(0)).buffer;
  }
  disposeShape(shape: KernelShape) { request({ op: "dispose", handle: handle(shape).proof.handle }); }
  // Unproved reference queries reject or return no eligible references.
  validatePlanarFace(shape: KernelShape, plane: SketchPlaneTransform): void {
    request({ op: "planarFace", handle: handle(shape).proof.handle, origin: xyz(plane.origin), normal: xyz(plane.normal) });
  }
  validatePocketFloor?: KernelAdapter["validatePocketFloor"];
  measureHoleRim?: KernelAdapter["measureHoleRim"];
  availableExtrudeCapEdges(_shape: KernelShape): ReturnType<NonNullable<KernelAdapter["availableExtrudeCapEdges"]>> { return []; }
  edgeProofSignature(_shape: KernelShape): string | undefined { return undefined; }
  hasCommonVolume(base: KernelShape, tool: KernelShape): boolean {
    const a = request<Proof>({ op: "inspect", handle: handle(base).proof.handle });
    const b = request<Proof>({ op: "inspect", handle: handle(tool).proof.handle });
    const upper = Math.min(a.geometryAssertions.volume, b.geometryAssertions.volume);
    const tolerance = Math.max(1e-9, Math.abs(upper) * 1e-10);
    const proof = request<Proof>({ op: "common", a: a.handle, b: b.handle });
    try {
      const volume = proof.geometryAssertions.volume;
      if (!Number.isFinite(volume) || volume < -tolerance || volume > upper + tolerance)
        throw new Error("Scope intersection produced inconsistent volume.");
      return volume > tolerance;
    } finally { request({ op: "dispose", handle: proof.handle }); }
  }
  cutScope: NonNullable<KernelAdapter["cutScope"]> = unsupported;
  joinAll: NonNullable<KernelAdapter["joinAll"]> = unsupported;
  nativeDrawingGeometry?: KernelAdapter["nativeDrawingGeometry"];
  getWasmHeapCapacityBytes() { return undefined; }
  revolveProfile = unsupported;
  extrudeToFace = unsupported;
  fillet = unsupported;
  chamfer = unsupported;
  shell(shape: KernelShape, opening: "startCap" | "endCap", wall: number): KernelShape {
    requireShellOpening(opening);
    const source = handle(shape).extrusion;
    const dimensions = source && rectangularShellDimensions(source.profile);
    if (!source || !dimensions || handle(shape).operation !== "extrusion" || source.distance <= 0)
      throw new Error("Shell requires one unchanged positive rectangular extrusion without holes.");
    const [width, height] = dimensions, depth = source.distance;
    if (!Number.isFinite(wall) || wall < 1e-6) throw new Error("Wall thickness must be at least 0.000001 mm.");
    if (Math.min(width - 2 * wall, height - 2 * wall, depth - wall) <= 1e-6)
      throw new Error("Wall thickness collapses the cavity.");
    const before = handle(shape).proof.geometryAssertions;
    const original = width * height * depth;
    const removed = (width - 2 * wall) * (height - 2 * wall) * (depth - wall);
    const expected = original - removed;
    const tolerance = Math.max(1e-9, Math.abs(before.volume) * 1e-10);
    if (removed <= tolerance || expected <= tolerance) throw new Error("Shell requires measurable wall and cavity volumes.");
    if (before.solidCount !== 1 || Math.abs(before.volume - original) > Math.max(1e-7, original * 1e-8))
      throw new Error("Shell requires one unchanged rectangular solid.");
    const result = request<Proof>({ op: "shell", handle: handle(shape).proof.handle, openFace: opening === "startCap" ? 0 : 1, wall });
    if (result.geometryAssertions.solidCount !== 1 || before.volume - result.geometryAssertions.volume <= tolerance || Math.abs(result.geometryAssertions.volume - expected) > Math.max(1e-7, expected * 1e-8)) {
      request({ op: "dispose", handle: result.handle });
      throw new Error("Shell did not produce its expected cavity volume.");
    }
    return owned(result, "shell", shape.id);
  }
  thread = unsupported;
  leadPart = unsupported;
  assertLeadPair = unsupported;
  importStep = unsupported;
  exportStep(_shapes: readonly KernelShape[]): NativeStepExport { return unsupported(); }
}
