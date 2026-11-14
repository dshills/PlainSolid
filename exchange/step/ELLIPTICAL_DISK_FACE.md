# Exact original planar elliptical STEP disk

`ReadEllipticalDiskFace` decodes the genuine OCCT-generated 47-entity single
elliptical disk face. It returns a native `trimmed.Face` with an analytic plane
chart, full elliptical UV loop, original analytic model ellipse, exact closing
vertex, original source graph, units, name and six source ancestry IDs. It does
not construct a material `Solid` from an open face.

The qualified ownership graph is:

- rooted product → manifold surface representation → shell-based surface model;
- one open shell → one positive advanced face and one positive FACE_BOUND;
- one edge loop → one positive oriented edge → one positive closed edge curve;
- one vertex, exactly at the model ellipse's positive major-axis point;
- a SURFACE_CURVE with authoritative PCURVE_S1, one original model ELLIPSE,
  and one PCURVE owned by the face's PLANE;
- a definitional representation containing the matching natural 2D ELLIPSE
  and an explicit two-dimensional parameter context.

All model, face, coedge and bound senses are positive in this first scope.
The source has no TRIMMED_CURVE or finite decimal angular endpoint. Both ellipse
and plane frames have identical exactly represented centers and perpendicular
signed-cardinal unit axes. The pcurve center is (0,0), reference direction +U,
and its positive major/minor radii equal the model radii exactly. The one source
vertex is checked against center + major × reference using exact arithmetic;
if a required native point is not exactly representable, import fails.

These coefficients prove `plane(a*cos(t), b*sin(t)) == modelEllipse(t)` for the
entire natural curve, without sampling or tolerance. With a>b>0 the ellipse is
simple, its UV winding is positive, and the oriented loop selects the bounded
convex disk. The injective planar chart therefore proves the bounded face,
including its normal. This is a face proof, not a closed-shell/material proof.

The complete graph is matched bijectively, allowing arbitrary entity IDs and
record order while rejecting detached records, shared foreign owners, different
frames, pcurve contexts, reference directions and source orientation variants.
Millimeter/radian/steradian units and original 1.E-07 uncertainty metadata are
validated; that uncertainty is never used for repair. String metadata is bounded
and preserved. Other graph encodings, units, circular equal radii, source trims,
assemblies and generic faces remain unsupported by this additive reader.

## Native and source persistence

Native face outer use has local edge role 0, with its original ID separately in
SourceEdgeID. The model Boundary is the full analytic ellipse in radians; the
native EllipseUV use is parameterized on [0,1] with the existing full-turn angular
convention. Floating-point point evaluations are numerical evaluations, not
bitwise identities for arbitrary trigonometric values.

`WriteEllipticalDiskFaceSource` re-proves and emits the original graph/header,
including its names, coefficients and IDs. It does not implicitly propagate
changes to the separate decoded fields.

`WriteEllipticalDiskFaceJSON` / `ReadEllipticalDiskFaceJSON` add a version 1
face-only native envelope. It contains native plane and ellipse geometry,
explicit full-loop coefficients, original STEP source, vertex, units and ancestry.
Reload reconstructs from the independently verified source and compares every
stored native field. Stale/forged geometry, orientation or ancestry fails before
any result is returned. Successful reload is canonical-byte stable. The envelope
is not a generic native face or closed-solid format.

STEP input is capped at 64 KiB, 47 entities, 4096 values and depth 16, or smaller caller
limits. Native JSON is capped at 256 KiB and reuses the same STEP constraints.
Caller-owned source trees receive a separate bounded traversal before writing,
including cycle/depth rejection. Output validation is staged before writing.

## Independent qualification

`tools/occtref/step_ellipse_face.cxx` creates an ellipse and explicit plane in
OCCT, then calls MakeEdge → MakeWire → MakeFace → STEPControl_AsIs. The retained
`occt-elliptical-disk-face.step` is the unchanged geometric export (line-end
whitespace removed only), not a Go-authored approximation or normalized graph.

Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 conventionally transfers both
original input and STEP→native face JSON→reload→STEP output as a valid one-face,
one-edge shape. Independent properties are area 8π, centroid (1,2,3), normal +Y,
and adaptive boundary length 19.376896441095354. Point/tangent evaluations agree
with native analytic geometry. Independent Simpson integration checks the edge
length. Unit conversions are not silently applied to native model coordinates.

Focused tests include original model/UV lift, all source roles, record renumbering
and reversal of encounter order, source and native reload, foreign pcurve owners,
wrong dimensional contexts, source normals/senses, source vertex ownership,
nonexact coefficients, stale native fields, limits and cyclic caller data.
The retained fuzz campaign runs 100 inputs. Existing wire and solid readers remain
unchanged. Live test environment variable: PLAINSOLID_OCCT_ELLIPSE_FACE.
