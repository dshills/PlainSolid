# General Boolean trial: planes and cylinders

Status: owner decisions recorded on 2026-11-18 (section 18). Rulings made
on the same day by the reviewing session, under the owner's delegation, are
in section 19 and wait for the owner to confirm or reverse them. This
document is a specification only and contains no implementation.

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
section. Rows of the section 11 matrix are C1 to C28. "Must" states a
requirement. OCCT is Open CASCADE Technology, the reference kernel pinned in
`tools/occtref/README.md`. STEP (ISO 10303) and IGES are CAD exchange
formats.

## 2. Governing decisions

- G1. The tolerance policy adopted by the owner on 2026-11-18 governs this
  work. Its text is the "Tolerance policy" section of `docs/SPECIFICATION.md`
  on `main`. In short: explicit tolerances
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
  and Solid Modeling", 1989). Implementation sessions work from this document
  and that literature and do not open the OCCT source (OD-6).
- G4. The trial does not edit `brep/trimmed`. It uses only identifiers
  declared in files of `brep/trimmed` that are in the default build: a file
  is in the default build when it does not start with `//go:build research`.
  The trial does not extend a research file and does not remove the tag from
  one (`AGENTS.md`). A use of a research identifier does not compile in the
  default build, so PC3 enforces this, and the standing check
  `go run ./tools/refgraph -tags research -pkg brep/trimmed -exclude examples -check research`
  must still pass (PC3).

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
  under a rigid placement reported by `parametric.SurfaceMapping`. Unlike
  an edge (S3), a face is recognized by its surface value, through these
  exported types and accessors, because nothing next to a face determines
  its carrier. A face stored as any other evaluator is outside the class
  and returns `ErrUnsupported` (S5), also when it is geometrically a plane
  or a cylinder: the arc-loop extrusion behind one branch of the bridge's
  `mixedExtrude` request stores its caps as B-spline patches and its walls
  as rational Bézier patches, and is rejected for that reason (15.2).
- S3. Every edge lies on a line, a circle or an ellipse (a circle is an
  ellipse with equal radii). The stored curve of an edge is used only by
  evaluating it over the edge's range, through `geom.Curve`. Its Go type is
  never inspected, so every evaluator is accepted, including the ones other
  packages do not export: an edge of a `parametric.Cylinder` is a side of a
  surface patch, and an edge of an extruded profile is a p-curve mapped
  through a surface. The carrier of an edge is derived by I5. Collapsed
  edges (nil curve, equal end vertices) are accepted only as the pole of a
  polar plane chart and carry no geometry into the arrangement.
- S4. A placement is rigid when its linear part M satisfies
  ‖MᵀM − I‖∞ × L ≤ ε and det M > 0, with L and ε of T1. Within this bound
  the placed surface differs from a true plane or circular cylinder by no
  more than about ε over the operands, and the carrier is built from M
  with its columns made orthonormal. A placement outside the bound, such
  as a non-uniform scaling, makes the face `ErrUnsupported` (S5).
- S5. An operand outside S1 to S4 is rejected with `ErrUnsupported` naming
  the first offending face or edge index, except where I5 names another
  error for an edge. Embedding (S1, no self-intersection)
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
- Modeling history, persistent naming of faces, edges or vertices, and
  ancestry of edges and vertices. The only ancestry in the contract is the
  face source list of P5. It names faces of the two operands of one call by
  index, says nothing about how a face was changed, and is not carried from
  one call to the next.
- Operands held by the bridge only as a recipe with no `trimmed.Solid`,
  which is how edge-treated stock is held (OD-8).
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
    tests, in (0, 0.5], with the meaning of `geom.IntersectionTolerance.Angular`
    (`geom/intersection.go:19`).
  - `Volume`: relative bound for the volume identities (V5), > 0.
  - `Samples`: number of membership samples (V6); zero selects 64.
  - Work limits (section 10); zero selects the stated default.
  - Integration options for mass properties, of type
    `parametric.IntegrationOptions`.
- P4. `Linear` and `Relative` must not both be zero, `Angular` must be > 0 and at most 0.5, so that no pair of directions is
  both parallel and perpendicular under T5, and
  `Volume` must be > 0. There is no hidden default tolerance. `Samples` and
  every work limit must be ≥ 0, and a nonzero `MaxGrowth` must be ≥ 1. Every
  option must be finite. A violation returns `ErrInvalid` before any
  geometry is examined.
- P5. The result carries: the output solid; for each shell its face indices,
  whether it is an outer shell or a void, and the index of the solid it
  belongs to; the number of solids; for each face of the output solid its
  source list; and a report. The source list of a face is the set of input
  faces it came from, each given as an operand (A or B) and the index of the
  face in that operand's face list. It has no duplicates and is ordered A
  before B, then by index. It is built from the pieces the face is made of:
  a piece contributes its own input face (F5); a kept shared piece (K1)
  contributes both faces of its same-surface pair; and a face made by M1
  has the union of the lists of the faces merged into it. Each of the two
  faces made by M6 has the list of the face that was divided. A piece of B
  kept with reversed orientation by a cut still names its face of B. Every face
  of a non-empty result has at least one source. The report contains ε, the
  largest node radius, counts of nodes, spans, bundles, traces and pieces,
  the volumes of A, B and the result, the largest edge and vertex residuals
  found by validation, and the list of touching contacts found.
- P6. On error the result is the zero value. A partial result is never
  returned.
- P7. `Combine` does not modify its operands and holds no package-level
  mutable state. Two calls with equal operands, operation and options return
  equal results.

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
  or surface when its distance to it is at most its radius. Two surface
  carriers are the same when they satisfy SP4, and two curve carriers are
  the same over an interval when they satisfy CC4.
- T4. A node radius must not exceed `MaxGrowth` × ε (default 4). A merge that
  would exceed it returns `ErrTolerance`, not `ErrBudget`. This stops chains of near
  coincidences from collapsing real geometry. The *filter margin* is
  μ = `MaxGrowth` × ε, the largest radius a node can have. Two points can
  be coincident (T3) only when they are at most 2μ apart. So every
  bounding box that is used to discard a pair without testing it is taken
  from the entities as they stand when the pass runs, with nodes at their
  merged positions, and is inflated by μ; and every proof that two things
  do not meet requires a gap greater than 2μ. Filtering can then never
  discard a pair that the coincidence rule would accept.
- T5. Angular decisions use `Angular`: directions are parallel when the
  absolute value of the sine of their angle is at most `Angular`, and
  perpendicular when the absolute value of the cosine is.
- T6. Tolerances live on nodes inside the arrangement only. The output solid
  records one value, τ = the largest node radius used by the result, as its
  `trimmed.Solid` tolerance. An empty result is the zero `trimmed.Solid`,
  which can record no tolerance; τ is then ε, and the ε of the report (P5)
  is where a caller reads it. Per-vertex and per-edge tolerances in the
  stored solid are not part of the trial.
- T7. There is no separate widening ("fuzzy") option. A caller widens
  coincidence by passing a larger `Linear`.
- T8. Where `ErrTolerance` is allowed. Four separations are measured on the
  two operands together. The points concerned are the vertices of both
  operands and the isolated points that sections 7.4 and 7.5 give for
  pairs of their carriers.
  - (a) The distance between two such points.
  - (b) The distance from such a point to an edge or a face of the other
    operand, taken to the trimmed edge or the trimmed face.
  - (c) For a pair of carriers that SP4 or CC4 tests for coincidence, the
    distance between them at each point where they test it.
  - (d) For each piece, the largest distance from one of its points to its
    boundary.

  A value of (a), (b) or (c) is a *coincidence* when it is at most ε and
  *clear* when it is at least 4ε. A value of (d) is clear when it is at
  least 4ε. A configuration is *tolerance-scale* when at least one of
  these five conditions holds:
  1. A pair of carriers is *divergent*: SP4 or CC4 finds it coincident at
     some of the places it tests and not at others. This condition does
     not depend on the values of (a) to (d). It holds also when every one
     of those values is a coincidence or clear, for example when the
     places that are not coincident are 4ε or more apart. Such a pair is
     parallel only by the angular tolerance, and is neither one surface or
     curve nor two that are clear of each other.
  2. An edge has two candidate carriers that both qualify (I5).
  3. A value of (a), (b) or (c) lies strictly between ε and 4ε.
  4. A value of (d) is below 4ε.
  5. An edge that is not closed has both of its end vertices in one node
     (X10): the edge is shorter than the tolerance.

  A configuration is *ordinary* only when none of the five holds. On an
  ordinary configuration `Combine` must not return `ErrTolerance`. On a
  tolerance-scale configuration it returns either `ErrTolerance` or a
  result that passes all of section 8, and never an unvalidated result.
  Which of the two is not specified, except under conditions 1, 2 and 5,
  where the outcome is fixed: `ErrTolerance` (SP4, CC4, I5, X10). So
  wherever SP4, CC4, I5 or the short-edge rule of X10 requires
  `ErrTolerance`, the configuration is tolerance-scale by condition 1, 2
  or 5, and those rules and this one cannot disagree. The other places
  that return `ErrTolerance` (T4, X4, X5, the other two cases of X10, and
  F6) are bound by this rule: they must not do so on an ordinary
  configuration.

  The guarantee for ordinary configurations holds when `MaxGrowth` is at
  least 4, which includes the default. In an ordinary configuration the
  points that merge into one node are within ε of each other, so the node
  radius stays below 2ε and T4 is not reached. A caller who sets
  `MaxGrowth` below 4 asks for stricter rejection, and T4 may then return
  `ErrTolerance` on any configuration in which points merge. The rows of
  section 11 are run with the default.

  C26 is tolerance-scale by condition 3 (a value of (b) is 2ε). C24 and
  C25 are ordinary: in C24 every node of each face is within its radius of
  the other plane, so the pair is coincident everywhere, and in C25 none
  is, so no pair is divergent.

