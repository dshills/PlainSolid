# Coaxial torus/cylinder material differential

Pinned OCCT source `4dbfdcc448c0b3e565488afcd626c84483172db9`, archive
`/tmp/plainsolid-occt-4dbfdcc.tar.gz`, SHA-256
`32b1e918a697a0398e8ce623099af2c1d62cdeda2b2eefa86f07c0aee9d4aaf7`.

The C++ fixture independently authors an origin-centered Z-axis ring torus
(major radius 5, minor radius 2) and finite cylinder (radius 6, z=-3..4),
then constructs actual OCCT Common, Cut and Fuse. Each must pass BRepCheck and have
one solid. No PlainSolid-produced BRep, mesh or boundary samples are consumed.

The line is p(t)=±(t,0,t/10), t in [-8,8]. OCCT's normalized physical
parameter is divided by sqrt(1.01) to recover original t. Four retained crossings for Common/Cut and two for Union are required, and both interval midpoints must be inside each
independent OCCT solid. Go compares all roots and checks exact source masks:
Common cylinder/torus/torus/cylinder; Difference the reverse ownership.
Union additionally tests forward/reversed axial cap crossings at z=-3 and4.
The kernel separately tests cap-plane coincidence, hidden wall intervals and full cylinder-generator coincidence, irrational
rim endpoints ±sqrt(3) with both owners, rational rim ties, isolated double
root touches, buried contacts, sub-float point probes and strict domain gates.

OCCT tolerance is 1e-10; raw-t comparison allowance is 1e-7. Oracle duplicate
face intersections are merged within 1e-8 only on these separated roots.
This finite-precision differential does not establish exact source ownership
or source regularity; those are independently proved by the kernel.

```sh
c++ -std=c++17 -O2 \
 -I/tmp/plainsolid-occt-build/include/opencascade \
 -L/tmp/plainsolid-occt-build/mac64/clang/lib \
 -Wl,-rpath,/tmp/plainsolid-occt-build/mac64/clang/lib \
 tools/occtref/torus_cylinder_material.cxx \
 -o /tmp/plainsolid-torus-cylinder-moments-oracle \
 -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomAlgo -lTKGeomBase \
 -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_TORUS_CYLINDER=/tmp/plainsolid-torus-cylinder-moments-oracle \
 GOCACHE=/tmp/plainsolid-go-cache go test ./geom ./internal/occtcheck \
 -run '^(TestTorusCylinderMaterial|TestTorusCylinderUnion|TestTorusCylinderVolume|TestTorusCylinderArea|TestTorusCylinderMoments|TestOCCTTorusCylinderMaterial)$' -count=1
```

Qualified executable SHA-256:
`b4a01a67ff6a3ca696241ab9e790b70b1153e4b4ffe6718632799fc5cc644a6e`.
No executable or generated reference data is committed. This milestone does
not add native torus/cylinder Boolean assembly, arbitrary caps, tangency source
regimes. The exposed original torus/wall/cap supports are distinguished even
where a line lies continuously on their cylindrical boundary.


## Certified volume comparison

OCCT independently integrates actual Boolean mass with BRepGProp. The Go
comparator requires it inside the certified rational volume interval (requested
width 0.01), with an additional 1e-6 finite-precision comparison allowance.
This is not an exactness claim for OCCT's numeric mass.

The kernel integrates cylindrical radial sections, not OCCT data. Writing
`a=c-R`, common volume is
`4*pi*(R*integral[-r,a]sqrt(r²-x²) dx - (r²-a²)^(3/2)/3)`.
Concavity proves trapezoid lower and midpoint upper integral bounds even at
the singular endpoint. Each sqrt is outward rational; pi is bounded by Machin's
identity using alternating arctangent remainders. All levels share one arithmetic
ledger. Difference and Union use rigorous interval subtraction from the full
torus and finite cylinder volumes. The separate area method below uses a direct meridian-angle proof.
For R5/r2/c6, width0.1 uses256 cells; width0.01 uses1024. Independent closed
antiderivative numeric checks include c=5 and c=6, all operations. MaxCells,
precision, work and cancellation failures return a zero report.


