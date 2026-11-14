# Independent mixed curved Boolean reference

`mixed_crossing_roofs.cxx` constructs two original graph solids from six
independent Bézier patches each, using the analytic roof equations in the
source. It does not read a PlainSolid curve, mesh, placement, or solid.
It sews each OCCT shell, checks validity, and runs OCCT intersection,
union, and difference. It then reports original topology, area, volume,
volume centroid, and inertia about the world origin as JSON lines. OCCT
translates its centroidal inertia tensor using `GProp::HOperator`.

Build against the OCCT revision in [the reference README](README.md):

```sh
sh tools/occtref/build_mixed_crossing_roofs.sh "$OCCT_INCLUDE" "$OCCT_LIB" /tmp/occt-crossing-roofs
PLAINSOLID_OCCT_CROSSING_ROOFS=/tmp/occt-crossing-roofs \
  go test ./brep/trimmed -run '^TestMixedOriginalSourceMassAndArea$' -count=1 -timeout=15m
```

The live run used OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9`.
Archive SHA-256: `32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.
Observed executable SHA-256:
`99e06b00164bac004cb08580c507194c34ae681111c6576ff77a6fbe702cb7df`.

| Operation | Faces | Solids | Euler characteristic | Volume | Surface area |
| --- | ---: | ---: | ---: | ---: | ---: |
| Intersection | 8 | 1 | 2 | 0.4843585758058463 | 3.992955181787411 |
| Union | 17 | 1 | 2 | 1.4287883967133232 | 8.487908279091027 |
| Difference | 14 | 2 | 4 | 0.7705242365509349 | 8.057158869984429 |

The Go test also supports the retained numerical reference values without a
local OCCT executable. Live tolerances describe reproducibility of OCCT
floating-point integration only. The native intervals use original trimmed
supports and their conservative chart covers. All three Boolean results
check containment of the OCCT volume, centroid, first moments and full
world inertia tensor. The independent evaluator does not prove a general Boolean
capability or exact equality between either engine's reported area values.

`BoundTrimmedGeometry` is a separate native enclosure query. It intersects
the operation's operand box with the outward hull of all retained and
unresolved source cells, using the same chart cover as the area and mass
queries. Its test checks independently classified retained points and atomic
failure on exhaustion. The result is a conservative box, not a claim of exact
coordinate extrema.
