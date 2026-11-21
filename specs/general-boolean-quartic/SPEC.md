# Quartic pairs in the general Boolean

Status: written on 2026-11-20 against `main` at `d6dcbcfe`, after the
owner asked for the skew and unequal-radius crossings to be started.
Section 9 lists the decisions this document takes by default for the
owner to confirm or change. Section 8 is the plan and records what is
implemented.

## 1. Purpose

`brep/boolean` combines solids bounded by planes and circular cylinders
([the general Boolean specification](../general-boolean/SPEC.md), the
*base specification*). Of two cylinder faces whose axes are not parallel
it resolves the equal pair
([the crossing specification](../general-boolean-crossing/SPEC.md)) and
refuses every other pair that it cannot prove separate (OD-2, SP3). The
bridge cuts plain bores with the general Boolean first (OD-11), so a
bore of another radius, or one whose axis passes the first, across a
bored body is cut by the legacy bore engine from the body's history
(`legacyStock`). The legacy unequal and skew families cannot be deleted
while that is so.

This document specifies the support of those pairs in `brep/boolean`:
a curve carrier for the curve in which two cylinders meet, its rows in
the two tables, its image in a chart, its share of the measures, and the
one change `brep/trimmed` needs to locate a point against a face trimmed
by it. The bridge and STEP follow in later phases (section 8) and are
specified here only as far as their acceptance.

Conventions are those of the crossing specification: "must" states a
requirement; ε is the linear tolerance of a call and μ its filter
margin; identifiers without a document name are the base
specification's; *equal pair* and *quartic pair* are the crossing
specification's terms.

## 2. Terms

For two cylinders whose axes are not parallel, the *small* one is the
one of smaller radius r, and the first of the pair when the radii are
equal; the other is the *large* one, of radius R. P and Q are the points
of the small and the large axis nearest each other, n is the unit
vector along z_large × z_small, d = (P − Q)·n is the signed distance
between the axes, and s > 0 and c are the sine and cosine of the angle
between the axis directions. θ is the angle of a point about the small
axis and ψ its angle about the large axis, both measured from n, each
counterclockwise about its own axis direction.

- **Meeting curve**: the set of points on both cylinders. A point of the
  small cylinder at angle θ and height v above P along its axis is on
  the large one exactly when

      d + r·cos θ = R·cos ψ    and    v = (r·c·sin θ − R·sin ψ) / s.

- **Full pair**: |d| + r < R. Every ruling of the small cylinder passes
  through the large one, and the meeting curve is two closed curves, one
  with sin ψ > 0 and one with sin ψ < 0.
- **Partial pair**: R − r < |d| < R + r. The meeting curve is one closed
  curve, which goes round neither axis.
- **Regular**: a pair is regular when the sine of each angle at which
  the meeting curve turns back is at least 1/64. For a full pair that is
  1 − ((|d| + r)/R)² ≥ 1/64². For a partial pair it is
  1 − ((|d| − R)/r)² ≥ 1/64² and 1 − ((|d| − r)/R)² ≥ 1/64². A pair
  that is not regular has cylinders that touch, or nearly touch, from
  inside or from outside.

## 3. Scope

### 3.1 In scope

- S1. A quartic pair that is full or partial, regular, and whose axes
  make an angle of sine at least 1/64, between any two cylinder faces of
  operands of the base specification's class (section 4).
- S2. An operand that has an edge on a meeting curve, as a result of an
  earlier call has. Such an edge may then meet any face or edge of the
  other operand that the tables resolve.
- S3. `trimmed.Solid.Classify` for a solid with a face trimmed along a
  meeting curve (section 5).

### 3.2 Out of scope

- N1. A quartic pair that is not regular, or whose axes make an angle of
  sine below 1/64. It is refused as today unless SP2 proves the faces
  separate (R1).
- N2. Any curve other than the meeting curve of two circular cylinders.
  Cones, spheres and tori stay outside the class.
- N3. Native persistence and IGES of a result with such an edge.
- N4. A proof of any result. Results are judged by validation V1 to V8
  within tolerance, as the base specification states.

## 4. Requirements on `brep/boolean`

### 4.1 The carrier

