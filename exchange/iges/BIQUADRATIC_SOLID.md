# A certified curved IGES186 hexahedron

`ReadBiquadraticHexahedronSolid` / `WriteBiquadraticHexahedronSolid` form a
separate material-solid codec. One original full 3×3 polynomial biquadratic
support replaces one of a six-face hexahedron's affine charts. Its four boundary
curves must remain the same exact straight rims. Five other faces remain affine.
The result is a native `trimmed.Solid` containing the original spline charts,
not an approximating planar BRep. All source geometry, topology IDs and directory
metadata remain separately available.

The type186/514/510/508/504/502 ownership graph and its model/UV edge roles first
pass the existing hexahedron proof using an internal corner-only planar scaffold.
Original directory semantics are checked before that normalization. The scaffold
is only a topology and boundary witness and is never returned as the material.
Each original face support is then composed exactly with every owner pcurve;
the complete resulting rational curve must be identically equal, in the same
parameter, to its authoritative source model edge. No sampling, fitted spline
or near-equality criterion is used.

The original supports are passed to `trimmed.SewSplineRectangles`, which proves
continuous shared isocurve identities. Material acceptance additionally requires
`ValidateSplineRectangleEmbedding` to return `Complete`: exact face injectivity,
whole-edge agreement, global pairwise separation/contact ownership, one shell,
and outward orientation. An unresolved sufficient proof is a rejection. Merely
closed incidence, matching endpoints, or the valid planar scaffold cannot promote
an unproved curved material.

Source degree/weight/domain gates remain strict: exactly one 3×3 degree-two chart,
five degree-one charts, clamped full unit domains, unit polynomial weights and
lossless original scalar coefficients. The hard envelope is 58 entities, 101
control points, 4096 values, 1024 records and 64KiB, reduced by caller limits.
Exact composition uses bounded patch arithmetic; embedding retains its native
bounded proof options. Both APIs take a context; canceled or incomplete work
fails without a partial model or writer output. No general curved-shell, periodic,
nonuniform rational, trimmed-subdomain or tolerance-repair support is implied.

The writer accepts all eight canonical hexahedron vertices, one face index and
an original immutable biquadratic support. Face indices are bottom, top, V-min,
V-max, U-min, U-max. The supplied parameter orientation must match that face's
canonical chart, including reflection. It stages serialized source readback and
the full original-chart material certificate before writing any bytes.

## Independent polynomial dome

The independently authored `biquadratic-solid.igs` uses a unit square base and
z = 1 + x(1−x)y(1−y) as its top. All nine top controls are dyadic; the middle
height is 1.25 and the other heights are 1. Its four top rims remain exactly at
height1. Direct integration gives volume37/36 and centroid(1/2,1/2,951/1850).

`author_iges_biquadratic_solid.py` builds the source entity graph independently
of the Go writer. The pinned OCCT revision
`4dbfdcc448c0b3e565488afcd626c84483172db9` transfers both source and Go output
through ordinary IGES TransferRoots/BRepCheck as one valid six-face solid, with
the analytic volume/centroid and curved area6.0110276096816966. The area is an
independent numerical measurement, not a claimed symbolic formula. Native mass
also matches the direct analytic integral. Set `PLAINSOLID_OCCT_IGES_SOLID` to
the existing standalone topology helper to repeat both directions.

Mutation tests change an entire source rim, fold the interior chart, move it
through the bottom, forge ownership/orientations, or attempt to erase style
metadata through normalization. They fail atomically. Small input budgets and
canceled contexts are covered, alongside a bounded direct-reader fuzz target.
The preceding planar tetrahedron/hexahedron codecs remain unchanged.
