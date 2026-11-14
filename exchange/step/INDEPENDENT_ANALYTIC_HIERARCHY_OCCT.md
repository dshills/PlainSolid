# Explicit pinned-OCCT relationship convention

`ReadOCCTNormalizedAnalyticHierarchy` is a separate opt-in interpreter of the
qualified OCCT AP214 relationship convention. It does not relax
`ReadNormalizedIndependentAnalyticHierarchy`, whose neutral-child-frame gate is
unchanged. It retains the same three original material families and bounded
hierarchy grammar, exact metre/mm normalization, exclusive geometry ownership,
proper signed-coordinate frames and exact represented composed translations.
It does not assert a universal interpretation of arbitrary STEP assemblies.

## Source investigation

Pinned OCCT revision `4dbfdcc448c0b3e565488afcd626c84483172db9`:

- `src/DataExchange/TKDESTEP/STEPControl/STEPControl_ActorRead.cxx`,
  `ComputeTransformation`, lines 2453–2534: authenticates/sometimes swaps frame
  membership; converts the origin with the child context's units and the target
  with the parent context's units; calls
  `Trsf.SetTransformation(ax3Targ, ax3Orig)`.
- `src/FoundationClasses/TKMath/gp/gp_Trsf.cxx`, `SetTransformation`, lines
  181–202: uses the second frame's transposed basis and subtracts its origin.

Writing F=[Rf,pf] for the child/from frame and T=[Rt,pt] for the parent/to frame,
the effective transfer is exactly

    inverse(F) * T = [transpose(Rf)*Rt, transpose(Rf)*(pt-pf)].

Both origins above have already been converted to the same explicitly selected
physical length unit. This differs from the earlier neutral-frame normalization
extension `T * inverse(F)` whenever the relevant rotations/translations do not
commute. The original nonzero-origin counterexample is therefore now explainable,
not evidence for loosening exact geometry checks.

The new mode requires correct original frame membership and relationship
child/parent direction. It does **not** imitate OCCT's warning-and-swap repair.
Each original transformation must belong to one relationship. Both original
frames must independently be proper signed-coordinate rigid frames. Exact
rational composition must produce binary64-representable translation and a
bounded exact terminating decimal; otherwise the operation rejects.

## Original and derived evidence

The result contains `Normalized` (original source, original representation units,
normalized native hierarchy) and `Mappings`. Each mapping contains the original
ITEM_DEFINED_TRANSFORMATION ID, original from/to frame IDs, separately allocated
derived frame IDs, and the exact effective transform.

Original source records and IDs are retained untouched. In the derived graph,
all original frame IDs remain, with dimensional point values normalized. The
same original relationship/transformation IDs now reference an added neutral
child frame and effective parent frame, each explicitly listed in its owning
representation. The complete ordinary hierarchy proof then validates this
staged graph, source model geometry, shared local assets, paths and incidence.
No rounded support, pcurve or source frame is repaired.

This staging admits at most eight additional records per original relationship
(maximum 512). Both original and derived graphs obey caller entity/value/byte
limits; source counts cannot use staging to evade them. Active cancellation,
exclusive closure ownership, depth eight and 64 expanded leaves remain enforced.
No material union or spatial separation claim is introduced.

## Persistence and transfer

`Write/ReadOCCTNormalizedAnalyticHierarchyJSON` use distinct version-one tag
`plainsolid-step-occt-normalized-analytic-hierarchy`. The inner claim tag is also
separate. One original-source reproof authenticates the derived mapping table,
all original ancestry, normalized graph, composed paths and native local bodies.
Wrong original/derived IDs, altered transforms/native bodies, tags or unit claims
fail atomically. Prior formats keep their original meaning and bytes.

`WriteOCCTNormalizedAnalyticHierarchySource` re-proves and emits original source.
The existing `WriteIndependentAnalyticHierarchySource` emits
`result.Normalized.Hierarchy.Assembly.Source`, whose neutral frames match the
pinned consumer's transfer without requiring it to interpret the nonneutral
original frames again.

## Independent fixtures and checks

The retained nonzero-origin mixed fixture and
`independent-analytic-hierarchy-occt-frames.step` are authored independently of Go.
The new author script adds nonzero child origins and proper axis-cycle/negative
axes at two levels, with both directions of metre/mm context transitions.
It retains the genuine OCCT box and independent exact model-only cylinder/torus.

Ordinary pinned OCCT TransferRoots accepts each original, native-reloaded original
and emitted derived graph. All have five solids, 20 faces, 80 edge uses,
V=24+12π²+24π mm³ and A=52+24π²+40π mm². Their world bounds are respectively
[-10,-4,-2]..[30,31,9] and [-10,-7,-2]..[27,7,24] mm. Tests independently compute
known local solids' surface first moments and extremal boxes under reported
native occurrences and compare them with OCCT. This checks placements, not just
rotation-invariant mass. Oracle tolerances do not expand source acceptance.

Focused live/legacy regression, race, Go 1.23, vet, bounded 100-execution STEP and
native fuzz campaigns, wrong owner/frame/derived-ID/native claims, unsupported
rotations, huge exponents, cancellation and original-plus-derived limits are
qualified. Comparator:
`PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY=/tmp/plainsolid-independent-torus-assembly-oracle`.
