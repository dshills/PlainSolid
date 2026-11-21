# General Boolean trial: planes and cylinders

Status: owner decisions recorded on 2026-11-18 (section 18). Rulings made
on 2026-11-18 and 2026-11-19 by the reviewing session, under the owner's
delegation, are in section 19. On 2026-11-19 the owner declared the trial
a pass with two recorded exceptions (OD-9), which confirms six of the
rulings; the others wait for the owner to confirm or reverse them. Section
20 lists what is open, and the plan states where the trial stands. Tier 2
has been supported since 2026-11-19 (RL-27), by
[the crossing specification](../general-boolean-crossing/SPEC.md), which
amends sections 7.4, 7.5 and 12 here. Since 2026-11-20 a placement that
reflects is accepted (S4, RL-28), the default bridge asks the general
path first for plain bores (OD-11), a face of B within ε of a plane
of A is seated on it (S7, RL-30), and cylinders of unequal radius or
with skew axes are cut
([the quartic specification](../general-boolean-quartic/SPEC.md),
RL-31). This document is a specification only and contains no
implementation.

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
- G4. The trial's own commits do not edit `brep/trimmed`. A limit of that
  package that a valid result exposes is fixed there on a separate
  foundation branch, which is merged into the trial branch (E1, RL-9). The
  trial uses only identifiers
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
  under a placement that keeps lengths (S4), reported by
  `parametric.SurfaceMapping`. Unlike
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
- S4. A placement keeps lengths when its linear part M satisfies
  ‖MᵀM − I‖∞ × L ≤ ε, with L and ε of T1. Within this bound the placed
  surface differs from a true plane or circular cylinder by no more than
  about ε over the operands, and the carrier is built from M with its
  columns made orthonormal. A placement outside the bound, such as a
  non-uniform scaling, makes the face `ErrUnsupported` (S5).

  A placement that reflects (det M < 0) is accepted on the same bound
  (RL-28). `trimmed.Solid.Transformed` leaves a mirrored solid in that
  form: every surface under the reflecting
  placement, every face with its orientation flag turned, and the loops
  as they were in the chart of the stored surface. The carrier of such a
  face is the plane or cylinder in the frame on the images of the origin,
  the x axis and the z axis of the stored frame. The normal of that
  carrier is against the normal of the placed surface, and its chart runs
  the other way round. Intake therefore reads the orientation of the face
  turned once more and takes each loop backwards, each use with its
  direction turned, so that the loop is a chain in the chart of the
  carrier (I3) with the face on its left, as every other loop is.
  Nothing after intake tells such a face from any other.
- S5. An operand outside S1 to S4 is rejected with `ErrUnsupported` naming
  the first offending face or edge index, except where I5 names another
  error for an edge. Embedding (S1, no self-intersection)
  is a precondition that intake does not test; see V7 for what is guaranteed
  when it is violated.
- S6. A zero-value `trimmed.Solid` is the empty solid and is a valid operand.
- S7. A planar face of B that lies within ε of the plane of a face of A
  is seated on that plane (RL-30). The two planes are parallel as the
  table of section 7.4 judges it, and the border of the face of B is
  within ε of the plane of A, less a part in 2³⁰ for rounding, at every
  sample by which I5 holds an edge to the carrier of its face. The
  distance of a planar face from a plane is greatest on its border. Intake
  then gives the face of B the carrier of the face of A, with its
  orientation and its loops turned as for a reflecting placement (S4)
  where the two planes face against each other. Its edges take their
  carriers from that plane by I5, its vertices are moved onto those
  carriers, and an edge that ends at a moved vertex ends there. B is so
  changed by at most ε, on the faces that C24 to C27 call coincident,
  and the result has those faces, their edges and their vertices in the
  plane of A to rounding and not to the gap. A is taken as it is.
  Cylinders that coincide are not seated.

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
    `parametric.IntegrationOptions`. They are used only where a solid is
    outside what the closed form of V5 covers (RL-17).
  - Known mass properties, optional, one set for A and one for B: the
    volume and the surface area of that operand, when the caller already
    has them (V5, RL-10). None is given by default. They are fields of
    the options type and need no further exported identifier (P2).
- P4. `Linear` and `Relative` must not both be zero, `Angular` must be > 0 and at most 0.5, so that no pair of directions is
  both parallel and perpendicular under T5, and
  `Volume` must be > 0. There is no hidden default tolerance. `Samples` and
  every work limit must be ≥ 0, and a nonzero `MaxGrowth` must be ≥ 1. A
  known volume or area must be ≥ 0. Every
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
  the volume and the surface area of A, of B and of the result (RL-10; for
  an operand with known mass properties, the values the caller gave), the
  centroid of the result (RL-17; an empty result has none, and the report
  gives the origin), the largest edge and vertex residuals
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
  has one tolerance. τ is the largest node radius used by the result, and
  nothing else: it does not include the rounding allowance σ of T9. The
  value the solid records as its `trimmed.Solid` tolerance is τ + σ, since
  that is the tolerance it is built with (B4). The report gives τ itself,
  as the largest node radius (P5). An empty result is the zero `trimmed.Solid`,
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
  3. A value of (a), (b) or (c) lies strictly between ε and 4ε, or within
     the rounding allowance σ of ε or of 4ε (T9).
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

- T9. Rounding allowance (RL-13). A result built from coincidences can
  have a residual that equals its tolerance exactly, for example an edge
  of one operand that lies exactly ε from a face of the other, and a
  comparison with the tolerance must not then fail by rounding. One
  allowance is defined for this, σ = 2⁻⁴⁹ × L, with L of T1: eight units
  in the last place of the model size, 1.8e-13 for a model of size 100.
  It follows the size of the model and not the tolerance, because the
  rounding error of a distance grows with the coordinates it is computed
  from. It is below ε / 1000 whenever ε is above 2e-12 × L. It is used in
  two places and no other:
  - Section 8 compares residuals with τ + σ, and the output solid is built
    with the tolerance τ + σ and records it (T6, B4), so that the checks
    `trimmed.NewSolidContext` makes itself carry the same allowance.
  - Condition 3 of T8 counts a value within σ of ε or of 4ε as lying in
    the band.

  The coincidence tests of T3, SP4 and CC4 are made on distances as they
  are computed, without σ. A distance that rounding can put on either
  side of ε is at the edge of the band, and T8 then allows either outcome,
  so those tests and T8 do not disagree. Because a result records τ + σ,
  each operation of a chain can raise the working tolerance of the next
  by σ.

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
  the faces of A and of B that meet there. X4 makes them for a vertex in
  a face, X5 for a span that lies in a face and for an isolated point of
  a span, and X6 for a touching trace. X6 makes none for an edge.

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
  trimmed region becomes a node on the span and in the face. A span that
  lies on the carrier and inside the trimmed region over its whole length
  is added to the face's interior-span list. A span that lies along the
  face boundary is already in a bundle, and X5 does not test it against
  that face again. A span that crosses the carrier, with or without an
  end on it, is the case of the isolated point.

  Touching (RL-23). One rule decides whether a contact found here is a
  touching contact (AR7), for a span that lies in the face and for an
  isolated point where the span is tangent to the face's carrier alike:
  the contact is touching only when the faces of the span's edge stay on
  one side of the carrier. Each of the two uses of the edge is judged on
  its own, from points of its face near the contact. The points are
  taken at short steps in the chart of that face, on a fan of directions
  from close to one direction of the edge round to close to the other. A
  face judged this way is on one side of the carrier, on both sides,
  which means it passes through the surface, or on the surface. A face
  on the surface is flush; that is a matter for the same-surface test
  and makes no touching contact. The contact is touching when both uses
  are on one side and it is the same side for both. Where the two uses
  are by one face, as at a seam (I4), both are judged: a seam that lies
  in a face of the other operand while its wall passes through that face
  is not a touching contact, and one whose wall is tangent to the face
  is. An isolated point where the span passes through the carrier is a
  crossing and makes no record. A direction that leaves the edge is not
  used to judge a side, because at a tangent contact such a direction
  lies in the tangent plane and tells nothing.

  Two outcomes return `ErrTolerance` (T8), because
  X1 to X3 should have settled them. The first is an isolated point that
  X9 puts on the boundary of the trimmed region where the span has no
  node. The second concerns only a span that is in no bundle with an edge
  of the face and whose carrier section 7.5 reports as lying on the
  face's carrier: its two ends and its mid point should then all be
  within tolerance of that carrier, and its mid point inside or outside
  the trimmed region. If some of the three points are on the carrier and
  some are not, or the mid point is on the boundary of the trimmed
  region, the span neither lies in the face nor is clear of it.
