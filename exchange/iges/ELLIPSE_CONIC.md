# Exact IGES104 full ellipse model curve

`ReadEllipseConic` / `WriteEllipseConic` are separate bounded codecs for one
type 104 form 1 root and an optional uniquely owned type 124 placement. This
unblocks an analytic ellipse atom without asserting a face, wire topology,
trimmed surface or material. Existing generic/topological readers remain strict.

The local equation is Ax²+Bxy+Cy²+Dx+Ey+F=0 at Z. This first domain requires
B=D=E=0,0<A<C,F<0. The major/minor radii are derived from exact rational
square roots of -F/A and -F/C; they must be positive lossless binary64 values.
Raw coefficient scaling is preserved. Authored start and end must both be
exactly (major,0). Under the IGES full-conic convention this establishes a full
ellipse with natural positive radian parameter, t=0 on local +X and increasing
t toward local +Y. It does not infer finite interval selection from a tolerance.

Optional type 124 must be a proper signed-coordinate rigid map (determinant +1),
with exactly represented translation and world center/major endpoint. Its
label/subscript and the conic's label/subscript are retained independently.
Shears, scales, reflections, arbitrary numeric rotation normalization, nested
transforms, shared dependencies and unused records fail. Source scalar decimals
must equal their binary64 values exactly; decimal approximations cannot be
silently promoted to exact conics. Radii requiring irrational or unrepresentable
values fail. Partial endpoints, off-diagonal or translated local conics are
future domains, not accepted aliases.

The codec admits at most 2 entities, 128 records, 256 values and 16 KiB. It checks
record ownership, entity/form numbers, type 124's own parameter type, directory
status/style, unit declarations and all source coordinates. The writer stages
and re-reads its output before writing and propagates short-write failures.
Source IDs are reported by the reader; generated IDs may differ on re-export.
The optional transformation returned by the reader owns its numeric matrix.

Independent OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 constructs
IGESGeom_ConicArc with coefficients (4,0,25,0,0,-100), Z=0 and equal endpoints
(5,0). Its proper type 124 maps (x,y,z) to (z+1,x+2,y+3). The original retained
IGES and Go output both transfer through IGESToBRep_BasicCurve with natural
range [0,2π], major endpoint (1,7,3), and matching point/derivative at t=0.375.
Floating trigonometric evaluation at 2π has the ordinary tiny numerical endpoint
error; the acceptance proof uses the exact source full-conic role instead.
The independent helper is `tools/occtref/iges_ellipse_conic.cxx`; optional live
test environment is `PLAINSOLID_OCCT_IGES_ELLIPSE_CONIC`.

Focused normal/race/Go 1.23, vet and 100 bounded fuzz cases cover source import,
coefficient/metadata retention, identity and signed placement, malformed forms,
partial endpoints, coefficient/radius failures, forged transform type, source
limits and atomic writer errors. No STEP/IGES package import cycle was added.
