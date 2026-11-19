# Crossing cylinders in the general Boolean

Status: written on 2026-11-19 against `main` at `d4a157a6`. The owner
decided OD-C1 to OD-C7 on 2026-11-19, each as this document recommended
(section 11). On the same day, when two rows with concealed cutters
still failed after section 6.3, the owner chose to have them worked on
(OD-C8); sections 6.4 and 6.7 are written to what that work built.
[The plan](PLAN.md) records what is implemented. Section 3.4 records the
experiment the design rests on.

## 1. Purpose

`brep/boolean` combines solids bounded by planes and circular cylinders
([the general Boolean specification](../general-boolean/SPEC.md), called
the *base specification* here). It refuses every pair of cylinder faces whose
axes are not parallel and that it has not proved separate. The
refusal is `ErrUnsupported`, "X6: face i of A and face j of B are cylinders
with axes that are not parallel" (`brep/boolean/pass_face.go:53`). SP2 of
the base specification is that proof of separation.

The bridge's legacy bore paths cut such bores. A probe on 2026-11-19 made
`internal/plaincadbridge` cut plain bores with the general Boolean first.
A bored box then answered as on the legacy path except that a second bore
crossing the first was refused, and nine bridge tests failed for that
reason. The owner's plan is to cut the plain bore families with the
general Boolean and then delete the legacy families it replaces, so that
plan cannot proceed while a crossing bore is refused.

This document specifies the support of one class of crossing cylinders,
those of equal radius whose axes meet, and of three things the bridge
needs around it. It states what stays out, and why.

Conventions. Requirements carry an identifier made of a letter prefix and
a number; the prefix only groups them by section. "Must" states a
requirement. ε is the linear tolerance of a call and μ its filter margin,
as the base specification defines them (T2, T4); with the bridge's options
ε is 1e-7. Identifiers without a document name, such as X7, CO2 or OP-6,
are those of the base specification.

## 2. Terms

- **Crossing pair**: two cylinder faces whose axes are not parallel (T5)
  and that SP2 does not prove separate.
- **Equal pair**: a crossing pair for which |r₁ − r₂| + |PQ| ≤ ε, where
  r₁ and r₂ are the radii and P and Q the points of the two axes nearest
  each other. Its surfaces meet, within ε, along two ellipses.
- **Quartic pair**: any other crossing pair. Its surfaces meet along a
  curve that is not a line, a circle or an ellipse.
- **Tangent node**: one of the two points at which the two ellipses of an
  equal pair meet each other. The two cylinders have the same tangent
  plane there.
- **Pinched common**: a common of two solids whose parts meet each other
  only at isolated points.
- **Tier 2 row**: one of the 17 acceptance rows T2-001 to T2-017 of
  [the acceptance record](../general-boolean/ACCEPTANCE.md), each the
  frozen trace of one bridge test.

## 3. What exists

### 3.1 The base specification

- The surface table of section 7.4 has a row for the equal pair: "Two
  ellipses, crossing, meeting each other at two nodes (X7)". The row is
  marked "Tier 2 only (OD-1)" and returns `ErrUnsupported` until Tier 2 is
  supported (SP3, 15.5). Every other crossing pair returns
  `ErrUnsupported`.
- OD-1 put Tier 2 in the trial as optional. OD-2 put quartic pairs out.
  RL-22 records that Tier 2 was attempted and not supported, and OD-9
  left it so.
- Three open items name what the attempt met: OP-5, a touching hit of an
  edge at a tangent node, which N3 refused; OP-6, the overlap volume of a
  pinched common, which N2 refuses; OP-7, a conic and a cylinder for which
  the curve table of section 7.5 has no entry.
- X7 is stated and implemented (`arrangement.meetings`,
  `brep/boolean/pass_face.go:75`): curves of one surface pair that meet
  each other get nodes where they meet.

### 3.2 The earlier attempt

`codex/general-boolean-tier2-attempt` (`cc7a679e`, 2026-11-18, not
merged) added the equal pair as one row of the surface table: the two
ellipses are the sections of one cylinder by the two planes through the
point where the axes meet, with normals z₁ − z₂ and z₁ + z₂, where z₁ and
z₂ are the unit axis directions. It adds no curve kind. It is 31 lines in
`surface_table.go` and `closed_forms.go`. With it and X7, 1 of the 17
Tier 2 rows passed. The commit message attributes the rest to OP-5 (nine
rows), OP-6 (eight), OP-7 (one), limits of `brep/trimmed` (eleven
responses) and time (one).

