# General Boolean trial: acceptance rows

This page freezes the acceptance cases of [the specification](SPEC.md),
section 15.1, as requirement A1 asks. It is not complete: **62 rows are
frozen and 52 functions are held for an owner decision** (see "Held").
Nothing here is a result of the trial; the three A8 columns stay empty until
Part 5.2 of [the plan](PLAN.md).

A row is one existing test function of `internal/plaincadbridge`. Its trace
is the list of requests the function issues through `Bridge.Call`, in order,
with the responses the default bridge gave, recorded on commit `9d7417dc` by
a temporary patch that is not in the tree. `TestGeneralBooleanTraces` in
`internal/plaincadbridge/general_boolean_trace_test.go` replays every trace
on a default bridge and requires the recorded responses within 1e-12
relative; the comment at the top of that file defines the trace format.

- Traces: `internal/plaincadbridge/testdata/general_boolean/<function>.jsonl`,
  62 files, 363231 bytes in all.
- Checked responses: every response of a trace is compared on a general
  bridge (A3, A4) unless the trace line carries `unchecked` with the
  exclusion of section 15.2 that applies. Frozen rows hold 1575 requests, 1568
  checked and 7 unchecked (face-role request ×7).
- Analytic values: a `near(got, want)` pair of the source test is kept, as
  `analytic`, on every earlier response of the same function whose volume,
  surface area or overlap value is bit for bit `got`.
- Operands: an operand counts as inside the surface class when it is the
  result of a checked Boolean request, which a general bridge builds itself,
  or when every face of its stored solid is a plane or a circular cylinder.
- Direct calls in frozen rows: 9 frozen rows also call `importSTEP`,
  `exportSTEP` or `putAnalyticBody` as Go methods. Those are assertions
  about STEP and proofs, which 15.2 leaves out, and no recorded request of
  those rows uses a handle that such a call published.
- `pocketFloor` (CO4) is issued by `TestNativeBoxPocketBoreAndNestedCut` (T1-046) only.
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
  skipped in the default tier too.

## Rows

