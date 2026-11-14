# Reindexed exact conical frustum material

`ReadReindexedConical45FrustumMaterial` authenticates both unplaced and properly
signed-axis placed45° IGES194 frusta with record, shell-face, edge-list and
vertex-list permutations plus cyclic lateral-loop starts. Exactly one `Local`
or `Placed` canonical native material proof is returned. Existing canonical
readers and their exact source geometry gates remain unchanged.

Roles are established independently of numeric IDs:

- Shell faces have unique (support type,sense) roles: positive194 wall,
  positive190 upper cap and negative190 lower cap.
- Each cap datum must identify exactly one original100 full circular rim by
  exact center and plane height. Its coincident endpoints must be on the
  positive local X ray. Placed curves and planes must own the same proper124.
- The resulting exact world rim endpoints must bijectively match502 vertices.
  Every504 endpoint ordinal is remapped, then the100 rim/110 seam directed
  incidence determines edge roles and every508 edge ordinal is remapped.
- The lateral508 may start cyclically at any of its four established directed
  uses; reversal, changed senses or pcurves remain unsupported.
- A bounded root traversal proves complete reference bijection and shared
  ownership. No unowned record, foreign reference or alias is ignored.

The normalized original scalars then pass the unchanged canonical cone or
placed-cone proof and exact sewn native constructor. Role classification is
never substituted for support identity. `SourceForCanonical`,
`SourceVertexOrdinals`, `SourceUseOrdinals`, `NativeFaceSourceIDs` and
`SourceRimCurveIDs` retain original identities. Separate version1 native
persistence re-derives those maps from retained source and compares the complete
canonical material recipe. Source re-export re-proves native geometry and
ancestry before returning original bytes.

Caps are32KiB source,512records,21entities,2048values,512graph traversal steps
and1MiB native envelope, additionally subject to caller limits. Exact angle45,
representable coordinates and proper signed frames remain required.

## Qualification

`reorder_iges_elliptical_prisms.py --loops --cones` independently permutes both
retained original sources at every supported topology level. Pinned OCCT's
general solid comparator accepts original/re-exported sources with three faces,
one solid and unchanged analytic area, volume and original/posed centroid.
Source and native reload retain identities. Mutations cover cone angle, circle
endpoint, vertex ordinal, shell face sense, foreign root, stale exposed
provenance and resource limits. The bounded seeded target is
`FuzzReindexedConical45Material`. Shared role helpers are regression-tested
against all four elliptical variants and placed circular filled/hollow material.
