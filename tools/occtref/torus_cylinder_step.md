# Native transverse torus/cylinder STEP qualification

`exchange/step.WriteTorusCylinderSets` emits original analytic torus, cylinder
and plane supports, circular rim and meridian model curves, and periodic seam
pcurves. It reconstructs the native Common/Cut/Fuse family before emission.
There are no sampled curves, mesh surfaces or rounded meridian trim angles.

STEP finite decimal coordinates cannot express the irrational height
`h = sqrt(r²-(c-R)²)` exactly. `CertifyTorusCylinderExport` proves, with exact
rational arithmetic, the height error bound `abs(hf²-h²)/hf`, the entire exported
rim's distance to the original torus `abs(hf²-h²)/r`, and the represented
`2*pi` periodic seam displacement. The sum must not exceed the mandatory export
tolerance. Source binary scalars are emitted as their exact decimal expansions;
short decimal formatting is not silently charged as zero. The native algebraic
source identity is separate and unchanged. These bounds do not certify a
reader's healing, numerical evaluation or complete STEP support. The paired native import described below is a separate restricted dialect.

The independent `torus_cylinder_step.cxx` reads each file through pinned OCCT,
requires a valid single solid/shell with the expected analytic support types and
original radii, and independently constructs an OCCT torus and finite cylinder
then applies Common/Cut/Fuse. It compares area/volume/centroid. It also samples
model-curve versus support/pcurve correspondence on all readback edge uses;
this is a numerical regression check, separate from the rational export proof.

Qualified fixtures: R=5, r=2, c in {4,6}, all three operations at caps [-3,4],
and Fuse at reflected unequal caps [-4,3]. The c=4 outer arc passes through both
latitude extrema, so endpoint matching cannot substitute a short arc. Readback
has 2 faces for Common/Cut, 5 for Fuse, and one solid/shell throughout. Maximum
observed edge residual was 1.9985e-15 model units. Comparison allowances are
1e-5 for volume/area, 1e-7 for centroid, and 1e-8 for edge residual. Actual kernel
export error bounds are much smaller than the requested 1e-8.

Pinned OCCT source: `4dbfdcc448c0b3e565488afcd626c84483172db9`, archive
`/tmp/plainsolid-occt-4dbfdcc.tar.gz`, SHA256
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.

Build independently (no produced BRep is used as reference source):

```sh
c++ -std=c++17 -O2 \
  -I/tmp/plainsolid-occt-build/include/opencascade \
  -L/tmp/plainsolid-occt-build/mac64/clang/lib \
  -Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
  tools/occtref/torus_cylinder_step.cxx \
  -o /tmp/plainsolid-torus-cylinder-step-oracle \
  -lTKDESTEP -lTKXSBase -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep \
  -lTKGeomAlgo -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_TORUS_CYLINDER_STEP=/tmp/plainsolid-torus-cylinder-step-oracle \
  go test -race ./exchange/step \
  -run '^TestOCCTTorusCylinderSTEPOriginalBoolean$' -count=1
```

The qualified local binary SHA256 was
`4944b596c43a2217ef54c1affd3344ff3993ef7256c11903d2ab351f84a6eea8`.
The earlier general STEP reader additionally passed the canonical three
exports; its binary SHA256 was
`a7a5ebbbb335e186f7efe99fb9be00cd5171d5804c751c3579a1e71f9d876ac0`.
No binaries are committed.


## Strict paired native import

`ReadTorusCylinderSet` accepts one original body in this writer's complete
analytic entity graph. It recovers represented source scalars from the torus,
cylinder and retained planes, and recovers the operation from the native face
layout and sense. Every support, model curve, pcurve, adjacency, trim orientation,
product link and detached entity is then checked against the independently
reconstructed graph. Numbers are compared as bounded exact decimal rationals;
entity IDs may be relabelled bijectively, but encounter order is part of this
first paired dialect. Genuine OCCT-authored/healed/reordered STEP remains
unsupported, even where it represents an equivalent solid. No vendor payload
or implicit tolerance matching is used as authority.

For Fuse the two cap coordinates are observable and retained. Common and Cut
contain no cap surfaces, so those authored cutter extents cannot be inferred.
The immutable result's `Source()` explicitly returns `CapsRetained=false` and
uses the exact represented clear proxy `±nextafter(r,+Inf)` for reconstruction.
This finite proxy exists because `r<R<Inf`, and the material of Common/Cut is
independent of clear cylinder cap locations. Tests qualify scales 2^-100 and
2^100, retained asymmetric caps, original owners, immutable algebraic rim
roots, exact canonical re-export, entity relabelling, resource limits, and
atomic rejection of all requested graph mutations including sub-float changes.

Import is bounded to at most 512 entities, 16384 values and 2 MiB, respecting
smaller caller limits. Exact decimal atoms have at most 1100 characters,
exponents in [-1100,1100] and numerator/denominator bit lengths at most 4096.
At most two native factory proofs run with their fixed per-proof root budget.
The live original-Boolean comparator also tests import/re-export and compares
the reconstructed native volume/area/centroid with independently authored OCCT
Common/Cut/Fuse, not merely the geometry emitted by PlainSolid.
