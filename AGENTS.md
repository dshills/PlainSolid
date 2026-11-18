# PlainSolid repository instructions

PlainSolid is a Go solid-modeling kernel. Its working goal is to be the kernel
that PlainCAD runs on, as defined by the gates in
[PlainCAD readiness](docs/PLAINCAD_READINESS.md). The full OCCT replacement in
[the specification](docs/SPECIFICATION.md) is a long-term direction, not a
task: work on a part of it only when the user asks for that part. If another
document in this repository conflicts with this file, this file wins.

## Scope

- Do the task the user gave, then stop and report what changed, what you ran,
  what you did not verify, and what you would do next. Do not start the next
  item unprompted. Earlier runs that chose their own next work added several
  hundred thousand lines that PlainCAD never calls.
- With no specific task, the next work is the first failing PlainCAD acceptance
  case. If none fails, say so and stop.
- A kernel capability is in scope when an operation in
  `internal/plaincadbridge` needs it. Anything else needs the user's request.
- Do not add a function, type or file family that handles one named
  arrangement of inputs, such as `CutParabolaThreeCirclesSplineGraphs`. If a
  task seems to need one, extend a general mechanism so the new case is data,
  or stop and describe the gap to the user. Enumerating arrangements does not
  converge on a kernel.
- A new capability does not automatically get native persistence, STEP and
  IGES. Add each only when the task needs it.
- A new exported identifier needs a caller outside its package in the same
  change.
- Prefer a new package to more files in `brep/trimmed`. That package has about
  2,000 files, so any edit to it reruns its tests and those of every package
  that imports it.
- Unsupported input returns an explicit error. Never return a plausible but
  incorrect shape, and never call a faceted approximation exact.
- The kernel uses the standard library only. Ask before adding a dependency.

## Verification budget

- For each change: `go build ./...`, `go vet ./...`, and `go test` on the
  packages you changed. Before merging to `main`: `go test ./...`. These run
  the fast tier described in [the CI notes](tools/ci/README.md#test-tiers).
- Run a slow test by name, with `PLAINSOLID_SLOW=1`, only when your change
  touches the code it covers.
- Run the complete suite, race runs, `tools/ci/run-go-qualification.py`,
  cross-platform builds and the PlainCAD browser gates only when the user asks.
  Never run them per commit: the complete Go suite is hours of machine time,
  and each qualification stage copies the whole tree. Delete a stage directory
  once you have reported its result.
- Write one test per behavior, table-driven where inputs vary. A new test
  finishes in under two seconds or starts with `slowtest.Skip(t)`.
- If a check fails for a reason unrelated to your change, report it and leave
  it alone.

## Records

- [The status page](docs/STATUS.md) and
  [the verification record](docs/VERIFICATION.md) say what is true now. Edit
  them in place when a fact on them changes. Never append dated entries or
  narrative, and keep each under about 150 lines. Their earlier narrative,
  about 1 MB, is in git history; each page gives the command to read it.
- The record of a change is its commit message: what changed, what was run,
  and what was not. Add a file under `docs/evidence` only for a complete run
  that the verification record then cites, and remove the file it replaces.
- If a change alters what the kernel supports, update the capability summary
  in [the README](README.md) and the matrix on the status page in a line or
  two.
- When resuming, read this file, the status page and `git log -15`. If work is
  interrupted, leave a commit labeled work in progress whose message says what
  remains.

## Git

- Use a separate `codex/` branch and Git worktree for each independent chat,
  and remove the worktree once its branch is merged. Leave another agent's
  checkout, branch and uncommitted work untouched.
- Use subagents only for independent parts of the task at hand. Give them
  disjoint files and review their combined changes before committing.
- Commit coherent, verified pieces. Stage only your own changes. A commit is a
  checkpoint, not an acceptance pass.
