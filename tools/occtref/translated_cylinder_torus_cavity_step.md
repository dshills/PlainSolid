# Three-coordinate torus-center cavity STEP

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9. Standard STEP build
flags; set PLAINSOLID_OCCT_TRANSLATED_CAVITY_STEP and run
TestTranslatedCylinderTorusCavitySTEP. Independently construct cylinder10
caps[0,7] minus torus5/2 centered(1,-1,3); compare transfer validity, one
solid/two shells, inward torus face, volume/area1e-6 and centroid1e-7.

The distinct local dialect recovers all three torus-center coordinates from
actual analytic placement, and both cylinder cap stations independently.
Complete graph reproof verifies translated curves/pcurves and orientation,
with no vendor source payload. Strict exact containment and representable seam
arithmetic remain native gates. Dyadic scalars bounded2^20, parser512entities,
16384values/2MiB, strict paired encounter order. Existing local eccentric,
coaxial and placed APIs are unchanged. No general healed STEP import claim.

## Independent source frame for occurrences

Arbitrary local centerZ and world translation cannot both be recovered from
boundary geometry alone: their axial gauge is unobservable. The separate
occurrence dialect therefore writes an actual AXIS2_PLACEMENT_3D in a related
SHAPE_REPRESENTATION, linked by standard REPRESENTATION_RELATIONSHIP. Its
origin/axes are transformed exactly with all 3D geometry. Import requires this
unique datum, pulls back every model point/direction, reconstructs the original
three source center coordinates and caps, and rechecks the entire world graph.
Missing/duplicate datum, nonterminating pose decimals or inconsistent geometry
reject atomically. A reader stripping the coordinate representation cannot
recover this source recipe and is not accepted by paired import.

The frame is deliberately NOT an item in ADVANCED_BREP_SHAPE_REPRESENTATION:
OCCT applies such an item as an additional placement. An independent centroid
check caught that double-placement and qualifies the separate representation
instead. TestTranslatedCylinderTorusCavityObliqueOccurrence transfers the exact
3/5–4/5 pose plus(1/10,-2/5,3/8), checks one solid/two shells/inward torus and
compares to independently authored original OCCT primitives at that pose.
