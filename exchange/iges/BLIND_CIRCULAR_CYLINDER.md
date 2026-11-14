# Exact canonical analytic blind-cylinder material

`ReadBlindCircularCylinderSolid` and `WriteBlindCircularCylinderSolid` handle a
separate bounded IGES type186 dialect: a complete positive-Z circular cylinder
with a smaller coaxial circular pocket opening at the upper cap and stopping
strictly inside the stock. `Floor` is height above `Origin`. The original five
faces are outer wall, reversed inner wall, upper annulus, bottom disk, and floor
disk. The floor is an attached material boundary, not a detached interior shell.

The 39 records preserve two type192 supports, three type190 supports, four full
type100 circular rims, two type110 seams, original type502 seam vertices, one
six-edge type504 list and six type508 loops. The upper face owns both an exterior
rim and oppositely directed hole rim. Every model edge has its prescribed
owners; both seams have two owner uses. Cylindrical pcurves preserve exact
0–360 degree domains and physical axial coordinates; plane loops use direct
model edges. No decimal approximation to a radian period is required.

The reader recovers origin/radii/heights from the original analytic supports,
then verifies every source number, integer pointer, flag, owner, loop and face
against the complete directed graph. All source decimals must denote exactly the
represented binary values; derived sums/differences must be lossless. Directory
styles, alternate topology, placements, void shells and extra entities fail.
The native `FiniteCylinderPairDifference` independently proves radial containment,
proper interior floor and outward material boundary. Its fixed 1e-9 construction
tolerance imposes an additional minimum-clearance gate; it is not an exchange
repair tolerance. Unsupported small dimensions fail rather than being snapped.
Native quarter-rim topology can differ from the source's complete circular edge;
the result retains both the original numeric graph/IDs and native ancestry.

Limits are at most64KiB,1024 records,39 entities,4096 parameter values. Parsing
and native reconstruction precede publication. Errors return no partial body;
writers stage and validate before output. Cancellation is checked before parsing
and during/after native proof. The older cylinder and collection dialects are
unchanged; this is not a generic IGES Boolean or vendor source importer.

## Independent qualification

`tools/occtref/author_iges_vendor_blind_cylinder.cxx` independently cuts a radius1
cylinder starting at z2 from radius3,height4 stock using OCCT. The unchanged
53-entity IGES186 output is retained as `occt-rounded-blind-cylinder.igs`.
Its type502 seam coordinates include Y=-7.347880795E-16 and
-2.449293598E-16, with alternate transformed model circles. The exact reader
rejects this source, although OCCT accepts its own tolerance-based material.

`author_iges_blind_cylinder.py` separately specifies exact analytic supports and
directed topology, producing `blind-circular-cylinder.igs` without using the Go
writer. Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9 ordinary
IGESControl_Reader.TransferRoots accepts this source, Go output, and the native
OCCT Boolean reference: one solid,five faces,volume34*pi,area46*pi,
centroid(0,0,33/17). BRepCheck_Analyzer validates each transferred body.

Run `TestOCCTBlindCircularCylinder` with `PLAINSOLID_OCCT_IGES_SOLID` pointing to
the existing `iges_topology_solid` oracle. Tests include native analytic mass,
translation/units, owner/winding/radius/floor/degree mutations, source limits,
atomic failure, cancellation, unchanged old-reader rejection and bounded fuzz.
