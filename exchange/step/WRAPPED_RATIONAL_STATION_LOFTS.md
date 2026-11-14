# Weighted station loft STEP

`WrappedRationalStationLoftFromSolid`, `WriteWrappedRationalStationLoft` and
`ReadWrappedRationalStationLoft` exchange one canonical closed weighted-quadrant
station loft with 1–16 original wall bands and two terminal planar caps. Adjacent
bands may change taper, including a reentrant waist. Each band must pass the
existing complete wrapped-profile embedding proof; the complete native loft
must reprove equal full station curves and ordered, disjoint axial slabs.

The output uses original rational B-spline surfaces, closed rational B-spline
model rim curves, original wall pcurves and lifted planar B-spline cap pcurves.
Station rims are shared STEP edges with both original surface owners. Artificial
per-band caps are removed. Imported geometry is rebuilt from the decoded original
wall supports; it is never replaced by circular geometry. Old circular APIs and
their limits remain unchanged.

The reader checks full cap/model correspondence, seam ownership, station vertex
and edge identity, orientation, detached surfaces and native embedding. It
accepts this bounded paired dialect, not arbitrary NURBS solids. Input limits
bound entities, poles, faces, edge uses and per-proof arithmetic. The existing
reader divides its operation allowance by face count before its fixed set of
per-face proof passes; this is a per-pass allowance, not a total scalar-operation
counter. Writers use the existing bounded default proof options and stage and
revalidate all output before writing.

Numeric semantics match the existing represented binary64 STEP spline dialect:
decimal tokens are decoded to represented coefficients, and normalization must
preserve their exact represented weight ratios. This is not an arbitrary exact
rational decimal importer. The qualified fixture uses dyadic poles/weights and
integer knots, so its exported decimal geometry is exact as well. Original
caller profile-gauge/scale factors cannot be recovered from physical station
walls; only the physical supports and STEP ownership are retained. Native
quarter-edge IDs are regenerated, while original STEP full-rim IDs remain in
`Walls`.

Qualification uses asymmetric weights 1/2 and 3/4, nonuniform quadrant knots
−2,−1,1,4,8, heights 0,1,3, and physical control scales 3,1,2. Independent Green
integration gives volume `9*profileArea`, and axial centroid is exactly 71/54.
Live OCCT `step-mass-precise` readback independently validates the emitted BRep
and agrees within 1e-6 volume / 1e-7 centroid. Enable with
`PLAINSOLID_OCCT_ORACLE` and `PLAINSOLID_OCCT_STEP=1`; the reference build uses
pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9. This readback is not evidence
of accepting arbitrary OCCT-authored station loft dialects.
