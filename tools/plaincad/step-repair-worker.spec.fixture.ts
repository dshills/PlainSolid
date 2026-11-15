import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import type { StepRepairProbeReply, StepRepairProbeRequest } from "../src/cad/kernel/plainSolidStepRepairProbe";

const fixtures = [
  { name: "millimetre", path: "/plainsolid-step-repair-input.step", units: undefined, otherPath: undefined, scale: 1, metresPerUnit: .001, grouped: false },
  { name: "metre", path: "/plainsolid-step-repair-metre.step", units: "m", otherPath: "/plainsolid-step-repair-inch.step", scale: 1000, metresPerUnit: 1, grouped: false },
  { name: "inch", path: "/plainsolid-step-repair-inch.step", units: "in", otherPath: "/plainsolid-step-repair-metre.step", scale: 25.4, metresPerUnit: .0254, grouped: false },
  { name: "grouped metre", path: "/plainsolid-grouped-step-repair-metre.step", units: "m", otherPath: "/plainsolid-grouped-step-repair-inch.step", scale: 1000, metresPerUnit: 1, grouped: true },
  { name: "grouped inch", path: "/plainsolid-grouped-step-repair-inch.step", units: "in", otherPath: "/plainsolid-grouped-step-repair-metre.step", scale: 25.4, metresPerUnit: .0254, grouped: true },
  { name: "converted metre", path: "/plainsolid-converted-step-repair-metre.step", units: "m", otherPath: "/plainsolid-step-repair-inch.step", scale: 1000, metresPerUnit: 1, grouped: false,
    converted: { ownerScale: "1/1000", axialEnd: "5000", height: "5" } },
  { name: "converted inch", path: "/plainsolid-converted-step-repair-inch.step", units: "in", otherPath: "/plainsolid-step-repair-metre.step", scale: 25.4, metresPerUnit: .0254, grouped: false,
    sourceScale: 5, maxDistance: 1e-11, rejectedDistance: 1e-12, poseTranslation: ["8", "6"],
    converted: { ownerScale: "5/127", axialEnd: "635", height: "25" } },
  { name: "grouped converted metre", path: "/plainsolid-grouped-converted-step-repair-metre.step", units: "m", otherPath: "/plainsolid-grouped-step-repair-inch.step", scale: 1000, metresPerUnit: 1, grouped: true,
    rejectedDistance: 1e-13, converted: { ownerScale: "1/1000", axialEnd: "5000", height: "5" } },
  { name: "grouped converted inch", path: "/plainsolid-grouped-converted-step-repair-inch.step", units: "in", otherPath: "/plainsolid-grouped-step-repair-metre.step", scale: 25.4, metresPerUnit: .0254, grouped: true,
    sourceScale: 5, maxDistance: 1e-11, rejectedDistance: 1e-12,
    converted: { ownerScale: "5/127", axialEnd: "635", height: "25" } },
  { name: "three-fillet", path: "/plainsolid-three-step-repair.step", units: undefined, otherPath: undefined, scale: 1, metresPerUnit: .001,
    grouped: true, three: true, rejectedDistance: 1e-13 },
] as const;
const sha256 = (bytes: Buffer) => createHash("sha256").update(bytes).digest("hex");

