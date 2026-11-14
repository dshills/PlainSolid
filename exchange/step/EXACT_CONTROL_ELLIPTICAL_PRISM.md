# Exact rational elliptical prism controls

`ReadExactControlEllipticalPrism` and `WriteExactControlEllipticalPrismSTEP`
are a separate bounded model-only STEP dialect. They retain the existing
Pythagorean reader's complete 76-entity ownership, three source faces, shared
full ellipse rims and opposite seam uses. The existing reader still requires
lossless binary64 poles and vertices; it is not widened.

The new native material uses two exact bilinear cap charts and four exact
homogeneous quadratic wall charts. Quarter ellipse weights are (1,1,2),
with rational model endpoints. `trimmed.ExactQuadraticSewFace` constructs
exact parameter/model edge pairs; `trimmed.SewRationalQuadratic` checks exact
vertex identity before continuous edge sewing. Distinct rational vertices
that share a rounded display point fail explicitly. Neither general sewing
helper independently proves global embedding. This codec's positive ellipse
radii, orthonormal rational frame and positive straight height establish the
specific prism's embedding and material orientation.

Source origin and normalized Pythagorean in-plane axes remain rational.
The axis is signed cardinal; dimensions remain losslessly represented
positive binary64, with major radius greater than minor. STEP Cartesian
coordinates must have bounded terminating decimal representations. A required
nonterminating source coordinate fails before output. Native float vertices
are display values; exact rational endpoints and homogeneous coefficients
survive ordinary `trimmed.WriteJSON` / `ReadJSON`. No approximate pcurve or
vendor seam is promoted. The exact volume coefficient returned by
`ExactVolumePiCoefficient` is copied from the immutable construction proof;
it describes the originally read prism, not later edits of public fields.

Independent reference: pinned OCCT 4dbfdcc448c0b3e565488afcd626c84483172db9,
`tools/occtref/step_exact_control_elliptical_prism.cxx`, origin (0.1,2,3),
reference ratio (3,4,0), radii 5 and 2, height 3. OCCT independently authors
the model-only source. Both retained source and generated output transfer as
one valid solid, three faces and six edge uses: volume 94.24777960769373
(30π), area 131.87119085879, center (0.1,2,4.5). The native solid has six
partitioned faces, twelve edges and eight rational vertices; all shared
continuous edge error bounds are exactly zero after native reload. Numerical
native mass integration is a separate coarse regression, not an acceptance
proof.

Focused qualification:

```
PLAINSOLID_OCCT_EXACT_CONTROL_ELLIPTICAL_PRISM=/tmp/plainsolid-step-exact-control-elliptical-prism go test ./exchange/step -run 'Test(OCCT)?ExactControlEllipticalPrism'
go test ./brep/trimmed -run TestExactQuadraticSewAPI
```