## Original-owner retained boundary area

The oracle identifies actual OCCT face supports with BRepAdaptor_Surface:
toroidal faces, cylindrical faces, and the lower/upper planar cap. It integrates
each retained face with BRepGProp and sums each owner group independently. Each
owner sum must lie within the kernel interval requested at width1e-6, allowing
an additional 1e-6 for OCCT numerical integration.

For a=c-R, h=sqrt(r²-a²), alpha=2*atan((r+a)/h), common toroidal area is
4*pi*r*(R*alpha-h); common/difference cylinder-wall area is4*pi*c*h.
Difference retains the complementary torus band with inward wall orientation.
Union retains that complementary torus band, the finite cylindrical wall minus
the common wall band, and two full disks. Rational range reduction places each
arctangent argument inside[-1/2,1/2], with alternating remainder bounds; all
square roots, pi and final areas are enclosed in exact rational arithmetic.
Seams have zero area and retain their separate exact query ownership.
The kernel tests c=3.25,5,6,6.75, all operations, original owner/normal fields,
independent closed-form evaluations, precision and cumulative operation limits.


## Axial centroid and inertia

OCCT's independently integrated original Boolean reports CentreOfMass and
MatrixOfInertia (about its center). The comparator checks axial first moment,
axial centroid, diagonal central inertia and reconstructed raw second moments
against the kernel intervals. Numerical allowance is1e-5. Each operation is
integrated once and reused for repeated line reference rows.

The kernel's shared radial partition integrates nonnegative weights rho,
rho³ and rho*(r²-(rho-R)²) against h=sqrt(r²-(rho-R)²). Concavity provides
lower endpoint chords and upper midpoint tangents. Polynomial-weighted line
integrals are exact. Tangent upper terms are rounded upward to a common dyadic
lattice to bound denominator growth; this error remains in the final enclosure.
They produce V=4*pi*I1, Qxx=Qyy=2*pi*I3, Qzz=4*pi*Iz/3. Full torus and finite
cylinder moments are analytic, with outward pi. Unequal cap heights contribute
the signed cylinder first moment in Union; other operations are z-symmetric.
Centroid and parallel-axis inertia use outward positive-volume division.

The four explicitly requested widths are volume0.01, raw second moment0.1,
centroid0.001 and inertia0.2. The source domain is unchanged. Unit tests use
c=R with an independent closed antiderivative, both cap ranges[-3,4] and[-4,3],
all operations, and measured one-operation-short/cell/cancellation failure.
No mixed-coordinate placement or radial containment extension is included.


## Native Common/Difference boundary qualification

The same independent OCCT references now also check actual parametric native
Common/Difference surface mass and area (1e-5 numerical allowance). The native
constructor uses two periodic analytic bands, explicit opposite rim incidence,
and distinct chart seams: V2/E4/F2, Euler0. Its torus meridian is a rational
circle parameterization with exact original coefficient K and algebraic sqrt(K),
not an acos-authored floating trim. Both rim heights retain immutable exact roots
of z²-[r²-(c-R)²], including the fixture's irrational ±sqrt(3). Source owners and
lower/upper rim edge IDs are exposed separately.

The exact original support/meridian identities and abstract topology are
separate from numerical native Eval and tolerance sewing. JSON stores original
radii and chart roles and reconstructs derived values; tests reject a tangent
radius tamper and compare canonical reload. Bounds and real-arithmetic derivative
interpolation bounds are registered; numerical evaluation roundoff remains outside
that deflection contract, as for the existing analytic parametric mesher.
Independent native tests qualify positive-volume, closed 1024-triangle displays
at requested0.5 analytic deflection with no nonincident triangle contacts.
The solid is not a mesh substitution. Native Union and arbitrary clipped caps
are still unsupported by this first native constructor; analytic queries retain
their wider existing all-operation domain.
