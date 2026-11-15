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
    grouped: true, fillets: 3, rejectedDistance: 1e-13 },
  { name: "three-fillet converted metre", path: "/plainsolid-three-converted-step-repair-metre.step", units: "m",
    otherPath: "/plainsolid-grouped-step-repair-inch.step", scale: 1000, metresPerUnit: 1,
    grouped: true, fillets: 3, rejectedDistance: 1e-13,
    converted: { ownerScale: "1/1000", axialEnd: "5000", height: "5" } },
  { name: "three-fillet converted inch", path: "/plainsolid-three-converted-step-repair-inch.step", units: "in",
    otherPath: "/plainsolid-grouped-step-repair-metre.step", scale: 25.4, metresPerUnit: .0254,
    grouped: true, fillets: 3, sourceScale: 5, maxDistance: 1e-11, rejectedDistance: 1e-12,
    converted: { ownerScale: "5/127", axialEnd: "635", height: "25" } },
  { name: "four-fillet", path: "/plainsolid-four-step-repair.step", units: undefined, otherPath: undefined, scale: 1, metresPerUnit: .001,
    grouped: true, fillets: 4, rejectedDistance: 1e-13 },
  { name: "four-fillet converted metre", path: "/plainsolid-four-converted-step-repair-metre.step", units: "m",
    otherPath: "/plainsolid-grouped-step-repair-inch.step", scale: 1000, metresPerUnit: 1,
    grouped: true, fillets: 4, rejectedDistance: 1e-13,
    converted: { ownerScale: "1/1000", axialEnd: "5000", height: "5" } },
  { name: "four-fillet converted inch", path: "/plainsolid-four-converted-step-repair-inch.step", units: "in",
    otherPath: "/plainsolid-grouped-step-repair-metre.step", scale: 25.4, metresPerUnit: .0254,
    grouped: true, fillets: 4, sourceScale: 5, maxDistance: 1e-11, rejectedDistance: 1e-12,
    converted: { ownerScale: "5/127", axialEnd: "635", height: "25" } },
  { name: "five-fillet", path: "/plainsolid-five-step-repair.step", negativePath: "/plainsolid-five-nondyadic.step",
    units: undefined, otherPath: undefined, scale: 1, metresPerUnit: .001,
    grouped: true, fillets: 5, rejectedDistance: 1e-13 },
  { name: "five-fillet converted metre", path: "/plainsolid-five-converted-step-repair-metre.step", units: "m",
    otherPath: "/plainsolid-grouped-step-repair-inch.step", scale: 1000, metresPerUnit: 1,
    grouped: true, fillets: 5, rejectedDistance: 1e-13,
    converted: { ownerScale: "1/1000", axialEnd: "5000", height: "5" } },
  { name: "five-fillet converted inch", path: "/plainsolid-five-converted-step-repair-inch.step", units: "in",
    otherPath: "/plainsolid-grouped-step-repair-metre.step", negativePath: "/plainsolid-five-unscaled-inch.step", scale: 25.4, metresPerUnit: .0254,
    grouped: true, fillets: 5, sourceScale: 5, maxDistance: 1e-11, rejectedDistance: 1e-12,
    converted: { ownerScale: "5/127", axialEnd: "635", height: "25" } },
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
  const phased = "fillets" in fixture;
  const three = phased && fixture.fillets === 3;
  const four = phased && fixture.fillets === 4;
  const five = phased && fixture.fillets === 5;
  const converted = "converted" in fixture;
  const linearDeflection = converted || phased ? .5 : .025 * fixture.scale;
  const maxDistance = "maxDistance" in fixture ? fixture.maxDistance : 1e-12;
  const rejectedDistance = "rejectedDistance" in fixture ? fixture.rejectedDistance : undefined;
  const negativePath = "negativePath" in fixture ? fixture.negativePath : undefined;
  const result = await page.evaluate(async ({ path, units, otherPath, negativePath, linearDeflection, maxDistance, rejectedDistance }) => {
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
    const negativeSource = negativePath ? await sourceAt(negativePath) : undefined;
    const repair = await ask({ operation: "repair", source, units, otherSource, negativeSource, linearDeflection, maxDistance, rejectedDistance });
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
      negativeFailure: repair.repair.negativeFailure,
      negativeSource,
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
  }, { path: fixture.path, units: fixture.units, otherPath: fixture.otherPath, negativePath, linearDeflection, maxDistance, rejectedDistance });

  expect(workers).toHaveLength(2);
  expect(wasmLoads.length).toBeGreaterThan(0);
  expect(wasmLoads.every(status => status === 200 || status === 304)).toBe(true);
  expect(occtRequests).toEqual([]);
  expect(result.strictFailure).toMatch(/PlainSolid UNSUPPORTED/);
  if (rejectedDistance !== undefined) {
    expect(result.budgetFailure).toMatch(/PlainSolid UNSUPPORTED: STEP source has no qualified default OCCT pcurve repair within maxDistance/);
  }
  let negativeSourceSha256: string | undefined;
  if (negativePath) {
    expect(result.negativeFailure).toMatch(/PlainSolid UNSUPPORTED: STEP source has no qualified default OCCT pcurve repair within maxDistance/);
    const staged = await readFile(`public${negativePath}`);
    expect(Buffer.from(result.negativeSource!, "utf8")).toEqual(staged);
    negativeSourceSha256 = sha256(staged);
  }
  expect(result.triangles).toBeGreaterThan(0);
  expect(result.positionsInWorldBounds).toBe(true);

  const source = await readFile(`public${fixture.path}`);
  const digest = sha256(source);
  const [x, z] = "poseTranslation" in fixture ? fixture.poseTranslation : fixture.grouped ? ["0", "0"] : ["8/5", "6/5"];
  const pose = [["0", "3/5", "4/5", x], ["-1", "0", "0", "0"], ["0", "-4/5", "3/5", z]];
  const family = fixture.grouped ? "occt-rational-posed-grouped-pcurve-convex-arc" : "occt-rational-posed-single-pcurve-convex-arc";
  const pcurveOwners = five ? 58 : four ? 46 : three ? 34 : fixture.grouped ? 28 : 24;
  expect(result.repairEvidence).toMatchObject({ family,
    sourceSHA256: digest, maxDistance, pcurveOwners, worldPose: pose, lengthUnit: fixture.metresPerUnit });
  const certificateBytes = Buffer.from(result.certificateFromWasm);
  expect(Buffer.from(result.certificate, "utf8")).toEqual(certificateBytes);
  expect(Buffer.from(result.certificateReopened, "utf8")).toEqual(certificateBytes);
  expect(result.certificate.endsWith("\n")).toBe(true);
  const certificate = JSON.parse(result.certificate);
  const format = fixture.grouped ? "plainsolid-step-rational-posed-grouped-pcurve-convex-arc-source" : "plainsolid-step-rational-posed-pcurve-convex-arc-source";
  expect(certificate).toMatchObject({ Format: format, Version: 1, MaxDistance: maxDistance,
    Claims: { SourceSHA256: digest, Pose: pose, MaxDistance: maxDistance } });
  expect(Buffer.from(certificate.Source, "base64")).toEqual(source);
  const finiteLineOwners = five ? (converted ? 30 : 12) : converted ? four ? 24 : 18 : 8;
  const groupedOwners = fixture.grouped ? { finiteLineOwners, rationalCaps: 2, axialTrims: phased ? 2 : 1, finiteTrims: 2 } : undefined;
  if (groupedOwners) {
    expect(result.repairEvidence).toMatchObject({ finiteLineOwners: groupedOwners.finiteLineOwners, rationalCaps: 2, axialTrims: groupedOwners.axialTrims });
    expect(result.repairEvidence.repairs).toHaveLength(phased ? 10 : 8);
    expect(certificate.Claims.FiniteLineOwners).toHaveLength(groupedOwners.finiteLineOwners);
    expect(certificate.Claims.Grouped.RationalCaps).toHaveLength(groupedOwners.rationalCaps);
    expect(certificate.Claims.Grouped.FiniteTrims).toHaveLength(groupedOwners.finiteTrims);
    expect(certificate.Claims.Grouped.AxialTrims).toHaveLength(groupedOwners.axialTrims);
  }
  const phaseKind = converted ? "finite-phase-cylinder-circle" : "phase-cylinder-circle";
  const phaseOwners = phased ? certificate.Claims.PCurves.filter((owner: { Kind: string }) => owner.Kind === phaseKind) : undefined;
  if (phased) {
    const owners = five ? (converted ? [207, 263, 319, 528, 574, 620] : [210, 267, 324, 539, 586, 633]) : four ? (converted ? [207, 263, 472, 518] : [210, 267, 482, 529]) :
      converted ? [207, 416] : [210, 425];
    expect(phaseOwners).toMatchObject(
      owners.map(PCurveID => ({ PCurveID, DistanceSquaredUpper: expect.any(String) })));
  }
  if (five && !converted) {
    expect(phaseOwners.filter((owner: { PCurveID: number }) => owner.PCurveID === 324 || owner.PCurveID === 633)).toMatchObject([
      { PCurveID: 324, GeometryID: 331, SupportID: 325 },
      { PCurveID: 633, GeometryID: 635, SupportID: 325 },
    ]);
    const original = source.toString("utf8");
    for (const statement of [
      "#332 = CARTESIAN_POINT('',(-3.14159265359,0.));", "#636 = CARTESIAN_POINT('',(-3.14159265359,5.));",
      "#316 = DIRECTION('',(0.,-1.,0.));", "#625 = DIRECTION('',(0.,-1.,0.));", "#329 = DIRECTION('',(0.,1.,0.));",
    ]) expect(original).toContain(statement);
    expect(certificate.Claims.Grouped.AxialTrims).toMatchObject([{ EdgeID: 395 }, { EdgeID: 667 }]);
    expect(certificate.Claims.Grouped.FiniteTrims).toMatchObject([
      { CapOwner: { PCurveID: 372 }, CylinderOwner: { PCurveID: 383 }, EndStation: expect.any(Object) },
      { CapOwner: { PCurveID: 432 }, CylinderOwner: { PCurveID: 425 }, EndStation: expect.any(Object) },
    ]);
  }
  if (four && !converted) {
    expect(certificate.Claims.Grouped.AxialTrims).toMatchObject([{ EdgeID: 338 }, { EdgeID: 563 }]);
    expect(certificate.Claims.Grouped.FiniteTrims).toMatchObject([
      { CapOwner: { PCurveID: 315 }, CylinderOwner: { PCurveID: 326 }, EndStation: expect.any(Object) },
      { CapOwner: { PCurveID: 375 }, CylinderOwner: { PCurveID: 368 }, EndStation: expect.any(Object) },
    ]);
  }
  const convertedOwners = converted ? {
    finiteLineOwners, cylindricalSpanOwners: five ? 20 : four ? 16 : three ? 12 : fixture.grouped ? 8 : 4, ...fixture.converted,
  } : undefined;
  const finiteAxialTransfer = (four || five) && converted ? {
    owner: certificate.Claims.CylindricalSpanOwners.filter((span: { ClippedOwnerParameters?: unknown }) => span.ClippedOwnerParameters !== undefined),
    maps: certificate.Claims.CylindricalSpanOwners.filter((span: { RepairedToOwner?: unknown }) => span.RepairedToOwner !== undefined),
    trims: certificate.Claims.Grouped.AxialTrims,
  } : undefined;
  if (convertedOwners) {
    const { ownerScale, axialEnd, height } = convertedOwners;
    expect(result.repairEvidence.finiteLineOwners).toBe(convertedOwners.finiteLineOwners);
    expect(result.repairEvidence.repairs).toHaveLength(phased ? 10 : fixture.grouped ? 8 : 3);
    expect(certificate.Claims.FiniteLineOwners).toHaveLength(convertedOwners.finiteLineOwners);
    for (const owner of certificate.Claims.FiniteLineOwners) expect(owner.PCurveParameterScale).toBe(ownerScale);
    const spans = certificate.Claims.CylindricalSpanOwners;
    expect(spans).toHaveLength(convertedOwners.cylindricalSpanOwners);
    if (five) {
      expect(phaseOwners.filter((owner: { PCurveID: number }) => owner.PCurveID === 319 || owner.PCurveID === 620)).toMatchObject([
        { PCurveID: 319, GeometryID: 326, SupportID: 320 }, { PCurveID: 620, GeometryID: 622, SupportID: 320 },
      ]);
      expect(spans.filter((span: { PCurveID: number }) => span.PCurveID === 319 || span.PCurveID === 620)).toMatchObject(
        [319, 620].map((PCurveID, i) => ({ PCurveID, ParameterInterval: ["314159265359/100000000000", "942477796077/200000000000"],
          UVStart: ["0", ["0", height][i]], UVDerivative: ["1", "0"], ModelAngleBounds: expect.any(Array) })));
      const metre = fixture.units === "m";
      const projected = metre ? "1250000000000000000000000000001/250000000000000000000000000" :
        "6250000000000000000000000000003/9842519685040000000000000000";
      expect(finiteAxialTransfer).toMatchObject({
        owner: metre ? [{ PCurveID: 666, ClippedOwnerParameters: ["0", axialEnd], RepairedToOwner: ["-1/250000000000000000000000000", "1"] }] : [],
        maps: [396, 666].map(PCurveID => ({ PCurveID, ParameterInterval: ["0", axialEnd], UVDerivative: ["0", ownerScale] })),
        trims: [{ EdgeID: 388 }, { EdgeID: 653, OriginalParameters: ["0", projected] }],
      });
      expect(certificate.Claims.Grouped.FiniteTrims).toMatchObject([
        { CapOwner: { PCurveID: 366 }, CylinderOwner: { PCurveID: 377 }, EndStation: expect.any(Object) },
        { CapOwner: { PCurveID: 423 }, CylinderOwner: { PCurveID: 417 }, EndStation: expect.any(Object) },
      ]);
      expect(source.toString("utf8")).toContain(metre ? "#29 = VECTOR('',#30,1.E-03);" : "#29 = VECTOR('',#30,3.937007874016E-02);");
    } else if (four) {
      const shifted = spans.filter((span: { ModelAngleBounds?: unknown }) => span.ModelAngleBounds !== undefined);
      expect(shifted.map((span: { PCurveID: number }) => span.PCurveID)).toEqual([207, 263, 472, 518]);
      if (fixture.units === "in") {
        const denominator = "/984251968504000000000000000";
        const ends = ["625000000000000000000000000001", "625000000000000000000000000003"];
        expect(finiteAxialTransfer).toMatchObject({
          owner: [],
          maps: [340, 564].map(PCurveID => ({
            PCurveID, ParameterInterval: ["0", axialEnd], UVDerivative: ["0", ownerScale],
            RepairedToOwner: [expect.any(String), expect.any(String)],
          })),
          trims: [332, 551].map((EdgeID, i) => ({
            EdgeID, OriginalParameters: ["0", ends[i] + denominator],
            RepairedParameters: [["1", "3"][i] + denominator, ends[i] + denominator],
          })),
        });
      } else {
        const excess = "1/25000000000000000000000000";
        const projectedEnd = "125000000000000000000000000001/25000000000000000000000000";
        expect(finiteAxialTransfer).toMatchObject({
          owner: [{
            PCurveID: 564, ParameterInterval: ["0", axialEnd], UVStart: ["553574358897/250000000000", "0"],
            UVDerivative: ["0", ownerScale], RepairedToOwner: [`-${excess}`, "1"], ClippedOwnerParameters: ["0", axialEnd],
          }],
          trims: [{ EdgeID: 332 }, {
            EdgeID: 551, OriginalParameters: ["0", projectedEnd], RepairedParameters: [excess, projectedEnd],
          }],
        });
      }
      expect(certificate.Claims.Grouped.FiniteTrims).toMatchObject([
        { CapOwner: { PCurveID: 310 }, CylinderOwner: { PCurveID: 321 }, EndStation: expect.any(Object) },
        { CapOwner: { PCurveID: 367 }, CylinderOwner: { PCurveID: 361 }, EndStation: expect.any(Object) },
      ]);
    } else if (three) {
      const shiftedInterval = ["2498091544797/1000000000000", "942477796077/200000000000"];
      const modelAngleBounds = [["624522886199/250000000000", "1249045772399/500000000000"],
        ["147262155637/31250000000", "2356194490193/500000000000"]];
      expect(spans.filter((span: { ModelAngleBounds?: unknown }) => span.ModelAngleBounds !== undefined)).toEqual([
        { PCurveID: 207, ParameterInterval: shiftedInterval, UVStart: ["0", "0"], UVDerivative: ["1", "0"], ModelAngleBounds: modelAngleBounds },
        { PCurveID: 416, ParameterInterval: shiftedInterval, UVStart: ["0", height], UVDerivative: ["1", "0"], ModelAngleBounds: modelAngleBounds },
      ]);
      expect(spans.filter((span: { RepairedToOwner?: unknown }) => span.RepairedToOwner !== undefined)).toMatchObject(
        [284, 462].map(PCurveID => ({ PCurveID, ParameterInterval: ["0", axialEnd], UVDerivative: ["0", ownerScale],
          RepairedToOwner: [expect.any(String), expect.any(String)] })));
      expect(certificate.Claims.Grouped.FiniteTrims).toMatchObject([
        { CapOwner: { PCurveID: 254 }, CylinderOwner: { PCurveID: 265 }, EndStation: expect.any(Object) },
        { CapOwner: { PCurveID: 311 }, CylinderOwner: { PCurveID: 305 }, EndStation: expect.any(Object) },
      ]);
    } else if (fixture.grouped) {
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
  const [volume, area] = five ? [39.084768347549009, 68.682455982680239] : four ? [22.155304183748054, 51.012178844414059] :
    three ? [28.968988172080639, 64.184278587623695] :
    fixture.grouped ? [29.077978807115539, 65.622954970084379] : [29.145041881053842, 66.186284791166074];
  const [min, max] = five ? [[0, 0, -2.4], [5.8, 2.75, 3]] : four ? [[0, 0, -1.6], [5.2, 2.84375, 3]] :
    three ? [[0, 0, -2.7], [6.025, 2.84375, 3]] :
    fixture.grouped ? [[0, 0, -2.7], [6.025, 3, 3]] : [[1, 0, -.7], [7.025, 3, 5]];
  expect(proof).toMatchObject({ valid: true, solidCount: 1 });
  expect(proof.volume / millimetresPerBase ** 3).toBeCloseTo(volume, 10);
  expect(proof.surfaceArea / millimetresPerBase ** 2).toBeCloseTo(area, 10);
  for (const [side, expected] of [["min", min], ["max", max]] as const) {
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
  if (five && !fixture.units) expect(canonical.toString("utf8")).toContain("SI_UNIT(.MILLI.,.METRE.)");
  await info.attach("step-repair-worker-proof", { contentType: "application/json", body: JSON.stringify({
    sourceSha256: digest, certificateSha256: sha256(certificateBytes), reopenedCertificateSha256: sha256(Buffer.from(result.certificateReopened)),
    canonicalSha256: sha256(canonical), reopenedCanonicalSha256: sha256(canonicalReopened),
    units: result.initialUnits, unitFailures: result.unitFailures, family, groupedOwners, convertedOwners, phaseOwners, finiteAxialTransfer, proof,
    maxDistance, rejectedDistance, budgetFailure: result.budgetFailure, negativeSourceSha256, negativeFailure: result.negativeFailure,
    linearDeflection, triangles: result.triangles,
  }, null, 2) });
});
