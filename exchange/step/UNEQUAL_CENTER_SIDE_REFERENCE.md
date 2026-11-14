# Independent unequal-center side opening reference

This qualification is independent of prospective native construction or STEP
writer/reader acceptance. The standalone overlapping-side-pockets OCCT helper
adds `unequal-center-side` and `unequal-center-through`. It builds a box, fuses
two independent cylinders and cuts the box, then unifies same-domain faces.
It uses the pinned OCCT revision in `tools/occtref/README.md`; all emitted and
reloaded bodies must pass BRepCheck and ordinary STEP TransferRoots.

Stock is [0,60] × [0,10] × [0,6]. Circle centers/radii are (7,-3)/5 and
(30,-20)/29. Their supplied upper intersection is (10,1); the other exact
intersection (1659/409,-2880/409) lies outside stock. Side opening intervals
are [3,11] and [9,51], giving the retained opening [3,51]. The exposed arcs
run from (3,0) to (10,1), then to (51,0).

These are exactly quarter circles: first endpoint radii (-4,3),(3,4) have zero
dot product, cross product -25 and squared norm 25. Second endpoint radii
(-20,21),(21,20) similarly have zero dot product, cross product -841 and
squared norm 841. Each arc is the clockwise minor quarter. The triangle between
these three points has area 24. Adding both circular segments gives removed
section area S=(25+841)π/4−(25+841)/2+24=216.5π−409. Exposed curved wall
length is (5+29)π/2=17π and the opening width is 48.

For depth d, volume is 3600−dS. Blind surface area is 2040+d(17π−48);
through additionally removes 2S. The independent oracle cuts from z=3 upward
for the blind case and spans the whole stock for through; native bottom-entry
comparisons must explicitly reflect source Z, not silently translate the floor.

| Mode | Volume | Surface area | Source centroid |
|---|---:|---:|---|
| blind | 2786.5355714934303 | 2056.2212253330799 | (30.254190845114884,5.4237261732495003,2.5621097913686786) |
| through | 1973.0711429868713 | 1530.1328316617905 | (30.717979008895039,6.1968428594471083,3) |

Both retained OCCT files have 582 entities, 9 unified faces, 7 planes and 2
cylinders. Tests verify source structure and legacy typed-reader rejection.
An opt-in live test compares both freshly generated and standard re-imported
OCCT shapes against the independent formulas above (1e-7 absolute measure bound).
Centroids are reported reference data, not asserted by the closed-form area test.
No vendor angular pcurve or rounded source trim is accepted by this milestone.