- X6. Face with face. Each pair is resolved by section 7.4 into one of: no
  contact; a same-surface pair (AR6), which creates no trace; or one or more
  carrier curves, each marked crossing or touching. Each carrier curve is cut
  at the nodes already on it, and each interval between consecutive nodes
  that lies inside both trimmed regions becomes a trace, unless a span or
  bundle already occupies that interval, in which case that span or bundle is
  recorded for both faces instead. A carrier curve marked touching makes a
  touching-contact record (AR7) for a trace only. Where a span or bundle
  occupies the interval, the two surfaces are tangent along an edge, and
  X6 makes no record for it: whether that edge is a touching contact is
  X5's question, which looks at the faces of the edge (RL-23). An interval that lies along the boundary
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
  nodes before X6 cuts the curves. No Tier 1 pair has two curves that
  meet, so the code lacked X7 until the Tier 2 attempt needed it; it is
  on the trial branch with a test of its own (RL-22). At such a node the
  two surfaces are tangent to each other, and how X5 and N3 are to treat
  an edge through it is open (section 20).
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
| Cylinder, cylinder | Not parallel, the difference of the radii and the distance of the axes together ≤ ε, sine of the angle between the axes ≥ 1/64 | Two ellipses, crossing, meeting each other at two nodes (X7): the sections of the first cylinder by the two planes through the point where the axes meet that bisect their angle. [The crossing specification](../general-boolean-crossing/SPEC.md), E1 to E4. |
| Cylinder, cylinder | Not parallel, any other, proved separate by SP2 | No contact. |
| Cylinder, cylinder | Not parallel, any other, not proved separate | `ErrUnsupported` naming both faces. |

- SP1. Plane with cylinder uses `geom.IntersectPlaneCylinder`
  (`geom/sections.go:52`). The parallel and coaxial cylinder cases have no
  existing helper and are written in the new package.
- SP2. Two non-parallel cylinder faces are proved separate when the distance
  between their axis segments (each axis restricted to the face's extent
  along it, extended by μ) exceeds r₁ + r₂ + 2μ, or when their bounding
  boxes, each inflated by μ, do not overlap (T4). A sharper proof may be
  added to these two (RL-14), on two conditions: it is a proof, so that
  it never calls two faces separate that come within 2μ of each other,
  and it is general, with nothing in it that depends on which faces or
  operands it is given (PC2). One such proof is by subdivision. The
  chart domain of each face is divided into cells; the box of a cell is
  computed from the true extent of the surface over the cell and not
  from samples, so it contains that part of the face; a pair of cells,
  one from each face, is separate when their boxes, each inflated by μ,
  do not overlap; a pair that is not separate has its cells halved, to a
  fixed depth and within a fixed number of box comparisons, the same for
  every pair of faces: twelve halvings in each chart direction and
  20,000 comparisons. The faces are proved separate when every pair of
  cells has been separated. When the depth or the count is used up,
  nothing is proved and the pair of faces returns `ErrUnsupported`. A
  cell may cover more of the surface than the face does, which leaves
  some separate faces unproved and proves none wrongly. Another such
  proof, tried before it, is that one face is clear of the whole of the
  other's cylinder: every point of it farther from the other axis than
  that radius and 2μ, or every point nearer than that radius less 2μ
  ([the crossing specification](../general-boolean-crossing/SPEC.md),
  G1 to G6). A proof does not decide that two faces meet:
  faces that are not proved separate, and that are outside the rows of
  the table, return `ErrUnsupported`.
- SP3. "Out", always `ErrUnsupported` unless SP2 applies: non-parallel
  cylinders whose axes make an angle of sine below 1/64, and non-parallel
  cylinders that are neither an equal pair nor a regular quartic pair,
  which are those that touch or come within a sine of 1/64 of touching
  ([the quartic specification](../general-boolean-quartic/SPEC.md),
  section 2). Until 2026-11-20 every pair of unequal radius, and every
  pair of equal radius whose axes do not meet, was out (OD-2). Equal
  radius and meeting axes are judged together, as the row of the table
  states.
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
  is reported (section 16). A sharper separation proof under SP2 is an
  addition of the same kind: it changes which pairs outside the table are
  found to have no contact, adds no carrier curve, and touches none of
  those sections.

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
| Circle or ellipse, cylinder; any other | The square of the distance from the conic's point to the cylinder's axis is a trigonometric polynomial of degree two in the conic's parameter; where it passes, or comes within tolerance of, the square of the radius ([the crossing specification](../general-boolean-crossing/SPEC.md), Q1 to Q6) | 0 to 4 points with tangency flag, conic in the surface, none. |

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
  *shared*, with that pair's same or opposite flag. Outside all of them: the
  piece is not shared and is classified by K2. A point that is inside none
  of them and on the boundary of one decides nothing, and the next
  interior point of the piece is taken, from the same points K2 uses. A
  valid input needs this: an edge that the other face uses twice, a seam
  or the radial edge of a polar plane chart (I4), runs through that
  face's region, and X9 reports a point on it as boundary. If every
  interior point tried is on a boundary, the call returns
  `ErrInconsistent`.
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
  points and are not a repeated corner. After M4, every curved edge of
  the result is cut into arcs of at most a quarter turn, the first step
  of B2. The vertices this adds are corners like any other, so a circle
  that touches an edge has corners beside the point of contact wherever
  its own vertex lies. Then, for each face that has a repeated corner, in
  index order:
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
     can lack one outright, when no chord between two of its corners
     stays inside it. Adding a vertex beyond those of the quarter-turn
     cut, to make a candidate, is not part of the trial, with one
     exception made since: on a cylinder, a chord round it that crosses
     the seam of the face, or that runs from a corner to the seam,
     which adds a vertex to the seam (K3 and Z1 of
     [the crossing specification](../general-boolean-crossing/SPEC.md)).
     If two
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
  vertices on curved edges until this holds. Every curved edge is first cut
  into arcs of at most a quarter turn, and where its carrier parameter
  passes a whole turn; this cut is made before M6, so that M6 can use its
  vertices. Then, while `trimmed.NewSolid` does not accept the solid,
  every arc of the solid is halved, at most six times. All arcs are
  halved and not only those of the loop that failed, because the error
  from `NewSolid` does not reliably name the loop. If the solid is still
  refused after six halvings the call returns `ErrValidation` with the
  first refusal, the one the quarter-turn arcs met. Since then one step
  comes before the first attempt: an arc with a vertex of its face
  between it and its chord is halved (Z2 of
  [the crossing specification](../general-boolean-crossing/SPEC.md)).
  Conditioning never moves geometry. Conditioning vertices are excluded from every topology count
  this document compares. Conditioning cannot separate two corners of a
  face at one chart point; M6 has removed every such pair before assembly.
- B3. P-curves are straight for a line on a plane and for a line or circle
  on a cylinder, and `trimmed.EllipseUV` (`brep/trimmed/pcurve.go:14`) for a
  circle or ellipse on a plane and for an ellipse on a cylinder. On a
  cylinder both ends of a straight p-curve along a section circle are at
  the height of that circle, and both ends of one along a ruling at the
  place of that ruling round the axis, taken from the carrier and not
  from the computed position of each vertex: positions computed vertex
  by vertex differ in their last bits on a cylinder off the coordinate
  axes, and corners in line but for that are refused by B4 (added on
  2026-11-20, after the complete Go suite failed on it under one
  release of Go and not another).
- B4. The output solid is built with `trimmed.NewSolidContext`
  (`brep/trimmed/solid.go:110`) and the tolerance T6 states, τ + σ. An error from it is
  returned as `ErrValidation` wrapping that error, except a context error,
  which is returned unwrapped as section 9 requires.
- B5. A selection that keeps no piece returns the empty solid and no error.
- B6. Every edge of the output solid stores its carrier as a `geom.Line` or
  `geom.Ellipse` value over the edge's interval. A result is a valid
  operand of a later call whatever the Go types of its curve values are,
  since I5 only evaluates them.

## 8. Validation before return

Every non-empty result must pass all of the following. A failure returns
`ErrValidation` naming the check and the entity. Where a residual is
compared with the tolerance, in V3, V4 and the checks of V1, the tolerance
is τ + σ, with the rounding allowance σ of T9, so that geometry that sits
exactly at τ does not fail by rounding.

- V1. Structure: `trimmed.NewSolidContext` accepts the result (closed loops,
  chart winding, every edge used exactly twice in opposite directions,
  sampled agreement of each p-curve with its edge within τ).
- V2. Shells: each shell is closed and consistently oriented; each outer
  shell encloses positive volume and each void negative volume.
- V3. Edge residual: at 17 evenly spaced parameters on every edge, including
  both ends, the distance from the edge point to the carrier of each of its
  two faces is ≤ τ + σ, and the distance to the face point given by its
  p-curve is ≤ τ + σ.
- V4. Vertex residual: the distance from every vertex to the corresponding
  end point of each incident edge is ≤ τ + σ.
