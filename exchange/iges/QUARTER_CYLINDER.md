# Exact cardinal quarter-cylinder material

`ReadQuarterCylinderSolid` and `WriteQuarterCylinderSolid` exchange a separate,
strict 35-entity IGES 186 material dialect. `QuarterCylinderDescriptor` specifies
finite translated origin, positive radius and height, positive global Z axis,
and the positive X-to-positive Y quarter. General sectors and prior readers are
unchanged. Inputs with rounded derived points are rejected, not snapped.

The source uses one type192 cylindrical wall, four type190 planes, six vertices
in a502 list, nine directed edges in a504 list (two100 quarter arcs and seven110
lines), five508 loops, five510 faces, one514 closed shell and one186 solid.
Each loop has model-only trims; no authored cylinder pcurve or approximate π/2
endpoint is inferred. The source model arc starts at exact +X and ends at exact
+Y, with the original positive-Z circle convention selecting the quarter rather
than its complement. Every scalar, pointer, edge endpoint, use orientation,
radial-plane frame and shell sense is checked against recovered dimensions.
The source record order/IDs are part of this deliberately narrow dialect.

Native reconstruction uses `parametric.CardinalQuarterCylinder`. Its quadratic
quarter-circle homogeneous coordinates are
`(R(1-t²),2Rt,1+t²)`, so `t=tan(angle/2)` on[0,1]. Positive weights1,1,2 and
exact cardinal poles define the complete circle arc, not a sampled fit. Tensor
extrusion and radial caps use the same parameter and exact cardinal planes.
All represented vertices/poles are required to be lossless before construction.
Native face order is wall, lower cap, upper cap, X radial, Y radial, mapped to
source IDs57,61,59,63,65. The source has nine nondegenerate edges; native
rectangular cap charts additionally have two collapsed center edges (eleven
edges total). Original source edge list45 and vertex list25 remain inspectable
through copied numeric `Parameters`; native edge numbers are not source IDs.

The reader caps input at32768bytes,512records,35entities,2048values (or smaller
caller limits), validates directory metadata, rejects transforms and requires
exact decimal-to-binary source coefficients. It returns no partial material on
error. The writer stages and re-reads before its single output write. Names and
metadata are retained in the reader result but the descriptor writer authors a
new graph rather than claiming a source-preserving rewrite. LengthUnit is
meters per model unit and follows the existing exact IGES global unit gate.

## Independent qualification

`tools/occtref/author_iges_quarter_cylinder.py` independently enumerates the
normative graph in `testdata/quarter-cylinder.igs` without invoking Go. Pinned
OCCT4dbfdcc448c0b3e565488afcd626c84483172db9 transfers both this source and Go
output as a valid one-solid/five-face material. An independent
`BRepPrimAPI_MakeCylinder(2,3,π/2)` IGES export is retained as
`occt-rounded-quarter-cylinder.igs`; OCCT accepts it with identical measurements,
but the strict reader rejects its different48-entity representation. This is
not a general vendor IGES import claim.

All three give volume3π, area5π+12, centroid(8/(3π),8/(3π),1.5).
Native reload preserves chart bytes and independently integrates these values.
Tests mutate original ownership, vertices, directed arcs, radial normals,
face support and shell sense; test smaller limits, nonrepresentable translation,
atomic rejection, and a bounded parser fuzz target.

Run optional live checks with
`PLAINSOLID_OCCT_IGES_SOLID=/path/to/oracle go test ./exchange/iges -run TestOCCTQuarterCylinder`.
