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

`MixedRoofMaterialQuery.BoundVolume` integrates through the original XY
footprints. It integrates polynomial column thickness exactly wherever the
roof order is proved. It retains enclosing contributions on crossing cells
and on cells with active nonconstant weights. Its separate bounds for the
three crossing operations contain the OCCT volumes above within enclosures
of width at most `1/1000`.

The same executable's `weighted-mass` mode sews rational surfaces for the
stock roof `(2+x²)/(1+x)` and a strictly enclosed tilted tool. The OCCT
Common volume is exactly recorded as `0.421875`; Fuse and Cut report
`1.5794415416886896` and `1.1575665416886896`. The Go test also checks the
independently integrated `3 ln(2) - 1/2` stock volume and the exact `27/64`
tool volume. To run both the station and mass comparisons:

```sh
PLAINSOLID_OCCT_WEIGHTED_ROOF=/tmp/occt-crossing-roofs \
PLAINSOLID_OCCT_WEIGHTED_ROOF_MASS=/tmp/occt-crossing-roofs \
  go test ./brep/trimmed -run '^TestOriginalWeightedRoofMaterialQueries$' -count=1
```

The single-span biquartic fixture keeps the same original edge curves while
adding two interior homogeneous terms to the stock's roof. The diagonal
line event is checked by exact substitution into the independently expanded
degree-eight polynomial, with a nonzero sign in the previous sextic. This
covers higher-order line roots without changing the native Boolean proof.

`MixedRoofMaterialQuery.BoundMass` integrates all ten raw monomials through
degree two over the same original column material. Fixed polynomial columns
have exact moments. Unresolved and rational columns use inner and outer
material intervals; the integration accounts for positive and negative parts
when a coordinate crosses zero. The crossing test checks volume, centroid and
the full inertia tensor for each operation. The weighted Common checks exact
local and placed tensors, and the weighted Cut checks live OCCT containment.

The creased-roof fixture is genuinely piecewise: the original stock roof is
`2+x` to the left of `x=1/2` and `3-x` to the right. An independently
authored OCCT fuse joins two sloped source halves before cutting the same
buried tool; no PlainSolid boundary is imported. The Go test retains the
single original roof owner across both spans, distinguishes the isolated
crease contact from an unchanged interior crossing, reports both one-sided
normals, and integrates the volume `117/64`, centroid, and full inertia
tensor exactly on the native knot partition. OCCT agrees within `1e-12`.
Run it with `PLAINSOLID_OCCT_CREASED_ROOF` pointing
at the executable built above.

`BoundTrimmedGeometry` is a separate native enclosure query. It intersects
the operation's operand box with the outward hull of all retained and
unresolved source cells, using the same chart cover as the area and mass
queries. Its test checks independently classified retained points and atomic
failure on exhaustion. The result is a conservative box, not a claim of exact
coordinate extrema.
