# Eccentric analytic cavity STEP

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9. Build the C++ fixture
with standard STEP reference flags, set PLAINSOLID_OCCT_ECCENTRIC_CAVITY_STEP,
and run TestEccentricCylinderTorusCavitySTEP.

The reader independently requires one valid solid/two shells and an inward
TOROIDAL_SURFACE. Compare imported volume/area/centroid against original OCCT
Cut of cylinder10 caps[-3,4] and torus5/2 centered(1,-1,0), with1e-6 mass/area
and1e-7 centroid thresholds. Native exact proof remains independent.

The separate paired dialect recovers torus offset from its actual placement,
retains cylinder placement and cap positions, and regenerates every entity.
Exact decimal equality plus complete graph matching rejects inconsistent
translated curves, changed pcurves/supports, detached topology or inverted void.
Canonical encounter order is required; this is not general OCCT-healed import.
Dyadic source offsets and dimensions are bounded by2^20 and exact seam-sum
representability; strict native clearance exceeds8e-9. Parser caps512entities,
16384values,2MiB and scalar limits are inherited from the paired primitive.

The separate occurrence dialect recovers the source origin by projecting the
original torus center onto the cylinder axis in that center's axial plane.
This preserves eccentric offset rather than conflating it with world pose.
Exact proper finite-decimal rotations/translations are checked and every world
model point/direction is pulled back before canonical proof and forward graph
comparison. No rounded world native solid is claimed. The independent oracle's
optional `oblique` argument builds both original primitives at X=(3/5,0,-4/5),
Z=(4/5,0,3/5), translation=(1/10,-2/5,3/8); run
TestEccentricCylinderTorusCavityObliqueOccurrence with the same environment.
