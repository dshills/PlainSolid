# Exact model-only natural torus products

`ReadIndependentToroidalMaterials` / `WriteIndependentToroidalMaterials` add a
separate full ring-torus family to independently owned AP214 product files. At
least one torus is required; previous planar, elliptical, split-cylinder and
sphere members retain their proofs. Earlier APIs, unit-normalization routes and
native envelope tags do not admit this family.

The supported body has one outward TOROIDAL_SURFACE face, one forward bound,
one four-use loop, two closed model CIRCLE edges and one shared vertex. Uses are
major-reverse, minor-reverse, major-forward, minor-forward. Each EDGE_CURVE has
same_sense true and its identical start/end vertex denotes the natural complete
circle. No pcurve, SEAM_CURVE, trim record, extra loop or extra material shell is
accepted. The proof preserves original body/shell/face/bound/loop/edge/circle/use
and vertex IDs, separately from native face ancestry.

The source frame is an exact signed-coordinate frame. Exact source scalars prove
`R > r > 0`; the major seam has center O, normal Z, reference X and radius R+r.
The minor seam has center O+R*X, normal -Y, reference X and radius r. Their common
vertex is exactly O+(R+r)*X. Rational sums/offsets must be losslessly representable.
These identities prove the two full natural parameter seams of the standard ring
torus; opposite paired uses define its fundamental rectangular quotient. The
ring inequality supplies the embedded material domain, rather than relying on
closed incidence alone. Native promotion uses the established analytic torus and
`trimmed.FromParametric`; it canonicalizes angular seams and **does not claim**
zero-discrepancy rational trigonometric sewing. Native reload and analytic mass
are independently checked.

The genuine retained OCCT torus is explicitly rejected. Its pcurves contain
`6.28318530718` rather than exact 2π, and its minor-circle frame contains
`±2.449293598295E-16`; these do not establish exact seam agreement. No source
number is snapped. `author_step_model_torus.py` instead independently authors a
standard model-only graph, unrelated to a repair of that vendor graph. Pinned
OCCT ordinary TransferRoots accepts it as one valid solid / one face / four edge
uses, with area 12π² and volume 6π² for R=3,r=1. It also accepts generated output
and an eight-product heterogeneous collection containing this torus. Use the
existing selected-material comparator binary through
`PLAINSOLID_OCCT_INDEPENDENT_TOROIDAL_MATERIALS` for live qualification.

Tests cover horn/spindle rejection, reversed face/coedge/edge sense, wrong vertex,
circle and minor frame, pcurve substitution, original vendor rejection, native
reload/mass, source-preserving roundtrip, cumulative roots, cancellation, legacy
rejection, race and bounded fuzz.
