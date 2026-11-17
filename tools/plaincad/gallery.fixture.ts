// @vitest-environment node
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { beforeAll, expect, it } from "vitest";
import type { CadDocument } from "../cad/document/schema";
import { clearNativeFeatureCache, rebuildDocument } from "../cad/features/rebuildGraph";
import type { BoundingBox, RenderMesh } from "../cad/kernel/KernelAdapter";
import { NativeKernel } from "../cad/kernel/NativeKernel";
import type { RebuildWarning } from "../cad/worker/workerProtocol";
import { loadProject } from "../persistence/projectRepository";

interface ReferenceBody {
  bodyId: string;
  component: string;
  volume: number;
  bounds: BoundingBox;
}

interface ReferenceProject {
  order: number;
  slug: string;
  parts: number;
  features: number;
  parameter: string;
  original: number;
  edited: number;
  projectFileSha256: string;
  native: { totalVolumeMm3: number; bodies: ReferenceBody[]; warnings: RebuildWarning[] };
  parameterEdit: {
    from: number;
    to: number;
    totalVolumeMm3: number;
    changedBodies: { component: string; editedVolumeMm3: number; editedBounds: BoundingBox }[];
  };
}

const references = JSON.parse(readFileSync("docs/examples/VALIDATION.json", "utf8")) as { projects: ReferenceProject[] };

function editParameter(document: CadDocument, name: string, before: number, after: number) {
  const parameter = document.parameters[name];
  if (parameter?.expression !== `${before}mm`) throw new Error(`Unexpected original ${name} expression`);
  document.parameters[name] = { ...parameter, expression: `${after}mm`, value: after };
}

function editFeature(document: CadDocument, name: string, change: (feature: CadDocument["features"][number]) => void) {
  const matches = document.features.filter(feature => feature.name === name);
  if (matches.length !== 1) throw new Error(`Expected exactly one authored feature named ${name}`);
  change(matches[0]);
}

const supplemental = [
  { slug: "11-trapezoidal-lead-screw", parts: 2, features: 2, volumes: [744.073, 628.451], variant: "screw length 18 to 20 mm",
    edit: (doc: CadDocument) => editFeature(doc, "Lead screw", feature => {
      if (feature.type !== "leadScrew" || feature.screwLength.expression !== "18mm") throw new Error("Unexpected original screw length");
      feature.screwLength = { ...feature.screwLength, expression: "20mm" };
    }) },
  { slug: "12-acme-lead-screw", parts: 2, features: 2, volumes: [1301.638, 790.977], variant: "left to right hand",
    edit: (doc: CadDocument) => editFeature(doc, "Lead screw", feature => {
      if (feature.type !== "leadScrew" || feature.hand !== "left") throw new Error("Unexpected original screw handedness");
      feature.hand = "right";
    }) },
  { slug: "13-m6-threaded-pair", parts: 2, features: 5, volumes: [750.991, 630.756], variant: "external thread length 8 to 7 mm",
    edit: (doc: CadDocument) => editFeature(doc, "External thread", feature => {
      if (feature.type !== "thread" || feature.threadLength.expression !== "8mm") throw new Error("Unexpected external thread length");
      feature.threadLength = { ...feature.threadLength, expression: "7mm" };
    }) },
  { slug: "14-quarter-inch-unc", parts: 2, features: 5, volumes: [814.297, 1201.099], variant: "internal thread length 0.25 to 0.2 in",
    edit: (doc: CadDocument) => editFeature(doc, "Internal thread", feature => {
      if (feature.type !== "thread" || feature.threadLength.expression !== "0.25in") throw new Error("Unexpected internal thread length");
      feature.threadLength = { ...feature.threadLength, expression: "0.2in" };
    }) },
  { slug: "orbit-drive-housing", parts: 1, features: 16, volumes: undefined, variant: "bearing bore diameter 40 to 42 mm",
    edit: (doc: CadDocument) => editParameter(doc, "bore_diameter", 40, 42) },
];

