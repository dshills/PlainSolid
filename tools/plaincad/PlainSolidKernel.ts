// Installed as OpenCascadeKernel.ts only in the isolated acceptance workspace.
// The class name preserves the application's import boundary; all geometry is Go.
import { rectangularShellDimensions, requireShellOpening } from "../features/shellFeature";
import type { NativeStepExport } from "./nativeStep";
import type { ComponentPlacement, TopologyRef } from "../document/schema";
import { placementTransform } from "../document/componentPlacement";
import { createId } from "../document/ids";
import type { KernelAdapter, KernelShape, RenderMesh, GeometryAssertions, BoundingBox, TessellationOptions } from "./KernelAdapter";
import { HoleScopeError } from "./KernelAdapter";
import type { SketchProfile, ProfileLoop } from "../sketch/profileDetection";
import { transformPoint, type SketchPlaneTransform } from "../sketch/planes";

type Proof = { handle: string; geometryAssertions: GeometryAssertions; bounds: BoundingBox };
type Handle = {
  proof: Proof;
  operation: RenderMesh["kernelOperation"];
  extrusion?: { profile: SketchProfile; distance: number; transform: SketchPlaneTransform };
};
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
    if (profile.outerLoop.type === "circle") {
      if (profile.innerLoops.length || profile.holes.length)
        throw new Error("PlainSolid circular extrusion does not yet support inner loops.");
      const { minX, maxX, minY, maxY } = profile.bounds;
      const diameter = maxX - minX, otherDiameter = maxY - minY;
      if (![minX, maxX, minY, maxY, diameter, otherDiameter, distance].every(Number.isFinite) || diameter <= 0 || otherDiameter <= 0 || distance === 0 ||
          Math.abs(diameter - otherDiameter) > 1e-8 * Math.max(1, diameter, otherDiameter))
        throw new Error("Circular extrusion requires a finite circle and nonzero distance.");
      const center = transformPoint(transform, minX + diameter / 2, minY + otherDiameter / 2);
      const shape = owned(request({ op: "circle", origin: xyz(center), normal: xyz(transform.normal), radius: diameter / 2, height: distance }), "extrusion");
      handle(shape).extrusion = { profile: structuredClone(profile), distance, transform: structuredClone(transform) };
      return shape;
    }
    const loop = (boundary: ProfileLoop) => {
      if (boundary.type !== "polygon" || !boundary.segments?.length || boundary.segments.some(segment => segment.type !== "line"))
        throw new Error("PlainSolid adapter requires exact straight polygon loops or a single full circle; mixed arcs and curved inner loops are unsupported.");
      return boundary.segments.map(segment => xyz(transformPoint(transform, segment.start.x, segment.start.y)));
    };
    const shape = owned(request({ op: "prism", outer: loop(profile.outerLoop), holes: profile.innerLoops.map(loop), offset: xyz(transform.normal).map(value => value * distance) }), "extrusion");
    handle(shape).extrusion = { profile: structuredClone(profile), distance, transform: structuredClone(transform) };
    return shape;
  }
  cloneShape(shape: KernelShape): KernelShape {
    return this.retainExtrusion(
      owned(request({ op: "clone", handle: handle(shape).proof.handle }), handle(shape).operation, shape.id),
      shape,
    );
  }
  placeShape(shape: KernelShape, placement: ComponentPlacement): KernelShape {
    const frame = placementTransform(placement);
    const matrix = (["x", "y", "z"] as const).map(axis => [frame.u[axis], frame.v[axis], frame.normal[axis], frame.origin[axis]]);
    const result = this.retainExtrusion(
      owned(request({ op: "transform", handle: handle(shape).proof.handle, matrix }), handle(shape).operation, shape.id),
      shape,
    );
    const source = handle(result).extrusion;
    if (source) {
      const vector = (v: { x: number; y: number; z: number }) => ({
        x: frame.u.x * v.x + frame.v.x * v.y + frame.normal.x * v.z,
        y: frame.u.y * v.x + frame.v.y * v.y + frame.normal.y * v.z,
        z: frame.u.z * v.x + frame.v.z * v.y + frame.normal.z * v.z,
      });
      const t = source.transform;
      source.transform = {
        origin: transformPoint(frame, t.origin.x, t.origin.y, t.origin.z),
        u: vector(t.u), v: vector(t.v), normal: vector(t.normal),
      };
    }
    return result;
  }
  private retainExtrusion(result: KernelShape, source: KernelShape): KernelShape {
    if (handle(source).extrusion) handle(result).extrusion = structuredClone(handle(source).extrusion);
    return result;
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
      return this.retainExtrusion(owned(result, op, base.id), base);
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
  tessellate(shape: KernelShape, options: TessellationOptions): RenderMesh {
    const mesh = request<Omit<RenderMesh, "id" | "bodyId">>({ op: "mesh", handle: handle(shape).proof.handle, linearDeflection: options.linearDeflection, angularDeflection: options.angularDeflection });
    return { ...mesh, id: shape.id, bodyId: shape.id, geometrySource: "plainsolid", kernelOperation: handle(shape).operation };
  }
  exportStl(shape: KernelShape): ArrayBuffer {
    const { base64 } = request<{ base64: string }>({ op: "stl", handle: handle(shape).proof.handle, linearDeflection: 0.5, angularDeflection: 0.2 });
    return Uint8Array.from(atob(base64), character => character.charCodeAt(0)).buffer;
  }
  disposeShape(shape: KernelShape) { request({ op: "dispose", handle: handle(shape).proof.handle }); }
  // Unproved reference queries reject or return no eligible references.
  validatePlanarFace(shape: KernelShape, plane: SketchPlaneTransform): void {
    request({ op: "planarFace", handle: handle(shape).proof.handle, origin: xyz(plane.origin), normal: xyz(plane.normal) });
  }
  validatePocketFloor?: KernelAdapter["validatePocketFloor"];
  measureHoleRim?: KernelAdapter["measureHoleRim"];
  private capEdge(shape: KernelShape, ref: TopologyRef): { edgeStart: number[]; edgeEnd: number[]; normal: number[] } {
    const source = handle(shape).extrusion;
    if (!source || ref.kind !== "edge" || ref.repairRequired || !ref.sourceEntityId ||
        (ref.role !== "startCapPerimeter" && ref.role !== "endCapPerimeter"))
      throw new Error("Edge treatment requires one authored straight cap edge on its original extrusion.");
    const segments = source.profile.outerLoop.type === "polygon" ? source.profile.outerLoop.segments ?? [] : [];
    const matches = segments.filter(segment => segment.id === ref.sourceEntityId);
    if (matches.length !== 1 || matches[0].type !== "line")
      throw new Error("Authored cap edge is missing, ambiguous, or curved; repair the edge reference.");
    const segment = matches[0];
    const height = ref.role === "startCapPerimeter" ? 0 : source.distance;
    return {
      edgeStart: xyz(transformPoint(source.transform, segment.start.x, segment.start.y, height)),
      edgeEnd: xyz(transformPoint(source.transform, segment.end.x, segment.end.y, height)),
      normal: xyz(source.transform.normal).map(value => value * Math.sign(source.distance)),
    };
  }
  availableExtrudeCapEdges(shape: KernelShape): ReturnType<NonNullable<KernelAdapter["availableExtrudeCapEdges"]>> {
    const source = handle(shape).extrusion;
    if (!source) return [];
    const loop = source.profile.outerLoop;
    if (loop.type !== "polygon") return [];
    const edges: ReturnType<NonNullable<KernelAdapter["availableExtrudeCapEdges"]>> = [];
    for (const role of ["endCapPerimeter", "startCapPerimeter"] as const) {
      for (const segment of loop.segments ?? []) {
        if (segment.type !== "line") continue;
        const ref: TopologyRef = { featureId: "proof", kind: "edge", transientId: "proof", role, sourceEntityId: segment.id };
        const { available } = request<{ available: boolean }>({
          op: "edgeAvailable", handle: handle(shape).proof.handle, ...this.capEdge(shape, ref),
        });
        if (available) edges.push({ role, sourceEntityId: segment.id });
      }
    }
    return edges;
  }
  edgeProofSignature(_shape: KernelShape): string | undefined { return undefined; }
  hasCommonVolume(base: KernelShape, tool: KernelShape): boolean {
    const a = request<Proof>({ op: "inspect", handle: handle(base).proof.handle });
    const b = request<Proof>({ op: "inspect", handle: handle(tool).proof.handle });
    const upper = Math.min(a.geometryAssertions.volume, b.geometryAssertions.volume);
    const tolerance = Math.max(1e-9, Math.abs(upper) * 1e-10);
    const { volume } = request<{ volume: number }>({ op: "overlapVolume", a: a.handle, b: b.handle });
    if (!Number.isFinite(volume) || volume < -tolerance || volume > upper + tolerance)
      throw new Error("Scope intersection produced inconsistent volume.");
    return volume > tolerance;
  }
  cutScope(targets: KernelShape[], tools: KernelShape[]): KernelShape[] {
    if (!targets.length || !tools.length)
      throw new Error("Hole scope requires target bodies and center tools.");
    const pending = new Set<KernelShape>();
    const hits = targets.map(() => tools.map(() => false));
    const reason = (error: unknown) => error instanceof Error ? error.message : String(error);
    try {
      // Every center is proved against the original scope, never an intermediate
      // result. A missed individual body is allowed; an unused center is not.
      for (const [toolIndex, tool] of tools.entries()) {
        let hit = false;
        try {
          for (const [targetIndex, target] of targets.entries()) {
            if (!this.hasCommonVolume(target, tool)) continue;
            const probe = this.cut(target, tool);
            pending.add(probe);
            this.disposeShape(probe);
            pending.delete(probe);
            hits[targetIndex][toolIndex] = true;
            hit = true;
          }
          if (!hit) throw new Error("Boolean cut removed no volume. Move the tool into the target bodies.");
        } catch (error) { throw new HoleScopeError("center", toolIndex, reason(error)); }
      }
      const outputs = targets.map((target, targetIndex) => {
        try {
          if (!hits[targetIndex].some(Boolean))
            throw new Error("Boolean cut removed no volume. Move a tool into the target body.");
          // Original-scope probes establish each center's authored intent.
          // Union the hit tools atomically so overlapping or redundant tools
          // need not remove additional material in an arbitrary sequential order.
          const before = request<Proof>({ op: "inspect", handle: handle(target).proof.handle });
          const result = request<Proof>({ op: "cutMany", a: before.handle, tools: tools.filter((_, index) => hits[targetIndex][index]).map(tool => handle(tool).proof.handle) });
          const output = this.retainExtrusion(owned(result, "cut", target.id), target);
          pending.add(output);
          const after = result.geometryAssertions;
          const tolerance = Math.max(1e-9, Math.abs(before.geometryAssertions.volume) * 1e-10);
          if (!after.valid || !Number.isFinite(after.volume) || after.volume <= 1e-12 || after.solidCount < 1)
            throw new Error("Boolean operation produced no solid volume.");
          if (before.geometryAssertions.volume - after.volume <= tolerance)
            throw new Error("Boolean cut removed no volume. Move the tools into the target body.");
          return output;
        } catch (error) { throw new HoleScopeError("target", targetIndex, reason(error)); }
      });
      for (const output of outputs) pending.delete(output);
      return outputs;
    } finally { for (const shape of pending) this.disposeShape(shape); }
  }
  joinAll: NonNullable<KernelAdapter["joinAll"]> = unsupported;
  nativeDrawingGeometry?: KernelAdapter["nativeDrawingGeometry"];
  getWasmHeapCapacityBytes() { return undefined; }
  revolveProfile = unsupported;
  extrudeToFace = unsupported;
  private treatEdge(shape: KernelShape, refs: TopologyRef[], size: number, operation: "fillet" | "chamfer"): KernelShape {
    try {
      if (!refs.length || !Number.isFinite(size) || size <= 0)
        throw new Error("Edge treatment requires authored cap edges and a positive finite size.");
      if (refs.some(ref => ref.featureId !== refs[0].featureId))
        throw new Error("Edge treatment references must share one source feature.");
      // Resolve all authored references before publishing any native result.
      // The bridge validates the complete batch against the original solid.
      const source = handle(shape).extrusion;
      for (const ref of refs) {
        if (ref.sourceEntityId) continue;
        if (!source || source.profile.innerLoops.length || source.profile.outerLoop.type !== "polygon")
          throw new Error("This complete perimeter is unsupported; select individual authored cap edges.");
        const segments = source.profile.outerLoop.segments ?? [];
        if (!segments.length || segments.some(segment => segment.type !== "line"))
          throw new Error("This complete perimeter is unsupported; select individual authored straight cap edges.");
        const available = this.availableExtrudeCapEdges(shape);
        if (segments.some(segment => !available.some(edge => edge.role === ref.role && edge.sourceEntityId === segment.id)))
          throw new Error("The complete perimeter contains an edge that was lost or is ambiguous. Select a surviving authored edge.");
        throw new Error("A complete nonparallel perimeter is unsupported; select individual parallel authored cap edges.");
      }
      const edges = refs.map(ref => this.capEdge(shape, ref));
      const before = request<Proof>({ op: "inspect", handle: handle(shape).proof.handle });
      const result = request<Proof>({
        op: operation, handle: before.handle, normal: edges[0].normal,
        edges: edges.map(edge => ({ start: edge.edgeStart, end: edge.edgeEnd })),
        [operation === "fillet" ? "radius" : "distance"]: size,
      });
      try {
        const after = result.geometryAssertions;
        const tolerance = Math.max(1e-9, before.geometryAssertions.volume * 1e-10);
        if (!after.valid || !Number.isFinite(after.volume) || after.volume <= 0 ||
            after.solidCount !== before.geometryAssertions.solidCount ||
            before.geometryAssertions.volume - after.volume <= tolerance)
          throw new Error("Edge treatment did not retain valid changed solid geometry.");
        return this.retainExtrusion(owned(result, operation, shape.id), shape);
      } catch (error) {
        request({ op: "dispose", handle: result.handle });
        throw error;
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      throw new Error(message.includes(`${operation} failed`) ? message : `${operation} failed: ${message}`);
    }
  }
  fillet(shape: KernelShape, refs: TopologyRef[], radius: number): KernelShape { return this.treatEdge(shape, refs, radius, "fillet"); }
  chamfer(shape: KernelShape, refs: TopologyRef[], distance: number): KernelShape { return this.treatEdge(shape, refs, distance, "chamfer"); }
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
