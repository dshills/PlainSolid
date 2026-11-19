# General Boolean trial: acceptance rows

This page freezes the acceptance cases of [the specification](SPEC.md),
section 15.1, as requirement A1 asks: 109 rows, 92 in Tier 1 and
17 in Tier 2. The rows and their traces are frozen. The three A8
columns, the A8 summary below the table and the section "Oracle run" at
the end are results of the trial, recorded by Part 5.2 of
[the plan](PLAN.md) on commit `1ae12376`.

A row is one existing test function of `internal/plaincadbridge`. Its trace
is the list of bridge operations the function performs, in order, as
requests, with the responses the default bridge gave. It was recorded on
commit `9d7417dc` by a temporary patch that is not in the tree.
`TestGeneralBooleanTraces` in
`internal/plaincadbridge/general_boolean_trace_test.go` replays every trace
on a default bridge and requires the recorded responses within 1e-12
relative; the comment at the top of that file defines the trace format.

- Traces: `internal/plaincadbridge/testdata/general_boolean/<function>.jsonl`,
  109 files, 7,937,677 bytes in all. Most of that is STEP text inside
  `importStep` requests.
- Direct calls. Many source tests do not send every operation through
  `Bridge.Call`; they call the bridge method that a request dispatches to,
  such as `b.cutMany(…)` or `b.importSTEP(…)`. The owner decided on
  2026-11-18 that such a call is recorded as the equivalent request, and the
  replay on the default bridge shows that each one gives the recorded
  response. 653 of the 3,118 requests, in
  55 rows, were recorded this way
  (cutMany ×213, exportSTEP ×261, importSTEP ×163, mixedExtrude ×1, overlapVolume ×15); a trace line marks
  each with `direct`. Calls of constructors that have no request form are
  in no trace: curvedBoolean ×3, cutSideDualDisk ×1, putAnalyticBody ×3, in 4 rows.
- Checked responses: every response of a trace is compared on a general
  bridge (A3, A4) unless the trace line carries `unchecked` with the
  exclusion of section 15.2 that applies. The rows hold 2,685 checked
  responses and 433 unchecked (STEP request ×424; face-role request ×7; operand outside the surface class ×2).
- STEP. An `importStep` or `exportStep` request is issued and its response
  is not compared. A shape that `importStep` publishes is an ordinary
  operand afterwards: the import is the same legacy code on any bridge and
  the trace carries the STEP text. 194 Boolean responses in 30 rows take
  such an operand and are checked (owner decision, 2026-11-18).
- Analytic values. A `near(got, want)` pair of the source test is kept, as
  `analytic`, on the response whose volume, surface area or overlap value
  is bit for bit `got`. 603 pairs match one value of their bridge and are
  attached by that rule; 406 match none and are dropped. 765 match more than
  one response or property, and a reading of the source test decided
  each: the `got` expression at the call names the property
  (11 placed by that alone), and the response is the request made
  inside the expression (246), the latest request assigned to the
  variable it reads (458), or the requests behind the values a loop
  ranges over (22). 28 read a mesh, a face or a bounds value that only
  happens to equal a response, and are left out. 0 could not be placed.
- Operands: an operand counts as inside the surface class when it is the
  result of a checked Boolean request, which a general bridge builds itself,
  or when every face of its stored solid is a `geom.PlaneSurface`, a
  `geom.CylinderSurface` or the polar plane chart.
- `pocketFloor` (CO4) is issued by `TestNativeBoxPocketBoreAndNestedCut` (T1-076) only.
- Mesh checks (A3). The source tests share no helper for them. They apply,
  inline: to a `mesh` response, that it decodes as `Mesh` with positions and
  indices and carries the shape's `geometryAssertions`
  (`oblique_bore_test.go:80`, `bore_test.go:124`); to an `stl` response, that
  it reads back with `exchange.ReadSTL`, passes `mesh.Mesh.ValidateSolid`
  (`mesh/mesh.go:49`) and has the shape's volume by
  `mesh.Mesh.MassProperties` (`planar_boolean_chain_test.go:28`,
  `oblique_bore_test.go:104`). `TestGeneralBooleanPublication` applies the
  same checks.
- `slow legacy` marks a row whose source test is skipped in the default
  tier; it was recorded once with `PLAINSOLID_SLOW=1`, and its replay is
  skipped in the default tier too. There are 13.

## Rows

"Analytic" is the number of analytic values attached. "Ambiguous" is the
number of `near` pairs that matched more than one value, as placed by the
source + left out. "Direct" is the number of the row's requests recorded
from direct calls.

