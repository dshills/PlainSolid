# Exact represented planar support certification

`Certify` returns an oriented rational supporting plane for every source polygon,
including its inner wires. Coordinates are interpreted as exact binary64 values.
Dominant-component normalization is rational and deterministic; it is not unit
Euclidean normalization. An oriented area normal handles concave polygons whose
first corner is reflex. Any exact nonplanarity fails, even when it lies within
the BRep constructor's geometric tolerance.

Optional `Options.Convex` proves all input vertices are on or inside every
oriented halfspace. `Violation` exposes the original face and vertex indices and
exact residual, with `errors.Is` support for nonplanarity/nonconvexity. Results
and errors own their rational data. Source face order is preserved. No topology
or coordinates are modified. This is a planarity/halfspace certificate, not a
shell embedding or material-orientation certificate; use collision validation
for those separate obligations.

Defaults bound faces to 4096, vertices to 16384, predicate work to 2000000 and
rational sizes to 16384 bits. Convex verification is O(faces*vertices). Empty
input is certified vacuously. Tests cover an oblique sheared box, reflex-first
concave faces, nonconvex witness diagnostics, tolerance-planar-but-not-exact
faces, resource limits and cancellation.
