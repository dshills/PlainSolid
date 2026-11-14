# Exact alternative disk-hole bound encoding

`ReadDiskHoleBoundEncoding` is a separate opt-in adapter for the already-proved
canonical disk-hole material family. It accepts a negative `FACE_BOUND` or
`FACE_OUTER_BOUND` orientation when the entire ordered loop and every directed
coedge are reversed consistently. It also permits generic `FACE_BOUND` in a
face's first exterior slot, but only if complete canonical reconstruction proves
that the slot is the actual exterior loop. It never infers an exterior loop from
approximate area or samples.

The normalization is an exact topological identity: changing the sign of the
bound and the direction of its whole traversal leaves the oriented boundary
unchanged. Bounds, loops and oriented-edge records must each have exactly one
incoming owner reference. Model edge records remain shared between their proper
faces. All loop ordering, edge endpoints, model/pcurve definitions and surface
ownership subsequently pass the original bijective canonical source graph proof.
A partial reversal, altered geometry, or different curve/trim encoding fails.
No model scalar, coordinate, analytic support or pcurve coefficient is modified.

The result retains the original parsed Source, exact source IDs of normalized
bounds, generic exterior bounds, and recovered DiskHoleExtrusionModel. Original
bound sense values remain visible in Source. The output model can use the existing
canonical writer. Existing readers are unchanged; this is not a general vendor
STEP importer and inherits the original canonical codec's parameter conventions.
Caller parsing/entity/depth/material budgets remain enforced; no result is
published before complete normalization and material verification.

## Independent representation and material evidence

`author_disk_bound_base.go` emits an explicitly identified PlainSolid canonical
baseline with R5,H4 stock, an R1 through hole at X=-2, and an R1 depth2 blind
opening at X=2. It is not described as an independently authored geometry fixture.
`reverse_disk_bounds.cxx` independently loads that graph through pinned OCCT's
STEP entity model, reads actual FaceBound/EdgeLoop/OrientedEdge relationships,
and authors opposite complete traversals using those source objects. It replaces
only topology records in the original text, retaining all numeric tokens verbatim
so OCCT's ordinary numeric formatting cannot round the original source basis.
The retained `disk-bound-occt-reversed.step` has nine reversed bounds, including
six generic exterior bounds; it was rejected by the old strict reader.

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9 ordinary TransferRoots and
BRepCheck accept the independently encoded input and canonical reexport:
one solid,six faces,volume94*pi,area100*pi,centroid(2/47,0,95/47).
`disk_bound_mass.cxx` independently computes those properties. Enable the optional
Go differential with `PLAINSOLID_OCCT_DISK_BOUND_MASS` pointing to that helper.
The representation author and mass helper are separate from the Go decoder.

## Explicit vendor boundary

The separately retained production-like OCCT disk-hole assembly uses seam points
such as (3,-7.347880794884E-16,z). Its squared radius is exactly greater than9
under the authored decimal interpretation, and its seam pcurve uses the rounded
period6.28318530718. Those are geometric discrepancies, not directed-loop
encoding differences. This adapter does not normalize or repair them. Both that
assembly and the independent standalone vendor disk-pocket source still fail.
The existing relative1e-9 analytic importer is a different contract and is not
called by this exact representation adapter.

Regressions cover wrong bound sense alone, one wrong coedge sense, wrong loop
order alone, shared coedges, radius changes, budgets, source immutability,
canonical reader rejection and bounded fuzz. These checks establish one new
source representation variation, not broad OCCT replacement parity.
