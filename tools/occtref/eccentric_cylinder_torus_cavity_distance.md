# Eccentric cavity nearest boundary

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9; build the C++ fixture
with standard non-STEP flags. Set PLAINSOLID_OCCT_ECCENTRIC_CAVITY_DISTANCE
and run TestEccentricCylinderTorusCavityWorldDistance.

Independent original torus5/2 centered(1,0,0), cylinder10 caps[-3,4].
Distance is minimized over actual Cut result faces, not solid interior.
Queries cover distinct torus/wall feet tied at squared distance1, irrational
radial norm, cap/wall shared rim, and the torus's original periodic seam at
(8,0,0). Comparator tolerance1e-7 is separate from the rigorous brackets.

Each source support has an explicit chart origin; torus and cylinder azimuths
are never conflated. Proper-rigid world witnesses map these origins and radial
vectors independently, preserving bounds and owners. ExactOwners reports only
proved complete rational ties. All other ambiguity remains PossibleOwners.
One source arithmetic ledger bounds both radial charts and cell refinement.