- C1. The package must have one curve carrier for a meeting curve. It is
  closed, with period 2π. Its parameter t is θ for a curve of a full
  pair. For the curve of a partial pair it is the polar angle of the
  point (x, y) about the origin, each coordinate divided by its greatest
  value on the curve, where x and y are θ and ψ counted from the middle
  of their ranges; along every ray from that origin
  r·cos x + R·cos y falls as the point moves out, so the ray meets the
  curve once.
- C2. The carrier must give, at every parameter, a point whose distance
  from each of the two cylinders is at most 1e-11·L, where
  L = (R + r)/s + |P| and |P| is the distance of P from the origin of
  the model, and a velocity that is the derivative of that point. The
  velocity of a regular pair is not zero.
- C3. The parameter of the point of the carrier nearest a given point
  must be found to within what moves the carrier point by ε/1024, for a
  point within 4ε of the carrier.
- C4. The carrier reversed (`flipped`) is the same set of points with
  the parameter negated.
- C5. The curve an edge of the result stores evaluates the carrier at
  its parameter and gives its velocity as derivative. Its interval is
  the interval of the edge on the carrier. Intake derives the carrier of
  such an edge from the carriers of its two faces by the row of T1 (I5),
  and does not inspect the stored curve's type (S3 of the base
  specification).

### 4.2 The tables

- T1. Surface table (7.4). For a quartic pair in scope the row for two
  cylinders must give the two curves of a full pair, or the one curve of
  a partial pair, each crossing and not touching. For any other quartic
  pair it gives what it gives today.
