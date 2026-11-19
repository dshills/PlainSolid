# Edge treatment of a general Boolean result

Status: written on 2026-11-19 against `main` at `4e960375`. The owner
decided OD-E1 to OD-E8 on 2026-11-19, each as this document recommended
(section 14), and asked for the implementation, which follows
[the plan](PLAN.md). Section 15 records the experiment the design rests
on.

## 1. Purpose

`internal/plaincadbridge` answers a Boolean request with the general
Boolean of `brep/boolean` where a legacy path declines it
(`orGeneral`, `general_boolean.go:421`). The answer is published by
`putGeneral` (`general_boolean.go:128`) as a body that holds one
`trimmed.Solid`, its volume and its area, and no recipe. In this document
such a body is a **general result**.

The bridge's edge treatments read recipes. On a general result `fillet`
and `chamfer` return `UNSUPPORTED` "stock type is unsupported"
(`edge_body.go:377`), and `capChamfer` and `capFillet` return
`UNSUPPORTED` "original cap perimeter was lost or is ambiguous"
(`retained_bore.go:85`). This is owner decision OD-8 of
[the general Boolean specification](../general-boolean/SPEC.md).

The owner's plan is to make the general Boolean the default for the plain
bore families and then delete the legacy families it replaces. PlainCAD
chamfers and fillets parts that already have holes, pockets and joins
(section 4.3), so that plan cannot proceed while a general result takes no
edge treatment.

This document specifies one mechanism that applies `fillet`, `chamfer` and
the four-edge `capChamfer` to a general result and publishes a general
result again. It has no code that depends on a named arrangement of
inputs: an edge, its two faces, its angle and a size are data.

Conventions. Requirements carry an identifier made of a letter prefix and
a number; the prefix only groups them by section. "Must" states a
requirement. Lengths are in millimetres, the bridge's unit. "The Boolean
specification" is `specs/general-boolean/SPEC.md`; a reference such as
"Boolean P5" names a requirement of it.

Names in capitals. `UNSUPPORTED`, `INVALID`, `LIMIT` and `INTERNAL` are
the failure codes of a bridge response. OCCT is Open CASCADE Technology,
the reference kernel that PlainCAD used before PlainSolid. STEP (ISO
10303) and IGES are CAD exchange formats. PC2 is the second PlainCAD gate
of `docs/PLAINCAD_READINESS.md`. OD-8 and the other identifiers that
begin with OD, RL, CO or P and are prefixed "Boolean" are entries of the
Boolean specification; OD-E1 to OD-E8 are the owner decisions of section
14 of this document.

## 2. Governing decisions

- G1. `AGENTS.md` governs scope and verification: no function, type or
  file family for one named arrangement of inputs; standard library only;
  a new exported identifier needs a caller outside its package in the same
  change; unsupported input returns an explicit error and never a
  plausible wrong shape; a new test finishes in under two seconds or
  starts with `slowtest.Skip(t)`.
- G2. The tolerance policy of `docs/SPECIFICATION.md` governs: explicit
  tolerances, coincidence within tolerance, results accepted on validation
  and not on proof, an explicit error when no result validates.
- G3. Provenance. This work is not derived from OCCT. Implementation
  sessions do not open, port or copy OCCT source, identifiers or comments
  (Boolean OD-6). The mechanism is the textbook construction of a chamfer
  and of a constant-radius fillet of a straight edge as a set operation
  with a prismatic tool.
- G4. This work does not edit `brep/trimmed` or `brep/boolean`, does not
  touch a file that starts with `//go:build research`, and adds no
  `t.Parallel()` to a test in `brep/trimmed`. If a treatment that this
  document puts in scope fails inside `boolean.Combine`, the session stops
  and reports the input; it does not patch the Boolean under this
  specification.

## 3. Terms

- **General result**: a stored body for which `Bridge.generalResult`
  (`general_boolean.go:167`) is true: an analytic body that holds a
  `trimmed.Solid`, a volume and an area and no legacy recipe.
- **Reference**: one pair of points `start`, `end` of a request, naming an
  edge by its two ends.
- **Edge chain**: one or more edges of the solid, end to end on one
  straight line.
- **Faces of an edge**: the two faces that use it, `F1` and `F2`, with
  outward unit normals `n1` and `n2`.
- **In-face direction** `a_i`: the unit vector in the plane of `F_i`,
  perpendicular to the edge, that points from the edge into `F_i`.
- **Opening angle** ψ: the angle between `a1` and `a2`, in (0, π).
- **Convex edge**: `a2 · n1 < 0`; the material lies between the two
  faces. **Concave edge**: `a2 · n1 > 0`; empty space lies between them.
- **Tool**: the solid that is cut from the body (convex edge) or united
  with it (concave edge).
- **Working face** of a tool: the face that becomes the new face of the
  result: the chamfer plane, or the fillet cylinder. Every other face of a
  tool is a **support face**: the two **flanks**, which lie in the planes
  of `F1` and `F2`, and the two **end caps**, which lie in the planes
  through the ends of the edge perpendicular to it.
- **Size**: `distance` of a chamfer request, `radius` of a fillet request.
- ε_p = 1e-7: the bridge's point tolerance (`nearPoint`, `edge_body.go:34`).
- τ = 1e-9: the bound on a sine or cosine in a parallel or perpendicular
  test, equal to `generalOptions.Angular`.
- β = 1e-7: `generalOptions.Volume`.

## 4. Present behaviour

This section is a record of what the code does on `main` at `4e960375`.
It states no requirement.

