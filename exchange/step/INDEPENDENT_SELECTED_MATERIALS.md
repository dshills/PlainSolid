# Explicit selection from independent STEP products

`ReadSelectedIndependentMaterials` takes original
`SHAPE_DEFINITION_REPRESENTATION` IDs and returns only their qualified material
members, the complete source, and an explicit list of omitted root IDs. Selection
is opt-in; whole-collection readers remain strict. Members retain source order
regardless of requested order. Unknown, duplicate and empty selections fail.

Every root, including omitted roots, must have the existing bounded independent
AP214 product envelope, identity neutral representation item, and a declared
MANIFOLD_SOLID_BREP or BREP_WITH_VOIDS item. All dependencies are traversed with
cycle and depth checks. Product/context linkage, category/APD attachment,
exclusive ownership, complete entity coverage and orphan rejection apply to the
whole file. Only the previously allowed identity neutral-frame records may be
shared. Cross-root geometry borrowing fails even when the borrowing root was
omitted. This does not validate omitted material geometry or its unit semantics.

Only selected roots enter the planar, elliptical, model-only cylinder or natural
sphere material proofs. Their units must agree. `MaxSolids` bounds **all** input
roots (maximum 16); byte/entity/value/depth limits bound the whole parse and
traversal. Native face/vertex/use limits apply cumulatively to selected output.
Cancellation and any ownership/material failure return no partial members.
The bounded traversal visits each source dependency at most once per root.

Selected member `Source` files can be passed to an appropriate existing writer
and re-proved independently. The retained complete source is not a whole-file
material certificate; neither a whole-file native persistence promise nor an
assembly interpretation is implied.

The fixture appends the untouched original pinned OCCT torus product (major
radius 3, minor radius 1), with ID rebasing only, to seven existing products.
The torus remains unsupported by these collection material decoders. The test
selects the original box and sphere, validates native reload, rejects whole-file
import, and re-exports two products. Independent OCCT accepts all eight original
solids and the two exported solids, with corresponding analytic volumes. Tests
also mutate an omitted root to borrow selected geometry, create a dependency
cycle, or introduce an orphan; all fail. Set
`PLAINSOLID_OCCT_INDEPENDENT_SELECTED_MATERIALS` to the comparator built by
`tools/occtref/build_step_independent_selected_materials.sh`.
