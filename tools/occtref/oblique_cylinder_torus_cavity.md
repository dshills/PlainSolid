# Exact rational independently oblique torus cavity query

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9; standard non-STEP build.
Set PLAINSOLID_OCCT_OBLIQUE_CAVITY and run TestObliqueCylinderTorusCavity.
Original cylinder8 axisZ caps[-6,6] minus original torus3/1 centered(1,0,0),
local X=(3/5,0,-4/5), local Z=(4/5,0,3/5). Actual OCCT Cut must be valid,
one solid/two shells; independent point IN/OUT/ON and six line contacts along
local X [-7.5,-4,-2,2,4,7.5] agree (1e-7 numeric event threshold).

Pure-Go proof stores exact rational orthonormal determinant+1 frame, proves
strict containment by the radiusR+r sphere envelope, and evaluates original
cylinder polynomials plus exactly inverse-transformed torus quartic. Rational
pose/center is immutable; constructor and line/point ledgers are bounded and
atomic. Some valid shapes outside the conservative envelope reject. This is
an exact analytic material/query API, not a native oblique BRep factory.
Existing native Torus takes geom.Frame with floating normalization; importing
that as exact3/5–4/5 source would silently replace the authored orientation.
A source-preserving rational analytic surface adapter is a separate prerequisite.
Perpendicular/coaxial native APIs and STEP dialects remain unchanged.

Exact raw mass coefficients are now independently qualified by the same oracle's
complete central tensor output. TestObliqueCylinderTorusCavityMoments compares
volume, centroid and all nine central inertia entries at1e-6. Torus raw tensor
is Qtorus=Qradial*I+(Qaxis-Qradial)*n*n^T+Vtorus*center*center^T before removal;
this independently yields retained Qxz coefficient+333/25*pi² for the fixture.
Full first moment centralization follows subtraction. Coefficients are exact;
pi and quotient-dependent centroidal values have rational outward intervals.
The world occurrence tensor uses full exact rotation/translation composition;
TestObliqueCavityWorldMoments qualifies translated cross terms and invariant
central inertia, including atomic mapping budget. BoundAffineRange reuses only
the original outer cylinder support, valid because strict containment excludes
the torus from all extrema; irrational sheared support is explicitly enclosed.
No native floating-frame oblique BRep is claimed by these measures.
