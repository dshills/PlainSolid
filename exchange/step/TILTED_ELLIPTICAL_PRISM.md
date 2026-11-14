# Tilted rational-frame elliptical prism

`ReadTiltedEllipticalPrism` / `WriteTiltedEllipticalPrismSTEP` add a distinct
model-only material domain. The normalized extrusion axis must have at least
two nonzero components. Exact rational dot products establish an orthonormal
right-handed basis; direction lengths must have rational square roots. No
floating normalization, tolerance fitting, sampled edge acceptance or seam
repair is used. Existing cardinal-axis APIs retain their old gates.

The reader verifies all 76 entities, the original three-face material shell,
two shared full ellipse rims, and opposite uses of the straight seam. Source
frames, top centers, vertices and line directions must match the exact derived
basis. Original units remain the existing strict millimeter envelope.
The native material uses exact homogeneous cap and wall charts, rational
vertices and exact continuous edge identities, partitioning the periodic wall
into four faces. Ordinary native JSON retains these coefficients and endpoints.
The proof of the specific solid follows from positive radii and height under
a proper rigid frame; the general sewing helper does not assert embedding.

The writer accepts represented direction ratios such as axis `(0,3,4)` and
reference `(1,0,0)`, not rounded binary approximations to their normalized
fractions. Required STEP points and normalized axis components must have
bounded terminating decimal expansions. Source radii and height remain
lossless binary64. This is not arbitrary rational STEP algebra or general
elliptical extrusion import. The source descriptor retains original role IDs.

Independent OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9 authors the retained
fixture using an ellipse at `(0.1,2,3)`, radii 5/2, axis direction `(0,3,4)`
and extrusion displacement `(0,3,4)`. The separately built helper
`step_tilted_elliptical_prism.cxx` validates both source and generated output
using ordinary TransferRoots: one solid, three faces, six edge uses, volume
157.07963267948946 (50π), area 177.89741605011943, and center `(0.1,3.5,5)`.
Native reload checks eight exact rational vertices and zero squared shared-edge
error on every use. Mutations of support radii, frame ratios and a source vertex
fail. The old cardinal reader rejects the tilted source.

Focused normal, race and Go 1.23 checks cover the new and previous APIs. Set
`PLAINSOLID_OCCT_TILTED_ELLIPTICAL_PRISM` to the built helper for the optional
live test. The decoder fuzz target is bounded to 64KiB/76 entities/4096 values
and can be run with `-fuzz FuzzTiltedEllipticalPrism -fuzztime 100x`.
