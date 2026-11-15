// @vitest-environment node
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { expect, test } from "vitest";
import { OpenCascadeKernel, type StepImportOptions, type StepRepairEvidence } from "../cad/kernel/OpenCascadeKernel";
import type { BoundingBox, KernelAdapter } from "../cad/kernel/KernelAdapter";

interface Fixture {
  file: string;
  family: StepRepairEvidence["family"];
  bounds: BoundingBox;
  measures: readonly [number, number];
  sites: number;
  owners: number;
  prefix?: string;
  pose?: StepRepairEvidence["worldPose"];
}

const fixtures: readonly Fixture[] = [
  {
    file: "occt-default-two-fillets-minus-x",
    family: "occt-posed-grouped-pcurve-convex-arc",
    bounds: { min: [-3, -6.375, 0], max: [2, -3, 3] },
    measures: [29.077978807115535, 65.622954970084379], sites: 6, owners: 28,
  },
  {
    file: "occt-default-tilted-translated-oblique-prism-fillet",
    family: "occt-rational-posed-single-pcurve-convex-arc",
    bounds: { min: [1, 0, -.7], max: [7.025, 3, 5] },
    measures: [29.145041881053842, 66.186284791166074], sites: 3, owners: 24,
    prefix: "plainsolid-step-rational-posed-pcurve-convex-arc-source",
    pose: [["0", "3/5", "4/5", "8/5"], ["-1", "0", "0", "0"], ["0", "-4/5", "3/5", "6/5"]],
  },
  {
    file: "occt-both-default-tilted-oblique-prism-fillet",
    family: "occt-rational-posed-grouped-pcurve-convex-arc",
    bounds: { min: [0, 0, -2.7], max: [6.025, 3, 3] },
    measures: [29.077978807115539, 65.622954970084379], sites: 8, owners: 28,
    prefix: "plainsolid-step-rational-posed-grouped-pcurve-convex-arc-source",
    pose: [["0", "3/5", "4/5", "0"], ["-1", "0", "0", "0"], ["0", "-4/5", "3/5", "0"]],
  },
];

for (const fixture of fixtures) {
  test(`real Go WASM STEP repair through the typed adapter: ${fixture.file}`, async () => {
    const source = await readFile(new URL(`./plainsolid-step-repair/${fixture.file}.step`, import.meta.url), "utf8");
    const kernel = new OpenCascadeKernel();
    const strictAdapter: KernelAdapter = kernel;
    expect(() => strictAdapter.importStep!(source)).toThrow(/PlainSolid UNSUPPORTED/);
    const options: StepImportOptions = { stepRepair: { maxDistance: 1e-12, native: !!fixture.prefix } };
    const shape = kernel.importStep(source, options);
    try {
      const evidence = kernel.nativeStepRepairEvidence(shape)!;
      const digest = createHash("sha256").update(source).digest("hex");
      expect(evidence.repairEvidence).toMatchObject({
        family: fixture.family, sourceSHA256: digest, lengthUnit: .001, maxDistance: 1e-12, pcurveOwners: fixture.owners,
      });
      expect(evidence.repairEvidence.provenance.Body).toBeGreaterThan(0);
      expect(evidence.repairEvidence.repairs).toHaveLength(fixture.sites);
      expect(evidence.repairEvidence.worldPose).toEqual(fixture.pose);
      if (fixture.prefix) {
        const json = evidence.nativeProofJSON!;
        expect(json.endsWith("\n")).toBe(true);
        const proof = JSON.parse(json);
        expect(proof).toMatchObject({ Format: fixture.prefix, Version: 1, MaxDistance: 1e-12,
          Claims: { SourceSHA256: digest, Pose: fixture.pose } });
        expect(Buffer.from(proof.Source, "base64").toString("utf8")).toBe(source);
      } else {
        expect(evidence.nativeProofJSON).toBeUndefined();
      }
      const clone = kernel.cloneShape(shape);
      try { expect(kernel.nativeStepRepairEvidence(clone)).toEqual(evidence); }
      finally { kernel.disposeShape(clone); }

      const bounds = kernel.nativeBounds(shape);
      for (const side of ["min", "max"] as const) {
        bounds[side].forEach((value, axis) => expect(value).toBeCloseTo(fixture.bounds[side][axis], 11));
      }
      const mesh = kernel.tessellate(shape, { linearDeflection: .025, angularDeflection: .2, analyticNormals: true });
      expect(mesh).toMatchObject({ geometrySource: "plainsolid", kernelOperation: "stepImport", bounds });
      expect(mesh.indices.length).toBeGreaterThan(0);
      expect(mesh.geometryAssertions).toMatchObject({ valid: true, solidCount: 1 });
      expect(mesh.geometryAssertions!.volume).toBeCloseTo(fixture.measures[0], 10);
      expect(mesh.geometryAssertions!.surfaceArea).toBeCloseTo(fixture.measures[1], 10);
      expect(Array.from(mesh.positions).every((value, i) => value >= bounds.min[i % 3] - 1e-12
        && value <= bounds.max[i % 3] + 1e-12)).toBe(true);

      const exported = kernel.exportStep([shape]);
      expect(exported.units).toBe("mm");
      expect(exported.before).toEqual([{ ...mesh.geometryAssertions, bounds }]);
      expect(exported.after).toEqual(exported.before);
      const strict = strictAdapter.importStep!(new TextDecoder().decode(exported.bytes));
      try {
        expect(kernel.nativeStepRepairEvidence(strict)).toBeUndefined();
        expect(kernel.nativeBounds(strict)).toEqual(bounds);
        expect(Buffer.from(kernel.exportStep([strict]).bytes)).toEqual(Buffer.from(exported.bytes));
      } finally { kernel.disposeShape(strict); }
    } finally { kernel.disposeShape(shape); }
  }, 60_000);
}
