# Original eccentric cavity mass tensors

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9. Build with standard
non-STEP flags. Set PLAINSOLID_OCCT_ECCENTRIC_CAVITY_MOMENTS and run
TestEccentricCylinderTorusCavityMoments.

Original cylinder radius10 caps[-3,4] or[-4,3] minus original torus5/2 at
local(1,-1,0). Both are independently placed by X=(3/5,0,-4/5),
Z=(4/5,0,3/5), translation=(1/10,-2/5,3/8), then OCCT Cut is integrated.
All world centroid and central inertia entries are compared within1e-6 of
certified native intervals. Native raw coefficients remain exact; numerical
OCCT tolerance is not a source proof tolerance.

Tests independently check negative removed Fx=-40pi², positive Fy=40pi²,
positive Qxy=40pi² (subtracting negative offset product), and torus Qxx
coefficient-600. Centralization subtracts full FiFj/V, not a diagonal shortcut.
World tensor transformation uses all nine source entries. Source/mapping
budgets fail atomically. Canonical cavity and query APIs remain unchanged.
