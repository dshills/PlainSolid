# Source-bound native NURBS planar assembly persistence

The unit, scaled and oblique authenticated NURBS assembly models have additive
native JSON read/write entry points:

| Source model | Write | Read | Explicit mode |
|---|---|---|---|
| `NURBSPlanarAssembly` | `WriteNURBSPlanarAssemblyJSON` | `ReadNURBSPlanarAssemblyJSON` | `unit-cardinal` |
| `ScaledNURBSPlanarAssembly` | `WriteScaledNURBSPlanarAssemblyJSON` | `ReadScaledNURBSPlanarAssemblyJSON` | `scaled-cardinal` |
| `ObliqueNURBSPlanarAssembly` | `WriteObliqueNURBSPlanarAssemblyJSON` | `ReadObliqueNURBSPlanarAssemblyJSON` | `oblique-rigid` |

All use the new format tag `plainsolid-step-nurbs-planar-assembly`, version 1.
Each typed reader requires its own mode and exact matching payload. There is no
fallback to another source codec, and no public mutable kind/payload union.
Earlier native formats, including contextual appearance and analytic hierarchy
envelopes, retain their meanings and reject this new format.

## Retained and authenticated claims

The envelope stores the original complete STEP source, native local products
once each, product/representation IDs and names, root and occurrence IDs,
parent/child relationships, exact represented poses, units, native vertices,
shared edges and oriented face loops, original body/face IDs, and body/face
appearance. Shared local assets are not duplicated per occurrence.

It also stores each mode's exact evidence:

- Unit: original support/pole/face roles, affine chart coefficients, model curve
  domains, pcurve equations and owner IDs.
- Scaled: original and normalized NURBS source, chart scaling factors and knot
  ranges, original/derived UV poles, original model curves and normalized
  pcurve equations.
- Oblique: original PRODUCT and product-definition/body/chart IDs, proper
  rational proof basis, and every original/proof 3D point pair. Native vertices
  remain in original local coordinates; occurrence poses are unchanged.

`Geometry.Source` in the envelope is explicitly the geometry proof source:
PLANE/LINE for unit and scaled models, original source for oblique models. It is
a claim to compare, not an alternate source from which to reconstruct geometry.
Rational coefficients remain strings and never pass through a floating parser.

## Reproof, failure and limits

A writer first takes bounded copies of all supplied geometry/evidence, then
invokes the matching source reader once and compares every regenerated claim.
Changing public derived geometry, colors, placement, ancestry or internal
source/evidence claims causes failure before destination output. This differs
intentionally from the source-only STEP exporters, which export retained source
rather than edited native claims.

A reader decodes bounded JSON, rejects unsupported tags, versions, mode/payload
combinations, unknown fields and trailing JSON, and invokes exactly the selected
source entry point once. That entry point performs its composed exact source,
ownership, global embedding and appearance proofs. Stored native geometry is
never constructed or used for repair. Only the newly proved model is returned
after a complete claim comparison; failure returns the zero model.

Readers use `IndependentMaterialJSONLimits`: at most 8 MiB of outer JSON, with
smaller caller byte limits permitted, plus the existing source `Limits`. Source
and each retained proof-source string are capped at 1 MiB. Claims are bounded
by 16 products/bodies, 4096 occurrences, 8192 native vertices/edges, 4096 faces,
16384 native face uses and 4096 source-evidence records (16 oblique frames).
Underlying exact source and embedding limits are unchanged.

Cancellation is checked before/after bounded input and throughout the selected
source proof. Writers stage and compare the entire envelope before one write,
check cancellation before output, and report short writes. Destination I/O
failure can still leave partial bytes. Canonical writer output is stable across
read/write; arbitrary JSON formatting is not retained.

## Qualification

Each independent retained AP242 fixture passes
`original STEP -> native JSON -> typed reload -> retained STEP -> OCCT`:

| Mode | Assembly volume | Area | World centroid |
|---|---:|---:|---|
| Unit | 54 | 126 | (85/18, 86/9, 5/2) |
| Scaled | 136 | 252 | (13/2, 180/17, 57/17) |
| Oblique | 1750 | 1350 | (69/14, 66/7, 99/14) |

The independently built XCAF observer confirms 3 solids, 18 face uses,
BRepCheck validity and both product colors for every native re-export. Use the
tracked oblique observer's generic `read` mode; it does not generate the native
geometry being checked.

```
sh tools/occtref/build_author_ap242_oblique_nurbs_planar.sh OCCT_INCLUDE OCCT_LIB OUTPUT
PLAINSOLID_OCCT_AP242_NURBS_NATIVE=OUTPUT go test ./exchange/step -run '^TestNURBSAssemblyNative' -count=1
go test -race ./exchange/step -run '^TestNURBSAssemblyNative' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzNURBSAssemblyNative$' -fuzztime=40x -parallel=2
```

Tests cover canonical bytes and all original claims, changed native topology,
face/body IDs, names, units, poses, colors, valid-but-different source appearance,
invalid original supports, forged chart/rational evidence, mixed mode payloads,
wrong/old tags, unknown/trailing fields, byte/source limits, cancellation during
input, stale writer models, atomic invalid writes and short destinations. The
fuzzer seeds all three modes and checks zero-result failures and accepted
native round trips under a fixed iteration budget.