// The unchanged source sketches produce these recorded diagnostics.
// Compare each original diagnostic and reject any additional warning.
const sketchDofs: Record<string, [string, number][]> = {
  "13-m6-threaded-pair": [["sketch_3cc07dd9", 12], ["sketch_cf08ee45", 12], ["sketch_f7fed670", 6]],
  "14-quarter-inch-unc": [["sketch_8b05ef68", 12], ["sketch_2271f0f2", 12], ["sketch_357bb553", 7]],
};

function authoredWarnings(slug: string): RebuildWarning[] {
  return (sketchDofs[slug] ?? []).map(([sourceId, dof]) => ({
    id: `sketch:${sourceId}:dof`, source: "sketch", sourceId,
    message: `Underconstrained sketch: ${dof} degrees of freedom remain.`,
  }));
}

const projects = [
  ...references.projects.map(reference => ({
    slug: reference.slug, parts: reference.parts, features: reference.features, reference, volumes: undefined,
    variant: `${reference.parameter} ${reference.original} to ${reference.edited} mm`,
    edit: (doc: CadDocument) => editParameter(doc, reference.parameter, reference.original, reference.edited),
  })),
  ...supplemental.map(project => ({ ...project, reference: undefined })),
];

const requested = process.env.PLAINSOLID_GALLERY?.split(",").map(token => token.trim()).filter(Boolean);
if (!requested?.length) throw new Error("Set PLAINSOLID_GALLERY to comma-separated project numbers or names.");
const selected = new Set(requested.map(token => {
  const project = projects.find(({ slug }) => slug === token || slug.split("-")[0] === token.padStart(2, "0"));
  if (!project) throw new Error(`Unknown gallery fixture ${token}`);
  return project.slug;
}));

beforeAll(async () => {
  const fetchRemote = globalThis.fetch;
  globalThis.fetch = async (input, init) => {
    if (!String(input).endsWith("/plainsolid-kernel.wasm")) return fetchRemote(input, init);
    const bytes = await readFile("src/cad/kernel/plainsolid-kernel.wasm");
    return new Response(bytes, { headers: { "Content-Type": "application/wasm" } });
  };
  try {
    expect(NativeKernel.source).toBe("plainsolid");
    await NativeKernel.initialize();
  } finally {
    globalThis.fetch = fetchRemote;
  }
}, 30_000);

const volumeTolerance = (expected: number) => Math.max(.001, expected * 1e-7);
const boundsTolerance = .5; // The OCCT gallery record contains default display mesh bounds.

function bodyMetrics(mesh: RenderMesh) {
  const hash = createHash("sha256");
  hash.update(Buffer.from(Float64Array.from(mesh.positions).buffer));
  hash.update(Buffer.from(Uint32Array.from(mesh.indices).buffer));
  return {
    bodyId: mesh.bodyId, provider: mesh.geometrySource, operation: mesh.kernelOperation,
    assertions: mesh.geometryAssertions, bounds: mesh.bounds, triangles: mesh.indices.length / 3,
    meshSha256: hash.digest("hex"),
  };
}

