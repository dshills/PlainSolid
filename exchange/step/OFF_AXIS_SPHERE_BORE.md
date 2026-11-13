# Off-axis spherical bore exchange boundary

The independently authored `testdata/off-axis-sphere-bore-occt.step` was generated with pinned OCCT commit `4dbfdcc448c0b3e565488afcd626c84483172db9`: subtract a radius-2 cylinder on axis `(1,0,z)`, starting at `z=-0.75` with height 7, from a radius-5 sphere centered at the origin. OCCT's BRepCheck_Analyzer accepts the result; STEPControl_AsIs writes 643 entities.

The standalone fixture generator is `tools/occtref/off_axis_sphere_export.cxx`; it uses the same pinned include/library tree as `tools/occtref/build.sh` and links the STEP, Boolean and primitive toolkits. It does not use native PlainSolid geometry.

Its shared intersection edge #21 references SURFACE_CURVE #24, whose model curve #25 is a nonrational degree-7 B-spline. The retained analytic surfaces are SPHERICAL_SURFACE #197 and CYLINDRICAL_SURFACE #376. This is a tolerance-based intersection representation, not an exact rational representation of the original intersection.

On each knot span this nonrational spline is polynomial. A nonconstant real polynomial vector cannot lie identically on a finite sphere: the highest-degree coefficient in its squared norm is a positive sum of squares. At least one nonconstant span therefore fails the sphere equation. Even the represented clamped first endpoint `(3,-4.440892098501E-16,4)` has a strictly positive exact decimal residual in `x²+y²+z²−25`. Matching supports, topology and a small sampled residual cannot convert that approximate edge into an exact seam proof.

The strict analytic STEP body reader rejects this source. No recipe is inferred from approximate trims. The separate `exchange.WriteSphereBoreRecipeJSON` / `ReadSphereBoreRecipeJSON` format preserves the defining native analytic recipe and full native boundary and verifies reconstruction; it is not a STEP interchange claim. Native incidence and evaluation retain their documented tolerance contracts.
