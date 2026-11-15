# Original cubic nonplanar STEP material

`ReadCubicNonplanarSplineShellSource` admits a bounded six-face polynomial
spline material with carrier degrees 1–3 and at most four original poles per
axis. It shares the exact original-source owner, model-edge, endpoint,
coedge, sewing and complete global embedding proof with the quadratic route.
The existing quadratic and multispan-UV public readers keep their old carrier
limits. This route still requires 24 uniquely owned two-pole degree-one UV
splines; it does not admit arbitrary nonlinear trim speed or rational carriers.

No source carrier is simplified. Original polynomial poles, knots, model
curves and UV ownership are retained, with exact source-decimal/binary equality
and complete continuous model/support comparison. At least one carrier must
be nonplanar. The existing native kernel must certify complete outward
material embedding; mere closed incidence is insufficient. Limits remain
1 MiB source, 2048 entities, 32768 values, parser depth 16 and six faces/twelve
edges. The cubic carrier gate bounds each net to 16 poles.

The distinct `plainsolid-step-cubic-nonplanar-spline-shell-source` version 1
native envelope retains source, original role IDs, canonical generated STEP
and native geometry claims. Reload reconstructs and re-proves from original
STEP and compares every claim. Stored native geometry cannot replace the
proof. Source writer emits the original file; `WriteSplineShells` emits a new
standard STEP model on the original retained carriers. Prior native tags are
unchanged and cannot be supplied as cubic tags.

## Independent source

`tools/occtref/author_ap242_cubic_spline_roof.cxx` builds six original OCCT
B-spline faces, sews/validates the solid and exports AP242. It uses no Go STEP
output or source tokens. On OCCT revision
`4dbfdcc448c0b3e565488afcd626c84483172db9`, the retained source has 350 entities.
The physical domain is

```
0 <= x <= 6, 0 <= y <= 6, 0 <= z <= 2+(x/6)^3+(y/6)^3.
```

The roof is an original bicubic 4×4 polynomial carrier with nonconstant
curvature in both axes. Four planar side carriers have cubic parameterizations
and curved upper rims. The bottom carrier is bilinear. Original poles are
integers, and the 24 original owner pcurves are bounded polynomial splines.
The new gate is necessary: the old source API rejects cubic carriers.

Independent analytic volume is 90 and centroid is
`(159/50,159/50,359/280)`. Surface area is
`102 + 36*integral_0^1 integral_0^1 sqrt(1+(u^4+v^4)/4) du dv`.
Pinned OCCT reports `139.72366383868012`. Tests compare native quadrature,
original source transfer and generated STEP transfer after source-bound native
reload. They also evaluate retained roof interiors with exact rational
arithmetic against the original cubic equation, verify zero squared continuous
edge discrepancy, and re-certify embedding after ordinary native reload.

```sh
sh tools/occtref/build_author_ap242_cubic_spline_roof.sh \
 /tmp/plainsolid-occt-build/include/opencascade \
 /tmp/plainsolid-occt-build/mac64/clang/lib \
 /tmp/plainsolid-ap242-cubic-spline-roof
PLAINSOLID_OCCT_CUBIC_SPLINE_ROOF=/tmp/plainsolid-ap242-cubic-spline-roof \
 go test ./exchange/step -run '^TestCubicNonplanarSplineShell' -count=1
go test -race ./exchange/step -run '^TestCubicNonplanarSplineShell' -count=1
go test ./exchange/step -run '^$' -fuzz '^FuzzCubicNonplanarSplineShellSource$' -fuzztime=40x -parallel=1
```

Adversaries include changed curved rim pole, quartic support, incorrect UV
speed/owner, source decimal loss, exponent attacks, detached geometry, face
sense and native source/geometry/tag forgery. Harmless record reorder,
source byte identity, copied ancestry and bounded/canceled atomic failures
are covered. No central status claims or generic STEP acceptance are changed.
