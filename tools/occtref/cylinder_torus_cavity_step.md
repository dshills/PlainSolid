# Cylinder with toroidal void STEP transfer

Pinned OCCT source 4dbfdcc448c0b3e565488afcd626c84483172db9.
Build cylinder_torus_cavity_step.cxx using the standard OCCT reference flags
plus TKDESTEP and TKXSBase. Set PLAINSOLID_OCCT_CAVITY_STEP to the executable
and run TestCylinderTorusCavitySTEP.

The independent reader requires one valid solid, two shells, an inward-oriented
original TOROIDAL_SURFACE face, and compares volume/area to 448π−40π² and
240π+40π² at 1e-6 absolute tolerance. Exact native material proof is separate.
The exporter accepts represented dyadic canonical dimensions bounded by
2^20, including signed/asymmetric axial cap coordinates. The seam radius R+r
and axial difference upper-lower must themselves be exactly representable;
unsupported rounding and native gaps no greater than eight times 1e-9 reject.
All primitive numeric atoms are expanded to exact dyadic decimal spellings.
The dyadic fixture R=5.125, r=2.25, c=8.5, caps[-3.25,4.5] also passes
independent transfer; the oracle optionally takes those five scalar arguments. There are no
irrational intersection rims in this strict containment family. The existing
analytic primitive writers use represented 2π pcurve endpoints; their seam
roundoff is not an exact symbolic angular encoding. No source-recovery import
or arbitrary placed dialect is claimed by this milestone.

The optional eighth argument `oblique` checks the rational proper pose with
X=(3/5,0,-4/5), Z=(4/5,0,3/5), translation=(1/10,-2/5,3/8).
It independently builds original OCCT primitives at that pose and performs
Cut, compares transferred volume/area/centroid, and intersects both independent
and transferred shapes with the transformed original X line. All six retained
boundary parameters are checked against [-8,-7,-3,3,7,8]. Run
TestCylinderTorusCavityObliqueOccurrence with the same environment variable.
The exact occurrence accepts proper rational rotations only when all source
pose and transformed geometry values have bounded finite decimal expansions.
