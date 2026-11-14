# Three nested radial contours on original supports

The independent OCCT reference constructs a box on `[-1/8,9/8]² × [0,1]`
and a second stock over `[0,1]²` between `z=-4` and the polynomial roof

```
q = (x-1/2)² + (y-1/2)²
h = -(1225/256) (q-(5/64)²) (q-(15/64)²) (q-(25/64)²)
```

`triple_radial_roofs.cxx` expands the three factors in a two-dimensional
power basis, converts that polynomial to Bézier controls, and sews all six
original stock surfaces in OCCT. It reads no PlainSolid output. `Common`
induces a disk lens and a separate annular lens. Their combined faces number
four; the annulus has one hole on each of its two faces. This is a regular
nested-contour example, not a general Boolean qualification.

```sh
c++ -std=c++17 -O2 -I"$OCCT_INCLUDE" -L"$OCCT_LIB" -Wl,-rpath,"$OCCT_LIB" \
  tools/occtref/triple_radial_roofs.cxx -o /tmp/occt-triple-radial \
  -lTKBO -lTKPrim -lTKTopAlgo -lTKBRep -lTKGeomBase -lTKG3d -lTKG2d -lTKMath -lTKernel
/tmp/occt-triple-radial
```

Pinned OCCT: `4dbfdcc448c0b3e565488afcd626c84483172db9`.
Observed macOS executable SHA-256:
`8eb92fbd6634f112f7e740eda6cc132b4639d56d0c0a67ef634f2bc6476e4a3a`.

The exact reference uses the polar area measure `π dq` and integrates the
roof over `[0,(5/64)²]` and `[(15/64)²,(25/64)²]`. Its result gives an
independent check despite the finite precision of the OCCT Boolean curves.

| Component | Faces | Face holes | Exact volume divided by π | Live OCCT volume |
| --- | ---: | ---: | --- | ---: |
| Disk | 2 | 0 | 613935546875/864691128455135232 | 0.0000022305480189917633 |
| Annulus | 2 | 2 | 478515625/6597069766656 | 0.00022787404880328323 |
| Total | 4 | 2 | 63333935546875/864691128455135232 | 0.00023010459826862094 |