### 3.3 What the bridge serves today

The legacy paths cut crossing bores only in a box, through recipes of
`brep/trimmed` that each take one arrangement:

- Equal radius, axes meeting at a right angle, each along a coordinate
  axis: through, blind, double blind, with a floor inside the crossing,
  with disjoint companions, and three concurrent bores.
- Equal radius, one axis along a coordinate axis and one oblique in a
  coordinate plane, with the direction ratios 4:3, 3:4, 12:5 and 5:12
  only.
- Unequal radius or skew axes, at a right angle only, in five test files
  that the base specification excludes (15.2).

Their measures are closed forms for equal radius and numerical
integration otherwise. Their STEP files hold the crossing curve as
`ELLIPSE` for equal radius and as a cubic B-spline of stated accuracy
1e-7 mm otherwise.

Of the nine bridge tests that failed in the probe, eight are Tier 2 rows:
T2-001, 002, 003, 006, 009, 010, 011 and 017. The ninth,
`TestSkewCrossedBoreScopeAndSTEP`, is a quartic pair: radii 5 and 2, axes
at a right angle and 1 apart.

### 3.4 The experiment of 2026-11-19

On a scratch copy of `main` at `d4a157a6`, not kept, the two files of the
earlier attempt's surface row were applied unchanged and the 17 Tier 2
rows were replayed on a general bridge
(`TestGeneralBooleanAcceptance/T2` with `PLAINSOLID_SLOW=1`).

With the surface row alone:

- Every checked response that gives a value agrees with its record. The
  largest errors over all rows, in the measure of A8, are 1.5e-14 for
  volume and 6.5e-16 for area.
- No response fails for a touching hit at a tangent node, and none for a
  limit of `brep/trimmed`. The rules and the mesher have changed since the
  earlier attempt (RL-23, and `948b2cec`), and OP-5 no longer arises in
  these rows.
- Four rows pass: T2-006, 009, 012 and 016.
- Eight rows fail only at `overlapVolume`, with N2 "the kept faces at … do
  not form one cycle": T2-001, 002, 003, 005, 010, 011, 014 and 017. This
  is OP-6.
- T2-004 fails at X6: a `cutMany` whose tools include a small bore along
  one axis and a larger bore across it, in an order that cuts the small
  one first. The pair is a quartic pair in that order and no pair at all
  in another, since a later tool covers the small bore.
- T2-015 fails at X5, "edge … is a conic that the table cannot meet with
  the cylinder of face …". This is OP-7.

With a second scratch change, by which `overlapVolume` falls back to the
volume a cut removes when the common is refused as not a manifold
(section 6.2):

- Ten rows pass: T2-001, 002, 003, 006, 009, 010, 011, 012, 016 and 017.
  These include all eight Tier 2 rows of the probe.
- T2-005 and T2-008 give every value and exceed two seconds only when the
  rows run side by side; run alone they take 1.36 s and 1.51 s.
- T2-007 and T2-013 give every value and take 2.13 s and 4.42 s run
  alone. T2-013 has 160 checked responses.
- T2-014 gives every value, and answers two requests that the record
  refuses: a blind bore whose floor lies inside the crossing.
- T2-004 and T2-015 fail as above.

A crossed pair at a right angle, one at an angle whose sine is 0.8, and
three concurrent bores were then cut on a general bridge and used:

| | Right angle | Oblique | Triple |
|---|---|---|---|
| Time for the cuts | 0.04 s | 0.02 s | 0.05 s |
| Volume against the closed form, absolute | 1.8e-12 | 1.8e-12 | 1.5e-11 |
| Mesh: valid, triangles, volume error | yes, 1,556, 4.5e-5 | yes, 1,556, 2.5e-5 | yes, 2,328, 2.4e-5 |
| STEP export, and import of the file | both succeed | both succeed | both succeed |
| Chamfer of an outer edge | succeeds | succeeds | succeeds |