### 4.1 Operations the bridge serves

All are dispatched at `bridge.go:314-340`.

| Operation | Request fields | Served by | Stock it accepts today |
|---|---|---|---|
| `edgeAvailable` | `handle`, `edgeStart`, `edgeEnd` | `stored.edgeAvailable` (`edge_body.go:216`) | Any; on an analytic body it looks for one straight edge with those ends and two differing face normals in the stored solid |
| `fillet`, `chamfer` | `handle`; `edges` (1 to 32 references) or `edgeStart` and `edgeEnd`; `normal`; `radius` or `distance`; optional `direction` | `treatEdge` (`edge_body.go:334`) | Treated stock (`edgeBody`), planar stock, bored stock (`boreBody`) with a rectangular recipe, and a pocketed analytic body that has a planar boundary |
| `capChamfer`, `capFillet` with four `edges` | `handle`, `edges`, `normal`, `distance` or `radius` | `treatCapPerimeter` (`retained_bore.go:85`) | Rectangular planar stock; rectangular bored stock (chamfer only); a cap-filleted box (chamfer of the other cap) |
| `capChamfer`, `capFillet` with `origin` and no `edges` | `handle`, `origin`, `normal`, `distance` or `radius` | `chamferAnnularStock`, `chamferRoundedStock`, `filletRoundedStock` | An unmodified annulus or rounded rectangle, by recipe |
| `filletCylinderRims`, `chamferCylinderRims` | `handle`, `rimCenters` (1 or 2), `normal`, `radius` or `distance` | `treatCylinderRims` (`cylinder_rim.go:11`) | An unmodified cylinder |

PlainCAD (`PlainSolidKernel.ts` on its branch `codex/plainsolid-backend`,
`treatEdge`) sends only references to authored edges of the original
extrusion: a straight segment of the profile on the start or end cap (a
cap edge); the two edges along the extrusion at the ends of a profile
line, with `direction` (axial edges); the whole perimeter of a
four-segment cap, as `capChamfer` or `capFillet` with four `edges`; the
rim of a circular extrusion; and the perimeter of a rounded rectangle or
an annulus, with `origin`. Before a treatment it asks `edgeAvailable` for
each authored edge. After it, it requires a valid result with the same
solid count and a changed volume or area.

### 4.2 Why each fails on a general result

| Operation | Where | What it reads that a general result lacks |
|---|---|---|
| `fillet`, `chamfer` | `edge_body.go:377` | One of: the `edgeBody` recipe (frame, box sizes, four corner records, optional bore, pocket and boss); a planar `brep.Solid`; a `boreBody` with its planar `base` and cylinder list. The treated solid is rebuilt from the recipe by `fillet.TreatLongEdgesGrouped` or a constructor of `brep/trimmed`; no path treats an edge of the stored `trimmed.Solid` itself |
| `capChamfer`, `capFillet` with `edges` | `retained_bore.go:85` | `rectangularBoredStock` of a `boreBody`, or an orthogonal planar prism, or the `filletedCap` recipe |
| `capChamfer`, `capFillet` with `origin` | `annular_stock.go:17`, `rounded_stock.go` | The `sector` or `rounded` recipe |
| `filletCylinderRims`, `chamferCylinderRims` | `cylinder_rim.go:17` | The `cylinderBody` |
| `edgeAvailable` | `edge_body.go:241` | Nothing: it reads the stored solid and answers. It takes one edge only, not a chain, and also answers true for a straight edge between a plane and a cylinder |

What a treatment needs from a body that is only a `trimmed.Solid` of
planes and cylinders: the edge and its two faces, found in the solid; a
way to remove or add the material of the treatment; and the volume and
area of the result. The first is a search of the solid. The second and
third are what `boolean.Combine` already does.

### 4.3 What surrounds a treatment

From the bridge tests and the PlainCAD end-to-end tests (the traces under
`testdata/general_boolean` contain no treatment request, because OD-8
excluded them):

- Every treated edge is an authored edge of the original extrusion. Where
  a Boolean precedes the treatment, the edge is an outer cap edge of a
  box, convex, at 90°, and it ends on faces perpendicular to it. Success
  requires the treatment to stay clear of the hole, pocket or join;
  contact is rejected (`retained_bore_edges_test.go`,
  `bored_cap_edge_chamfer_test.go`, `pocket_edge_test.go`).
- Boolean first, treatment second, in PlainCAD: `hole-edit.spec.ts:160`
  (two holes, a counterbore, then a cap-edge chamfer);
  `retained-edge-picking.spec.ts:71` (a cut or a join, then a cap-edge
  chamfer or fillet); `retained-references.spec.ts:315` (a through hole,
  a chamfer of the whole cap perimeter, a cut and a join, then a cap-edge
  fillet). None of these is in the PC2 inventory of
  `docs/PLAINCAD_READINESS.md`; they are among the tests that inventory
  says are still to be enumerated.
- The four PC2 cases treat a plain box first and apply Booleans
  afterwards. They are served by the `edgeBody` recipe and its own Boolean
  (`edge_boolean.go`), and this specification does not change them.
- Edges that are not at 90°, and concave edges, occur only on plain prisms
  with no Boolean before them: the axial edges of a triangular prism (an
  interior angle of about 36.9°) and of an L-shaped prism (one convex and
  one concave edge in one request).
