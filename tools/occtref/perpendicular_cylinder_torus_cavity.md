# Independent perpendicular toroidal cavity

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9. Build C++ with standard
non-STEP flags, set PLAINSOLID_OCCT_PERPENDICULAR_CAVITY and run
TestPerpendicularCylinderTorusCavity. Original torus major3/minor1, axis+X,
center(1,0,0), is subtracted from original cylinder radius6 axis+Z caps[-5,5].
Require one valid solid/two shells; original line X=1,Z=0 alongY has six roots
[-sqrt35,-4,-2,2,4,sqrt35] at1e-7 numerical comparison. Native mass/area/
centroid compare1e-6; analytic V=360pi-6pi², A=192pi+12pi², Fx=-6pi².
Native reload, positive closed display and global nonincident contacts checked.

Containment is exact and conservative: the torus lies in the radiusR+r ball
about its center. Positive c-R-r with (c-R-r)²>cx²+cy² proves radial separation;
centerZ±(R+r) are exact torus Z extrema for this perpendicular orientation.
Native clearances additionally exceed8*tolerance. Some geometrically valid
cavities outside this envelope reject. The original X-axis torus support uses
an exact signed-coordinate frame; no normalized floating oblique surrogate.
Point/line proof transforms only torus polynomial coordinates(Y-cy,Z-cz,X-cx),
including direction, while the stock cylinder keeps original coordinates.
No coaxial distance or inertia formula is inherited. General tilted rational
axes remain outside this first nonparallel native slice.