The drawing of a crossed pair has no bore callout and one warning, on the
general path as on the legacy path. Display edges, `holeRim`, `mesh` and
`stl` answer on both.

The unequal pair of `TestSkewCrossedBoreScopeAndSTEP` is refused at X6, as
expected.

## 4. Scope

### 4.1 In scope

- S1. The equal pair, for any two cylinder faces of operands of the base
  specification's class, at any angle between the axes that E4 admits.
  The stock need not be a box and neither axis need lie along a
  coordinate axis.
- S2. `overlapVolume` where the common is a pinched common (section 6.2).
- S3. `cutMany` whose tools give a quartic pair in the order stated and
  none in another (section 6.3).
- S4. The meeting of a circle or an ellipse with a cylinder where the
  curve table has no entry (section 6.4).
- S5. A proof that a face of one cylinder is clear of the whole of
  another cylinder (section 6.7).

### 4.2 Out of scope

- N1. Quartic pairs. A call that meets one returns `ErrUnsupported`, as
  today (R1). OD-2 stands.
- N2. Any change to `brep/trimmed`, `geom` or `exchange/step`, and any
  change to what `brep/blend` does. One test of `brep/blend` and one
  sentence of its package comment said that two fillets meeting at a
  corner are refused, because the Boolean took no such pair; it now
  does, and the test holds the result to its closed form instead. The
  bridge still refuses a fillet round a cap on a general result (OD-E6
  of [the edge treatment specification](../general-edge-treatment/SPEC.md)),
  which is that document's to change.
- N3. The drawing callout of a crossed bore. The drawing warns and omits
  the callout, as the legacy path does (section 3.4).
- N4. Making the general Boolean the bridge's first path for any family.
  The default bridge keeps its legacy paths first (D3).

Why quartic pairs stay out. An equal pair adds no kind of curve: its
edges are arcs of ellipses, which every stage of the Boolean and every
consumer of its result already carries. A quartic pair needs a new kind
of curve in all of these:

- the curve carrier of `brep/boolean`, whose interface is the form
  base + lin·t + cos·cos t + sin·sin t (`carrier.go:39`), and the chart
  path of the same form that every chart operation is written against;
- a numerical intersection of that curve with lines, conics, planes and
  cylinders, where the package today has closed forms only;
- a p-curve kind that `trimmed.Solid.Classify` accepts, which today takes
  only arcs of the kinds `brep/trimmed` itself defines
  (`location.go:359`);
- measures by numerical integration, since the closed forms of V5 cover
  rulings and arcs of section circles and ellipses only;
- a STEP writer and reader for a curve that STEP has no exact entity for.

This document judges that to be several times the work of everything in
section 6 together, and it is needed by one of the nine tests of the
probe. Section 12 states what leaving it out means for the owner's plan.

## 5. Dependencies that do not change

- U1. X7 is used as it stands.
- U2. Sections 7.2, 7.3 and 7.6 to 7.11 of the base specification are
  used as they stand. The experiment found no row of Tier 2 that needs a
  change to them (section 3.4). If the implementation finds one, that is
  stop condition ST-C2.
- U3. Validation V1 to V8 applies to every result unchanged.

## 6. Requirements

### 6.1 The equal pair

- E1. The surface table must give, for an equal pair, two ellipses,
  crossing. With M the midpoint of P and Q, the ellipses are the sections
  of the first cylinder of the pair by the two planes through M whose
  normals are z₁ − z₂ and z₁ + z₂. Each section is computed by the
  existing row for a plane and a cylinder. The condition of the row is
  that of section 2, |r₁ − r₂| + |PQ| ≤ ε, which is narrower than the
  base specification's row (radii within ε and axes within ε); a pair
  that meets the wider condition and not the narrower is a quartic pair
  (R1). If the existing row gives anything other than one ellipse for
  either plane, the row of the equal pair returns `ErrUnsupported` naming
  both faces; with E4 this arises only for a value of `Angular` so large
  that T5 calls a bisecting plane parallel to an axis.
- E2. The row adds no curve kind, no carrier and no chart operation.
- E3. Each point of either ellipse lies on the first cylinder and within
  |r₁ − r₂| + |PQ| of the second. The line PQ is perpendicular to both
  axes, so P and Q lie in both planes; the mirror in either plane
  therefore maps the first axis to the line through P that has the
  direction of the second axis or its opposite, and that line is |PQ|
  from the second axis. With the condition of E1 that distance is at
  most ε, so each point of either ellipse is on both cylinders in the
  sense of T3. The implementation must not rely on a smaller bound.
