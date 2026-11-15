# Exact planar components and point contacts in STEP

`ReadVertexContactAssembly` and `WriteVertexContactAssembly` preserve
separately indexed planar bodies and report their exact geometric
vertex contacts. A regular advanced-face file uses explicit straight model
lines and planes. If the original edges carry parameter curves, both
distinct incident owner charts must lift their directed lines exactly.
Vertices must be exactly representable; two different source
coordinates cannot be turned into a contact by rounding.

The exact import checks source dimensional exponents and physical
conversion factors before comparing body coordinates. SI prefixes, inch
and foot use their exact defined ratios; other metre-per-unit values
must be exactly representable. The writer preserves the same ratios
when it composes an advanced cavity representation. Distinct conversion
decimals that happen to round to the same binary64 value are rejected.

`Proof.AllowCavities` admits standard advanced `BREP_WITH_VOIDS`
bodies with an outward closed exterior and directly enclosed closed
voids. Each original void must be referenced by its own reversed
`ORIENTED_CLOSED_SHELL`. Bodies and their declared shell incidences
are reassembled as distinct native topology. The collision proof
checks exact embedding, nesting, and material disjointness, and checks
an independent noncontact vertex on each shell when required.

Faceted output, located occurrences, and different kinds of contact
are outside the cavity option. The opt-in writer reopens the
complete generated file before publishing bytes.

The pinned OCCT witness authors a concave exterior with a void and
a separate part. Both original and canonical files reopen as two
solids, 20 faces, volume 441 and area 398. A third oblique part
that meets only one void corner retains two separate contact
vertex IDs and independently verified net volume `28225/64`.

`ReadEdgeContactPair` and `WriteEdgeContactPair` are separate opt-in
entry points for two single-shell parts whose only contact is one
complete original edge. They retain both edge and incident face owners.
The reader proves full interval coverage and rejects partial-edge and
face contact. Pinned OCCT authors and reopens the tested two-box pair
as two independent solids with 12 faces, volume 2 and area 12.
