# Non-homothetic concentric elliptical annular prism

`ReadNestedEllipticalAnnularPrism` and
`WriteNestedEllipticalAnnularPrismSTEP` add a separate material class with
independent inner/outer semiaxis ratios. The previous homothetic API is
unchanged. The new API deliberately rejects homothetic dimensions, keeping
the two explicit domains disjoint.

For outer semiaxes a>b>0 and inner c>d>0, exact inequalities c<a and d<b
prove strict containment: every inner boundary point has outer ellipse
quadratic form at most max((c/a)²,(d/b)²)<1. A common rational orthonormal
frame and positive extrusion height preserve this proof globally. The cap
hole has independent exact rational U and V scales, rather than an inferred
uniform shrink. Both scales enter the original homogeneous quadratic UV
controls; their exact compositions agree continuously with the inner wall.
No sampled winding, containment, or approximate sewing is used.

All existing model-only source ownership checks apply: four source faces,
six distinct source edges, opposite seam uses and oppositely oriented inner
rim loops in the 118-entity rooted material graph. Original analytic ellipse
and extrusion supports are retained. Native geometry has ten faces and
sixteen rational vertices; ordinary native reload preserves every coordinate,
and complete continuous shared-edge bounds are zero. The same strict unit,
terminating STEP coordinate, rational-normalization, positive represented
dimension and resource limits apply as for the homothetic codec. Offset or
independently rotated inner ellipses remain unsupported.

Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 independently constructs an
annular face with outer radii 5/2, inner radii 3/1, origin (0.1,2,3), axis
ratio (0,3,4), reference (1,0,0), then extrudes by (0,3,4). Its source and Go
output transfer as valid solids with four faces and twelve edge uses.
Independent result: volume 109.95574287564276 (35π), area 225.87232623135725,
center (0.1,3.5,5). The retained source is a typed OCCT model-only export,
not a Go-token fixture or repaired rounded pcurve.

Focused normal/race/Go 1.23 tests cover source import, new output, native
reload, exact shared edges, cross-domain rejection and owner/support
mutations. The optional live test uses
`PLAINSOLID_OCCT_NESTED_ELLIPTICAL_ANNULAR_PRISM`; its standalone helper is
`tools/occtref/step_nested_elliptical_annular_prism.cxx`. The decoder fuzz
target is capped at 64 KiB, 118 entities, 4096 values and depth 16, and was
qualified with 100 fixed-work cases. No broader elliptic material guarantee
is implied.
