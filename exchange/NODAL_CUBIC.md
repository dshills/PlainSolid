# Original nodal cubic Common and Fuse transfer

`step.WriteNodalCubicCommon`, `step.WriteNodalCubicFuse`,
`iges.WriteNodalCubicCommon`, and `iges.WriteNodalCubicFuse` transfer the
closed source-backed solids admitted by the trimmed kernel's nodal cubic
family. Each result has the two original roof charts, all four original
node rays, and the seam on the original exterior owner chart. STEP
retains both owner pcurves for each model edge. Both formats retain the
original coordinates for every vertex.

The verifiers take the native Common or Fuse proof and the intended
physical units. They authenticate the canonical exchange graph, including
owner uses and orientations. An explicit `PoleTolerance` bounds the
decimal serialization of derived interior model and pcurve poles. The
fixtures use millimetres and a `1e-14` pole bound.

The two independent original owner orders reopen in pinned OCCT as one
valid solid with 12 faces, 28 edges and 18 original vertices. Common
measures volume `807933/65536` and area `32.5713789018693`; Fuse
measures volume `821259/65536` and area `33.2084356859493`. The
independent readers count the four node rays and the original seam from
the reopened model edge endpoints.

Build the two pinned readers using the
[clipped Cut component-reader instructions](iges/CLIPPED_POLYNOMIAL_CUT.md).
Then run:

```sh
PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS=/tmp/plainsolid-occt-clipped-cut-step \
PLAINSOLID_OCCT_CLIPPED_CUT_IGES=/tmp/plainsolid-occt-clipped-cut-iges \
  go test ./exchange -run '^TestNodalCubicEnvelopeExchange$' -count=1 -v
```

Strict verification rejects changes to the STEP curve master and the
declared IGES physical units. Independent foreign nodal graph recovery
remains outside these source-backed writer and verifier APIs.