| Id | Function | File | Tier | Family | Boolean requests | Checked | Unchecked | Analytic | Ambiguous | Direct | Slow legacy | Volume error | Area error | Outcome |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| T1-001 | `TestThroughCircleCutAllAxesAndSigns` | `bore_test.go` | 1 | Through bore | cut ×6, common ×6 | 24 | face-role request ×6 | 18 | 6 + 0 | 0 |  | 1.2e-16 | 0 | pass |
| T1-002 | `TestFiniteCylinderCoverageAndPartialCommon` | `bore_test.go` | 1 | Through bore | cut ×6, common ×5 | 21 | 0 | 4 | 1 + 0 | 0 |  | 1.1e-16 | 1.4e-16 | pass |
| T1-003 | `TestBoreClonePlacementAndRepeatedCuts` | `bore_test.go` | 1 | Through bore | cut ×3 | 12 | 0 | 3 | 2 + 0 | 0 |  | 3e-16 | 0 | pass |
| T1-004 | `TestSingleBoreMeshKeepsVolumeWithScaleAwareSampling` | `bore_mesh_test.go` | 1 | Through bore | cut ×2 | 6 | 0 | 0 | 0 | 0 |  | 0 | 1.8e-16 | pass |
| T1-005 | `TestGuidedRadiusThree` | `blind_test.go` | 1 | Blind bore | common ×1 | 3 | 0 | 0 | 0 | 0 |  | 3.3e-09 | 4.4e-09 | pass |
| T1-006 | `TestBlindAllAxesAndSigns` | `blind_test.go` | 1 | Blind bore | cut ×6 | 18 | 0 | 12 | 0 | 0 |  | 1.2e-16 | 0 | pass |
| T1-007 | `TestBlindScopedRaisedStockAndMixedRecipe` | `blind_test.go` | 1 | Blind bore | cut ×2, common ×1 | 10 | 0 | 6 | 1 + 0 | 0 |  | 4.1e-15 | 6.2e-16 | pass |
| T1-008 | `TestRadiallyDisjointCommonAndUnsupportedCavity` | `blind_test.go` | 1 | Blind bore | cut ×1, common ×4 | 9 | 0 | 4 | 4 + 0 | 0 |  | 0 | 0 | pass |
| T1-009 | `TestBlindFloorNearExitReferenceIsAmbiguous` | `blind_test.go` | 1 | Blind bore | cut ×2 | 4 | 0 | 0 | 0 | 0 |  | 1.1e-10 | 0.015 | excluded |
| T1-010 | `TestMultipleBlindPocketsRetainSharedCaps` | `blind_test.go` | 1 | Blind bore | cut ×4 | 10 | 0 | 4 | 0 | 0 |  | 1.2e-16 | 0 | pass |
| T1-011 | `TestGuidedOppositeEndBlindToolAllAxes` | `blind_test.go` | 1 | Blind bore | cut ×6 | 18 | 0 | 12 | 0 | 0 |  | 1.8e-16 | 0 | pass |
| T1-012 | `TestExitOpenBlindRejectsUnpreservedEndpoints` | `blind_test.go` | 1 | Blind bore | cut ×2 | 5 | 0 | 0 | 0 | 0 |  | 0 | 0 | pass |
| T1-013 | `TestScopedHoleDefaultMeshVolume` | `blind_test.go` | 1 | Blind bore | cut ×1 | 3 | 0 | 0 | 0 | 0 |  | 1.3e-16 | 2e-16 | pass |
| T1-014 | `TestGuidedDefaultMeshRemovedVolumeAndNormalTurn` | `blind_test.go` | 1 | Blind bore | cut ×4 | 10 | 0 | 0 | 0 | 0 |  | 3.7e-16 | 2.6e-16 | pass |
| T1-015 | `TestExactBlindBoreTopologyDiagnostic` | `bore_contact_test.go` | 1 | Blind bore | cut ×1 | 7 | 0 | 0 | 0 | 0 |  | 0 | 0 | pass |
| T1-016 | `TestUnequalDepthBoreAxesSignsAndCaps` | `bore_layers_test.go` | 1 | Bore layers | cutMany ×12 | 48 | 0 | 24 | 0 | 0 |  | 1.3e-16 | 1.6e-16 | pending, RL-9: trimmed: curved trim refinement folds or degenerates in UV (2 checked responses without a value) |
| T1-017 | `TestSequentialHoleOppositeBlindEntryAndOverlapQuery` | `bore_layers_test.go` | 1 | Bore layers | cut ×12, cutMany ×6, overlapVolume ×18 | 60 | 0 | 30 | 12 + 0 | 0 |  | 7.7e-16 | 1.6e-16 | pass |
| T1-018 | `TestOverlapVolumeAtFullHandleCapacity` | `bore_layers_test.go` | 1 | Bore layers | cut ×1, overlapVolume ×2 | 258 | 0 | 1 | 0 | 0 |  | 3e-15 | 0 | pass |
| T1-019 | `TestOverlapVolumePreservesOrdinaryQueriesAndCommonGeometry` | `bore_layers_test.go` | 1 | Bore layers | cut ×1, common ×1, overlapVolume ×7 | 15 | 0 | 6 | 3 + 0 | 0 |  | 6.7e-15 | 0 | pass |
| T1-020 | `TestOverlappingHoleUnionAxesSignsAndDepths` | `bore_union_test.go` | 1 | Bore union | cutMany ×12 | 48 | 0 | 37 | 0 | 0 |  | 1.4e-16 | 0 | pass |
| T1-021 | `TestHoleUnionDuplicatesContainmentAndTripleOverlap` | `bore_union_test.go` | 1 | Bore union | cut ×3, cutMany ×4 | 16 | 0 | 7 | 3 + 0 | 0 |  | 3.6e-16 | 0 | pass |
| T1-022 | `TestHoleUnionUnsupportedBatchDoesNotPublish` | `bore_union_test.go` | 1 | Bore union | cutMany ×1 | 4 | 0 | 0 | 0 | 0 |  | 0 | 0 | pass |
| T1-023 | `TestContainedUnionAnchorChangeDoesNotPublishTinyReduction` | `bore_union_test.go` | 1 | Bore union | cut ×4, cutMany ×3 | 12 | 0 | 0 | 0 | 0 |  | 0 | 0 | pass |
| T1-024 | `TestThroughUnionOpposedAxesAndInputOrders` | `bore_union_test.go` | 1 | Bore union | cutMany ×4 | 16 | 0 | 8 | 0 | 0 |  | 3.6e-16 | 0 | pass |
| T1-025 | `TestOpposedBlindBoreMerge` | `opposed_bore_test.go` | 1 | Opposed blind | cut ×36 | 72 | 0 | 12 | 0 | 0 |  | 2.3e-16 | 0 | pass |
| T1-026 | `TestOpposedBlindBoreStrictOverlapAndUnrelated` | `opposed_bore_test.go` | 1 | Opposed blind | cutMany ×2 | 257 | 0 | 1 | 1 + 0 | 0 |  | 2.4e-16 | 1.7e-16 | pass |
| T1-027 | `TestOpposedBlindBoresIntersectAtDifferentDepths` | `opposed_blind_bore_test.go` | 1 | Opposed blind | cut ×6, cutMany ×6, overlapVolume ×9 | 30 | 0 | 21 | 27 + 12 | 0 |  | 2.7e-16 | 1.7e-16 | pass |
| T1-028 | `TestOpposedBlindBoresMeetingAtOneLevel` | `opposed_blind_bore_test.go` | 1 | Opposed blind | cutMany ×4 | 16 | 0 | 8 | 0 | 0 |  | 1.3e-16 | 3.2e-16 | pass |
| T1-029 | `TestObliqueThroughBoreScopeAndSTL` | `oblique_bore_test.go` | 1 | Oblique through | cut ×2, cutMany ×2, overlapVolume ×4 | 16 | STEP request ×6 | 12 | 10 + 0 | 8 |  | 9.1e-16 | 1.8e-16 | pending, RL-9: trimmed tessellation budget on a result) |
| T1-030 | `TestObliqueThroughBoreRejectsIncompleteOpeningsAtomically` | `oblique_bore_test.go` | 1 | Oblique through | cutMany ×3, overlapVolume ×3 | 10 | 0 | 0 | 0 | 6 |  | 0 | 0 | pass |
| T1-031 | `TestDisjointBoresSTEP` | `disjoint_bores_test.go` | 1 | Disjoint bores | cut ×3, cutMany ×1, overlapVolume ×4 | 13 | STEP request ×4 | 12 | 8 + 0 | 5 |  | 2.2e-16 | 0 | pass |
| T1-032 | `TestDisjointBlindBoresSTEP` | `disjoint_blind_bores_test.go` | 1 | Disjoint bores | cut ×3, cutMany ×1, overlapVolume ×6 | 14 | STEP request ×4 | 14 | 11 + 0 | 5 |  | 2.1e-16 | 3.1e-16 | pass |
| T1-033 | `TestFiniteDisjointBoresSTEP` | `disjoint_finite_bores_test.go` | 1 | Disjoint bores | cut ×3, cutMany ×1, overlapVolume ×6 | 14 | STEP request ×4 | 8 | 6 + 0 | 11 |  | 2.1e-16 | 5.3e-16 | pass |
| T1-034 | `TestRadialDisjointBoresSTEP` | `disjoint_radial_bores_test.go` | 1 | Disjoint bores | cut ×3, cutMany ×1, overlapVolume ×6 | 14 | STEP request ×4 | 14 | 10 + 0 | 5 |  | 2.2e-16 | 1.5e-16 | pass |
| T1-035 | `TestDisjointObliqueBoreScopeAndSTEP` | `disjoint_oblique_bore_test.go` | 1 | Disjoint bores | cut ×4, cutMany ×1, overlapVolume ×3 | 11 | STEP request ×2 | 7 | 4 + 0 | 3 |  | 6.8e-16 | 1.8e-16 | pass |
| T1-036 | `TestPlacedSideCrossingCircularCut` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×3, cutMany ×3 | 12 | 0 | 3 | 3 + 2 | 0 |  | 0 | 1.1e-16 | pass |
| T1-037 | `TestPlacedSideCrossingBlindCylinderCut` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×24, cutMany ×24, overlapVolume ×48 | 126 | 0 | 144 | 108 + 0 | 0 |  | 4.9e-14 | 1.9e-16 | pass |
| T1-038 | `TestPlainCADSideNotchSTEPRoundTrip` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cutMany ×4 | 20 | STEP request ×24 | 24 | 17 + 0 | 28 |  | 1.5e-16 | 1.6e-16 | pass |
| T1-039 | `TestCornerBlindCutQualifiesEitherToolEndpoint` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×2, cutMany ×1, overlapVolume ×2 | 8 | STEP request ×2 | 2 | 1 + 0 | 2 |  | 2e-16 | 0 | pass |
| T1-040 | `TestNotchedStockSTEPUnitsRetainRecipe` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cutMany ×1 | 2 | STEP request ×2 | 2 | 0 | 3 |  | 3e-16 | 1.9e-16 | pass |
| T1-041 | `TestSideNotchMixedBoreBatchIsAtomic` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×4, cutMany ×20, overlapVolume ×60 | 116 | STEP request ×28 | 112 | 93 + 0 | 48 | slow legacy | 1.6e-16 | 1.6e-16 | excluded (4 checked responses without a value) |
| T1-042 | `TestSideNotchRetainsConnectedCoaxialCuts` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×4, cutMany ×8, overlapVolume ×8 | 30 | STEP request ×6 | 20 | 16 + 0 | 14 |  | 4.2e-15 | 1.3e-16 | pass |
| T1-043 | `TestNotchedStockMergesInteriorBoresInAnyOrder` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×1, cutMany ×6, overlapVolume ×1 | 13 | STEP request ×4 | 11 | 6 + 0 | 10 |  | 3.8e-16 | 0 | pass |
| T1-044 | `TestMultiBodyFiniteCornerRevolveCuts` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×6 | 14 | 0 | 5 | 2 + 2 | 0 |  | 1.2e-16 | 0 | pass |
| T1-045 | `TestMultiBodyExtrudeDisjointNoop` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×1 | 3 | 0 | 0 | 0 | 0 |  | 0 | 0 | pass |
| T1-046 | `TestCornerThroughRejectsFiniteShortTool` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×1 | 3 | 0 | 0 | 0 | 0 |  | 0 | 0 | pass |
| T1-047 | `TestCornerThroughReversedFiniteAxis` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×1 | 3 | 0 | 1 | 0 | 0 |  | 0 | 0 | pass |
| T1-048 | `TestSideOpeningPairScopedAndSuccessive` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×4, cutMany ×12, overlapVolume ×16 | 56 | STEP request ×24 | 40 | 28 + 0 | 36 |  | 6.9e-15 | 1.6e-16 | pass |
| T1-049 | `TestSideOpeningPairOpposedScopes` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×2, cutMany ×4, overlapVolume ×6 | 15 | STEP request ×4 | 10 | 6 + 0 | 8 |  | 4.2e-15 | 1.6e-16 | pass |
| T1-050 | `TestSideOpeningPairDifferentStockSides` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×4, cutMany ×8, overlapVolume ×4 | 22 | 0 | 12 | 6 + 0 | 8 |  | 4.1e-15 | 1.6e-16 | pass |
| T1-051 | `TestSideOpeningCutsRetainHistoryAndAcceptFurtherHoles` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×1, cutMany ×4, overlapVolume ×4 | 16 | STEP request ×3 | 7 | 7 + 0 | 7 |  | 5.6e-16 | 1.3e-16 | pass |
| T1-052 | `TestSideOpeningCutsContinueStrictDualDisk` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×1, cutMany ×2, overlapVolume ×1 | 8 | STEP request ×1 | 3 | 4 + 0 | 3 |  | 3.1e-16 | 2.5e-16 | pass |
| T1-053 | `TestIntersectingCornerOpeningHoles` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×12, cutMany ×3, overlapVolume ×12 | 36 | STEP request ×18 | 23 | 18 + 0 | 21 |  | 3.2e-15 | 1.6e-16 | pass |
| T1-054 | `TestConcealedHoleSTEPAndFurtherCut` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×2, cutMany ×2, overlapVolume ×2 | 11 | STEP request ×4 | 3 | 0 | 6 |  | 3.1e-15 | 1.6e-16 | pass |
| T1-055 | `TestPlainCADSideOpenDualDiskCuts` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×6, cutMany ×6, overlapVolume ×20 | 40 | STEP request ×10 | 46 | 36 + 2 | 10 | slow legacy | 5.1e-16 | 1.9e-16 | pass |
| T1-056 | `TestPlainCADUnequalSideOpenDualDiskCuts` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×3, cutMany ×2, overlapVolume ×4 | 12 | STEP request ×3 | 10 | 6 + 2 | 3 | slow legacy | 8.1e-16 | 1.2e-16 | pass |
| T1-057 | `TestPlainCADOffCenterSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×2, cutMany ×1, overlapVolume ×1 | 7 | STEP request ×2 | 3 | 1 + 1 | 2 | slow legacy | 6.1e-16 | 1.1e-16 | pass |
| T1-058 | `TestPlainCADSideOpeningFallbackAtNonbinaryCrossing` | `side_dual_disk_test.go` | 1 | Side dual disk | cutMany ×1 | 4 | STEP request ×1 | 2 | 1 + 0 | 2 |  | 1.8e-16 | 0 | pass |
| T1-059 | `TestPlainCADExactWallSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×4, cutMany ×2 | 14 | STEP request ×6 | 10 | 6 + 2 | 6 | slow legacy | 1.7e-16 | 2.9e-16 | pass |
| T1-060 | `TestPlainCADOffsetSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×4, cutMany ×2, overlapVolume ×6 | 18 | STEP request ×4 | 18 | 10 + 0 | 4 | slow legacy | 7.3e-15 | 3.4e-16 | pass |
| T1-061 | `TestPlainCADUnequalCenterSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×4, cutMany ×2, overlapVolume ×6 | 18 | STEP request ×4 | 18 | 10 + 0 | 4 | slow legacy | 7.4e-15 | 7.7e-15 | pass |
| T1-062 | `TestCircularStockFiniteEccentricHoleAndSTEP` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×4, cutMany ×4, overlapVolume ×8 | 24 | STEP request ×8 | 24 | 12 + 0 | 8 |  | 3.8e-16 | 0 | pass |
| T1-063 | `TestCircularStockGroupedAndSuccessiveDisjointBores` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×3, cutMany ×5, overlapVolume ×7 | 22 | STEP request ×2 | 13 | 6 + 0 | 7 |  | 3.8e-16 | 2e-16 | pass |
| T1-064 | `TestCircularStockAuthoredEccentricInnerWireAndSignedCut` | `circular_stock_bore_test.go` | 1 | Circular stock | cutMany ×2, overlapVolume ×1 | 6 | 0 | 3 | 1 + 0 | 3 |  | 4.4e-16 | 4.6e-16 | pass |
| T1-065 | `TestCircularStockDisjointOpposedBoresAndSTEP` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×2, cutMany ×4, overlapVolume ×26 | 40 | STEP request ×8 | 42 | 34 + 0 | 12 |  | 2.3e-16 | 1.8e-16 | pass |
| T1-066 | `TestCircularStockOverlappingOpposedAndSequentialBores` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×2, cutMany ×8, overlapVolume ×14 | 28 | STEP request ×8 | 34 | 28 + 0 | 16 |  | 2.9e-16 | 3.5e-16 | pass |
| T1-067 | `TestCircularStockCoaxialOpposedSTEPAndThroughCut` | `circular_stock_bore_test.go` | 1 | Circular stock | cutMany ×2, overlapVolume ×2 | 8 | STEP request ×2 | 4 | 2 + 0 | 4 |  | 1.4e-16 | 1.9e-16 | pass |
| T1-068 | `TestFullTubeSTEPAndCoaxialCut` | `tube_step_test.go` | 1 | Coaxial cut | cut ×3 | 6 | STEP request ×5; operand outside the surface class ×1 | 2 | 2 + 0 | 5 |  | 3.4e-16 | 0 | pass |
| T1-069 | `TestCylinderSectorSTEPBridge` | `cylinder_sector_step_test.go` | 1 | Coaxial cut | cut ×1 | 4 | STEP request ×6 | 3 | 2 + 0 | 6 |  | 1.7e-16 | 0 | pass |
| T1-070 | `TestQuarterAnnularSTEPAndFurtherCut` | `quarter_annular_step_test.go` | 1 | Coaxial cut | cut ×3 | 6 | STEP request ×4; operand outside the surface class ×1 | 1 | 1 + 0 | 4 |  | 1.1e-16 | 0 | pass |
| T1-071 | `TestCoaxialDispatchPlacementAndMeshBudget` | `coaxial_join_test.go` | 1 | Coaxial join | fuse ×2 | 7 | 0 | 0 | 0 | 0 |  | 1.8e-16 | 1.5e-16 | pass |
| T1-072 | `TestScopedJoinPublishesOnlyConnectedAddedMaterial` | `join_all_test.go` | 1 | Join all | joinAll ×4 | 14 | 0 | 6 | 5 + 0 | 0 |  | 0 | 0 | pass |
| T1-073 | `TestScopedJoinWithContainingRevolvedTool` | `join_all_test.go` | 1 | Join all | joinAll ×1 | 7 | 0 | 2 | 2 + 0 | 0 |  | 1.8e-16 | 3.3e-16 | pass |
| T1-074 | `TestContainingCylinderOverlapCapture` | `cylinder_containment_overlap_test.go` | 1 | Containment overlap | overlapVolume ×3 | 6 | 0 | 0 | 0 | 3 |  | 2.3e-16 | 0 | pass |
| T1-075 | `TestRevolveJoinCaptureContainedPlanarVolume` | `overlap_capture_test.go` | 1 | Containment overlap | overlapVolume ×3 | 6 | 0 | 3 | 2 + 0 | 3 |  | 2.3e-16 | 0 | pass |
| T1-076 | `TestNativeBoxPocketBoreAndNestedCut` | `box_pocket_test.go` | 1 | Pocket and planar | cut ×2, cutMany ×1, pocketFloor ×2 | 9 | face-role request ×1 | 3 | 0 | 0 |  | 0 | 2e-16 | pass |
| T1-077 | `TestScopedRotatedPocketCopiesOnPocketedStock` | `box_pocket_test.go` | 1 | Pocket and planar | cut ×4, cutMany ×1, overlapVolume ×3 | 13 | 0 | 4 | 2 + 3 | 0 |  | 7.3e-16 | 0 | pass |
| T1-078 | `TestPatternOverlapUsesFiniteCylinderLens` | `box_pocket_test.go` | 1 | Pocket and planar | overlapVolume ×2 | 4 | 0 | 1 | 0 + 1 | 0 |  | 1.6e-16 | 0 | pass |
| T1-079 | `TestPlanarPocketAfterFourDisjointBores` | `box_pocket_test.go` | 1 | Pocket and planar | cut ×1, cutMany ×1 | 8 | 0 | 1 | 0 | 0 |  | 4.7e-16 | 6.5e-16 | pass |
| T1-080 | `TestCutManyPlanarAtomicAndRedundantTools` | `cut_many_test.go` | 1 | Pocket and planar | cutMany ×4 | 8 | 0 | 2 | 2 + 0 | 0 |  | 0 | 0 | pass |
| T1-081 | `TestCutManyValidationDoesNotPublish` | `cut_many_test.go` | 1 | Pocket and planar | cutMany ×4 | 8 | 0 | 0 | 0 | 0 |  | 0 | 0 | pass |
| T1-082 | `TestExtrudeDialogJoinThenCutExactVolume` | `planar_boolean_chain_test.go` | 1 | Pocket and planar | cut ×1, fuse ×1 | 8 | 0 | 5 | 4 + 1 | 0 |  | 4.3e-16 | 0 | pass |
| T1-083 | `TestPocketFloorCircleCutsAndCounterbore` | `pocket_cylinder_cut_test.go` | 1 | Pocket with cylinder | cut ×6, cutMany ×3 | 21 | 0 | 3 | 0 | 0 |  | 1.5e-16 | 0 | pass |
| T1-084 | `TestRetainedPlanarBoreFillRoofAndRedrill` | `retained_planar_bores_test.go` | 1 | Pocket with cylinder | cut ×7, fuse ×3, cutMany ×3 | 27 | 0 | 7 | 5 + 0 | 0 | slow legacy | 0 | 0 | pass |
| T1-085 | `TestRetainedPlanarBoreProofRejectsUnsafeDomains` | `retained_planar_bores_test.go` | 1 | Pocket with cylinder | cut ×2, fuse ×1 | 10 | 0 | 0 | 0 | 0 |  | 0 | 0 | pass |
| T1-086 | `TestIslandPocketFiniteSignedCaps` | `island_pocket_test.go` | 1 | Pocket with cylinder | cut ×24 | 72 | 0 | 0 | 0 | 0 |  | 1.7e-16 | 0 | pass |
| T1-087 | `TestToFaceFiniteCoverageAndHoles` | `to_face_test.go` | 1 | To face | prismToFace ×2, circleToFace ×6 | 10 | 0 | 4 | 1 + 0 | 0 |  | 1.1e-15 | 0 | pass |
| T1-088 | `TestToFaceCircleSignedAxesRetainsAnalyticSolid` | `to_face_test.go` | 1 | To face | circleToFace ×6 | 12 | 0 | 12 | 0 | 0 |  | 3.4e-16 | 1.9e-16 | pass |
| T1-089 | `TestToFacePolygonObliqueAndBoundary` | `to_face_test.go` | 1 | To face | prismToFace ×3 | 5 | 0 | 2 | 0 | 0 |  | 1.8e-16 | 6.4e-16 | pass |
| T1-090 | `TestToFaceAnalyticCircularHoleCoverage` | `to_face_analytic_test.go` | 1 | To face | prismToFace ×2 | 3 | 0 | 1 | 0 | 0 |  | 0 | 0 | pass |
| T1-091 | `TestToFaceAnalyticCircleHoleExactGap` | `to_face_analytic_test.go` | 1 | To face | circleToFace ×2 | 3 | 0 | 1 | 0 | 0 |  | 0 | 0 | excluded (1 checked responses without a value) |
| T1-092 | `TestObliqueToFaceUsesCircularExtent` | `to_face_oblique_test.go` | 1 | To face | circleToFace ×3 | 5 | 0 | 1 | 0 | 0 |  | 1.4e-15 | 7.6e-16 | pending, RL-9: trimmed tessellation budget on a result) |
| T2-001 | `TestCrossedBoreScopeAndSTEP` | `crossed_bore_test.go` | 2 | Crossed bores | cut ×2, cutMany ×3, overlapVolume ×4 | 12 | STEP request ×9 | 10 | 8 + 0 | 12 |  | 1.1e-16 | 1.6e-16 | Tier 2, not supported (8 checked responses without a value) |
| T2-002 | `TestBlindCrossedBoreScopeAndSTEP` | `blind_crossed_bore_test.go` | 2 | Crossed bores | cut ×2, cutMany ×2, overlapVolume ×3 | 10 | STEP request ×6 | 7 | 5 + 0 | 8 |  | 1.1e-16 | 1.6e-16 | Tier 2, not supported (6 checked responses without a value) |
| T2-003 | `TestDoubleBlindCrossedBoreScopeAndSTEP` | `double_blind_crossed_bore_test.go` | 2 | Crossed bores | cut ×8, cutMany ×4, overlapVolume ×8 | 32 | STEP request ×8 | 24 | 16 + 0 | 12 |  | 0 | 1.6e-16 | Tier 2, not supported (16 checked responses without a value) |
| T2-004 | `TestContainedCrossedBores` | `contained_crossed_bores_test.go` | 2 | Crossed bores | cut ×1, cutMany ×4, overlapVolume ×2 | 13 | STEP request ×2 | 5 | 0 | 6 |  | 0 | 0 | Tier 2, not supported (6 checked responses without a value) |
| T2-005 | `TestClippedCrossedBoreScopeAndSTEP` | `clipped_crossed_bore_test.go` | 2 | Crossed bores | cut ×12, cutMany ×6, overlapVolume ×12 | 48 | STEP request ×12 | 36 | 24 + 0 | 18 | slow legacy | 0 | 1.3e-16 | Tier 2, not supported (24 checked responses without a value) |
| T2-006 | `TestTripleCrossedBoreScopeAndSTEP` | `triple_crossed_bore_test.go` | 2 | Triple and companion | cut ×3, cutMany ×1, overlapVolume ×4 | 12 | STEP request ×2 | 6 | 4 + 0 | 3 |  | 0 | 0 | Tier 2, not supported (7 checked responses without a value) |
| T2-007 | `TestBlindTripleCrossedBoreScopeAndSTEP` | `blind_triple_crossed_bore_test.go` | 2 | Triple and companion | cutMany ×6, overlapVolume ×18 | 48 | STEP request ×12 | 42 | 24 + 0 | 18 | slow legacy | 0 | 0 | Tier 2, not supported (24 checked responses without a value) |
| T2-008 | `TestMultipleBlindTripleCrossedBoreScopeAndSTEP` | `blind_triple_crossed_bore_test.go` | 2 | Triple and companion | cut ×11, cutMany ×4, overlapVolume ×11 | 40 | STEP request ×8 | 27 | 18 + 0 | 12 | slow legacy | 1.2e-16 | 1.8e-16 | Tier 2, not supported (23 checked responses without a value) |
| T2-009 | `TestCrossedCompanionBoreScopeAndSTEP` | `crossed_companion_bore_test.go` | 2 | Triple and companion | cut ×3, cutMany ×1, overlapVolume ×2 | 10 | STEP request ×2 | 6 | 4 + 0 | 3 |  | 1.2e-16 | 1.8e-16 | Tier 2, not supported (5 checked responses without a value) |
| T2-010 | `TestCrossedObliqueBoreScope` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×2, cutMany ×1, overlapVolume ×3 | 9 | STEP request ×2 | 7 | 3 + 0 | 3 |  | 1.5e-16 | 0 | Tier 2, not supported (5 checked responses without a value) |
| T2-011 | `TestCrossedObliqueReciprocalScope` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×8, cutMany ×2, overlapVolume ×6 | 24 | STEP request ×4 | 14 | 6 + 0 | 6 |  | 1.5e-16 | 1.2e-16 | Tier 2, not supported (12 checked responses without a value) |
| T2-012 | `TestCrossedObliquePairWithSeparatedOppositeZOpenings` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cutMany ×1 | 4 | 0 | 1 | 0 | 1 |  | 1.5e-16 | 1.3e-16 | pass |
| T2-013 | `TestCrossedObliqueBoreCoordinatePlacements` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cutMany ×32, overlapVolume ×32 | 160 | STEP request ×96 | 64 | 0 | 128 | slow legacy | 0 | 0 | Tier 2, not supported (64 checked responses without a value) |
| T2-014 | `TestBlindCrossedObliqueBoreSTEPAndFurtherCut` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×2, cutMany ×14, overlapVolume ×8 | 38 | STEP request ×14 | 12 | 0 | 28 | slow legacy | 1.5e-16 | 1.2e-16 | Tier 2, not supported (18 checked responses without a value) |
| T2-015 | `TestConcealedCrossedObliqueBoreSTEPCanonical` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cutMany ×14, overlapVolume ×12 | 42 | STEP request ×10 | 12 | 10 + 0 | 24 |  | 0 | 0 | Tier 2, not supported (18 checked responses without a value) |
| T2-016 | `TestObliqueFloorCrossedBoreReimport` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cutMany ×6, overlapVolume ×4 | 18 | STEP request ×10 | 12 | 4 + 0 | 16 |  | 0 | 0 | Tier 2, not supported (10 checked responses without a value) |
| T2-017 | `TestDoubleBlindCrossedObliqueBore` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×2, cutMany ×6, overlapVolume ×8 | 24 | STEP request ×10 | 16 | 6 + 0 | 16 |  | 1.5e-16 | 1.2e-16 | Tier 2, not supported (14 checked responses without a value) |

