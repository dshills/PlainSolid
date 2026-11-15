# Combined original affine NURBS material

`ReadCompleteAffineNURBSPlanarAssembly` combines three independently qualified
representation layers in one original AP242 material assembly:

* Degree-2×2 rational affine supports, positive weights varying in both axes.
* Quadratic rational affine UV owner pcurves with positive nonconstant weights.
* Continuous straight polynomial 3D model splines, degree 1–8, with bounded
  degree elevation/internal knots and original parameterization.

“Complete” refers to these three specific layers, not arbitrary NURBS geometry
or unrestricted STEP. All prior readers remain strict. This route requires
positive evidence from all three helpers and preserves the existing limits and
unsupported families of each. It does not approximate curved material.

The reader reuses the exact model Greville proof, tensor homogeneous degree-3×3
identity, and UV homogeneous cubic identity. Their private equivalent views
preserve model/owner IDs and parameter ranges. Only then does one existing
planar material proof authenticate continuous model/UV agreement, both original
face owners, edge vertices/senses, closed embedded topology, proper product
frames, shared occurrences and appearance. Individually affine curves with
inconsistent cross-layer parameterizations do not pass.

The returned original assembly source is the complete unmodified graph.
`ModelCurves()`, `RationalSupports()` and `PCurves()` return copied original
basis evidence, including source pole/knot/weight IDs and owner chains. Source
export re-proves every layer and emits original bytes. Native JSON uses distinct
version-one tag `plainsolid-step-complete-affine-nurbs-planar-assembly`; reload
re-proves original source and checks every stored native and provenance claim.
The private polynomial view is not exported as original rational source.

The shared source limit is 1 MiB / 4096 entities / 65536 values / depth 16.
Helper caps remain 128 rational supports, 512 rational pcurves, 4096 processed
model poles with at most 17 per curve. Original scalar numerator/denominator
limits are 256 bits, Greville/model proof results 1024 bits, homogeneous results
4096 bits. Fixed helper loops and the common source cap bound cumulative work.
Exact finite-decimal private coordinates retain their serialization gate.
Native input/output is capped at 16 MiB. Caller limits, cancellation and atomic
failure are retained. Existing standalone native tags keep their old meanings.

## Independent combined source

The standalone helper directly constructs original rational supports and UV
curves in OCCT, and independently elevates/inserts knots in the original 3D
model curves before XCAF export. The retained original contains 12 rational
supports, 48 rational pcurves and 24 cubic five-pole model curves. The support
weights are `[1,2,3] ⊗ [1,2,3]`; the UV weights are `[1,2,3]`; the model curves
have a genuine internal knot. Normal and shifted unequal UV domains are
retained and tested live. No Go-generated source tokens author the positive.

Original and native-source re-export transfer as three solids / eighteen faces,
volume 10800, area 4860, centroid `(9.3125,14.5,12.0625)`, with original shared
products and colors. The pinned OCCT revision is
`4dbfdcc448c0b3e565488afcd626c84483172db9`.

```sh
sh tools/occtref/build_author_ap242_complete_affine_nurbs_planar.sh \
  /tmp/plainsolid-occt-build/include/opencascade \
  /tmp/plainsolid-occt-build/mac64/clang/lib \
  /tmp/plainsolid-ap242-complete-affine-nurbs-planar
PLAINSOLID_OCCT_COMPLETE_AFFINE_NURBS_PLANAR=/tmp/plainsolid-ap242-complete-affine-nurbs-planar \
  go test ./exchange/step -run '^TestCompleteAffineNURBS' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzCompleteAffineNURBSPlanar$' \
  -fuzztime=40x -parallel=1
```

Focused adversaries cover changed interior model poles; model knot-speed scaling
under the same owner; coherent model translation; coherent wrong UV line,
wrong UV speed and reversal; mixed support coefficients; weight and pole sharing;
owner rewiring; source reorder; forged native evidence in every layer; old-route
rejection; limits; cancellation; and bounded decoder fuzzing. This demonstrates
joint source correspondence rather than acceptance based only on independent
straight-line or planar loci.
