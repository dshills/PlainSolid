# Axially and radially translated toroidal cavity

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9; build with standard
non-STEP flags. Set PLAINSOLID_OCCT_TRANSLATED_CAVITY and run
TestTranslatedCylinderTorusCavity. Independent torus5/2 at(1,1,3), original
cylinder10 caps[0,7], actual Cut. Require one valid solid/two shells, six
section parameters at Y=1,Z=3, and native mass/area/centroid agreement1e-6.

Exact source reduction shifts Z and both original cap coordinates by centerZ,
requiring represented differences rather than rounding. Native assembly keeps
original cap and torus frames. Strict radial/axial separation remains exact,
with8*tolerance native margin. All source masks and line t remain unchanged.
Distance APIs are not silently reused as world charts.

The measure adapter propagates shifted exact raw pi/pi² coefficients by exact
rational translation, then returns original-chart intervals. Source coefficients
remain explicitly labeled shifted-chart. Analytic tests check Fz=2450pi-120pi²,
Qxz=-120pi², Qzz=34300pi/3-400pi². Original affine bounds use the outer cylinder
alone, giving canonical Z[0,7], independent of cavity center.
