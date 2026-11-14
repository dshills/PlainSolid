# Independent exact cylinder products

`ReadIndependentCylinderMaterials` and `WriteIndependentCylinderMaterials` add
one explicitly selected family to the independent AP214 product collection:
finite cylinders with two half-circle or four quarter-circle lateral cells,
exact model-only LINE/CIRCLE edges, two disk caps, and a signed-coordinate frame.
At least one cylinder product is required. Planar and exact elliptical products
retain their existing proofs. The older collection APIs and version-one native
collection envelope do not accept cylinders.

The shared parser authenticates complete product roots, neutral representation
items, exclusive dependencies, units and cumulative limits. The cylinder proof
then operates on a dependency-closed geometry view with unchanged source IDs and
numbers. It proves directed cell coverage using the existing strict cell reader.
Native geometry uses exact rational quarter patches; source face/edge IDs and
native cap/quarter face correspondences remain separate. This is a collection of
independent products, not a union or a guarantee of spatial disjointness.
Vendor-rounded pcurves, arbitrary angular cells and unsupported product signatures
remain outside this API. Native solids can be saved through trimmed JSON; no new
source-bound collection persistence version is introduced here.

The retained six-root fixture combines the independently OCCT-authored planar and
elliptical products with independently authored model-only half/quarter cylinder
sources. `tools/occtref/author_step_independent_cylinder_materials.py` adds standard
AP214 product records without using the Go writer. The pinned OCCT comparator
checks ordinary TransferRoots, validity, 6 solids, 26 faces and 84 edge uses on
both original and Go output. Expected aggregate volume is `24 + 135.25*pi`;
area is `522.168698195557 + 40*pi`. The cylinder occurrences intentionally coincide.
Set `PLAINSOLID_OCCT_INDEPENDENT_CYLINDER_MATERIALS` to the binary built by
`tools/occtref/build_step_independent_cylinder_materials.sh` to run live checks.

Focused validation includes radius, sense, support type, duplicate face and unit
mutations; atomic failure, cancellation, cumulative bounds, old API rejection,
exact native edge agreement after reload, and a 100-execution bounded fuzz run.
