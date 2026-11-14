# Orthogonal concentric elliptical annular prism

The additive `ReadOrthogonalEllipticalAnnularPrism` and
`WriteOrthogonalEllipticalAnnularPrismSTEP` APIs support independent in-plane
ellipse frames with exactly one positive quarter-turn: inner X=outer Y,
inner Y=-outer X, common Z. Original source reference ratios are authenticated,
not replaced by approximate rotations. `InnerRationalFrame` and the outer
`RationalFrame` return caller-owned exact coefficients. Aligned homothetic
and non-homothetic APIs retain their distinct prior acceptance behavior.

Let outer semiaxes be a>b>0 and inner c>d>0. This bounded domain requires
c<b and d<a. On the rotated inner boundary, the outer ellipse form is at most
max((d/a)²,(c/b)²)<1, proving strict continuous nesting. Native cap trims use
an exact coordinate swap plus independent signed rational scale factors.
The original inner wall uses its own rational frame. Thus cap and wall
model curves agree identically, not within a sewing tolerance. Positive height
and a proper common rational frame prove the resulting annular extrusion's
material orientation and embedding.

The full model-only 118-entity source graph retains four faces, six model
edges, both wall seams and all directed rim ownership. The inner reference
must equal normalized outer Y exactly; a negative quarter-turn or shifted
seam is not accepted as equivalent. Partial/vendor-rounded pcurves, offsets,
rotated arbitrary-angle holes and nonterminating required STEP coordinates
remain unsupported. Original millimeter and resource restrictions remain.
Native output has ten exact spline faces and sixteen rational vertices;
ordinary native JSON reload preserves them and every continuous shared-edge
bound is zero. No general solid claim is made by the reusable sewing helper.

Independent OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 fixture: outer radii
5/2, inner 1.5/0.5, origin (0.1,2,3), axis ratio (0,3,4), outer reference (1,0,0),
inner reference (0,4,-3), height 5. It authors the annular face with distinct
ellipse frames, extrudes it, and exports model-only curve geometry. Source
and generated STEP both pass ordinary TransferRoots and BRepCheck: one solid,
four faces, twelve edge uses, volume 145.29866022852741 (46.25π),
area 206.59726012112046, center (0.1,3.5,5).

Normal/race/Go 1.23/vet and 100 bounded fuzz cases qualify the milestone. The
optional independent test uses `PLAINSOLID_OCCT_ORTHOGONAL_ELLIPTICAL_ANNULAR_PRISM`
with `tools/occtref/step_orthogonal_elliptical_annular_prism.cxx`.
