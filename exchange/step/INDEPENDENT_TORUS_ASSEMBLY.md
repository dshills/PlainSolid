# Exact model-only torus product assembly

`ReadIndependentTorusAssembly` accepts a deliberately bounded flat standard
AP214/AP242 NAUO/CDSR assembly: one assembly-only root, one exact planar product,
and one exact model-only natural ring torus product. Each leaf owns one material
body. The torus must be used at least twice; the planar leaf at least once. Three
to sixteen occurrences are permitted. Products share one explicit metre or
millimetre unit declaration. This is neither a generic assembly importer nor an
extension of the existing wrapped rational-toroid assembly API.

Each leaf receives its existing complete material proof. The torus has its
original TOROIDAL_SURFACE, two model CIRCLE seams, one vertex and four directed
uses. R>r and exact circle/support/vertex agreement remain mandatory. Rounded
vendor SEAM_CURVE/pcurve records remain rejected. Native torus material uses the
previously qualified angular analytic representation; no exact rational patch
sewing claim is added.

Occurrence source frames and their composition are independently computed with
rationals, required to be binary64-exact proper signed-coordinate poses, and
compared against the public occurrence matrix. This avoids promoting a rounded
frame normalization or translated subtraction into an exact placement. Geometry
stays local: the product-definition keyed result holds each material once, while
NAUO IDs retain each distinct occurrence. Root, product, definition,
representation, body and torus seam role IDs retain their original source values.
No inter-occurrence disjointness or Boolean union is claimed.

Local dependency closures cannot borrow geometry from other products or from
assembly frames. All other records must belong to the registered assembly/unit
metadata types and be reachable from authenticated SDR/CDSR/APD roots. Detached
geometry, extra products, nested groups, alternate families, unsupported units,
cycles and invalid senses fail without partial results. Parse work is capped at
1 MiB, 4096 entities, 65536 values and depth 16, in addition to lower caller limits
and cumulative native topology limits. Cancellation is checked before parsing
and between material proofs; the existing parser itself is not interruptible.

`WriteIndependentTorusAssembly` accepts separately authenticated independent
product sources plus explicit occurrence descriptors. It reuses the existing
standard assembly writer and re-proves the complete staged output before writing
any bytes. It creates new assembly entity IDs and does not promise original
source IDs. `WriteIndependentTorusAssemblySource` instead re-proves and re-emits
an accepted original graph retaining its IDs; it never trusts edited native
payloads as a replacement for source proof.

`tools/occtref/author_step_independent_torus_assembly.py` independently authors the
standard product relationships around an untouched genuine OCCT planar box and
a separately authored exact torus. This is not a repair of an OCCT rounded torus.
The torus is reused at translation (20,0,0) and at a signed-coordinate rotation
with translation (0,20,0). The pinned OCCT commit
`4dbfdcc448c0b3e565488afcd626c84483172db9` accepts both this source and Go-authored
output using ordinary `TransferRoots` and `BRepCheck_Analyzer`:

- Three solid occurrences, eight faces, 32 edge uses.
- Volume `24+12*pi²`, area `52+24*pi²` in the mm context.
- World bounds `[-10,-4,-2]` to `[25,25,6]` (the OCCT numerical bounding
  result includes its approximately 1e-7 topology margin; test tolerance 2e-7).
- Independent surface centroid from the planar face quadrature and transformed
  torus centers agrees with the directly calculated first moment.

Build `build_step_independent_torus_assembly.sh INCLUDE LIB OUTPUT`, then set
`PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY=OUTPUT` for the live test. Native local
material reload, source reordering/ID retention, wrong seam/circle, cross-product
borrowing, nonbinary pose, unit, orphan/cycle, rounded vendor, limit, cancellation
and atomic-write regressions accompany the fixture. Existing collection and
assembly entry points are unchanged.