- E4. An equal pair is admitted when the sine of the angle between its
  axes is at least 1/64 (about 0.9 degrees). Below that the row returns
  `ErrUnsupported` naming both faces, unless SP2 proves the faces
  separate. The legacy paths admit sines of 1, 0.8, 0.6, 12/13 and 5/13
  only.
- E5. SP3 changes accordingly: non-parallel cylinders of equal radius
  whose axes meet are in, within E4. The other two cases of SP3 stay out.

### 6.2 The overlap volume of a pinched common

- P1. CO2 changes. `overlapVolume(a, b)` is V(a ∩ b). Where `Combine`
  refuses the common with `ErrNonManifold`, the bridge computes the cut
  a − b with `Combine` and answers V(a) − V(a − b), which is the same
  quantity for any two solids. V(a) is the volume the call reports for
  its first operand.
- P2. Where the cut fails as well, the request fails with the error of
  the common, unchanged, unless the cut failed with a context error; a
  context error is returned as it is, as section 9 of the base
  specification requires at every stage.
- P3. The fallback is taken for `ErrNonManifold` only. A common that is
  refused for another reason fails as today.
- P4. The answer of the fallback is the difference of two volumes that
  `Combine` reports, and it is as accurate as they are. This document
  gives no further bound for it. Where the overlap is much smaller than
  a, the difference loses accuracy in proportion, and that is a stated
  limit of the fallback, not an error. The rows of A-C1 must meet A3.

### 6.3 The order of tools in `cutMany`

- F1. CO1 changes for `cutMany`. The tools are applied one after another
  as today. A tool whose step returns `ErrUnsupported` is set aside and
  the fold goes on with the next tool.
- F2. The first pass over the tools is the first round. After it the
  tools set aside are tried again, in their original order, against the
  body as it then stands. A round is one pass over the tools then set
  aside, from the first to the last; a tool that applies in a round
  leaves the set, and the round goes on with the next. Rounds repeat
  until no tool is set aside or a whole round applies none.
- F3. When a round applies none, the request fails with the error that
  the first tool still set aside returned in that round, and nothing is
  published.
- F4. A tool is never dropped. The result, when the request succeeds, is
  the body less every tool. As a set of points that does not depend on
  the order in which the tools were applied. Two orders that both succeed
  need not give the same faces, edges or tolerance; each result passes
  validation, and their volumes and areas agree within the bound of A3.
- F5. Only `ErrUnsupported` sets a tool aside. Any other error fails the
  request at once, as today.
- F6. The rules of CO1 on the whole request are unchanged: it must remove
  material and leave some.
- F7. A request of n tools makes at most n(n + 1)/2 calls of `Combine`:
  rounds over at most n, n − 1, … tools, of which every round but the
  last applies at least one. With the bridge's limit of 32 tools that is
  528.

### 6.4 A conic that the curve table cannot meet with a cylinder

- Q1. The curve table of section 7.5 gets a result where it has
  `ErrUnsupported` today, for a circle or an ellipse with a cylinder in
  the case the table calls "any other": 0 to 4 points with tangency
  flag, conic in the surface, none. The row for two coplanar conics that
  are not homothetic is not changed: no Tier 2 row needs it, and it
  returns `ErrUnsupported` as before.
- Q2. Let d(t) be the distance from the point of the conic at parameter
  t to the cylinder's axis, r the radius, and σ(t) = (d(t)² − r²)/(2r).
  σ is the distance of the point from the cylinder's surface to within
  ε²/(2r) wherever that distance is at most ε, and it is the quantity
  the tolerance is applied to. The parameters of one period at which
  |σ| ≤ ε form a set of intervals. Each hit also carries the distance of
  its point from the surface itself, and it is that distance to which a
  caller applies the coincidence rule T3.
