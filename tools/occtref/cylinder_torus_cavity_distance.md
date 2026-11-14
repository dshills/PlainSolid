# Original toroidal cavity boundary distance

Pinned OCCT source 4dbfdcc448c0b3e565488afcd626c84483172db9.
Build cylinder_torus_cavity_distance.cxx with the standard non-STEP OCCT flags.
Set PLAINSOLID_OCCT_CAVITY_DISTANCE and run TestCylinderTorusCavityWorldDistance.
The independent oracle builds original cylinder radius8 caps[-3,4] minus
original torus major5/minor2. It minimizes point-to-face distances over all
retained faces, deliberately not point-to-solid distance (which is zero inside
material). The test compares squared distance with certified rational brackets,
allowing1e-7 only for the finite-precision independent comparator.

Native results certify every retained support lower bound and attained upper
witness. ExactOwners is a complete mask only when all possible supports have
proved equal rational minima; other ties remain explicitly unresolved.
Proper rational world mapping preserves the interval, original owner and
symbolic point/circle witness. ExactOperations counts source work; separate
world transforms have4096-bit bounds. Limits fail atomically.
