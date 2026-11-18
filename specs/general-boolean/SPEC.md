# General Boolean trial: planes and cylinders

Status: draft for owner review, 2026-11-18. This document is a specification
only. It contains no implementation and authorizes none until a plan is
written and approved.

## 1. Purpose

PlainSolid has no general Boolean. `internal/plaincadbridge` serves the
PlainCAD operations `cut`, `fuse`, `common`, `cutMany`, `joinAll` and
`overlapVolume` by trying special-case constructors in a fixed order: about 18
bridge paths for `cut`, 12 for `cutMany` and 25 for `overlapVolume`
(`internal/plaincadbridge/bridge.go:378-529`, `cut_many.go:13`,
`overlap_volume.go:17`), each handling one named arrangement of inputs. The
only general entry points are for planar solids (`csg/poly.Combine`,
`csg.Union`, `csg.Intersect`).

This trial tests one claim: **a single pipeline, with no code that depends
on a named arrangement of inputs, reproduces what those special-case
families do for solids bounded by planes and cylinders.** The trial is
bounded. It ends in a pass (section 15) or a stop (section 16), and both are
defined before any code is written.

Conventions. Requirements carry identifiers made of a letter prefix and a
number (for example S1, X6, PC2, OD-1); the prefix only groups them by
section. Rows of the section 11 matrix are C1 to C27. "Must" states a
requirement. OCCT is Open CASCADE Technology, the reference kernel pinned in
`tools/occtref/README.md`. STEP (ISO 10303) and IGES are CAD exchange
formats.

## 2. Governing decisions

- G1. The tolerance policy adopted by the owner on 2026-11-18 governs this
  work. Its text is the "Tolerance policy" section of `docs/SPECIFICATION.md`
  at commit `d3b8a6e5` on branch `codex/trimmed-split-plan`
  (`git show d3b8a6e5:docs/SPECIFICATION.md`). In short: explicit tolerances
  passed as options, coincidence within tolerance, results accepted on
  validation and not on proof, an explicit error when no result validates,
  and a result that records the tolerance it was built with.
- G2. `AGENTS.md` governs scope and verification. In particular: no function,
  type or file family that handles one named arrangement of inputs; standard
  library only; a new package in preference to more files in `brep/trimmed`;
  an exported identifier needs a caller outside its package in the same
  change; a new test finishes in under two seconds or starts with
  `slowtest.Skip(t)`.