The trace of a row is the file named after its function.

### A8 summary

From one run of `TestGeneralBooleanAcceptance` on commit `1ae12376`, with
`PLAINSOLID_SLOW=1` so that every row runs. An error is
|value − expected| / max(1, |expected|), with the analytic value as
expected where the source test has one and the legacy value otherwise.
Every row produced at least one value.

- Tier 1: 86 of 92 rows pass. Three are pending on a limit of
  `brep/trimmed` (RL-9): T1-016, T1-029, T1-092. Three are excluded:
  T1-009 and T1-091 (RL-12), T1-041 (S4).
- Over the 86 rows that pass, the largest volume error is 3.3e-9 and the
  largest area error 4.4e-9, both in T1-005, where a cap lies exactly the
  working tolerance from a face. No passing row is above 1e-8 in either;
  every other passing row is at 1e-14 or below.
- Over all 109 rows, one is above 1e-8: T1-009, excluded, with an area
  error of 0.015.
- Tier 2 is not supported: 1 of 17 rows passes (T2-012).

## Dropped

Dropped for a reason in section 15.2: every Boolean response of the function
is unchecked.

| Function | File | Tier | Family | Reason |
|---|---|---|---|---|
| `TestPlainCADRoundedHousingJoinPocketAndThroughProfiles` | `rounded_stack_test.go` | 1 | Rounded profiles | The stock gets a cap fillet or chamfer before the first Boolean request, so every Boolean operand is edge-treated stock with torus or cone faces (15.2, edge-treated stock before the first Boolean). cut ×8, fuse ×1, cutMany ×4. |
| `TestPlainCADRotaryPlatterSlotsAndBores` | `rotary_platter_test.go` | 1 | Rounded profiles | The stock gets a cap fillet or chamfer before the first Boolean request, so every Boolean operand is edge-treated stock with torus or cone faces (15.2, edge-treated stock before the first Boolean). cut ×8, cutMany ×2. |
| `TestPlainCADAnnularPedestalAndThroughBores` | `annular_pedestal_test.go` | 1 | Rounded profiles | The stock gets a cap fillet or chamfer before the first Boolean request, so every Boolean operand is edge-treated stock with torus or cone faces (15.2, edge-treated stock before the first Boolean). cut ×1, fuse ×1, cutMany ×1, overlapVolume ×1. |