for (const fixture of fixtures) test(`PlainSolid module workers preserve the ${fixture.name} STEP repair certificate and strict canonical reopen`, async ({ page }, info) => {
  const workers: string[] = [];
  const wasmLoads: number[] = [];
  const occtRequests: string[] = [];
  page.on("worker", worker => workers.push(worker.url()));
  page.on("response", response => {
    if (new URL(response.url()).pathname === "/assets/plainsolid-kernel.wasm") wasmLoads.push(response.status());
  });
  page.on("request", request => {
    if (/opencascade.*\.(?:wasm|js)/i.test(request.url())) occtRequests.push(request.url());
  });
  await page.goto("/plainsolid-step-repair-worker.html");
  const three = "three" in fixture;
  const linearDeflection = "converted" in fixture || three ? .5 : .025 * fixture.scale;
  const maxDistance = "maxDistance" in fixture ? fixture.maxDistance : 1e-12;
  const rejectedDistance = "rejectedDistance" in fixture ? fixture.rejectedDistance : undefined;
  const result = await page.evaluate(async ({ path, units, otherPath, linearDeflection, maxDistance, rejectedDistance }) => {
    function ask(request: StepRepairProbeRequest, transfer: ArrayBuffer[] = []): Promise<StepRepairProbeReply> {
      const worker = new Worker("/src/cad/kernel/plainSolidStepRepairProbe.ts", { type: "module" });
      return new Promise((resolve, reject) => {
        worker.onmessage = event => { worker.terminate(); resolve(event.data); };
        worker.onerror = event => { worker.terminate(); reject(new Error(event.message)); };
        worker.onmessageerror = () => { worker.terminate(); reject(new Error("Unreadable module worker message")); };
        worker.postMessage(request, transfer);
      });
    }
    async function sourceAt(url: string): Promise<string> {
      const response = await fetch(url);
      if (!response.ok) throw new Error(`STEP fixture fetch failed (${response.status})`);
      return response.text();
    }
    const source = await sourceAt(path);
    const otherSource = otherPath ? await sourceAt(otherPath) : undefined;
    const repair = await ask({ operation: "repair", source, units, otherSource, linearDeflection, maxDistance, rejectedDistance });
    if (!repair.ok) throw new Error(repair.error);
    if (!repair.repair || !repair.evidence?.nativeProofJSON) throw new Error("Missing the repaired worker reply");
    const fromWasm = repair.repair.certificateFromWasm;
    const certificate = repair.evidence.nativeProofJSON;
    const certificateFromWasm = Array.from(new Uint8Array(fromWasm));
    const strict = await ask({ operation: "reopen", source: new TextDecoder().decode(repair.canonical), units,
      receipt: { certificate, fromWasm } }, [fromWasm]);
    if (!strict.ok) throw new Error(strict.error);
    if (!strict.reopen) throw new Error("Missing the strict worker reply");
    return {
      certificate, certificateFromWasm,
      certificateReopened: strict.reopen.reprovedCertificate,
      repairEvidence: repair.evidence.repairEvidence,
      strictFailure: repair.repair.strictFailure,
      budgetFailure: repair.repair.budgetFailure,
      triangles: repair.repair.triangles,
      positionsInWorldBounds: repair.repair.positionsInWorldBounds,
      before: repair.before,
      after: repair.after,
      strictBefore: strict.before,
      strictAfter: strict.after,
      strictEvidence: strict.evidence,
      initialUnits: { file: repair.units, proof: repair.proofUnits, source: repair.sourceUnit },
      reopenedUnits: { file: strict.units, proof: strict.proofUnits, source: strict.sourceUnit },
      unitFailures: { repairMismatch: repair.repair.wrongImport, defaultExport: repair.repair.omittedExport,
        exportMismatch: repair.repair.wrongExport, mixedExport: repair.repair.mixedExport,
        defaultReopen: strict.reopen.omittedImport, reopenMismatch: strict.reopen.wrongImport },
      canonical: Array.from(new Uint8Array(repair.canonical)),
      canonicalReopened: Array.from(new Uint8Array(strict.canonical)),
    };
  }, { path: fixture.path, units: fixture.units, otherPath: fixture.otherPath, linearDeflection, maxDistance, rejectedDistance });

  expect(workers).toHaveLength(2);
  expect(wasmLoads.length).toBeGreaterThan(0);
  expect(wasmLoads.every(status => status === 200 || status === 304)).toBe(true);
  expect(occtRequests).toEqual([]);
  expect(result.strictFailure).toMatch(/PlainSolid UNSUPPORTED/);
  if (rejectedDistance !== undefined) {
    expect(result.budgetFailure).toMatch(/PlainSolid UNSUPPORTED: STEP source has no qualified default OCCT pcurve repair within maxDistance/);
  }
  expect(result.triangles).toBeGreaterThan(0);
  expect(result.positionsInWorldBounds).toBe(true);

  const source = await readFile(`public${fixture.path}`);
  const digest = sha256(source);
  const [x, z] = "poseTranslation" in fixture ? fixture.poseTranslation : fixture.grouped ? ["0", "0"] : ["8/5", "6/5"];
  const pose = [["0", "3/5", "4/5", x], ["-1", "0", "0", "0"], ["0", "-4/5", "3/5", z]];
  const family = fixture.grouped ? "occt-rational-posed-grouped-pcurve-convex-arc" : "occt-rational-posed-single-pcurve-convex-arc";
  expect(result.repairEvidence).toMatchObject({ family,
    sourceSHA256: digest, maxDistance, pcurveOwners: three ? 34 : fixture.grouped ? 28 : 24, worldPose: pose, lengthUnit: fixture.metresPerUnit });
  const certificateBytes = Buffer.from(result.certificateFromWasm);
  expect(Buffer.from(result.certificate, "utf8")).toEqual(certificateBytes);
  expect(Buffer.from(result.certificateReopened, "utf8")).toEqual(certificateBytes);
  expect(result.certificate.endsWith("\n")).toBe(true);
  const certificate = JSON.parse(result.certificate);
  const format = fixture.grouped ? "plainsolid-step-rational-posed-grouped-pcurve-convex-arc-source" : "plainsolid-step-rational-posed-pcurve-convex-arc-source";
  expect(certificate).toMatchObject({ Format: format, Version: 1, MaxDistance: maxDistance,
    Claims: { SourceSHA256: digest, Pose: pose, MaxDistance: maxDistance } });
  expect(Buffer.from(certificate.Source, "base64")).toEqual(source);
  const groupedOwners = fixture.grouped ? { finiteLineOwners: "converted" in fixture ? 18 : 8, rationalCaps: 2, axialTrims: three ? 2 : 1, finiteTrims: 2 } : undefined;
  if (groupedOwners) {
    expect(result.repairEvidence).toMatchObject({ finiteLineOwners: groupedOwners.finiteLineOwners, rationalCaps: 2, axialTrims: groupedOwners.axialTrims });
    expect(result.repairEvidence.repairs).toHaveLength(three ? 10 : 8);
    expect(certificate.Claims.FiniteLineOwners).toHaveLength(groupedOwners.finiteLineOwners);
    expect(certificate.Claims.Grouped.RationalCaps).toHaveLength(groupedOwners.rationalCaps);
    expect(certificate.Claims.Grouped.FiniteTrims).toHaveLength(groupedOwners.finiteTrims);
    expect(certificate.Claims.Grouped.AxialTrims).toHaveLength(groupedOwners.axialTrims);
  }
  const phaseOwners = three ? certificate.Claims.PCurves.filter((owner: { Kind: string }) => owner.Kind === "phase-cylinder-circle") : undefined;
  if (three) {
    expect(phaseOwners).toMatchObject([
      { PCurveID: 210, DistanceSquaredUpper: expect.any(String) },
      { PCurveID: 425, DistanceSquaredUpper: expect.any(String) },
    ]);
  }
  const convertedOwners = "converted" in fixture ? { finiteLineOwners: 18, cylindricalSpanOwners: fixture.grouped ? 8 : 4, ...fixture.converted } : undefined;
  if (convertedOwners) {
    const { ownerScale, axialEnd, height } = convertedOwners;
    expect(result.repairEvidence.finiteLineOwners).toBe(convertedOwners.finiteLineOwners);
    expect(result.repairEvidence.repairs).toHaveLength(fixture.grouped ? 8 : 3);
    expect(certificate.Claims.FiniteLineOwners).toHaveLength(convertedOwners.finiteLineOwners);
    for (const owner of certificate.Claims.FiniteLineOwners) expect(owner.PCurveParameterScale).toBe(ownerScale);
    const spans = certificate.Claims.CylindricalSpanOwners;
    expect(spans).toHaveLength(convertedOwners.cylindricalSpanOwners);
    if (fixture.grouped) {
      const quarterTurn = "314159265359/200000000000";
      const repairedToOwner = fixture.units === "in" ? [
        "-625000000000000000000000000001/615157480315000000000000000000000000000000000000000000000",
        "625000000000000000000000000001/625000000000000000000000000000",
      ] : [
        "7499999999999999999999999999991/1250000000000000000000000000000000000000000000000000000000",
        "2499999999999999999999999999997/2500000000000000000000000000000",
      ];
      expect(spans).toEqual(expect.arrayContaining([
        { PCurveID: 237, ParameterInterval: ["0", quarterTurn], UVStart: ["0", "0"], UVDerivative: ["-1", "0"] },
        { PCurveID: 277, ParameterInterval: ["0", quarterTurn], UVStart: ["0", height], UVDerivative: ["-1", "0"] },
        { PCurveID: 256, ParameterInterval: ["0", axialEnd], UVStart: [`-${quarterTurn}`, "0"], UVDerivative: ["0", ownerScale],
          RepairedToOwner: repairedToOwner },
      ]));
      expect(certificate.Claims.Grouped.FiniteTrims).toMatchObject([
        { CapOwner: { PCurveID: 226 }, CylinderOwner: { PCurveID: 237 }, EndStation: expect.any(Object) },
        { CapOwner: { PCurveID: 283 }, CylinderOwner: { PCurveID: 277 }, EndStation: expect.any(Object) },
      ]);
    } else {
      const sweep = "2498091544797/1000000000000";
      expect(spans).toEqual(expect.arrayContaining([
        { PCurveID: 125, ParameterInterval: ["0", axialEnd], UVStart: ["0", "0"], UVDerivative: ["0", ownerScale] },
        { PCurveID: 225, ParameterInterval: ["0", axialEnd], UVStart: [sweep, "0"], UVDerivative: ["0", ownerScale] },
        { PCurveID: 157, ParameterInterval: ["0", sweep], UVStart: ["0", "0"], UVDerivative: ["1", "0"] },
        { PCurveID: 246, ParameterInterval: ["0", sweep], UVStart: ["0", height], UVDerivative: ["1", "0"] },
      ]));
    }
    if (fixture.units === "in" && !fixture.grouped) expect(certificate.Claims.PCurves).toContainEqual(expect.objectContaining({
      PCurveID: 125, Kind: "finite-cylinder-line", DistanceSquaredUpper: "625/244140625000031250000000001",
    }));
  }

  expect(result.before).toHaveLength(1);
  const [proof] = result.before;
  const millimetresPerBase = fixture.scale * ("sourceScale" in fixture ? fixture.sourceScale : 1);
  expect(proof).toMatchObject({ valid: true, solidCount: 1 });
  expect(proof.volume / millimetresPerBase ** 3).toBeCloseTo(three ? 28.968988172080639 : fixture.grouped ? 29.077978807115539 : 29.145041881053842, 10);
  expect(proof.surfaceArea / millimetresPerBase ** 2).toBeCloseTo(three ? 64.184278587623695 : fixture.grouped ? 65.622954970084379 : 66.186284791166074, 10);
  for (const [side, expected] of [["min", fixture.grouped ? [0, 0, -2.7] : [1, 0, -.7]],
    ["max", three ? [6.025, 2.84375, 3] : fixture.grouped ? [6.025, 3, 3] : [7.025, 3, 5]]] as const) {
    proof.bounds[side].forEach((value, axis) => expect(value / millimetresPerBase).toBeCloseTo(expected[axis], 11));
  }
  const expectedUnits = { file: fixture.units ?? "mm", proof: fixture.units ? "mm" : undefined, source: fixture.units };
  expect(result.initialUnits).toEqual(expectedUnits);
  expect(result.reopenedUnits).toEqual(expectedUnits);
  if (fixture.units) {
    const { repairMismatch, defaultExport, exportMismatch, mixedExport, defaultReopen, reopenMismatch } = result.unitFailures;
    for (const message of [repairMismatch, defaultReopen, reopenMismatch]) expect(message).toMatch(/PlainSolid INVALID: STEP import unit must match/);
    for (const message of [defaultExport, exportMismatch, mixedExport]) expect(message).toMatch(/PlainSolid INVALID: STEP export unit must match/);
  }
  expect(result.after).toEqual(result.before);
  expect(result.strictBefore).toEqual(result.before);
  expect(result.strictAfter).toEqual(result.before);
  expect(result.strictEvidence).toBeUndefined();
  const canonical = Buffer.from(result.canonical);
  const canonicalReopened = Buffer.from(result.canonicalReopened);
  expect(canonicalReopened).toEqual(canonical);
  if (fixture.units === "m") expect(canonical.toString("utf8")).toContain("SI_UNIT($,.METRE.)");
  if (fixture.units === "in") expect(canonical.toString("utf8")).toContain("LENGTH_MEASURE(0.0254)");
  await info.attach("step-repair-worker-proof", { contentType: "application/json", body: JSON.stringify({
    sourceSha256: digest, certificateSha256: sha256(certificateBytes), reopenedCertificateSha256: sha256(Buffer.from(result.certificateReopened)),
    canonicalSha256: sha256(canonical), reopenedCanonicalSha256: sha256(canonicalReopened),
    units: result.initialUnits, unitFailures: result.unitFailures, family, groupedOwners, convertedOwners, phaseOwners, proof,
    maxDistance, rejectedDistance, budgetFailure: result.budgetFailure, linearDeflection, triangles: result.triangles,
  }, null, 2) });
});
