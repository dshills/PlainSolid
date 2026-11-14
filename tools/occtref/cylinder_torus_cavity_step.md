# Cylinder with toroidal void STEP transfer

Pinned OCCT source 4dbfdcc448c0b3e565488afcd626c84483172db9.
Build cylinder_torus_cavity_step.cxx using the standard OCCT reference flags
plus TKDESTEP and TKXSBase. Set PLAINSOLID_OCCT_CAVITY_STEP to the executable
and run TestCylinderTorusCavitySTEP.

The independent reader requires one valid solid, two shells, an inward-oriented
original TOROIDAL_SURFACE face, and compares volume/area to 448π−40π² and
240π+40π² at 1e-6 absolute tolerance. Exact native material proof is separate.
The exported first slice accepts integral canonical dimensions bounded by
2^20, including signed/asymmetric axial cap coordinates. There are no
irrational intersection rims in this strict containment family. The existing
analytic primitive writers use represented 2π pcurve endpoints; their seam
roundoff is not an exact symbolic angular encoding. No source-recovery import
or arbitrary placed dialect is claimed by this milestone.