- G3. Provenance. The owner allowed the pinned OCCT source to be read for the
  shape of a solution and forbade derived work: no porting or translation, no
  copied identifiers, comments or code. This document was written after
  reading the structure of that source. It describes a design in PlainSolid's
  own terms; the architecture it describes (pairwise interference detection
  in order of dimension, a shared record of splits, face subdivision, set
  membership classification, selection and merging) is the boundary
  evaluation scheme of the published solid-modeling literature (Requicha and
  Voelcker, "Boolean Operations in Solid Modeling: Boundary Evaluation and
  Merging Algorithms", 1985; Tilove, "Set Membership Classification", 1980;
  Mäntylä, "An Introduction to Solid Modeling", 1988; Hoffmann, "Geometric
  and Solid Modeling", 1989). Implementation works from this document and
  that literature. See open decision OD-6.
- G4. `brep/trimmed` and `specs/brep-trimmed-split` belong to another work
  stream. The trial does not edit either. It uses only identifiers in files
  that `specs/brep-trimmed-split/core-files.txt` (branch
  `codex/trimmed-split-plan`) keeps in the default build.

## 3. Terms

| Term | Meaning in this document |
|---|---|
| Operand | One of the two input solids, called A and B. For `cut`, A is the body and B is the tool. |
| Working tolerance, ε | The single linear tolerance of one operation, defined in section 6. |
| Carrier | The unbounded curve or surface an edge or face lies on: a line, circle or ellipse; a plane or circular cylinder. |
| Arrangement | The one shared record built by the intersection stage and read by the building stage (section 7.2). |
| Node | A point of the arrangement. One node stands for every input vertex and computed point merged into it. |
| Span | The part of one input edge between two consecutive nodes on it. |
| Bundle | A set of spans, from different edges, that occupy the same curve interval within tolerance. A bundle is one edge of the result. |
| Trace | A span created where a face of A and a face of B meet in a curve that is not already an edge of either. |
| Crossing contact | A contact where the boundary of one operand passes from one side of the other operand's boundary to the other side. |
| Touching contact | A contact, at a point or along a curve, where the two boundaries meet without crossing. Tangency is a touching contact. |
| Same-surface pair | A face of A and a face of B whose carriers are the same surface within tolerance. |
| Piece | A connected part of one input face after the face has been split along every span and trace that lies in it. |
| Chart | The planar coordinate domain a face is split in: frame coordinates for a plane; for a cylinder, arc length around the axis and distance along it, cut along one seam line. |
| Regularized | Closure of the interior. A regularized result has no isolated faces, edges or vertices and no zero-thickness material. |

## 4. Scope requirements

### 4.1 Input class

- S1. Each operand is a `trimmed.Solid` (`brep/trimmed/solid.go:46`) that is a
  closed, outward-oriented, embedded boundary. Operands with several shells
  are in scope: disjoint components and internal voids.
- S2. Every face carrier is a plane or a circular cylinder. Accepted surface
  values are `geom.PlaneSurface`, the polar plane chart recognized by
  `parametric.PolarPlaneFrame`, `geom.CylinderSurface`, and any of these
  under a rigid placement reported by `parametric.SurfaceMapping`.
- S3. Every edge carrier is a line, circle or ellipse. Accepted curve values
  are `geom.Line` and `geom.Ellipse` (a circle is an ellipse with equal
  radii), and either under a rigid placement reported by
  `parametric.CurveMapping`. Collapsed edges (nil curve) are accepted only as
  the pole of a polar plane chart and carry no geometry into the arrangement.
- S4. A placement is rigid when its linear part M satisfies
  ‖MᵀM − I‖∞ ≤ the angular tolerance and det M > 0.
- S5. An operand outside S1 to S4 is rejected with `ErrUnsupported` naming
  the first offending face or edge index. Embedding (S1, no self-intersection)
  is a precondition that intake does not test; see V7 for what is guaranteed
  when it is violated.
- S6. A zero-value `trimmed.Solid` is the empty solid and is a valid operand.

### 4.2 Operations

Union (A ∪ B), common (A ∩ B) and cut (A − B), each regularized. These are the
bridge operations `fuse`, `common` and `cut`. `cutMany`, `joinAll` and
`overlapVolume` are compositions of them (section 12).

### 4.3 Output

One `trimmed.Solid` that satisfies S1 to S3, or the empty solid, together
with the grouping of its shells into solids and a report (section 5). The
output class equals the input class, so a result is a valid operand of a
later operation.

### 4.4 Non-goals

Each of these is outside the trial, and input that needs one returns
`ErrUnsupported` or is simply not offered by the package:

- Spline, conical, spherical and toroidal surfaces; spline, parabolic,
  hyperbolic and helical edges.
- Cylinder pairs listed as "out" in section 7.4, including every pair whose
  intersection is a quartic space curve.
- Fillets, chamfers, offsets, shelling, drafts and any other modification
  operation. A solid that already contains a cylindrical face made by an
  earlier fillet is an ordinary operand.
- History, ancestry and persistent naming of faces, edges or vertices.
- STEP, IGES and native persistence of results beyond what `trimmed.Solid`
  already provides.
- Non-manifold results, open shells, sheets and wires as operands or results.
- Section, split and "general fuse" of more than two operands as public
  operations.
- Repair of invalid operands and detection of self-intersecting operands.
- Parallel execution and performance work beyond the limit in A7.
- Making the general path the bridge default, and deleting the families
  listed in section 15. Both follow a pass and need a separate request.
- PlainCAD browser gates, race runs, cross-platform builds and the complete
  Go suite, unless the owner asks.

## 5. Public contract

- P1. The new package is `plainsolid/brep/boolean` (directory `brep/boolean`).
- P2. It exports exactly one operation entry point, `Combine`, taking a
  context, operand A, operand B, an operation and options, and returning a
  result and an error. It also exports the operation enumeration (`Union`,
  `Common`, `Cut`), the options type, the result type and the error values of
  section 9. Nothing else is exported.
- P3. Options carry:
  - `Linear`: absolute linear tolerance in model units, ≥ 0.
  - `Relative`: dimensionless tolerance, ≥ 0, scaled by model size (T1).
  - `Angular`: bound on the absolute sine used in parallel and perpendicular
    tests, in [0, 1), with the meaning of `geom.IntersectionTolerance.Angular`
    (`geom/intersection.go:19`).
  - `Volume`: relative bound for the volume identities (V5), > 0.
  - `Samples`: number of membership samples (V6); zero selects 64.
  - Work limits (section 10); zero selects the stated default.
  - Integration options for mass properties, of type
    `parametric.IntegrationOptions`.
- P4. `Linear` and `Relative` must not both be zero, `Angular` must be > 0 and
  `Volume` must be > 0. There is no hidden default tolerance. `Samples` and
  every work limit must be ≥ 0, and a nonzero `MaxGrowth` must be ≥ 1. Every
  option must be finite. A violation returns `ErrInvalid` before any
  geometry is examined.
- P5. The result carries: the output solid; for each shell its face indices,
  whether it is an outer shell or a void, and the index of the solid it
  belongs to; the number of solids; and a report. The report contains ε, the
  largest node radius, counts of nodes, spans, bundles, traces and pieces,
  the volumes of A, B and the result, the largest edge and vertex residuals
  found by validation, and the list of touching contacts found.
- P6. On error the result is the zero value. A partial result is never
  returned.
- P7. `Combine` does not modify its operands and holds no package-level
  mutable state. Two calls with equal operands, operation and options return
  equal results: all iteration is in index order and no randomness is used.

## 6. Tolerance model

- T1. ε = max(`Linear`, `Relative` × L, tolerance of A, tolerance of B), where
  L is the diagonal of the bounding box of both operands
  (`trimmed.Solid.Bounds`, `brep/trimmed/bounds.go:15`) and operand
  tolerances come from `trimmed.Solid.Tolerance`
  (`brep/trimmed/topology.go:93`). An empty operand contributes no box and
  tolerance 0. If ε ≥ L / 100 the call returns `ErrInvalid`. When both
  operands are empty, L is undefined, `Relative` contributes nothing, the
  ε ≥ L / 100 test is skipped, and the result is the empty solid.
- T2. Every node has a radius ρ ≥ ε. A node made from one input vertex or one
  computed point has ρ = ε. A node made by merging has the mean position of
  its members and ρ = ε + the largest distance from a member to that
  position.
- T3. Two points are coincident when their distance is at most the sum of
  their radii (ε for a point that is not yet a node). A point is on a curve
  or surface when its distance to it is at most its radius. Two carriers are
  the same when they satisfy the tests of sections 7.4 and 7.5.
- T4. A node radius must not exceed `MaxGrowth` × ε (default 4). A merge that
  would exceed it returns `ErrTolerance`, not `ErrBudget`. This stops chains of near
  coincidences from collapsing real geometry.
- T5. Angular decisions use `Angular`: directions are parallel when the sine
  of their angle is at most `Angular`, perpendicular when the cosine is.
- T6. Tolerances live on nodes inside the arrangement only. The output solid
  records one value, τ = the largest node radius used by the result, as its
  `trimmed.Solid` tolerance. For an empty result τ = ε. Per-vertex and
  per-edge tolerances in the stored solid are not part of the trial.
- T7. There is no separate widening ("fuzzy") option. A caller widens
  coincidence by passing a larger `Linear`.
- T8. A configuration with a feature whose size lies strictly between ε and
  4ε (for example one operand penetrating the other by 2ε) is
  tolerance-scale. For such input `Combine` returns either `ErrTolerance` or
  a result that passes all of section 8; which of the two is not specified.
  Features smaller than or equal to ε are coincidences. Features of 4ε and
  larger are ordinary geometry and must be handled.

## 7. Functional behavior: the pipeline

There is one pipeline. Every input in the class of section 4.1 passes through
the same stages in the same order. No stage inspects how many faces an
operand has, which primitive it came from, or how it is placed, other than
through the general tests below.

### 7.1 Intake

- I1. Validate options (P4), the context and both operands (S1 to S6).
- I2. Short cuts that are part of the contract, decided before any pairwise
  work: if either operand is empty, or the bounding boxes inflated by ε do
  not overlap, the result follows table 7.8 with every piece of A classified
  outside B and every piece of B outside A. The same stages 7.9 to 7.11 still
  produce and validate the result.
- I3. Reduce every face to its carrier (plane: point and unit normal;
  cylinder: axis point, unit axis direction, radius), its orientation
  relative to the carrier normal, and its loops as chains of edge uses in a
  chart. Reduce every edge to its carrier and parameter interval.
- I4. A cylinder face is cut along one seam line parallel to its axis so that
  it has a single chart. A seam is an ordinary edge of the arrangement with
  two uses by the same face. The seam of an input cylinder face that already
  has one is kept where it is.

### 7.2 Arrangement

The arrangement is the only data passed from the intersection stage to the
building stage. It holds:

- AR1. Nodes: position, radius, and the input vertices merged into each.
- AR2. For each input edge, the ordered list of nodes on it and the spans
  between them.
- AR3. Bundles: for each set of coincident spans, the member spans and their
  relative directions.
- AR4. Traces: carrier, end nodes, the face of A and the face of B it lies
  in, and whether the contact is crossing or touching.
- AR5. For each input face, three lists: spans and traces that lie in its
  interior, spans of the other operand that lie along its boundary, and nodes
  that lie in its interior without an incident span in the face.
- AR6. Same-surface pairs, each with a flag saying whether the two faces have
  the same or opposite outward normals.
- AR7. Touching-contact records: the locus (a node, or a span or trace) and
  the faces of A and of B that meet there.

- AR8. Architectural constraints, not functional behavior: every later
  stage reads these lists and none recomputes an intersection; there is one
  arrangement per call; and the passes run in the order of section 7.3.
  These are not observable from outside `Combine`. They are what makes the
  pipeline one path, and they are checked by review of the diff under PC2.

### 7.3 Intersection stage

The stage runs six passes in this fixed order. Lower-dimensional coincidences
are settled first, so that a later pass attaches what it finds to nodes that
already exist instead of creating near duplicates. Each pass tests only pairs
whose bounding boxes, inflated by ε, overlap, and considers only pairs with
one member from A and one from B.

- X1. Vertex with vertex. Coincident vertices merge into one node.
- X2. Vertex with edge. A node on an edge of the other operand, and not
  coincident with an end of that edge, is added to the edge's node list.
- X3. Edge with edge. Using section 7.5, each pair yields isolated points,
  an overlap interval, or nothing. An isolated point becomes a node on both
  edges (merging with an existing node where coincident) and is recorded as
  crossing or touching. The ends of an overlap interval become nodes on both
  edges and the spans between them form a bundle.
- X4. Vertex with face. A node on a face's carrier and strictly inside its
  trimmed region, with no coincidence already recorded by X1 or X2 against
  that face's boundary, is recorded in the face's interior-node list. It is
  also recorded as a touching contact (AR7) when every face of its own
  operand at that vertex lies strictly on one side of the face's carrier
  near the vertex.
- X5. Edge with face. Each span, as split so far, is tested against each
  face of the other operand using section 7.5. An isolated point inside the
  trimmed region becomes a node on the span and in the face, recorded as
  crossing or touching. A span that lies on the carrier and inside the
  trimmed region over its whole length is added to the face's interior-span
  list, and is also recorded as a touching contact (AR7) when both faces
  next to it in its own operand lie strictly on one side of the face's
  carrier near the span. A span that lies along the face boundary is already in a bundle.
- X6. Face with face. Each pair is resolved by section 7.4 into one of: no
  contact; a same-surface pair (AR6), which creates no trace; or one or more
  carrier curves, each marked crossing or touching. Each carrier curve is cut
  at the nodes already on it, and each interval between consecutive nodes
  that lies inside both trimmed regions becomes a trace, unless a span or
  bundle already occupies that interval, in which case that span or bundle is
  recorded for both faces instead. A closed carrier curve with no node on it
  that lies inside both trimmed regions first receives one node, at the
  point of parameter zero of the carrier, and then becomes one closed trace
  whose two end nodes are that node. Every trace therefore has end nodes
  (AR4), and F3 walks a closed trace as a loop through its one node.
- X7. When a surface pair yields more than one carrier curve, the points
  where those curves meet each other inside both trimmed regions become
  nodes before X6 cuts the curves.
- X8. After X6, any two traces in one face that cross at a point that is not
  a node indicate an inconsistent arrangement, and the call returns
  `ErrInconsistent`.
- X9. Membership of a point in a trimmed region is decided in the face's
  chart by a tolerant point-in-loops test: a point within its radius of a
  loop is on the boundary, otherwise inside or outside by crossing count.

### 7.4 Surface pairs

Let d be the distance between two cylinder axes, r₁ ≥ r₂ their radii, and let
"parallel", "perpendicular" follow T5.

| Pair | Configuration | Result |
|---|---|---|
| Plane, plane | Not parallel | One line, crossing. `geom.IntersectPlanes`. |
| Plane, plane | Parallel, separation of every node of either face from the other plane ≤ its radius | Same-surface pair. |
| Plane, plane | Parallel, every such separation greater | No contact. |
| Plane, cylinder | Plane perpendicular to axis | One circle, crossing. |
| Plane, cylinder | Plane neither parallel nor perpendicular to axis | One ellipse, crossing. |
| Plane, cylinder | Plane parallel to axis, axis distance < r − ε | Two lines, crossing. |
| Plane, cylinder | Plane parallel to axis, axis distance within ε of r | One line, touching. |
| Plane, cylinder | Plane parallel to axis, axis distance > r + ε | No contact. |
| Cylinder, cylinder | Axes parallel and every node of either face within its radius of the other cylinder's surface (this needs d ≤ ε and radii that differ by ≤ ε, up to node radii) | Same-surface pair. |
| Cylinder, cylinder | Coaxial (axes parallel, d ≤ ε), no node of either face within its radius of the other surface | No contact. |
| Cylinder, cylinder | Parallel, r₁ − r₂ + ε < d < r₁ + r₂ − ε | Two lines, crossing. |
| Cylinder, cylinder | Parallel, d within ε of r₁ + r₂ | One line, touching (external tangency). |
| Cylinder, cylinder | Parallel, d > ε and d within ε of r₁ − r₂ | One line, touching (internal tangency). |
| Cylinder, cylinder | Parallel, d > r₁ + r₂ + ε, or d < r₁ − r₂ − ε | No contact. |
| Cylinder, cylinder | Not parallel, radii differ by ≤ ε, axes meet (d ≤ ε) | Two ellipses, crossing, meeting each other at two nodes (X7). Tier 2 only; see OD-1. |
| Cylinder, cylinder | Not parallel, any other, proved separate by SP2 | No contact. |
| Cylinder, cylinder | Not parallel, any other, not proved separate | `ErrUnsupported` naming both faces. |

- SP1. Plane with cylinder uses `geom.IntersectPlaneCylinder`
  (`geom/sections.go:52`). The parallel and coaxial cylinder cases have no
  existing helper and are written in the new package.
- SP2. Two non-parallel cylinder faces are proved separate when the distance
  between their axis segments (each axis restricted to the face's extent
  along it, extended by ε) exceeds r₁ + r₂ + ε, or when their bounding boxes
  inflated by ε do not overlap. No other proof is attempted in the trial, so
  some non-parallel faces that do not in fact meet return `ErrUnsupported`.
- SP3. "Out" for the trial, always `ErrUnsupported` unless SP2 applies:
  non-parallel cylinders of unequal radius; non-parallel cylinders of equal
  radius whose axes do not meet; and, unless Tier 2 is implemented,
  non-parallel cylinders of equal radius whose axes meet.
- SP4. For parallel planes and for parallel cylinders the same-surface test
  is made node by node, before any other row of the table: a node of one
  face passes when its distance to the other face's carrier is at most its
  radius. All nodes of both faces pass: same-surface pair. Some pass and
  some do not: `ErrTolerance`, because the pair is neither coincident nor
  separated at this tolerance. None passes: the remaining rows decide.
- SP5. Adding a surface pair after the trial must be an addition to this
  table and to the curve table of section 7.5. If it needs a change to
  sections 7.2, 7.3 or 7.6 to 7.11, that is evidence against the design and
  is reported (section 16).

### 7.5 Curve with curve, curve with surface

| Pair | Method | Results |
|---|---|---|
| Line, line | Closest points | Point (crossing or touching), overlap interval, none. |
| Line, plane | `geom.IntersectLinePlane` (`geom/intersection.go:50`) | Point, line in plane, none. |
| Line, cylinder | `geom.IntersectLineCylinder` (`geom/line_surfaces.go:120`) | 0 to 2 points with tangency flag, line on surface, none. |
| Line, circle or ellipse | Line with the conic's plane, then `geom.IntersectLineEllipse` (`geom/planar_intersections.go:36`) when coplanar | 0 to 2 points with tangency flag, none. |
| Circle or ellipse, plane | Plane with the conic's plane (`geom.IntersectPlanes`), then line with conic | 0 to 2 points with tangency flag, conic in plane, none. |
| Circle or ellipse, cylinder with axis parallel to the axis of the conic's own cylinder | Project along the axis; both become circles; circle with circle | 0 to 2 points with tangency flag, conic on surface, none. |
| Two conics in different planes | Line of the two planes, then each conic with that line | Points common to both. |
| Two coplanar conics, homothetic (equal axis ratio and parallel axes, within tolerance; all circles qualify) | Affine map taking both to circles; circle with circle | 0 to 2 points with tangency flag, overlap interval when the same conic, none. |
| Two coplanar conics, not homothetic | — | `ErrUnsupported`. |
| Circle or ellipse, cylinder not covered above | — | `ErrUnsupported` unless the owning faces are proved separate by SP2. |

- CC1. Circle with circle, the homothetic reduction, the parallel-cylinder
  reduction, point-to-parameter inversion for a conic, and segment-to-segment
  distance have no existing helper and are written in the new package.
- CC2. A result point is crossing when the curve passes from one side of the
  other carrier to the other at that point, and touching when it does not.
  A tangency flag from a `geom` helper marks touching.

### 7.6 Splitting faces

For each input face, in its chart:

- F1. Collect its boundary spans, its interior spans and its traces (AR5).
- F2. Remove dangling chains: repeatedly discard a span or trace that has an
  end node with no other span or trace of this face. A discarded element
  stays in AR7 if it is a touching contact, and otherwise must be an edge of
  the other operand lying in the face; anything else returns
  `ErrInconsistent`.
- F3. Trace the remaining graph into closed loops. Every interior span and
  trace is traversed once in each direction and every boundary span once, in
  its face's loop direction. Arriving at a node along an element, the walk
  leaves along the first other element met when turning clockwise in the
  chart from the reversed arrival direction, so that each loop keeps its
  region on its left. Directions are tangent directions in the chart at the
  node. Elements whose tangents agree within `Angular` are ordered by signed
  curvature in the chart, the one curving most to the left first. Elements
  that agree in tangent and curvature are the same carrier and must already
  be one bundle; otherwise the call returns `ErrInconsistent`.
- F4. A loop with positive area in the chart bounds a piece. A loop with
  negative area is a hole and is assigned to the smallest positive loop that
  contains it.
- F5. Each piece keeps its face's carrier and orientation and records its
  input face and operand.
- F6. A piece with no interior point farther than ε from its boundary
  returns `ErrTolerance` (T8).

### 7.7 Classification

- K1. A piece of a face that is in one or more same-surface pairs is tested,
  at one interior point, against the trimmed region of the other face of
  each of those pairs, in index order. Inside one of them: the piece is
  *shared*, with that pair's same or opposite flag. A boundary answer from
  any of them returns `ErrInconsistent`. Outside all of them: the piece is
  not shared and is classified by K2.
- K2. Every piece that K1 does not mark shared is classified *inside* or *outside* the other operand
  at one interior point at least ε from the piece boundary, using
  `trimmed.Solid.Classify` (`brep/trimmed/location.go:63`) on the other
  operand with tolerance ε. If the answer is boundary, up to four further
  interior points are tried; if all are boundary the call returns
  `ErrInconsistent`.
- K3. Two pieces of the same operand that meet along a span that is in no
  bundle with the other operand's spans, is not a trace, and is not on a
  same-surface region boundary must have the same class. A disagreement
  returns `ErrInconsistent`.

### 7.8 Selection

| Piece | Union | Common | Cut A − B |
|---|---|---|---|
| Of A, outside B | keep | drop | keep |
| Of A, inside B | drop | keep | drop |
| Of B, outside A | keep | drop | drop |
| Of B, inside A | drop | keep | keep, orientation reversed |
| Shared, same normals | keep A's copy | keep A's copy | drop both |
| Shared, opposite normals | drop both | drop both | keep A's copy |

### 7.9 Manifold test

After selection, before merging:

- N1. Every span, bundle or trace used by a kept piece must have exactly two
  uses by kept pieces, in opposite directions.
- N2. At every node that lies on at least one kept piece, the kept pieces
  around it, linked through the kept spans at that node, must form exactly
  one cycle. A node on no kept piece is not tested and is not in the result.
- N3. For every touching-contact record (AR7), kept pieces of A and kept
  pieces of B must not both contain the contact locus.

A failure of N1, N2 or N3 returns `ErrNonManifold` with the locus. A touching
contact that passes N3 leaves no edge or vertex in the result.

### 7.10 Merging

- M1. An edge whose two uses are by kept pieces on the same surface, with
  consistent orientation, is removed and the pieces become one face. This
  applies to pieces of one input face and to pieces of different faces or
  operands. The merged face takes the carrier of its lowest-indexed
  contributor, A before B.
- M2. A seam is removed by M1 only from a cylinder face that no longer winds
  fully around its axis; that face's chart is then moved so it is continuous.
- M3. A merge that would make a loop visit a node twice is not performed.
- M4. After M1, a node with exactly two incident edges on the same carrier
  is removed and the edges become one.
- M5. Merging is part of the contract, not an option: a result has no two
  adjacent faces on the same surface except across a required seam (M2) or
  across an edge that M3 keeps, and no vertex that M4 would remove except
  those added by B2.

### 7.11 Assembly

Sewing here means assembling faces that already share edges by identity in
the arrangement. No geometric welding takes place at this stage.

- B1. Faces are grouped into shells through shared edges. A shell with
  positive enclosed volume is an outer shell; one with negative volume is a
  void and belongs to the outer shell that contains it. Each outer shell with
  its voids is one solid. Solids are ordered by the lexicographically
  smallest corner of their bounding boxes.
- B2. Output conditioning. `trimmed.NewSolid` requires at least three uses
  per loop, distinct chart vertices within a face, and loops whose chord
  polygons are simple (`brep/trimmed/solid.go:211-306`). Assembly must add
  vertices on curved edges until this holds: it first cuts every curved edge
  into arcs of at most a quarter turn, then halves the arcs of any loop that
  still fails, at most six times. If a loop still fails after six halvings
  the call returns `ErrValidation`. Conditioning never moves
  geometry. Conditioning vertices are excluded from every topology count
  this document compares.
- B3. P-curves are straight for a line on a plane and for a line or circle
  on a cylinder, and `trimmed.EllipseUV` (`brep/trimmed/pcurve.go:14`) for a
  circle or ellipse on a plane and for an ellipse on a cylinder.
- B4. The output solid is built with `trimmed.NewSolidContext`
  (`brep/trimmed/solid.go:110`) and tolerance τ (T6). An error from it is
  returned as `ErrValidation` wrapping that error, except a context error,
  which is returned unwrapped as section 9 requires.
- B5. A selection that keeps no piece returns the empty solid and no error.

## 8. Validation before return

Every non-empty result must pass all of the following. A failure returns
`ErrValidation` naming the check and the entity.

- V1. Structure: `trimmed.NewSolidContext` accepts the result (closed loops,
  chart winding, every edge used exactly twice in opposite directions,
  sampled agreement of each p-curve with its edge within τ).
- V2. Shells: each shell is closed and consistently oriented; each outer
  shell encloses positive volume and each void negative volume.
- V3. Edge residual: at 17 evenly spaced parameters on every edge, including
  both ends, the distance from the edge point to the carrier of each of its
  two faces is ≤ τ, and the distance to the face point given by its p-curve
  is ≤ τ.
- V4. Vertex residual: the distance from every vertex to the corresponding
  end point of each incident edge is ≤ τ.
- V5. Volume identities. Let V(·) be volume from
  `trimmed.Solid.MassProperties` (`brep/trimmed/properties.go:33`) for A, B
  and the result, and let the volumes of A ∩ B, A − B, B − A and A ∪ B be
  computed from the classified pieces of this one arrangement. Then, with
  bound β = `Volume` × max(V(A), V(B)) + ε × (area(A) + area(B)):
  - |V(A − B) + V(A ∩ B) − V(A)| ≤ β
  - |V(B − A) + V(A ∩ B) − V(B)| ≤ β
  - |V(A ∪ B) − V(A) − V(B) + V(A ∩ B)| ≤ β
  - |V(result) − the piece-based volume of the requested operation| ≤ β
  - 0 ≤ V(A ∩ B) ≤ min(V(A), V(B)) + β and V(A ∪ B) ≥ max(V(A), V(B)) − β
- V6. Membership sampling. The sample points are terms 1 to `Samples` of the
  Halton sequence in bases 2, 3 and 5, whose three coordinates in [0, 1) map
  linearly onto the x, y and z extents of the bounding box of both operands
  inflated by 4ε. Classify each point against
  A and against B with `trimmed.Solid.Classify` at tolerance ε, and against
  the result at tolerance τ. Skipping
  points that any of the three reports as boundary, the result's answer must
  equal the operation applied to the answers for A and B. This check does not
  use the piece classification of section 7.7.
- V7. If an operand violates the embedding precondition (S5), `Combine` still
  never returns a result that fails V1 to V6, but it is not guaranteed to
  detect the violation.
- V8. For an empty result, V5 and V6 still run, with V(result) = 0, except
  when both operands are empty: then there is no bounding box, no check
  runs, and the empty solid is returned.

## 9. Errors

Each is a distinct value testable with `errors.Is`, wrapped with the stage,
the operand and the input face or edge indices involved.

| Error | When |
|---|---|
| `ErrInvalid` | Nil context, an operation value other than `Union`, `Common` or `Cut`, invalid options, non-finite input, ε ≥ L / 100. |
| `ErrUnsupported` | Operand outside section 4.1; a surface or curve pair marked unsupported in 7.4 or 7.5. |
| `ErrNonManifold` | N1, N2 or N3 fails. |
| `ErrTolerance` | T4, SP4 or F6. |
| `ErrInconsistent` | X8, F2, F3, K1, K2 or K3: the arrangement contradicts itself. |
| `ErrValidation` | Any check of section 8, or B4. |
| `ErrBudget` | A work limit of section 10 is exceeded. |
| Context error | The context is cancelled; returned unwrapped. It takes precedence over every other error, at every stage. |

`ErrInconsistent` and `ErrValidation` mean the trial's own logic failed on an
input in scope. Every occurrence on an acceptance or matrix case counts
against the pass condition.

## 10. Work limits

| Limit | Default | Counts |
|---|---|---|
| `MaxFaces` | 256 | Faces per operand. |
| `MaxPairs` | 65,536 | Geometric pair tests across passes X1 to X6. A pair rejected by its bounding boxes is not counted. In X5 each span tested against a face counts once. |
| `MaxNodes` | 16,384 | Nodes. |
| `MaxPieces` | 4,096 | Pieces. |

Exceeding one of these limits returns `ErrBudget`. `MaxGrowth` (default 4,
T4) is a tolerance bound, not a work limit, and its violation returns
`ErrTolerance`. Checking these limits is the one place where the number of
faces, nodes or pieces of an operand is read (PC2). The context is checked at least once
per pair test and once per piece. Loops in the output are also subject to
`brep.MaxHoledFaceVertices` (256, `brep/holes.go:15`), enforced by
`trimmed.NewSolid`.

## 11. Coincident and tangent cases

These are requirements. Each row is a test in the new package with the
stated outcome; volumes are analytic. Boxes are axis-aligned and written as
intervals; cylinders as axis line, radius r and height interval. Operations
not listed for a row follow from the rules above and are tested where a
volume is given.

| # | Configuration | Union | Common | Cut A − B |
|---|---|---|---|---|
| C1 | Disjoint boxes | Two solids, volumes summed | Empty | A |
| C2 | B identical to A | A | A | Empty |
| C3 | B strictly inside A, no contact | A | B | One solid with one void; V(A) − V(B) |
| C4 | A = [0,2]³, B = [0,2]²×[2,3]: one whole face shared, opposite normals | One box, 6 faces, V = 12 | Empty | A |
| C5 | A = [0,2]³, B = [1,3]×[0,2]×[2,3]: part of a face shared | One solid, V = 12 | Empty | A |
| C6 | A = [0,2]³, B = [0,1]³: three faces flush, same normals | A | B | V = 7, one solid |
| C7 | A = [0,2]³, B = [2,3]×[0,2]×[2,3]: one edge shared, nothing else | `ErrNonManifold` | Empty | A |
| C8 | A = [0,2]³, B = [2,3]³: one vertex shared | `ErrNonManifold` | Empty | A |
| C9 | A box; B a box rotated 45° about a horizontal axis, resting on one edge in the interior of A's top face | `ErrNonManifold` | Empty | A |
| C10 | A box; B a tilted box touching the interior of A's top face at one vertex | `ErrNonManifold` | Empty | A |
| C11 | A box; B a cylinder outside A, axis parallel to a face of A at distance r from it (touching along a line) | `ErrNonManifold` | Empty | A |
| C12 | A box; B a cylinder inside A, axis at distance r from one face (touching it along a line from inside), ends strictly inside | A | B | `ErrNonManifold` |
| C13 | Parallel cylinders, axis distance r₁ + r₂ | `ErrNonManifold` | Empty | A |
| C14 | Parallel cylinders, B inside A, axis distance r₁ − r₂, same height interval | A | B | `ErrNonManifold` |
| C15 | Coaxial cylinders, equal radius, heights [0,2] and [1,3] | One cylinder, 3 faces, V = 3πr² | V = πr² | V = πr² |
| C16 | Coaxial cylinders, equal radius, heights [0,2] and [2,4]: caps shared, opposite normals | One cylinder, 3 faces, V = 4πr² | Empty | A |
| C17 | Coaxial cylinders: A of radius R and height interval [0,2], B of radius r < R and height interval [−1,3] | A with B's two ends standing out; V = 2πR² + 2πr² | V = 2πr² | Tube, V = 2π(R² − r²), 4 faces |
| C18 | Parallel cylinders of radius r, axis distance d < 2r, same height h | V = h(2πr² − lens) | V = h·lens, lens = 2r²·acos(d/2r) − (d/2)·√(4r² − d²) | V = h(πr² − lens) |
| C19 | A box; B a cylinder standing on A's top face, base circle strictly inside the face | One solid, V(A) + V(B); the top face has one hole | Empty | A |
| C20 | As C19, but the base circle touches one edge of the top face from inside at a point | `ErrNonManifold` | Empty | A |
| C21 | A box; B a cylinder through A along z with both caps flush with A's faces | A | B | Box with a through bore, V(A) − πr²h, 7 faces |
| C22 | A box; B a cylinder entering A's top face and ending inside A | V(A) + part of B outside | Part of B inside | Blind hole; V(A) − πr²·depth |
| C23 | A box; B a cylinder whose axis is oblique to the faces it passes through | — | Part of B inside A | Through bore with elliptic openings; V(A) − V(common) |
| C24 | C4 with B moved away from A by ε / 2 | As C4 within β | Empty | A |
| C25 | C4 with B moved into A by 100ε | One solid, V = 12 − 400ε within β | V = 400ε within β | V = 8 − 400ε within β |
| C26 | C4 with B moved into A by 2ε | `ErrTolerance` or a validated result (T8) | same | same |
| C27 | Either operand empty | The other operand | Empty | A if B is empty, else empty |

In C7 to C14 and C20 the result where an error is listed is a point set whose
boundary is not a manifold. The trial reports it; it does not represent it.
See OD-3.

## 12. Compositions used by the bridge

- CO1. `cutMany(body, tools)` is the left fold `(…((body − t₁) − t₂)…)`.
  `joinAll(bodies)` is the left fold of union. Either is atomic: if any step
  errors, the whole operation errors and nothing is published. Argument
  validation of the bridge operations is unchanged by the trial: a request
  that the bridge rejects today (for example an empty list of bodies or
  tools) is rejected the same way before `Combine` is called, and a list of
  one body is returned as that body.
- CO2. `overlapVolume(a, b)` is V(a ∩ b), and 0 when the common is empty.
- CO3. `prismToFace` and `circleToFace` build a tool that ends on a target
  face. Their result is reproduced as the common of the sweep, extended past
  the target plane, with a block one face of which lies on the target plane.
- CO4. `pocketFloor` is a query on a result, not a Boolean. It must give the
  same answer on a general-path result as on the legacy result, which
  depends on M5 producing one face per floor.

## 13. Existing packages

What the trial uses, unchanged, and what it does not.

| Package | Used as is | Not used, and why |
|---|---|---|
| `geom` | `Vec3`, `Vec2`, `Box`, `Bounds` (`vector.go:47,75`); `Frame`, `Line`, `Ellipse`, `NewCircle`, `PlaneSurface`, `CylinderSurface` (`analytic.go:18-182`); accessors (`parameters.go:6-8`); `Line.Closest` (`intersection.go:34`); `PlaneSurface.Project`, `CylinderSurface.Project` (`projection.go:58,68`); `IntersectionTolerance`, `IntersectLinePlane`, `IntersectPlanes` (`intersection.go:19,50,80`); `IntersectPlaneCylinder` (`sections.go:52`); `IntersectLineCylinder` (`line_surfaces.go:120`); `IntersectLineEllipse` (`planar_intersections.go:36`); `Orient2D` (`vector.go:91`) | The perpendicular, skew and bisector cylinder helpers (`perpendicular_cylinders.go`, `skew_orthogonal_cylinders.go`, `nonparallel_cylinder_query.go`) are exact, budget-only and outside the Tier 1 class; `IntersectEqualRadiusCylinderBisectors` (`equal_radius_cylinder_bisectors.go:41`) is a candidate for Tier 2 only. Nothing is added to `geom`: it has about 1,300 files and every package imports it. |
| `brep` | `Solid` as a bridge operand, through `trimmed.FromPlanar`; `MaxHoledFaceVertices` (`holes.go:15`) | `SplitPlanarEdges` (`split_planar_edges.go:76`) is exact, planar and never snaps. |
| `brep/parametric` | `Edge`, `Interval` (`solid.go:21,36`); `Location` (`ball_query.go:10`); `IntegrationOptions`, `Properties` (`properties.go:13`); `ErrBudget` (`solid.go:17`); `SurfaceMapping`, `CurveMapping` (`transform.go:86,95`); `PolarPlaneFrame`, `Cylinder` (`primitives.go:82,134`) | `Face` is a rectangular patch with no trims. The `cylinder_pair_*`, `oblique_*` and `sphere_*` constructors are the enumeration this trial is an alternative to. |
| `brep/trimmed` | `Solid`, `Face`, `Use`, `PCurve`, `NewSolidContext` (`solid.go:19-110`); `EllipseUV` (`pcurve.go:14`); `FromPlanar`, `FromParametric` (`transform.go:17,137`); `Solid.Classify`, `Solid.Ray` (`location.go:27,63`); `Solid.MassProperties` (`properties.go:33`); `Solid.Bounds` (`bounds.go:15`); `Solid.Tolerance`, `Solid.Topology` (`topology.go:93`); `Solid.Tessellate` (`solid.go:489`) in tests | `Sew` (`sew.go:42`) joins only exactly equal endpoints with a proved common evaluator and never welds within tolerance; the arrangement already shares edges. `ValidateTrimLoops` and the other functions of `validation.go` wrap the exact certifier and leave ellipse p-curves unresolved. `ValidateEdgeAgreement` (`edge_validation.go:35`) handles rational Bézier edges only. Every named-arrangement constructor. |
| `brep/trimvalidate` | Nothing | Exact, chart-only, no tolerance parameter; it does not check model-space closure, edge-on-surface or vertex-on-edge residuals, which is what section 8 needs. |
| `intersect` | Nothing | Spline and marching oriented. `Trace` (`trace.go:113`) returns `ErrSingular` on tangency and does not detect coincidence. It is the likely home of a later surface pair that has no closed form. |
| `csg`, `csg/poly` | `poly.Combine` (`csg/poly/history.go:79`) in tests only, as an independent volume check on all-planar cases | `csg.Union` and `csg.Intersect` (`boolean.go:37`, `intersect.go:37`) are convex-only. `poly.Combine` is exact over a tessellation and returns triangles, so it cannot carry cylinders. |

- E1. The trial needs from `brep/trimmed` that `NewSolidContext` accept every
  result that meets B2 and B3, and that `Classify` answer inside, outside or
  boundary for every operand in the class at tolerance ε. If either fails on
  a valid input, the trial does not edit `brep/trimmed`: it records the case
  and stops under section 16.

## 14. Code location

- L1. All new kernel code is in `brep/boolean`. It imports `geom`, `brep`,
  `brep/parametric` and `brep/trimmed` and nothing else from this module. The
  closed forms of SP1 and CC1 are unexported there.
- L2. The outside caller of `Combine` (G2) is one new file in
  `internal/plaincadbridge`. It converts a stored shape to a `trimmed.Solid`
  where the shape already holds one or lifts to one (planar solids through
  `trimmed.FromPlanar`, cylinders and patch solids through
  `trimmed.FromParametric`, bored and analytic bodies through the solid they
  hold), calls `Combine`, and publishes the result through the existing
  analytic-body path. A stored shape with no such solid returns the bridge's
  `UNSUPPORTED` failure.
- L3. The general path is selected per `Bridge` value by an unexported field
  set through a constructor used only by tests. The default bridge behavior
  does not change during the trial, and there is no environment variable or
  package-level switch.
- L4. Bridge tolerances for the trial: `Linear` 1e-7, `Relative` 0,
  `Angular` 1e-9, `Volume` 1e-7, integration relative tolerance 1e-9.
- L5. No file in `brep/trimmed`, `geom` or `specs/brep-trimmed-split` is
  changed. No dependency is added.
- L6. The oracle helper of A6 is `tools/occtref/general_boolean.cxx` with one
  Go test in `internal/occtcheck`, enabled by an environment variable naming
  the built helper, following `tools/occtref/README.md`.

## 15. Acceptance criteria and pass condition

### 15.1 Acceptance cases

The cases are existing bridge tests that today run through special-case
constructors. Each row below names test files in `internal/plaincadbridge`;
the acceptance set of a row is every test function in those files that issues
`cut`, `fuse`, `common`, `cutMany`, `joinAll`, `overlapVolume`, `pocketFloor`,
`prismToFace` or `circleToFace`, less the exclusions of 15.2. Counts are from
a survey on 2026-11-18 and are approximate until A1 freezes them.

Tier 1, required:

| Family | Test files | Tests | Arrangement | Legacy path |
|---|---|---|---|---|
| Through bore | `bore_test.go`, `bore_mesh_test.go` | 5 | Box less a through cylinder, every axis and sign | `newBore`, `trimmed.BoreCylinders` |
| Blind bore | `blind_test.go`, `bore_contact_test.go` | 13 | Blind holes in a box | `trimmed.BoreCylinders`, `stopBore` |
| Bore layers | `bore_layers_test.go` | 4 | Coaxial and parallel bores of different depths; box ∩ box | `newBore` layered |
| Bore union | `bore_union_test.go` | 5 | Overlapping parallel bores | `newUnionBore`, `trimmed.BoreCylinderUnion` |
| Opposed blind | `opposed_bore_test.go`, `opposed_blind_bore_test.go` | 5 | Opposed blind bores, coaxial or offset | `mergeOpposedBlindBores` |
| Oblique through | `oblique_bore_test.go` | 3 | Oblique cylinder through a box | `trimmed.BoreCylinderAlong` |
| Disjoint bores | `disjoint_bores_test.go`, `disjoint_blind_bores_test.go`, `disjoint_finite_bores_test.go`, `disjoint_radial_bores_test.go`, `disjoint_oblique_bore_test.go` | 5 | Two or three bores on different axes that do not meet | `trimmed.BoreCylinders`, `newDisjointObliqueBore` |
| Side and corner cylinder | `side_cylinder_test.go`, `multi_body_corner_test.go` | 12 | Cylinder breaking a side or corner of a box | `cornerCylinderBoolean`, `trimmed.BoxSideCylinder*`, `trimmed.BoxCornerCylinder*` |
| Side opening pair | `side_opening_pair_test.go` | 7 | Parallel blind bores opening a side wall | `trimmed.BoxSideOpeningCuts` |
| Side dual disk | `side_dual_disk_test.go` | 8 | Two overlapping parallel side-open bores | `trimmed.Cut*SideOpenDualDisks` |
| Circular stock | `circular_stock_bore_test.go` | 7 | Axial bores in cylindrical stock | `cutCircularStockBores` |
| Coaxial cut | `tube_step_test.go`, `cylinder_sector_step_test.go`, `quarter_annular_step_test.go` | 4 | Coaxial bore through a cylinder or sector | `coaxialThrough` |
| Coaxial join | `coaxial_join_test.go` | 4 | Cylinder joined to a coaxial cylinder | `fuseCoaxial` |
| Join all | `join_all_test.go` | 2 | Boxes bridged by a box; boxes inside a cylinder | `planarUnion`, containment |
| Containment overlap | `cylinder_containment_overlap_test.go`, `overlap_capture_test.go` | 2 | Box inside a large cylinder | `containedPlanarCylinderVolume` |
| Pocket and planar | `box_pocket_test.go`, `cut_many_test.go`, `planar_boolean_chain_test.go` | 9 | Box pocket with bore, nested pocket, box less inner box, the one `pocketFloor` test | `rectangularPocket`, `poly.Combine` |
| Pocket with cylinder | `pocket_cylinder_cut_test.go`, `retained_planar_bores_test.go`, `island_pocket_test.go` | 5 | Hole in a pocket floor, counterbore, refill | `cutPocketCylinder`, `retainedPlanarBoolean` |
| Rounded profiles | `rounded_stack_test.go`, `rotary_platter_test.go`, `annular_pedestal_test.go` | 3 | Rounded-rectangle and annulus extrusions with axial bores | `cutRoundedStack`, `cutThroughProfiles` |
| To face | `to_face_test.go`, `to_face_analytic_test.go`, `to_face_oblique_test.go` | 7 | Extrude a polygon or circle up to a planar face (CO3) | `brep.PrismToFace`, `trimmed.CylinderToFace` |

Tier 2, required only if OD-1 keeps it in the trial:

| Family | Test files | Tests | Arrangement | Legacy path |
|---|---|---|---|---|
| Crossed bores | `crossed_bore_test.go`, `blind_crossed_bore_test.go`, `double_blind_crossed_bore_test.go`, `contained_crossed_bores_test.go`, `clipped_crossed_bore_test.go` | 5 | Two equal bores with meeting axes | `newCrossedBore`, `newBlindCrossedBore` |
| Triple and companion | `triple_crossed_bore_test.go`, `blind_triple_crossed_bore_test.go`, `crossed_companion_bore_test.go` | 4 | Three equal bores; a crossed pair with a third apart | `newTripleCrossedBore`, `newCrossedCompanionBore` |
| Crossed oblique | `crossed_oblique_bore_test.go` | 8 | A bore crossed by an oblique equal bore | `newObliqueCrossedBore` |

### 15.2 Exclusions

Not acceptance cases, with the reason:

- Quartic intersection edges (SP3): `unequal_crossed_bore_test.go`,
  `partial_skew_bore_test.go`, `finite_partial_skew_bore_test.go`,
  `finite_unequal_bore_test.go`, `skew_companion_bore_test.go`.
- Non-manifold by design: `tangent_bore_test.go`, `tangent_reconcile_test.go`,
  `far_tangent_crossed_bore_test.go`. The general path must return
  `ErrNonManifold` for these, which section 11 tests directly.
- Operand held only as a recipe with no `trimmed.Solid` (edge-treated stock):
  `edge_chain_oracle_test.go`, `retained_bore_edges_test.go`,
  `bored_cap_edge_chamfer_test.go`, `pocket_chamfer_fallback_test.go`.
- Outside the surface class: cones, tori, threads, lead screws, rim
  treatments and STEP repair families.
- Within an acceptance test, assertions about STEP, IGES, native proof JSON,
  face roles and edge treatments are not reproduced (section 4.4).

### 15.3 How a case is checked

- A1. The first task of the plan freezes 15.1 into
  `specs/general-boolean/ACCEPTANCE.md`: one row per test function, with its
  tier, the bridge requests it issues and the expected values. A function is
  dropped only for a reason in 15.2, stated in its row.
- A2. One table-driven test in `internal/plaincadbridge` replays each row's
  requests on a bridge with the general path selected (L3) and on a default
  bridge.
- A3. For every shape or volume the row produces, the general path must
  succeed and:
  - `valid` is true and `solidCount` equals the legacy value;
  - volume and surface area equal the analytic value in the source test
    where it has one, and the legacy value in every case, within
    1e-7 × max(1, |expected|);
  - bounds equal the legacy bounds within 1e-7 in each coordinate;
  - the mesh of the result is watertight under the bridge's existing mesh
    checks.
- A4. Where the source test expects a failure, the general path must return a
  failure too.
- A5. The section 11 matrix passes in `brep/boolean`.
- A6. Oracle. One helper builds a case from data (boxes, prisms and cylinders
  with placements, combined by union, common and cut) with the pinned OCCT
  and reports validity, volume, area, centroid and solid count. Cases are
  rows of a data file; there is no per-case C++. It is run once, at the end
  of the trial, on every section 11 row that returns a solid and on one row
  per Tier 1 family, with the comparison bound of
  `internal/occtcheck/oracle_test.go:68` (3e-7 relative for area and volume,
  3e-7 absolute for centroid). Face counts are compared for the section 11
  rows that state one. See OD-5.
- A7. Every acceptance row and matrix row runs in under two seconds under
  `go test` on the development machine. A row that needs longer is a failure
  of the row, not a candidate for the slow tier.

### 15.4 Pass condition

The trial passes when all of these hold on one commit:

- PC1. Every Tier 1 row passes A3 and A4, section 11 passes (A5), and the
  oracle run passes (A6). `ErrInconsistent` or `ErrValidation` on any of them
  is a failure.
- PC2. One path. `brep/boolean` exports one operation entry point; the bridge
  file of L2 reads a stored shape only to obtain its `trimmed.Solid`; and no
  function in either tests the number of faces, edges or vertices of an
  operand, the primitive it was built from, or its placement, except through
  sections 7.3 to 7.5 and the work limits of section 10. The architectural
  constraints of AR8 hold. This is checked by review of the complete diff.
- PC3. `go build ./...`, `go vet ./...`, `golangci-lint run ./...` and
  `go test ./...` pass. `go test ./...` without `PLAINSOLID_SLOW` runs the
  default test tier of `tools/ci/README.md`, which `AGENTS.md` requires
  before a merge; it is not the complete suite that section 4.4 excludes.
- PC4. Non-test Go in `brep/boolean` is at most 12,000 lines, and the bridge
  file of L2 at most 400.
PC1 to PC4 decide the pass. Tier 2 does not: if OD-1 keeps Tier 2 in the
trial, it is attempted only after PC1 to PC4 hold, by changes to the tables
of 7.4 and 7.5 and their tests only (SP5). Tier 2 is *supported* when its
rows then pass A3 and A4 with PC2 to PC4 still holding; otherwise it is
reported as unsupported, the change is not kept, and the pass stands.

On a pass, the report to the owner lists the bridge paths and kernel
constructors that no acceptance row needs any longer. From the survey these
are expected to be, for Tier 1: `newBore` with `trimmed.BoreCylinders` and
`stopBore`, `newUnionBore`, `mergeOpposedBlindBores`,
`trimmed.BoreFiniteCylinders`, `newObliqueThroughBore`,
`newDisjointObliqueBore`, `cornerCylinderBoolean` and its four
`trimmed.Box*Cylinder*` constructors, `cutSideOpeningFamilies` and the five
`trimmed.Cut*SideOpenDualDisks`, `cutSideNotchBores`, `cutCircularStockBores`,
`coaxialThrough`, `fuseCoaxial`, `cutPocketCylinder`, `rectangularPocket`,
`rectangularIslandPocket`, `cutRoundedStack`, `cutRoundedBores`,
`cutThroughProfiles`, `retainedPlanarBoolean`,
`containedPlanarCylinderVolume` and `parallelCylinderOverlap`; for Tier 2:
`newCrossedBore`, `newBlindCrossedBore`, `newTripleCrossedBore`,
`newCrossedCompanionBore` and `newObliqueCrossedBore`. Deleting them is not
part of the trial.

## 16. Stop condition

The trial ends as a failure, and work stops with a report to the owner, at
the first of:

- ST1. Wrong result. `Combine` returns a result that passes section 8 and
  differs from the analytic or oracle value beyond A3 or A6, and no general
  strengthening of section 8 catches it.
- ST2. Special-casing. The only known way to make a Tier 1 row pass is code
  that violates PC2.
- ST3. Not converging. After the last planned phase and one corrective
  phase, any Tier 1 row or section 11 row still fails.
- ST4. Size. PC4 is exceeded before PC1 holds.
- ST5. Foundation. E1 fails on a valid input, or a Tier 1 row cannot pass
  without changing `brep/trimmed`, `geom` or adding a dependency.
- ST6. Design. A Tier 1 row needs a change to sections 7.2, 7.3 or 7.6 to
  7.11 that this document does not allow, as opposed to an addition to the
  tables of 7.4 and 7.5.

A Tier 2 failure does not fail the trial. It is reported, and the crossed
bore families stay.

On a stop: the branch is left unmerged with a commit labeled work in
progress; the legacy constructors and default bridge behavior are untouched;
the report states which condition was met, the failing rows, and the smallest
input that shows the failure.

## 17. Records

- RC1. The trial edits `docs/STATUS.md`, `docs/VERIFICATION.md` and
  `README.md` only if it passes and the owner merges it; until then its
  record is its commit messages and `specs/general-boolean/`.
- RC2. An oracle run (A6) adds one file under `docs/evidence` only if the
  verification record then cites it.

## 18. Open decisions for the owner

A plan must not be written until these are answered. Each states the
default this document assumes.

- OD-1. Tier 2. Equal-radius cylinders with meeting axes give two ellipses,
  which is inside the edge class, and cover about 17 bridge tests and the
  crossed-bore constructors. They also meet at two points where the surfaces
  are tangent. Assumed: in the trial as Tier 2, added last, as a test of SP5,
  and not required for a pass. Alternative: out, returning `ErrUnsupported`.
- OD-2. Unequal and skew crossed bores have quartic intersection curves and
  are out (SP3). Their constructors and five test files stay whatever the
  trial's outcome. Assumed: accepted.
- OD-3. Touching results. A result whose boundary is not a manifold returns
  `ErrNonManifold`, as `csg/poly` and the bridge's tangent-bore recipe do
  today. This includes the union of two solids that touch only along an edge
  or at a vertex. Alternative for that one case: return the two solids
  unjoined. Assumed: error.
- OD-4. Bounds of the trial: 12,000 non-test lines (PC4), one corrective
  phase (ST3) and two seconds per case (A7). Assumed: as written.
- OD-5. The oracle run (A6) needs the pinned OCCT build, which `AGENTS.md`
  reserves for an owner request. Assumed: approving this specification is
  that request, for one run at the end of the trial.
- OD-6. Reading OCCT during implementation. The headers of the pinned source
  carry a notice that using it as a reference to produce derivative code
  requires compliance with its license. This document was written from its
  structure only (G3). Assumed: implementation sessions work from this
  document and the cited literature and do not open the OCCT source.
- OD-7. Accuracy. The general path integrates volume and area numerically,
  so A3 uses 1e-7 where the existing bridge tests use 1e-8 against
  closed-form values. Assumed: 1e-7 is acceptable for the trial.
- OD-8. Edge-treated stock is held as a recipe and is excluded (15.2).
  Assumed: accepted; lifting it to a `trimmed.Solid` is separate work.
