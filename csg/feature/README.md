# Interior planar sketch features

`Extrude` adds a protrusion to a selected face of a convex planar BRep or removes
a blind pocket. Its world-space sketch is a simple planar polygon strictly
inside the selected convex face. Concave sketches and either winding are
accepted; holes, face-boundary contact and through pockets are not supported.
The offset is a full translation vector and may be oblique. Protrusions must
point outward; pockets must point inward and remain strictly inside every other
noncoplanar support. Convexity makes endpoint checks sufficient to exclude
side-wall and bottom breakthrough throughout the translation.

Base coplanarity, convexity, sketch support incidence, strict containment and
translation addition are checked with exact rationals over represented binary64
coordinates. Translation must be exactly representable in the tool BRep: for
example, a binary64 offset of `0.1` added to coordinate `4` generally fails with
`ErrPrecision`. No tolerance silently changes the requested feature. The prism
constructor checks sketch simplicity, and `poly` validates both operands and the
result geometrically. The final Boolean uses exact rational BSP splitting;
output conversion follows `poly.Options` and can fail explicitly.

The result includes the closed tool and full face/edge topology history.
Operand A identifies the base, operand B the tool. `Mouth` and `Terminal` name
the tool caps; `SketchWalls` maps each original sketch segment to its swept tool
face regardless of winding. Pocket wall sources are reversed by subtraction.
Output faces are triangulated; this API does not claim same-domain unification,
curved sketch features, variable-depth pockets or persistent feature naming.

Input validation and the Boolean/history passes have separate explicit work
budgets and cancellation checks. Default feature limits are 256 sketch vertices,
256 base faces and 100000 validation predicates. Runtime dependencies are only
other PlainSolid packages and the Go standard library.

Document regeneration exposes `document.PlanarProtrusionFeature` and
`document.PlanarPocketFeature`. Set `Input`, `EntryRole` (for example `z-max`),
`Profile` and `Offset`. Named-face selection currently accepts a box generator
through affine transforms; profile coordinates are in the transformed input
asset's coordinates. Output is triangulated and does not inherit box face-role
indices. Recipes persist through document serialization, transactions and
undo/redo. Evaluation failures return no partially materialized state.

`ThroughPocket` removes a prism between explicitly selected entry and exit
faces. The supports must be parallel with opposite outward normals; each
translated sketch vertex must land exactly on the exit support and strictly
inside its polygon. All other supports must stay strictly outside the tool,
which proves absence of side breakthrough for the complete sweep on a convex
base. Oblique translations are allowed. Both tool cap faces disappear under
subtraction; reversed tool side-face and edge provenance remain available.
Tests verify a genus-one tunnel, volume, area, centroid, and cap ancestry.

`document.PlanarThroughPocketFeature` adds the same construction to regeneration,
using `EntryRole` and `ExitRole` on a box or transformed box, plus `Profile` and
`Offset`. The offset must terminate exactly at the named exit face.

`ExtrudeProfiles` applies multiple profiles on one selected face with one common
offset and protrusion/pocket kind. Profiles must have strictly separated closed
tool boundaries, established by exact boundary distance; touching, overlap and
nested footprints are rejected atomically. Nested footprints necessarily overlap
the tool caps on their shared starting plane. This API does not merge conflicting
sketch regions. It validates every profile, constructs a disjoint compound tool,
and performs one final Boolean against the original base.

`BatchResult.Profiles` preserves caller profile order and maps caps/walls into the
compound tool. `ToolFaces` and `ToolEdges` map every compound entity back to its
profile; Boolean operand B references that returned compound. Default maximum
profile count is 8, with a hard maximum of 64. Per-profile validation, each pair's
exact distance search, and final Boolean/history have separate explicit budgets.
There is no wall-clock or total-across-all-profiles work guarantee beyond these
bounded counts. Errors discard the entire result. Multiple-profile recipes are
currently a direct modeling API, not a document feature kind.

`ExtrudeWithHoles` and `ThroughPocketWithHoles` accept a simple outer ring with
strictly contained, disjoint inner rings. Either winding is supported. The
convex-base, support containment and exact coordinate representability rules
remain unchanged and are checked on every ring. `HoledResult.HoleWalls` maps each
inner ring's original segments to compound tool wall faces; outer ancestry uses
`Result.SketchWalls`. Ring validation passes have separate feature work budgets;
total ring vertices share `MaxSketchVertices`, and final Boolean/history limits
apply to the complete tool. Invalid/touching/nested holes fail atomically.

A blind annular pocket leaves the central material connected through its floor.
An annular through-cut can separate a central material island into another
closed solid; the result retains it rather than silently deleting it. Cap and
inner/outer wall ancestry remain available. Analytic tests verify volumes 72, 56,
and 32 for a 4-cube fixture, and live OCCT independently authors a holed face prism
for all three cases and compares mass properties.
