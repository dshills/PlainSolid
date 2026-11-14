# Weighted station through-bore STEP

`WrappedRationalStationBoreFromSolid`, `WriteWrappedRationalStationBore` and
`ReadWrappedRationalStationBore` preserve a bounded canonical through-bore with
one normalized rational weighted profile, one axis, equal terminal planes and
strictly separated inner/outer scales. Both wall grids may differ and may have
creases. Each source has 1–16 bands; the refined native result must also pass
the existing 16-band-per-side source proof. Contacts, changing profile shape,
unequal end planes and unsupported refinement are rejected.

The existing circular bore staging and strict adjacency proof are reused through
private dispatch. The new path authenticates every source wall with the weighted
station reader, then uses `CombineCoaxialWrappedRationalStationLofts` and
`ValidateWrappedRationalStationBore` to independently reprove the material. It
never infers nesting from a sampled contour. The exported inner surfaces remain
original weighted B-splines with inward orientation; the two terminal caps have
real outer/inner B-spline loops. Cap pcurves and shared rim model curves must
agree over their entire domain. Original STEP rim/vertex IDs are restored after
temporary proof-view decomposition; native quarter topology is regenerated.

The writer ignores cached Solid and public entity IDs, stages/revalidates the
complete output and writes only after success. Input entity/pole/face/edge limits
remain enforced. Reader work is divided between the two loft proof views, then
by each view's face count, as in the filled dialect; fixed native reproof passes
retain their own bounded arithmetic options. This is not a total scalar-operation
counter. Decimal/binary64 semantics and original profile-gauge limitations are
identical to WRAPPED_RATIONAL_STATION_LOFTS.md. Circular APIs are unchanged.

The independent qualification uses outer heights 0,1,3 / scales 3,1,2 and inner
heights 0,2,3 / scales 1/2,3/4,1/2, on an asymmetric 1/2,3/4 weighted profile.
The two grids are genuinely different. Integrating the squared linear station
scales gives volume `(125/16)*profileArea` and exact axial centroid `383/300`.
The profile area oracle is separate Green quadrature, not native evaluation.
Pinned OCCT precise STEP readback validates the emitted BRep and agrees within
1e-6 volume and 1e-7 axial centroid. Run `TestWrappedRationalStationBoreSTEPOCCT`
with the same oracle environment as the filled loft. This is paired export
readback evidence, not arbitrary independently authored OCCT-dialect acceptance.

Tests also cover a single band per side, original support data and source IDs,
inward faces after native JSON reload, deterministic reexport, missing annular
hole, wrong inward orientation, wrong pcurve owner, zero clearance, arithmetic
quotas, and atomic failure. No tessellation is used by this exchange proof.