## 7. Functional behavior: the pipeline

There is one pipeline. Every input in the class of section 4.1 passes through
the same stages in the same order. No stage inspects how many faces an
operand has, which primitive it came from, or how it is placed, other than
through the general tests below, the work limits of section 10 and the
test for the empty solid (S6, I2).

### 7.1 Intake

- I1. Validate options (P4), the context and both operands (S1 to S6).
- I2. Short cuts that are part of the contract, decided before any pairwise
  work: if either operand is empty, or the bounding boxes of the two
  operands, each inflated by μ (T4), do not overlap, the result follows
  table 7.8 with every piece of A classified outside B and every piece of
  B outside A. The same stages 7.9 to 7.11 still produce and validate the
  result. An operand is empty when it has no face; this is the one test
  of a face count outside section 10 (PC2).
- I3. Reduce every face to its carrier (plane: point and unit normal;
  cylinder: axis point, unit axis direction, radius), its orientation
  relative to the carrier normal, and its loops as chains of edge uses in a
  chart. Reduce every edge to its carrier and parameter interval (I5).
- I4. A cylinder face is cut along one seam line parallel to its axis so that
  it has a single chart. A seam is an ordinary edge of the arrangement with
  two uses by the same face. The seam of an input cylinder face that already
  has one is kept where it is. A planar face given in a polar chart has a
  radial edge that the face uses twice, from its pole to its rim; it is
  treated as a seam too. M1 removes it, since both uses are by pieces on
  one plane, and the pole node, then on no edge, is not in the result.
- I5. Edge carriers. The carrier of an edge follows from the faces it lies
  on, and the stored curve is only checked against it. For each edge that
  is not collapsed:
  1. Samples. The stored curve is evaluated at 17 evenly spaced parameters
     of its range, both ends included. Step 6 adds samples on a circle or
     ellipse where the stored parameterization is uneven; every sample,
     first or added, is subject to steps 2 and 5. An evaluation error or a
     non-finite point returns `ErrInvalid` naming the edge.
  2. Faces. Let S₁ and S₂ be the carriers of the faces of the edge's two
     uses; for a seam they are one carrier. Every sample must be within ε
     of S₁ and of S₂. Otherwise the operand is not a valid boundary at this
     tolerance and the call returns `ErrInvalid` naming the edge.
  3. Candidates when S₁ and S₂ are different surfaces. S₁ and S₂ are the
     same surface when they are the carrier of one face (a seam), parallel
     planes, or a candidate pair of parallel cylinders (SP4); given step 2,
     such a pair agrees within tolerance wherever the edge runs. Any other
     pair is two different surfaces. For those, the candidates are
     the carrier curves that the table of section 7.4 gives for the pair
     (S₁, S₂), taken as unbounded carriers: a line, a circle, an ellipse,
     or two lines. This is the same table that X6 uses, so a pair that the
     table does not support makes the edge unsupported, and a row added to
     the table later admits operands with such edges without a change here.
  4. Candidates when S₁ and S₂ are the same surface (two faces on one
     carrier, or a seam). The faces do not determine a curve, so the
     samples do. If every sample is within ε of the line through the two
     samples that are farthest apart, that line is the one candidate.
     Otherwise take three samples: the first, the one farthest from it, and
     the one farthest from the line through those two. On a cylinder, the
     candidates are the carrier curves that the table of 7.4 gives for the
     plane through the three samples with the cylinder. On a plane, the
     candidate is the circle through the three samples if every sample is
     within ε of it, and otherwise the conic through the samples of index
     0, 3, 6, 9 and 12, which must be an ellipse.
  5. Choice. The carrier of the edge is the candidate that every sample is
     within ε of. If no candidate qualifies, the stored curve does not
     agree with a line, circle or ellipse on its faces, and the call
     returns `ErrUnsupported` naming the edge and its largest deviation.
     If two candidates qualify, it returns `ErrTolerance`.
  6. Interval and direction. S3 accepts any parameterization of the stored
     curve, so this step does not assume it is even. Each sample is given
     its parameter on the carrier (CC1). The advance from one sample to the
     next is the difference of those parameters, measured the shorter way
     round on a circle or ellipse.
     - On a line. The carrier is oriented from the first sample to the
       last, and every advance must be positive. One that is not returns
       `ErrUnsupported` naming the edge, at once: a sample between two
       samples of a line cannot make a backward step forward, so no
       sample is added.
     - On a circle or ellipse, direction. A step that covers more than
       half a period is measured the wrong way round, and only such a
       step is. An edge covers at most one period, so at most one of the
       sixteen first steps can be of that kind. The direction of the edge
       is therefore the direction that more of the sixteen first advances
       have, an advance of zero counting for neither. If the two
       directions have equal numbers, the call returns `ErrUnsupported`
       naming the edge.
     - On a circle or ellipse, refinement. A step is refined when its
       advance is more than one eighth of a period or is against the
       direction of the edge: the stored curve is evaluated at the middle
       of that step of its own parameter and the new sample is inserted.
       This repeats on the steps so made. A step measured the wrong way
       round is always refined, because its advance is either against the
       direction or more than an eighth of a period.
     - Bound. No step of the first sixteen is halved more than six times,
       so at most 1,025 samples are taken. If an advance is still too
       large or still against the direction after six halvings, the call
       returns `ErrUnsupported` naming the edge. A parameterization on a
       circle or ellipse is therefore accepted when it never reverses, it
       advances in at least three of the sixteen first steps, and no
       1/1024 of its range covers more than one eighth of the period.

     The carrier is oriented so that its parameter increases in the
     direction of the edge, and the edge's interval runs from the parameter
     of the first sample to that of the last, the advances being summed. A
     closed edge, whose first and last samples coincide while the advances
     sum to one period, has the whole period as its interval.

  After I5 the stored curve is not used again. Agreement is checked at the
  samples taken and nowhere else. A stored curve that leaves its carrier
  between samples is not detected, and V7 applies to it as it does to an
  operand that is not embedded.

### 7.2 Arrangement

The arrangement is the only data passed from the intersection stage to the
building stage. It holds:

- AR1. Nodes: position, radius, and the input vertices merged into each.
- AR2. For each input edge, the ordered list of nodes on it and the spans
  between them. A closed edge, such as a full circle, has at least one
  node, that of its vertex. With exactly one node it has one span, a
  *closed span*, both of whose ends are that node.
- AR3. Bundles: for each set of coincident spans, the member spans and their
  relative directions. The members of a bundle are all closed spans or all
  open spans.
- AR4. Traces: carrier, end nodes, the face of A and the face of B it lies
  in, and whether the contact is crossing or touching.
- AR5. For each input face, three lists: spans and traces that lie in its
  interior, spans of the other operand that lie along its boundary, and nodes
  that lie in its interior without an incident span in the face.
- AR6. Same-surface pairs, each with a flag saying whether the two faces have
  the same or opposite outward normals.
- AR7. Touching-contact records: the locus (a node, or a span or trace) and
  the faces of A and of B that meet there.

### 7.3 Intersection stage

The stage runs six passes in this fixed order. Lower-dimensional coincidences
are settled first, so that a later pass attaches what it finds to nodes that
already exist instead of creating near duplicates. Each pass tests only pairs
whose bounding boxes, each inflated by the filter margin μ (T4), overlap,
and considers only pairs with one member from A and one from B. The box of
a vertex, span or face is taken when the pass runs, from the current node
positions.

- X1. Vertex with vertex. Coincident vertices merge into one node.
- X2. Vertex with edge. A node on an edge of the other operand, and not
  coincident with an end of that edge, is added to the edge's node list.