- V5. Volume identities. Let V(·) and area(·) be the volume and the
  surface area of A, of B and of the result. The paragraphs after the
  list say where they come from: the closed form of this package
  (RL-17), the caller (RL-10), or `trimmed.Solid.MassProperties`
  (`brep/trimmed/properties.go:33`) where the closed form does not
  apply. Let the volumes of A ∩ B, A − B, B − A and A ∪ B be
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

  Closed form (RL-17). For the surface class of the trial the volume, the
  surface area and the centroid of a solid are finite sums of closed-form
  terms, and `brep/boolean` computes them with one general routine: face
  by face, as boundary integrals in the chart of the face. The routine
  covers a face in a plane whose loops are made of lines and of arcs of
  circles and ellipses, and a face on a cylinder whose loops in the chart
  are made of rulings, arcs of section circles and arcs of section
  ellipses. That is every face of the class (S2, S3), and the routine has
  no case for a kind of solid. `Combine` uses it for the result, always,
  and for each operand whose mass properties the caller did not supply.
  For the result it reads the face and loop data that assembly hands to
  `trimmed.NewSolidContext`, after M6 and B2, which is the solid that is
  returned; it does not read the pieces or the merged faces of the
  pipeline. The fourth identity below therefore still compares what is
  returned with what the kept pieces account for. For an operand it
  reads the faces as intake reduced them (I3). If a face of a solid is
  outside what the routine covers, the mass properties of that whole
  solid are taken from `MassProperties`, with the integration options of
  the call.

  `MassProperties` remains the independent measurement, but it is no
  longer made on every call. It is made by tests. Every leaf of section
  11 that returns a solid compares the closed-form volume, area and
  centroid of its result with those `MassProperties` gives at the
  integration options of L4, and the acceptance replay does the same for
  every result in the slow tier. They must agree within
  1e-8 × max(1, |value|) for the volume and the area, and within
  1e-8 × L for the centroid. A difference beyond that fails the leaf or
  the row.

  If `MassProperties` is used for A, B or the result and returns an error, V5 cannot be
  evaluated and `Combine` returns an error: `ErrBudget` wrapping the cause
  when the cause is `parametric.ErrBudget` (the integration budget of P3 was
  exhausted), the context error unwrapped when the context was cancelled,
  and `ErrValidation` wrapping the cause otherwise.

  Known mass properties (RL-10). When the caller supplies the volume and
  the area of an operand (P3), V5 uses them as V and area of that operand
  and does not integrate it. They are trusted as given: `Combine` checks
  nothing about them beyond P4. They are meant to be values the caller holds
  for the same solid, from the report of an earlier call or from the
  definition of the shape, and the caller answers for that. The mass properties of the result are never taken from the caller:
  they are computed by this call, as the paragraphs on the closed form
  state.
  Neither the integration tolerance nor the bound β is loosened. A supplied
  volume that is wrong by more than β makes one of the identities fail,
  and the call then returns `ErrValidation` as for any other failure of
  V5. A supplied area that is wrong is not detected: the area enters
  only the bound β and the report.
- V6. Membership sampling. The sample points are terms 1 to `Samples` of the
  Halton sequence in bases 2, 3 and 5, whose three coordinates in [0, 1) map
  linearly onto the x, y and z extents of the bounding box of both operands
  inflated by 4ε. Classify each point against
  A and against B with `trimmed.Solid.Classify` at tolerance ε, and against
  the result at tolerance τ. Skipping
  points that any of the three reports as boundary, the result's answer must
  equal the operation applied to the answers for A and B.

  Which checks are independent (RL-17). V6 is not an independent check
  of classification. It does not read the piece classes of section 7.7,
  so it detects errors in splitting, selection, merging and assembly. But
  K2 classifies pieces with `trimmed.Solid.Classify` and V6 classifies its
  sample points with the same function, so a wrong answer from `Classify`
  on an operand can make both wrong in the same way, and V6 then passes.

  V5 is not an independent measurement either. The volumes of A, B and
  the result are computed by this package, in closed form, from the same
  carriers and charts the pipeline works with, and the volumes of the
  pieces come from the same arrangement. V5 is a check of selection,
  merging and assembly against the classified pieces. Its first three
  conditions say that splitting neither lost nor added volume, and they
  hold for any assignment of classes. Its fourth says that the solid
  returned encloses what the kept pieces account for. Only the bounds in
  its last line can expose a wrong class. An error that the pipeline and
  the closed form share, such as a wrong carrier taken at intake, passes
  V5. Where the caller supplies the mass properties of an operand, the
  first two conditions do compare the pieces with a value from outside
  the call.

  The checks that are independent of this package are made outside
  `Combine`: the comparison of the closed form with `MassProperties` in
  the leaves of section 11 and in the slow-tier replay (V5); the analytic
  values of section 11; the legacy values of A3; and the oracle run of
  A6, which alone uses neither `Classify` nor `MassProperties`. The trial
  relies on these to expose a wrong carrier, a wrong class or a wrong
  `Classify` answer.
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
against the pass condition. An `ErrValidation` that comes only from a
limit of `brep/trimmed` is reported under E1 and RL-9: it is not a pass,
and it does not count toward a stop until the owner rules.

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
| C20 | As C19, but the base circle touches one edge of the top face from inside at a point | One solid, V(A) + V(B); the top face of A touches itself at the point of contact and is divided in two (M6) | Empty | A |
| C21 | A box; B a cylinder through A along z with both caps flush with A's faces | A | B | Box with a through bore, V(A) − πr²h, 7 faces |
| C22 | A box; B a cylinder entering A's top face and ending inside A | V(A) + part of B outside | Part of B inside | Blind hole; V(A) − πr²·depth |
| C23 | A box; B a cylinder whose axis is oblique to the faces it passes through | One solid, V(A) + V(B) − V(common) | Part of B inside A | Through bore with elliptic openings; V(A) − V(common) |
| C24 | C4 with B moved away from A by ε / 2 | As C4 within β | Empty | A |
| C25 | C4 with B moved into A by 100ε | One solid, V = 12 − 400ε within β | V = 400ε within β | V = 8 − 400ε within β |
| C26 | C4 with B moved into A by 2ε | `ErrTolerance` or a validated result (T8) | same | same |
| C27 | Either operand empty | The other operand | Empty | A if B is empty, else empty |
| C28 | A = [0,5]×[0,8]×[2,12]; B the box with corners (3,6,2), (5,8,3), (3,9,5), (1,7,4), (5,2,6), (7,4,7), (5,5,9), (3,3,8). The corner (5,8,3) of B lies on an edge of A, B passes through the faces x = 5 and y = 8 of A, and the corner (3,6,2) of B touches the face z = 2 of A from inside | One solid, V = 410.125, 14 faces: two faces of A touch themselves at (5,8,3) and each is divided in two (M6) | One solid, V = 43.875, 8 faces | `ErrNonManifold` |

In C7 to C14 the result where an error is listed is a point set whose
boundary is not a manifold. The trial reports it; it does not represent it
(OD-3). The same holds for the cut of C28, whose cavity touches the face
z = 2 at one point.

