# Exact aligned concentric IGES elliptical annulus

`ReadEllipseAnnulusFace` / `WriteEllipseAnnulusFace` are a separate11-record
face codec. They preserve the disk protocol's exact190 chart and104 atoms but
require one explicit outer142 and one distinct inner142 on that same plane.
Each142 owns separate UV and model104 records. All original references, source
IDs, labels, subscripts and units are retained; no extra/shared boundary records
or ignored entities are allowed. The disk API remains8-record/no-hole only.

Both authored source curves use the positive full ellipse, starting/ending at
(+major,0). IGES144's inner-boundary role defines the excluded interior; there
is no142 reversal flag to invent. `NativeFace` returns a native trimmed plane
with the inner use explicitly reversed and its UV parameter decreasing2π→0.
The original model ellipse remains positive: native inner t maps to model
angle2π(1−t). Complete model/UV coefficient equality proves continuous lift,
not sampled proximity. Plane height and every owner linkage are re-proved.

The inner ellipse is centered and aligned with the outer ellipse, with both
semiaxes strictly smaller. Exact comparisons prove strict nesting because each
positive scaled squared-coordinate term on the inner boundary is reduced in
the outer normalized ellipse equation. Touching, crossed, equal or outside
profiles fail. Distinct axis ratios are allowed. There are no rotated/eccentric
holes, multiple holes, transformed charts or solid/material claims.

The separate version1 `Write/ReadEllipseAnnulusFaceJSON` recipe retains the
source and native descriptor. Reload independently revalidates the complete
source plus every geometry/metadata/ancestry field. The disk native format stays
strict. Source limits are32KiB/256records/11entities/1024values; JSON128KiB.
Read failure is atomic; writers validate before I/O (I/O failure may write).

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9 independently authors the
retained fixture in `tools/occtref/iges_ellipse_annulus.cxx`. Outer radii5,2 and
inner2,1 atZ3 transfer as one valid face with area25.132741228718348(8π).
The source→native JSON reload→generated IGES chain yields the same independent
result. Optional live test variable: `PLAINSOLID_OCCT_IGES_ELLIPSE_ANNULUS`.
Normal/race/Go1.23/vet and100x fuzz checks cover ownership, limits, stale native
ancestry, wrong plane, touching/crossed nesting and unchanged disk behavior.
