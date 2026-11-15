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
  sourceScale?: number;
  maxDistance?: number;
  rejectedDistance?: number;
  grouped?: true;
  fillets?: 3 | 4;
  converted?: { ownerScale: string; axialEnd: string; height: string };
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
const tiltedThree: Fixture = {
  ...tiltedGrouped, file: "occt-three-default-tilted-oblique-prism-fillet", fillets: 3, rejectedDistance: 1e-13,
  bounds: { min: [0, 0, -2.7], max: [6.025, 2.84375, 3] },
  measures: [28.968988172080639, 64.184278587623695], sites: 10, owners: 34,
};
const tiltedFour: Fixture = {
  ...tiltedGrouped, file: "occt-four-default-tilted-trapezoidal-prism-fillet", fillets: 4, rejectedDistance: 1e-13,
  bounds: { min: [0, 0, -1.6], max: [5.2, 2.84375, 3] },
  measures: [22.155304183748054, 51.012178844414059], sites: 10, owners: 46,
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
  { ...tiltedSingle, file: `${tiltedSingle.file}-converted-metre`, units: "m", mmPerUnit: 1000, metresPerUnit: 1,
    converted: { ownerScale: "1/1000", axialEnd: "5000", height: "5" } },
  { ...tiltedSingle, file: `${tiltedSingle.file}-converted-inch`, units: "in", mmPerUnit: 25.4, metresPerUnit: .0254,
    sourceScale: 5, maxDistance: 1e-11, rejectedDistance: 1e-12,
    pose: [["0", "3/5", "4/5", "8"], ["-1", "0", "0", "0"], ["0", "-4/5", "3/5", "6"]],
    converted: { ownerScale: "5/127", axialEnd: "635", height: "25" } },
  { ...tiltedGrouped, file: `${tiltedGrouped.file}-converted-metre`, units: "m", mmPerUnit: 1000, metresPerUnit: 1,
    rejectedDistance: 1e-13, converted: { ownerScale: "1/1000", axialEnd: "5000", height: "5" } },
  { ...tiltedGrouped, file: `${tiltedGrouped.file}-converted-inch`, units: "in", mmPerUnit: 25.4, metresPerUnit: .0254,
    sourceScale: 5, maxDistance: 1e-11, rejectedDistance: 1e-12,
    converted: { ownerScale: "5/127", axialEnd: "635", height: "25" } },
  tiltedThree,
  { ...tiltedThree, file: `${tiltedThree.file}-converted-metre`, units: "m", mmPerUnit: 1000, metresPerUnit: 1,
    converted: { ownerScale: "1/1000", axialEnd: "5000", height: "5" } },
  { ...tiltedThree, file: `${tiltedThree.file}-converted-inch`, units: "in", mmPerUnit: 25.4, metresPerUnit: .0254,
    sourceScale: 5, maxDistance: 1e-11, rejectedDistance: 1e-12,
    converted: { ownerScale: "5/127", axialEnd: "635", height: "25" } },
  tiltedFour,
  { ...tiltedFour, file: `${tiltedFour.file}-converted-metre`, units: "m", mmPerUnit: 1000, metresPerUnit: 1,
    converted: { ownerScale: "1/1000", axialEnd: "5000", height: "5" } },
];