## Not in the acceptance set

Section 15.1 takes the functions that issue a Boolean operation. These issue
none that has a request form, so they are not cases.

| Function | File | Tier | Family | What it does instead |
|---|---|---|---|---|
| `TestAngularMappingUnderflowDoesNotDisableChecks` | `blind_test.go` | 1 | Blind bore | calls kernel or bridge helpers only; no bridge operation |
| `TestExactBlindBoreNoProofForObliqueFrame` | `bore_contact_test.go` | 1 | Blind bore | 1 request, none Boolean (through Bridge.Call) |
| `TestOpposedBlindBoreRejectsDifferentSupport` | `opposed_bore_test.go` | 1 | Opposed blind | 4 requests, none Boolean (through Bridge.Call) |
| `TestObliqueFiniteCapContactProof` | `oblique_bore_test.go` | 1 | Oblique through | calls kernel or bridge helpers only; no bridge operation |
| `TestPlainCADSideOpenDualDiskSTEPUnits` | `side_dual_disk_test.go` | 1 | Side dual disk | 1 request, none Boolean (importSTEP ×1) |
| `TestSTEPIndependentlyAuthoredRoundBlindAndThroughBores` | `circular_stock_bore_test.go` | 1 | Circular stock | 2 requests, none Boolean (exportSTEP ×1, importSTEP ×1) |
| `TestFullTubeSTEPIndependentAndUnits` | `tube_step_test.go` | 1 | Coaxial cut | 4 requests, none Boolean (exportSTEP ×2, importSTEP ×2) |
| `TestCylinderSectorSTEPBridgeIndependentAndUnits` | `cylinder_sector_step_test.go` | 1 | Coaxial cut | 4 requests, none Boolean (exportSTEP ×2, importSTEP ×2) |
| `TestQuarterAnnularSTEPMillimeterNormalization` | `quarter_annular_step_test.go` | 1 | Coaxial cut | 2 requests, none Boolean (exportSTEP ×1, importSTEP ×1) |
| `TestCoaxialIntervalUnion` | `coaxial_join_test.go` | 1 | Coaxial join | calls fuseCoaxial ×24 directly, a constructor with no request of its own; 36 requests, none Boolean (circle ×36) |
| `TestCoaxialIntervalUnionContainmentTouchAndAtomicReject` | `coaxial_join_test.go` | 1 | Coaxial join | calls fuseCoaxial ×5 directly, a constructor with no request of its own; 6 requests, none Boolean (circle ×6) |
| `TestCoaxialDecimalTranslatedJoin` | `coaxial_join_test.go` | 1 | Coaxial join | calls fuseCoaxial ×1 directly, a constructor with no request of its own; 2 requests, none Boolean (circle ×2) |
| `TestIslandPocketFiniteRejectsAreAtomic` | `island_pocket_test.go` | 1 | Pocket with cylinder | calls rectangularIslandPocket ×4 directly, a constructor with no request of its own; 5 requests, none Boolean (through Bridge.Call) |
| `TestToFaceExactDiskSeparation` | `to_face_analytic_test.go` | 1 | To face | calls kernel or bridge helpers only; no bridge operation |