- After a treatment the tests apply: a second treatment; `cut`, `cutMany`
  and `fuse` (a hole, a pocket, a boss); `overlapVolume`; `mesh`, `stl`;
  `exportStep` and `importStep`; `displayEdges`; `planarFace`;
  `edgeAvailable`; `inspect`. In PlainCAD also save and open, which
  rebuilds the part from its features and issues the same requests again.
- Volumes and areas are compared with closed forms at a relative 1e-8
  (`near`, `bridge_test.go:78`).

## 5. Scope

- S1. In scope: `fillet` and `chamfer` of one or more straight edges, each
  between two planar faces, on a general result; `capChamfer` with four
  `edges` on a general result; and `edgeAvailable` on a general result.
- S2. Convex and concave edges are both in scope, with every opening
  angle that R7 admits, and so is a request that names several edges.
- S3. The result is a general result. Every operation that the bridge
  serves on a general result today is served on it (section 10).
- S4. Stock that is not a general result is out of scope. Every request
  on such stock is answered exactly as it is today (D4).
- S5. Section 9 lists the inputs that are out of scope on a general
  result, each with the error it returns.

## 6. Request contract and dispatch

- D1. No request field is added, removed or renamed, and no response
  field changes. The operation list of `capabilities` is unchanged.
- D2. The argument checks of `treatEdge` that precede the stock test
  (`edge_body.go:335-361`) are unchanged and apply first: at most 32
  references; either `edges` or the pair `edgeStart`, `edgeEnd`, not
  both; both ends of every reference; `normal` present and nonzero; a
  finite positive size. Their codes and messages are those of today.
  So are the argument checks of `treatCapPerimeter`: `normal` present
  and nonzero, exactly four `edges`, a positive size.
- D3. Dispatch. Where `treatEdge` returns "stock type is unsupported"
  today (`edge_body.go:377`), it must first test whether the stored shape
  is a general result, and if so answer with the mechanism of section 7.
  Where `treatCapPerimeter` rejects stock that is neither bored, planar
  nor a cap-filleted box (`retained_bore.go`, the final `else` of the
  stock switch), it must make the same test and answer with section 7.7.
  The handling of `edgeAvailable` (`bridge.go:334-339`,
  `edge_body.go:216`) must make the same test before the analytic branch
  is reached and answer with R11.
- D4. No other branch of the bridge changes. For stock that is not a
  general result, every request must give the response it gives at
  `4e960375`. No existing test file of `internal/plaincadbridge` is
  edited, and every existing test must pass.
- D5. The operand of the mechanism is the solid that `generalOperand`
  returns for the stored shape, and its known volume and area are those
  of `generalKnown`. Every call of `boolean.Combine` uses
  `generalOptions`.
- D6. A request is atomic. If any step fails, the bridge publishes
  nothing: the number of handles and the handle counter are what they
  were before the request.
- D7. An error message of `fillet` or `chamfer` begins with the operation
  name followed by " failed: ", as today's messages do; PlainCAD tests
  for that prefix.

## 7. Functional behavior: the mechanism

### 7.1 Resolving a reference

- R1. A reference resolves to an edge chain of the operand solid: edges
  `e_1 … e_k`, k ≥ 1, no edge twice, such that the first begins within ε_p of one end of
  the reference, the last ends within ε_p of the other, each begins where
  the one before ends, and every vertex of the chain lies within ε_p of
  the straight segment between the two ends of the reference.
- R2. Every edge of the chain must be straight: its curve, read with
  `parametric.CurveMapping`, is a `geom.Line`.
- R3. Every edge of the chain must have exactly two uses, on two
  different faces.
- R4. Both faces of every edge must be planes: the surface, read with
  `parametric.SurfaceMapping`, is a `geom.PlaneSurface`.
- R5. The two planes must be the same for every edge of the chain: equal
  normals within τ and equal offsets within ε_p.
- R6. Exactly one chain must satisfy R1 to R5 for the reference. None, or
  more than one, fails the reference. Two chains are one when they have
  the same set of edges, whatever the order or direction.
- R7. The planes must not be parallel: `|n1 × n2| > τ`. This excludes a
  seam between coplanar faces and a knife edge.
- R8. Classification of a reference that does not resolve, in this
  order. If some chain satisfies R1, R2 and R3 and one of its edges has a
  face that is not a plane, the reference is "on a curved face" (X2).
  Otherwise, if some chain satisfies R1 to R5 and fails R7, it is
  "between parallel faces" (X3). Otherwise it is "lost" (X1).
- R9. References of one request that resolve to the same chain are one
  edge. Two different chains of one `fillet` or `chamfer` request must
  not share a vertex (OD-E3): no vertex of the operand solid, by its
  index, is a vertex of both. A `capChamfer` request is governed by 7.7.
- R10. All references are resolved against the operand solid before any
  tool is built. A request with a reference that fails is rejected whole.
- R11. `edgeAvailable` on a general result is true exactly when its one
  reference satisfies R1 to R7, and false otherwise. It never returns an
  error for a reference that fails them.

### 7.2 The request's `normal` and `direction`

- N1. For every resolved edge, the unit `normal` of the request must be
  parallel or antiparallel, within τ, to `n1` or to `n2`, or parallel or
  antiparallel to the edge. PlainCAD sends the cap normal for a cap edge
  and the normal of the side face for an axial edge; the bridge's own
  tests also send the extrusion axis for an axial edge.
- N2. `direction`, when present, must be nonzero and parallel or
  antiparallel, within τ, to every resolved edge.
- N3. Whether an edge is convex or concave is decided from the solid
  (section 3) and from nothing in the request. `direction` is not
  required for a concave edge (OD-E7).