- T2. Curve table (7.5), a meeting curve with a plane or a cylinder. The
  signed distance f(t) from the carrier point to the surface is sampled
  at 256 equal steps of the period. A hit through the surface is placed,
  by bisection, where f changes sign between a sample above ε and one
  below −ε. A touching hit is placed where |f| comes within ε of zero
  and turns back on the same side; a dip that passes −ε between two
  samples and returns is two hits through. Between two neighbouring
  samples on one side of zero a search is made only where one of them
  is a least value of |f| among its own neighbours; the least value is
  then found by narrowing. Bisection and narrowing stop when the
  interval is no wider than two neighbouring floating-point numbers or
  after 80 steps, and a value is compared with ε as computed. A least
  value about which the function is smooth, rising over a step four
  times what it rises over half the step and not twice, is then placed
  where its slope changes sign (D6). When
  every sample is within ε the curve is reported as one that may lie in
  the surface (CC2's `alike`). There is no upper bound on the number of
  hits.
- T3. Curve table, a meeting curve with a line, a circle, an ellipse or
  another meeting curve. The distance g(t) from the carrier point to the
  other curve is sampled at 256 equal steps of the period, and each
  sample that is a least value among its neighbours is improved by
  narrowing, to the stopping rule of T2. A least value of at most 4ε is a hit, reported with that
  distance as its gap; the passes apply their own bounds to it (X3, X8).
  When every sample of two meeting curves is within ε, the two are
  candidates for an overlap (CC4).
- T4. Sampling at 256 steps is taken to find every hit: along a regular
  meeting curve f and g have at most a few turns per period. This is a
  judgment within tolerance (N4), and V5 and V6 check each result.

### 4.3 Charts and measures

- H1. A chart path must be able to carry a leg whose image has no form
  base + lin·t + cos·cos t + sin·sin t: it then gives its chart point
  and its chart velocity at a parameter by evaluation. Every chart
  operation of the base specification (laying out loops, region tests,
  turning order at a node, area, flux) must work on such a path.
- H2. In the chart of a cylinder, the first coordinate of the image of a
  meeting curve must be continuous along the leg: it advances by one
  period of the chart round a curve of a full pair on the small
  cylinder, and returns to its start on the large one and on both
  cylinders of a partial pair.
- H3. A region test cuts a path where its first coordinate turns back.
  For a path of H1 those places are found from the sign of the first
  component of the velocity, sampled at no more than π/32 of the
  parameter and placed by bisection.
- M1. The area, the volume share and the first moment of a face bounded
  in part by a meeting curve must be computed by this package. Along
  such a leg the boundary integrals of V5 are taken by five-point Gauss
  quadrature on parts of at most π/32 of the parameter; along every
  other leg they keep their closed forms. The statement of the base
  specification that nothing in the measures is a quadrature is amended
  accordingly (section 7).
- M2. The volume of a result so measured must agree with the reference
  of A2 within 1e-9 of the volume of the stock.

### 4.4 Results

- B1. A use along a meeting curve stores a p-curve that evaluates the
  chart image of its leg, mapped to the parameters of the stored
  surface. It evaluates to the declared ends of the use at 0 and at 1,
  to the last bit, and lists the parameters at which either of its
  coordinates turns back (section 5).
- B2. An edge on a meeting curve is cut into arcs of at most a quarter
  of the period, as B2 of the base specification cuts a circle.
- R1. Errors. A quartic pair out of scope that SP2 does not prove
  separate returns `ErrUnsupported` from X6, naming both faces and
  saying which bound it fails: the angle between the axes, or how
  nearly the cylinders touch.

## 5. Requirement on `brep/trimmed`

- P1. `brep/trimmed` must export one interface,

      type MonotoneUV interface {
          PCurve
          TurnsUV() []float64
      }

  a p-curve that lists the parameters strictly between 0 and 1, in
  increasing order, at which either of its coordinates turns back.
  `Classify` and the ray contacts
  it rests on must accept such a p-curve wherever they accept an arc of
  an ellipse: they cut it at the listed parameters into parts monotone
  in both coordinates and apply to each part the crossing rule they
  apply to a part of an arc.
- P2. No other behaviour of `brep/trimmed` changes, and no file of it
  that starts with `//go:build research` is touched.

## 6. Acceptance criteria

- A1. Tests of the carrier: at 64 parameters of each curve, for a full
  pair at a right angle, a full pair with d ≠ 0 at a slant, a partial
  pair and a partial pair of equal radii, the point is on both
  cylinders (C2), the velocity agrees with a central difference of the
  point within 1e-6 of its length, and `param` returns the parameter
  (C3).
- A2. Reference volume. The solid common to two cylinders has volume

      (4 / s) · ∫ √(R² − y²) · √(r² − (y − d)²) dy

  over the y at which both roots are real, because its section at
  height y along n is a parallelogram with those two half-widths. Tests
  compute the integral by a quadrature of their own and hold `Combine`
  to it (M2) for: a box with a bore of radius 5 cut by a cylinder of
  radius 2 at a right angle with d = 0 (full); the same at a sine of
  0.6 with d = 1 (full, skew); radii 5 and 3 with d = 4 (partial); and
  radii 3 and 3 with d = 2 (partial, equal radii). Each is run as a
  cut of the second bore from the bored stock, and the first two also
  as the common of the two cylinders.
- A3. A result of A2 is cut again by a plane-faced tool and by a third
  cylinder parallel to the first, each chosen to pass through a meeting
  curve, and the volume holds to the same bound against the sum of its
  parts (S2).
- A4. Tests of this package hold a pair that touches from inside, one
  that nearly does from either side of that, one that touches from
  outside and one at a sine below 1/64 to `ErrUnsupported` with the
  reason of R1. The full and partial pairs are those of A2.
- A5. `trimmed.Solid.Classify` gives inside, outside and boundary at
  chosen points of a result of A2, including points whose rays cross a
  face trimmed by a meeting curve.
- A6. Every test added finishes in under two seconds or starts with
  `slowtest.Skip(t)`. `go test ./...` passes, and so does
  `go build -tags research ./... && go vet -tags research ./...`.
- A7. Phase 2: the five bridge tests of the legacy unequal and skew
  families (`TestSkewCrossedBoreScopeAndSTEP`,
  `TestUnequalCrossedBoreScopeAndSTL`,
  `TestPartialSkewBoreScopeSTEPAndSTL`,
  `TestFiniteUnequalCrossedBoreScopeAndSTEP`,
  `TestFinitePartialSkewBoreScopeAndSTEP`) pass when the default bridge
  cuts them with the general Boolean, with volumes and areas held to
  the values they held the legacy path to. Their bodies were meshed and
  written to STEP from the bore history (D1) until phase 3.
- A8. Phase 3, mesh: such a body is meshed as a closed solid within the
  bridge's volume budget with no use of its history: as cut, cut again
  by a tool that is not a bore, and mirrored.
- A9. Phase 3, STEP: such a body exports to STEP by the writers of the
  general path and reads back to the same volume and face count, with
  no use of its history. Section 10 states the form and the tests
  (A10 to A13).

## 7. Amendments to the base and crossing specifications

Made in the phase that implements each:

- OD-2 is reversed for the pairs of S1, by the owner's instruction of
  2026-11-20; it stands for the pairs of N1.
- SP3 and the row for two cylinders in section 7.4: as T1.
- Section 7.5 gains the rows of T2 and T3.
- The paragraphs on the closed forms of V5 (section 12, RL-17): as M1.
- N1 and section 12 of the crossing specification: quartic pairs in
  scope here are no longer refused.
- A new ruling in section 14 cites this document.

## 8. Plan

One branch, `codex/quartic-crossings`. Each phase is committed when its
own tests pass; the slow checks of A6 run once before the branch merges.

1. Kernel: the carrier (4.1), the tables (4.2), charts and measures
   (4.3), results (4.4), `brep/trimmed` (5). Done when A1 to A6 pass.
   Implemented on 2026-11-20.
2. Bridge: the default bridge cuts the pairs of S1 with the general
   Boolean and keeps `legacyStock` for what R1 refuses. Done when A7
   passes and the bridge's default tier passes. Implemented on
   2026-11-20, with D1.
3. Mesh and STEP: a mesher that finishes a face trimmed along a meeting
   curve (D3), done on 2026-11-20 with A8; and a STEP writer and reader
   for an edge on one (section 10), implemented on 2026-11-20 on the
   branch `codex/meeting-step` with A9 to A13, and with D4 to D6.

What phase 2 found:

- D1. `trimmed.Solid.Tessellate` does not finish a body with a face
  trimmed along a meeting curve. It refines every face evenly; on such
  a face the first refinement folds where the curve runs along the
  chart direction, and the retries that hold the boundary fixed exceed
  the limit of a million triangles before the wall is fine enough.
  Cutting the curve into shorter arcs does not change that. The STEP
  boundary writer has no entity for the curve. So a drilled body, which
  has a bore history, is meshed and written to STEP from that history by
  the legacy bore engine, as it was before this work, and reads back as
  a legacy bore body. A body with a meeting curve and no history, such
  as one cut again by a tool that is not a bore, can be measured,
  located against and cut further, and returns an explicit error when a
  mesh or a STEP file is asked of it. The legacy unequal and skew
  families therefore cannot be deleted until phase 3.
- D3. The mesher is a new package, `brep/chartmesh`, and not a change
  to `brep/trimmed`. It cuts every edge once into equal steps of its
  parameter, so that both faces of an edge take the same points, and
  meshes each face in its own chart: a plane on its boundary points
  alone, a cylinder with inner sides halved where a triangle spans too
  wide an angle about the axis or its normal leaves the surface normal
  by more than the facet angle. Triangles are exchanged by the rule of
  Delaunay in a chart squashed along the axis, since on a cylinder a
  facet's error grows with how far it goes round the axis and not
  along it; without that a tall wall with a hole was meshed in fifty
  times the triangles. The bridge uses it for a general result with a
  face trimmed along a meeting curve, and asks a finer facet angle of
  it than of the mesher of `brep/trimmed` to reach the same budget.
  With it the mesh of D1 no longer comes from the bore history, and a
  body without one is meshed. The STEP file of D1 still does.
- D4. The boundary reader took the height of every vertex on a
  cylinder from that vertex. Under a frame off the coordinate axes the
  vertices along one rim then differed in the last bits, the corners of
  the rim were not in line in the chart, and `trimmed.NewSolid` refused
  one of the eight bodies of A11 for a fold in its validation mesh. The
  reader now gives the vertices along a section across the axis the
  height of that section's centre, as the Boolean's own charts do.
- D5. The pinned OCCT build read the eight files of A11 through the
  project's helper `tools/occtref/read_split_cylinder.cxx`: each is a
  valid solid with the face count written and a volume within 1e-8 of
  the kernel's, relative. This check is not automated.
- D6. A cut that removes nothing returned a body the general Boolean
  did not take as an operand again. Where the axis of one bore lies on
  the wall of the other and the axes are along the coordinate axes
  (`TestPartialSkewBoreScopeSTEPAndSTL`), the meeting curve turns back
  about an axis exactly on a seam of that cylinder. Cutting the tool
  of that bore again, as `cutMany` with a repeated tool does, meets
  the curve with the seam of the tool, which touches it there. T3
  placed that touch by the values of the distance alone, which about a
  smooth least value tell places apart only to the square root of
  their rounding: 1e-7 along the seam. The vertex, the mean of its
  members, moved by 4.8e-8; from it the curve leaves at an angle to
  the seam above the angular tolerance and, to the walk of the merge,
  on the wrong side of it, and intake ended with "two walks leave
  point … along one element". The legacy fallback of the bridge had
  hidden this since phase 2. `leastOf` now places a smooth least value
  where the slope changes sign, to about 1e-11, and leaves a least
  value at a corner, which its values place to the last bits, as it
  was (`TestLeastOf`, `TestQuarticTurnOnSeam`). The limit that
  remains is of input, not of results: a body from elsewhere whose
  vertex at such a touch is off it along the seam by more than the
  angular tolerance allows is refused with the same error.
- D2. Bridge tests that held a refusal of these pairs now hold the
  volume their common gives, and rule X11 of the edge-treatment
  specification lost its example of a bore across a fillet, which X8
  now refuses.

Budget. `brep/boolean` grows by at most 900 lines outside test files,
and `brep/trimmed` by at most 80. A phase that needs more stops and is
reported.

Stop conditions. Work stops and is reported when a test of A2 or A3
gives a wrong volume in a result that passed validation, or when a
requirement needs a rule for one named arrangement of two cylinders
beyond the two cases of section 2.

## 9. Decisions taken by default

Each is the owner's to change.

- OD-Q1. The bound of regularity and the least angle are both a sine of
  1/64, the bound E4 already uses.
- OD-Q2. Hits are found by sampling at 256 steps and not by a closed
  form or a proof (T4).
- OD-Q3. A face bounded by a meeting curve is measured by quadrature
  along that leg (M1).
- OD-Q4. `brep/trimmed` gains one exported interface (P1), which is the
  smallest change that lets `Classify` take the new p-curve.
- OD-Q5. STEP by the general path comes last (phase 3). Until then a
  drilled body is written from its bore history (D1). Section 10 ends
  that.
- OD-Q6. The mesher is its own package and is used only for bodies with
  a meeting curve (D3). It would mesh any result of the general Boolean
  without a pole; whether it should replace the mesher of `brep/trimmed`
  for those is not decided here.
- OD-Q7. The edge is written as an `INTERSECTION_CURVE` on its two
  cylinders with a cubic B-spline in space (E2). The two surfaces fix
  the curve, so the reader of this kernel recovers it exactly, and a
  reader that knows only the spline has it to within half the tolerance
  of the solid.
- OD-Q8. The reader takes the curve from the two surfaces and uses the
  spline only to choose among the curves of the pair and their
  direction (E3). A file whose spline is farther from every such curve
  than the tolerance of the read is refused, as a vertex off its curve
  is.
- OD-Q9. The geometry of the carrier moves to a package of its own,
  `brep/meeting` (E1), so that `exchange/step` does not import
  `brep/boolean`.

## 10. STEP for an edge on a meeting curve

Phase 3 of section 8. `step` is the package `exchange/step`; its
*boundary writer* is `step.WriteTrimmedBoundary` and its *boundary
reader* `step.ReadTrimmedBoundary`.

### 10.1 Requirements

- E1. A new package `brep/meeting` holds the geometry of section 4.1
  with no dependency on `brep/boolean`: a type `Curve` that is a
  `geom.Curve` by the parameter of C2; `Between`, which gives the
  curves of two cylinders or the reason of R1 for which it gives none;
  `Param`, the parameter of a point; `Turn`, the continuous angle of H2
  about the axis of a frame; `Reversed`; and `Trim`, the p-curve of an
  arc of a `Curve` in the angle and height of one of its cylinders,
  which satisfies `trimmed.MonotoneUV` (P1). The carrier of
  `brep/boolean` must delegate to it, and an edge of a result must
  store a `meeting.Curve`. Nothing else in the behaviour of
  `brep/boolean` changes: the tests of A1 to A5 pass as they are.
- E2. The boundary writer must write an edge whose stored curve is
  neither a line nor an ellipse, and whose two uses are on faces of two
  cylinders, as an `EDGE_CURVE` on
  `INTERSECTION_CURVE('',c,(s1,s2),.CURVE_3D.)`. s1 and s2 are the
  `CYLINDRICAL_SURFACE` entities of the two faces. c is a
  `B_SPLINE_CURVE_WITH_KNOTS` of degree 3 that follows the edge over
  its parameter interval, cut into equal spans, and has the point and
  the direction of the stored curve at the ends of every span; its
  knots are the whole numbers from zero, double between the ends. The
  spans are doubled,
  from four, until at the quarter points of every span the spline is
  within half the tolerance of the solid of the stored curve at the
  same parameter. The writer must not inspect the Go type of the
  stored curve. It must return an error that wraps
  `step.ErrTrimmedBoundary`, and write nothing, when a use of such an
  edge is on a plane, when a sample is farther than the writer's
  tolerance from either cylinder, or when 4,096 spans do not reach the
  bound.
- E3. The boundary reader must read an `EDGE_CURVE` on an
  `INTERSECTION_CURVE`. Where its curve in space is a `LINE`, `CIRCLE`
  or `ELLIPSE` the edge is read as that curve. Otherwise the curve in
  space must decode as a B-spline and the two associated items must be
  two `CYLINDRICAL_SURFACE` entities, or `PCURVE` entities on them;
  the edge then stores the curve of `meeting.Between` for the two
  cylinders from which 17 evenly spaced points of the spline are all
  within the tolerance of the read, run in the direction in which
  those points advance along the edge, over the interval from the
  parameter of its start vertex to that of its end vertex. A use of
  the edge on one of the two cylinders gets a `meeting.Trim`. The
  reader must return an error that wraps `step.ErrTrimmedBoundary` for
  a pair `Between` gives no curve for (with its reason), for an arc of
  more than half a period (as it does for an arc of a conic), and for
  a use on a plane or on a cylinder the curve does not lie on; and one
  that wraps `geom.ErrInvalid` when no curve of the pair is within
  tolerance of the spline.
- E4. `writeGeneralSTEPBody` of the bridge must no longer write a
  general result from its bore history. A body of A7 or A8 is written
  by the boundary writer and read back by `importGeneralSTEP`.

### 10.2 Acceptance criteria

- A10. `brep/meeting`: for the four pairs of A1, every curve of
  `Between` has 64 points within 1e-12 of both cylinders, `Param`
  returns the parameter of each within 1e-9, and a `Trim` of each
  quarter of it on each cylinder evaluates, lifted to the cylinder, to
  within 1e-12 of the curve, with no turn of either coordinate between
  two neighbours of its `TurnsUV`.
- A11. `step`: each of the four pairs of A1 cut as two bores from a
  prism, as placed and turned off the coordinate axes, written by the
  boundary writer and read by the boundary reader at the linear
  tolerance of the Boolean, has the same counts of faces, edges and
  vertices, and united with the empty solid by the general Boolean has
  a volume within 1e-9 of the stock volume of the body written; what
  was read is written and read again to the same.
- A12. `step`: a file of A11 edited so that (a) every pole of one
  spline is moved by a thousand times the tolerance, or (b) an
  `INTERSECTION_CURVE` names one surface, is refused with the error E3
  names for it; and (c) the writer refuses a solid with an edge of an
  unknown kind between two plane faces. The reader samples a spline at
  17 points, so it does not see a change to one pole of many.
- A13. Bridge: the bodies of A7 and A8 export to STEP with an
  `INTERSECTION_CURVE` in the file, the export's read-back agrees with
  the body in volume within the bridge's export tolerance, and the
  file imports as a general result of the same face count. A file
  re-exported from what was read is not held to the bytes of the
  first.
- A14. `brep/boolean`: the two bodies of D6, cut again by the tool of
  their first bore, keep every vertex within 1e-9 and are taken as an
  operand again; a smooth least value is placed within 1e-9 and one at
  a corner within 1e-14 (D6).

### 10.3 Limits

`exchange/step` grows by at most 300 lines outside test files and
`brep/boolean` does not grow. Work stops and is reported when a body of
A8 written by E2 does not read back by E3 within tolerance.