- X3. Edge with edge. Using section 7.5, each pair yields isolated points,
  one or two overlap intervals, an overlap of the whole of two closed edges
  (CC4), or nothing. An isolated point becomes a node on both edges
  (merging with an existing node where coincident). It carries no crossing
  or touching label (CC2). The ends of each overlap interval become nodes
  on both edges; a whole overlap has no ends and adds no node. Every node
  of either edge that lies in an overlap is then a node of both, so within
  the overlap the two edges have the same nodes and their spans correspond
  one to one. Each pair of corresponding spans forms a bundle (AR3). When
  two closed edges overlap wholly and their vertices are one node, each has
  one closed span and the two closed spans form one bundle.
- X4. Vertex with face. A node on a face's carrier and strictly inside its
  trimmed region, with no coincidence already recorded by X1 or X2 against
  that face's boundary, is recorded in the face's interior-node list. It is
  also recorded as a touching contact (AR7) when every face of its own
  operand at that vertex lies strictly on one side of the face's carrier
  near the vertex. A node that X9 puts on the boundary of the trimmed
  region, although X1 and X2 recorded it on no edge of that boundary,
  returns `ErrTolerance` (T8): the two tolerant tests disagree.
- X5. Edge with face. Each span, as split so far, is tested against each
  face of the other operand using section 7.5. An isolated point inside the
  trimmed region becomes a node on the span and in the face, recorded as
  crossing or touching. A span that lies on the carrier and inside the
  trimmed region over its whole length is added to the face's interior-span
  list, and is also recorded as a touching contact (AR7) when both faces
  next to it in its own operand lie strictly on one side of the face's
  carrier near the span. A span that lies along the face boundary is already
  in a bundle. Two outcomes return `ErrTolerance` (T8), because X1 to X3
  should have settled them: an isolated point that X9 puts on the boundary
  of the trimmed region where no node of the span exists; and a span with
  one end on the face's carrier and the other off it, or with both ends on
  it and its mid point on the boundary of the trimmed region, which
  neither lies in the face nor is clear of it.
- X6. Face with face. Each pair is resolved by section 7.4 into one of: no
  contact; a same-surface pair (AR6), which creates no trace; or one or more
  carrier curves, each marked crossing or touching. Each carrier curve is cut
  at the nodes already on it, and each interval between consecutive nodes
  that lies inside both trimmed regions becomes a trace, unless a span or
  bundle already occupies that interval, in which case that span or bundle is
  recorded for both faces instead. An interval that lies along the boundary
  of one of the two faces, where that face has no span between the two
  nodes, contradicts the earlier passes and returns `ErrInconsistent`. On a
  closed carrier curve with exactly
  one node, the one interval is the whole curve, from that node round to
  itself; a closed span or a bundle of closed spans occupies it when it
  lies on that curve. A closed carrier curve with no node on it
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
- X10. Making spans. Spans and bundles (AR2, AR3) and the lists of AR5 are
  made after X6, from the node lists as they then stand, since a node can
  merge with another after a pass has recorded something about it. Three
  contradictions can show at that point, and each returns `ErrTolerance`
  (T8):
  - an edge that is not closed has both of its end vertices in one node,
    so the edge is shorter than the tolerance (condition 5 of T8);
  - within an overlap, the spans of the two edges do not correspond one to
    one, which X3 requires;
  - a part of an edge that X5 or X6 recorded as lying in a face no longer
    runs from one node of that edge to another.

  A part of an edge between two nodes that have merged into one, on an
  edge with other nodes, has no length and makes no span.

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
| Cylinder, cylinder | Axes parallel, d + (r₁ − r₂) < 4ε (a candidate pair, SP4), and every node of either face within its radius of the other cylinder's surface | Same-surface pair. |
| Cylinder, cylinder | Coaxial (axes parallel, d ≤ ε) and SP4 has not made the pair a same-surface pair or returned `ErrTolerance` | No contact. |
| Cylinder, cylinder | Parallel, r₁ − r₂ + ε < d < r₁ + r₂ − ε | Two lines, crossing. |
| Cylinder, cylinder | Parallel, d within ε of r₁ + r₂ | One line, touching (external tangency). |
| Cylinder, cylinder | Parallel, d > ε and d within ε of r₁ − r₂ | One line, touching (internal tangency). |
| Cylinder, cylinder | Parallel, d > r₁ + r₂ + ε, or d < r₁ − r₂ − ε | No contact. |
| Cylinder, cylinder | Not parallel, radii differ by ≤ ε, axes meet (d ≤ ε) | Two ellipses, crossing, meeting each other at two nodes (X7). Tier 2 only (OD-1); until Tier 2 is supported (15.5) this row returns `ErrUnsupported` as SP3 states. |
| Cylinder, cylinder | Not parallel, any other, proved separate by SP2 | No contact. |
| Cylinder, cylinder | Not parallel, any other, not proved separate | `ErrUnsupported` naming both faces. |

- SP1. Plane with cylinder uses `geom.IntersectPlaneCylinder`
  (`geom/sections.go:52`). The parallel and coaxial cylinder cases have no
  existing helper and are written in the new package.
- SP2. Two non-parallel cylinder faces are proved separate when the distance
  between their axis segments (each axis restricted to the face's extent
  along it, extended by μ) exceeds r₁ + r₂ + 2μ, or when their bounding
  boxes, each inflated by μ, do not overlap (T4). No other proof is
  attempted in the trial, so some non-parallel faces that do not in fact
  meet return `ErrUnsupported`.
- SP3. "Out" for the trial, always `ErrUnsupported` unless SP2 applies:
  non-parallel cylinders of unequal radius; non-parallel cylinders of equal
  radius whose axes do not meet; and, until Tier 2 is supported (15.5),
  non-parallel cylinders of equal radius whose axes meet.
- SP4. The same-surface test is made node by node, before any other row of
  the table, for every pair of parallel planes and for every *candidate*
  pair of parallel cylinders. A pair of parallel cylinders is a candidate
  when d + (r₁ − r₂) < 4ε, which bounds the largest distance between the two
  surfaces by the tolerance scale of T8. A node of one face passes when its
  distance to the other face's carrier is at most its radius. All nodes of
  both faces pass: same-surface pair. Some pass and some do not: the pair
  is divergent and the call returns `ErrTolerance`, whatever the distances
  of the nodes that fail. T8 names a divergent pair as tolerance-scale for
  this reason, so this rule governs also when those distances are 4ε or
  more: two faces that are parallel only within `Angular` and run from
  coincident at one end to well apart at the other are neither one surface
  nor two separate ones at this tolerance. None passes: the remaining rows
  decide. Parallel cylinders
  that are not a candidate pair are never tested node by node and go
  straight to the remaining rows; nodes of one face that lie on the other
  carrier are then ordinary intersection points, as in C18.
- SP5. Adding a surface pair after the trial must be an addition to this
  table and to the curve table of section 7.5. If it needs a change to
  sections 7.2, 7.3 or 7.6 to 7.11, that is evidence against the design and
  is reported (section 16).

### 7.5 Curve with curve, curve with surface

| Pair | Method | Results |
|---|---|---|
| Line, line | Closest points | Point, overlap interval (CC4), none. |
| Line, plane | `geom.IntersectLinePlane` (`geom/intersection.go:50`) | Point, line in plane, none. |
| Line, cylinder | `geom.IntersectLineCylinder` (`geom/line_surfaces.go:120`) | 0 to 2 points with tangency flag, line on surface, none. |
| Line, circle or ellipse | Line with the conic's plane, then `geom.IntersectLineEllipse` (`geom/planar_intersections.go:36`) when coplanar | 0 to 2 points with tangency flag, none. |
| Circle or ellipse, plane | Plane with the conic's plane (`geom.IntersectPlanes`), then line with conic | 0 to 2 points with tangency flag, conic in plane, none. |
| Circle or ellipse, cylinder; the conic's plane is parallel to the cylinder's axis (T5) | The conic's plane with the cylinder (`geom.IntersectPlaneCylinder`) gives 0 to 2 lines; each line with the conic | 0 to 4 points, none. A point is touching when the plane is tangent to the cylinder or the conic is tangent to the line. |
| Circle or ellipse, cylinder; the conic's plane is not parallel to the axis and the conic projects to a circle along the axis (CC5) | Project along the axis; the cylinder becomes a circle; circle with circle | 0 to 2 points with tangency flag, conic on surface, none. |
| Two conics in different planes | Line of the two planes, then each conic with that line | Points common to both. |
| Two coplanar conics, homothetic (CC3) | Affine map taking both to circles; circle with circle | 0 to 2 points with tangency flag, overlap interval when the same conic, none. |
| Two coplanar conics, not homothetic | — | `ErrUnsupported`. |
| Circle or ellipse, cylinder; any other | — | None when the conic is proved clear of the cylinder face (CC5); otherwise `ErrUnsupported`. |

- CC1. Circle with circle, the homothetic reduction, the projection of a
  conic along a cylinder axis (CC5), point-to-parameter inversion for a conic, and segment-to-segment
  distance have no existing helper and are written in the new package.
