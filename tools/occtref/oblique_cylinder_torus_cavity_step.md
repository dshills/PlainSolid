# Exact oblique analytic cavity STEP

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9, standard STEP build
flags. Set PLAINSOLID_OCCT_OBLIQUE_CAVITY_STEP and run
TestObliqueCylinderTorusCavitySTEP. Original OCCT cylinder8 axisZ caps[-6,6]
minus original torus3/1 centered(1,0,0), axis(4/5,0,3/5), X(3/5,0,-4/5).
Require valid imported one solid/two shells, reversed torus face, actual oblique
support axis agreement1e-12, mass/area1e-6 and centroid1e-7 versus original Cut.

Export re-proves the exact rational source frame and sphere-envelope containment,
transforms only torus model points/directions exactly, and retains original
cylinder/planes, 2D pcurves and shared circular seam topology. Finite exact decimal
pose/model coordinates required; dyadic dimensions bounded2^20 with represented
R+r and cap height. Standard BREP_WITH_VOIDS carries analytic supports, never
sampled curves. Existing represented2pi seam convention remains explicit.

Strict paired read recovers torus X/Z/origin from actual support placement and
Y=Z crossX, proves proper unit rational frame without normalization, recovers
cylinder/cap scalars, then rechecks every regenerated graph entity as exact
numeric atoms. Mutation of frame, pcurve, support or orientation rejects.
Source getter deeply owns rationals. Parsed recipe provides exact analytic query;
no floating geom.Frame native oblique BRep is claimed. Parser bounds512entities,
16384values/2MiB and rational transformations4096bits. Generic OCCT reordering/
healing dialects remain outside paired import; no vendor source payload is used.