| Id | Function | File | Tier | Family | Boolean requests | Checked | Unchecked | Analytic | Slow legacy | Volume error | Area error | Outcome |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| T1-001 | `TestThroughCircleCutAllAxesAndSigns` | `bore_test.go` | 1 | Through bore | cut ×6, common ×6 | 24 | face-role request ×6 | 24 |  | | | |
| T1-002 | `TestFiniteCylinderCoverageAndPartialCommon` | `bore_test.go` | 1 | Through bore | cut ×6, common ×5 | 21 | 0 | 4 |  | | | |
| T1-003 | `TestBoreClonePlacementAndRepeatedCuts` | `bore_test.go` | 1 | Through bore | cut ×3 | 12 | 0 | 4 |  | | | |
| T1-004 | `TestSingleBoreMeshKeepsVolumeWithScaleAwareSampling` | `bore_mesh_test.go` | 1 | Through bore | cut ×2 | 6 | 0 | 0 |  | | | |
| T1-005 | `TestGuidedRadiusThree` | `blind_test.go` | 1 | Blind bore | common ×1 | 3 | 0 | 0 |  | | | |
| T1-006 | `TestBlindAllAxesAndSigns` | `blind_test.go` | 1 | Blind bore | cut ×6 | 18 | 0 | 12 |  | | | |
| T1-007 | `TestBlindScopedRaisedStockAndMixedRecipe` | `blind_test.go` | 1 | Blind bore | cut ×2, common ×1 | 10 | 0 | 7 |  | | | |
| T1-008 | `TestRadiallyDisjointCommonAndUnsupportedCavity` | `blind_test.go` | 1 | Blind bore | cut ×1, common ×4 | 9 | 0 | 8 |  | | | |
| T1-009 | `TestBlindFloorNearExitReferenceIsAmbiguous` | `blind_test.go` | 1 | Blind bore | cut ×2 | 4 | 0 | 0 |  | | | |
| T1-010 | `TestMultipleBlindPocketsRetainSharedCaps` | `blind_test.go` | 1 | Blind bore | cut ×4 | 10 | 0 | 4 |  | | | |
| T1-011 | `TestGuidedOppositeEndBlindToolAllAxes` | `blind_test.go` | 1 | Blind bore | cut ×6 | 18 | 0 | 12 |  | | | |
| T1-012 | `TestExitOpenBlindRejectsUnpreservedEndpoints` | `blind_test.go` | 1 | Blind bore | cut ×2 | 5 | 0 | 0 |  | | | |
| T1-013 | `TestScopedHoleDefaultMeshVolume` | `blind_test.go` | 1 | Blind bore | cut ×1 | 3 | 0 | 0 |  | | | |
| T1-014 | `TestGuidedDefaultMeshRemovedVolumeAndNormalTurn` | `blind_test.go` | 1 | Blind bore | cut ×4 | 10 | 0 | 0 |  | | | |
| T1-015 | `TestExactBlindBoreTopologyDiagnostic` | `bore_contact_test.go` | 1 | Blind bore | cut ×1 | 7 | 0 | 0 |  | | | |
| T1-016 | `TestUnequalDepthBoreAxesSignsAndCaps` | `bore_layers_test.go` | 1 | Bore layers | cutMany ×12 | 48 | 0 | 24 |  | | | |
| T1-017 | `TestSequentialHoleOppositeBlindEntryAndOverlapQuery` | `bore_layers_test.go` | 1 | Bore layers | cut ×12, cutMany ×6, overlapVolume ×18 | 60 | 0 | 30 |  | | | |
| T1-018 | `TestOverlapVolumeAtFullHandleCapacity` | `bore_layers_test.go` | 1 | Bore layers | cut ×1, overlapVolume ×2 | 258 | 0 | 1 |  | | | |
| T1-019 | `TestOverlapVolumePreservesOrdinaryQueriesAndCommonGeometry` | `bore_layers_test.go` | 1 | Bore layers | cut ×1, common ×1, overlapVolume ×7 | 15 | 0 | 6 |  | | | |
| T1-020 | `TestOverlappingHoleUnionAxesSignsAndDepths` | `bore_union_test.go` | 1 | Bore union | cutMany ×12 | 48 | 0 | 39 |  | | | |
| T1-021 | `TestHoleUnionDuplicatesContainmentAndTripleOverlap` | `bore_union_test.go` | 1 | Bore union | cut ×3, cutMany ×4 | 16 | 0 | 8 |  | | | |
| T1-022 | `TestHoleUnionUnsupportedBatchDoesNotPublish` | `bore_union_test.go` | 1 | Bore union | cutMany ×1 | 4 | 0 | 0 |  | | | |
| T1-023 | `TestContainedUnionAnchorChangeDoesNotPublishTinyReduction` | `bore_union_test.go` | 1 | Bore union | cut ×4, cutMany ×3 | 12 | 0 | 0 |  | | | |
| T1-024 | `TestThroughUnionOpposedAxesAndInputOrders` | `bore_union_test.go` | 1 | Bore union | cutMany ×4 | 16 | 0 | 8 |  | | | |
| T1-025 | `TestOpposedBlindBoreMerge` | `opposed_bore_test.go` | 1 | Opposed blind | cut ×36 | 72 | 0 | 12 |  | | | |
| T1-026 | `TestOpposedBlindBoreStrictOverlapAndUnrelated` | `opposed_bore_test.go` | 1 | Opposed blind | cutMany ×2 | 257 | 0 | 2 |  | | | |
| T1-027 | `TestOpposedBlindBoresIntersectAtDifferentDepths` | `opposed_blind_bore_test.go` | 1 | Opposed blind | cut ×6, cutMany ×6, overlapVolume ×9 | 30 | 0 | 27 |  | | | |
| T1-028 | `TestOpposedBlindBoresMeetingAtOneLevel` | `opposed_blind_bore_test.go` | 1 | Opposed blind | cutMany ×4 | 16 | 0 | 8 |  | | | |
| T1-029 | `TestPlacedSideCrossingCircularCut` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×3, cutMany ×3 | 12 | 0 | 8 |  | | | |
| T1-030 | `TestPlacedSideCrossingBlindCylinderCut` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×24, cutMany ×24, overlapVolume ×48 | 126 | 0 | 144 |  | | | |
| T1-031 | `TestCornerBlindCutQualifiesEitherToolEndpoint` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×2, cutMany ×1, overlapVolume ×2 | 8 | 0 | 2 |  | | | |
| T1-032 | `TestMultiBodyFiniteCornerRevolveCuts` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×6 | 14 | 0 | 6 |  | | | |
| T1-033 | `TestMultiBodyExtrudeDisjointNoop` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×1 | 3 | 0 | 0 |  | | | |
| T1-034 | `TestCornerThroughRejectsFiniteShortTool` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×1 | 3 | 0 | 0 |  | | | |
| T1-035 | `TestCornerThroughReversedFiniteAxis` | `multi_body_corner_test.go` | 1 | Side and corner cylinder | cut ×1 | 3 | 0 | 1 |  | | | |
| T1-036 | `TestPlainCADSideOpenDualDiskCuts` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×6, cutMany ×6, overlapVolume ×20 | 40 | 0 | 40 | slow legacy | | | |
| T1-037 | `TestPlainCADUnequalSideOpenDualDiskCuts` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×3, cutMany ×2, overlapVolume ×4 | 12 | 0 | 10 | slow legacy | | | |
| T1-038 | `TestPlainCADOffCenterSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×2, cutMany ×1, overlapVolume ×1 | 7 | 0 | 3 | slow legacy | | | |
| T1-039 | `TestPlainCADExactWallSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×4, cutMany ×2 | 14 | 0 | 6 | slow legacy | | | |
| T1-040 | `TestPlainCADOffsetSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×4, cutMany ×2, overlapVolume ×6 | 18 | 0 | 14 | slow legacy | | | |
| T1-041 | `TestPlainCADUnequalCenterSideOpenDualDisk` | `side_dual_disk_test.go` | 1 | Side dual disk | cut ×4, cutMany ×2, overlapVolume ×6 | 18 | 0 | 14 | slow legacy | | | |
| T1-042 | `TestCircularStockFiniteEccentricHoleAndSTEP` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×4, cutMany ×4, overlapVolume ×8 | 24 | 0 | 24 |  | | | |
| T1-043 | `TestCoaxialDispatchPlacementAndMeshBudget` | `coaxial_join_test.go` | 1 | Coaxial join | fuse ×2 | 7 | 0 | 0 |  | | | |
| T1-044 | `TestScopedJoinPublishesOnlyConnectedAddedMaterial` | `join_all_test.go` | 1 | Join all | joinAll ×4 | 14 | 0 | 10 |  | | | |
| T1-045 | `TestScopedJoinWithContainingRevolvedTool` | `join_all_test.go` | 1 | Join all | joinAll ×1 | 7 | 0 | 3 |  | | | |
| T1-046 | `TestNativeBoxPocketBoreAndNestedCut` | `box_pocket_test.go` | 1 | Pocket and planar | cut ×2, cutMany ×1, pocketFloor ×2 | 9 | face-role request ×1 | 3 |  | | | |
| T1-047 | `TestScopedRotatedPocketCopiesOnPocketedStock` | `box_pocket_test.go` | 1 | Pocket and planar | cut ×4, cutMany ×1, overlapVolume ×3 | 13 | 0 | 11 |  | | | |
| T1-048 | `TestPatternOverlapUsesFiniteCylinderLens` | `box_pocket_test.go` | 1 | Pocket and planar | overlapVolume ×2 | 4 | 0 | 2 |  | | | |
| T1-049 | `TestPlanarPocketAfterFourDisjointBores` | `box_pocket_test.go` | 1 | Pocket and planar | cut ×1, cutMany ×1 | 8 | 0 | 1 |  | | | |
| T1-050 | `TestCutManyPlanarAtomicAndRedundantTools` | `cut_many_test.go` | 1 | Pocket and planar | cutMany ×4 | 8 | 0 | 4 |  | | | |
| T1-051 | `TestCutManyValidationDoesNotPublish` | `cut_many_test.go` | 1 | Pocket and planar | cutMany ×4 | 8 | 0 | 0 |  | | | |
| T1-052 | `TestExtrudeDialogJoinThenCutExactVolume` | `planar_boolean_chain_test.go` | 1 | Pocket and planar | cut ×1, fuse ×1 | 8 | 0 | 8 |  | | | |
| T1-053 | `TestPocketFloorCircleCutsAndCounterbore` | `pocket_cylinder_cut_test.go` | 1 | Pocket with cylinder | cut ×6, cutMany ×3 | 21 | 0 | 3 |  | | | |
| T1-054 | `TestRetainedPlanarBoreFillRoofAndRedrill` | `retained_planar_bores_test.go` | 1 | Pocket with cylinder | cut ×7, fuse ×3, cutMany ×3 | 27 | 0 | 13 | slow legacy | | | |
| T1-055 | `TestRetainedPlanarBoreProofRejectsUnsafeDomains` | `retained_planar_bores_test.go` | 1 | Pocket with cylinder | cut ×2, fuse ×1 | 10 | 0 | 0 |  | | | |
| T1-056 | `TestIslandPocketFiniteSignedCaps` | `island_pocket_test.go` | 1 | Pocket with cylinder | cut ×24 | 72 | 0 | 0 |  | | | |
| T1-057 | `TestToFaceFiniteCoverageAndHoles` | `to_face_test.go` | 1 | To face | prismToFace ×2, circleToFace ×6 | 10 | 0 | 5 |  | | | |
| T1-058 | `TestToFaceCircleSignedAxesRetainsAnalyticSolid` | `to_face_test.go` | 1 | To face | circleToFace ×6 | 12 | 0 | 12 |  | | | |
| T1-059 | `TestToFacePolygonObliqueAndBoundary` | `to_face_test.go` | 1 | To face | prismToFace ×3 | 5 | 0 | 2 |  | | | |
| T1-060 | `TestToFaceAnalyticCircularHoleCoverage` | `to_face_analytic_test.go` | 1 | To face | prismToFace ×2 | 3 | 0 | 1 |  | | | |
| T1-061 | `TestToFaceAnalyticCircleHoleExactGap` | `to_face_analytic_test.go` | 1 | To face | circleToFace ×2 | 3 | 0 | 1 |  | | | |
| T1-062 | `TestObliqueToFaceUsesCircularExtent` | `to_face_oblique_test.go` | 1 | To face | circleToFace ×3 | 5 | 0 | 1 |  | | | |

The trace of a row is the file named after its function.

## Dropped

Dropped for a reason in section 15.2: every Boolean response of the function
is unchecked.

| Function | File | Tier | Family | Reason |
|---|---|---|---|---|
| `TestPlainCADRoundedHousingJoinPocketAndThroughProfiles` | `rounded_stack_test.go` | 1 | Rounded profiles | The stock gets a cap fillet or chamfer before the first Boolean request, so every Boolean operand has torus or cone faces: rim treatment, outside the surface class (15.2, fourth item). cut ×8, fuse ×1, cutMany ×4. |
| `TestPlainCADRotaryPlatterSlotsAndBores` | `rotary_platter_test.go` | 1 | Rounded profiles | The stock gets a cap fillet or chamfer before the first Boolean request, so every Boolean operand has torus or cone faces: rim treatment, outside the surface class (15.2, fourth item). cut ×8, cutMany ×2. |

## Not in the acceptance set

Section 15.1 takes the functions that issue a Boolean operation. These issue
none, by request or by a direct call, so they were never cases.

| Function | File | What it does instead |
|---|---|---|
| `TestAngularMappingUnderflowDoesNotDisableChecks` | `blind_test.go` | calls kernel or bridge helpers only; no bridge request |
| `TestExactBlindBoreNoProofForObliqueFrame` | `bore_contact_test.go` | 1 request, none Boolean |
| `TestOpposedBlindBoreRejectsDifferentSupport` | `opposed_bore_test.go` | 4 requests, none Boolean |
| `TestObliqueFiniteCapContactProof` | `oblique_bore_test.go` | calls kernel or bridge helpers only; no bridge request |
| `TestPlainCADSideOpenDualDiskSTEPUnits` | `side_dual_disk_test.go` | direct calls: importSTEP ×1 |
| `TestSTEPIndependentlyAuthoredRoundBlindAndThroughBores` | `circular_stock_bore_test.go` | direct calls: exportSTEP ×1, importSTEP ×1 |
| `TestFullTubeSTEPIndependentAndUnits` | `tube_step_test.go` | direct calls: exportSTEP ×2, importSTEP ×2 |
| `TestCylinderSectorSTEPBridgeIndependentAndUnits` | `cylinder_sector_step_test.go` | direct calls: exportSTEP ×2, importSTEP ×2 |
| `TestQuarterAnnularSTEPMillimeterNormalization` | `quarter_annular_step_test.go` | direct calls: exportSTEP ×1, importSTEP ×1 |
| `TestToFaceExactDiskSeparation` | `to_face_analytic_test.go` | calls kernel or bridge helpers only; no bridge request |

## Held

Not frozen. The plan records what a test sends through `Bridge.Call`. These
functions also call bridge operations as Go methods, which that recording
does not see, so their traces are incomplete in one or both of these ways:

- a Boolean operation is called directly (`b.cutMany(…)`, `b.overlapVolume(…)`
  and others), so its result is in no trace;
- a recorded request uses a handle that only a direct call published, most
  often `b.importSTEP(…)`, so the trace cannot be replayed.

The plan says to stop and ask the owner in both situations, and not to drop
or shorten a row. "Requests" counts the Boolean requests that did go through
`Bridge.Call`.

| Function | File | Tier | Family | Boolean requests | Direct Boolean calls | Other direct calls | Requests that use an unrecorded handle |
|---|---|---|---|---|---|---|---|
| `TestObliqueThroughBoreScopeAndSTL` | `oblique_bore_test.go` | 1 | Oblique through | cut ×2, overlapVolume ×4 | cutMany ×2 | exportSTEP ×4, importSTEP ×2 | 4 |
| `TestObliqueThroughBoreRejectsIncompleteOpeningsAtomically` | `oblique_bore_test.go` | 1 | Oblique through | none | cutMany ×3, overlapVolume ×3, curvedBoolean ×3 | none | 0 |
| `TestDisjointBoresSTEP` | `disjoint_bores_test.go` | 1 | Disjoint bores | cut ×3, overlapVolume ×4 | cutMany ×1 | exportSTEP ×2, importSTEP ×2 | 4 |
| `TestDisjointBlindBoresSTEP` | `disjoint_blind_bores_test.go` | 1 | Disjoint bores | cut ×3, overlapVolume ×6 | cutMany ×1 | exportSTEP ×2, importSTEP ×2 | 3 |
| `TestFiniteDisjointBoresSTEP` | `disjoint_finite_bores_test.go` | 1 | Disjoint bores | cut ×3 | cutMany ×1, overlapVolume ×6 | exportSTEP ×2, importSTEP ×2 | 0 |
| `TestRadialDisjointBoresSTEP` | `disjoint_radial_bores_test.go` | 1 | Disjoint bores | cut ×3, overlapVolume ×6 | cutMany ×1 | exportSTEP ×2, importSTEP ×2 | 3 |
| `TestDisjointObliqueBoreScopeAndSTEP` | `disjoint_oblique_bore_test.go` | 1 | Disjoint bores | cut ×4, overlapVolume ×3 | cutMany ×1 | exportSTEP ×1, importSTEP ×1 | 1 |
| `TestPlainCADSideNotchSTEPRoundTrip` | `side_cylinder_test.go` | 1 | Side and corner cylinder | none | cutMany ×4 | exportSTEP ×12, importSTEP ×12 | 8 |
| `TestNotchedStockSTEPUnitsRetainRecipe` | `side_cylinder_test.go` | 1 | Side and corner cylinder | none | cutMany ×1 | exportSTEP ×1, importSTEP ×1 | 0 |
| `TestSideNotchMixedBoreBatchIsAtomic` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×4, overlapVolume ×60 | cutMany ×20 | exportSTEP ×16, importSTEP ×12 | 60 |
| `TestSideNotchRetainsConnectedCoaxialCuts` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×4, overlapVolume ×8 | cutMany ×8 | exportSTEP ×6 | 6 |
| `TestNotchedStockMergesInteriorBoresInAnyOrder` | `side_cylinder_test.go` | 1 | Side and corner cylinder | cut ×1, overlapVolume ×1 | cutMany ×6 | exportSTEP ×4 | 1 |
| `TestSideOpeningPairScopedAndSuccessive` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×4, overlapVolume ×16 | cutMany ×12 | exportSTEP ×12, importSTEP ×12 | 24 |
| `TestSideOpeningPairOpposedScopes` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×2, overlapVolume ×6 | cutMany ×4 | exportSTEP ×4 | 4 |
| `TestSideOpeningPairDifferentStockSides` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×4, overlapVolume ×4 | cutMany ×8 | none | 0 |
| `TestSideOpeningCutsRetainHistoryAndAcceptFurtherHoles` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×1, overlapVolume ×4 | cutMany ×4 | exportSTEP ×2, importSTEP ×1 | 3 |
| `TestSideOpeningCutsContinueStrictDualDisk` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×1, overlapVolume ×1 | cutMany ×2 | importSTEP ×1 | 2 |
| `TestIntersectingCornerOpeningHoles` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×12, overlapVolume ×12 | cutMany ×3 | exportSTEP ×12, importSTEP ×6 | 12 |
| `TestConcealedHoleSTEPAndFurtherCut` | `side_opening_pair_test.go` | 1 | Side opening pair | cut ×2, overlapVolume ×2 | cutMany ×2 | exportSTEP ×3, importSTEP ×1 | 3 |
| `TestPlainCADSideOpeningFallbackAtNonbinaryCrossing` | `side_dual_disk_test.go` | 1 | Side dual disk | none | cutMany ×1, cutSideDualDisk ×1 | importSTEP ×1 | 0 |
| `TestCircularStockGroupedAndSuccessiveDisjointBores` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×3, overlapVolume ×7 | cutMany ×5 | exportSTEP ×1, importSTEP ×1 | 1 |
| `TestCircularStockAuthoredEccentricInnerWireAndSignedCut` | `circular_stock_bore_test.go` | 1 | Circular stock | overlapVolume ×1 | cutMany ×2 | mixedExtrude ×1 | 1 |
| `TestCircularStockDisjointOpposedBoresAndSTEP` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×2, overlapVolume ×26 | cutMany ×4 | exportSTEP ×4, importSTEP ×4 | 24 |
| `TestCircularStockOverlappingOpposedAndSequentialBores` | `circular_stock_bore_test.go` | 1 | Circular stock | cut ×2, overlapVolume ×14 | cutMany ×8 | exportSTEP ×4, importSTEP ×4 | 12 |
| `TestCircularStockCoaxialOpposedSTEPAndThroughCut` | `circular_stock_bore_test.go` | 1 | Circular stock | overlapVolume ×2 | cutMany ×2 | exportSTEP ×1, importSTEP ×1 | 2 |
| `TestFullTubeSTEPAndCoaxialCut` | `tube_step_test.go` | 1 | Coaxial cut | cut ×3 | none | exportSTEP ×4, importSTEP ×1 | 1 |
| `TestCylinderSectorSTEPBridge` | `cylinder_sector_step_test.go` | 1 | Coaxial cut | cut ×1 | none | exportSTEP ×4, importSTEP ×2 | 1 |
| `TestQuarterAnnularSTEPAndFurtherCut` | `quarter_annular_step_test.go` | 1 | Coaxial cut | cut ×3 | none | exportSTEP ×3, importSTEP ×1 | 1 |
| `TestCoaxialIntervalUnion` | `coaxial_join_test.go` | 1 | Coaxial join | none | fuseCoaxial ×24 | circle ×36 | 0 |
| `TestCoaxialIntervalUnionContainmentTouchAndAtomicReject` | `coaxial_join_test.go` | 1 | Coaxial join | none | fuseCoaxial ×5 | circle ×6 | 0 |
| `TestCoaxialDecimalTranslatedJoin` | `coaxial_join_test.go` | 1 | Coaxial join | none | fuseCoaxial ×1 | circle ×2 | 0 |
| `TestContainingCylinderOverlapCapture` | `cylinder_containment_overlap_test.go` | 1 | Containment overlap | none | overlapVolume ×3 | none | 0 |
| `TestRevolveJoinCaptureContainedPlanarVolume` | `overlap_capture_test.go` | 1 | Containment overlap | none | overlapVolume ×3 | none | 0 |
| `TestIslandPocketFiniteRejectsAreAtomic` | `island_pocket_test.go` | 1 | Pocket with cylinder | none | rectangularIslandPocket ×4 | none | 0 |
| `TestPlainCADAnnularPedestalAndThroughBores` | `annular_pedestal_test.go` | 1 | Rounded profiles | cut ×1, fuse ×1, cutMany ×1 | overlapVolume ×1 | none | 0 |
| `TestCrossedBoreScopeAndSTEP` | `crossed_bore_test.go` | 2 | Crossed bores | cut ×2, overlapVolume ×4 | cutMany ×3 | exportSTEP ×6, importSTEP ×3 | 3 |
| `TestBlindCrossedBoreScopeAndSTEP` | `blind_crossed_bore_test.go` | 2 | Crossed bores | cut ×2, overlapVolume ×3 | cutMany ×2 | exportSTEP ×4, importSTEP ×2 | 2 |
| `TestDoubleBlindCrossedBoreScopeAndSTEP` | `double_blind_crossed_bore_test.go` | 2 | Crossed bores | cut ×8, overlapVolume ×8 | cutMany ×4 | exportSTEP ×4, importSTEP ×4 | 4 |
| `TestContainedCrossedBores` | `contained_crossed_bores_test.go` | 2 | Crossed bores | cut ×1, overlapVolume ×2 | cutMany ×4 | exportSTEP ×2 | 3 |
| `TestClippedCrossedBoreScopeAndSTEP` | `clipped_crossed_bore_test.go` | 2 | Crossed bores | cut ×12, overlapVolume ×12 | cutMany ×6 | exportSTEP ×6, importSTEP ×6 | 6 |
| `TestTripleCrossedBoreScopeAndSTEP` | `triple_crossed_bore_test.go` | 2 | Triple and companion | cut ×3, overlapVolume ×4 | cutMany ×1 | exportSTEP ×1, importSTEP ×1 | 3 |
| `TestBlindTripleCrossedBoreScopeAndSTEP` | `blind_triple_crossed_bore_test.go` | 2 | Triple and companion | overlapVolume ×18 | cutMany ×6 | exportSTEP ×6, importSTEP ×6 | 18 |
| `TestMultipleBlindTripleCrossedBoreScopeAndSTEP` | `blind_triple_crossed_bore_test.go` | 2 | Triple and companion | cut ×11, overlapVolume ×11 | cutMany ×4 | exportSTEP ×5, importSTEP ×3 | 11 |
| `TestCrossedCompanionBoreScopeAndSTEP` | `crossed_companion_bore_test.go` | 2 | Triple and companion | cut ×3, overlapVolume ×2 | cutMany ×1 | exportSTEP ×1, importSTEP ×1 | 1 |
| `TestCrossedObliqueBoreScope` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×2, overlapVolume ×3 | cutMany ×1 | exportSTEP ×1, importSTEP ×1 | 2 |
| `TestCrossedObliqueReciprocalScope` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×8, overlapVolume ×6 | cutMany ×2 | exportSTEP ×2, importSTEP ×2 | 4 |
| `TestCrossedObliquePairWithSeparatedOppositeZOpenings` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | none | cutMany ×1 | none | 0 |
| `TestCrossedObliqueBoreCoordinatePlacements` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | overlapVolume ×32 | cutMany ×32 | exportSTEP ×64, importSTEP ×32 | 32 |
| `TestBlindCrossedObliqueBoreSTEPAndFurtherCut` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×2, overlapVolume ×8 | cutMany ×14 | exportSTEP ×12, importSTEP ×2 | 6 |
| `TestConcealedCrossedObliqueBoreSTEPCanonical` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | overlapVolume ×12 | cutMany ×14 | exportSTEP ×6, importSTEP ×4 | 12 |
| `TestObliqueFloorCrossedBoreReimport` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | overlapVolume ×4 | cutMany ×6 | exportSTEP ×8, importSTEP ×2 | 4 |
| `TestDoubleBlindCrossedObliqueBore` | `crossed_oblique_bore_test.go` | 2 | Crossed oblique | cut ×2, overlapVolume ×8 | cutMany ×6 | exportSTEP ×8, importSTEP ×2 | 6 |

## Counts

| Tier | Family | Survey (15.1) | Functions in the files | Frozen | Dropped (15.2) | Not in the set | Held |
|---|---|---|---|---|---|---|---|
| 1 | Through bore | 5 | 4 | 4 | 0 | 0 | 0 |
| 1 | Blind bore | 13 | 13 | 11 | 0 | 2 | 0 |
| 1 | Bore layers | 4 | 4 | 4 | 0 | 0 | 0 |
| 1 | Bore union | 5 | 5 | 5 | 0 | 0 | 0 |
| 1 | Opposed blind | 5 | 5 | 4 | 0 | 1 | 0 |
| 1 | Oblique through | 3 | 3 | 0 | 0 | 1 | 2 |
| 1 | Disjoint bores | 5 | 5 | 0 | 0 | 0 | 5 |
| 1 | Side and corner cylinder | 12 | 12 | 7 | 0 | 0 | 5 |
| 1 | Side opening pair | 7 | 7 | 0 | 0 | 0 | 7 |
| 1 | Side dual disk | 8 | 8 | 6 | 0 | 1 | 1 |
| 1 | Circular stock | 7 | 7 | 1 | 0 | 1 | 5 |
| 1 | Coaxial cut | 4 | 6 | 0 | 0 | 3 | 3 |
| 1 | Coaxial join | 4 | 4 | 1 | 0 | 0 | 3 |
| 1 | Join all | 2 | 2 | 2 | 0 | 0 | 0 |
| 1 | Containment overlap | 2 | 2 | 0 | 0 | 0 | 2 |
| 1 | Pocket and planar | 9 | 7 | 7 | 0 | 0 | 0 |
| 1 | Pocket with cylinder | 5 | 5 | 4 | 0 | 0 | 1 |
| 1 | Rounded profiles | 3 | 3 | 0 | 2 | 0 | 1 |
| 1 | To face | 7 | 7 | 6 | 0 | 1 | 0 |
| 2 | Crossed bores | 5 | 5 | 0 | 0 | 0 | 5 |
| 2 | Triple and companion | 4 | 4 | 0 | 0 | 0 | 4 |
| 2 | Crossed oblique | 8 | 8 | 0 | 0 | 0 | 8 |
| 1 | **All of tier 1** | **110** | **109** | **62** | **2** | **10** | **35** |
| 2 | **All of tier 2** | **17** | **17** | **0** | **0** | **0** | **17** |
