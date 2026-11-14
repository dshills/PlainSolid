# Original natural elliptical STEP wire

`ReadEllipticalWire` adds a distinct source-supported geometric wire family:
one complete `ELLIPSE` directly owned by `GEOMETRIC_CURVE_SET` inside a rooted
`GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION`. It returns immutable native
`geom.Ellipse` geometry, exact source frame/radii, product name, millimeter unit,
original entity-role IDs and caller-owned parsed source. This is neither a
trimmed face nor a material solid, and does not assert explicit EDGE_LOOP
incidence absent from this geometric representation.

The qualified graph has 26 entities, one AP214-style product/context/root, one
identity representation placement, one signed-cardinal ellipse placement,
strictly ordered positive major/minor radii, millimeter length units, radians,
steradians and the original `1.E-07` length uncertainty metadata. The uncertainty
is retained, never used to snap or accept geometry. All records must match a
bijective rooted role graph; source numbering and order are arbitrary. The
entire source reference structure, unit categories, directions and owner links
are checked. String metadata is bounded and preserved rather than interpreted
as geometry. Every geometric scalar must be an exactly represented decimal;
near-cardinal axes and nearly equal rounded coefficients are rejected. Equal
radii, other units, maps, placements as occurrences, trimming, detached entities
and other representation variants remain outside this API.

The natural closed ellipse uses the original STEP radian parameter, with
mathematical period 2π. This is an analytic domain statement, not a claim that
binary64 evaluation at `2*math.Pi` yields bit-identical points. Native evaluation
uses the existing analytic evaluator; no polygon or NURBS fit is introduced.

`WriteEllipticalWireSource` revalidates and re-emits original entity records,
header, IDs and names. It does not silently write edits to the returned
Geometry fields. Mutated source geometry is accepted only if the complete
source graph still proves this family; broken ownership fails before output.
Its caller-owned input also receives bounded traversal before serialization.

Reader limits additionally cap input at 64 KiB, 26 entities, 2048 values and
nesting depth 16. Failed reads return no partial result. Writer validation is
staged before its single output write.

## Independent OCCT evidence

`tools/occtref/step_ellipse_wire.cxx` first constructs a genuine OCCT ellipse at
(1,2,3), major radius 4, minor radius 2, normal +Y and reference −Z. Ordinary
`BRepBuilderAPI_MakeEdge` → wire → STEP export produces the retained negative
`occt-rounded-ellipse-wire.step`: its preferred PARAMETER trimming ends at the
rational decimal `6.28318530718`, not exact 2π. Coincident Cartesian selectors
do not override that preference. This file remains rejected without repair.

In `write-natural` mode the independent OCCT author puts the original
`StepGeom_Ellipse` directly into its geometric curve set, removes only the
unneeded finite-trim wrapper/dependencies by rebuilding the rooted model, and
writes `occt-natural-ellipse-wire.step`. This standard original analytic
representation defines a complete ellipse without claiming the rounded trim is
equivalent. No Go geometry or STEP string generator is used for the fixture.

Pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 transfers both this source and
Go source re-export through conventional TransferRoots as one valid edge.
Independent OCCT point/tangent at 0.375 radians match native evaluation. Adaptive
`GCPnts_AbscissaPoint` gives length 19.376896441095354, matching an independent
Simpson integral of analytic speed. Default OCCT LinearProperties quadrature
was measurably coarser on this ellipse; its centroid is not used as evidence.

Optional test:
`PLAINSOLID_OCCT_ELLIPSE_WIRE=/path/to/oracle go test ./exchange/step -run TestOCCTEllipticalWire`.

Tests also cover arbitrary source IDs/order, typed read/source re-export,
nonexact coefficients, wrong owner/units, duplicated members, altered axes,
original rounded trimming rejection, bounded input and atomic writer failure.
The decoder has a bounded fuzz target; the retained milestone ran 100 inputs.