### 7.3 The tool of one edge

Let P and Q be the two ends of the chain, L = |Q − P|, s the size, and
`a1`, `a2`, ψ as in section 3. The tool is the same for a convex and for
a concave edge; only the operation of 7.4 differs.

- T1. Chamfer. The tool is the prism with the triangular section
  P, P + s·`a1`, P + s·`a2`, swept from P to Q. The distance is measured
  along each face, which is what the legacy convex-prism chamfer does.
  Its working face is the face through P + s·`a1` and P + s·`a2`.
  Section area S = ½ s² sin ψ; width of the working face
  w = 2 s sin(ψ/2); setback along each face g = s.
- T2. Fillet. Let g = s / tan(ψ/2), T1 = P + g·`a1`, T2 = P + g·`a2`,
  and C = P + (s / sin(ψ/2))·b, where b is the unit vector along
  `a1` + `a2`. The tool is the prism with the section P, T1, C, T2 swept
  from P to Q, less the cylinder of radius s whose axis runs from C to
  C + (Q − P). It is made with one call of `boolean.Combine` (prism, cylinder,
  `Cut`). The cylinder passes through the two prism edges at T1 and T2
  and is tangent there to the flanks. Its working faces are the faces of
  that result which have a face of the cylinder among their sources
  (Boolean P5). Section area S = s·g − s²(π − ψ)/2; width of the
  working face w = s(π − ψ).
- T3. A tool spans exactly the chain: its end caps lie in the planes
  through P and through Q perpendicular to the edge. A tool is not
  extended past an end and not shortened.
- T4. Every tool is a valid operand of `boolean.Combine` (Boolean 4.1).
  P6 states how tools are built.
- T5. If the prism or the cylinder of a tool cannot be constructed,
  because the section is degenerate at the working tolerance or a
  constructor refuses it, the request fails with X7. An error of the
  `Combine` call of T2 is not X7: it is an error of `boolean.Combine`
  and fails the request with X11 (L2).

### 7.4 Combining

- B1. The tools of the convex edges of a request are united into one
  solid `Tx` by a left fold of `boolean.Combine` with `Union`; the tools
  of the concave edges into `Tv` likewise. A set of one tool needs no
  call. The union of tools that do not touch is a solid of several
  shells, which is a valid operand (Boolean 4.3).
- B2. Through each union, a face of the united tool is a working face
  exactly when every one of its sources is a working face; otherwise it
  is a support face. A working face of the united tool belongs to every
  edge whose tool contributed one of its sources.
- B3. With body A: if `Tx` is not empty, `R1 = Combine(A, Tx, Cut)`,
  otherwise `R1 = A`. If `Tv` is not empty,
  `R = Combine(R1, Tv, Union)`, otherwise `R = R1`.
- B4. Each call passes the known volume and area of its first operand, as
  the folds of the Boolean adapter do (Boolean CO1).

### 7.5 Checks on the result

The checks apply to each call of B3 separately. For a call, `A'` is its
first operand, `T` its tool, `R'` its result, and "an edge of the call"
is an edge whose tool is part of `T`. A **working result face** is a face
of `R'` that has a working face of `T` among its sources. The
**treatment planes** of the call are, for each edge of the call, the
planes of its `F1` and `F2` and the two planes through its ends
perpendicular to it. The checks are made in the order K1 to K7, and the
first that fails decides the error (section 9).

- K1. The result is not empty: `R'` has at least one solid and
  `V(R') > 0`.
- K2. Supports leave no trace. No face of `R'` has a support face of `T`
  among its sources. A flank that lies wholly on the face it belongs to
  is a shared piece, which the cut drops (same normals) and the union
  drops (opposite normals); so is an end cap that lies wholly on a face
  of the body. A flank that passes under a neighbouring feature, and an
  end cap that lies in the material or in empty space, are kept and are
  found by this check.
- K3. Clearance. Every face of `R'` that shares a vertex with a working
  result face must itself be a working result face, or a planar face
  that lies in a treatment plane of the call (the plane test of R5).
  A hole, a pocket wall, the far side of a thin wall or any other
  feature that meets or touches the new face is found by this check,
  whatever the volume involved. Two points of `R'` are one vertex when
  the Boolean made them one node, so a feature closer to the treatment
  than the Boolean's working tolerance counts as touching it.
- K4. The tool is wholly material, or wholly empty. For the cut,
  `|V(A') − V(R') − V(T)| ≤ β·V(T)`; for the union,
  `|V(R') − V(A') − V(T)| ≤ β·V(T)`. The three volumes are those the
  `Combine` result reports. This finds a closed void inside a convex
  tool, or material inside a concave one, that K3 does not see because
  it does not reach the new face. A discrepancy below the bound is
  accepted; the result is then still the exact difference or union.
- K5. The solid count of `R'` equals that of `A'`.
- K6. Every edge of the call left its face: for each, at least one
  working result face has a working face of that edge's tool among its
  sources (B2 keeps the edge of each working face through the union).
- K7. After the last call, no chain of the request resolves in `R` (R1
  to R7): the treated edges are gone.

K2, K3 and K4 together are the legacy rule that a treatment must stay
clear of every other feature (4.3), stated on the result and not on the
arrangement. Contact counts: a treatment that touches a feature at a
point or along a line is rejected by K3, as the legacy paths reject it
(OD-E5).

### 7.6 Publishing

