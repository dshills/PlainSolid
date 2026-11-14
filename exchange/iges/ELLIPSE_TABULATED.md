# Closed ellipse directrix tabulated surface

`ReadEllipseTabulated` / `WriteEllipseTabulated` are a separate exact type122
codec: one surface owns one type102 closed ring, four directed quarter104
members, and optionally their common proper signed-coordinate124. The type122
and102 cannot carry transforms. Root, directrix, leaf and transform original
IDs and metadata remain inspectable. No ignored records or shared external
owners are accepted. Existing spline126 tabulated-surface APIs remain strict.

`End` is the original model-space generatrix terminal point, not a displacement
or a local point. The displacement is exactly `End - firstDirectrixPoint`.
Source decimal conversion and original coefficients retain the atom's exact
representability gates. Zero displacement is rejected. No direction or radius
normalization is performed. Negative, oblique or in-plane nonzero displacement
is representable; regularity or embedding is not claimed by this geometry API.

`ExactPatches` returns four immutable rational degree(2,1) tensor patches on
[0,1]². Homogeneous controls (X,w) become (X,w) and (X+wD,w); hence the complete
surface identity is C(t)+vD. The four directed quarter edges close exactly.
There is no invented global smooth parameter across members. V is normalized
0..1 (OCCT's lateral charts use physical distance and may differ in orientation).
This is a finite generatrix surface description with periodic directrix, not
an unbounded extension, cap, trimmed face, sewn shell or solid.

The independent helper `tools/occtref/iges_ellipse_tabulated.cxx` uses pinned
OCCT4dbfdcc448c0b3e565488afcd626c84483172db9's native104/102/122 entities. Its
placed5-by2 ellipse starts at(1,7,3), with terminal point(4,7,3). TransferRoots
produces four valid lateral faces with area69.039337786994892. Original and
PlainSolid-generated files pass that same independent check. This confirms the
composite directrix semantics without inferring material from OCCT's faces.

Input caps:32KiB,256 records,7 entities,1024 values. All-or-none read and staged
writer validation; an output I/O error can still partially write. Geometry
construction uses the existing exact surface work/bit/cancellation options.
Tests include negative displacement, zero/bad terminal point, strict old APIs,
limits, native exact patch translation and100 fixed fuzz executions. Optional
live test: `PLAINSOLID_OCCT_IGES_ELLIPSE_TABULATED=/path/to/helper`.
