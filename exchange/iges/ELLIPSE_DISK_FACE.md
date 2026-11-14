# Exact planar IGES ellipse disk

`ReadEllipseDiskFace` / `WriteEllipseDiskFace` exchange one independently
qualified type144 face with explicit outer type142, parametrized type190 plane,
and separate complete type104 model/UV ellipses. The eight records are uniquely
owned: plane location116, normal123 and reference123, plus those five records.
There are no holes, transforms, nested composites, ignored entities or material
shells. Unlike the ring wire API, this uses a full analytic conic for each owner.

The conservative chart is `(u,v)->(u,v,z)` with exact authored z, normal+Z and
reference+X. Both curves retain identical original coefficient scaling with
B=D=E=0,0<A<C,F<0 and lossless rational-square radii. UV Z must be0; model Z
must equal the plane location. Both start/end pairs must select the complete
positive ellipse at(+major,0). This proves continuous lift identity, positive
winding, simplicity and the bounded disk domain, not merely sampled agreement.
The142 creation mode is0 and equal preference3. Every link and directory status
is checked; all records must belong to this one face. Future placements or
alternate declarations require separate proof, not rounding of source values.

`NativeFace` returns an existing native `trimmed.Face` on `geom.PlaneSurface`
plus its `geom.Ellipse` model boundary. Native `EllipseUV` takes normalized t
in[0,1]; the model curve takes radians with angle=2πt. The exact analytic
coefficients establish the identity; numeric evaluation retains ordinary trig
roundoff. The native use's explicit seam endpoints coincide exactly. This is
an independently bounded face, not a `trimmed.Solid` or material classification.

`Write/ReadEllipseDiskFaceJSON` persist a version1 source-bound native face
recipe. Reload revalidates the whole source and compares all metadata, geometry
and original entity IDs; stale public fields, unsupported versions and unknown
fields fail atomically. The descriptor rebuilds the native analytic objects;
it does not serialize an invented closed shell. JSON is capped at128KiB;
source at32KiB,256 records,eight entities,1024 values. Writers stage validation
before I/O; I/O errors can partially write.

The standalone helper `tools/occtref/iges_ellipse_disk.cxx` authors104/190/142/144
via pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9. Original and generated
IGES pass ordinary TransferRoots/BRepCheck as one face, area31.415926535897938
and centroidZ3 (ellipse radii5,2). Set `PLAINSOLID_OCCT_IGES_ELLIPSE_DISK` to the
built helper for this independent source→native reload→generated IGES check.
Focused normal/race/Go1.23/vet and100x fuzz runs cover rejection and persistence.
