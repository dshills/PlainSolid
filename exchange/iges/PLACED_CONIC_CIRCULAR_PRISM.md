# Properly placed circular-conic prism material

`ReadPlacedConicCircularPrismMaterial` supports both filled and strictly nested
annular circular-conic materials with one common type124 proper signed-coordinate
placement. `WritePlacedConicCircularPrism` and
`WritePlacedConicCircularAnnularPrism` author these separate canonical21/29-record
graphs. Existing ellipse, unplaced circle and type192-cylinder APIs stay strict.

The transform is attached only to original type104 curves, type110 seams and
type190 planes. Type116/123 plane datums remain local; their parent190 owns the
transform. Type122 is not transformed: its directrix already carries placement,
so its terminal is explicitly world-space. Type502 vertices are also world-space.
Any missing, duplicate, foreign or incorrectly attached transform is rejected.

The reader authenticates the original transform matrix as a proper signed
coordinate frame, proves all source numeric tokens lossless, inverse-maps each
world terminal and vertex using exact rational arithmetic, and invokes the
unchanged strict local circular material proof. Every intermediate coordinate
must remain exactly representable. General approximate rotations, reflections,
scale/shear and lossy translations fail rather than being normalized. The actual
world native solid is rebuilt with exact circular rational quarter controls and
continuous shared-edge proof. Local original circular coefficients, world
vertices/terminals, transform ID/metadata and native source-face lineage stay
separate in the result.

`WritePlacedConicCircularPrismSource` re-proves the source and current native
binding before returning unchanged bytes. Separate version1 native persistence
retains original source, geometry and placement metadata and reconstructs all
proofs on reload. Source caps are32KiB,512records,29entities,2048values; native
envelope cap is512KiB. Caller limits remain effective. This milestone preserves
canonical record/list order; reindexed placement is not implied.

## Independent qualification

`author_iges_placed_elliptical_prisms.py --circle` independently authors the
placement (x,y,z)→(z+7.5,x−4.25,y+2.125) on the retained filled and hollow circular
source fixtures, retaining the exact122/502 world-coordinate contract. Pinned
OCCT accepts original and Go-emitted files as one valid solid with3/4 faces,
respective area90π/98π and volume100π/84π, and centroid(12.5,−4.25,2.125).
Native mass and exact zero edge discrepancy agree. Native reload preserves the
original file and metadata. Tests also exercise a proper signed frame reversing
the local cylinder axis, reject missing leaf placement, incorrect terminal,
reflected/rounded frames, nonrepresentable translation and forged persisted
placement. The two-source fuzz target is `FuzzPlacedConicCircularPrism`.
