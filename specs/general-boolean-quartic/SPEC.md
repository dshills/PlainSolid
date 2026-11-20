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
  after 80 steps, and a value is compared with ε as computed. When
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
  the values they held the legacy path to. Their bodies are meshed and
  written to STEP from the bore history (D1).
- A8. Phase 3, mesh: such a body is meshed as a closed solid within the
  bridge's volume budget with no use of its history: as cut, cut again
  by a tool that is not a bore, and mirrored.
- A9. Phase 3, STEP: such a body exports to STEP by the writers of the
  general path and reads back to the same volume and face count, with
  no use of its history.

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
   for an edge on one, done when A9 passes and not started.

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
  drilled body is written from its bore history (D1).
- OD-Q6. The mesher is its own package and is used only for bodies with
  a meeting curve (D3). It would mesh any result of the general Boolean
  without a pole; whether it should replace the mesher of `brep/trimmed`
  for those is not decided here.
