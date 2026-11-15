# Independently owned multiple nonplanar STEP voids

`ReadNonplanarSplineVoidsSource` adds a bounded standard `BREP_WITH_VOIDS`
material with **two to four voids**. Each shell has six original polynomial
spline faces and twelve model edges, and satisfies the existing quadratic,
unit-domain, genuinely nonplanar source proof. Every void occurrence must be
explicitly inward. The prior one-void API and native tag remain unchanged.

This is an ownership and material-domain expansion, not a fixed coordinate
recipe. Shell roles come from the outer shell reference and ordered original
void occurrences; entity numbers are never used to infer geometric roles.
Complete dependency closures are compared across every pair. Only parametric
context records may be shared; shared face, edge, point, support, pcurve or
model curve ownership is rejected. Unknown/unowned geometry is not discarded
by staging. Each outer/void pair retains the complete original model/pcurve
proof and strict outer containment proof.

The native material retains each original carrier once, reversing only the
void face orientation. `ValidateNestedSplineComponents` must certify all
components and every boundary pair, with the outer shell at depth zero and
**every void at depth one with the same outer parent**. Thus overlapping,
touching and nested void interiors cannot pass. `AssembleShells` alone is not
used as a material certificate. The exact native face mapping follows original
shell/face ordering and is exposed separately from original source entity IDs.

Limits are 1 MiB source, 2048 entities, 32768 values, parse depth 16, at most
five total shells/thirty faces, and caller face/use/Cartesian-point limits on
the complete original model. Independent geometric checks use the existing
bounded separation, embedding and ray algorithms with cancellation. Failures
return an empty result. Proof views are implementation details; original
source, shell/occurrence/face/edge/pcurve identities remain retained.

## Generated output and persistence

`WriteNonplanarSplineVoidsSource` emits the unchanged original source after
reproof. `WriteNonplanarSplineVoids` authors a new standard single material root
with every independent inward shell and re-proves the generated file. The
previous exact polynomial/UV authoring helper is shared with the one-void
writer; it does not widen any external source interpretation.

The distinct `plainsolid-step-nonplanar-spline-multiple-voids-source` version 1
JSON envelope retains source, ordered occurrence/role IDs, original owner data,
native face correspondence, generated STEP, the complete material and every
individually outward shell's native claim. Reload reconstructs/proves original
source and strict disjoint containment before comparing all claims. Stored
native or ancestry data never substitutes for proof. Original and ordinary
native reloads retain the independently reversed void components.

## Independently authored qualification

`tools/occtref/author_ap242_spline_roof_voids.cxx` constructs original OCCT spline
shells, inserts their reversed shell occurrences into one solid, validates and
exports AP242. It does not use Go-generated STEP. Pinned OCCT revision:
`4dbfdcc448c0b3e565488afcd626c84483172db9`.

The retained two-void source has 926 entities and eighteen faces. Outer roof is
`0<=x<=4, 0<=y<=3, 0<=z<=2+(x/4)^2`. Its distinct inner roofs are scale `1/4`
with translation `(1/2,1/2,1/2)`, and scale `3/8` with translation `(2,1,1/4)`.
Independent volume is `3339/128`; centroid is
`(56737/26712,217/144,6149/5040)`; area is `77/64` times the original roof area.
Pinned OCCT reports volume `26.0859375`, area `69.96101191391968`.

The helper also independently authors three and four voids using two additional
strictly separated `1/8`-scale roofs, thereby testing the upper grammar bound.
All three modes pass original/native/source and generated STEP transfer with
one valid material and respectively 18, 24 and 30 faces. Native continuous
edge discrepancy is exactly zero. Native queries distinguish material between
voids from each independent void interior.

```sh
sh tools/occtref/build_author_ap242_spline_roof_voids.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib \
 /tmp/plainsolid-ap242-spline-roof-voids
PLAINSOLID_OCCT_SPLINE_ROOF_VOIDS=/tmp/plainsolid-ap242-spline-roof-voids \
 go test ./exchange/step -run '^TestNonplanarSplineVoids' -count=1
go test -race ./exchange/step -run '^TestNonplanarSplineVoids' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzNonplanarSplineVoidsSource$' -fuzztime=40x -parallel=1
```

Adversaries cover intersecting, nested and touching voids, outward void sense,
duplicate occurrences/shells, cross-shell support, recursive shell references,
excess void count, forged source/native face correspondence and old format tags.
Harmless entity reorder and semantic void-list permutation retain the actual
source order. Cumulative limits, canceled/size-limited reads, copied evidence
and stale-material atomic write rejection are checked. Potentially difficult
negative geometry and fuzz cases use explicit test context deadlines; unresolved
proofs reject rather than accepting an approximate material.
