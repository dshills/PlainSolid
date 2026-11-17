import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { constants, cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const runner = fileURLToPath(import.meta.url);
const tools = dirname(runner);
const repo = resolve(tools, "../..");
const appRevision = "ff9287dd57303bb416fbaf8226902133ef013e99";
const supplemental = [
  "11-trapezoidal-lead-screw",
  "12-acme-lead-screw",
  "13-m6-threaded-pair",
  "14-quarter-inch-unc",
  "orbit-drive-housing",
];
const usage = "node tools/plaincad/gallery.mjs --app PLAINCAD_CHECKOUT --work NEW_DIRECTORY --kernel-revision COMMIT --select 03,07,08,09";

function options(args) {
  const names = new Set(["--app", "--work", "--kernel-revision", "--select"]);
  const result = new Map();
  for (let i = 0; i < args.length; i += 2) {
    const [name, value] = args.slice(i, i + 2);
    if (!names.has(name) || result.has(name) || !value || value.startsWith("--")) throw new Error(usage);
    result.set(name, value);
  }
  if (result.size !== names.size || !result.get("--select").split(",").every(token => token.trim())) throw new Error(usage);
  return result;
}

function run(cwd, executable, args, env = process.env) {
  const result = spawnSync(executable, args, { cwd, env, encoding: "utf8", maxBuffer: 16 * 1024 * 1024 });
  if (result.error) throw result.error;
  return result;
}

function checked(cwd, executable, args, env) {
  const result = run(cwd, executable, args, env);
  if (result.status !== 0) throw new Error(`${executable} ${args.join(" ")} failed\n${result.stdout}${result.stderr}`);
  return result.stdout.trim();
}

function archive(cwd, revision, target) {
  const file = `${target}.tar`;
  mkdirSync(target);
  checked(cwd, "git", ["archive", "--format=tar", "--output", file, revision]);
  checked(target, "tar", ["-xf", file]);
  rmSync(file);
}

function sha256(bytes) {
  return createHash("sha256").update(bytes).digest("hex");
}

function fixtureHashes(app, slugs) {
  const files = ["VALIDATION.json", ...slugs.map(slug => `${slug}.pcaddoc`)];
  return Object.fromEntries(files.map(file => [file, sha256(readFileSync(join(app, "docs/examples", file)))]));
}

function selections(value, worklist) {
  return [...new Set(value.split(",").map(token => {
    const item = token.trim();
    const exact = worklist.find(slug => slug === item || slug.split("-")[0] === item.padStart(2, "0"));
    if (!exact) throw new Error(`Unknown gallery fixture ${item}. Available fixtures are ${worklist.join(", ")}.`);
    return exact;
  }))];
}

function qualify() {
  const args = options(process.argv.slice(2));
  const runnerSHA256 = sha256(readFileSync(runner));
  const appSource = resolve(args.get("--app"));
  const work = resolve(args.get("--work"));
  const dependencies = join(appSource, "node_modules");
  if (!existsSync(join(dependencies, "vitest/vitest.mjs"))) throw new Error("The PlainCAD checkout needs its installed npm dependencies.");
  const reference = JSON.parse(checked(appSource, "git", ["show", `${appRevision}:docs/examples/VALIDATION.json`]));
  const worklist = [...reference.projects.map(project => project.slug), ...supplemental];
  const selected = selections(args.get("--select"), worklist);
  const kernelRevision = checked(repo, "git", ["rev-parse", "--verify", "--end-of-options", `${args.get("--kernel-revision")}^{commit}`]);
  mkdirSync(work);
  const app = join(work, "app");
  const kernel = join(work, "kernel");
  archive(appSource, appRevision, app);
  const originals = fixtureHashes(app, worklist);

  archive(repo, kernelRevision, kernel);
  cpSync(dependencies, join(app, "node_modules"), {
    recursive: true, verbatimSymlinks: true, mode: constants.COPYFILE_FICLONE,
  });

  const adapter = join(app, "src/cad/kernel/PlainSolidKernel.ts");
  const appConfig = join(app, "vite.config.ts");
  const runtime = join(app, "src/cad/kernel/wasm_exec.js");
  const wasm = join(app, "src/cad/kernel/plainsolid-kernel.wasm");
  const fixture = join(app, "src/tests/plainsolid-gallery.focus.test.ts");
  cpSync(join(tools, "gallery.fixture.ts"), fixture);

  const goRoot = checked(kernel, "go", ["env", "GOROOT"]);
  const runtimeSource = join(goRoot, "lib/wasm/wasm_exec.js");
  cpSync(existsSync(runtimeSource) ? runtimeSource : join(goRoot, "misc/wasm/wasm_exec.js"), runtime);
  const goArgs = ["build", "-trimpath", "-buildvcs=false", "-o", wasm, "./cmd/plaincadwasm"];
  const go = run(kernel, "go", goArgs, {
    ...process.env, GOOS: "js", GOARCH: "wasm", CGO_ENABLED: "0", GOMAXPROCS: process.env.GOMAXPROCS ?? "2",
    GOCACHE: process.env.GOCACHE ?? join(tmpdir(), "plainsolid-gallery-go-cache"),
  });
  writeFileSync(join(work, "wasm-build.log"), go.stdout + go.stderr);
  if (go.status !== 0) throw new Error(`The Go WASM build failed. See ${join(work, "wasm-build.log")}.`);

  const config = join(app, "vitest.gallery.config.ts");
  // The pinned application's ordinary Vitest config uses its OCCT provider.
  // Use the application's production NativeKernel resolution in this stage.
  writeFileSync(config, `import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";
const provider = fileURLToPath(new URL("./src/cad/kernel/PlainSolidKernel.ts", import.meta.url));
export default defineConfig({
  cacheDir: ".plainSolidGalleryCache",
  plugins: [{
    name: "plaincad-native-kernel",
    enforce: "pre",
    resolveId(source) {
      if (/(?:^|\\/)NativeKernel$/.test(source)) return provider;
    },
  }],
  test: { environment: "node", include: ["src/tests/plainsolid-gallery.focus.test.ts"] },
});
`);
  const resultFile = join(work, "vitest.json");
  const testArgs = [
    join(app, "node_modules/vitest/vitest.mjs"), "run", "--config", config,
    "--maxWorkers=1", "--reporter=verbose", "--reporter=json", `--outputFile.json=${resultFile}`,
  ];
  const test = run(app, process.execPath, testArgs, { ...process.env, PLAINSOLID_GALLERY: selected.join(",") });
  const log = test.stdout + test.stderr;
  writeFileSync(join(work, "vitest.log"), log);

  const after = fixtureHashes(app, worklist);
  if (JSON.stringify(originals) !== JSON.stringify(after)) throw new Error("The staged original gallery files changed.");
  const vitest = existsSync(resultFile) ? JSON.parse(readFileSync(resultFile, "utf8")) : null;
  const results = log.split("\n").filter(line => line.startsWith("gallery-proof "))
    .map(line => JSON.parse(line.slice("gallery-proof ".length)));
  const artifacts = Object.fromEntries(
    [adapter, appConfig, runtime, wasm, fixture, config, join(work, "vitest.log"), ...(vitest ? [resultFile] : [])]
      .map(file => [basename(file), sha256(readFileSync(file))]),
  );
  const everyCaseRebuilt = selected.every(slug => {
    const cases = results.filter(result => result.slug === slug);
    return cases.length === 2 && cases.every(result => result.status) && cases.filter(result => result.variant === "original").length === 1;
  });
  const passed = test.status === 0 && vitest?.success === true && everyCaseRebuilt &&
    vitest.numTotalTests === 2 * selected.length && vitest.numPassedTests === vitest.numTotalTests;
  const report = {
    schema: 1, backend: "plainsolid-go-wasm", appRevision, kernelRevision,
    goVersion: checked(kernel, "go", ["version"]), nodeVersion: process.version,
    occtReferenceAppRevision: reference.appCommit, selected, originals, artifacts, results, runnerSHA256,
    commands: {
      wasm: { cwd: kernel, args: ["go", ...goArgs], env: { GOOS: "js", GOARCH: "wasm", CGO_ENABLED: "0" } },
      test: { cwd: app, args: [process.execPath, ...testArgs], env: { PLAINSOLID_GALLERY: selected.join(",") } },
    },
    checks: { fixtureBytesUnchanged: true, everyCaseRebuilt, passed },
    vitest: { status: test.status, passed: vitest?.numPassedTests, failed: vitest?.numFailedTests },
  };
  const reportFile = join(work, "qualification.json");
  writeFileSync(reportFile, JSON.stringify(report, null, 2) + "\n");
  console.log(`PlainCAD gallery ${passed ? "passed" : "failed"}. Evidence is in ${reportFile}.`);
  process.exitCode = passed ? 0 : 1;
}

try {
  qualify();
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
