# General Boolean trial: acceptance rows

This page freezes the acceptance cases of [the specification](SPEC.md),
section 15.1, as requirement A1 asks: 109 rows, 92 in Tier 1 and
17 in Tier 2. Nothing here is a result of the trial; the three A8
columns stay empty until Part 5.2 of [the plan](PLAN.md).

A row is one existing test function of `internal/plaincadbridge`. Its trace
is the list of bridge operations the function performs, in order, as
requests, with the responses the default bridge gave. It was recorded on
commit `9d7417dc` by a temporary patch that is not in the tree.
`TestGeneralBooleanTraces` in
`internal/plaincadbridge/general_boolean_trace_test.go` replays every trace
on a default bridge and requires the recorded responses within 1e-12
relative; the comment at the top of that file defines the trace format.

- Traces: `internal/plaincadbridge/testdata/general_boolean/<function>.jsonl`,
  109 files, 7,945,868 bytes in all. Most of that is STEP text inside
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
- Analytic values: a `near(got, want)` pair of the source test is kept, as
  `analytic`, on every earlier response of the same function whose volume,
  surface area or overlap value is bit for bit `got`.
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

"Direct" is the number of the row's requests recorded from direct calls.

| Id | Function | File | Tier | Family | Boolean requests | Checked | Unchecked | Analytic | Direct | Slow legacy | Volume error | Area error | Outcome |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| T1-001 | `TestThroughCircleCutAllAxesAndSigns` | `bore_test.go` | 1 | Through bore | cut ×6, common ×6 | 24 | face-role request ×6 | 24 | 0 |  | | | |
| T1-002 | `TestFiniteCylinderCoverageAndPartialCommon` | `bore_test.go` | 1 | Through bore | cut ×6, common ×5 | 21 | 0 | 4 | 0 |  | | | |
| T1-003 | `TestBoreClonePlacementAndRepeatedCuts` | `bore_test.go` | 1 | Through bore | cut ×3 | 12 | 0 | 4 | 0 |  | | | |
| T1-004 | `TestSingleBoreMeshKeepsVolumeWithScaleAwareSampling` | `bore_mesh_test.go` | 1 | Through bore | cut ×2 | 6 | 0 | 0 | 0 |  | | | |
| T1-005 | `TestGuidedRadiusThree` | `blind_test.go` | 1 | Blind bore | common ×1 | 3 | 0 | 0 | 0 |  | | | |
| T1-006 | `TestBlindAllAxesAndSigns` | `blind_test.go` | 1 | Blind bore | cut ×6 | 18 | 0 | 12 | 0 |  | | | |
| T1-007 | `TestBlindScopedRaisedStockAndMixedRecipe` | `blind_test.go` | 1 | Blind bore | cut ×2, common ×1 | 10 | 0 | 7 | 0 |  | | | |
| T1-008 | `TestRadiallyDisjointCommonAndUnsupportedCavity` | `blind_test.go` | 1 | Blind bore | cut ×1, common ×4 | 9 | 0 | 8 | 0 |  | | | |
| T1-009 | `TestBlindFloorNearExitReferenceIsAmbiguous` | `blind_test.go` | 1 | Blind bore | cut ×2 | 4 | 0 | 0 | 0 |  | | | |
| T1-010 | `TestMultipleBlindPocketsRetainSharedCaps` | `blind_test.go` | 1 | Blind bore | cut ×4 | 10 | 0 | 4 | 0 |  | | | |
| T1-011 | `TestGuidedOppositeEndBlindToolAllAxes` | `blind_test.go` | 1 | Blind bore | cut ×6 | 18 | 0 | 12 | 0 |  | | | |
| T1-012 | `TestExitOpenBlindRejectsUnpreservedEndpoints` | `blind_test.go` | 1 | Blind bore | cut ×2 | 5 | 0 | 0 | 0 |  | | | |
| T1-013 | `TestScopedHoleDefaultMeshVolume` | `blind_test.go` | 1 | Blind bore | cut ×1 | 3 | 0 | 0 | 0 |  | | | |
| T1-014 | `TestGuidedDefaultMeshRemovedVolumeAndNormalTurn` | `blind_test.go` | 1 | Blind bore | cut ×4 | 10 | 0 | 0 | 0 |  | | | |
| T1-015 | `TestExactBlindBoreTopologyDiagnostic` | `bore_contact_test.go` | 1 | Blind bore | cut ×1 | 7 | 0 | 0 | 0 |  | | | |
| T1-016 | `TestUnequalDepthBoreAxesSignsAndCaps` | `bore_layers_test.go` | 1 | Bore layers | cutMany ×12 | 48 | 0 | 24 | 0 |  | | | |
| T1-017 | `TestSequentialHoleOppositeBlindEntryAndOverlapQuery` | `bore_layers_test.go` | 1 | Bore layers | cut ×12, cutMany ×6, overlapVolume ×18 | 60 | 0 | 30 | 0 |  | | | |
| T1-018 | `TestOverlapVolumeAtFullHandleCapacity` | `bore_layers_test.go` | 1 | Bore layers | cut ×1, overlapVolume ×2 | 258 | 0 | 1 | 0 |  | | | |
| T1-019 | `TestOverlapVolumePreservesOrdinaryQueriesAndCommonGeometry` | `bore_layers_test.go` | 1 | Bore layers | cut ×1, common ×1, overlapVolume ×7 | 15 | 0 | 6 | 0 |  | | | |
| T1-020 | `TestOverlappingHoleUnionAxesSignsAndDepths` | `bore_union_test.go` | 1 | Bore union | cutMany ×12 | 48 | 0 | 39 | 0 |  | | | |
| T1-021 | `TestHoleUnionDuplicatesContainmentAndTripleOverlap` | `bore_union_test.go` | 1 | Bore union | cut ×3, cutMany ×4 | 16 | 0 | 8 | 0 |  | | | |
| T1-022 | `TestHoleUnionUnsupportedBatchDoesNotPublish` | `bore_union_test.go` | 1 | Bore union | cutMany ×1 | 4 | 0 | 0 | 0 |  | | | |
| T1-023 | `TestContainedUnionAnchorChangeDoesNotPublishTinyReduction` | `bore_union_test.go` | 1 | Bore union | cut ×4, cutMany ×3 | 12 | 0 | 0 | 0 |  | | | |
| T1-024 | `TestThroughUnionOpposedAxesAndInputOrders` | `bore_union_test.go` | 1 | Bore union | cutMany ×4 | 16 | 0 | 8 | 0 |  | | | |
| T1-025 | `TestOpposedBlindBoreMerge` | `opposed_bore_test.go` | 1 | Opposed blind | cut ×36 | 72 | 0 | 12 | 0 |  | | | |
| T1-026 | `TestOpposedBlindBoreStrictOverlapAndUnrelated` | `opposed_bore_test.go` | 1 | Opposed blind | cutMany ×2 | 257 | 0 | 2 | 0 |  | | | |
| T1-027 | `TestOpposedBlindBoresIntersectAtDifferentDepths` | `opposed_blind_bore_test.go` | 1 | Opposed blind | cut ×6, cutMany ×6, overlapVolume ×9 | 30 | 0 | 27 | 0 |  | | | |
| T1-028 | `TestOpposedBlindBoresMeetingAtOneLevel` | `opposed_blind_bore_test.go` | 1 | Opposed blind | cutMany ×4 | 16 | 0 | 8 | 0 |  | | | |
| T1-029 | `TestObliqueThroughBoreScopeAndSTL` | `oblique_bore_test.go` | 1 | Oblique through | cut ×2, cutMany ×2, overlapVolume ×4 | 16 | STEP request ×6 | 16 | 8 |  | | | |
| T1-030 | `TestObliqueThroughBoreRejectsIncompleteOpeningsAtomically` | `oblique_bore_test.go` | 1 | Oblique through | cutMany ×3, overlapVolume ×3 | 10 | 0 | 0 | 6 |  | | | |
| T1-031 | `TestDisjointBoresSTEP` | `disjoint_bores_test.go` | 1 | Disjoint bores | cut ×3, cutMany ×1, overlapVolume ×4 | 13 | STEP request ×4 | 12 | 5 |  | | | |
| T1-032 | `TestDisjointBlindBoresSTEP` | `disjoint_blind_bores_test.go` | 1 | Disjoint bores | cut ×3, cutMany ×1, overlapVolume ×6 | 14 | STEP request ×4 | 14 | 5 |  | | | |
| T1-033 | `TestFiniteDisjointBoresSTEP` | `disjoint_finite_bores_test.go` | 1 | Disjoint bores | cut ×3, cutMany ×1, overlapVolume ×6 | 14 | STEP request ×4 | 8 | 11 |  | | | |
| T1-034 | `TestRadialDisjointBoresSTEP` | `disjoint_radial_bores_test.go` | 1 | Disjoint bores | cut ×3, cutMany ×1, overlapVolume ×6 | 14 | STEP request ×4 | 14 | 5 |  | | | |
| T1-035 | `TestDisjointObliqueBoreScopeAndSTEP` | `disjoint_oblique_bore_test.go` | 1 | Disjoint bores | cut ×4, cutMany ×1, overlapVolume ×3 | 11 | STEP request ×2 | 11 | 3 |  | | | |
| T1-036 | `TestPlacedSideCrossingCircularCut` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×3, cutMany ×3 | 12 | 0 | 8 | 0 |  | | | |
| T1-037 | `TestPlacedSideCrossingBlindCylinderCut` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×24, cutMany ×24, overlapVolume ×48 | 126 | 0 | 144 | 0 |  | | | |
| T1-038 | `TestPlainCADSideNotchSTEPRoundTrip` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cutMany ×4 | 20 | STEP request ×24 | 27 | 28 |  | | | |
| T1-039 | `TestCornerBlindCutQualifiesEitherToolEndpoint` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×2, cutMany ×1, overlapVolume ×2 | 8 | STEP request ×2 | 2 | 2 |  | | | |
| T1-040 | `TestNotchedStockSTEPUnitsRetainRecipe` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cutMany ×1 | 2 | STEP request ×2 | 2 | 3 |  | | | |
| T1-041 | `TestSideNotchMixedBoreBatchIsAtomic` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×4, cutMany ×20, overlapVolume ×60 | 116 | STEP request ×28 | 116 | 48 | slow legacy | | | |
| T1-042 | `TestSideNotchRetainsConnectedCoaxialCuts` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×4, cutMany ×8, overlapVolume ×8 | 30 | STEP request ×6 | 24 | 14 |  | | | |
| T1-043 | `TestNotchedStockMergesInteriorBoresInAnyOrder` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×1, cutMany ×6, overlapVolume ×1 | 13 | STEP request ×4 | 11 | 10 |  | | | |
| T1-044 | `TestMultiBodyFiniteCornerRevolveCuts` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×6 | 14 | 0 | 6 | 0 |  | | | |
| T1-045 | `TestMultiBodyExtrudeDisjointNoop` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×1 | 3 | 0 | 0 | 0 |  | | | |
| T1-046 | `TestCornerThroughRejectsFiniteShortTool` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×1 | 3 | 0 | 0 | 0 |  | | | |
| T1-047 | `TestCornerThroughReversedFiniteAxis` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×1 | 3 | 0 | 1 | 0 |  | | | |
| T1-048 | `TestSideOpeningPairScopedAndSuccessive` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×4, cutMany ×12, overlapVolume ×16 | 56 | STEP request ×24 | 52 | 36 |  | | | |
| T1-049 | `TestSideOpeningPairOpposedScopes` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×2, cutMany ×4, overlapVolume ×6 | 15 | STEP request ×4 | 10 | 8 |  | | | |
| T1-050 | `TestSideOpeningPairDifferentStockSides` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×4, cutMany ×8, overlapVolume ×4 | 22 | 0 | 12 | 8 |  | | | |
| T1-051 | `TestSideOpeningCutsRetainHistoryAndAcceptFurtherHoles` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×1, cutMany ×4, overlapVolume ×4 | 16 | STEP request ×3 | 12 | 7 |  | | | |
| T1-052 | `TestSideOpeningCutsContinueStrictDualDisk` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×1, cutMany ×2, overlapVolume ×1 | 8 | STEP request ×1 | 5 | 3 |  | | | |
| T1-053 | `TestIntersectingCornerOpeningHoles` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×12, cutMany ×3, overlapVolume ×12 | 36 | STEP request ×18 | 29 | 21 |  | | | |
| T1-054 | `TestConcealedHoleSTEPAndFurtherCut` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×2, cutMany ×2, overlapVolume ×2 | 11 | STEP request ×4 | 3 | 6 |  | | | |
| T1-055 | `TestPlainCADSideOpenDualDiskCuts` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×6, cutMany ×6, overlapVolume ×20 | 40 | STEP request ×10 | 54 | 10 | slow legacy | | | |
| T1-056 | `TestPlainCADUnequalSideOpenDualDiskCuts` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×3, cutMany ×2, overlapVolume ×4 | 12 | STEP request ×3 | 10 | 3 | slow legacy | | | |
| T1-057 | `TestPlainCADOffCenterSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×2, cutMany ×1, overlapVolume ×1 | 7 | STEP request ×2 | 3 | 2 | slow legacy | | | |
| T1-058 | `TestPlainCADSideOpeningFallbackAtNonbinaryCrossing` | `side_dual_disk_test.go` | 1 | Side dual disk | cutMany ×1 | 4 | STEP request ×1 | 2 | 2 |  | | | |
| T1-059 | `TestPlainCADExactWallSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×4, cutMany ×2 | 14 | STEP request ×6 | 10 | 6 | slow legacy | | | |
| T1-060 | `TestPlainCADOffsetSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×4, cutMany ×2, overlapVolume ×6 | 18 | STEP request ×4 | 20 | 4 | slow legacy | | | |
| T1-061 | `TestPlainCADUnequalCenterSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×4, cutMany ×2, overlapVolume ×6 | 18 | STEP request ×4 | 20 | 4 | slow legacy | | | |
| T1-062 | `TestCircularStockFiniteEccentricHoleAndSTEP` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×4, cutMany ×4, overlapVolume ×8 | 24 | STEP request ×8 | 24 | 8 |  | | | |
| T1-063 | `TestCircularStockGroupedAndSuccessiveDisjointBores` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×3, cutMany ×5, overlapVolume ×7 | 22 | STEP request ×2 | 17 | 7 |  | | | |
| T1-064 | `TestCircularStockAuthoredEccentricInnerWireAndSignedCut` | `circular_stock_bore_test.go` | 1 | Circular stock | cutMany ×2, overlapVolume ×1 | 6 | 0 | 4 | 3 |  | | | |
| T1-065 | `TestCircularStockDisjointOpposedBoresAndSTEP` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×2, cutMany ×4, overlapVolume ×26 | 40 | STEP request ×8 | 52 | 12 |  | | | |
| T1-066 | `TestCircularStockOverlappingOpposedAndSequentialBores` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×2, cutMany ×8, overlapVolume ×14 | 28 | STEP request ×8 | 42 | 16 |  | | | |
| T1-067 | `TestCircularStockCoaxialOpposedSTEPAndThroughCut` | `circular_stock_bore_test.go` | 1 | Circular stock | cutMany ×2, overlapVolume ×2 | 8 | STEP request ×2 | 5 | 4 |  | | | |
| T1-068 | `TestFullTubeSTEPAndCoaxialCut` | `tube_step_test.go` | 1 | Coaxial cut | cut ×3 | 6 | STEP request ×5; operand outside the surface class ×1 | 4 | 5 |  | | | |
| T1-069 | `TestCylinderSectorSTEPBridge` | `cylinder_sector_step_test.go` | 1 | Coaxial cut | cut ×1 | 4 | STEP request ×6 | 5 | 6 |  | | | |
| T1-070 | `TestQuarterAnnularSTEPAndFurtherCut` | `quarter_annular_step_test.go` | 1 | Coaxial cut | cut ×3 | 6 | STEP request ×4; operand outside the surface class ×1 | 2 | 4 |  | | | |
| T1-071 | `TestCoaxialDispatchPlacementAndMeshBudget` | `coaxial_join_test.go` | 1 | Coaxial join | fuse ×2 | 7 | 0 | 0 | 0 |  | | | |
| T1-072 | `TestScopedJoinPublishesOnlyConnectedAddedMaterial` | `join_all_test.go` | 1 | Join all | joinAll ×4 | 14 | 0 | 10 | 0 |  | | | |
| T1-073 | `TestScopedJoinWithContainingRevolvedTool` | `join_all_test.go` | 1 | Join all | joinAll ×1 | 7 | 0 | 3 | 0 |  | | | |
| T1-074 | `TestContainingCylinderOverlapCapture` | `cylinder_containment_overlap_test.go` | 1 | Containment overlap | overlapVolume ×3 | 6 | 0 | 0 | 3 |  | | | |
| T1-075 | `TestRevolveJoinCaptureContainedPlanarVolume` | `overlap_capture_test.go` | 1 | Containment overlap | overlapVolume ×3 | 6 | 0 | 4 | 3 |  | | | |
| T1-076 | `TestNativeBoxPocketBoreAndNestedCut` | `box_pocket_test.go` | 1 | Pocket and planar | cut ×2, cutMany ×1, pocketFloor ×2 | 9 | face-role request ×1 | 3 | 0 |  | | | |
| T1-077 | `TestScopedRotatedPocketCopiesOnPocketedStock` | `box_pocket_test.go` | 1 | Pocket and planar | cut ×4, cutMany ×1, overlapVolume ×3 | 13 | 0 | 11 | 0 |  | | | |
| T1-078 | `TestPatternOverlapUsesFiniteCylinderLens` | `box_pocket_test.go` | 1 | Pocket and planar | overlapVolume ×2 | 4 | 0 | 2 | 0 |  | | | |
| T1-079 | `TestPlanarPocketAfterFourDisjointBores` | `box_pocket_test.go` | 1 | Pocket and planar | cut ×1, cutMany ×1 | 8 | 0 | 1 | 0 |  | | | |
| T1-080 | `TestCutManyPlanarAtomicAndRedundantTools` | `cut_many_test.go` | 1 | Pocket and planar | cutMany ×4 | 8 | 0 | 4 | 0 |  | | | |
| T1-081 | `TestCutManyValidationDoesNotPublish` | `cut_many_test.go` | 1 | Pocket and planar | cutMany ×4 | 8 | 0 | 0 | 0 |  | | | |
| T1-082 | `TestExtrudeDialogJoinThenCutExactVolume` | `planar_boolean_chain_test.go` | 1 | Pocket and planar | cut ×1, fuse ×1 | 8 | 0 | 8 | 0 |  | | | |
| T1-083 | `TestPocketFloorCircleCutsAndCounterbore` | `pocket_cylinder_cut_test.go` | 1 | Pocket with cylinder | cut ×6, cutMany ×3 | 21 | 0 | 3 | 0 |  | | | |
| T1-084 | `TestRetainedPlanarBoreFillRoofAndRedrill` | `retained_planar_bores_test.go` | 1 | Pocket with cylinder | cut ×7, fuse ×3, cutMany ×3 | 27 | 0 | 13 | 0 | slow legacy | | | |
| T1-085 | `TestRetainedPlanarBoreProofRejectsUnsafeDomains` | `retained_planar_bores_test.go` | 1 | Pocket with cylinder | cut ×2, fuse ×1 | 10 | 0 | 0 | 0 |  | | | |
| T1-086 | `TestIslandPocketFiniteSignedCaps` | `island_pocket_test.go` | 1 | Pocket with cylinder | cut ×24 | 72 | 0 | 0 | 0 |  | | | |
| T1-087 | `TestToFaceFiniteCoverageAndHoles` | `to_face_test.go` | 1 | To face | prismToFace ×2, circleToFace ×6 | 10 | 0 | 5 | 0 |  | | | |
| T1-088 | `TestToFaceCircleSignedAxesRetainsAnalyticSolid` | `to_face_test.go` | 1 | To face | circleToFace ×6 | 12 | 0 | 12 | 0 |  | | | |
| T1-089 | `TestToFacePolygonObliqueAndBoundary` | `to_face_test.go` | 1 | To face | prismToFace ×3 | 5 | 0 | 2 | 0 |  | | | |
| T1-090 | `TestToFaceAnalyticCircularHoleCoverage` | `to_face_analytic_test.go` | 1 | To face | prismToFace ×2 | 3 | 0 | 1 | 0 |  | | | |
| T1-091 | `TestToFaceAnalyticCircleHoleExactGap` | `to_face_analytic_test.go` | 1 | To face | circleToFace ×2 | 3 | 0 | 1 | 0 |  | | | |
| T1-092 | `TestObliqueToFaceUsesCircularExtent` | `to_face_oblique_test.go` | 1 | To face | circleToFace ×3 | 5 | 0 | 1 | 0 |  | | | |
| T2-001 | `TestCrossedBoreScopeAndSTEP` | `crossed_bore_test.go` | 2 | Crossed bores | cut ×2, cutMany ×3, overlapVolume ×4 | 12 | STEP request ×9 | 16 | 12 |  | | | |
| T2-002 | `TestBlindCrossedBoreScopeAndSTEP` | `blind_crossed_bore_test.go` | 2 | Crossed bores | cut ×2, cutMany ×2, overlapVolume ×3 | 10 | STEP request ×6 | 11 | 8 |  | | | |
| T2-003 | `TestDoubleBlindCrossedBoreScopeAndSTEP` | `double_blind_crossed_bore_test.go` | 2 | Crossed bores | cut ×8, cutMany ×4, overlapVolume ×8 | 32 | STEP request ×8 | 32 | 12 |  | | | |
| T2-004 | `TestContainedCrossedBores` | `contained_crossed_bores_test.go` | 2 | Crossed bores | cut ×1, cutMany ×4, overlapVolume ×2 | 13 | STEP request ×2 | 5 | 6 |  | | | |
| T2-005 | `TestClippedCrossedBoreScopeAndSTEP` | `clipped_crossed_bore_test.go` | 2 | Crossed bores | cut ×12, cutMany ×6, overlapVolume ×12 | 48 | STEP request ×12 | 48 | 18 | slow legacy | | | |
| T2-006 | `TestTripleCrossedBoreScopeAndSTEP` | `triple_crossed_bore_test.go` | 2 | Triple and companion | cut ×3, cutMany ×1, overlapVolume ×4 | 12 | STEP request ×2 | 8 | 3 |  | | | |
| T2-007 | `TestBlindTripleCrossedBoreScopeAndSTEP` | `blind_triple_crossed_bore_test.go` | 2 | Triple and companion | cutMany ×6, overlapVolume ×18 | 48 | STEP request ×12 | 42 | 18 | slow legacy | | | |
| T2-008 | `TestMultipleBlindTripleCrossedBoreScopeAndSTEP` | `blind_triple_crossed_bore_test.go` | 2 | Triple and companion | cut ×11, cutMany ×4, overlapVolume ×11 | 40 | STEP request ×8 | 36 | 12 | slow legacy | | | |
| T2-009 | `TestCrossedCompanionBoreScopeAndSTEP` | `crossed_companion_bore_test.go` | 2 | Triple and companion | cut ×3, cutMany ×1, overlapVolume ×2 | 10 | STEP request ×2 | 8 | 3 |  | | | |
| T2-010 | `TestCrossedObliqueBoreScope` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×2, cutMany ×1, overlapVolume ×3 | 9 | STEP request ×2 | 9 | 3 |  | | | |
| T2-011 | `TestCrossedObliqueReciprocalScope` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×8, cutMany ×2, overlapVolume ×6 | 24 | STEP request ×4 | 32 | 6 |  | | | |
| T2-012 | `TestCrossedObliquePairWithSeparatedOppositeZOpenings` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cutMany ×1 | 4 | 0 | 1 | 1 |  | | | |
| T2-013 | `TestCrossedObliqueBoreCoordinatePlacements` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cutMany ×32, overlapVolume ×32 | 160 | STEP request ×96 | 92 | 128 | slow legacy | | | |
| T2-014 | `TestBlindCrossedObliqueBoreSTEPAndFurtherCut` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×2, cutMany ×14, overlapVolume ×8 | 38 | STEP request ×14 | 14 | 28 | slow legacy | | | |
| T2-015 | `TestConcealedCrossedObliqueBoreSTEPCanonical` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cutMany ×14, overlapVolume ×12 | 42 | STEP request ×10 | 12 | 24 |  | | | |
| T2-016 | `TestObliqueFloorCrossedBoreReimport` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cutMany ×6, overlapVolume ×4 | 18 | STEP request ×10 | 12 | 16 |  | | | |
| T2-017 | `TestDoubleBlindCrossedObliqueBore` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×2, cutMany ×6, overlapVolume ×8 | 24 | STEP request ×10 | 16 | 16 |  | | | |

