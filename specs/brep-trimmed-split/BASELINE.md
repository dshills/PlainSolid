# Baseline for the `brep/trimmed` separation

Recorded on 2026-11-18 at `f51e7b95` (kernel code identical to `58c27fa4`),
on macOS ARM64, 14 cores. Later phases compare against these figures.

## Toolchain

`go version go1.27.4 darwin/arm64`; `GOFLAGS` and `GOEXPERIMENT` empty;
`CGO_ENABLED=1` for native builds.

## PlainCAD WASM

`GOOS=js GOARCH=wasm go build -trimpath -buildvcs=false ./cmd/plaincadwasm`,
built twice, each from an empty build cache:

- Both builds: `sha256 10a8a36495e417cf90559a7173f005e8174f73c0714560caaba7a6eb2085748c`
- With `-ldflags=-buildid=` added: 22,097,824 bytes,
  `sha256 62c434e885db3b5768daf32722daa224ea164c51d20d364fdbe31617faf7ac5c`

The build is reproducible.

## Linked code of the bridge

A native build of a `main` that calls `plaincadbridge.New().Call`:

- 7,138 text symbols; code section (`__text`) 6,554,116 bytes.
- With the 540 research files deleted by `go build -overlay`: the same 7,138
  symbols with the same sizes, and the same code-section size. Type
  descriptors shrink from 686,792 to 678,048 bytes and the line table from
  4,265,606 to 4,259,718 bytes. The WASM shrinks to 22,083,398 bytes.

No research function is linked into the PlainCAD bridge today.

## Test inventory

`go test -json -list '^(Test|Fuzz|Example)' ./...`, reduced to
`package<TAB>name` lines and sorted with `LC_ALL=C`: 6,908 lines,
`sha256 2c939363fb2b4e85ed337fae17a6f376fe6181466cc2d9452f65ff49841bd39e`.

## Default-tier time for `brep/trimmed`

`go test -count=1 ./brep/trimmed`, warm build cache, default `-parallel`:
261 s, 261 s, 255 s. Median 261 s.

## Classification

`go run ./tools/refgraph -pkg brep/trimmed -exclude examples`, identical on
`darwin/arm64`, `linux/amd64` and `js/wasm`:

- 1,101 production files: 561 core, 540 research, 0 references from core to
  research.
- 899 in-package test files: 498 hold a research declaration, 126 of them
  mixed. 29 external-package test files: 24 research.
- 33 files in other packages use a research symbol.

## Dry run

- `go build -overlay` with the research production files and
  `examples/tubelens` deleted: every package builds.
- Test build of `brep/trimmed` with research test files deleted, mixed files
  kept: 861 errors, all in 120 of the 126 mixed files.
- Test build of the other packages with their research test files deleted:
  fails in `internal/occtcheck`, `exchange` and `exchange/step` on helpers
  declared in deleted files. Phase 2 moves those helpers.

## Reflection and interface assertions

- `reflect` in non-test code is `reflect.DeepEqual` (391 uses) and
  `reflect.ValueOf(d).IsZero()` (3 uses). Nothing looks up a method by name.
- Inline interface assertions in non-test code name two methods,
  `EllipseParameters` and `PartialSkewAngularInterval`. Neither is declared
  in a research file.
- Use of a package type as a named interface is covered by the classifier,
  which found one such case (`FirstBoundaryHit`, used by `document`) and
  keeps it core.