- Q3. Hits. An interval of Q2 with σ > ε before it and σ < −ε after it,
  or the reverse, holds one *crossing hit*, at a parameter in it where σ
  is zero. An interval with σ > ε on both sides, or σ < −ε on both, holds
  one *touching hit*, at the parameter in it where σ is farthest towards
  the other side. So a conic that comes within ε of the cylinder and
  turns back touches it, whether or not it dips through the surface by
  less than ε on the way; these are the meanings CC2 gives the two words.
- Q4. The method must return every hit of Q3, each once, with its
  parameter on the conic, its point, and the distance of the point from
  the cylinder. Where |σ| ≤ ε all the way round, it returns no hit and
  marks the pair as one whose conic lies in the surface within
  tolerance, the mark the row for a conic that projects to a circle
  already returns; the caller then decides on the nodes of the edge.
- Q5. Completeness. 2rσ = d² − r² is a trigonometric polynomial of
  degree two in t, and B, the sum of the absolute values of its two
  first-order coefficients and four times those of its two second-order
  ones, bounds its second derivative: over a part of width h it leaves
  the chord between its end values by at most B·h²/8. The method must
  decide every part of the period on that bound: a part is above the
  tolerance, below it, or within it only where the bound shows it. A
  part it cannot decide is halved. Halving stops at a width of
  w = (π/4)/2³⁰; a part still undecided then has a value within B·w²/8
  of 2rε or of −2rε, and is counted as within the tolerance, which moves
  an end of an interval of Q2 by less than w.
- Q6. Deciding at the tolerance and not at zero is required: at an exact
  tangency the rounding of d² − r² changes sign several times within
  1e-8 of the point, and a method that looks for changes of sign reports
  crossings there that do not exist.

### 6.5 Errors

- R1. A quartic pair that SP2 does not prove separate returns
  `ErrUnsupported` with the message of today, which names both faces.
- R2. An equal pair below the angle of E4 returns `ErrUnsupported` with a
  message that names both faces and says that the axes are too nearly
  parallel.
- R3. No requirement of this document turns a refusal into a result
  without validation: every result passes V1 to V8 (U3).

### 6.6 What a result must still do

- D1. A result with an equal pair must mesh to a valid solid whose
  volume, rounded as an STL rounds it, is within 1e-4 of the solid's, by
  the rule the bridge applies to a general result (`tessellateGeneral`).
- D2. Such a result must be written by `step.WriteTrimmedBoundary` and
  read back by `step.ReadTrimmedBoundary` to the same volume, area and
  bounds. Two values agree here when they differ by at most
  max(1e-7, 1e-8 × the larger magnitude), which is the test the bridge's
  `exportStep` applies to its read-back.
- D3. On the default bridge every response to the frozen traces must stay
  byte for byte what it is at `d4a157a6`, except a response that is a
  legacy refusal there and an answer of the general Boolean after. Each
  such response must be listed, with its legacy message, as A4 requires.
- D4. On the default bridge this work must add no call of the general
  Boolean before a legacy path has declined the request as unsupported.
  A request that a legacy path answers therefore runs the code it runs
  today.

### 6.7 A face clear of the whole of another cylinder

- G1. SP2 of the base specification gains one proof, for a face F of a
  cylinder A and a cylinder B whose axis is not parallel to A's. F is
  *clear* of B when every point of F is farther from B's axis than
  r_B + 2μ, or r_B > 2μ and every point is nearer than r_B − 2μ. Two
  cylinder faces are proved separate when either is clear of the other's
  cylinder.
- G2. The distance from a point of F to B's axis is greatest on an edge
  of F: along A's axis its square is a quadratic that opens upwards. It
  is least either on an edge of F or at one of at most four points of A:
  where the ruling of A through the common perpendicular of the axes, on
  either side, comes nearest to B's axis, and where a ruling of A meets
  B's axis.
- G3. On an edge that is an arc of a circle or an ellipse the square of
  the distance is the trigonometric polynomial of Q5, and the proof
  holds only where the bound of Q5 shows it above (r_B + 2μ)², or below
  (r_B − 2μ)², over the whole arc. On a straight edge it is a quadratic
  in the parameter, least at an end or where it turns and greatest at an
  end.
- G4. F is proved outside B only if, besides its edges, none of the
  points of G2 that is within r_B + 2μ of B's axis lies in F or on its
  rim (X9, with the filter margin as radius).
