import { readFile } from "node:fs/promises";
import { expect, test, type Page } from "@playwright/test";
import { createEmptyDocument, createExtrudeFeature, upsertFeature, upsertSketch } from "../src/cad/document/CadDocument";
import { addCircleAt, addCornerRectangle, createXySketch } from "../src/cad/sketch/SketchModel";
import { solveSketch } from "../src/cad/sketch/SketchSolver";
import { detectProfiles } from "../src/cad/sketch/profileDetection";
import type { ViewerSnapshot } from "../src/viewer/viewerDiagnostics";
import { assertAiAcceptanceViewer, stlSignedVolume } from "./aiAcceptanceHelpers";
import { generatePublicStep, observeStepProofs, stepTranscripts } from "./stepExportHelpers";
import { openStepImport, stepCommand } from "./stepImportAcceptance";

const bounds = { min: [0, 0, 0], max: [10, 6, 6] };
const stockArea = 312, stockVolume = 360;
const angle = Math.PI - Math.acos(.6), capOpening = 6.25 * angle + 3;
function fixture(depth: number) {
  const base = addCornerRectangle(createXySketch("Rectangle"), "10mm", "6mm");
  const stock = createExtrudeFeature({ name: "Pocket stock", sketchId: base.id,
    profileId: detectProfiles(solveSketch(base, {})).profiles[0].id, operation: "newBody", direction: "positive",
    distance: { expression: "6mm", unit: "mm" } });
  let document = upsertFeature(upsertSketch(createEmptyDocument("Side-opening pocket pair"), base), stock);
  for (const x of [3.5, 6.5]) {
    const circle = addCircleAt(createXySketch(`Cutter at ${x}`), `${x}mm`, "0mm", "2.5mm");
    const cut = createExtrudeFeature({ name: `Cut ${x}`, sketchId: circle.id,
      profileId: detectProfiles(solveSketch(circle, {})).profiles[0].id, operation: "cut", direction: "positive",
      distance: { expression: `${depth}mm`, unit: "mm" }, targetBodyIds: [`body:${stock.id}`] });
    document = upsertFeature(upsertSketch(document, circle), cut);
  }
  return { document, bodyId: `body:${stock.id}` };
}

test("side opening close-up: refined display leaves the accepted pick geometry intact", async ({ page }) => {
  await page.goto("/");
  const { document } = fixture(3);
  expect(await stepCommand(page, { command: "file.openProject", arguments: { file: { name: "pocket-closeup.pcaddoc", text: JSON.stringify(document) } } })).toMatchObject({ ok: true });
  const accepted = await rebuilt(page);
  await assertAiAcceptanceViewer(page, 1);
  const view = () => page.evaluate<ViewerSnapshot>(async () => {
    const path = "/src/viewer/viewerDiagnostics.ts";
    return (await import(path)).inspectViewer();
  });
  const before = await view();
  await page.getByLabel("3D modeling canvas", { exact: true }).hover();
  await page.mouse.wheel(0, -1300);
  await expect.poll(async () => (await view()).displayDetail?.status, { timeout: 25000 }).toBe("ready");
  await expect.poll(async () => {
    const frame = await view();
    return frame.performance?.triangles === frame.displayDetail?.triangles;
  }).toBe(true);
  const detail = await view();
  expect(detail.displayDetail!.triangles).toBeGreaterThan(before.meshes[0].indices.length / 3);
  expect(detail.meshes[0].geometryId).toBe(before.meshes[0].geometryId);
  expect(detail.meshes[0].positions).toEqual(before.meshes[0].positions);
  expect((await rebuilt(page)).rebuild.bodies).toEqual(accepted.rebuild.bodies);
});

interface Snapshot {
  session: number;
  document: { id: string; features: Array<{ id: string; type: string; stepSource?: string }> };
  rebuild: { native: boolean; bodies: Array<{ id: string; bounds: typeof bounds;
    assertions: { valid: boolean; solidCount: number; volume: number; surfaceArea: number } }> };
}
async function rebuilt(page: Page) {
  const state = await stepCommand(page, { command: "runtime.awaitNative" });
  if (!state.ok) throw new Error(state.error.message);
  return state.value as unknown as Snapshot;
}

