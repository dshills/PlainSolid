// @vitest-environment node
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { expect, test } from "vitest";
import { OpenCascadeKernel, type StepImportOptions, type StepRepairEvidence, type StepSourceUnit } from "../cad/kernel/OpenCascadeKernel";
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
  units?: StepSourceUnit;
  mmPerUnit?: number;
  metresPerUnit?: number;
  grouped?: true;
  converted?: true;
}

const tiltedSingle: Fixture = {
  file: "occt-default-tilted-translated-oblique-prism-fillet",
  family: "occt-rational-posed-single-pcurve-convex-arc",
  bounds: { min: [1, 0, -.7], max: [7.025, 3, 5] },
  measures: [29.145041881053842, 66.186284791166074], sites: 3, owners: 24,
  prefix: "plainsolid-step-rational-posed-pcurve-convex-arc-source",
  pose: [["0", "3/5", "4/5", "8/5"], ["-1", "0", "0", "0"], ["0", "-4/5", "3/5", "6/5"]],
};
const tiltedGrouped: Fixture = {
  file: "occt-both-default-tilted-oblique-prism-fillet",
  family: "occt-rational-posed-grouped-pcurve-convex-arc",
  bounds: { min: [0, 0, -2.7], max: [6.025, 3, 3] },
  measures: [29.077978807115539, 65.622954970084379], sites: 8, owners: 28,
  prefix: "plainsolid-step-rational-posed-grouped-pcurve-convex-arc-source",
  pose: [["0", "3/5", "4/5", "0"], ["-1", "0", "0", "0"], ["0", "-4/5", "3/5", "0"]],
  grouped: true,
};
const fixtures: readonly Fixture[] = [
  {
    file: "occt-default-two-fillets-minus-x",
    family: "occt-posed-grouped-pcurve-convex-arc",
    bounds: { min: [-3, -6.375, 0], max: [2, -3, 3] },
    measures: [29.077978807115535, 65.622954970084379], sites: 6, owners: 28,
  },
  tiltedSingle,
  tiltedGrouped,
  { ...tiltedSingle, file: `${tiltedSingle.file}-metre`, units: "m", mmPerUnit: 1000, metresPerUnit: 1 },
  { ...tiltedSingle, file: `${tiltedSingle.file}-inch`, units: "in", mmPerUnit: 25.4, metresPerUnit: .0254 },
  { ...tiltedGrouped, file: `${tiltedGrouped.file}-metre`, units: "m", mmPerUnit: 1000, metresPerUnit: 1 },
  { ...tiltedGrouped, file: `${tiltedGrouped.file}-inch`, units: "in", mmPerUnit: 25.4, metresPerUnit: .0254 },
  { ...tiltedSingle, file: `${tiltedSingle.file}-converted-metre`, units: "m", mmPerUnit: 1000, metresPerUnit: 1, converted: true },
];