for (const project of projects.filter(project => selected.has(project.slug))) {
  let originalBodies: ReturnType<typeof bodyMetrics>[] | undefined;
  for (const edited of [false, true]) {
    const variant = edited ? project.variant : "original";
    it(`${project.slug} ${variant}`, async () => {
      const raw = await readFile(`docs/examples/${project.slug}.pcaddoc`);
      const sourceSHA256 = createHash("sha256").update(raw).digest("hex");
      const reference = project.reference;
      if (reference) expect(sourceSHA256).toBe(reference.projectFileSha256);
      const expectedWarnings = reference?.native.warnings ?? authoredWarnings(project.slug);
      const doc = loadProject(raw.toString("utf8"));
      expect(doc.features).toHaveLength(project.features);
      if (edited) project.edit(doc);
      const authored = structuredClone(doc);
      clearNativeFeatureCache();
      const model = rebuildDocument(doc, { includeDisplayEdges: false, reuseNativeFeatures: false });
      const bodies = model.meshes.map(bodyMetrics);
      const totalVolumeMm3 = bodies.reduce((sum, mesh) => sum + (mesh.assertions?.volume ?? 0), 0);
      const expectedTotal = edited ? reference?.parameterEdit.totalVolumeMm3 : reference?.native.totalVolumeMm3;
      const comparison = reference?.native.bodies.map(body => {
        const actual = bodies.find(mesh => mesh.bodyId === body.bodyId);
        const edit = edited ? reference.parameterEdit.changedBodies.find(change => change.component === body.component) : undefined;
        const expected = { volume: edit?.editedVolumeMm3 ?? body.volume, bounds: edit?.editedBounds ?? body.bounds };
        const volumeDifference = actual?.assertions ? actual.assertions.volume - expected.volume : undefined;
        const boundsDifference = actual ? Math.max(...(["min", "max"] as const)
          .flatMap(end => expected.bounds[end].map((x, axis) => Math.abs(x - actual.bounds[end][axis])))) : undefined;
        return { bodyId: body.bodyId, component: body.component, expected, volumeDifference, boundsDifference };
      });
      console.log("gallery-proof", JSON.stringify({
        slug: project.slug, variant, sourceSHA256, status: model.success, errors: model.errors, warnings: model.warnings, expectedWarnings,
        durationMs: model.durationMs, operations: model.metrics?.operationCount, bodies, totalVolumeMm3, expectedTotal, comparison,
      }));
      expect(doc).toEqual(authored);
      expect(model.errors).toEqual([]);
      expect(model.warnings).toEqual(expectedWarnings);
      expect(model.success).toBe(true);
      expect(model.bodies).toHaveLength(project.parts);
      expect(bodies).toHaveLength(project.parts);
      expect(new Set(bodies.map(body => body.bodyId)).size).toBe(project.parts);
      expect(model.metrics?.operationCount).toBe(project.features);
      for (const mesh of bodies) {
        expect(mesh.provider).toBe("plainsolid");
        expect(mesh.assertions).toMatchObject({ valid: true, solidCount: 1 });
        const { volume, surfaceArea } = mesh.assertions!;
        expect(Number.isFinite(volume) && volume > 0).toBe(true);
        expect(Number.isFinite(surfaceArea) && surfaceArea > 0).toBe(true);
        expect(mesh.triangles).toBeGreaterThan(0);
        expect([...mesh.bounds.min, ...mesh.bounds.max].every(Number.isFinite)).toBe(true);
        expect(mesh.bounds.min.every((lower, axis) => lower <= mesh.bounds.max[axis])).toBe(true);
      }
      if (comparison && expectedTotal !== undefined) {
        expect(Math.abs(totalVolumeMm3 - expectedTotal)).toBeLessThanOrEqual(volumeTolerance(expectedTotal));
        for (const measured of comparison) {
          expect(measured.volumeDifference, measured.component).toBeDefined();
          expect(Math.abs(measured.volumeDifference!), measured.component).toBeLessThanOrEqual(volumeTolerance(measured.expected.volume));
          expect(measured.boundsDifference, measured.component).toBeLessThanOrEqual(boundsTolerance);
        }
      }
      const volumes = project.volumes;
      if (!edited && volumes) {
        bodies.forEach((body, index) => expect(body.assertions?.volume).toBeCloseTo(volumes[index], 2));
      }
      if (edited) {
        if (!originalBodies) throw new Error("The original rebuild did not qualify.");
        const authoredBodies = originalBodies;
        expect(bodies.map(body => body.bodyId)).toEqual(authoredBodies.map(body => body.bodyId));
        expect(bodies.some((body, index) => body.meshSha256 !== authoredBodies[index].meshSha256)).toBe(true);
      } else {
        originalBodies = bodies;
      }
    }, 90_000);
  }
}