for (const depth of [3, 6]) {
  test(`Go WASM overlap of two side-opening circles: ${depth === 3 ? "blind" : "through"} rebuild, reopen, STL and STEP`, async ({ page }, info) => {
    const volume = stockVolume - depth * capOpening;
    const area = stockArea + depth * (5 * angle - 8) - (depth === 6 ? 2 * capOpening : 0);
    const forbidden: string[] = [];
    page.on("request", request => { if (/opencascade\.(?:wasm|js)/.test(request.url())) forbidden.push(request.url()); });
    await observeStepProofs(page);
    await page.goto("/");
    const { document, bodyId } = fixture(depth);
    const opened = await stepCommand(page, { command: "file.openProject", arguments: { file: { name: "pocket-pair.pcaddoc", text: JSON.stringify(document) } } });
    expect(opened).toMatchObject({ ok: true });
    const check = async (expectedBody?: string) => {
      const state = await rebuilt(page);
      expect(state.rebuild.native).toBe(true);
      expect(state.rebuild.bodies).toHaveLength(1);
      const [body] = state.rebuild.bodies;
      if (expectedBody) expect(body.id).toBe(expectedBody);
      expect(body.bounds).toEqual(bounds);
      expect(body.assertions).toMatchObject({ valid: true, solidCount: 1 });
      expect(body.assertions.volume).toBeCloseTo(volume, 7);
      expect(body.assertions.surfaceArea).toBeCloseTo(area, 7);
      return state.session;
    };
    const session = await check(bodyId);
    await assertAiAcceptanceViewer(page, 1);
    const save = page.waitForEvent("download");
    expect(await stepCommand(page, { command: "file.saveProject" })).toMatchObject({ ok: true });
    const saved = info.outputPath("side-opening.pcaddoc");
    await (await save).saveAs(saved);
    const text = await readFile(saved, "utf8");
    expect(JSON.parse(text).features.map((f: { operation: string }) => f.operation)).toEqual(["newBody", "cut", "cut"]);
    expect(text).not.toMatch(/kernelHandle|geometryAssertions/);
    expect(await stepCommand(page, { command: "file.openProject", arguments: { file: { name: "pocket-pair.pcaddoc", text } } })).toMatchObject({ ok: true });
    expect(await check(bodyId)).toBeGreaterThan(session);

    const stlDownload = page.waitForEvent("download");
    expect(await stepCommand(page, { command: "file.exportStl" })).toMatchObject({ ok: true });
    const mesh = info.outputPath("side-opening.stl");
    await (await stlDownload).saveAs(mesh);
    expect(Math.abs(stlSignedVolume(await readFile(mesh)) - volume)).toBeLessThan(.3);

    const dialog = await generatePublicStep(page);
    const proof = (await stepTranscripts(page)).at(-1)!;
    expect(proof.bodyIds).toEqual([bodyId]);
    expect(proof.units).toBe("mm");
    for (const set of [proof.before, proof.after]) {
      expect(set).toHaveLength(1);
      expect(set[0]).toMatchObject({ valid: true, solidCount: 1, bounds });
      expect(set[0].volume).toBeCloseTo(volume, 7);
      expect(set[0].surfaceArea).toBeCloseTo(area, 7);
    }
    const stepDownload = page.waitForEvent("download");
    await dialog.getByRole("button", { name: "Download STEP file", exact: true }).click();
    const step = info.outputPath("side-opening.step");
    await (await stepDownload).saveAs(step);
    const source = await readFile(step, "utf8");
    expect(source.match(/CYLINDRICAL_SURFACE\(/g)).toHaveLength(2);
    expect(source).not.toContain("PCURVE(");

    const empty = createEmptyDocument("STEP readback");
    expect(await stepCommand(page, { command: "file.openProject", arguments: { file: { name: "empty.pcaddoc", text: JSON.stringify(empty) } } })).toMatchObject({ ok: true });
    await expect.poll(async () => {
      const state = await stepCommand(page, { command: "runtime.snapshot" });
      return state.ok ? (state.value as unknown as { document: { id: string } }).document.id : undefined;
    }).toBe(empty.id);
    const importDialog = await openStepImport(page);
    await importDialog.getByLabel("STEP file", { exact: true }).setInputFiles(step);
    const apply = importDialog.getByRole("button", { name: "Add STEP solid", exact: true });
    await expect(apply).toBeEnabled();
    await apply.click();
    await expect(importDialog).toBeHidden();
    // Immediately after Apply, the command can still see the prior empty
    // rebuild for the same project ID. Observe the imported feature and its
    // own native body together before asking for the independent snapshot.
    let importedBody = "";
    await expect.poll(async () => {
      const state = await stepCommand(page, { command: "runtime.snapshot" });
      if (!state.ok) throw new Error(state.error.message);
      const snapshot = state.value as unknown as Snapshot;
      const feature = snapshot.document.features.find(f => f.type === "stepImport" && f.stepSource === source);
      if (!feature || !snapshot.rebuild.native) return false;
      importedBody = `body:${feature.id}`;
      return snapshot.rebuild.bodies.some(body => body.id === importedBody);
    }).toBe(true);
    await check(importedBody);
    expect(forbidden).toEqual([]);
  });
}