The union of C20 is a manifold, and an earlier revision of this table was
wrong to list `ErrNonManifold` for it. Around the point of contact the
kept corners form one cycle: the side face of A, the top face of A on one
side of the point, the wall of B, the top face on the other side. Every
edge there has two uses (N1), and no touching contact is recorded (N3):
the circle and the edge are two edges that meet, which carry no label
(CC2), the base of B lies in the top face and not strictly on one side of
it (X5), and the side face of A and the wall of B share only that point,
so their line of tangency makes no trace (X6). The top face of the result
passes twice through the point and M6 divides it, between a corner of the
face and a vertex of the quarter-turn cut of the circle. This holds
wherever the vertex of the base circle lies, the point of contact
included. No other row changes with M6: in C7 to C14 an error comes from
N1, N2 or N3 before merging, and in the union of C14 and the cut of C20
the face that touches itself is merged into one without the contact (M1,
M3).

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
source lists must be the same for all of them. Source lists are compared
by operand: for each face of the result, which of A and B its sources
come from. They are not compared by the number of sources, because one
wall of a cylinder is one stored face in some forms and several in
others, four in SF3, and a merged wall names every stored face it came
from (P5):

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
middle is tested on the edge alone, below `Combine`. For the same reason
the two tests of the sampling bound of I5 step 6, an uneven
parameterization of a full circle and a curve that runs forward and then
back, are made on the edge alone: `trimmed.NewSolid` would reject either
curve in a solid. In `internal/plaincadbridge`, the
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
within τ. Five rows also fix the lists. Where a list below names the
lateral face of a cylinder, it means every stored face of that operand's
wall, one or several by the stored form; what is fixed is which operands
the sources come from and that each is a face of the wall:

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

  Two rules of the bridge's own operations hold on the general path as on
  the legacy paths (RL-15), and a request that breaks one fails. A `cut`,
  and a `cutMany` as a whole, must remove material and must leave some:
  it fails when the result is empty, or when its volume is not below the
  volume of the body by the bridge's own measure (`removesMaterial`). A
  `joinAll` must give one solid and must add volume. Forty-one recorded
  failures of the frozen rows are these rules. They are stated on the
  volumes and the solid count of results and do not depend on the family
  of the operands (PC2).

  Where a cut passes that measure and the two volumes differ by no more
  than ε × the area of the result, the volumes do not settle it (RL-29):
  a boundary known to ε accounts for a difference of that size. The
  adapter then asks `Combine` for the common of the body and each tool.
  The cut fails as one that removes no material when every common is the
  empty solid. It stands when a common has a solid, and when `Combine`
  returns an error for a common, since the volumes are then the only
  evidence. The case this rule is for is a tool with a face within ε of a
  face of the body (C24): the result is measured as built, with the wall
  of the tool ending where the tool ended, and the same tool cut again
  finds a volume that differs by the gap times the area of the wall's
  end.

  A `joinAll` is held to the same rule on the volume it must add. The
  request names target bodies and one tool; the fold unites the targets
  and then the tool, and the tool is what must add volume. The rule is
  applied once, at that last step. Where the volume of the result is
  above the volume of the union of the targets by no more than ε × the
  area of the result, the adapter asks `Combine` for the tool less the
  union of the targets. The join fails as one that adds no volume when
  that is the empty solid, and stands otherwise, an error from `Combine`
  included.

  Mass properties in a fold (RL-10). Each step passes, as the known mass
  properties of its first operand, the volume and the area that the
  result of the step before reported, so that an intermediate solid is
  integrated once and not again as an operand.

  Order of the tools of a `cutMany`. A tool whose step returns
  `ErrUnsupported` is set aside and tried again after the others, round
  after round while each round applies at least one; a round that applies
  none fails the request with its first error
  ([the crossing specification](../general-boolean-crossing/SPEC.md),
  F1 to F7).
- CO2. `overlapVolume(a, b)` is V(a ∩ b), and 0 when the common is empty.
  When the common is pinched into parts that meet only at points, N2
  refuses it. The request then answers V(a) − V(a − b) from the cut,
  which is the same quantity; where the cut fails as well it fails with
  the error of the common
  ([the crossing specification](../general-boolean-crossing/SPEC.md),
  P1 to P4).
- CO3. `prismToFace` and `circleToFace` build a tool that ends on a target
  face. Their result is reproduced as the common of the sweep, extended past
  the target plane, with a block one face of which lies on the target plane.
  Three rules of the operation hold on the general path as on the legacy
  path, and a request that breaks one fails:
  - the target body still has a face on the target plane with the stated
    normal;
  - the whole profile lies before that plane along the sweep;
  - the target covers the end of the tool (RL-16): every point of the
    tool's end cap, which is the face of the tool on the target plane,
    lies on a face of the target body on that plane that faces the tool.

  The third rule is part of the operation and not of one path. The
  plane is only a locator and never stands in for the face, as the
  bridge's own code says of it, and the legacy path fails with
  "termination face does not cover the projected profile". Without the
  rule the general path returns a tool that ends in mid air where the
  record fails, which four frozen rows showed. Coverage is decided with
  `Combine` and its face source lists (P5), and not by a rule about the
  profile or the target (PC2). With T the tool, the end cap is covered
  exactly when no face of the union of T with the target body, and no
  face of their common, has the end cap of T among its sources. A part of
  the cap that lies on a face of the target facing the tool is a shared
  piece with opposite normals, which both operations drop (7.8); a part
  that hangs free is kept by the union; a part inside the target's
  material, or on a face of it that faces away, is kept by the common or
  the union. An error from either call fails the request.
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
  boundary for every operand in the class at tolerance ε. The trial's own
  commits do not edit `brep/trimmed` (G4, L5). What happens when the
  package fails on a valid input depends on the failure:

  - A limit of `brep/trimmed` that a valid result or operand exposes: the
    package cannot represent, or wrongly rejects, a result that meets B2
    and B3, or returns an error where it should answer for an operand in
    the class.
    This does not end the trial (RL-9, section 19). The limit is fixed in
    `brep/trimmed` itself, on a separate foundation branch,
    `codex/trimmed-collinear-corners`, which is merged into the trial
    branch. Until the fix is merged, `Combine` returns `ErrValidation` for
    such input (B2, B4) and does not work around the limit. A leaf or row
    that fails for such a limit alone is reported under that reason, with
    its input. It is not a pass, and it does not count toward ST3 or ST5
    until the owner rules.
  - Any other failure, such as a wrong answer from `Classify` on a valid
    operand: the case is recorded and the trial stops under section 16
    (ST5).

  Three limits were known after Phase 2, and the first run of the
  acceptance rows added three:

  - Collinear corners. `NewSolidContext` rejects a valid planar result
    with "mesh: triangle N is degenerate or overflows" when a face in a
    plane that is not axis-aligned has three consecutive corners on one
    straight line: the three points are exactly collinear in space and
    their rounded chart coordinates are not. The stress test of the
    planar phase met this in 12 of 4,500 operations. The smallest input
    is A = [3,11]×[4,8]×[4,12] and B the box with corners (2,0,−1),
    (6,4,1), (2,6,5), (−2,2,3), (3,−2,1), (7,2,3), (3,4,7), (−1,0,5),
    union, where a face of B has the corners (2,6,5), (3,5.5,4), (6,4,1)
    in a row. No matrix leaf has met it.
  - Arcs of two curves at one vertex. `NewSolid` requires the end of one
    use and the start of the next to be the same bits, each being what its
    p-curve evaluates to, and `Classify` accepts only `trimmed.EllipseUV`
    as an arc p-curve from outside the package. Two different `EllipseUV`
    arcs give a common point values that differ in the last bits. So a
    face in which arcs of two different circles or ellipses meet at a
    vertex cannot be built. This blocks all three leaves of C18, whose
    caps are bounded by arcs of two circles, and every other result with
    a lens-shaped face; the Tier 1 families Bore union and Side dual disk
    are overlapping parallel bores and probably depend on it. In the
    cylinder stress test of Phase 2 it was 162 of 600 operations on two
    upright cylinders. `Combine` reports the case itself, before it calls
    `NewSolid`.
  - A display mesh that does not close. `NewSolidContext` rejects the
    union of C23 with "mesh: edge is not closed and consistently
    oriented", in every conditioning round. The input is A =
    [0,4]×[0,4]×[0,2] and B a cylinder of radius 0.5 and length 6 whose
    axis passes through (2,2,1) at 30° from the vertical. The cut and the
    common of the same operands pass. Row T1-029 meets it too.
  - `Classify` on an unexported arc p-curve. A body that the bridge's
    STEP import stores as a bore union has p-curves of the unexported
    type `trimmed.unionArcUV`, and `Classify` returns "unsupported ray
    p-curve" for it, so K2 cannot classify against that operand. Rows
    T1-041, T1-051, T1-053, T1-054, T1-066 and T1-067.
  - A display triangle on a seam. The display triangulation of a face
    that winds once around a cylinder, whose seam is both vertical sides
    of its chart rectangle, can make a triangle with two corners on the
    seam and a normal of exactly zero, and `NewSolidContext` then
    returns "mesh: triangle N is degenerate or overflows" in every
    conditioning round. Rows T1-083, T1-034 and T1-041. The smallest
    input is the box [0,60]×[0,40]×[0,20] less the box
    [12,24]×[12,20]×[14,20], less a cylinder of radius 2 from (45,25,20)
    down 20, less a cylinder of radius 3 from (45,25,20) down 1.
  - `Tessellate` and a positive angle bound. On a wall with a seam
    `Solid.Tessellate` never meets a positive angle bound, and the
    bridge's `mesh` request returns "tessellation budget exceeded" with
    its default options. Rows T1-029 and T1-092.

  Four of these are written up as defect reports for the work on the
  foundation branch (RL-9): the triangle on a seam, `Classify` and
  `unionArcUV`, `Tessellate` with a positive angle bound, and the mesh
  edge that is not closed.

  The foundation branch ended at `b0c23193` and is merged into the trial
  branch (RL-26). It fixed collinear corners, arcs of two curves at one
  vertex, unclosed edges on a wall with a seam, which also removed the
  triangle on a seam and a disconnected fan of triangles, curved trims
  that left their seed triangle, and part of tessellation with an angle
  bound. No leaf of section 11 is held by a limit any longer. Two Tier 1
  rows are:
  - T1-092. Its Booleans and its to-face rules pass. Its `mesh` request
    with an angle bound still returns "tessellation budget exceeded".
  - T1-029. Every checked response agrees. It takes 3.6 seconds alone,
    over the bound of A7, because the display meshes its requests ask
    for have 557,056 and 139,264 triangles, where the default bridge's
    own mesher returns 144 for that bore. The time is spent in
    `Tessellate` of `brep/trimmed` and not in `Combine`, so the row is
    counted as held by a limit of that package (RL-26): it is not a
    pass, and it does not count toward a stop until the owner rules.

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
  holding the result solid (RL-2), with the volume and the area that the
  result report gives (RL-10); the adapter does not integrate the result
  again. A result that is not empty is stored by calling the bridge's
  `publish` directly from the adapter file, and not through
  `putAnalyticBody`: that function accepts one shell only, so it cannot
  take a result with a void or with several solids, and it pads the
  bounds. An empty result is published through `put`, as the legacy paths
  publish one. A stored shape with no such solid returns the bridge's
  `UNSUPPORTED` failure. The adapter reads the shell grouping of the
  result for `solidCount`.

  Bounds (RL-15). The bounds the adapter publishes are the box of the
  result's edges, each curved edge taken with its true extent, found from
  its turning points and not from samples. They are not those of
  `trimmed.Solid.Bounds`, as an earlier revision had it: that function
  boxes the whole parameter rectangle of every face and was up to 30
  units too wide on results of the trial, besides padding by the
  tolerance. The box of the edges contains the solid for the surface
  class of S2. A bounded face in a plane lies in the convex hull of its
  boundary. Every point of a face on a cylinder lies on a ruling, and
  the part of that ruling inside the face ends on the boundary of the
  face at both ends, so the point lies between two boundary points. No
  coordinate of a face therefore exceeds those of its edges. A test
  must show this on results of section 11, by comparing the published
  box with points sampled on every face.

  Known mass properties (RL-10). The adapter passes, as the known mass
  properties of an operand (P3), the volume and the area the bridge
  stores for that shape. For a body the adapter published they are the
  values of the result report. For a legacy shape they are the bridge's
  own closed forms for that shape, and not a measurement of the solid it
  lifts to. V5 then compares the pieces of the lifted solid with a value
  that did not come from it, and fails where the two disagree by more
  than β; before, it could not notice such a disagreement. Within a fold
  of CO1 each step's reported values go to the next step.

  Source lists (RL-18). The adapter may read the face source lists of a
  result, and does so in one place, the coverage rule of CO3. The list is a field of the result type, in the
  contract at the owner's request, and is otherwise read by the tests of
  section 11.
