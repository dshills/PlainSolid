#!/usr/bin/env node
// Runs real PlainCAD fixtures against Go WASM in an isolated copy. Never edits
// the source app and never imports OCCT from the replacement kernel adapter.
import { cp, mkdir, readdir, readFile, writeFile, symlink, stat, realpath } from "node:fs/promises";
import { spawn, execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { tmpdir } from "node:os";
import { resolve, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const args = process.argv.slice(2);
const option = (key, fallback) => { const index = args.indexOf(key); return index < 0 ? fallback : args[index + 1]; };
if (!option("--source")) throw new Error("Pass --source /absolute/path/to/PlainCAD explicitly.");
const source = resolve(option("--source"));
const work = resolve(option("--work", join(tmpdir(), `plainsolid-plaincad-${Date.now()}`)));
const port = option("--port", "5279");
if (!/^\d+$/.test(port) || Number(port) < 1024 || Number(port) > 65535) throw new Error("Invalid --port");
const testFile = option("--test", "e2e/extrusion-hole.spec.ts");
const grep = option("--grep", "native negative/symmetric coordinate extents, parameter edits, save/open and STL winding");
const gitIdentity = cwd => {
  const git = (...argv) => { try { return execFileSync("git", argv, { cwd, encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim(); } catch { return null; } };
  return { head: git("rev-parse", "HEAD"), status: git("status", "--porcelain=v1"), branch: git("branch", "--show-current") };
};
const hash = bytes => createHash("sha256").update(bytes).digest("hex");
const sourceIdentity = gitIdentity(source);
if (!sourceIdentity.head) {
  try {
    const bytes = await readFile(join(source, ".plainSolidSourceIdentity.json"));
    const archive = JSON.parse(bytes.toString("utf8"));
    if (archive.kind !== "git-archive" || archive.cleanCommittedArchive !== true ||
        typeof archive.head !== "string" || !/^[0-9a-f]{40}$/.test(archive.head) ||
        typeof archive.sourceRepository !== "string" || !archive.sourceRepository.startsWith("/"))
      throw new Error("Invalid committed archive identity manifest.");
    // This is explicit archive provenance, not a claim that the copied tree has
    // a git branch/index. Per-file hashes below prove the actual tested content.
    Object.keys(sourceIdentity).forEach(key => delete sourceIdentity[key]);
    Object.assign(sourceIdentity, { kind: "git-archive", head: archive.head,
      sourceRepository: archive.sourceRepository, cleanCommittedArchive: true,
      manifestSha256: hash(bytes) });
  } catch (error) {
    if (error?.code !== "ENOENT") throw error;
  }
}
const report = { schema: 1, backend: "plainsolid-go-wasm", source, workspace: work, startedAt: new Date().toISOString(), selected: { testFile, grep }, sourceHashes: {}, sourceIdentity, plainSolidIdentity: gitIdentity(root), plainSolidSourceHashes: {}, providerLabelAdaptations: [], tests: [], status: "running", notes: ["Only selected fixtures are measured; all unselected acceptance tests remain untested.", "Existing behavior/geometry assertions are preserved. Exact provider string literals are adapted from opencascade to plainsolid in the isolated copy."] };
async function command(program, argv, logName, env = {}, cwd = work) {
  const logPath = join(work, logName);
  const chunks = [];
  const child = spawn(program, argv, { cwd, env: { ...process.env, ...env }, stdio: ["ignore", "pipe", "pipe"] });
  for (const stream of [child.stdout, child.stderr]) stream.on("data", data => { chunks.push(data); process.stdout.write(data); });
  const code = await new Promise((resolveExit, reject) => { child.on("error", reject); child.on("close", resolveExit); });
  await writeFile(logPath, Buffer.concat(chunks));
  if (code !== 0) throw new Error(`${program} exited ${code}; see ${logPath}`);
}
async function files(directory) {
  const paths = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const name = join(directory, entry.name);
    if (entry.isDirectory()) paths.push(...await files(name));
    else if (entry.isFile()) paths.push(name);
  }
  return paths;
}
await mkdir(work, { recursive: true });
if ((await readdir(work)).length) throw new Error("Acceptance workspace must be empty; choose a fresh --work directory.");
try {
  await stat(join(source, "node_modules/@playwright/test"));
  await cp(source, work, { recursive: true, filter: path => !/(^|\/)(node_modules|\.git|dist|test-results|playwright-report|\.plainSolidViteCache)(\/|$)/.test(path) });
  await mkdir(join(work, "node_modules"));
  for (const name of await readdir(join(source, "node_modules"))) {
    if (name.startsWith(".") && name !== ".bin") continue;
    await symlink(join(source, "node_modules", name), join(work, "node_modules", name));
  }
  for (const folder of ["src", "e2e", "production-e2e", "cross-browser-e2e"]) {
    for (const path of await files(join(work, folder))) {
      if (!/\.[cm]?[jt]sx?$/.test(path)) continue;
      const relative = path.slice(work.length + 1);
      const before = await readFile(path, "utf8");
      report.sourceHashes[relative] = hash(before);
      const matches = [...before.matchAll(/(["'])opencascade\1/g)];
      if (matches.length) {
        const after = before.replace(/(["'])opencascade\1/g, "$1plainsolid$1");
        await writeFile(path, after);
        report.providerLabelAdaptations.push({ file: relative, count: matches.length, beforeSha256: hash(before), afterSha256: hash(after) });
      }
    }
  }
  for (const folder of ["internal/plaincadbridge", "cmd/plaincadwasm", "tools/plaincad"]) for (const path of await files(join(root, folder))) {
    report.plainSolidSourceHashes[path.slice(root.length + 1)] = hash(await readFile(path));
  }
  // Pin every local Go source selected for the WASM build, including kernel
  // dependencies outside the bridge. Record collection tooling separately too.
  const runtimeSources = execFileSync("go", ["list", "-deps", "-f", '{{if not .Standard}}{{range .GoFiles}}{{$.Dir}}/{{.}}{{"\\n"}}{{end}}{{end}}', "./cmd/plaincadwasm"], {
    cwd: root, encoding: "utf8", env: { ...process.env, GOOS: "js", GOARCH: "wasm", GOCACHE: join(tmpdir(), "plainsolid-cad-review-go-cache") },
  }).trim().split("\n").filter(path => path.startsWith(root + "/"));
  for (const path of [...runtimeSources, join(root, "go.mod")])
    report.plainSolidSourceHashes[path.slice(root.length + 1)] = hash(await readFile(path));
  await cp(join(root, "tools/plaincad/PlainSolidKernel.ts"), join(work, "src/cad/kernel/OpenCascadeKernel.ts"));
  let vite = await readFile(join(work, "vite.config.ts"), "utf8");
  vite = vite.replace("export default defineConfig({", 'export default defineConfig({\n  cacheDir: ".plainSolidViteCache",');
  vite = vite.replace("server: {", `server: {\n    fs: { allow: [${JSON.stringify(await realpath(work))}, ${JSON.stringify(await realpath(join(source, "node_modules")))}] },`);
  await writeFile(join(work, "vite.config.ts"), vite);
  const playwrightConfig = await readFile(join(work, "playwright.config.ts"), "utf8");
  await writeFile(join(work, "playwright.config.ts"), playwrightConfig.replaceAll("5279", port));
  await mkdir(join(work, "public/assets"), { recursive: true });
  await command("go", ["build", "-o", join(work, "public/assets/plainsolid-kernel.wasm"), "./cmd/plaincadwasm"], "wasm-build.log", { GOCACHE: join(tmpdir(), "plainsolid-cad-review-go-cache"), GOOS: "js", GOARCH: "wasm" }, root);
  const goroot = execFileSync("go", ["env", "GOROOT"], { encoding: "utf8" }).trim();
  await cp(join(goroot, "lib/wasm/wasm_exec.js"), join(work, "src/cad/kernel/wasm_exec.js"));
  await writeFile(join(work, "src/cad/kernel/wasm_exec.d.ts"), "export {};\n");
  report.wasmSha256 = hash(await readFile(join(work, "public/assets/plainsolid-kernel.wasm")));
  await writeFile(join(work, "e2e/plainsolid-backend.spec.ts"), `import { test, expect } from "@playwright/test";
test("PlainSolid backend: real Go WASM dispatcher and no OCCT requests", async ({ page }) => {
  const forbidden: string[] = [];
  page.on("request", request => { if (/opencascade\\.(?:wasm|js)/.test(request.url())) forbidden.push(request.url()); });
  await page.goto("/");
  const proof = await page.evaluate(async () => {
    const path = "/src/cad/kernel/OpenCascadeKernel.ts";
    const { OpenCascadeKernel } = await import(path);
    await OpenCascadeKernel.initialize();
    const kernel = new OpenCascadeKernel();
    const shape = kernel.createBox(2, 3, 4);
    const mesh = kernel.tessellate(shape, { linearDeflection: .1, angularDeflection: .2 });
    kernel.disposeShape(shape);
    const base = kernel.createBox(2, 3, 4);
    const inside = kernel.createBox(1, 1, 1);
    const covering = kernel.createBox(3, 4, 5);
    const far = kernel.placeShape(inside, { translation: [10, 0, 0], rotation: [0, 0, 0] });
    const rejects = (operation: () => unknown) => { try { operation(); return false; } catch { return true; } };
    const rejected = { noOpCut: rejects(() => kernel.cut(base, far)), emptyCut: rejects(() => kernel.cut(base, covering)), disconnectedJoin: rejects(() => kernel.fuse(base, far)), containedJoin: rejects(() => kernel.fuse(base, inside)) };
    const contractPath = "/src/cad/kernel/KernelAdapter.ts";
    const { HoleScopeError } = await import(contractPath);
    const farBase = kernel.placeShape(base, { translation: [10, 0, 0], rotation: [0, 0, 0] });
    const missedBase = kernel.placeShape(base, { translation: [20, 0, 0], rotation: [0, 0, 0] });
    const missedTool = kernel.placeShape(inside, { translation: [20, 0, 0], rotation: [0, 0, 0] });
    const scopeError = (targets: any[], tools: any[]) => {
      try { for (const output of kernel.cutScope(targets, tools)) kernel.disposeShape(output); return null; }
      catch (error) {
        if (!(error instanceof HoleScopeError)) return { unexpected: String(error) };
        const scoped = error as Error & { scope: "center" | "target"; index: number };
        return { scope: scoped.scope, index: scoped.index, message: scoped.message };
      }
    };
    const crossed = kernel.cutScope([base, farBase], [inside, far]);
    const crossedVolumes = crossed.map((output: any) => output.kernelHandle.proof.geometryAssertions.volume);
    for (const output of crossed) kernel.disposeShape(output);
    const missedCenter = scopeError([base, farBase], [inside, far, missedTool]);
    // Two output handles exist before target 3 fails. After repeated rollback,
    // fill the remaining handle capacity: any leaked probe/output prevents this.
    const rollbackErrors = [];
    for (let i = 0; i < 3; i++) rollbackErrors.push(scopeError([base, farBase, missedBase], [inside, far]));
    const capacityShapes = [];
    let capacityOverflowRejected = false;
    try {
      // Exactly seven caller-owned inputs are live at this point.
      for (let i = 0; i < 249; i++) capacityShapes.push(kernel.createBox(1, 1, 1));
      capacityOverflowRejected = rejects(() => kernel.createBox(1, 1, 1));
    } finally { for (const input of capacityShapes) kernel.disposeShape(input); }
    const retainedVolumes = [base, farBase, missedBase, inside, far, missedTool].map((input: any) => kernel.tessellate(input, { linearDeflection: .1, angularDeflection: .2 }).geometryAssertions.volume);
    for (const input of [farBase, missedBase, missedTool]) kernel.disposeShape(input);
    const large = kernel.createBox(10000, 10000, 10000);
    const symmetricScope = [kernel.hasCommonVolume(large, inside), kernel.hasCommonVolume(inside, large)];
    const width = 2.000002, height = 2.000002;
    const points = [{x:0,y:0},{x:width,y:0},{x:width,y:height},{x:0,y:height}];
    const profile = { id: "probe", sketchId: "probe", signature: "probe", outerLoop: { type: "polygon", entityIds: [], segments: points.map((start,i) => ({ type: "line", id: String(i), start, end: points[(i+1)%4] })) }, innerLoops: [], holes: [], bounds: { minX: 0, maxX: width, minY: 0, maxY: height } };
    const tinyCavity = kernel.extrudeProfile(profile, 1.000002);
    const tinyCavityRejected = rejects(() => kernel.shell(tinyCavity, "endCap", 1));
    const union = kernel.unionForExport(base, far);
    const unionProof = kernel.tessellate(union, { linearDeflection: .1, angularDeflection: .2 }).geometryAssertions;
    const circleProfile = { id: "circle-probe", sketchId: "probe", signature: "circle-probe", outerLoop: { type: "circle", entityIds: ["circle"] }, innerLoops: [], holes: [], bounds: { minX: 2, maxX: 6, minY: 2, maxY: 6 } };
    const cylinder = kernel.extrudeProfile(circleProfile, 3);
    const cylinderMesh = kernel.tessellate(cylinder, { linearDeflection: .1, angularDeflection: .1 });
    const cornerBox = (width: number, height: number, depth: number) => {
      const centered = kernel.createBox(width, height, depth);
      try { return kernel.placeShape(centered, { translation: [width / 2, height / 2, 0], rotation: [0, 0, 0] }); }
      finally { kernel.disposeShape(centered); }
    };
    const boreBase = cornerBox(8, 8, 3);
    const bored = kernel.cut(boreBase, cylinder);
    const boreProof = kernel.tessellate(bored, { linearDeflection: .1, angularDeflection: .1 }).geometryAssertions;
    const signedVolume = (mesh: any) => {
      let volume = 0;
      for (let i = 0; i < mesh.indices.length; i += 3) {
        const [a, b, c] = mesh.indices.slice(i, i + 3).map((index: number) => mesh.positions.slice(index * 3, index * 3 + 3));
        volume += (a[0] * (b[1]*c[2]-b[2]*c[1]) - a[1] * (b[0]*c[2]-b[2]*c[0]) + a[2] * (b[0]*c[1]-b[1]*c[0])) / 6;
      }
      return volume;
    };
    const boreMeshVolume = signedVolume(kernel.tessellate(bored, { linearDeflection: .5, angularDeflection: .2 }));
    const shiftedCircle = kernel.placeShape(cylinder, { translation: [1, 0, 0], rotation: [0, 0, 0] });
    const containedCircle = kernel.extrudeProfile({ ...circleProfile, bounds: { minX: 3, maxX: 5, minY: 3, maxY: 5 } }, 3);
    const overlapShapes = [
      ...kernel.cutScope([boreBase], [cylinder, cylinder]),
      ...kernel.cutScope([boreBase], [cylinder, shiftedCircle, containedCircle]),
      ...kernel.cutScope([boreBase], [containedCircle, shiftedCircle, cylinder]),
    ];
    const overlapProofs = overlapShapes.map((shape: any) => kernel.tessellate(shape, { linearDeflection: .5, angularDeflection: .2 }).geometryAssertions);
    const overlapMeshVolumes = overlapShapes.map((shape: any) => signedVolume(kernel.tessellate(shape, { linearDeflection: .5, angularDeflection: .2 })));
    let overlapStlNormalsValid = true;
    const stlVolume = (bytes: ArrayBuffer) => {
      const view = new DataView(bytes);
      let volume = 0;
      for (let i = 0; i < view.getUint32(80, true); i++) {
        const p = Array.from({length: 9}, (_, j) => view.getFloat32(96 + i * 50 + j * 4, true));
        const normal = Array.from({length: 3}, (_, j) => view.getFloat32(84 + i * 50 + j * 4, true));
        const ab = [p[3]-p[0], p[4]-p[1], p[5]-p[2]], ac = [p[6]-p[0], p[7]-p[1], p[8]-p[2]];
        const cross = [ab[1]*ac[2]-ab[2]*ac[1], ab[2]*ac[0]-ab[0]*ac[2], ab[0]*ac[1]-ab[1]*ac[0]];
        overlapStlNormalsValid &&= [...p, ...normal].every(Number.isFinite) && normal.reduce((sum, value, axis) => sum + value * cross[axis], 0) > 0;
        volume += (p[0]*(p[4]*p[8]-p[5]*p[7])-p[1]*(p[3]*p[8]-p[5]*p[6])+p[2]*(p[3]*p[7]-p[4]*p[6]))/6;
      }
      return volume;
    };
    const overlapStlVolumes = overlapShapes.map((shape: any) => stlVolume(kernel.exportStl(shape)));
    const strictNoOpRejected = rejects(() => kernel.cut(overlapShapes[0], cylinder));
    const docsPath = "/src/cad/document/CadDocument.ts", sketchPath = "/src/cad/sketch/SketchModel.ts";
    const profilePath = "/src/cad/sketch/profileDetection.ts", solvePath = "/src/cad/sketch/SketchSolver.ts";
    const rebuildPath = "/src/cad/features/rebuildGraph.ts";
    const docs = await import(docsPath), sketches = await import(sketchPath);
    const { detectProfiles } = await import(profilePath), { solveSketch } = await import(solvePath);
    const { rebuildDocument } = await import(rebuildPath);
    const section = sketches.addCornerRectangle(sketches.createXySketch("Overlap stock"), "8mm", "8mm");
    const extrusion = docs.createExtrudeFeature({ name: "Stock", sketchId: section.id, profileId: detectProfiles(solveSketch(section, {})).profiles[0].id, operation: "newBody", direction: "positive", distance: { expression: "3mm", unit: "mm" } });
    let overlapDocument = docs.upsertFeature(docs.upsertSketch(docs.createEmptyDocument("Scoped overlap"), section), extrusion);
    const firstCenter = sketches.addPoint(sketches.createXySketch("Centers"), "4mm", "4mm");
    const secondCenter = sketches.addPoint(firstCenter.sketch, "5mm", "4mm");
    overlapDocument = docs.upsertFeature(docs.upsertSketch(overlapDocument, secondCenter.sketch), { id: "overlap-hole", name: "Overlapping scoped hole", type: "hole", sketchId: secondCenter.sketch.id, centerPointIds: [firstCenter.pointId, secondCenter.pointId], targetBodyIds: ["body:" + extrusion.id], diameter: { expression: "4mm", unit: "mm" }, depth: "throughAll" });
    const rebuiltOverlap = rebuildDocument(overlapDocument);
    const rebuiltOverlapProof = { success: rebuiltOverlap.success, errors: rebuiltOverlap.errors, source: rebuiltOverlap.meshes[0]?.geometrySource, assertions: rebuiltOverlap.meshes[0]?.geometryAssertions, disposalFailures: rebuiltOverlap.metrics?.disposalFailures };
    const serializePath = "/src/persistence/exportProject.ts", reopenPath = "/src/persistence/importProject.ts";
    const { serializeProject } = await import(serializePath), { importProjectText } = await import(reopenPath);
    const reopenedOverlap = rebuildDocument(importProjectText(serializeProject(overlapDocument)));
    const reopenedOverlapProof = { success: reopenedOverlap.success, errors: reopenedOverlap.errors, source: reopenedOverlap.meshes[0]?.geometrySource, assertions: reopenedOverlap.meshes[0]?.geometryAssertions, disposalFailures: reopenedOverlap.metrics?.disposalFailures };
    const stlPath = "/src/cad/kernel/stlExport.ts", { exportMeshesToStl } = await import(stlPath);
    const sequentialProofs = [];
    for (const reverse of [false, true]) {
      const originalHole = overlapDocument.features.find((feature: any) => feature.id === "overlap-hole");
      const through = { ...originalHole, centerPointIds: [firstCenter.pointId], depth: "throughAll" };
      const shallowHole = { ...originalHole, centerPointIds: [secondCenter.pointId], depth: { expression: "2mm", unit: "mm" } };
      const first = reverse ? shallowHole : through, second = reverse ? through : shallowHole;
      let document = docs.upsertFeature({ ...overlapDocument, features: overlapDocument.features.filter((feature: any) => feature.id === extrusion.id) }, first);
      document = docs.upsertFeature(document, { ...second, id: "second-depth-hole", timelineStep: undefined });
      for (const candidate of [document, importProjectText(serializeProject(document))]) {
        const result = rebuildDocument(candidate);
        sequentialProofs.push({ success: result.success, errors: result.errors, source: result.meshes[0]?.geometrySource, assertions: result.meshes[0]?.geometryAssertions, disposalFailures: result.metrics?.disposalFailures, meshVolume: signedVolume(result.meshes[0]), stlVolume: stlVolume(exportMeshesToStl(result.meshes)) });
      }
    }
    const shallow = kernel.extrudeProfile(circleProfile, 2);
    const farBoreBase = kernel.placeShape(boreBase, { translation: [20, 0, 0], rotation: [0, 0, 0] });
    const farCylinder = kernel.placeShape(cylinder, { translation: [20, 0, 0], rotation: [0, 0, 0] });
    const shiftedShallow = kernel.placeShape(shallow, { translation: [1, 0, 0], rotation: [0, 0, 0] });
    const unequalDepthOutputs = kernel.cutScope([farBoreBase, boreBase], [farCylinder, cylinder, shiftedShallow]);
    const reversedDepthOutputs = kernel.cutScope([boreBase], [shiftedShallow, cylinder]);
    const unequalDepthProofs = [...unequalDepthOutputs, ...reversedDepthOutputs].map((shape: any) => kernel.tessellate(shape, { linearDeflection: .5, angularDeflection: .2 }).geometryAssertions);
    const unequalDepthMeshVolumes = [...unequalDepthOutputs, ...reversedDepthOutputs].map((shape: any) => signedVolume(kernel.tessellate(shape, { linearDeflection: .5, angularDeflection: .2 })));
    const unequalDepthStlVolumes = [...unequalDepthOutputs, ...reversedDepthOutputs].map((shape: any) => stlVolume(kernel.exportStl(shape)));
    const tangentStock = cornerBox(12, 8, 3);
    const tangentCylinder = kernel.placeShape(cylinder, { translation: [4, 0, 0], rotation: [0, 0, 0] });
    const tangentFailure = scopeError([tangentStock], [cylinder, tangentCylinder]);
    const lateOverlapFailure = scopeError([farBoreBase, tangentStock], [farCylinder, cylinder, tangentCylinder]);
    const overlapRetainedVolumes = [farBoreBase, boreBase, tangentStock].map((shape: any) => kernel.tessellate(shape, { linearDeflection: .5, angularDeflection: .2 }).geometryAssertions.volume);
    const blind = kernel.cut(boreBase, shallow);
    const blindProof = kernel.tessellate(blind, { linearDeflection: .1, angularDeflection: .1 }).geometryAssertions;
    const blindMeshVolume = signedVolume(kernel.tessellate(blind, { linearDeflection: .5, angularDeflection: .2 }));
    const oppositeTool = kernel.placeShape(shallow, { translation: [0, 0, 1], rotation: [0, 0, 0] });
    const oppositeBlind = kernel.cut(boreBase, oppositeTool);
    const oppositeBlindProof = kernel.tessellate(oppositeBlind, { linearDeflection: .1, angularDeflection: .1 }).geometryAssertions;
    const internalTool = kernel.placeShape(shallow, { translation: [0, 0, .5], rotation: [0, 0, 0] });
    const internalCutRejected = rejects(() => kernel.cut(boreBase, internalTool));
    const ring = kernel.extrudeProfile({ ...circleProfile, innerLoops: [{ type: "circle", entityIds: ["inner"] }], holes: [{ id: "inner", x: 4, y: 4, radius: 1 }] }, 3);
    const ringProof = kernel.tessellate(ring, { linearDeflection: .05, angularDeflection: .1 }).geometryAssertions;
    kernel.disposeShape(ring);
    const edgePoints = [{x:0,y:0},{x:20,y:0},{x:20,y:10},{x:0,y:10}];
    const edgeProfile = { ...profile, id: "edge-stock", signature: "edge-stock", bounds: { minX: 0, minY: 0, maxX: 20, maxY: 10 }, outerLoop: { type: "polygon", entityIds: [], segments: edgePoints.map((start, i) => ({ type: "line", id: "authored-" + i, start, end: edgePoints[(i+1)%4] })) } };
    const edgeStock = kernel.extrudeProfile(edgeProfile, 10);
    const edgeRef = { featureId: "edge-stock", kind: "edge", transientId: "edge", role: "endCapPerimeter", sourceEntityId: "authored-0" };
    const edgeBefore = kernel.availableExtrudeCapEdges(edgeStock);
    const filleted = kernel.fillet(edgeStock, [edgeRef], 1);
    const edgeAfterFillet = kernel.availableExtrudeCapEdges(filleted);
    const repeatedEdgeRejected = rejects(() => kernel.fillet(filleted, [edgeRef], 1));
    const chamfered = kernel.chamfer(filleted, [{ ...edgeRef, role: "startCapPerimeter" }], 1);
    const edgeAfterChamfer = kernel.availableExtrudeCapEdges(chamfered);
    const edgeMeshes = [filleted, chamfered].map((shape: any) => kernel.tessellate(shape, { linearDeflection: .5, angularDeflection: .2 }));
    const edgeProofs = edgeMeshes.map((mesh: any) => ({ assertions: mesh.geometryAssertions, source: mesh.geometrySource, operation: mesh.kernelOperation, volume: signedVolume(mesh) }));
    const frontStart = { ...edgeRef, role: "startCapPerimeter" };
    const backStart = { ...frontStart, sourceEntityId: "authored-2" };
    const batchChamfers = [
      kernel.chamfer(edgeStock, [frontStart, backStart], 1),
      kernel.chamfer(edgeStock, [backStart, frontStart], 1),
      kernel.chamfer(edgeStock, [frontStart, frontStart], 1),
    ];
    const batchChamferProofs = batchChamfers.map((shape: any) => {
      const mesh = kernel.tessellate(shape, { linearDeflection: .5, angularDeflection: .2 });
      return { assertions: mesh.geometryAssertions, operation: mesh.kernelOperation, volume: signedVolume(mesh), stlVolume: stlVolume(kernel.exportStl(shape)) };
    });
    const edgeBatchRejected = {
      mixedOwners: rejects(() => kernel.chamfer(edgeStock, [frontStart, { ...backStart, featureId: "other-owner" }], 1)),
      nonparallel: rejects(() => kernel.chamfer(edgeStock, [frontStart, { ...frontStart, sourceEntityId: "authored-1" }], 1)),
      lostSecond: rejects(() => kernel.fillet(batchChamfers[0], [edgeRef, frontStart], 1)),
    };
    const edgeBatchInputProof = kernel.tessellate(batchChamfers[0], { linearDeflection: .5, angularDeflection: .2 }).geometryAssertions;
    const untouchedEndSurvives = kernel.availableExtrudeCapEdges(batchChamfers[0]).some((edge: any) => edge.role === "endCapPerimeter" && edge.sourceEntityId === "authored-0");
    const xyPlane = { origin: { x: 0, y: 0, z: 0 }, u: { x: 1, y: 0, z: 0 }, v: { x: 0, y: 1, z: 0 }, normal: { x: 0, y: 0, z: 1 } };
    const targetCap = { ...xyPlane, origin: { x: 0, y: 0, z: 10 } };
    const finiteCircle = kernel.extrudeToFace(circleProfile, xyPlane, edgeStock, targetCap);
    const finitePolygon = kernel.extrudeToFace(profile, xyPlane, edgeStock, targetCap);
    const toFaceProofs = [finiteCircle, finitePolygon].map((shape: any) => {
      const mesh = kernel.tessellate(shape, { linearDeflection: .05, angularDeflection: .1 });
      return { assertions: mesh.geometryAssertions, operation: mesh.kernelOperation, volume: signedVolume(mesh), stlVolume: stlVolume(kernel.exportStl(shape)) };
    });
    const referenceCircle = kernel.extrudeProfile(circleProfile, 10);
    const toFaceDefaultStlMatchesCircle = stlVolume(kernel.exportStl(referenceCircle)) === stlVolume(kernel.exportStl(finiteCircle));
    kernel.disposeShape(referenceCircle);
    const toFaceRejected = {
      zero: rejects(() => kernel.extrudeToFace(circleProfile, xyPlane, edgeStock, xyPlane)),
      behind: rejects(() => kernel.extrudeToFace(circleProfile, { ...xyPlane, origin: { x: 0, y: 0, z: 11 } }, edgeStock, targetCap)),
      uncovered: rejects(() => kernel.extrudeToFace(circleProfile, { ...xyPlane, origin: { x: 20, y: 0, z: 0 } }, edgeStock, targetCap)),
      missing: rejects(() => kernel.extrudeToFace(circleProfile, xyPlane, edgeStock, { ...targetCap, origin: { x: 0, y: 0, z: 9 } })),
    };
    const toFaceTargetVolume = kernel.tessellate(edgeStock, { linearDeflection: .5, angularDeflection: .2 }).geometryAssertions.volume;
    kernel.disposeShape(finiteCircle); kernel.disposeShape(finitePolygon);
    for (const item of [edgeStock, filleted, chamfered, ...batchChamfers]) kernel.disposeShape(item);
    for (const item of [base, inside, covering, far, union, large, tinyCavity, cylinder, boreBase, bored, shiftedCircle, containedCircle, ...overlapShapes, shallow, farBoreBase, farCylinder, shiftedShallow, ...unequalDepthOutputs, ...reversedDepthOutputs, tangentStock, tangentCylinder, blind, oppositeTool, oppositeBlind, internalTool]) kernel.disposeShape(item);
    return { toFaceDefaultStlMatchesCircle, toFaceProofs, toFaceRejected, toFaceTargetVolume, batchChamferProofs, edgeBatchRejected, edgeBatchInputProof, untouchedEndSurvives, edgeBefore, edgeAfterFillet, edgeAfterChamfer, repeatedEdgeRejected, edgeProofs, sequentialProofs, unequalDepthProofs, unequalDepthMeshVolumes, unequalDepthStlVolumes, lateOverlapFailure, tangentFailure, overlapRetainedVolumes, overlapStlNormalsValid, reopenedOverlapProof, rebuiltOverlapProof, overlapProofs, overlapMeshVolumes, overlapStlVolumes, strictNoOpRejected, boreMeshVolume, blindMeshVolume, oppositeBlindProof, capacityCount: capacityShapes.length, capacityOverflowRejected, blindProof, internalCutRejected, crossedVolumes, missedCenter, rollbackErrors, retainedVolumes, cylinderProof: cylinderMesh.geometryAssertions, cylinderBounds: cylinderMesh.bounds, boreProof, ringProof, symmetricScope, tinyCavityRejected, rejected, unionProof, dispatcher: typeof (globalThis as any).plainSolidCall, source: mesh.geometrySource, proof: mesh.geometryAssertions };
  });
  expect(proof.dispatcher).toBe("function");
  expect(proof.source).toBe("plainsolid");
  expect(proof.toFaceDefaultStlMatchesCircle).toBe(true);
  expect(proof.toFaceProofs).toHaveLength(2);
  proof.toFaceProofs.forEach((item: any, index: number) => {
    const expected = index === 0 ? 40 * Math.PI : 10 * 2.000002 * 2.000002;
    expect(item).toMatchObject({ operation: "toFace", assertions: { valid: true, solidCount: 1 } });
    expect(item.assertions.volume).toBeCloseTo(expected, 8);
    expect(Math.abs(item.volume - expected) / expected).toBeLessThan(.01);
    expect(Math.abs(item.stlVolume - expected) / expected).toBeLessThan(.03);
  });
  expect(proof.toFaceRejected).toEqual({ zero: true, behind: true, uncovered: true, missing: true });
  expect(proof.toFaceTargetVolume).toBe(2000);
  expect(proof.edgeBefore).toEqual(expect.arrayContaining([{ role: "endCapPerimeter", sourceEntityId: "authored-0" }, { role: "startCapPerimeter", sourceEntityId: "authored-0" }]));
  expect(proof.edgeAfterFillet).not.toContainEqual({ role: "endCapPerimeter", sourceEntityId: "authored-0" });
  expect(proof.edgeAfterFillet).toContainEqual({ role: "startCapPerimeter", sourceEntityId: "authored-0" });
  expect(proof.edgeAfterChamfer).not.toContainEqual({ role: "startCapPerimeter", sourceEntityId: "authored-0" });
  expect(proof.repeatedEdgeRejected).toBe(true);
  expect(proof.batchChamferProofs).toHaveLength(3);
  proof.batchChamferProofs.forEach((item: any, index: number) => {
    const expected = index === 2 ? 1990 : 1980;
    expect(item).toMatchObject({ operation: "chamfer", assertions: { valid: true, solidCount: 1 } });
    expect(item.assertions.volume).toBeCloseTo(expected, 8);
    expect(item.volume).toBeCloseTo(expected, 5);
    expect(item.stlVolume).toBeCloseTo(expected, 4);
  });
  expect(proof.edgeBatchRejected).toEqual({ mixedOwners: true, nonparallel: true, lostSecond: true });
  expect(proof.edgeBatchInputProof.volume).toBeCloseTo(1980, 8);
  expect(proof.untouchedEndSurvives).toBe(true);
  expect(proof.edgeProofs).toHaveLength(2);
  proof.edgeProofs.forEach((item: any, index: number) => {
    const volume = 2000 - 20 * (1 - Math.PI / 4) - (index ? 10 : 0);
    expect(item).toMatchObject({ source: "plainsolid", operation: index ? "chamfer" : "fillet", assertions: { valid: true, solidCount: 1 } });
    expect(item.assertions.volume).toBeCloseTo(volume, 8);
    expect(Math.abs(item.volume - volume) / volume).toBeLessThan(.005);
  });
  expect(proof.crossedVolumes).toEqual([23, 23]);
  expect(proof.missedCenter).toMatchObject({ scope: "center", index: 2 });
  expect(proof.missedCenter?.message).toContain("removed no volume");
  expect(proof.rollbackErrors).toHaveLength(3);
  expect(proof.capacityCount).toBe(249);
  expect(proof.capacityOverflowRejected).toBe(true);
  for (const error of proof.rollbackErrors) {
    expect(error).toMatchObject({ scope: "target", index: 2 });
    expect(error?.message).toContain("removed no volume");
  }
  expect(proof.retainedVolumes).toEqual([24, 24, 24, 1, 1, 1]);
  expect(proof.proof).toMatchObject({ valid: true, volume: 24, solidCount: 1 });
  expect(proof.cylinderProof).toMatchObject({ valid: true, solidCount: 1 });
  expect(proof.cylinderProof.volume).toBeCloseTo(12 * Math.PI, 8);
  expect(proof.cylinderProof.surfaceArea).toBeCloseTo(20 * Math.PI, 8);
  for (const [side, expected] of [["min", [2, 2, 0]], ["max", [6, 6, 3]]] as const) {
    for (let axis = 0; axis < 3; axis++) {
      const value = proof.cylinderBounds[side][axis];
      expect(Math.abs(value - expected[axis])).toBeLessThan(1e-8);
      if (side === "min") expect(value).toBeLessThanOrEqual(expected[axis]);
      else expect(value).toBeGreaterThanOrEqual(expected[axis]);
    }
  }
  expect(proof.boreProof).toMatchObject({ valid: true, solidCount: 1 });
  const lens = 8 * Math.acos(.25) - .5 * Math.sqrt(15);
  const unionArea = 8 * Math.PI - lens;
  const unionPerimeter = 8 * (Math.PI - Math.acos(.25));
  expect(proof.rebuiltOverlapProof).toMatchObject({ success: true, errors: [], source: "plainsolid", disposalFailures: 0 });
  expect(proof.rebuiltOverlapProof.assertions.volume).toBeCloseTo(192 - 3 * unionArea, 8);
  expect(proof.reopenedOverlapProof).toMatchObject({ success: true, errors: [], source: "plainsolid", disposalFailures: 0 });
  expect(proof.reopenedOverlapProof.assertions.volume).toBeCloseTo(192 - 3 * unionArea, 8);
  const removedAreas = [4 * Math.PI, unionArea, unionArea];
  expect(proof.overlapProofs).toHaveLength(3);
  proof.overlapProofs.forEach((item: any, index: number) => {
    expect(item).toMatchObject({ valid: true, solidCount: 1 });
    expect(item.volume).toBeCloseTo(192 - 3 * removedAreas[index], 8);
    expect(item.surfaceArea).toBeCloseTo(index === 0 ? 224 + 4 * Math.PI : 224 - 2 * unionArea + 3 * unionPerimeter, 8);
    expect(Math.abs(proof.overlapMeshVolumes[index] - item.volume)).toBeLessThan(3 * removedAreas[index] * .02);
    expect(Math.abs(proof.overlapStlVolumes[index] - item.volume)).toBeLessThan(3 * removedAreas[index] * .02);
    expect(proof.overlapStlVolumes[index]).toBeGreaterThan(0);
  });
  expect(proof.strictNoOpRejected).toBe(true);
  expect(proof.overlapStlNormalsValid).toBe(true);
  const unequalRemoved = [12 * Math.PI, 20 * Math.PI - 2 * lens, 20 * Math.PI - 2 * lens];
  expect(proof.sequentialProofs).toHaveLength(4);
  proof.sequentialProofs.forEach((item: any) => {
    expect(item).toMatchObject({ success: true, errors: [], source: "plainsolid", disposalFailures: 0 });
    expect(item.assertions.volume).toBeCloseTo(192 - unequalRemoved[1], 8);
    expect(item.assertions.surfaceArea).toBeCloseTo(224 - 4 * Math.PI + 2 * unionPerimeter, 8);
    expect(Math.abs(item.meshVolume - item.assertions.volume)).toBeLessThan(unequalRemoved[1] * .02);
    expect(Math.abs(item.stlVolume - item.assertions.volume)).toBeLessThan(unequalRemoved[1] * .02);
  });
  expect(proof.unequalDepthProofs).toHaveLength(3);
  proof.unequalDepthProofs.forEach((item: any, index: number) => {
    expect(item).toMatchObject({ valid: true, solidCount: 1 });
    expect(item.volume).toBeCloseTo(192 - unequalRemoved[index], 8);
    expect(item.surfaceArea).toBeCloseTo(index === 0 ? 224 + 4 * Math.PI : 224 - 4 * Math.PI + 2 * unionPerimeter, 8);
    expect(Math.abs(proof.unequalDepthMeshVolumes[index] - item.volume)).toBeLessThan(unequalRemoved[index] * .02);
    expect(Math.abs(proof.unequalDepthStlVolumes[index] - item.volume)).toBeLessThan(unequalRemoved[index] * .02);
  });
  expect(proof.lateOverlapFailure).toMatchObject({ scope: "target", index: 1 });
  expect(proof.tangentFailure).toMatchObject({ scope: "target", index: 0 });
  expect(proof.overlapRetainedVolumes).toEqual([192, 192, 288]);
  expect(proof.boreProof.volume).toBeCloseTo(192 - 12 * Math.PI, 8);
  expect(proof.boreProof.surfaceArea).toBeCloseTo(224 + 4 * Math.PI, 8);
  expect(Math.abs(proof.boreMeshVolume - proof.boreProof.volume)).toBeLessThan(12 * Math.PI * .02);
  expect(proof.blindProof).toMatchObject({ valid: true, solidCount: 1 });
  expect(proof.blindProof.volume).toBeCloseTo(192 - 8 * Math.PI, 8);
  expect(proof.blindProof.surfaceArea).toBeCloseTo(224 + 8 * Math.PI, 8);
  expect(Math.abs(proof.blindMeshVolume - proof.blindProof.volume)).toBeLessThan(8 * Math.PI * .02);
  expect(proof.oppositeBlindProof).toMatchObject({ valid: true, solidCount: 1 });
  expect(proof.oppositeBlindProof.volume).toBeCloseTo(proof.blindProof.volume, 8);
  expect(proof.oppositeBlindProof.surfaceArea).toBeCloseTo(proof.blindProof.surfaceArea, 8);
  expect(proof.internalCutRejected).toBe(true);
  expect(proof.ringProof).toMatchObject({ valid: true, solidCount: 1 });
  expect(proof.ringProof.volume).toBeCloseTo(9 * Math.PI, 8);
  expect(proof.symmetricScope).toEqual([true, true]);
  expect(proof.tinyCavityRejected).toBe(true);
  expect(proof.rejected).toEqual({ noOpCut: true, emptyCut: true, disconnectedJoin: true, containedJoin: true });
  expect(proof.unionProof).toMatchObject({ valid: true, volume: 25, solidCount: 2 });
  expect(forbidden).toEqual([]);
});\n`);
  await command("node", [join(source, "node_modules/typescript/bin/tsc"), "-b", "--noEmit"], "typecheck.log");
  await command("node", [join(source, "node_modules/typescript/bin/tsc"), "-p", "tsconfig.e2e.json"], "typecheck-e2e.log");
  report.typecheck = "passed";
  // Fail the backend preflight before spending time on application fixtures.
  const resultPaths = [];
  try {
    const preflight = join(work, "backend-results.json");
    resultPaths.push(preflight);
    await command("node", [join(source, "node_modules/@playwright/test/cli.js"), "test", "e2e/plainsolid-backend.spec.ts", "--reporter=list,json"], "backend.log", { PLAYWRIGHT_JSON_OUTPUT_FILE: preflight });
    const acceptance = join(work, "playwright-results.json");
    resultPaths.push(acceptance);
    await command("node", [join(source, "node_modules/@playwright/test/cli.js"), "test", testFile, "--grep", grep, "--reporter=list,json"], "acceptance.log", { PLAYWRIGHT_JSON_OUTPUT_FILE: acceptance });
    report.status = "passed";
  } catch (error) { report.status = "failed"; report.error = String(error); }
  try {
    const visit = suite => {
      for (const spec of suite.specs ?? []) for (const test of spec.tests ?? []) report.tests.push({ file: spec.file, line: spec.line, title: spec.title, status: test.status, results: test.results.map(result => ({ status: result.status, durationMs: result.duration, errors: result.errors })) });
      for (const child of suite.suites ?? []) visit(child);
    };
    report.summaries = [];
    for (const path of resultPaths) {
      const results = JSON.parse(await readFile(path, "utf8"));
      for (const suite of results.suites ?? []) visit(suite);
      report.summaries.push({ path, ...results.stats });
    }
    const passed = test => test.status === "expected" && test.results.length > 0 && test.results.every(result => result.status === "passed");
    report.backendVerified = report.tests.some(test => test.title.startsWith("PlainSolid backend:") && passed(test));
    const existing = report.tests.filter(test => !test.title.startsWith("PlainSolid backend:"));
    if (!report.backendVerified || existing.length === 0 || existing.some(test => !passed(test))) {
      report.status = "failed";
      report.error ??= "A passing real backend probe and actual passes for every selected existing fixture are required; skips and interruptions do not count.";
    }
  } catch (error) { report.status = "failed"; report.error ??= `Cannot read test results: ${error}`; }
} catch (error) {
  report.status = "setup_failed";
  report.error = String(error);
}
report.finishedAt = new Date().toISOString();
await writeFile(join(work, "readiness.json"), JSON.stringify(report, null, 2));
console.log(`PlainSolid readiness: ${report.status}; ${join(work, "readiness.json")}`);
if (report.status !== "passed") { console.error(report.error); process.exitCode = 1; }