- G5. The proof has nothing in it that depends on which faces or
  operands it is given, and it never calls two faces separate that come
  within 2μ of each other, which are the two conditions SP2 sets for an
  added proof. It is tried after the two tests of SP2 and before the
  proof by subdivision.
- G6. It is an addition under SP5: it changes which pairs outside the
  table are found to have no contact, adds no carrier curve, and touches
  none of sections 7.2, 7.3 and 7.6 to 7.11.

## 7. Amendments to the base specification

Made in the phase that implements each, not before:

- Section 7.4: the row for the equal pair loses "Tier 2 only (OD-1);
  until Tier 2 is supported (15.5) this row returns `ErrUnsupported` as
  SP3 states" and gains E1 and E4.
- SP3: as E5.
- Section 7.5: the row "Circle or ellipse, cylinder; any other" gives the
  method of 6.4.
- CO1: F1 to F7. CO2: P1 to P4.
- Sections 15.1 and 15.5 and OD-1: Tier 2 is supported, with the rows
  that A-C3 lists as exceptions.
- OP-5: closed; it does not arise (section 3.4). OP-6: closed by P1.
  OP-7: closed by 6.4.
- SP2: the proof of 6.7.
- RL-22 is superseded by a new ruling that cites this document.
- `ACCEPTANCE.md`, `pending.txt` and `legacy_refusals.txt` record the
  outcome of each Tier 2 row.

## 8. Acceptance criteria

- A-C1. Every Tier 2 row passes A3 and A4 on a general bridge, except a
  row that A-C3 lists. A row that passes is removed from
  `internal/plaincadbridge/testdata/general_boolean/pending.txt`.
- A-C2. `brep/boolean` has tests with closed forms, each below the bound
  of A3, for the volume and the area a second bore of radius r removes
  from a stock that has the first, with s the sine of the angle between
  the axes:
  - two through bores: the volume removed is 16r³/(3s) less, and the area
    of the result 16r²/s less, than for the same two bores apart, for
    s = 1, 0.8, 0.6, 5/13, 1/8 and 1/64;
  - three concurrent bores at right angles: the solid common to the three
    cylinders has volume 8(2 − √2)r³;
  - a stock that is not a box and axes of which none lies along a
    coordinate axis, against the same closed form.
- A-C3. Exceptions, each recorded in `pending.txt` with its reason:
  T2-007 and T2-013, for time alone (OD-C5). There is no other.
- A-C4. The requests of T2-014 that the record refuses and the general
  Boolean answers are classified as declined inputs (A4), listed in
  `legacy_refusals.txt`, and added to the oracle run (OD-C7). There are
  four, at lines 12, 26, 39 and 53 of the trace: a cutter that deepens a
  blind bore from inside the stock, and a blind bore whose floor lies
  inside the crossing, each in two placements. OD-C7 was decided for the
  first kind; the second came with section 6.4 and is treated the same.
- A-C5. The oracle run (A6) gains one case for each of the three Tier 2
  families, and each case A-C4 lists. The bounds are those of A6.
- A-C6. The matrix of section 11 of the base specification gains rows
  for: an equal pair at a right angle and at a slant; a pair at the
  bound of E4 and one below it; radii that differ by ε/2 with axes that
  pass ε/4 apart (admitted); radii that differ by ε/2 with axes that pass
  ε apart (a quartic pair by E1, refused); a quartic pair of radii 5 and
  2 (refused); and the three set operations on one equal pair.
- A-C7. Section 6.4 has a test table of an ellipse with a cylinder, each
  expected point in closed form: touching at two points, through at
  four, inside with no hit, touching from outside, within ε/2 of
  touching, clear of touching by 2ε with no hit, through at two points,
  and a circle in a tilted plane through at four. The polynomial of Q5
  has its own table: no zero, zero all the way round, two and four
  zeros, touching from above and from below, within and clear of the
  tolerance, a dip through zero of less than the tolerance, a double
  zero, and zeros on either side of the parameter origin.
- A-C7a. Section 6.7 is tested through section 6.3: three bores, of
  which a later one covers an earlier one, cut in all six orders to one
  closed-form volume.
- A-C8. D1 to D3 are each a test in `internal/plaincadbridge`.
- A-C9. The eight Tier 2 tests of section 3.3 pass when the bridge cuts
  their first bore with the general Boolean. This is checked once, by a
  scratch change that is not kept, and reported.