The trace of a row is the file named after its function.

## Dropped

Dropped for a reason in section 15.2: every Boolean response of the function
is unchecked.

| Function | File | Tier | Family | Reason |
|---|---|---|---|---|
| `TestPlainCADRoundedHousingJoinPocketAndThroughProfiles` | `rounded_stack_test.go` | 1 | Rounded profiles | The stock gets a cap fillet or chamfer before the first Boolean request, so every Boolean operand has torus or cone faces: rim treatment, outside the surface class (15.2, fourth item). cut ×8, fuse ×1, cutMany ×4. |
| `TestPlainCADRotaryPlatterSlotsAndBores` | `rotary_platter_test.go` | 1 | Rounded profiles | The stock gets a cap fillet or chamfer before the first Boolean request, so every Boolean operand has torus or cone faces: rim treatment, outside the surface class (15.2, fourth item). cut ×8, cutMany ×2. |
| `TestPlainCADAnnularPedestalAndThroughBores` | `annular_pedestal_test.go` | 1 | Rounded profiles | The stock gets a cap fillet or chamfer before the first Boolean request, so every Boolean operand has torus or cone faces: rim treatment, outside the surface class (15.2, fourth item). cut ×1, fuse ×1, cutMany ×1, overlapVolume ×1. |

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
| 1 | Rounded profiles | 3 | 3 | 0 | 3 | 0 |
| 1 | To face | 7 | 7 | 6 | 0 | 1 |
| 2 | Crossed bores | 5 | 5 | 5 | 0 | 0 |
| 2 | Triple and companion | 4 | 4 | 4 | 0 | 0 |
| 2 | Crossed oblique | 8 | 8 | 8 | 0 | 0 |
| 1 | **All of tier 1** | **110** | **109** | **92** | **3** | **14** |
| 2 | **All of tier 2** | **17** | **17** | **17** | **0** | **0** |