for (const fixture of fixtures) {
  test(`real Go WASM STEP repair through the typed adapter: ${fixture.file}`, async () => {
    const source = await readFile(new URL(`./plainsolid-step-repair/${fixture.file}.step`, import.meta.url), "utf8");
    const kernel = new OpenCascadeKernel();
    const strictAdapter: KernelAdapter = kernel;
    expect(() => strictAdapter.importStep!(source)).toThrow(/PlainSolid UNSUPPORTED/);
    const units = fixture.units;
    const scale = fixture.mmPerUnit ?? 1;
    const options: StepImportOptions = { stepRepair: { maxDistance: 1e-12, native: !!fixture.prefix }, units };
    if (units) {
      expect(() => kernel.importStep(source, { units })).toThrow(/PlainSolid UNSUPPORTED/);
      expect(() => kernel.importStep(source, { stepRepair: options.stepRepair })).toThrow(/STEP import unit must match/);
    }
    const shape = kernel.importStep(source, options);
    try {
      expect(kernel.nativeStepSourceUnit(shape)).toBe(units);
      const evidence = kernel.nativeStepRepairEvidence(shape)!;
      const digest = createHash("sha256").update(source).digest("hex");
      expect(evidence.repairEvidence).toMatchObject({
        family: fixture.family, sourceSHA256: digest, lengthUnit: fixture.metresPerUnit ?? .001, maxDistance: 1e-12, pcurveOwners: fixture.owners,
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
        if (fixture.grouped) {
          expect(evidence.repairEvidence).toMatchObject({ finiteLineOwners: 8, rationalCaps: 2, axialTrims: 1 });
          expect(proof.Claims.FiniteLineOwners).toHaveLength(8);
          expect(proof.Claims.Grouped.RationalCaps).toHaveLength(2);
          expect(proof.Claims.Grouped.FiniteTrims).toHaveLength(2);
          expect(proof.Claims.Grouped.AxialTrims).toHaveLength(1);
        }
        if (fixture.converted) {
          expect(evidence.repairEvidence.finiteLineOwners).toBe(18);
          expect(proof.Claims.FiniteLineOwners).toHaveLength(18);
          for (const owner of proof.Claims.FiniteLineOwners) expect(owner.PCurveParameterScale).toBe("1/1000");
          const spans = proof.Claims.CylindricalSpanOwners;
          const sweep = "2498091544797/1000000000000";
          expect(spans).toHaveLength(4);
          expect(spans).toEqual(expect.arrayContaining([
            { PCurveID: 125, ParameterInterval: ["0", "5000"], UVStart: ["0", "0"], UVDerivative: ["0", "1/1000"] },
            { PCurveID: 225, ParameterInterval: ["0", "5000"], UVStart: [sweep, "0"], UVDerivative: ["0", "1/1000"] },
            { PCurveID: 157, ParameterInterval: ["0", sweep], UVStart: ["0", "0"], UVDerivative: ["1", "0"] },
            { PCurveID: 246, ParameterInterval: ["0", sweep], UVStart: ["0", "5"], UVDerivative: ["1", "0"] },
          ]));
        }
      } else {
        expect(evidence.nativeProofJSON).toBeUndefined();
      }
      const clone = kernel.cloneShape(shape);
      try { expect(kernel.nativeStepRepairEvidence(clone)).toEqual(evidence); }
      finally { kernel.disposeShape(clone); }

      const bounds = kernel.nativeBounds(shape);
      for (const side of ["min", "max"] as const) {
        bounds[side].forEach((value, axis) => expect(value / scale).toBeCloseTo(fixture.bounds[side][axis], 11));
      }
      const mesh = kernel.tessellate(shape, { linearDeflection: fixture.converted ? .5 : .025 * scale, angularDeflection: .2, analyticNormals: true });
      expect(mesh).toMatchObject({ geometrySource: "plainsolid", kernelOperation: "stepImport", bounds });
      expect(mesh.indices.length).toBeGreaterThan(0);
      expect(mesh.geometryAssertions).toMatchObject({ valid: true, solidCount: 1 });
      expect(mesh.geometryAssertions!.volume / scale ** 3).toBeCloseTo(fixture.measures[0], 10);
      expect(mesh.geometryAssertions!.surfaceArea / scale ** 2).toBeCloseTo(fixture.measures[1], 10);
      expect(Array.from(mesh.positions).every((value, i) => value >= bounds.min[i % 3] - 1e-12 * scale
        && value <= bounds.max[i % 3] + 1e-12 * scale)).toBe(true);

      if (units) {
        expect(() => strictAdapter.exportStep!([shape])).toThrow(/STEP export unit must match/);
        expect(() => kernel.exportStep([shape], { units: units === "m" ? "in" : "m" })).toThrow(/STEP export unit must match/);
      }
      const exported = units ? kernel.exportStep([shape], { units }) : strictAdapter.exportStep!([shape]);
      expect(exported.units).toBe(units ?? "mm");
      if (units) expect(exported).toMatchObject({ proofUnits: "mm" });
      expect(exported.before).toEqual([{ ...mesh.geometryAssertions, bounds }]);
      expect(exported.after).toEqual(exported.before);
      const canonical = new TextDecoder().decode(exported.bytes);
      if (units === "m") expect(canonical).toContain("SI_UNIT($,.METRE.)");
      if (units) expect(() => strictAdapter.importStep!(canonical)).toThrow(/STEP import unit must match/);
      const strict = units ? kernel.importStep(canonical, { units }) : strictAdapter.importStep!(canonical);
      try {
        expect(kernel.nativeStepRepairEvidence(strict)).toBeUndefined();
        expect(kernel.nativeStepSourceUnit(strict)).toBe(units);
        expect(kernel.nativeBounds(strict)).toEqual(bounds);
        const again = units ? kernel.exportStep([strict], { units }) : strictAdapter.exportStep!([strict]);
        expect(Buffer.from(again.bytes)).toEqual(Buffer.from(exported.bytes));
      } finally { kernel.disposeShape(strict); }
    } finally { kernel.disposeShape(shape); }
  }, 60_000);
}