## Counts

| Tier | Family | Survey (15.1) | Functions in the files | Frozen | Dropped (15.2) | Not in the set |
|---|---|---|---|---|---|---|
| 1 | Through bore | 5 | 4 | 4 | 0 | 0 |
| 1 | Blind bore | 13 | 13 | 11 | 0 | 2 |
| 1 | Bore layers | 4 | 4 | 4 | 0 | 0 |
| 1 | Bore union | 5 | 5 | 5 | 0 | 0 |
| 1 | Opposed blind | 5 | 5 | 4 | 0 | 1 |
| 1 | Oblique through | 3 | 3 | 2 | 0 | 1 |
| 1 | Disjoint bores | 5 | 5 | 5 | 0 | 0 |
| 1 | Side and corner cylinder | 12 | 12 | 12 | 0 | 0 |
| 1 | Side opening pair | 7 | 7 | 7 | 0 | 0 |
| 1 | Side dual disk | 8 | 8 | 7 | 0 | 1 |
| 1 | Circular stock | 7 | 7 | 6 | 0 | 1 |
| 1 | Coaxial cut | 4 | 6 | 3 | 0 | 3 |
| 1 | Coaxial join | 4 | 4 | 1 | 0 | 3 |
| 1 | Join all | 2 | 2 | 2 | 0 | 0 |
| 1 | Containment overlap | 2 | 2 | 2 | 0 | 0 |
| 1 | Pocket and planar | 9 | 7 | 7 | 0 | 0 |
| 1 | Pocket with cylinder | 5 | 5 | 4 | 0 | 1 |
| 1 | Rounded profiles | excluded (15.2) | 3 | 0 | 3 | 0 |
| 1 | To face | 7 | 7 | 6 | 0 | 1 |
| 2 | Crossed bores | 5 | 5 | 5 | 0 | 0 |
| 2 | Triple and companion | 4 | 4 | 4 | 0 | 0 |
| 2 | Crossed oblique | 8 | 8 | 8 | 0 | 0 |
| 1 | **All of tier 1** | **107** | **109** | **92** | **3** | **14** |
| 2 | **All of tier 2** | **17** | **17** | **17** | **0** | **0** |

