# Embedded glTF scenes

`ReadGLTFScene` and `ReadGLBScene` return `MeshScene` without flattening instances.
`Meshes` contains named mesh definitions; `Nodes` preserves source indices, names,
children, local transforms and accumulated world transforms. A node's `Mesh` is
an index into `Meshes`, or -1 for a transform-only node. `Roots` identifies the
single selected scene's root nodes. Names are optional and need not be unique.
Geometry coordinates and glTF's coordinate conventions remain unchanged.

The supported subset has one scene, one embedded buffer, multiple mesh definitions,
and one POSITION-only triangle primitive per mesh. Float32 positions and implicit
or uint8/16/32 indices use the existing strict accessor and bounds validation.
Repeated nodes share a definition; no vertex data is copied for an instance.
Unused mesh definitions are preserved, but unused nodes are rejected. Materials,
textures, extensions, animations, cameras, skins and multiple scenes are unsupported.

Nodes accept glTF column-major affine matrices or TRS (translation, quaternion
rotation in x/y/z/w order, scale), never both. Local matrices must decompose into
TRS without shear. Quaternions must be unit length within 1e-6 and are normalized
within that tolerance. Nonuniform and negative scales are supported; singular
transforms, nonfinite composition and overflowing transformed mesh bounds fail.
World transforms use parent * local composition. The hierarchy must be a forest:
cycles, duplicate roots, duplicate child links and multiple parents fail.

`SceneLimits` embeds the existing byte/vertex/triangle `Limits` and adds
`MaxNodes` (default 100000), `MaxMeshes` (10000), and `MaxDepth` (256, hard maximum
4096). Vertex/triangle limits count the sum over mesh definitions, not occurrences.
Input bytes bound JSON allocations; traversal is iterative and depth checked.
The embedded payload is decoded once, then compact accessor descriptions reuse
the existing mesh decoder. GLB accepts JSON followed by at most one BIN chunk,
with strict sizes, alignment and padding. External URIs are never fetched.

`ReadGLTF` and `ReadGLB` retain their original single identity instance contract.
The new APIs do not change or infer CAD topology, units beyond glTF conventions,
or persistent identities across model edits.

`WriteGLTFScene` and `WriteGLBScene` emit this same subset with the default
`SceneLimits`. They preserve names, definition indices, root/child order and
shared mesh references. `Local` is authoritative; `World` is ignored and
recomputed by the reader. Callers must supply valid local transforms (use
`geom.Identity()` for an identity node) and Mesh=-1 for transform-only nodes.
Local transforms are emitted as glTF column-major matrices, preserving TRS
semantics without a decomposition round trip. The writers convert positions
to float32 once per definition and reject overflow or triangle collapse.
They validate the staged forest, transformed bounds and total resource usage
before writing any destination bytes. Writer and short-write errors propagate.
JSON field names, merged binary offsets, shared references and GLB chunks have
independent inspection tests in addition to reader round trips.
