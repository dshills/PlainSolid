# Exact rational spherical hierarchy occurrences

`ReadRationalSphericalHierarchy` is a distinct common-unit mode for the bounded
planar/cylinder/torus plus optional sphere hierarchy. Local native materials are
unchanged. Proper rational orthonormal occurrence targets, including 3:4:5
Pythagorean directions, are retained separately from those materials. No
approximate `geom.Affine` is returned for rational placements.

The source grammar retains 4–16 products, at most 64 original relationships,
eight levels, 64 expanded leaves, required shared torus and exclusive local
geometry. Units must be common explicit metre/mm. Child mapping frames must be
neutral. This avoids any ambiguity between the earlier source and explicit OCCT
from/to conventions: both give the target matrix for a neutral child frame.
Mixed units and nonneutral child frames remain outside this mode.

## Rational and original-source proof

Raw X/Z directions must be exactly perpendicular and have rational lengths.
Their normalized axes and origins must have exact terminating-decimal values;
a 3:4:5 frame qualifies, while a 5:12:13 normalized coefficient does not qualify
under this narrower finite-decimal contract. Every matrix coefficient and every
composed prefix is bounded to 256-bit numerator/denominator and the existing
64-character/64-decimal-place exact formatter. No source coordinate is fitted,
snapped or rounded. Nearly perpendicular authored decimals fail.

`Relationships` contain immutable `ExactNAUOOccurrence` poses and original
NAUO/CDSR/relationship/transformation/from/to/parent/child IDs. `Instances` contain
the original complete NAUO paths and immutable exact composed poses. `Matrix`
and `ApplyExact` return independent rational copies. `ApplyRepresented` succeeds
only if the resulting coordinates are exactly binary64-representable; it does
not silently round a rational placement. An invalid zero occurrence fails.

Before local-material proof, each source representation-frame dependency closure
is proved disjoint from every MANIFOLD_SOLID_BREP closure. A **private** neutral
placement view replaces only those authenticated frame dependencies; it leaves
all material geometry, topology, ownership links and source product/relationship
IDs unchanged. The existing strict spherical hierarchy validates the complete
view. Every original record is either retained in that complete proof or belongs
to a validated original representation-frame closure. Sharing a frame dependency
with actual material geometry is explicitly rejected. No private view is exposed
as the original placed assembly or through the source-export API.

Public local materials retain original source closures. The independently
constructed native local bodies are byte-identical to the earlier cardinal
fixture's bodies. Only separate exact occurrences change world placement.
The native analytic sphere remains the already qualified angular sphere, not a
claim of exact rational surface sewing.

## Persistence and re-export

`Write/ReadRationalSphericalHierarchyJSON` use distinct version-one tag
`plainsolid-step-rational-spherical-hierarchy`. The envelope retains original
source, original definitions/roots, exact decimal relationship/path matrices,
and separately identified private local proof/native claims. One original-source
reproof checks all stored fields. Stored native bodies or the private neutral
view never substitute for original pose verification.

`WriteRationalSphericalHierarchySource` re-proves and re-emits actual original
source and placements. This milestone provides source-preserving transfer, not
arbitrary native occurrence editing. Existing signed-coordinate and OCCT-mode
APIs and native meanings are unchanged.

Input remains bounded to 1 MiB/4096 entities/65536 values/depth16, at most256
representation frames, with caller limits, cancellation and bounded rational
composition. Native envelopes retain the 16 MiB and per-local1024-record cap.
Failures return no partial result and writes validate before output.

## Independent evidence

`author_step_rational_spherical_hierarchy.py` independently changes original
AP214 target directions to exact integer ratios. One shared analytic group's
frame uses X=(3,0,-4), Z=(4,0,3), giving a genuinely tilted cylinder and torus.
One sphere occurrence uses X=(3,4,0), Z=(0,0,1). Original local materials and
relationship IDs are retained. A composed cylinder origin is exactly
(144/5,0,-17/5), despite not being binary64-representable.

Pinned OCCT `4dbfdcc448c0b3e565488afcd626c84483172db9` accepts original and native
re-exported actual source: seven solids,22 faces,88 edge uses; V=24+12π²+
(24+64/3)π mm³, A=52+24π²+72π mm²; world bounds [-23,-20,-5]..[162/5,30,15] mm.
Independent analytic support extents and original-solid surface first moments
qualify placements, not merely rotation-invariant mass. Numerical comparison
tolerance is not part of source acceptance.

Tests include original/local byte identity, immutable rational copies,
nonrepresentable point rejection, exact composition, native/source reload,
nonterminating normalized axes, nearly perpendicular input, frame/body sharing,
foreign owner and nonneutral child rejection, matrix/path/native tampering,
limits and cancellation. Qualification uses live OCCT, recent legacy regression,
race, Go1.23, vet and fixed100-execution STEP/native fuzz campaigns.
