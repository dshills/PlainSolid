# Explicit nonneutral rational OCCT hierarchy frames

`ReadOCCTRationalSphericalHierarchy` extends the rational hierarchy through a
separate opt-in API and version-one native tag,
`plainsolid-step-occt-rational-spherical-hierarchy`. The neutral-child rational
API/tag and signed-coordinate APIs keep their original meanings and reject the
new nonneutral fixture.

## Qualified convention and exact scope

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` uses
`STEPControl_ActorRead::ComputeTransformation` →
`gp_Trsf::SetTransformation(target, origin)`. As documented in
`INDEPENDENT_ANALYTIC_HIERARCHY_OCCT.md`, this is

    inverse(child/from frame) * parent/to frame.

The new mode evaluates that formula with exact rational arithmetic, then
composes every root-to-leaf prefix exactly. Both original frames must be proper
rational orthonormal frames; normalized coefficients, translations, relative
poses and prefixes must pass the existing 256-bit and terminating-decimal
64-character/64-place gates. Noncommuting rotations and nonbinary terminating
origins are supported without converting them into a `geom.Affine`.

Common explicit metre/mm units only are admitted. This is not a mixed-unit
extension, a universal STEP relationship interpretation, or OCCT's
warning-and-swap repair. Original frame membership, child/parent relationships,
geometry ownership and bounded hierarchy remain mandatory.

The existing disjoint private neutral-placement view proves local material and
DAG topology without changing native local geometry. Every original frame
closure must be separate from every material closure. Original source,
NAUO/CDSR/relationship/transformation/from/to IDs and complete paths remain
public evidence. The private view is never exported as the original assembly.

Immutable `Matrix` and `ApplyExact` results are independent rational copies;
`ApplyRepresented` still rejects any coordinate requiring binary64 rounding.
There is no rounded world-geometry cache or baked solid claim.

## Persistence and transfer

`Write/ReadOCCTRationalSphericalHierarchyJSON` use the separate tag above and one
original-source reproof. Exact relative and path matrices, original IDs and
native local bodies are authenticated. A stored matrix or private proof cannot
replace original-source reconstruction. Old tags are not inferred or upgraded.

`WriteOCCTRationalSphericalHierarchySource` re-proves and emits original source
records. Thus STEP → exact native envelope → original STEP preserves the actual
consumer placement convention. Arbitrary authored native pose edits and a new
neutral-frame source generator are not part of this milestone.

Prior bounds remain: 1 MiB/4096 source entities/65536 values/depth16, at most256
representation frames,16 products,64 original uses,8 hierarchy levels,64 expanded
leaves,16 MiB native envelope and caller limits. Common local families remain
planar/cylinder/torus plus an optional sphere. Failures are atomic, cancellation
is active, and local materials are not spatially unioned.

## Independent fixture

`author_step_occt_rational_spherical_hierarchy.py` independently adds a child
frame with origin(1/10,-2,3), X=(3,4,0), Z=(0,0,1) beneath a different oblique
parent frame. Another hierarchy level uses X=(1,0,0), Z=(0,3,4) and nonzero origin.
All source units are mm. The local body records are unchanged from the earlier
rational assembly. These rotations do not commute.

A relative translation is exactly(677/50,-368/25,-3). A composed torus center is
exactly(3169/250,-2821/125,33/25). Tests check both before consulting OCCT, and
verify immutable rational copies and unchanged local native bytes.

Pinned ordinary OCCT TransferRoots accepts original and native-reexported source:
7 solids,22 faces,88 edge uses, V=24+12π²+(24+64/3)π mm³ and
A=52+24π²+72π mm². World bounds are approximately
[-23,-27.0990117466,-11.8]..[28.4945369760,16.1367498170,15] mm.
Tests compute analytic sphere/cylinder/torus support extrema and surface first
moments under the reported exact poses independently of source pose helpers.
The comparison includes position/orientation, not only invariant mass. Numerical
comparison tolerances never expand source acceptance.

Focused live and legacy regressions, race, Go1.23, vet and two fixed100-execution
fuzz campaigns qualify the slice. Adversaries include nonorthogonal and
nonterminating child frames, foreign frame ownership, frame/body sharing, huge
exponents, stale original/from/path/matrix/native claims, wrong tag, source/native
limits and cancellation. The old neutral-child reader explicitly rejects the
positive new source.
