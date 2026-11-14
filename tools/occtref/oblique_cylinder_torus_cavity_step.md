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

A separate exact whole-occurrence API preserves both original local rational
torus frame and cylinder chart. It uses the already-qualified separately related
standard source coordinate datum; missing/duplicate datum and inconsistent world
geometry reject. All original model points/directions and that datum transform
exactly, while source pcurves remain unchanged. The optional `oblique` oracle
argument applies another proper3/5–4/5 world rotation plus(1/10,-2/5,3/8) to
both independently authored OCCT primitives. The torus then has world axis
(24/25,0,-7/25), distinct from the stock axis(4/5,0,3/5). Imported support axis,
solid/shell counts, inward orientation, mass and centroid are independently
checked. Run TestObliqueCylinderTorusCavityObliqueOccurrence. Local finite-decimal
center(1/10,1/5,1/8) and dyadic caps[-5.5,6.25] also roundtrip exactly.
