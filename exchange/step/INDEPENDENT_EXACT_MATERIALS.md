# Independently rooted planar and elliptical STEP materials

`ReadIndependentExactMaterials` and `WriteIndependentExactMaterials` admit one
to sixteen complete, independently owned standard product roots in one STEP
file. A product is either an exact cardinal planar ADVANCED_BREP material, or
one of the seven existing exact filled/annular elliptical prism variants.
The planar branch proves original model lines, every owner pcurve and plane
frame using the existing exact mapped-planar predicates, then verifies global
planar embedding. Elliptical products retain their existing original-profile,
model-only trim and exact native sewing proof. Unknown or mixed supports inside
one product are rejected; no tolerant repair branch is tried.

This expands the former ellipse-only collection without weakening it. Both use
one shared bounded ownership partitioner. Each root must have one standard
product chain, one advanced representation with neutral frame and one body,
one attached application-protocol declaration and one product category. Only
neutral identity-placement records may be shared between roots. Source body,
representation, product, definition and SDR IDs and product names are retained.
Geometry, unit or product dependency sharing, orphan records, malformed product
chains and conflicting units fail atomically. All roots and dependencies must
be consumed exactly; caller limits on solids, faces, vertices and uses apply
cumulatively. The broader parser also caps at 1024 entities and 131072 bytes per
maximum member, depth 16 and 16384 values per maximum member. Old ellipse APIs
retain their smaller limits and reject the planar member.

The writer takes validated source graphs, retains original local geometry and
metadata, reassigns references consistently, stages the complete standard AP214
file, and reimports it before emitting bytes. It does not infer a shared asset,
assembly occurrence, placement or union. Members may overlap; this API makes no
inter-material disjointness claim. Native materials remain separate solids.
Source product names and native body names are separate fields.

## Independent OCCT evidence

`tools/occtref/step_independent_exact_materials.cxx` creates a box at
(−10,−4,−2), size(2,3,4), plus the prior three exact ellipse profiles: two filled
and one orthogonal annular material, with rational tilted frames and distinct
locations. All are generated from OCCT geometry. The box keeps its original
planar model and pcurves. For the ellipse products, the helper explicitly exports
standard model-only EDGE_CURVE geometry from OCCT's original 3D curves and omits
the rounded pcurve graph, as in the independently qualified ellipse codec.
No Go-generated geometry tokens are used. The retained file has 608 entities.

Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 transfers both the original
and Go-rewritten files through TransferRoots: four valid solids, 16 faces and 48
edge uses. Volume is 24+111.25π; area is 522.16869819556, independently checked
using exact cap areas and numerical ellipse-perimeter quadrature. Each native
member survives native JSON reconstruction. Optional live comparison:

```
PLAINSOLID_OCCT_INDEPENDENT_MATERIALS=/tmp/plainsolid-independent-exact-materials-oracle go test ./exchange/step -run TestIndependentExactMaterials
```

Build the comparator with the isolated script
`tools/occtref/build_step_independent_exact_materials.sh`. Tests cover writer
roundtrip, source identities, global limits, cross-body aliases, orphan geometry,
conflicting units, unsupported support changes and atomic invalid writer input.
Focused normal/live/race, Go1.23, vet, prior ellipse collection/compound
regressions and 100 bounded fuzz executions qualify this scope. No PlainCAD bore
or bridge path changes are involved.