- L3. The general path is selected per `Bridge` value by an unexported field
  set through a constructor used only by tests. The default bridge behavior
  does not change during the trial, and there is no environment variable or
  package-level switch. The selector needs two kinds of edit outside the
  adapter file, and both are allowed: the declaration of that field on
  `Bridge` in `bridge.go`, and one branch in each bridge operation the
  trial serves (`cut`, `fuse`, `common`, `cutMany`, `joinAll`,
  `overlapVolume`, `prismToFace`, `circleToFace`) that calls the adapter
  when the field is set, placed after the operation's existing argument
  validation (CO1). A bridge whose field is not set never reaches those
  branches. One further change outside the adapter file is allowed
  (RL-2, RL-15): in the `transform` request of `bridge.go`, one branch
  behind the same field sends every analytic body that the adapter
  published, of one shell or of several, back to the adapter to be
  republished. Several shells need it because `putAnalyticBody` fails
  with "expected one connected analytic shell". One shell needs it too,
  because `putAnalyticBody` pads the bounds by the tolerance times the
  norm of the matrix, 1.7e-7 for a permutation of the axes, and A3
  compares at 1e-7. A default bridge never takes that branch, so its
  `transform` is unchanged; the existing tests of the default bridge and
  the trace replay on it verify that.
- L4. Bridge options for the trial: `Linear` 1e-7, `Relative` 0,
  `Angular` 1e-9, `Volume` 1e-7, and integration options
  (`parametric.IntegrationOptions`) with relative tolerance 1e-9 and a
  sample budget, `MaxSamples`, of 64,000,000. Since RL-17 `Combine`
  integrates numerically only a solid that the closed form of V5 does not
  cover, so these options no longer set the time of a call. They are the
  options of that fallback and of the comparisons that V5 requires of
  the tests. The sample budget is stated because the default is not
  enough: at relative tolerance 1e-9 with the default budget of
  4,000,000 samples, `MassProperties` runs out of samples on three
  results with curved caps (the tube of C17, the common of C23, and a
  leaning cylinder that enters a top face), and with 64,000,000 all three
  converge and match their analytic volumes to 1e-9. When the acceptance
  rows first ran on the general bridge, integration was 84 percent of the
  time in bridge requests. Known mass properties (RL-10) removed the
  repeated integrations, and nine Tier 1 rows still failed A7 for the one
  integration of each result, at 5 to 25 seconds a row; that is what
  RL-17 removes.
- L5. No file in `geom` is changed, and no commit of the trial changes a
  file in `brep/trimmed`. Changes to `brep/trimmed` that fix a limit E1
  names are made on the foundation branch and reach the trial branch only
  by a merge of that branch (RL-9). No dependency is added.
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
(15.5); supported after the trial (RL-27), with the outcome of each row
in the third record of [the acceptance record](ACCEPTANCE.md):

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
- A recorded value that depends on a feature smaller than the bridge
  tolerance (RL-12). Row T1-009, `TestBlindFloorNearExitReferenceIsAmbiguous`,
  cuts a box of height 8 with a cylinder of height 7.99999995 and records
  a blind hole whose floor is 5e-8 thick. The bridge tolerance is 1e-7
  (L4). Under the tolerance policy (G1) the cap of the tool and the face
  of the stock are then coincident, row C24 governs, and the general
  path returns the through hole. The record and C24 cannot both hold, so
  the row is excluded. Any other row whose recorded result depends on a
  separation between the operands that is below ε is excluded for the
  same reason, stated in its row. Row T1-091 is one (RL-19): it gives a
  tool tangent to a hole and the same tool 1e-15 away, which are one
  input at the working tolerance and get one outcome, while the record
  expects two.
- No row is excluded for a placement since RL-28. Row T1-041 mirrors a
  notched stock with the `transform` request and cuts it again; RL-19
  excluded it while S4 required det M > 0.

  With T1-009 and T1-091 excluded, 90 of the 92 frozen Tier 1 rows are
  in scope.

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
    1e-7 × max(1, |expected|). The values compared are those the result
    report gives, which are the closed forms of V5 (RL-17);
  - bounds equal the legacy bounds within 1e-7 in each coordinate;
  - the mesh of the result is watertight under the bridge's existing mesh
    checks.
- A4. Recorded failures (RL-11). Where the source test expects a failure,
  what the general path must do depends on what the failure is.
  - A contract failure must still fail. It follows from the bridge
    operation itself, whichever path serves it: the bridge rejects the
    request before any Boolean is attempted (argument validation, handles,
    limits; CO1); a rule that section 12 states for the operation is
    broken (a cut that removes nothing or leaves nothing, a `joinAll`
    that does not give one solid or adds no volume, the three rules of
    CO3); or this document defines the result as an error, such as a
    boundary that is not a manifold (OD-3).
  - A declined input need not fail. It is a failure by which the legacy
    path that served the request says that the input is outside what that
    path represents or measures, where none of the above applies.
    Recorded examples are "finite cylinder is not a measurable entry-open
    blind pocket or through bore", "coaxial notch cuts require connected
    axial spans" and "overlap volume requires a strictly contained radial
    footprint". Reproducing such a refusal would need a rule that depends
    on the family (ST2). If the general path fails too, with any error,
    the response passes. If it returns a result, that result has passed
    section 8 inside `Combine`; the response then passes, the row is
    marked in `ACCEPTANCE.md` as succeeding where the record declines,
    and, there being no recorded value to compare with, the case joins
    the oracle run (A6).

  `ACCEPTANCE.md` lists every recorded failure of a Boolean request that
  the general path does not reproduce, with its message and which of the
  two kinds it is, so that the owner can check the classification. A
  failure whose kind these rules do not settle is treated as a contract
  failure until the owner rules. The declined inputs are data, in
  `internal/plaincadbridge/testdata/general_boolean/legacy_refusals.txt`,
  by row and line of the trace with the legacy message. The replay reads
  that file and has no other knowledge of them.