Checked Boolean responses by the request that made an operand (section 11
of the specification, stored forms). A response with operands from two of
these requests counts under both. "Directly" leaves out operands that reach
the Boolean through `clone` or `transform`.

| Request | Checked Boolean responses with an operand made by it | Directly |
|---|---|---|
| `box` | 528 | 525 |
| `prism` | 44 | 44 |
| `circle` | 1114 | 1114 |
| `mixedExtrude` | 31 | 31 |

Boolean responses left unchecked because an operand has faces stored as
spline patches (S2, the arc-loop branch of `mixedExtrude`): 0. No frozen
row makes such a body: 0 of the 27 `mixedExtrude` requests in the rows
stores a spline patch.

## Oracle run

One run of `TestGeneralBooleanOracle` on commit `1ae12376`, with the
helper built from `tools/occtref/general_boolean.cxx` against the pinned
OCCT. Bounds: 3e-7 relative for volume and area, 3e-7 absolute for the
centroid, equal solid counts, and equal face counts for the section 11
cases that state one.

88 cases, 257 steps: all agree. 63 steps have an empty result on both
sides. Largest differences over the other 194: 1.7e-9 relative in volume
and 2.7e-8 in the centroid (the first step of the Blind bore case, the
operands of T1-005), 5.4e-15 relative in area. All 47 section 11 cases
that state a face count agree with it, the three unions that M6 divides
after subtracting their dividers.

A case is a list of primitives and Boolean steps. The volumes and areas
below are those of its last step; the differences are the largest over
its steps, relative for volume and area together, absolute for the
centroid. Faces are the kernel's count and the oracle's.

Not compared: the To face family (its tool is not a box, a prism or a
cylinder), C27 (one operand), and the leaves of C24 to C26 (tolerance
rows, by A6). A family case holds the first bridge of the first row of
its family, up to the first request that the primitives cannot express.