- CC2. Crossing and touching are defined for a curve met with a surface
  carrier (X5) and for the carrier curves of a surface pair (X6, section
  7.4). A point where a curve meets a surface is crossing when the curve
  passes from one side of the surface to the other at that point, and
  touching when it does not; a tangency flag from a `geom` helper marks
  touching. A point where two edges meet (X3) has no such label, because a
  curve in space has no sides. A non-manifold contact at such a node is
  found by N1 and N2.
- CC3. Two coplanar conics with major and minor radii a₁ ≥ b₁ and a₂ ≥ b₂
  are homothetic when |a₁·b₂ − a₂·b₁| ≤ ε × max(a₁, a₂) and either both are
  circles (aᵢ − bᵢ ≤ ε for both) or the sine of the angle between their
  major axes is at most `Angular`. Two circles always qualify.
- CC4. Overlap of two edges. Two lines are candidates when they are
  parallel (T5). Two conics are candidates when their planes are parallel
  (T5), they are homothetic (CC3), and their centres, major radii and minor
  radii each differ by at most ε. A line and a conic, and two conics in
  planes that are not parallel, never overlap.

  For a candidate pair, the second edge is projected onto the first
  carrier, and the candidate intervals are the parts of the first edge that
  the projection covers. Intervals are taken in the parameter of the first
  carrier. For a conic that parameter is periodic: an interval may run
  through the parameter origin of the carrier, or through the vertex of a
  closed edge, and is still one interval. The outcomes are:
  - no candidate interval;
  - one candidate interval;
  - two candidate intervals, which happens only for two arcs of one closed
    carrier that overlap at both ends; each is treated on its own;
  - a *whole overlap*, when both edges are closed and so each covers the
    whole carrier. It has no end points.

  The mid point of an interval is the point at the middle of its parameter
  range, taken in the direction of the first edge. An interval is
  degenerate when its mid point is within ε of both of its end points; this
  measures its extent along the carrier and not the separation of its end
  points, so an interval that runs almost all the way round a closed
  carrier is not degenerate. A degenerate interval is no overlap, and the
  pair is resolved for isolated points only.

  Each interval that is not degenerate is tested at its two end points and
  its mid point, and a whole overlap at three points of the first carrier
  one third of the period apart, starting at the first edge's vertex. Each
  point is taken on the first carrier and tested against the second: all
  three within ε gives an overlap; none within ε gives no overlap; any
  other outcome is a divergent pair (T8) and returns `ErrTolerance`,
  whatever the distances at the points that fail.
- CC5. A circle or ellipse with a cylinder. Nothing here refers to a
  cylinder that the conic's own faces lie on. Write the conic as centre c
  and two axis vectors u and v (major radius times major direction, minor
  radius times minor direction), and let u′ and v′ be their projections
  onto a plane perpendicular to the cylinder's axis. The conic projects to
  a circle along the axis when the lengths of u′ and v′ differ by at most ε
  and the absolute value of the cosine of the angle between them is at
  most `Angular`; its projected radius is the mean of the two lengths. The
  conic is proved clear of a cylinder face when the distance from c to the
  face's axis segment (the segment of SP2) exceeds a + r + 2μ, where a is
  the conic's major radius and r the cylinder's radius.

### 7.6 Splitting faces

For each input face, in its chart:

- F1. Collect its boundary spans, its interior spans and its traces (AR5).
- F2. Remove dangling chains. The degree of a node in a face is the number
  of ends of that face's spans and traces at the node; a closed trace or
  closed span contributes two ends to its one node. Repeatedly discard a
  span or trace that has an end at a node of degree one. A closed trace is
  therefore never discarded. A discarded element stays in AR7 if it is a
  touching contact, and otherwise must be an edge of the other operand lying
  in the face; anything else returns `ErrInconsistent`.
- F3. Trace the remaining graph into closed loops. Every interior span and
  trace is traversed once in each direction and every boundary span once, in
  its face's loop direction. Arriving at a node along an element, the walk
  leaves along the first other element met when turning clockwise in the
  chart from the reversed arrival direction, so that each loop keeps its
  region on its left. Directions are tangent directions in the chart at the
  node. Elements whose tangents agree within `Angular` are ordered by signed
  curvature in the chart, the one curving most to the left first. Elements
  that agree in tangent and curvature are the same carrier and must already
  be one bundle; otherwise the call returns `ErrInconsistent`. Because the
  walk turns to the nearest element, a loop passes through a node once for
  each part of its region that meets the node. Where a piece touches itself
  at a node, one loop therefore passes through that node twice or more. It
  is still one loop and the piece is one piece; M6 makes such a piece
  representable if it is kept.
- F4. A loop with positive area in the chart bounds a piece. A loop with
  negative area is a hole and is assigned to the smallest positive loop that
  contains it. A hole that no positive loop contains returns
  `ErrInconsistent`.
- F5. Each piece keeps its face's carrier and orientation and records its
  input face and operand. This record is the origin of the face source list
  of P5.
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
- N2. At every node that is an end of at least one span, bundle or trace
  used by a kept piece, the corners of
  kept pieces at that node, linked through the kept spans, bundles and
  traces that end there, must form exactly one cycle. A corner is one
  passage of a loop through the node, between the element it arrives by
  and the element it leaves by. A piece that touches itself at the node
  (F3) has two or more corners there, and each is a member of the cycle on
  its own. Any other node is not tested and is not in the result: a node
  on no kept piece, and a node that lies in the interior of a kept piece
  with no kept element ending at it, such as the touching vertex that X4
  records inside a face. The cut of C10 returns A for that reason.
- N3. For every touching-contact record (AR7), kept pieces of A and kept
  pieces of B must not both contain the contact locus.

A failure of N1, N2 or N3 returns `ErrNonManifold` with the locus. A touching
contact that passes N3 adds no edge or vertex to the result. Edges and
vertices that the kept pieces already have at that locus remain: in C8 the
cut returns A with its vertex at the contact.

### 7.10 Merging

- M1. An edge whose two uses are by kept pieces on the same surface, with
  consistent orientation, is removed and the pieces become one face. This
  applies to pieces of one input face and to pieces of different faces or
  operands. The merged face takes the carrier of its lowest-indexed
  contributor, A before B, and the union of its contributors' source lists
  (P5).
- M2. A seam is removed by M1 only from a cylinder face that no longer winds
  fully around its axis; that face's chart is then moved so it is continuous.
- M3. A merge is not performed when the merged face would have a repeated
  corner (M6): the merge would create one, or one of the two faces has one
  already and the merge does not remove it. The edge between the two faces
  stays. A merge that removes a repeated corner is performed.
- M4. After M1, a node with exactly two incident edges on the same carrier
  is removed and the edges become one.
- M5. Merging is part of the contract, not an option: a result has no two
  adjacent faces on the same surface except across a required seam (M2),
  across an edge that M3 keeps, or across a divider (M6), and no vertex
  that M4 would remove except those added by B2.
- M6. A face that touches itself. A *corner* of a face is the start of one
  use in one of its loops, taken as a point of the face's chart. A face has
  a *repeated corner* when two of its corners are the same chart point.
  `trimmed.NewSolid` requires distinct chart vertices within a face (B2),
  and no vertex can be added or moved to separate two corners at one
  point, so such a face is divided. A repeated corner comes from F3: a
  piece whose loop passes twice through a node, which M1 has not merged
  into a face without it. The two ends of a seam are different chart
  points and are not a repeated corner. After M4, for each face that has
  a repeated corner, in index order:
  1. Numbering. The corners of a loop are numbered from 0 in loop order,
     starting at the loop's first use. The loops of a face are taken outer
     loop first, then its holes in order.
  2. Candidates. A candidate is a pair of corners x < y of one loop, at
     two different chart points, that meets all of these:
     - (a) Carrier. The straight chord between them in the chart is a
       line, circle or ellipse in space (S3). In a plane chart every chord
       is a line. In a cylinder chart only a chord along a ruling or
       around a circle is, so one chart coordinate of the two corners
       must agree within ε, and the chord must not cross the seam.
     - (b) Inside. No corner of the face other than x and y is on the
       chord (T3). The chord meets no use of any loop of the face except
       at its two ends, tested with the rows of 7.5. Its mid point is
       inside the face (X9).
     - (c) Not along the boundary. At neither end is the chord parallel
       (T5) to one of the two uses of the loop that meet at that corner.
       A chord that continued a boundary edge would give one of the new
       faces three corners on one line.
     - (d) Separating. Some chart point holds a corner numbered strictly
       between x and y and also a corner numbered below x or above y.
  3. Choice. The candidate with the shortest chord in space is taken. Of
     chords equal in length, the one in the earlier loop, then with the
     lower x, then with the lower y.
  4. Division. The chord becomes a new edge of the result, a *divider*,
     on the chord's carrier, from the node of x to the node of y. No node
     is added and none is moved. The loop is replaced by the two loops the
     divider makes of it: one from x round to y and back along the
     divider, the other from y round to x and back along the divider. A
     loop of positive area in the chart is the outer loop of a face, and
     the remaining loops are assigned as in F4. The result is two faces
     in place of one. Both keep the carrier, the orientation and the
     source list of the face that was divided (P5).
  5. Repetition. Steps 2 to 4 are repeated on the faces so made until no
     face has a repeated corner. By (d) each division separates at least
     two corners that were at one point, so a loop that passes k times
     through a point needs at most k − 1 divisions for it, and the
     process ends.
  6. Failure. A face with a repeated corner and no candidate returns
     `ErrUnsupported`, naming the sources of the face and the point. A
     planar face with straight edges always has a chord that meets (a),
     (b) and (d) exactly, so it can lack a candidate only through (c) or
     the tolerances of (b). A face with curved edges, or on a cylinder,
     can lack one outright: a circular hole that touches a straight edge
     has no corner on the circle other than the point of contact. Adding
     a vertex to make a candidate is not part of the trial. If two
     different loops of one face hold corners at one chart point, the call
     returns `ErrInconsistent`: F3 puts every passage of a piece through
     one point into one loop, and M3 keeps merging from changing that.

  A divider is an ordinary edge of the result. It has exactly two uses,
  by the two faces it separates, in opposite directions, and it joins
  those two faces in the cycle at each of its end nodes, so N1 and N2 hold
  after a division as they did before it. M1 and M4 are not applied again
  after M6. In a later call the two faces are two faces of an operand on
  one surface, and the divider is an edge whose carrier I5 takes from its
  samples.