- A5. The section 11 matrix passes in `brep/boolean`.
- A6. Oracle. One helper builds a case from data (boxes, prisms and cylinders
  with placements, combined by union, common and cut) with the pinned OCCT
  and reports validity, volume, area, centroid and solid count. Cases are
  rows of a data file; there is no per-case C++. The cases are every
  section 11 row that returns a solid, except the tolerance rows C24 to
  C26; one case per Tier 1 family; and every case that A4 marks as
  succeeding where the record declines. The comparison bound is that of
  `internal/occtcheck/oracle_test.go:68` (3e-7 relative for area and volume,
  3e-7 absolute for centroid). Face counts are compared for the section 11
  rows that state one, the dividers of M6 not counted.

  What the run does not compare:
  - C24 to C26, which depend on ε. The oracle does not share it, and they
    are checked against their analytic values only.
  - C27, which has one operand against nothing and so no Boolean to make.
  - The To face family. Its tool is the result of a to-face request and
    not a box, a prism or a cylinder, so the family has no independent
    check.
  - Tier 2, which is not supported (RL-22).
  - The rest of a family's row. A family case holds the first bridge of
    the first row of its family and stops at the first request the
    primitives cannot express, so it covers the start of that row and
    not the whole row.
  - A case marked by A4 that cannot be built from the helper's
    primitives, for example one with a STEP-imported operand. It is
    reported as succeeding without an independent check.

  The run is the last verification step of the trial, after Tier 2 has
  been kept or reverted (15.5). The owner approved one run (OD-5). By
  ruling two were made (RL-21, RL-25), and both are recorded in
  `ACCEPTANCE.md`.
- A7. Every acceptance row and matrix row runs in under two seconds under
  `go test` on the 14-core development machine that `tools/ci/README.md`
  refers to, with the default settings of `go test` (no race detector, no
  change to `-p` or `-parallel`). The time of a row is the elapsed time
  that `go test -json` reports for its subtest, which includes building its
  operands, when the row is run alone, by name. A row that needs longer
  fails A7. It is not made to pass by moving it to the slow tier.

  A7 times a row. The rule of `AGENTS.md` (G2) times a test function, and
  a function that holds many rows meets it by running them as parallel
  subtests (RL-7); that uses the default `-parallel` and changes no
  setting. Rows that run side by side take longer and vary from run to
  run, so the replay test treats time in its own way (RL-15, RL-20). In
  the default tier it logs the time of a row and fails none for it: the
  replay then runs beside every other package, and a row that takes 1.6
  seconds alone took 5.5 seconds there. A row listed in the pending list
  as taking more than two seconds runs only with `PLAINSOLID_SLOW=1`. In
  the slow tier, where the replay runs by itself, a row that is not
  listed fails for its time beyond four seconds. None of this changes
  what the pass needs: every Tier 1 row in scope under two seconds when
  run alone, and none left on the list of slow rows.
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
  does not gate. The values are the closed forms of V5 (RL-17). In the
  slow tier the test also logs for each row the largest difference
  between the closed form and `MassProperties`, in volume and in area,
  and the report to the owner gives the largest of each over the rows.
- A9. Audit of refusals (RL-23). A refusal as non-manifold is an explicit
  error, and a false one hides a result that exists. Before a pass is
  declared, on the final code, every `ErrNonManifold` that the stress
  sets return is confirmed by a method that does not reuse the
  classification of this package: the contact the error names must be
  shown to be a place where the boundary of the result is not a
  manifold. The stress sets are the five the trial ran, each with union,
  common and cut and with volumes compared with `csg/poly` or a closed
  form: pairs of axis-aligned boxes on a grid; chains that feed a result
  back as an operand; a rotated box with integer corners against a box;
  a box against an upright cylinder; and two upright cylinders. Two
  methods were used, and each is accepted where it is decisive.
  - For a contact between planar faces: the operands are taken as
    membership tests in closed form and the result as the Boolean of
    those tests, and the connected regions inside and outside the result
    are counted on small spheres about the contact. At a manifold point
    there is one of each. This is not decisive where two surfaces are
    tangent.
  - For a tangent contact: operand B is moved by an amount far above the
    tolerance and far below any feature, in fourteen directions, six
    along the axes and eight diagonal, and the numbers of solids, voids
    and handles of each result are noted. The trial moved B by 0.001, in
    sets whose features are no smaller than 0.5. If
    they are the same for all, nothing changes at the unmoved
    configuration, and a refusal there refuses a manifold.

  A refusal that neither method settles is settled by hand and named in
  the report. A refusal found false is a defect of section 7.3 or 7.9
  and fails the audit. The sets and the audit run outside the default
  tier, and their outputs go with the report to the owner.

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

- PC1. Every Tier 1 row in scope (15.2) passes A3, A4 and A7, section 11
  passes (A5), the oracle run passes (A6), the audit of refusals finds
  none false (A9), and the record of A8 is complete for every Tier 1
  row in scope. `ErrInconsistent` or `ErrValidation` on any of them is a failure,
  and so is a leaf or row held back by a limit of `brep/trimmed` (E1,
  RL-9): PC1 holds only when it passes, after the fix on the foundation
  branch has been merged. A row with a response that A4 marks as
  succeeding where the record declines passes A4, and the oracle run
  must agree on that case (A6). A row excluded under 15.2 is not a Tier 1
  row. The pass of 2026-11-19 was declared with two rows that do not meet
  this criterion, by the owner's decision OD-9; the criterion itself is
  unchanged.
- PC2. One path. `brep/boolean` exports one operation entry point; the bridge
  file of L2 reads a stored shape only to obtain its `trimmed.Solid` and,
  the volume and area the bridge stores for it (L2); it may read the face
  source lists of a result to apply the coverage rule of CO3, which is a
  reading of what `Combine` returned and not a test of what kind of
  feature an operand is (RL-18); and no
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
  file of L2 at most 600. The second figure was 400 (RL-24). The cap is
  there to catch rules about families growing in the adapter, and its
  purpose is unchanged.

PC1 to PC4 decide the pass. Tier 2 does not (OD-1): it is attempted only
after PC1 to PC4 hold in every respect except the oracle run, which comes
last (A6) so that the one approved run tests the final code. Tier 2 is
made by changes to the tables
of 7.4 and 7.5 and their tests only (SP5). Tier 2 is *supported* when its
rows then pass A3 and A4 while every Tier 1 row, section 11 and PC2 to PC4
still hold; otherwise it is reported as unsupported, the change is not
kept, and the trial goes on to the oracle run without it.

Two things were done in another order, by ruling (RL-21). Tier 2 was
attempted before every Tier 1 row had passed, and the oracle run was made
before the last rows held by limits of `brep/trimmed` were settled. Both
were allowed because those rows waited on `brep/trimmed` and not on the
Boolean. Tier 2 is not supported (RL-22): the attempt was not kept and is
on its own branch.

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
- ST5. Foundation. `Classify` or another function of `brep/trimmed` gives
  a wrong answer on a valid input, or a Tier 1 row cannot pass without
  changing `geom` or adding a dependency. A limit of `brep/trimmed` that
  a valid result exposes is not counted here: E1 and RL-9 send it to the
  foundation branch, and it counts only if the owner rules that it does.
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

The owner decided OD-1 to OD-8 on 2026-11-18 and OD-9 on 2026-11-19. None
is open. Each entry states the decision and where this document applies it.

- OD-1. Tier 2 is in the trial. Equal-radius cylinders with meeting axes
  give two ellipses, which is inside the edge class, and cover about 17
  bridge tests and the crossed-bore constructors. The two ellipses meet at
  two points where the surfaces are tangent. Decided: Tier 2 is attempted
  last, as a test of SP5, and is not required for a pass (7.4, 15.1, 15.5).
  After the trial the owner had Tier 2 specified and built on its own
  (RL-27).
- OD-2. Unequal and skew crossed bores are out. Their intersection curves
  are quartic (SP3). Their constructors and five test files stay whatever
  the trial's outcome (15.2). Reversed on 2026-11-20 for regular pairs,
  at the owner's instruction to start them (RL-31); it stands for pairs
  that touch or nearly touch.
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
- OD-9. The trial is a pass, with two exceptions. PC1 asks that every
  Tier 1 row in scope pass, and 87 of the 89 do. T1-092 and T1-029 do
  not: the first exhausts the tessellation budget of `brep/trimmed` on a
  `mesh` request with an angle bound, and the second agrees in every value
  but takes longer than A7 allows because of the size of its display
  meshes (E1). Both are limits of the mesher and not of the Boolean, and
  the owner accepts them as recorded exceptions. This settles OP-8. No
  stop condition applies. Tier 2 stays unsupported (RL-22) and does not
  decide the pass (OD-1). The pass rests on RL-1, RL-9, RL-11, RL-12,
  RL-19 and RL-26, which were put to the owner before the declaration and
  are confirmed by it. RL-3, RL-4, RL-22, RL-24 and RL-25 were put to the
  owner at the same time and were not reversed. The other rulings were
  not put to the owner one by one and stand as section 19 says. A pass
  changes no default: the default bridge stays on the legacy path (L3),
  and the work on the mesher that the two rows need comes before any
  family is moved to the general path.
