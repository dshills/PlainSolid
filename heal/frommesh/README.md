# Indexed mesh to planar BRep

`Convert` creates shared topological edges directly from triangle vertex indices,
then calls exact same-domain `heal/unify`. Adjacent exactly coplanar oriented
triangles become polygon faces, including holes. It never welds equal or nearby
coordinates, fits curved surfaces, or treats nearly planar triangles as coplanar.
The resulting geometry uses only original vertex coordinates.

`TriangleFaces` maps every input triangle to its output polygon;
`FaceTriangles` is the inverse mapping in ascending input order. `VertexMap`
maps original vertices to retained output indices or `-1` for unused or removed
collinear vertices. Original triangle order establishes deterministic face
ordering through unification. Returned slices are caller-owned.

Only closed indexed, embedded, oriented material boundaries are accepted.
Cavity shells must point inward relative to their enclosed void; outer shells
must point outward. Nonmanifold/open inputs, self intersections, coincident
shells, inconsistent material orientation and ambiguous same-domain junctions
fail explicitly. No partial conversion is returned. Empty meshes are supported.

Default input caps are 16384 vertices and 8192 triangles; unification has its own
face, triangle, predicate and collision budgets. Preparation checks context
between bounded steps. Mesh validation is size-bounded and not internally
interruptible. Hole complexity and required collinear junction restrictions
are inherited from the planar BRep and unification packages.
