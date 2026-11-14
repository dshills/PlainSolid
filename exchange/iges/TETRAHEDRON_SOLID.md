# Exact IGES manifold tetrahedron

`ReadTetrahedronSolid` accepts a distinct type186 material-solid family. Its
source graph contains one closed type514 shell, four type510 triangular faces,
four type508 loops, one shared six-edge type504 list and four-point type502 list.
Four affine bilinear type128 charts, six polynomial linear type126 model curves
and twelve independent type126 owner pcurves retain the original geometry.
This is a genuine closed solid, not a group of disconnected trimmed surfaces.

All source geometry scalars must be exact binary64 representations of their
original decimal values. The source must use full unit domains, degree one,
positive equal weights and exact clamping. Linear curves use an undeclared planar
flag and zero normal; tolerance-checked planar-normal metadata is not admitted. Each support's four poles must obey
P00+P11=P01+P10 as an exact rational identity. The triangle uses the chart corners
(0,0),(1,0),(0,1); its unused fourth control point defines the original affine
support. Every pcurve's endpoints must equal the exact owner coordinates of its
shared model edge. Because both the model and affine-composed pcurve are linear
with identical parameterization, these endpoint identities prove agreement over
the entire edge, not merely sampled coincidence.

The reader checks closed directed loops, unique ownership of every support,
model edge and pcurve, all six unordered vertex pairs, and all four distinct
triangular faces. An exact rational determinant places each opposite vertex
strictly inside its outward face half-space. These four inequalities prove the
nondegenerate convex tetrahedron's global embedding and material orientation.
Native `brep.NewSolid` is called only after these proofs. The result retains all
source topology IDs, geometry entities and directory labels/subscripts. Failure
returns a zero result; unsupported geometry is never partially imported.

`WriteTetrahedronSolid` accepts four represented independent vertices and an
explicit IGES length unit. It authors canonical polynomial supports/model curves/
pcurves with shared topology, including correct outward orientation after a
reflection. It validates the serialized graph with the same strict reader before
writing any bytes. If the fourth affine chart pole or any decimal source value
would lose exactness, staged validation rejects the write. The canonical writer
creates fresh source metadata; it does not claim to reproduce original labels or
header provenance.

The dialect is limited to 34 entities, 52 total control points, 4096 scalar values,
1024 fixed-width records and 64KiB, further restricted by caller limits. Transforms,
noncanonical directory ownership statuses, nonzero styles/levels, properties, void shells, holes, curved supports, arbitrary
UV triangles, rational nonuniform curve weights and tolerant repair are excluded.
Generic IGES geometry and earlier closed-lens APIs remain unchanged.

## Independent qualification

`tools/occtref/author_iges_tetrahedron.py` independently authors the retained
standard entity graph, without using the Go writer. It uses the four vertices
(0,0,0),(2,0,0),(0,2,0),(0,0,2). The separate C++ material reader performs ordinary
IGES `TransferRoots`, BRepCheck validity, solid/face counts and numerical mass.
Both that source and the Go writer output transfer through pinned OCCT revision
`4dbfdcc448c0b3e565488afcd626c84483172db9`: one solid, four faces, volume 4/3,
area 6+2√3, centroid (1/2,1/2,1/2). Tests use 1e-10 numerical comparison only
for the independent reference; native acceptance remains exact.

Build `build_iges_topology_solid.sh` and set `PLAINSOLID_OCCT_IGES_SOLID` to repeat
both directions. Mutation tests reject wrong shell/loop senses, topology flags,
changed supports, model edges or vertices, and undersized resource budgets.
Native reflection and atomic invalid-writer cases, race, Go1.23, package vet and
a fixed 100-execution fuzz campaign supplement the live reference.

## Type154 investigation boundary

The initial independent probe used `IGESSolid_Cylinder` to author a valid finite
cylinder entity (type154), height3/radius2/base(2,3,4)/axis+X. Pinned OCCT reads
its entity but ordinary `TransferRoots` produces no shape; its current IGESToBRep
translator has no type154 material branch. The retained `cylinder-solid.igs` and
`iges_cylinder_solid.cxx` probe document that limitation. No production type154
codec or claim of material interoperability was added. This evidence motivated
the independently transferred explicit type186 topology above.