- M7. After M6 every face has exactly one outer loop: one loop of
  positive area in its chart, every other loop negative. A face with none
  or with two returns `ErrValidation`.

### 7.11 Assembly

Sewing here means assembling faces that already share edges by identity in
the arrangement. No geometric welding takes place at this stage.

- B1. Faces are grouped into shells through shared edges. A shell with
  positive enclosed volume is an outer shell; one with negative volume is a
  void and belongs to the outer shell of smallest enclosed volume that
  contains it. Each outer shell with
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
  this document compares. Conditioning cannot separate two corners of a
  face at one chart point; M6 has removed every such pair before assembly.
- B3. P-curves are straight for a line on a plane and for a line or circle
  on a cylinder, and `trimmed.EllipseUV` (`brep/trimmed/pcurve.go:14`) for a
  circle or ellipse on a plane and for an ellipse on a cylinder.
- B4. The output solid is built with `trimmed.NewSolidContext`
  (`brep/trimmed/solid.go:110`) and tolerance τ (T6). An error from it is
  returned as `ErrValidation` wrapping that error, except a context error,
  which is returned unwrapped as section 9 requires.
- B5. A selection that keeps no piece returns the empty solid and no error.
- B6. Every edge of the output solid stores its carrier as a `geom.Line` or
  `geom.Ellipse` value over the edge's interval. A result is a valid
  operand of a later call whatever the Go types of its curve values are,
  since I5 only evaluates them.

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
  - −β ≤ V(A ∩ B) ≤ min(V(A), V(B)) + β and V(A ∪ B) ≥ max(V(A), V(B)) − β

  The lower bound of V(A ∩ B) carries the slack β like the others: the
  volume comes from pieces, and when the common is empty or thin its
  computed value can fall below zero by rounding.

  If `MassProperties` returns an error for A, B or the result, V5 cannot be
  evaluated and `Combine` returns an error: `ErrBudget` wrapping the cause
  when the cause is `parametric.ErrBudget` (the integration budget of P3 was
  exhausted), the context error unwrapped when the context was cancelled,
  and `ErrValidation` wrapping the cause otherwise.
- V6. Membership sampling. The sample points are terms 1 to `Samples` of the
  Halton sequence in bases 2, 3 and 5, whose three coordinates in [0, 1) map
  linearly onto the x, y and z extents of the bounding box of both operands
  inflated by 4ε. Classify each point against
  A and against B with `trimmed.Solid.Classify` at tolerance ε, and against
  the result at tolerance τ. Skipping
  points that any of the three reports as boundary, the result's answer must
  equal the operation applied to the answers for A and B.

  V6 is not an independent check of classification. It does not read the
  piece classes of section 7.7, so it detects errors in splitting,
  selection, merging and assembly. But K2 classifies pieces with
  `trimmed.Solid.Classify` and V6 classifies its sample points with the same
  function, so a wrong answer from `Classify` on an operand can make both
  wrong in the same way, and V6 then passes. The checks that do not depend
  on `Classify` are V5, which takes V(A), V(B) and V(result) from
  `MassProperties`, and the oracle run of A6, which uses neither function.
  V5 is limited too: its first three conditions sum every piece of one
  operand and so hold for any assignment of classes, and its fourth tests
  assembly. Only the bounds in its last line can expose a wrong class. The
  trial therefore relies on the oracle run (A6) and on the analytic and
  legacy values of section 11 and A3 to expose a wrong `Classify` answer.
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
| `ErrInvalid` | Nil context, an operation value other than `Union`, `Common` or `Cut`, invalid options, non-finite input, ε ≥ L / 100, an edge that cannot be evaluated or does not lie on its faces (I5). |
| `ErrUnsupported` | Operand outside section 4.1, including an edge with no carrier or a stored curve that runs back along its carrier (I5); a surface or curve pair marked unsupported in 7.4 or 7.5; a face that touches itself and has no divider (M6). |
| `ErrNonManifold` | N1, N2 or N3 fails. |
| `ErrTolerance` | T4; an edge with two qualifying carriers (I5); a divergent pair (SP4, CC4); X4; X5; X10; F6. T8 says where each is allowed. |
| `ErrInconsistent` | X6, X8, F2, F3, F4, K1, K2, K3, or two loops of one face at one chart point (M6): the arrangement contradicts itself. |
| `ErrValidation` | Any check of section 8, M7, B2, B4, or a mass-property failure other than budget or cancellation (V5). |
| `ErrBudget` | A work limit of section 10 is exceeded, or mass-property integration exhausts its budget (V5). |
| Context error | The context is cancelled; returned unwrapped. It takes precedence over every other error, at every stage. |

`ErrInconsistent` and `ErrValidation` mean the trial's own logic failed on an
input in scope. Every occurrence on an acceptance or matrix case counts
against the pass condition, with the one exception E1 states (RL-1).

## 10. Work limits

| Limit | Default | Counts |
|---|---|---|
| `MaxFaces` | 256 | Faces per operand. |
| `MaxPairs` | 65,536 | Geometric pair tests across passes X1 to X6. A pair rejected by its bounding boxes is not counted. In X5 each span tested against a face counts once. |
| `MaxNodes` | 16,384 | Nodes. |
| `MaxPieces` | 4,096 | Pieces. |

Exceeding one of these limits returns `ErrBudget`. `MaxFaces` is checked
first, before any face or edge of an operand is examined, so an operand
that is both too large and outside the class of section 4.1 returns
`ErrBudget`. `MaxGrowth` (default 4,
T4) is a tolerance bound, not a work limit, and its violation returns
`ErrTolerance`. Checking these limits is the one place where the number of
faces, nodes or pieces of an operand is read (PC2), apart from the test
that an operand has no face at all, which is how the empty solid of S6 is
told (I2). The context is checked at least once
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
| C18 | Parallel cylinders of radius r, axis distance d with 4ε ≤ d ≤ 2r − 4ε, same height h | V = h(2πr² − lens) | V = h·lens, lens = 2r²·acos(d/2r) − (d/2)·√(4r² − d²) | V = h(πr² − lens) |
| C19 | A box; B a cylinder standing on A's top face, base circle strictly inside the face | One solid, V(A) + V(B); the top face has one hole | Empty | A |
| C20 | As C19, but the base circle touches one edge of the top face from inside at a point | `ErrNonManifold` | Empty | A |
| C21 | A box; B a cylinder through A along z with both caps flush with A's faces | A | B | Box with a through bore, V(A) − πr²h, 7 faces |
| C22 | A box; B a cylinder entering A's top face and ending inside A | V(A) + part of B outside | Part of B inside | Blind hole; V(A) − πr²·depth |
| C23 | A box; B a cylinder whose axis is oblique to the faces it passes through | — | Part of B inside A | Through bore with elliptic openings; V(A) − V(common) |
| C24 | C4 with B moved away from A by ε / 2 | As C4 within β | Empty | A |
| C25 | C4 with B moved into A by 100ε | One solid, V = 12 − 400ε within β | V = 400ε within β | V = 8 − 400ε within β |
| C26 | C4 with B moved into A by 2ε | `ErrTolerance` or a validated result (T8) | same | same |
| C27 | Either operand empty | The other operand | Empty | A if B is empty, else empty |
| C28 | A = [0,5]×[0,8]×[2,12]; B the box with corners (3,6,2), (5,8,3), (3,9,5), (1,7,4), (5,2,6), (7,4,7), (5,5,9), (3,3,8). The corner (5,8,3) of B lies on an edge of A, B passes through the faces x = 5 and y = 8 of A, and the corner (3,6,2) of B touches the face z = 2 of A from inside | One solid, V = 410.125, 14 faces: two faces of A touch themselves at (5,8,3) and each is divided in two (M6) | One solid, V = 43.875, 8 faces | `ErrNonManifold` |

