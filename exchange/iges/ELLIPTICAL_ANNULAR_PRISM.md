# Exact hollow elliptical prism IGES material

The separate `Read/WriteEllipticalAnnularPrismTopology` codec authenticates one
canonical28-record model-only186 graph. Two distinct122 supports extrude separate
full104 ellipses; one504 owns six edges and one502 owns four seam vertices.
Two510 lateral faces have opposing material orientation, and two190-backed cap
faces have explicit outer and negative inner508 loops. The514 owns all four
faces with declared senses. Every source scalar, model support, endpoint,
ordered coedge, inner loop and reference is compared to the recovered canonical
graph. No pcurves are present or inferred. The original20-record filled prism
reader remains strict and rejects this graph.

The domain is positive-Z, centered and aligned. Both conic atoms retain exact
source coefficients and rational-square radii; inner major/minor are separately
strictly smaller than the corresponding outer radius. This is a continuous
strict nesting proof and permits non-homothetic profiles. Zero height,
nonrepresentable height arithmetic, touching/intersecting profiles, alternate
encodings and rounded vendor substitutions fail without repair.

`ReadEllipticalAnnularPrismMaterial` promotes the authenticated source directly
through the neutral `internal/ellipticprism.Annulus` builder. It reuses exact
quarter geometry and proves each continuous model/pcurve join with native Sew.
There are ten native faces: lower annular cap, upper annular cap, four outer
walls, four reversed inner walls. Source face IDs are51,49,45×4,47×4. Native rim
IDs are indexed[outer/inner][lower/upper][quarter], distinct from original full
rim curve IDs{{23,19},{29,25}}. The positive extrusion of strictly nested simple
ellipses supplies the constructive material proof. No STEP dependency exists.

The separate version1 `Write/ReadEllipticalAnnularPrismMaterialJSON` persists
source, full native trimmed data and partition ancestry. Reload re-proves the
source and rebuilds canonical native geometry before comparing all data. Stale
geometry, source role IDs, native rims and unsupported formats fail atomically.
Source limits are32KiB,512records,28entities,2048values; envelope512KiB maximum,
further bounded by caller limits. Positive native tolerance never snaps source
geometry. Output is staged before I/O; ordinary I/O failure may partially write.

Independent `author_iges_elliptical_annular_prism.py` creates the source without
Go helpers. Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9 accepts original
and native-reloaded generated output as four faces and one solid. Outer5×2,
inner2×1, Z[3,7] gives volume32π, centroidZ5 and area181.07172572228598.
The existing standalone `iges_elliptical_prism_topology` comparator integrates
original support derivatives independently, avoiding inaccurate default area
quadrature. Native mass agrees before/after plain trimmed reload; continuous
edge bounds are exactly zero. A closed native mesh passes at deflection0.1,
MaxSubdivisions256, MaxTriangles1000000; the smaller disk-prism mesh budget at
0.03/64/100000 was insufficient and is not claimed here.

Focused normal/race/Go1.23/vet,100x source and native fuzz campaigns, old filled
prism/STEP regressions, and live optional
`PLAINSOLID_OCCT_IGES_ELLIPTICAL_PRISM_TOPOLOGY` qualify this bounded scope.