| Case | Steps | Kernel volume | OCCT volume | Kernel area | OCCT area | Largest relative difference | Largest centroid difference | Faces | Result |
|---|---|---|---|---|---|---|---|---|---|
| section11/C1-planar/union | 1 | 9 | 9 | 30 | 30 | 0 | 0 | 12 / 12 | pass |
| section11/C1-planar/cut | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 0 | 6 / 6 | pass |
| section11/C2-planar/union | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 0 | 6 / 6 | pass |
| section11/C2-planar/common | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 0 | 6 / 6 | pass |
| section11/C3-planar/union | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 0 | 6 / 6 | pass |
| section11/C3-planar/common | 1 | 1 | 1 | 6 | 6 | 2.2e-16 | 0 | 6 / 6 | pass |
| section11/C3-planar/cut | 1 | 7 | 7 | 30 | 30 | 5.1e-16 | 2.2e-16 | 12 / 12 | pass |
| section11/C4-planar/union | 1 | 12 | 12 | 32 | 32 | 3e-16 | 2.2e-16 | 6 / 6 | pass |
| section11/C4-planar/cut | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 0 | 6 / 6 | pass |
| section11/C5-planar/union | 1 | 12 | 12 | 36 | 36 | 3e-16 | 6.4e-16 |  | pass |
| section11/C5-planar/cut | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 2.2e-16 | 6 / 6 | pass |
| section11/C6-planar/union | 1 | 8 | 8 | 24 | 24 | 0 | 0 | 6 / 6 | pass |
| section11/C6-planar/common | 1 | 1 | 1 | 6 | 6 | 2.2e-16 | 0 | 6 / 6 | pass |
| section11/C6-planar/cut | 1 | 7 | 7 | 24 | 24 | 1.3e-16 | 3.1e-16 | 9 / 9 | pass |
| section11/C7-planar/cut | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 0 | 6 / 6 | pass |
| section11/C8-planar/cut | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 0 | 6 / 6 | pass |
| section11/C9-planar/cut | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 2.2e-16 | 6 / 6 | pass |
| section11/C10-planar/cut | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 2.2e-16 | 6 / 6 | pass |
| section11/C28-planar/union | 1 | 410.125 | 410.125 | 351.25 | 351.25 | 1.6e-16 | 2.2e-15 | 14 / 12 | pass |
| section11/C28-planar/common | 1 | 43.875 | 43.875 | 78.75 | 78.75 | 1.1e-15 | 6.1e-15 | 8 / 8 | pass |
| section11/C1-cyl/union | 1 | 7.06858347058 | 7.06858347058 | 23.5619449019 | 23.5619449019 | 0 | 1.3e-16 | 6 / 6 | pass |
| section11/C1-cyl/cut | 1 | 6.28318530718 | 6.28318530718 | 18.8495559215 | 18.8495559215 | 1.4e-16 | 2.2e-16 | 3 / 3 | pass |
| section11/C2-cyl/union | 1 | 6.28318530718 | 6.28318530718 | 18.8495559215 | 18.8495559215 | 1.4e-16 | 2.2e-16 | 3 / 3 | pass |
| section11/C2-cyl/common | 1 | 6.28318530718 | 6.28318530718 | 18.8495559215 | 18.8495559215 | 1.4e-16 | 2.2e-16 | 3 / 3 | pass |
| section11/C3-cyl/union | 1 | 6.28318530718 | 6.28318530718 | 18.8495559215 | 18.8495559215 | 1.4e-16 | 2.2e-16 | 3 / 3 | pass |
| section11/C3-cyl/common | 1 | 0.785398163397 | 0.785398163397 | 4.71238898038 | 4.71238898038 | 1.4e-16 | 2e-16 | 3 / 3 | pass |
| section11/C3-cyl/cut | 1 | 5.49778714378 | 5.49778714378 | 23.5619449019 | 23.5619449019 | 4.8e-16 | 6.5e-16 | 6 / 6 | pass |
| section11/C11-cyl/cut | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 2.2e-16 | 6 / 6 | pass |
| section11/C12-cyl/union | 1 | 8 | 8 | 24 | 24 | 1.1e-16 | 1.6e-16 | 6 / 6 | pass |
| section11/C12-cyl/common | 1 | 0.785398163397 | 0.785398163397 | 4.71238898038 | 4.71238898038 | 1.4e-16 | 2.3e-16 | 3 / 3 | pass |
| section11/C13-cyl/cut | 1 | 6.28318530718 | 6.28318530718 | 18.8495559215 | 18.8495559215 | 1.4e-16 | 2.2e-16 | 3 / 3 | pass |
| section11/C14-cyl/union | 1 | 6.28318530718 | 6.28318530718 | 18.8495559215 | 18.8495559215 | 0 | 2.4e-16 | 3 / 3 | pass |
| section11/C14-cyl/common | 1 | 1.57079632679 | 1.57079632679 | 7.85398163397 | 7.85398163397 | 2.8e-16 | 2.5e-16 | 3 / 3 | pass |
| section11/C15-cyl/union | 1 | 9.42477796077 | 9.42477796077 | 25.1327412287 | 25.1327412287 | 0 | 2.6e-16 | 3 / 3 | pass |
| section11/C15-cyl/common | 1 | 3.14159265359 | 3.14159265359 | 12.5663706144 | 12.5663706144 | 1.4e-16 | 3.3e-16 | 3 / 3 | pass |
| section11/C15-cyl/cut | 1 | 3.14159265359 | 3.14159265359 | 12.5663706144 | 12.5663706144 | 1.4e-16 | 2.2e-16 | 3 / 3 | pass |
| section11/C16-cyl/union | 1 | 12.5663706144 | 12.5663706144 | 31.4159265359 | 31.4159265359 | 1.4e-16 | 3.1e-16 | 3 / 3 | pass |
| section11/C16-cyl/cut | 1 | 6.28318530718 | 6.28318530718 | 18.8495559215 | 18.8495559215 | 1.4e-16 | 2.2e-16 | 3 / 3 | pass |
| section11/C17-cyl/union | 1 | 7.85398163397 | 7.85398163397 | 25.1327412287 | 25.1327412287 | 2.3e-16 | 3.4e-16 |  | pass |
| section11/C17-cyl/common | 1 | 1.57079632679 | 1.57079632679 | 7.85398163397 | 7.85398163397 | 4.2e-16 | 2.3e-16 |  | pass |
| section11/C17-cyl/cut | 1 | 4.71238898038 | 4.71238898038 | 23.5619449019 | 23.5619449019 | 1.9e-16 | 3e-16 | 4 / 4 | pass |
| section11/C18-cyl/union | 1 | 10.1096312171 | 10.1096312171 | 26.8647920363 | 26.8647920363 | 1.4e-15 | 8.9e-16 |  | pass |
| section11/C18-cyl/common | 1 | 2.45673939722 | 2.45673939722 | 10.8343198068 | 10.8343198068 | 2e-15 | 2.5e-16 |  | pass |
| section11/C18-cyl/cut | 1 | 3.82644590996 | 3.82644590996 | 16.3928165243 | 16.3928165243 | 4.8e-15 | 1.4e-15 |  | pass |
| section11/C19-cyl/union | 1 | 8.7853981634 | 8.7853981634 | 27.1415926536 | 27.1415926536 | 0 | 3.8e-16 | 8 / 8 | pass |
| section11/C19-cyl/cut | 1 | 8 | 8 | 24 | 24 | 1.1e-16 | 1.6e-16 | 6 / 6 | pass |
| section11/C20-cyl/union | 1 | 8.7853981634 | 8.7853981634 | 27.1415926536 | 27.1415926536 | 2e-16 | 4.4e-16 | 9 / 8 | pass |
| section11/C20-cyl/cut | 1 | 8 | 8 | 24 | 24 | 2.2e-16 | 2.2e-16 | 6 / 6 | pass |
| section11/C20-cyl-quarter/union | 1 | 8.7853981634 | 8.7853981634 | 27.1415926536 | 27.1415926536 | 2e-16 | 2.2e-16 | 9 / 8 | pass |
| section11/C20-cyl-quarter/cut | 1 | 8 | 8 | 24 | 24 | 0 | 2.5e-16 | 6 / 6 | pass |
| section11/C21-cyl/union | 1 | 8 | 8 | 24 | 24 | 3.3e-16 | 3.1e-16 | 6 / 6 | pass |
| section11/C21-cyl/common | 1 | 1.57079632679 | 1.57079632679 | 7.85398163397 | 7.85398163397 | 2.8e-16 | 3.3e-16 | 3 / 3 | pass |
| section11/C21-cyl/cut | 1 | 6.42920367321 | 6.42920367321 | 28.7123889804 | 28.7123889804 | 2.8e-16 | 3.1e-16 | 7 / 7 | pass |
| section11/C22-cyl/union | 1 | 8.7853981634 | 8.7853981634 | 27.1415926536 | 27.1415926536 | 0 | 3.8e-16 |  | pass |
| section11/C22-cyl/common | 1 | 0.785398163397 | 0.785398163397 | 4.71238898038 | 4.71238898038 | 2.8e-16 | 5.4e-16 |  | pass |
| section11/C22-cyl/cut | 1 | 7.2146018366 | 7.2146018366 | 27.1415926536 | 27.1415926536 | 0 | 4.2e-16 |  | pass |
| section11/C23-cyl/common | 1 | 1.81379936423 | 1.81379936423 | 9.06899682117 | 9.06899682117 | 3.1e-15 | 1.3e-09 |  | pass |
| section11/C23-cyl/cut | 1 | 30.1862006358 | 30.1862006358 | 69.4413980927 | 69.4413980927 | 2e-16 | 8.5e-11 |  | pass |
| family/Through-bore/T1-001 | 2 | 25.1327412287 | 25.1327412287 | 56.5486677646 | 56.5486677646 | 1.4e-16 | 5.3e-16 |  | pass |
| family/Blind-bore/T1-005 | 1 | 565.486679531 | 565.486680474 | 433.53978808 | 433.53978808 | 1.7e-09 | 2.7e-08 |  | pass |
| family/Bore-layers/T1-016 | 2 | 901.643100854 | 901.643100854 | 687.964594301 | 687.964594301 | 3.8e-16 | 3.3e-16 |  | pass |
| family/Bore-union/T1-020 | 2 | 939.342212697 | 939.342212697 | 650.265482457 | 650.265482457 | 1.8e-16 | 2.1e-16 |  | pass |
| family/Opposed-blind/T1-025 | 2 | 968.584073464 | 968.584073464 | 656.548667765 | 656.548667765 | 2.3e-16 | 4.5e-16 |  | pass |
| family/Oblique-through/T1-029 | 4 | 117.566025875 | 117.566025875 | 161.097098864 | 161.097098864 | 6e-16 | 2.5e-09 |  | pass |
| family/Disjoint-bores/T1-031 | 6 | 7865.59741089 | 7865.59741089 | 3225.72560583 | 3225.72560583 | 1.6e-16 | 6.4e-15 |  | pass |
| family/Side-and-corner-cylinder/T1-036 | 2 | 4598.58518558 | 4598.58518558 | 2024.29435994 | 2024.29435994 | 2e-16 | 1.8e-15 |  | pass |
| family/Side-opening-pair/T1-048 | 10 | 0 | 0 | 0 | 0 | 6.4e-16 | 1.6e-14 |  | pass |
| family/Side-dual-disk/T1-055 | 18 | 309.481923083 | 309.481923083 | 321.214461534 | 321.214461534 | 1.8e-15 | 7.8e-15 |  | pass |
| family/Circular-stock/T1-062 | 4 | 603.185789489 | 603.185789489 | 452.389342117 | 452.389342117 | 1.9e-16 | 5e-16 |  | pass |
| family/Coaxial-cut/T1-068 | 2 | 502.654824574 | 502.654824574 | 603.185789489 | 603.185789489 | 3.4e-16 | 1.3e-15 |  | pass |
| family/Coaxial-join/T1-071 | 1 | 150.796447372 | 150.796447372 | 169.646003294 | 169.646003294 | 1.7e-16 | 2.4e-16 |  | pass |
| family/Join-all/T1-072 | 5 | 5000 | 5000 | 2200 | 2200 | 7.6e-16 | 1.5e-14 |  | pass |
| family/Containment-overlap/T1-074 | 2 | 1000 | 1000 | 600 | 600 | 4.5e-16 | 8.9e-15 |  | pass |
| family/Pocket-and-planar/T1-076 | 3 | 47163.6725877 | 47163.6725877 | 9278.19467106 | 9278.19467106 | 3.9e-16 | 1.5e-14 |  | pass |
| family/Pocket-with-cylinder/T1-083 | 3 | 47073.7124191 | 47073.7124191 | 9384.00439557 | 9384.00439557 | 3.1e-16 | 1.2e-14 |  | pass |
| legacy-refused/T1-002/line-7 | 1 | 486.867258771 | 486.867258771 | 427.98229715 | 427.98229715 | 1.2e-16 | 5.4e-17 |  | pass |
| legacy-refused/T1-002/line-9 | 1 | 486.867258771 | 486.867258771 | 427.98229715 | 427.98229715 | 1.2e-16 | 5.6e-17 |  | pass |
| legacy-refused/T1-008/line-9 | 5 | 1937.16814693 | 1937.16814693 | 1087.9645943 | 1087.9645943 | 2.1e-16 | 1.8e-15 |  | pass |
| legacy-refused/T1-012/line-3 | 1 | 1267.43362939 | 1267.43362939 | 793.132741229 | 793.132741229 | 1.8e-16 | 1.8e-15 |  | pass |
| legacy-refused/T1-041/line-36 | 27 | 2950.59498628 | 2950.59498628 | 1564.49785482 | 1564.49785482 | 5.8e-16 | 6.4e-15 |  | pass |
| legacy-refused/T1-041/line-73 | 27 | 3031.04884973 | 3031.04884973 | 1511.55483876 | 1511.55483876 | 4.4e-16 | 5.6e-15 |  | pass |
| legacy-refused/T1-041/line-110 | 27 | 2910.54673378 | 2910.54673378 | 1536.06349265 | 1536.06349265 | 4.4e-16 | 5.6e-15 |  | pass |
| legacy-refused/T1-041/line-147 | 27 | 3016.91168279 | 3016.91168279 | 1494.84244977 | 1494.84244977 | 4.4e-16 | 5.6e-15 |  | pass |
| legacy-refused/T1-042/line-9 | 4 | 4619.78286374 | 4619.78286374 | 1812.26250651 | 1812.26250651 | 3e-15 | 1.1e-14 |  | pass |
| legacy-refused/T1-042/line-28 | 4 | 4736.38274876 | 4736.38274876 | 1748.41150082 | 1748.41150082 | 1.9e-16 | 1.8e-15 |  | pass |
| legacy-refused/T1-046/line-3 | 1 | 968.584073464 | 968.584073464 | 585.132741229 | 585.132741229 | 1.9e-16 | 3.7e-15 |  | pass |
| legacy-refused/T1-074/line-6 | 3 | 9.93331732418 | 9.93331732418 | 41.7892519893 | 41.7892519893 | 3.3e-14 | 3.2e-13 |  | pass |
| legacy-refused/T1-081/line-4 | 2 | 768.584073464 | 768.584073464 | 576.548667765 | 576.548667765 | 4.3e-16 | 2.2e-15 |  | pass |
