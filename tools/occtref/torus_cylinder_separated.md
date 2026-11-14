# Strict separated torus/cylinder material

`geom.NewSeparatedTorusCylinderMaterial` proves the represented rational
inequalities R>r>0, 0<c<R-r, lower<-r and upper>r. Every torus point has radial
coordinate at least R-r while every cylinder point has radial coordinate at
most c. Thus the closed solids have exact positive separation R-r-c, attained
in their equatorial section. Tangency, overlap, cap-clipped and encompassing
regimes are not accepted by this separate API.

Common is the empty regularized material, Difference retains the full original
torus, and Union retains two disjoint components. The existing exact polynomial
engine supplies bounded rational point classification, finite line intervals,
isolated tangencies, continuous cylinder-generator intervals and original
normal/owner masks. The wrapper intentionally does not expose transverse-only
rim or integration methods: no imaginary transverse rim is manufactured.

`parametric.NewSeparatedTorusCylinderSet` uses existing complete analytic torus
and cylinder constructors. It requires separation greater than eight caller
sewing tolerances and an exactly representable native cylinder height for Fuse.
It retains independent components, optionally as a two-shell native compound,
with original support/component/global face ancestry. Empty Common has no
native boundary; `Compound()` rejects it expressly. Native evaluation keeps its
existing numerical/sewing semantics. No placed STEP paths are modified.

Ideal measures are returned as owned rational coefficients of pi² and pi:

- torus volume 2Rr²*pi², area 4Rr*pi²;
- cylinder volume c²(upper-lower)*pi;
- cylinder area (2c(upper-lower)+2c²)*pi;
- cylinder first Z moment c²(upper²-lower²)*pi/2.

The independent OCCT fixture builds R5/r2 torus and c2 finite cylinder with
caps [-3,4] and [-4,3], then runs actual Common/Cut/Fuse. It requires 0/1/2 solids
and shells respectively, and independently supplies mass and finite X-axis
boundary crossings. Native Common is empty; Cut V=40*pi²; Fuse V=40*pi²+28*pi,
A=40*pi²+36*pi, and first Z moment ±14*pi. Kernel tests also qualify exact
sub-float probes, owner masks at cap rims, isolated torus tangency, strict
one-ULP radial separation, contact/native-clearance rejection, nonrepresentable
height rejection, budgets, cancellation, owned results and native JSON reload.
At deflection0.5 the Cut/Fuse displays have512/2016 triangles respectively,
closed positive incidence and no exact global nonincident contacts.

Pinned OCCT source is `4dbfdcc448c0b3e565488afcd626c84483172db9`, archive SHA256
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.
The comparator uses numerical allowances1e-6 for area/volume/first moment,
1e-7 for centroid and section parameters. Those checks do not replace the
independent exact source proof.

```sh
c++ -std=c++17 -O2 \
  -I/tmp/plainsolid-occt-build/include/opencascade \
  -L/tmp/plainsolid-occt-build/mac64/clang/lib \
  -Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
  tools/occtref/torus_cylinder_separated.cxx \
  -o /tmp/plainsolid-torus-cylinder-separated-oracle \
  -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomAlgo -lTKGeomBase \
  -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_TORUS_CYLINDER_SEPARATED=/tmp/plainsolid-torus-cylinder-separated-oracle \
  go test -race ./internal/occtcheck -run '^TestOCCTSeparatedTorusCylinder$' -count=1
```

Qualified binary SHA256:
`7bf134ec607b3209129a2c9145f855d6ce92836cc5f8b6027ded5b410b146cb9`.
No binary is committed.