## 9. Budgets

- B1. `brep/boolean` grows by at most 450 lines outside test files. It
  had 6,012 before this work, against the base specification's limit of
  15,000. The first figure here was 350; sections 6.4 and 6.7 together
  were known to exceed it when the owner chose them (OD-C8).
- B2. The bridge adapter grows by at most 100 lines outside test files.
  The first figure was 80, and sections 6.2 and 6.3 took 89.
- B3. A new test finishes in under two seconds or starts with
  `slowtest.Skip(t)`.
- B4. The Tier 2 rows are held to the two seconds of A7, measured as the
  base specification states, with the exceptions of A-C3. This document
  sets no other bound on time.

## 10. Stop conditions

Work stops and is reported, without going on, when:

- ST-C1. A Tier 2 row, or a test of A-C2, gives a wrong value in a result
  that passed validation.
- ST-C2. A requirement needs a change to section 7.2, 7.3 or 7.6 to 7.11
  of the base specification (SP5), or to any package of N2.
- ST-C3. The method of 6.4 cannot meet Q3 to Q5 on the table of A-C7
  without a rule for one arrangement of a conic and a cylinder.
- ST-C4. A budget of section 9 is exceeded.
- ST-C5. A response of D3 changes that is not a legacy refusal turned
  into an answer.
- ST-C6. A case of A-C2 or A-C6 that this document says is admitted
  returns an error at the bridge's options.

## 11. Open decisions

Each was the owner's. The owner decided all seven on 2026-11-19, each
as recommended, so the text above stands as written.

- OD-C1. Scope. Recommended: the equal pair only; quartic pairs stay out
  (OD-2 stands). Alternative: specify quartic pairs as well, as a second
  document; section 4.2 lists what it needs.
- OD-C2. `overlapVolume` of a pinched common. Recommended: P1, the volume
  a cut removes. Alternative: measure the pinched common itself, which
  needs a rule in N2 for a common that is only measured and never
  published.
- OD-C3. The order of tools in `cutMany`. Recommended: 6.3. Alternative:
  leave it; T2-004 then stays unsupported, and a `cutMany` fails in one
  order of its tools where it succeeds in another.
- OD-C4. Conics the table cannot meet. Recommended: 6.4, as the last
  phase. It serves one Tier 2 row today, and any later cut of a part that
  already has an equal pair is likely to need it. Alternative: leave it;
  T2-015 stays unsupported.
- OD-C5. Time. T2-007 and T2-013 take 2.13 s and 4.42 s against the two
  seconds of A7, with every value right. Recommended: list both as
  exceptions for time, as the base specification does for Tier 1 rows.
  Alternative: require them under two seconds, which is a piece of work
  on the speed of `Combine` that this document does not size.
- OD-C6. The smallest angle between the axes of an equal pair.
  Recommended: a sine of 1/64 (E4). The legacy paths go no lower than
  5/13.
- OD-C7. The two refusals of T2-014. Recommended: declined inputs; the
  general Boolean cuts a floor inside the crossing and the oracle checks
  the value. Alternative: reproduce the refusal, which needs a rule that
  depends on the family (ST2 of the base specification).

- OD-C8. Concealed cutters. After section 6.3, T2-004 and T2-015 still
  failed: a cutter that a later bore covers could not be proved clear of
  the wall of a bore across it. Asked whether to finish without those
  rows, the owner chose on 2026-11-19 to have them worked on. Sections
  6.4 and 6.7 are that work, and B1 and B2 were raised for it.

## 12. What this leaves for the owner's plan

With sections 6.1 to 6.3, eight of the nine bridge tests that failed in
the probe pass when their first bore is cut by the general Boolean. The
ninth is a quartic pair. If the plain bore families are then cut by the
general Boolean first, a second bore of unequal radius or with a skew
axis across such a bore is refused where the legacy path cuts it today.
The choices then are to accept that refusal, to keep the legacy paths
first for bores, or to commission the work of section 4.2. That decision
is not part of this document.

The legacy recipes for equal radius that this work would let the owner
delete are the constructors of section 3.3's first two items and their
STEP writers and readers. The recipes for unequal radius and skew axes
would stay.