for (const fixture of fixtures) {
  test(`real Go WASM STEP repair through the typed adapter: ${fixture.file}`, async () => {
    const source = await readFile(new URL(`./plainsolid-step-repair/${fixture.file}.step`, import.meta.url), "utf8");
    const kernel = new OpenCascadeKernel();
    const strictAdapter: KernelAdapter = kernel;
    expect(() => strictAdapter.importStep!(source)).toThrow(/PlainSolid UNSUPPORTED/);
    const units = fixture.units;
    const scale = fixture.mmPerUnit ?? 1;
    const millimetresPerBase = scale * (fixture.sourceScale ?? 1);
    const maxDistance = fixture.maxDistance ?? 1e-12;
    const options: StepImportOptions = { stepRepair: { maxDistance, native: !!fixture.prefix }, units };
    if (units) {
      expect(() => kernel.importStep(source, { units })).toThrow(/PlainSolid UNSUPPORTED/);
      expect(() => kernel.importStep(source, { stepRepair: options.stepRepair })).toThrow(/STEP import unit must match/);
      expect(() => kernel.importStep(source, { ...options, units: units === "m" ? "in" : "m" })).toThrow(/STEP import unit must match/);
    }
    const rejectedDistance = fixture.rejectedDistance;
    if (rejectedDistance !== undefined) {
      expect(() => kernel.disposeShape(kernel.importStep(source, { units, stepRepair: { maxDistance: rejectedDistance, native: true } })))
        .toThrow(/PlainSolid UNSUPPORTED: STEP source has no qualified default OCCT pcurve repair within maxDistance/);
    }
    const shape = kernel.importStep(source, options);
    try {
      expect(kernel.nativeStepSourceUnit(shape)).toBe(units);
      const evidence = kernel.nativeStepRepairEvidence(shape)!;
      const digest = createHash("sha256").update(source).digest("hex");
      expect(evidence.repairEvidence).toMatchObject({
        family: fixture.family, sourceSHA256: digest, lengthUnit: fixture.metresPerUnit ?? .001, maxDistance, pcurveOwners: fixture.owners,
      });
      expect(evidence.repairEvidence.provenance.Body).toBeGreaterThan(0);
      expect(evidence.repairEvidence.repairs).toHaveLength(fixture.sites);
      expect(evidence.repairEvidence.worldPose).toEqual(fixture.pose);
      if (fixture.prefix) {
        const json = evidence.nativeProofJSON!;
        expect(json.endsWith("\n")).toBe(true);
        const proof = JSON.parse(json);
        expect(proof).toMatchObject({ Format: fixture.prefix, Version: 1, MaxDistance: maxDistance,
          Claims: { SourceSHA256: digest, Pose: fixture.pose, MaxDistance: maxDistance } });
        expect(Buffer.from(proof.Source, "base64").toString("utf8")).toBe(source);
        const finiteLineOwners = fixture.converted ? fixture.fillets === 4 ? 24 : 18 : 8;
        if (fixture.grouped) {
          const axialTrims = fixture.fillets ? 2 : 1;
          expect(evidence.repairEvidence).toMatchObject({ finiteLineOwners, rationalCaps: 2, axialTrims });
          expect(proof.Claims.FiniteLineOwners).toHaveLength(finiteLineOwners);
          expect(proof.Claims.Grouped.RationalCaps).toHaveLength(2);
          expect(proof.Claims.Grouped.FiniteTrims).toHaveLength(2);
          expect(proof.Claims.Grouped.AxialTrims).toHaveLength(axialTrims);
        }
        if (fixture.fillets && !fixture.converted) {
          const phases = fixture.fillets === 4 ? [210, 267, 482, 529] : [210, 425];
          expect(proof.Claims.PCurves.filter((owner: { Kind: string }) => owner.Kind === "phase-cylinder-circle")).toMatchObject(
            phases.map(PCurveID => ({ PCurveID, DistanceSquaredUpper: expect.any(String) })));
        }
        if (fixture.fillets === 4 && !fixture.converted) {
          expect(proof.Claims.Grouped.AxialTrims).toMatchObject([{ EdgeID: 338 }, { EdgeID: 563 }]);
          expect(proof.Claims.Grouped.FiniteTrims).toMatchObject([
            { CapOwner: { PCurveID: 315 }, CylinderOwner: { PCurveID: 326 }, EndStation: expect.any(Object) },
            { CapOwner: { PCurveID: 375 }, CylinderOwner: { PCurveID: 368 }, EndStation: expect.any(Object) },
          ]);
        }
        if (fixture.converted) {
          const { ownerScale, axialEnd, height } = fixture.converted;
          expect(evidence.repairEvidence.finiteLineOwners).toBe(finiteLineOwners);
          expect(proof.Claims.FiniteLineOwners).toHaveLength(finiteLineOwners);
          for (const owner of proof.Claims.FiniteLineOwners) expect(owner.PCurveParameterScale).toBe(ownerScale);
          const spans = proof.Claims.CylindricalSpanOwners;
          if (fixture.fillets === 4) {
            const excess = "1/25000000000000000000000000";
            const projectedEnd = "125000000000000000000000000001/25000000000000000000000000";
            const shifted = spans.filter((span: { ModelAngleBounds?: unknown }) => span.ModelAngleBounds !== undefined);
            expect(shifted.map((span: { PCurveID: number }) => span.PCurveID)).toEqual([207, 263, 472, 518]);
            expect(spans).toHaveLength(16);
            expect(spans.filter((span: { ClippedOwnerParameters?: unknown }) => span.ClippedOwnerParameters !== undefined)).toEqual([{
              PCurveID: 564, ParameterInterval: ["0", axialEnd], UVStart: ["553574358897/250000000000", "0"],
              UVDerivative: ["0", ownerScale], RepairedToOwner: [`-${excess}`, "1"], ClippedOwnerParameters: ["0", axialEnd],
            }]);
            expect(proof.Claims.Grouped.AxialTrims).toMatchObject([{ EdgeID: 332 }, {
              EdgeID: 551, OriginalParameters: ["0", projectedEnd], RepairedParameters: [excess, projectedEnd],
            }]);
            expect(proof.Claims.Grouped.FiniteTrims).toMatchObject([
              { CapOwner: { PCurveID: 310 }, CylinderOwner: { PCurveID: 321 }, EndStation: expect.any(Object) },
              { CapOwner: { PCurveID: 367 }, CylinderOwner: { PCurveID: 361 }, EndStation: expect.any(Object) },
            ]);
          } else if (fixture.fillets === 3) {
            expect(proof.Claims.PCurves.filter((owner: { Kind: string }) => owner.Kind === "finite-phase-cylinder-circle")).toMatchObject([
              { PCurveID: 207, DistanceSquaredUpper: expect.any(String) },
              { PCurveID: 416, DistanceSquaredUpper: expect.any(String) },
            ]);
            const shiftedInterval = ["2498091544797/1000000000000", "942477796077/200000000000"];
            const modelAngleBounds = [["624522886199/250000000000", "1249045772399/500000000000"],
              ["147262155637/31250000000", "2356194490193/500000000000"]];
            expect(spans).toHaveLength(12);
            expect(spans.filter((span: { ModelAngleBounds?: unknown }) => span.ModelAngleBounds !== undefined)).toEqual([
              { PCurveID: 207, ParameterInterval: shiftedInterval, UVStart: ["0", "0"], UVDerivative: ["1", "0"], ModelAngleBounds: modelAngleBounds },
              { PCurveID: 416, ParameterInterval: shiftedInterval, UVStart: ["0", height], UVDerivative: ["1", "0"], ModelAngleBounds: modelAngleBounds },
            ]);
            expect(spans.filter((span: { RepairedToOwner?: unknown }) => span.RepairedToOwner !== undefined)).toMatchObject(
              [284, 462].map(PCurveID => ({ PCurveID, ParameterInterval: ["0", axialEnd], UVDerivative: ["0", ownerScale],
                RepairedToOwner: [expect.any(String), expect.any(String)] })));
            expect(proof.Claims.Grouped.FiniteTrims).toMatchObject([
              { CapOwner: { PCurveID: 254 }, CylinderOwner: { PCurveID: 265 }, EndStation: expect.any(Object) },
              { CapOwner: { PCurveID: 311 }, CylinderOwner: { PCurveID: 305 }, EndStation: expect.any(Object) },
            ]);
          } else if (fixture.grouped) {
            const quarterTurn = "314159265359/200000000000";
            const repairedToOwner = units === "in" ? [
              "-625000000000000000000000000001/615157480315000000000000000000000000000000000000000000000",
              "625000000000000000000000000001/625000000000000000000000000000",
            ] : [
              "7499999999999999999999999999991/1250000000000000000000000000000000000000000000000000000000",
              "2499999999999999999999999999997/2500000000000000000000000000000",
            ];
            expect(spans).toHaveLength(8);
            expect(spans).toEqual(expect.arrayContaining([
              { PCurveID: 237, ParameterInterval: ["0", quarterTurn], UVStart: ["0", "0"], UVDerivative: ["-1", "0"] },
              { PCurveID: 277, ParameterInterval: ["0", quarterTurn], UVStart: ["0", height], UVDerivative: ["-1", "0"] },
              { PCurveID: 256, ParameterInterval: ["0", axialEnd], UVStart: [`-${quarterTurn}`, "0"], UVDerivative: ["0", ownerScale],
                RepairedToOwner: repairedToOwner },
            ]));
            expect(proof.Claims.Grouped.FiniteTrims).toMatchObject([
              { CapOwner: { PCurveID: 226 }, CylinderOwner: { PCurveID: 237 }, EndStation: expect.any(Object) },
              { CapOwner: { PCurveID: 283 }, CylinderOwner: { PCurveID: 277 }, EndStation: expect.any(Object) },
            ]);
          } else {
            const sweep = "2498091544797/1000000000000";
            expect(spans).toHaveLength(4);
            expect(spans).toEqual(expect.arrayContaining([
              { PCurveID: 125, ParameterInterval: ["0", axialEnd], UVStart: ["0", "0"], UVDerivative: ["0", ownerScale] },
              { PCurveID: 225, ParameterInterval: ["0", axialEnd], UVStart: [sweep, "0"], UVDerivative: ["0", ownerScale] },
              { PCurveID: 157, ParameterInterval: ["0", sweep], UVStart: ["0", "0"], UVDerivative: ["1", "0"] },
              { PCurveID: 246, ParameterInterval: ["0", sweep], UVStart: ["0", height], UVDerivative: ["1", "0"] },
            ]));
          }
          if (units === "in" && !fixture.grouped) expect(proof.Claims.PCurves).toContainEqual(expect.objectContaining({
            PCurveID: 125, Kind: "finite-cylinder-line", DistanceSquaredUpper: "625/244140625000031250000000001",
          }));
        }
      } else {
        expect(evidence.nativeProofJSON).toBeUndefined();
      }
      const clone = kernel.cloneShape(shape);
      try { expect(kernel.nativeStepRepairEvidence(clone)).toEqual(evidence); }
      finally { kernel.disposeShape(clone); }

      const bounds = kernel.nativeBounds(shape);
      for (const side of ["min", "max"] as const) {
        bounds[side].forEach((value, axis) => expect(value / millimetresPerBase).toBeCloseTo(fixture.bounds[side][axis], 11));
      }
      const mesh = kernel.tessellate(shape, { linearDeflection: fixture.converted || fixture.fillets ? .5 : .025 * scale, angularDeflection: .2, analyticNormals: true });
      expect(mesh).toMatchObject({ geometrySource: "plainsolid", kernelOperation: "stepImport", bounds });
      expect(mesh.indices.length).toBeGreaterThan(0);
      expect(mesh.geometryAssertions).toMatchObject({ valid: true, solidCount: 1 });
      expect(mesh.geometryAssertions!.volume / millimetresPerBase ** 3).toBeCloseTo(fixture.measures[0], 10);
      expect(mesh.geometryAssertions!.surfaceArea / millimetresPerBase ** 2).toBeCloseTo(fixture.measures[1], 10);
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
      if (units === "in") expect(canonical).toContain("LENGTH_MEASURE(0.0254)");
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