- OD-10. Threads stay on the legacy path. Decided on 2026-11-20. The
  bridge's `thread` request cuts nothing: it builds the threaded solid
  from a recipe that the stock holds, which is a cylinder, a stepped
  shaft, a tube or a block with a bore. A primitive gives the stock that
  recipe, and one Boolean keeps it, the legacy cut of a socket into the
  head of a stepped shaft (`cutShaftSocket`). A result of the general
  Boolean holds no recipe, and a thread on one is refused as unsupported.
  That cut is not a bore family and is not moved to the general path, so
  moving the bore families leaves threads as they are. Two ways to
  thread a general result were set aside: to recognize a stepped shaft
  in a boundary, which is a function for one arrangement of inputs, and
  to replace a complete cylinder face of any solid by a threaded band,
  which is a mechanism of `brep/trimmed` that no bridge operation needs
  while the socket cut stays where it is.
- OD-11. The plain bores move to the general path. Decided on
  2026-11-20. This ends the part of L3 and of OD-9 that kept the default
  bridge on the legacy paths; the selector of L3 stays for the tests.

  On the default bridge the general path is asked first for two kinds of
  request, and a legacy path only where `Combine` or the adapter fails
  it. The first is a `cut` or a `cutMany` of cylinders from planar
  stock. The second is a `cut`, `fuse`, `common`, `cutMany` or
  `overlapVolume` with a result of the general path among its operands,
  which no legacy path takes. Every other request is answered as before:
  a legacy path first, and the general path where that declines the
  request as unsupported. A bridge built by a second test constructor
  keeps that older order for every request, for the tests of the legacy
  paths and of the fallback.

  Where the general path fails a request it is asked first for, and the
  legacy path fails it too, the response is the failure of the legacy
  path. The `transform` request republishes a result of the general
  path through the adapter on every bridge, of one shell or of several,
  as L3 describes for the selected bridge. The baseline of A2 and A3 is
  the frozen record of each row: the replay that checks the records
  against the legacy paths runs on the legacy-first bridge, and the rows
  are replayed against their records on the general bridge and on the
  default bridge.

  A result of the general path that is planar stock less cylinders keeps
  that stock and those cylinders, in order, through `clone` and
  `transform`. Where the general path fails a request on such a result,
  the legacy path is given the bore body that the legacy bore engine
  builds from them, with the measures the result was published with. So
  a bore that the general path does not cut, one of another radius or
  with an axis that passes the first's (OD-2), is still cut, and the
  result is then a legacy bore body.

  STEP files are not required to be byte for byte the same. Two results
  of the general path that are one solid differ in the last digits of
  their coordinates and in the order of their entities with the order
  they were cut in, and so do a file and the export of what was read
  from it. They are required to state one solid: read back, the shapes
  agree within the tolerance of A3. The STEP products of the legacy bore
  families, with their names, their units and the recipes they restore,
  are written by a legacy bore body and not by a result of the general
  path.

## 19. Rulings made under delegation

The reviewing session made these on 2026-11-18, while the owner was away,
under the owner's delegation to it. Each stands until the owner confirms or
reverses it; OD-9 says which the owner has confirmed. Each entry states the
ruling and where it is applied.

- RL-1. A known defect of `brep/trimmed`, the rejection of some planar
  results with three collinear corners, does not end the trial. RL-9 now
  states this for every such limit and replaces the wording of RL-1; the
  collinear-corner case is the first of the limits E1 lists.