- U1. `R` is published with `putGeneral`: its volume and area are the
  closed-form values the last `Combine` call reports, its solid count is
  that call's, and its bounds are those of `generalBounds`. The adapter
  does not integrate.
- U2. The published body is a general result, so a later treatment of it
  is answered by this mechanism and a later Boolean by the general
  Boolean.

### 7.7 `capChamfer` with four edges

- C1. The four references are resolved by R1 to R7 and R10. A reference
  that does not resolve fails the request with X13, whatever R8 would
  call it.
- C2. They must resolve to four different chains that form one closed
  loop: each end of each chain is, within ε_p, an end of exactly one
  other. Each chain must have a face whose normal is parallel, within τ
  and with the same sign, to the request's `normal`, and that face must
  be the same plane for all four (R5's test).
- C3. Each edge must be convex.
- C4. The four chamfer tools of T1, with s = `distance`, are combined and
  checked by 7.4 and 7.5 and published by 7.6. Tools that meet at a
  corner overlap; the union of B1 makes them one tool, and K4 is taken on
  the united tool.
- C5. A request that fails C1 to C3 returns today's rejection of
  `treatCapPerimeter` (section 9).

## 8. Limits

- L1. A request makes at most 2n + 1 calls of `boolean.Combine` for n
  edges: n to build fillet tools, n − 1 to unite, and at most 2 to
  combine with the body. n ≤ 32 (D2).
- L2. An error of any call of `boolean.Combine`, in T2, B1 or B3, is
  mapped by `generalFailure` (`general_boolean.go:401`), and D7 applies
  to the message (X11).
- L3. The work is bounded at 1,200 lines of Go outside test files, over
  the new package and the bridge adapter together. Reaching the bound
  stops the work and is reported to the owner.

## 9. Inputs out of scope on a general result

Each returns an explicit error and publishes nothing (D6). "Today's" means
the code and message the operation returns at `4e960375`, unchanged.

Order. A request is examined in this order, and the first failure is the
answer: the argument checks of D2; the references in the order of the
request, each by R1 to R8 (X1, X2, X3; for `capChamfer`, X13 in place
of all three); N1 (X4) and N2 (X5) for each edge in that order; R9
(X6), or C2 and C3 for `capChamfer` (X13); the tools in the order of
the edges, each constructed (X7) and, for a fillet, cut by T2 (X11);
the calls of B1 and then of B3 (X11); and after each call of B3 the
checks K1 to K7 in their order.
`capFillet` with four `edges` returns X12 as soon as the argument
checks of `treatCapPerimeter` (D2) have passed.

| # | Input | Code | Message |
|---|---|---|---|
| X1 | A reference that is lost (R8): no chain, or more than one; this includes an edge consumed by an earlier treatment and a reference that covers only part of an edge | `UNSUPPORTED` | "`<op>` failed: edge reference was lost or is ambiguous; no complete sharp edge resolves" (today's text) |
| X2 | A reference on a curved face (R8): a straight edge with a cylinder on one side | `UNSUPPORTED` | "`<op>` failed: this stock takes a treatment only on a straight edge between two planar faces" |
| X3 | A reference between parallel faces (R8) | `UNSUPPORTED` | as X1 |
| X4 | `normal` fails N1 | `UNSUPPORTED` | "`<op>` failed: extrusion normal does not match the selected edge" |
| X5 | `direction` fails N2 | `UNSUPPORTED` | "`<op>` failed: direction does not match the selected edge" |
| X6 | Two edges of one `fillet` or `chamfer` request share a vertex (R9) | `UNSUPPORTED` | "`<op>` failed: grouped edges must not share an end point" |
| X7 | A tool cannot be built (T5) | `UNSUPPORTED` | "`<op>` failed: the treatment size does not fit the selected edge" |
| X8 | K1, K3 or K4 fails: the size exceeds the stock, or a void, another feature or the far side of the stock lies within the size or touches the new face | `UNSUPPORTED` | "`<op>` failed: the treatment does not fit; a void, another feature or the far side of the stock lies within its size" |
| X9 | K2 fails: an edge that does not end on faces perpendicular to it (an oblique end, or an end in the material at a wall), or a flank under a neighbouring feature | `UNSUPPORTED` | "`<op>` failed: the treatment does not end on faces of the stock, or reaches under a neighbouring feature" |
| X10 | K5, K6 or K7 fails | `UNSUPPORTED` | "`<op>` failed: the treatment would divide the stock or leaves the selected edge in place" |
| X11 | `boolean.Combine` returns an error in T2, B1 or B3. The known case: a fillet cylinder that meets a cylinder with a different axis direction, for example a bore across the fillet, or two fillets that meet at a corner inside one united tool | as L2 maps it; `UNSUPPORTED` for the known case | "`<op>` failed: " and the Boolean's message |
| X12 | `capFillet` with four `edges`: its fillets meet at corners, where two cylinders of equal radius cross; the Boolean leaves that pair unsupported (Boolean RL-22) | `UNSUPPORTED` | "capFillet failed: fillets that meet at a corner are unsupported on this stock" |
| X13 | `capChamfer` with a reference that does not resolve (C1), or whose edges fail C2 or C3 | `UNSUPPORTED` | "original cap perimeter was lost or is ambiguous" (today's) |
| X14 | `capChamfer` or `capFillet` with `origin` and no `edges` | today's | today's: the result needs a cone or a torus, outside the surface class of a general result |
| X15 | `filletCylinderRims`, `chamferCylinderRims` | today's | today's: the result needs a torus or a cone |

Not offered at all, because the request contract has no field for them:
a chamfer with two distances, a variable radius, and a chain of edges
that turns a corner.

Whether the acceptance cases need any of these (4.3): no. X2, X9 (oblique
ends), X12, X14 and X15 on stock that a Boolean has already cut are
rejected by the legacy paths as well. X6 is rejected by the legacy paths
for every stock. X11 is the legacy clearance rule, met here by the
Boolean's own refusal.

## 10. The result stays usable

- F1. On a body published by 7.6 each of these requests must succeed, and
  a test must issue it to a filleted and to a chamfered general result:
  `inspect`; `clone`; `transform` with a rigid matrix that is not
  axis-aligned; `mesh` with `linearDeflection` 0.05 and
  `angularDeflection` 0.2, with and without `analyticNormals`; `stl`;
  `displayEdges` in both budget formats; `drawingGeometry`; `planarFace`
  of an untouched face; `exportStep`, and `importStep` of the exported
  file with volume and area within 1e-7 of the exported body.
- F2. On the body of Q1 after its chamfer, with volume V_c, and on the
  body of Q2 after its fillet, with volume V_f, each of these must
  succeed with the stated volume within 1e-8 relative:
  - `cut` with the cylinder of radius 1 about the line x = 9, y = 5, from
    z = −1 to z = 11, which is clear of the treatment: V − 10π;
  - `cut` with the box (8,−1,−1)–(10,3,3), which crosses the treated
    edge: V_c − 17.75 and V_f − (10 + 2π);
  - `fuse` with the cylinder of radius 1 and height 3 standing on the
    top face at (5,5,10): V + 3π;
  - `cutMany` with cylinders of radius 1 about x = 3, y = 5 and about
    x = 9, y = 5, each from z = −1 to z = 11: V − 20π;
  - `overlapVolume` with the box (0,4,4)–(1,6,6): 4;
  - a second `chamfer` 0.5, and a second `fillet` 1, of the edge
    (0,10,0)–(20,10,0): the closed form of 11.1 applied to V.
- F3. `edgeAvailable` must be false for a treated edge and true for an
  untouched edge of the same body.
- F4. A smooth edge, where a fillet meets a face, must not resolve: its
  two normals are equal, and R4 fails for it as well.

## 11. Acceptance criteria: how correctness is judged

Tests are in the new package for the geometry and in
`internal/plaincadbridge` for requests. Every test finishes in under two
seconds.

### 11.1 Closed forms

For a request whose edges do not meet and whose tools do not overlap one
another, with the tools wholly in the material or wholly outside it and
every end on a face perpendicular to its edge:

- V = V₀ − Σ L·S over convex edges + Σ L·S over concave edges;
- A = A₀ + Σ L·(w − 2g) − Σ 2S over convex edges + Σ 2S over concave
  edges;

with S, w and g from T1 and T2. Each row below is a test; volume and area
must agree with the closed form within 1e-8 relative, on the scale
`max(1, |expected|)` (the `near` helper), the solid count must be 1 and
the bounds must agree within 1e-7.

| # | Body | Request |
|---|---|---|
| Q1 | The slotted box: the box (0,0,0)–(20,10,10) cut by the box (12,−1,6)–(16,11,11), a general result with V₀ = 1840 and A₀ = 1048 | `chamfer` 0.5 of the edge (0,0,0)–(20,0,0) |
| Q2 | As Q1 | `fillet` 2 of the same edge |
| Q3 | As Q1 | `fillet` 2 of the edges (0,0,0)–(20,0,0) and (0,10,0)–(20,10,0) in one request |
| Q4 | Box 20×10×10 with two through holes of radius 2 and 2.5 and a counterbore of radius 3 and depth 1 (the body of `TestHoleEditLayeredSingleEdgeChamfer`) | `chamfer` 0.5 of the edge (0,0,0)–(20,0,0) |
| Q5 | Box 40×30×10 with a blind pocket (the body of `TestPocketChamferIndependentFallback`) | `chamfer` 1 and `fillet` 1 of each of the four top edges in turn |
| Q6 | The box (0,0,0)–(20,20,10) with a through hole of radius 2 about the line x = 10, y = 10 | `capChamfer` 1 of the four top edges. Its edges meet, so the formulas above do not apply; expected volume 4000 − 40π − (40 − 4/3) and area 1444 + 76√2 + 32π |
| Q7 | Regular hexagonal prism about the z axis, circumradius 5, one vertex at (5,0), height 8 | `chamfer` 1 and `fillet` 1.5 of one axial edge (ψ = 120°), with `direction` |
| Q8 | Triangular prism (0,0), (4,0), (0,3), height 5 | `chamfer` 5/16 of the axial edges at (4,0) and at (0,0); `fillet` 5/16 of the axial edge at (4,0) |
| Q9 | L-shaped prism with the profile (0,0), (20,0), (20,10), (10,10), (10,20), (0,20) and height 10 (V₀ = 3000, A₀ = 1400), as in `TestOrthogonalAxialEdgeTreatmentPreservesMixedCornerVolume` | `chamfer` 1 and `fillet` 1 of the concave axial edge at (10,10) alone, and of that edge with the convex axial edge at (20,10) in one request |
| Q10 | The body of Q1 under a rigid motion that is not axis-aligned | `fillet` 2 of the moved edge |

Q7 to Q9 are plain prisms, which the bridge holds as planar stock and
treats on the legacy path (D4). For those rows the test applies the
mechanism to the lifted solid directly, below the bridge's dispatch.

### 11.2 Agreement with the legacy path

For stock that both paths treat, the same request is answered by the
legacy path on a default bridge and by the mechanism applied to the
lifted stock, and the two answers must agree: valid, the same solid
count, volume and area within 1e-7 relative, bounds within 1e-7. The
rows: a plain box with one cap edge, `fillet` and `chamfer`; a plain box
with two parallel cap edges in one request; the triangular prism of Q8;
the L-shaped prism of Q9; a box with one through hole cut by its legacy
path, with a cap-edge `chamfer`; the pocketed box of Q5; the bored box of
Q6 with `capChamfer`.

### 11.3 Rejections

Each row of section 9 from X1 to X13 has at least one test that issues
the input to a general result and requires the stated code, the stated
message and D6. The tests must include these inputs, taken from the
legacy tests and from the experiment:

- X1: the same chamfer issued twice; a reference to half of an edge.
- X3: on the box (0,0,0)–(20,10,10) united with a cylinder of radius 2
  standing on its top face at (10,2,10), which touches the top front
  edge, the edge by which the Boolean divided the top face.
- X9, on an edge chain: `chamfer` 0.5 of (0,0,10)–(20,0,10) of that
  body. The cylinder touches the edge at (10,0,10) and stands on the
  flank, so K2 rejects it.
- X8 by K1: `chamfer` 100 on the box of Q1 (PlainCAD's invalid-size
  case).
- X8 by K3: a chamfer of 1 beside two holes of radius 1 whose axes are
  2 from the edge, which touches them
  (`TestRetainedCavityEdgeTreatmentRejectsContactAtomically`); the same
  with the axes 1.999 from the edge; a pocket wall inside the treatment
  (`TestBlindPocketTreatmentRejectsPocketWallIntersection`); a wall
  thinner than the size; a fillet whose radius equals the width of a
  face.
- X8 by K4: a body with a closed void that lies inside the tool and does
  not reach the new face.
- X9: a cap edge of the hexagonal prism of Q7; an edge that ends at a
  taller block; a boss standing on the flank.
- X11: a fillet of an edge that a bore crosses at right angles.
- Accepted, as the counterpart of the K3 rows: the chamfer of 1 with the
  hole axes 2.000001 from the edge succeeds with the closed-form volume.

Each of K2, K3 and K4 must have a row above that it rejects and that
passes every other check, so that removing the check makes a test fail.
K1, K5, K6 and K7 guard against a result that the other checks let
through; each has a test below the bridge, on a tool made for the
purpose.

### 11.4 What is not judged

No proof is built or required. PlainCAD's browser gates, the complete Go
suite, race runs and the qualification script are run only when the owner
asks.

## 12. Project constraints on the code

These are constraints of this repository (G1) and not behavior of the
bridge. An implementation that meets sections 6 to 11 and breaks one of
these does not conform.

- P1. The mechanism of 7.1 to 7.5 is a new package, `plainsolid/brep/blend`
  (OD-E1). It imports `brep`, `brep/boolean`, `brep/parametric`,
  `brep/trimmed` and `geom`, and nothing outside the standard library
  besides.
- P2. It exports what the bridge calls and nothing else: one function
  that resolves a reference in a solid, one function that treats a set of
  resolved edges, the type of a resolved edge, the two treatment kinds,
  the result type and the error values that section 9 distinguishes.
  Every exported identifier is called from `internal/plaincadbridge` in
  the change that adds it.
- P3. The package holds no package-level mutable state and does not
  modify its operands.
- P4. The bridge adapter is one new file in `internal/plaincadbridge`. It
  reads the request, calls the package, maps errors to section 9 and
  publishes by 7.6. The three dispatch points of D3 are the only edits to
  existing bridge files.
- P5. No native persistence, STEP or IGES code is added. A treated
  general result is written to STEP by the writer that serves every
  general result.
- P6. A prism is built with `brep.Prism` and lifted with
  `trimmed.FromPlanar`; a cylinder is built with `parametric.Cylinder`
  and lifted with `trimmed.FromParametric`, as the Boolean adapter lifts
  its operands. No tool face is built by hand.

## 13. Non-goals

- Treating stock that is not a general result, and replacing or deleting
  any legacy treatment path (OD-E2).
- Lifting recipe-held treated stock to a `trimmed.Solid` so that the
  general Boolean takes it (the follow-up of Boolean OD-8). A body
  treated by this mechanism is already a general result; a body treated
  by a legacy path stays a recipe.
- Edges on or ending at a cylinder, rims, the perimeters of rounded and
  annular stock, fillets that meet at a corner, ends that are not on a
  perpendicular face, chamfers with two distances, variable radii.
- Any change to `brep/boolean`, including the cylinder pairs it leaves
  unsupported.
- Making the general Boolean the default for any family.

## 14. Owner decisions

The owner decided OD-E1 to OD-E8 on 2026-11-19, each as this document
recommended. None is open. The decision reached the implementing session
through the session that reviews this work, which quoted the owner's
answer to "Approve the eight recommended decisions so the OD-8 session
can start implementing?" as "Approve all eight". Each entry states the
decision, where this document applies it, and what was not taken.

- OD-E1. The code lives in a new package, `brep/blend`, with a thin
  bridge adapter (section 12). Not taken: everything in
  `internal/plaincadbridge`.
- OD-E2. Only a general result takes the new path (S4, D3), so no
  response for other stock changes. Not taken: asking the mechanism
  wherever a legacy treatment declines as unsupported, which would make
  requests succeed that the legacy tests require to be rejected; and
  treating plain planar stock with the mechanism, which would change the
  PC2 path.
- OD-E3. Edges of one `fillet` or `chamfer` request that share a vertex
  are rejected (X6), as every legacy path rejects them. The mechanism
  itself chamfers them when asked, and does so for `capChamfer`.
- OD-E4. Ends. Only an edge whose tool ends on faces of the body is
  accepted (K2), which is every edge that ends on perpendicular faces.
  Not accepted: an edge that runs into a wall, where the treatment would
  stop at a new face in the wall's plane, and an edge with an oblique end
  face, where the tool would have to be extended and trimmed. Both fit
  the same mechanism as data and are left for a later request.
- OD-E5. A treatment crosses and touches nothing (K3, K4), which is the
  legacy clearance rule. The experiment shows that the Boolean cuts a
  chamfer correctly where its wedge crosses a bore, and where it touches
  a hole at one point; K3 rejects both because the legacy paths do and
  the bridge's tests expect it. Not taken: dropping K3 and accepting
  whatever the Boolean validates.
- OD-E6. `capFillet` with four edges on a general result is unsupported
  (X12) until the Boolean takes equal cylinders with crossing axes
  (Boolean OD-1, RL-22). The legacy path also refuses it on bored stock.
- OD-E7. Concavity is decided from the solid, and `direction` is not
  required for a concave edge (N3). The legacy path refuses a concave
  axial edge without it.
- OD-E8. The acceptance bounds are 1e-8 relative against closed forms
  (11.1), which is what the bridge's treatment tests use, and 1e-7
  against the legacy path (11.2), which is Boolean OD-7.

## 15. Record of the experiment

Made on 2026-11-19 at `4e960375` with a scratch test in
`internal/plaincadbridge` that was not committed. Every call was
`boolean.Combine` with `generalOptions`.

- Chamfer, box 20×10×10, edge of length 20, distance 0.5: the wedge cut
  gave volume 1997.5 and the closed-form area exactly, 7 faces, for a
  tool flush with the faces and ends and for tools that overshoot them.
- Fillet, same edge, radius 2. The question was what `Combine` does with
  the tool's tangential contact. It builds the tool: the square prism
  less the cylinder, where the cylinder passes through two edges of the
  prism tangent to its faces, gave one solid of 5 faces with volume
  17.1681469282 = 20·(4 − π). Cutting the body with that tool gave
  volume 1982.83185307 and area 981.115038379, both equal to the closed
  form to the last digit printed, 7 faces, no touching contact reported.
  Three other formulations (notch then add a quarter cylinder; notch then
  add the whole cylinder; an overshooting tool) gave the same solid. The
  whole-cylinder formulation fails on a body with a bore, since the
  cylinder then crosses the bore; the tool of T2 does not.
- Concave edge of an L-shaped prism: chamfer and fillet by union, exact.
- Opening angle 120° (hexagonal prism), chamfer and fillet: exact. A box
  under an oblique rigid motion, fillet: volume within 1.2e-12.
- Four chamfer wedges around a cap, as a fold of cuts and as one united
  tool: both gave 2000 − 30 + 4/3. Two fillet tools that do not touch,
  united into a two-shell solid and cut at once: exact.
- Sources. A good chamfer and a good fillet show only the working face of
  the tool among the sources of the result. A half-length tool shows an
  end cap; a tool under a boss shows a flank (and the fillet divides the
  body in two); a tool that ends at a taller block shows an end cap. A
  wall thinner than the distance and a pocket within the wedge show no
  support face and fail the volume identity (2.1 and 2.365 removed
  against a tool of 2.5).
- Contact. A chamfer of 1 beside holes of radius 1 whose axes are 2
  from the edge, so that the wedge touches each hole at one point: the
  Boolean returns a validated solid with the full wedge removed and the
  cap face divided at the points of contact. With the axes at 1.999 the
  wedge enters the holes; the volume removed falls short of the tool by
  4.8e-8, less than 1e-8 of the tool, so a volume test alone does not
  see it. The clearance test of K3, written out in the scratch test,
  found foreign neighbours of the new face in both cases and at 1.9999999,
  and none at 2.000001 or 2.5; none for the chamfer of Q4, for a fillet
  on the slotted box or for a chamfer on a plain box; and it found the
  far faces for a wall thinner than the size and for a fillet as wide as
  its face.
- Two fillets that meet at a corner, and a bore across a fillet:
  `ErrUnsupported`, "cylinders with axes that are not parallel".
- A fillet whose radius equals the width of the face: cut correctly, to a
  quarter cylinder.
- On a filleted general result, on a default bridge: `inspect`, `mesh`,
  `stl`, `exportStep`, `importStep` of the exported file (same volume
  and area), `displayEdges` (both formats, with PlainCAD's budgets),
  `drawingGeometry`, `planarFace`, `transform` and `edgeAvailable`
  succeed; a bore clear of the fillet, a pocket box across the fillet and
  a boss are combined correctly.

- A box united with a cylinder that stands on its top face and touches
  the top front edge at one point: the Boolean divides the top face in
  two, the top front edge becomes two edges with different top faces in
  one plane, and the divider is a straight edge between two coplanar
  faces. These are the inputs of R5, R7 and X3.

Not tried: a treatment through the bridge, since no code of it exists;
`pocketFloor` and `holeRim` on a treated result; a mirrored transform;
opening angles near 0 or π; sizes near the tolerance; any PlainCAD run.