In C7 to C14 and C20 the result where an error is listed is a point set whose
boundary is not a manifold. The trial reports it; it does not represent it
(OD-3). The same holds for the cut of C28, whose cavity touches the face
z = 2 at one point.

C28 is the smallest input that the stress test of the planar phase found
for a face that touches itself. In its union, the piece of the face x = 5
of A outside B is a rectangle less the triangle (5,8,3), (5,2,6), (5,5,9),
which meets the rectangle's edge at (5,8,3); the piece of the face y = 8
is a rectangle less the triangle (5,8,3), (2,8,4.5), (3.5,8,6). The
loop of each passes twice through (5,8,3). M6 must give the divider from
(5,2,6) to (5,0,2) on x = 5 and the divider from (2,8,4.5) to (0,8,2) on
y = 8. On x = 5 the chord from (5,5,9) to (5,8,12) is shorter, and
condition (c) excludes it because it continues a side of the triangle.
Each of the four faces so made has one source, the face of A it was
divided from. V(B) = 54 and V(A ∩ B) = 54 − 9 − 1.125.

C1 to C3 and C27 are run with box operands and again with cylinder operands.

Stored forms (S3, I5). The carrier of an edge must not depend on how its
curve is stored, so C15 and C21 are each run with the cylinder operand in
every one of these forms, and the outcome, volume, face count and face
source lists must be the same for all of them:

- SF1. Edges stored as `geom.Line` and `geom.Ellipse` values, the form B6
  gives a result.
- SF2. `parametric.Cylinder` lifted with `trimmed.FromParametric`: the
  edges are sides of surface patches, and each cap is a polar plane chart
  with a collapsed edge at its pole and a radial edge used twice.
- SF3. A disc built by `trimmed.ExtrudeDiskWithHoles`, one of the
  constructors behind the bridge's `mixedExtrude` request: its faces are
  `geom` plane and cylinder values, and its edges are stored as evaluators
  over a surface and not as `geom` curve values.
- SF4. Each of SF1 to SF3 under a rigid placement that is not
  axis-aligned.

Two further tests fix the failures of I5: a solid with one edge replaced by
a curve that lies on both of its faces within ε at its ends and leaves one
of them by 10ε at nine sixteenths of its range returns `ErrInvalid` naming
the edge; and a cylinder whose wall is split in two faces along a helical
edge returns `ErrUnsupported` naming that edge. The deviation of the first
test cannot be at the middle of the range: `trimmed.NewSolid` samples
every edge at the eighths of its range and would refuse to build the
operand, while I5 samples at the sixteenths. A deviation at the exact
middle is tested on the edge alone, below `Combine`. In `internal/plaincadbridge`, the
acceptance rows already use operands made by the `box`, `prism`, `circle`
and `mixedExtrude` requests; the frozen list must contain at least one
checked Boolean response with an operand from each of those four requests,
and the plan's freeze reports the count for each (A1).
The all-planar rows, used by the gate of PL1, are C1 to C10 and C24 to C28
with box operands.

Face sources (P5) are checked on every row that returns a non-empty solid:
each face has at least one source; the list has no duplicates and is in the
order P5 states; each index names a face of its operand; and the carrier of
each source face is the same surface as the carrier of the result face
within τ. Five rows also fix the lists:

- C4, union: the bottom face has one source, a face of A; the top face has
  one source, a face of B; each of the four side faces has two sources, one
  face of A and one face of B (merged by M1).
- C6, common: the three faces on the planes shared with A each have two
  sources, the face of A and the face of B of the same-surface pair; the
  other three each have one source, a face of B.
- C15, union: the lateral face has two sources, the lateral faces of A and
  of B; one cap has a face of A as its only source and the other a face of
  B.
- C21, cut: six faces each have one source, a face of A; the bore wall has
  one source, the lateral face of B.
- C28, union: the two faces on the plane x = 5 each have one source, the
  face x = 5 of A, and the two on the plane y = 8 each have the face y = 8
  of A (M6).

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
| `brep/parametric` | `Edge`, `Interval` (`solid.go:21,36`); `Location` (`ball_query.go:10`); `IntegrationOptions`, `Properties` (`properties.go:13`); `ErrBudget` (`solid.go:17`); `SurfaceMapping` (`transform.go:86`); `PolarPlaneFrame`, `Cylinder` (`primitives.go:82,134`) | `Face` is a rectangular patch with no trims. `CurveMapping` is not needed: edge curves are only evaluated (S3, I5). The `cylinder_pair_*`, `oblique_*` and `sphere_*` constructors are the enumeration this trial is an alternative to. |
| `brep/trimmed` | `Solid`, `Face`, `Use`, `PCurve`, `NewSolidContext` (`solid.go:19-110`); `EllipseUV` (`pcurve.go:14`); `FromPlanar`, `FromParametric` (`transform.go:17,137`); `Solid.Classify`, `Solid.Ray` (`location.go:27,63`); `Solid.MassProperties` (`properties.go:33`); `Solid.Bounds` (`bounds.go:15`); `Solid.Tolerance`, `Solid.Topology` (`topology.go:93`); `Solid.Tessellate` (`solid.go:489`) in tests | `Sew` (`sew.go:42`) joins only exactly equal endpoints with a proved common evaluator and never welds within tolerance; the arrangement already shares edges. `ValidateTrimLoops` and the other functions of `validation.go` wrap the exact certifier and leave ellipse p-curves unresolved. `ValidateEdgeAgreement` (`edge_validation.go:35`) handles rational Bézier edges only. Every named-arrangement constructor. |
| `brep/trimvalidate` | Nothing | Exact, chart-only, no tolerance parameter; it does not check model-space closure, edge-on-surface or vertex-on-edge residuals, which is what section 8 needs. |
| `intersect` | Nothing | Spline and marching oriented. `Trace` (`trace.go:113`) returns `ErrSingular` on tangency and does not detect coincidence. It is the likely home of a later surface pair that has no closed form. |
| `csg`, `csg/poly` | `poly.Combine` (`csg/poly/history.go:79`) in tests only, as an independent volume check on all-planar cases | `csg.Union` and `csg.Intersect` (`boolean.go:37`, `intersect.go:37`) are convex-only. `poly.Combine` is exact over a tessellation and returns triangles, so it cannot carry cylinders. |

- E1. The trial needs from `brep/trimmed` that `NewSolidContext` accept every
  result that meets B2 and B3, and that `Classify` answer inside, outside or
  boundary for every operand in the class at tolerance ε. If either fails on
  a valid input, the trial does not edit `brep/trimmed`: it records the case
  and stops under section 16, with one exception.

  Exception (RL-1, section 19). `trimmed.NewSolidContext` rejects a valid
  planar result with "mesh: triangle N is degenerate or overflows" when a
  face in a plane that is not axis-aligned has three consecutive corners
  on one straight line: the three points are exactly collinear in space
  and their rounded chart coordinates are not. The stress test of the
  planar phase met this in 12 of 4,500 operations. The smallest input is
  A = [3,11]×[4,8]×[4,12] and B the box with corners (2,0,−1), (6,4,1),
  (2,6,5), (−2,2,3), (3,−2,1), (7,2,3), (3,4,7), (−1,0,5), union, where a
  face of B has the corners (2,6,5), (3,5.5,4), (6,4,1) in a row. This is
  a robustness defect of `brep/trimmed`. It is investigated in separate
  work on its own branch and does not end the trial. `Combine` returns
  `ErrValidation` for such input (B4) and does not work around it. A leaf
  or row that fails for this reason alone is reported as such, with its
  input, and does not count toward ST3, ST5 or the pass condition until
  the owner rules. Every other failure of E1 is ST5 as before.

## 14. Code location and architecture

- L1. All new kernel code is in `brep/boolean`. It imports `geom`, `brep`,
  `brep/parametric` and `brep/trimmed` and nothing else from this module. The
  closed forms of SP1 and CC1 are unexported there.