- RL-2. Publication (the plan's decision D4). The adapter stores a result
  with a void or with several solids by calling `publish` directly, and an
  empty result through `put`. It removes the tolerance pad of
  `trimmed.Solid.Bounds` before publishing bounds. One minimal change to
  the `transform` path of `bridge.go` is allowed, so that it republishes
  an analytic body of more than one shell, with the behavior for one
  shell unchanged (L2, L3). RL-15 later
  replaced two parts of this ruling: the bounds are the box of the
  result's edges, and `transform` republishes every body the adapter
  published.
- RL-3. Faces stored as spline patches stay excluded. The freeze counted no
  Boolean response lost to them (S2, 15.2).
- RL-4. Two Boolean responses on STEP-imported bodies stay unchecked. The
  bodies have faces stored as evaluators that S2 does not accept (15.2).
- RL-5. Size of the frozen traces. They are 7.9 MB, 91 percent of it STEP
  text inside `importStep` requests. This is accepted on the trial branch.
  It is a condition of any later merge of the trial to `main` that the
  distinct STEP sources are stored compressed or are otherwise reduced.
  The condition is met on the trial branch at `5f5b03ec`: each distinct
  text is stored once, gzip-compressed, and the traces with their STEP
  files are 1.9 MB.
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
  The union of C20 was corrected with it: N1 to N3 pass there, so the row
  states a solid with a divided top face, and M6 takes its corners after
  the quarter-turn cut of B2 so that the outcome does not depend on where
  the vertex of the base circle lies.
- RL-9. Limits of `brep/trimmed`. A limit of `brep/trimmed` that a valid
  result exposes is fixed in `brep/trimmed`, on a separate foundation
  branch, `codex/trimmed-collinear-corners`, which is merged into the
  trial branch. The trial's own commits still do not edit `brep/trimmed`.
  A leaf or row that fails only for such a limit is reported under that
  reason and does not count toward ST3 or ST5 until the owner rules. It
  is not a pass either: PC1 holds only when the leaf or row passes. Three
  limits were known when this was ruled: collinear corners (RL-1), arcs of
  two curves at one vertex of a face, which blocks C18, and the display
  mesh of the union of C23. E1 lists those found since (G4, E1, L5, ST5,
  PC1, section 9).

Rulings of 2026-11-19, after the acceptance rows first ran on the general
path (36 of 92 Tier 1 rows passing, no wrong volume):

- RL-10. Mass properties in the contract. Integration was 84 percent of
  the time in bridge requests: 64 percent inside `Combine` for V5 and 20
  percent in the adapter, which integrated each result again for its
  area. The result report now carries surface area with volume, and a
  caller may pass known mass properties for an operand, which V5 uses
  without integrating that operand and trusts as given. The adapter and
  the folds pass them along. The integration tolerance and the bound of
  V5 are not loosened (P3, P4, P5, V5, CO1, L2). The report carries the
  area of the operands as well as of the result, so that a request that
  calls `Combine` several times on one solid integrates it once.
- RL-11. Recorded failures. A recorded failure that is a contract failure
  must still fail on the general path. One that is only the legacy path
  declining an input need not: if the general result passes section 8,
  the row passes, is marked in `ACCEPTANCE.md`, and joins the cases of
  the oracle run (A4, A6, PC1).
- RL-12. A recorded row that depends on a feature below the bridge
  tolerance is excluded. T1-009 records a floor of 5e-8 under a tolerance
  of 1e-7; that is a coincidence, and C24 governs (15.2).
- RL-13. Rounding at the tolerance. A residual compared with τ must not
  fail by rounding when the geometry sits exactly at τ. One allowance,
  stated once, is used (T9, T6, T8, section 8, B4). It also applies to
  the tolerance handed to `trimmed.NewSolid`, so the stored solid's
  tolerance is τ plus the allowance while the report gives τ. Row T1-005, with a
  gap of exactly ε, showed the need.
- RL-14. A sharper separation proof may be added to SP2, provided it is a
  proof and general. Row T1-034, two bores at right angles whose finite
  walls do not meet, needs one (SP2, SP5).
- RL-15. Adapter behavior, confirmed: the published bounds are the box of
  the result's edges with each arc's true extent; on a general bridge
  every body the adapter published is republished by the adapter under
  `transform`; `cut` and `cutMany` fail when nothing is removed or
  nothing is left, and `joinAll` requires one solid and added volume;
  in the replay test a row known to exceed two seconds runs only with
  `PLAINSOLID_SLOW=1`, and a row not listed fails on time only beyond
  four seconds (L2, L3, CO1, A7).
- RL-16. To-face coverage. The reviewing session left it to the session
  that keeps this document to decide whether CO3 needs a coverage rule,
  four rows succeeding where the record fails. It does: the rule is part
  of the operation, and without it the general path returns a different
  shape where the bridge reports a failure. CO3 states the rule and a
  general way to decide it. Those four recorded failures are contract
  failures under A4.
- RL-17. Closed-form mass properties. After RL-10, nine Tier 1 rows still
  failed only on the two-second bound, and what remained of their time
  was one numerical integration of each result at 1e-9 for V5. For the
  trial's class the integrals have closed forms. `brep/boolean` computes
  the volume, the area and the centroid of a result, and of an operand
  whose values the caller did not supply, in closed form, with one
  general routine over planar faces bounded by lines and conic arcs and
  cylinder faces whose chart boundary is lines, circles and ellipse
  sections. V5 and the report use it. `trimmed.Solid.MassProperties`
  stays as the independent check, made by the leaves of section 11 and
  by the slow-tier replay and not on every call. A solid with a face
  outside what the closed form covers is integrated numerically. V5 is
  then a check of selection, merging and assembly against the classified
  pieces, not an independent measurement (P3, P5, V5, section 8, L4, A3,
  A8).
- RL-18. The adapter may read the face source lists of a result in order
  to apply the coverage rule of CO3 (L2, PC2).

Rulings of 2026-11-19, as the trial came to its closing state:

- RL-19. Two more rows are excluded. T1-091 is a distinction below the
  tolerance, like T1-009. T1-041 has a stock that is an import with a
  placement that is not rigid (S4). In scope are 89 of the 92 Tier 1
  rows (15.2, PC1).
- RL-20. The replay fails a row for its time only in the slow tier, where
  it runs by itself; in the default tier it logs the time. T1-065 passes
  A7 on its time alone (A7).
- RL-21. Order of the phases. Phase 4 was attempted before gate G3, and
  the oracle run was made before the last rows limited by the foundation
  were settled, because those rows waited on `brep/trimmed` and not on
  the Boolean (15.5, A6).
- RL-22. Tier 2 is not supported. The attempt is on
  `codex/general-boolean-tier2-attempt` (`cc7a679e`); 1 of 17 rows
  passes. The surface pair itself is one row of 7.4, as SP5 expects. The
  attempt needed X7, which this document states and the code lacked; X7
  is now on the trial branch with its own test. It also met two gaps in
  the rules, which section 20 names as open, and one row needs an entry
  of 7.5 for a conic with a cylinder (X7, CO2, 15.5).
- RL-23. Touching contacts at an edge. An audit of the refusals as
  non-manifold, by an independent method, found 56 false refusals among
  operations on a box and a cylinder or on two cylinders. One rule now
  decides them: an edge is a touching contact only when the faces of the
  edge stay on one side of the other surface, judged from points of each
  face near the contact, and X6 makes no touching record for an edge
  that a touching trace runs along. The audit is a required check (X5,
  X6, AR7, A9, PC1).
- RL-24. The cap on the adapter file is 600 lines, raised from 400
  because comments were being shortened to fit. Its purpose is unchanged
  (PC4).
- RL-25. Oracle. Two runs were made, on `1ae12376` and on `8a76218e`.
  The reviewing session authorized the second because the touching rule
  and `brep/trimmed` had changed after the first. Both agree with OCCT on
  all 88 cases and 257 steps. OD-5 approved one run; the second is for
  the owner to confirm (A6).
- RL-26. Foundation. `codex/trimmed-collinear-corners` ended at
  `b0c23193` and is merged into the trial branch. Two Tier 1 rows remain
  held, T1-092 and T1-029 (E1). An amendment to B2 that the reviewing
  session approved for T1-092, to cut a rim where it turns back along
  its wall, was built and measured, did not help, and made one working
  mesh fail. It was not kept and was never part of this document; it is
  on `codex/general-boolean-rim-extremes-attempt` (`36758499`).
- RL-27. Tier 2 is supported. This supersedes RL-22. On 2026-11-19 the
  owner decided the open items of
  [the crossing specification](../general-boolean-crossing/SPEC.md) and
  had it built and merged. Fifteen of the 17 rows pass and two are
  listed for time with every value right. The row of section 7.4 is in
  force with a narrower condition and a least angle; section 7.5 has
  the entry for a conic with a cylinder; SP2 has a further proof; CO1
  and CO2 are amended; OP-5, OP-6 and OP-7 are closed. OD-2 stands:
  cylinders of unequal radius, or with axes that do not meet, are out.
  One limit is recorded with the rows: a blind floor strictly inside
  the band of the smaller of the two crossing ellipses is refused in
  some arrangements.
- RL-28. A placement that reflects is accepted. This amends S2 and S4
  and withdraws the part of RL-19 that excluded T1-041. On 2026-11-20
  the owner asked for it, after a run of the bridge tests with the general Boolean first
  found that a general result mirrored by the `transform` request was
  refused as stock. The change is at intake alone. T1-041 is in scope:
  every checked response agrees, with the largest error 1.6e-16 in
  volume and in area, and the row takes 2.8 to 2.9 seconds alone, so it
  is listed for its time under A7 as T2-007 and T2-013 are. Whether that
  listing stands is the owner's to confirm.
- RL-29. A cut that removes nothing, where the volumes differ within
  tolerance. This adds to RL-15. On 2026-11-20 the owner asked for it,
  after a run of the bridge tests with the general Boolean first found
  that a tool cut a second time from a bore whose floor was within ε of
  the far face was published as removing 1e-7 of volume. Section 12 has
  the rule: where the volumes differ by no more than ε × the area of the
  result, the common of the body and the tools decides. `joinAll`,
  which must add volume, had the same case: a boss 5e-8 above the face
  it is joined to, and then a tool inside the two, was published as
  adding 1e-7. It is held to the same rule at its last step, with the
  tool less the union of the targets in place of the common.
- RL-30. A face of B within ε of a plane of A is seated on it (S7). On
  2026-11-20 the PlainCAD gates on the commit that moved the plain bores
  failed in four development cases and in the production suite, all on
  one thing: the STEP export of a posed plate with a through hole. The
  application cuts such a hole with a cylinder 1e-7 longer than the
  plate is thick, which is ε. The cap of the tool was coincident with
  the far face (C24), and the result kept the rim of the tool, 1e-7 off
  that face; the export was refused when the pose had rounded the gap to
  just over the tolerance of the reader. The repeated cut of RL-29 had
  the same cause. With S7 the result is that of a through bore to
  rounding. `step.WriteTrimmedBoundary` writes a zero without a sign,
  since a carrier taken from A has negative zeros among the components
  of its axes.
- RL-31. Quartic pairs are in, where they are regular. On 2026-11-20 the
  owner asked for the skew and unequal-radius crossings to be started.
  [The quartic specification](../general-boolean-quartic/SPEC.md) adds
  one curve carrier, the closed curve in which two cylinders meet, with
  its rows in the tables of 7.4 and 7.5 found by sampling, and amends
  SP3, OD-2 and the statement of section 12 that no measure is a
  quadrature: a face bounded by such a curve takes its share of the
  measures by Gauss quadrature along that leg, and every other leg keeps
  its closed form. `brep/trimmed` gained one exported interface so that
  `Classify` crosses the new p-curve. The decisions that document takes
  by default (its section 9) wait for the owner.

## 20. Open items

These are known and not settled, apart from OP-8. No ruling covers them,
and this document does not decide them.

- OP-1. X4 still judges a vertex in a face by directions that leave the
  vertex. That is the weakness the audit found and RL-23 fixed for an
  edge; for a vertex it has not been audited.
- OP-2. A result may now have a cuspidal edge: two faces tangent along an
  edge with the material between them. No rule says whether such a
  result is wanted or is to be refused.
- OP-3. The audit of refusals (A9) covers boxes, rotated boxes and
  upright cylinders. It does not cover oblique cylinders or the
  acceptance rows.
- OP-4. The recorded failures of the rows that are slow on the default
  bridge, other than T1-041, have not been read. Under A4 they count as
  contract failures without having been classified.
- OP-5. A touching hit where two traces cross. Where equal cylinders
  whose axes meet cross along two ellipses, the ellipses meet at two
  points at which the cylinders are tangent to each other. An edge of
  one operand through such a point is tangent to the other cylinder
  there; X5 records a touching contact and N3 refuses the result,
  although the operands cross at that point. Sections 7.3 and 7.9 have
  no rule for it. Nine Tier 2 rows. Closed on 2026-11-19: with the row
  of section 7.4 in place on the rules as they stand since RL-23, no
  Tier 2 row meets it, and all seventeen give their values.
- OP-6. The overlap volume of a pinched common (CO2). The common of a
  stock with one bore and the tool of a crossing bore is two wedges that
  meet only at two points. N2 refuses it, correctly, and the record gives
  a volume. Eight Tier 2 rows. Closed on 2026-11-19: CO2 now answers
  with the volume the cut removes.
- OP-7. Section 7.5 has no entry for the conic and cylinder of one Tier 2
  row, T2-015. This one is an addition to the table. Closed on
  2026-11-19: the table has the entry.
- OP-8. Settled by OD-9: T1-092 and T1-029 are accepted as they stand,
  as exceptions to PC1. The work on the mesher that they need is not
  part of the trial.
- OP-9. The rounding allowance of T9 follows the size of the bounding
  box, while rounding grows with the distance of the model from the
  origin. A small model far from the origin is not covered.
