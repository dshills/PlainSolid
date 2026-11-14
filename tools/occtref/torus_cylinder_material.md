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
 -o /tmp/plainsolid-torus-cylinder-volume-oracle \
 -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomAlgo -lTKGeomBase \
 -lTKG3d -lTKG2d -lTKMath -lTKernel
PLAINSOLID_OCCT_TORUS_CYLINDER=/tmp/plainsolid-torus-cylinder-volume-oracle \
 GOCACHE=/tmp/plainsolid-go-cache go test ./geom ./internal/occtcheck \
 -run '^(TestTorusCylinderMaterial|TestTorusCylinderUnion|TestTorusCylinderVolume|TestOCCTTorusCylinderMaterial)$' -count=1
```

Qualified executable SHA-256:
`1e6d2184588b8582a433681c0a2bcea367485ae2e929316d9d2688928b6b0876`.
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
torus and finite cylinder volumes. No surface-area certificate is claimed.
For R5/r2/c6, width0.1 uses256 cells; width0.01 uses1024. Independent closed
antiderivative numeric checks include c=5 and c=6, all operations. MaxCells,
precision, work and cancellation failures return a zero report.