- L2. The outside caller of `Combine` (G2) is one new file in
  `internal/plaincadbridge`. It converts a stored shape to a `trimmed.Solid`
  where the shape already holds one or lifts to one (planar solids through
  `trimmed.FromPlanar`, cylinders and patch solids through
  `trimmed.FromParametric`, bored and analytic bodies through the solid they
  hold), calls `Combine`, and publishes the result as an analytic body
  holding the result solid (RL-2). A result that is not empty is stored
  by calling the bridge's `publish` directly from the adapter file, and
  not through `putAnalyticBody`, for two reasons: that function accepts
  one shell only, so it cannot take a result with a void or with several
  solids; and it takes its bounds from `trimmed.Solid.Bounds`, which
  widens the box by the solid's tolerance, while A3 compares bounds at
  1e-7. The bounds the adapter publishes are those of
  `trimmed.Solid.Bounds` with that pad removed. An empty result is
  published through `put`, as the legacy paths publish one. A stored shape with no such solid returns the bridge's
  `UNSUPPORTED` failure. The adapter reads the shell grouping of the result
  for `solidCount`. It does not read the face source list during the trial;
  that list is a field of the result type, in the contract at the owner's
  request, and is read by the tests of section 11.
- L3. The general path is selected per `Bridge` value by an unexported field
  set through a constructor used only by tests. The default bridge behavior
  does not change during the trial, and there is no environment variable or
  package-level switch. One change outside the adapter file is allowed
  (RL-2): the `transform` request in `bridge.go` may republish an analytic
  body of more than one shell, which today fails with "expected one
  connected analytic shell". The change must be minimal and must leave
  the transform of a body of one shell as it is; the existing tests of
  the default bridge and the trace replay on it verify that.
- L4. Bridge tolerances for the trial: `Linear` 1e-7, `Relative` 0,
  `Angular` 1e-9, `Volume` 1e-7, integration relative tolerance 1e-9.
- L5. No file in `brep/trimmed` or `geom` is changed. No dependency is
  added.
- AR8. Architecture, a constraint on the code and not functional behavior:
  the arrangement of section 7.2 is the only data passed from the
  intersection stage to the building stage; every later stage reads its
  lists and none recomputes an intersection of A with B; there is one
  arrangement per call; the passes run in the order of section 7.3; and
  every traversal is in index order with no randomness, which is how the
  guarantee of P7 is met. Classification and validation make the point
  and residual tests that sections 7.7 and 8 state. Beyond those, the one
  place after the passes where curves are intersected is the test of a
  candidate chord against the loops of its own face (M6), which involves
  one face of the result and no pair of operands. None of this is
  observable from outside `Combine`. It is what makes the pipeline one
  path, and it is checked by review of the diff under PC2.
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
| To face | `to_face_test.go`, `to_face_analytic_test.go`, `to_face_oblique_test.go` | 7 | Extrude a polygon or circle up to a planar face (CO3) | `brep.PrismToFace`, `trimmed.CylinderToFace` |

Tier 2, in the trial by OD-1, attempted last and not required for a pass
(15.5):

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
  `bored_cap_edge_chamfer_test.go`, `pocket_chamfer_fallback_test.go`. This
  exclusion has a plain consequence (OD-8): PlainCAD's fillet, chamfer,
  hole, pocket and boss chain, in which a hole, pocket or boss is applied to
  stock that already has a fillet or chamfer, stays on the legacy path even
  after a pass.
- Edge-treated stock before the first Boolean, found when the rows were
  frozen (A1): the Rounded profiles family, `rounded_stack_test.go`,
  `rotary_platter_test.go` and `annular_pedestal_test.go` (rounded-rectangle
  and annulus extrusions with axial bores; legacy paths `cutRoundedStack`
  and `cutThroughProfiles`). All three of its test functions fillet or
  chamfer the stock before their first Boolean request, so every Boolean in
  them has an operand that OD-8 excludes. The family has no Tier 1 row and
  is not required for a pass. It was listed as a Tier 1 family in 15.1
  until the freeze.
- Faces stored as spline patches (S2): a body made by the arc-loop branch
  of `mixedExtrude` (`trimmed.ExtrudeExactCircularLoop`). The freeze (A1)
  counted the Boolean responses this leaves unchecked: none. The exclusion
  stays (RL-3).
- Faces stored as other evaluators (S2), found by the freeze: a body that
  the bridge's STEP import stores with faces of the unexported
  `brep/parametric` types `revolvedSide`, `ruledArcs` or
  `reparameterizedCylinder`. These are planes and cylinders in shape and
  not in the surface values S2 accepts. Two Boolean responses cut such a
  body, one each in `TestFullTubeSTEPAndCoaxialCut` and
  `TestQuarterAnnularSTEPAndFurtherCut`, and are unchecked (RL-4). The
  other Boolean responses of those two rows are checked.
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
  rows of a data file; there is no per-case C++. It is run once, as the last step
  of the trial, after Tier 2 has been kept or reverted (15.5), on every section 11 row that returns a solid, except the
  tolerance rows C24 to C26, and on one row per Tier 1 family, with the comparison bound of
  `internal/occtcheck/oracle_test.go:68` (3e-7 relative for area and volume,
  3e-7 absolute for centroid). Face counts are compared for the section 11
  rows that state one. C24 to C26 depend on ε, which the oracle does not
  share, and are checked against their analytic values only. The owner
  approved this one run (OD-5).
- A7. Every acceptance row and matrix row runs in under two seconds under
  `go test` on the 14-core development machine that `tools/ci/README.md`
  refers to, with the default settings of `go test` (no race detector, no
  change to `-p` or `-parallel`). The time of a row is the elapsed time
  that `go test -json` reports for its subtest, which includes building its
  operands. A row that needs longer is a failure
  of the row, not a candidate for the slow tier. A7 times a row. The rule
  of `AGENTS.md` (G2) times a test function, and a function that holds
  many rows meets it by running them as parallel subtests (RL-7); that
  uses the default `-parallel` and changes no setting.
- A8. Achieved accuracy (OD-7). For every acceptance row, the test of A2
  reports the error the general path achieves in volume and in surface area:
  |value − expected| / max(1, |expected|), where expected is the analytic
  value in the source test where it has one and the legacy value otherwise.
  A row can produce several shapes and volumes: its two numbers are the
  largest error in volume and the largest error in area over the row's
  checked responses that produced a value, and the row also records how
  many of its checked responses produced none.
  The test writes these to its log for each row. At the end of the
  trial, on a pass or a stop, the numbers from one run are written into two
  columns of `specs/general-boolean/ACCEPTANCE.md`, a row that did not
  produce a result being marked as such, and the report to the owner gives
  the largest volume error, the largest area error, and the number of rows
  above 1e-8 in either. The acceptance bound stays 1e-7 (A3); A8 records and
  does not gate.

### 15.4 Order of work

These are requirements on the plan.

- PL1. Planes first. After the freeze of A1, the first implementation phase
  of the plan covers operands whose faces are all planar, through the whole
  pipeline: intake, the arrangement, the six passes of 7.3, splitting,
  classification, selection, the manifold test, merging, assembly and all
  of section 8, for union, common and cut.
- PL2. That phase has its own gate. Every all-planar row of section 11 (C1
  to C10 and C24 to C28 with box operands) gives its stated outcome,
  including the face source checks. C28 was added after the gate had
  passed on the other rows (RL-8); the plan says when it is due, and no
  cylinder work depends on it. For each of those rows whose stated
  outcome is a solid or the empty solid, except C26, the volume of the
  result also equals the volume of the result of `csg/poly.Combine`
  (`csg/poly/history.go:79`) on the same operands and operation within the
  bound β of V5. No comparison with `poly.Combine` is made for a row whose
  stated outcome is `ErrNonManifold`, or for C26.
- PL3. No cylinder code is written before the gate of PL2 passes: no
  cylinder chart, no circle or ellipse edge, and no cylinder row of 7.4 or
  7.5. The planar phase must still be built on the carrier tables, charts
  and arrangement of section 7, so that cylinders arrive as a chart and as
  table rows and not as a second path (PC2, SP5).
- PL4. If the gate of PL2 fails at the end of the planar phase, no later
  phase starts. The one corrective phase of ST3 may be spent there; if it
  is, none remains for later phases.

### 15.5 Pass condition

The trial passes when all of these hold on one commit:

- PC1. Every Tier 1 row passes A3 and A4, section 11 passes (A5), the
  oracle run passes (A6), and the record of A8 is complete for every Tier 1
  row. `ErrInconsistent` or `ErrValidation` on any of them is a failure,
  except as E1 states for one known defect of `brep/trimmed` (RL-1).
- PC2. One path. `brep/boolean` exports one operation entry point; the bridge
  file of L2 reads a stored shape only to obtain its `trimmed.Solid`; and no
  function in either tests the number of faces, edges or vertices of an
  operand, the primitive it was built from, or its placement, except through
  the recognition of surface values and rigid placements at intake (S2,
  S4, section 7.1), sections 7.3 to 7.5, the work limits of section 10,
  and the one test that an operand has no face, which tells the empty
  solid (S6, I2). The architectural
  constraints of AR8 hold. This is checked by review of the complete diff.
