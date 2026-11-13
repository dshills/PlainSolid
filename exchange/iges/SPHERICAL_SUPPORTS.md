# Standalone exact sphere supports

`ReadSphericalSupports` and `WriteSphericalSupports` exchange IGES196 form1:
center point116, positive radius, axis123 and reference123. `Geometry` returns
`geom.SphereSurface`. Longitude U is measured around the axis from the reference
meridian; latitude V ranges over [-π/2,π/2]. Longitude is periodic and the two
poles are parameter singularities. These natural analytic conventions do not
assert any authored finite trim, closed shell, material orientation or solid.

The explicit frame must contain perpendicular signed coordinate unit vectors.
No direction normalization or projection changes the source. Original geometry
scalars must convert exactly from decimal to float64; staged writer read-back
enforces the same restriction. Labels and subscripts of all dependencies and
explicit length units survive; coordinates are not rescaled. Presentation style
is outside this descriptor and geometry usage status is emitted canonically.

Implicit form0 frames, placements, topology, shared dependencies and unused
entities fail. At most 64 surfaces and 256 entities are accepted, in addition
to the ordinary record/value/byte limits. Existing plane, cylinder, generic and
topology entry points retain their earlier acceptance rules. This API does not
claim general IGES sphere-face or solid import.

The retained source fixture and optional `PLAINSOLID_OCCT_IGES_SPHERE` helper
use independently authored OCCT type196 entities and direct analytic transfer.
Both axis signs and a nondefault reference meridian are qualified by points
and both derivatives. Native tests cover natural latitude rejection and pole/
periodic evaluations. Numerical evaluations do not serve as source-identity
proofs: that acceptance uses exact source scalars and the explicit frame gate.