- PC3. `go build ./...`, `go vet ./...` and `go test ./...` pass, and
  `golangci-lint run --new-from-rev=<base> ./...` reports nothing, where
  `<base>` is the commit of `main` the trial branch was created from. The
  bridge had lint findings before the trial; only findings the trial
  introduces count. `go test ./...` without `PLAINSOLID_SLOW` runs the
  default test tier of `tools/ci/README.md`, which `AGENTS.md` requires
  before a merge; it is not the complete suite that section 4.4 excludes.
  The research configuration still compiles and the tag still marks the
  right files (G4): `go build -tags research ./...`,
  `go vet -tags research ./...` and
  `go run ./tools/refgraph -tags research -pkg brep/trimmed -exclude examples -check research`
  pass.
- PC4. Non-test Go in `brep/boolean` is at most 15,000 lines, and the bridge
  file of L2 at most 400.

PC1 to PC4 decide the pass. Tier 2 does not (OD-1): it is attempted only
after PC1 to PC4 hold in every respect except the oracle run, which comes
last (A6) so that the one approved run tests the final code. Tier 2 is
made by changes to the tables
of 7.4 and 7.5 and their tests only (SP5). Tier 2 is *supported* when its
rows then pass A3 and A4 while every Tier 1 row, section 11 and PC2 to PC4
still hold; otherwise it is reported as unsupported, the change is not
kept, and the trial goes on to the oracle run without it.

On a pass, the report to the owner lists the bridge paths and kernel
constructors that no acceptance row needs any longer. From the survey these
are expected to be, for Tier 1: `newBore` with `trimmed.BoreCylinders` and
`stopBore`, `newUnionBore`, `mergeOpposedBlindBores`,
`trimmed.BoreFiniteCylinders`, `newObliqueThroughBore`,
`newDisjointObliqueBore`, `cornerCylinderBoolean` and its four
`trimmed.Box*Cylinder*` constructors, `cutSideOpeningFamilies` and the five
`trimmed.Cut*SideOpenDualDisks`, `cutSideNotchBores`, `cutCircularStockBores`,
`coaxialThrough`, `fuseCoaxial`, `cutPocketCylinder`, `rectangularPocket`,
`rectangularIslandPocket`, `retainedPlanarBoolean`,
`containedPlanarCylinderVolume` and `parallelCylinderOverlap`; for Tier 2:
`newCrossedBore`, `newBlindCrossedBore`, `newTripleCrossedBore`,
`newCrossedCompanionBore` and `newObliqueCrossedBore`. `cutRoundedStack`,
`cutRoundedBores` and `cutThroughProfiles` are expected to stay, because
the Rounded profiles family is excluded (15.2). Deleting any of them is not
part of the trial.

The same report states what a pass does not cover: operations on
edge-treated stock stay on the legacy path (15.2, OD-8), and lifting
recipe-held stock to a `trimmed.Solid` is the first follow-up after a pass.

## 16. Stop condition

The trial ends as a failure, and work stops with a report to the owner, at
the first of:

- ST1. Wrong result. `Combine` returns a result that passes section 8 and
  differs from the analytic or oracle value beyond A3 or A6, and the
  session that finds it cannot state a general check which, added to
  section 8, rejects that result. A check it can state is reported to the
  owner with the case, and the owner decides whether it is added in the
  corrective phase; until then the row counts as failing.
- ST2. Special-casing. The only known way to make a Tier 1 row pass is code
  that violates PC2.
- ST3. Not converging. After the last planned phase and one corrective
  phase, any Tier 1 row or section 11 row still fails; or the planar gate
  of PL2 still fails after the corrective phase has been spent on it (PL4).
- ST4. Size. PC4 is exceeded before PC1 holds.
- ST5. Foundation. E1 fails on a valid input, or a Tier 1 row cannot pass
  without changing `brep/trimmed`, `geom` or adding a dependency. The
  failure that E1 names as its exception (RL-1) is not counted here until
  the owner rules.
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

## 18. Owner decisions

The owner decided these on 2026-11-18. None is open. Each entry states the
decision and where this document applies it.

- OD-1. Tier 2 is in the trial. Equal-radius cylinders with meeting axes
  give two ellipses, which is inside the edge class, and cover about 17
  bridge tests and the crossed-bore constructors. The two ellipses meet at
  two points where the surfaces are tangent. Decided: Tier 2 is attempted
  last, as a test of SP5, and is not required for a pass (7.4, 15.1, 15.5).
- OD-2. Unequal and skew crossed bores are out. Their intersection curves
  are quartic (SP3). Their constructors and five test files stay whatever
  the trial's outcome (15.2).
- OD-3. A touching result is an error. A result whose boundary is not a
  manifold returns `ErrNonManifold` (7.9, section 11). This includes the
  union of two solids that touch only along an edge or at a vertex; the
  alternative of returning the two solids unjoined was not taken.
- OD-4. The bounds of the trial are 15,000 non-test lines (PC4), one
  corrective phase (ST3) and two seconds per case (A7). The line cap was
  first set at 12,000; the owner raised it once the plan estimated 11,300
  lines for the required work, so that the trial does not stop on size
  alone. The plan's per-part line estimates remain as warnings.
- OD-5. One oracle run with the pinned OCCT build, at the end of the trial,
  is approved (A6). `AGENTS.md` reserves such runs for an owner request, and
  this decision is that request.
- OD-6. Implementation sessions work from this document and the literature
  cited in G3 and do not open the OCCT source. The headers of the pinned
  source carry a notice that using it as a reference to produce derivative
  code requires compliance with its license, and this document was written
  from its structure only (G3). The oracle helper of L6 is written against
  the public interface of the built library, as the existing helpers in
  `tools/occtref` are.
- OD-7. 1e-7 is the acceptance bound (A3), with a condition. The general
  path integrates volume and area numerically, where the existing bridge
  tests compare closed-form values at 1e-8. The trial must record, for each
  acceptance row, the volume and area error it achieves against the analytic
  or legacy value, so that the owner can see whether 1e-8 would hold (A8,
  PC1).
- OD-8. Edge-treated stock is excluded (4.4, 15.2). The bridge holds it as
  a recipe with no `trimmed.Solid`. This leaves PlainCAD's fillet, chamfer,
  hole, pocket and boss chain on the legacy path even after a pass. Lifting
  recipe-held stock to a `trimmed.Solid` is the first follow-up after a
  pass (15.5); it is not part of the trial.

## 19. Rulings made under delegation

The reviewing session made these on 2026-11-18, while the owner was away,
under the owner's delegation to it. Each stands until the owner confirms or
reverses it. Each entry states the ruling and where it is applied.

- RL-1. A known defect of `brep/trimmed` does not end the trial.
  `trimmed.NewSolidContext` rejects some valid planar results whose face,
  in a plane that is not axis-aligned, has three collinear corners. The
  defect is investigated in separate work on its own branch. `Combine`
  returns `ErrValidation` for such input, and a leaf or row that fails for
  this reason alone does not count toward ST3, ST5 or the pass condition
  until the owner rules (E1, ST5, PC1, section 9).
- RL-2. Publication (the plan's decision D4). The adapter stores a result
  with a void or with several solids by calling `publish` directly, and an
  empty result through `put`. It removes the tolerance pad of
  `trimmed.Solid.Bounds` before publishing bounds. One minimal change to
  the `transform` path of `bridge.go` is allowed, so that it republishes
  an analytic body of more than one shell, with the behavior for one
  shell unchanged (L2, L3).
- RL-3. Faces stored as spline patches stay excluded. The freeze counted no
  Boolean response lost to them (S2, 15.2).
- RL-4. Two Boolean responses on STEP-imported bodies stay unchecked. The
  bodies have faces stored as evaluators that S2 does not accept (15.2).
- RL-5. Size of the frozen traces. They are 7.9 MB, 91 percent of it STEP
  text inside `importStep` requests. This is accepted on the trial branch.
  It is a condition of any later merge of the trial to `main` that the
  distinct STEP sources are stored compressed or are otherwise reduced.
- RL-6. One branch. The planar phase ran beside the freeze on
  `codex/general-boolean-kernel`. That branch, with `codex/general-boolean`
  merged into it, is the one branch of the trial (the plan's rule R1).
- RL-7. Time of the replay test. The test that replays the frozen rows on
  the default bridge runs its rows as parallel subtests so that the test
  function meets the two-second rule of `AGENTS.md`. A7 times rows; the
  repository rule times the function (A7).
- RL-8. A face that touches itself. The stress test of the planar phase
  found that splitting can make a piece whose loop passes twice through
  one node, in 148 of 4,500 operations on a rotated box, and that
  `trimmed.NewSolid` rejects such a face. Sections 7.6 to 7.11 had no rule
  for it. M6 was added, with the changes to F3, N2, M3 and M5 that go with
  it, the check M7, and row C28. This is a change to sections 7.6 to 7.11
  made by a ruling, so ST6 does not apply to it. It is recorded here as a
  change to the design that a test outside the acceptance rows forced.
