# Exact planar material with explicit STEP mapped uses

`ReadMappedPlanarMaterial` handles a previously rejected representation family:
one original planar closed body in ADVANCED_BREP_SHAPE_REPRESENTATION, one
REPRESENTATION_MAP with an identity AXIS2_PLACEMENT_3D origin, and a root
SHAPE_REPRESENTATION containing1–16 named MAPPED_ITEM uses. Each use has an
explicit proper signed-coordinate, unit-scale CARTESIAN_TRANSFORMATION_OPERATOR_3D.
The local and root representations must use the very same unit context.

This is a representation-map workflow, not an inferred NAUO product assembly.
The result preserves one native local Body, actual source body/map/item/operator
IDs, names, units, the original parsed Source, and each explicit Transform.
Repeated uses do not duplicate or modify the underlying body geometry. The old
ReadAssembly continues to reject mapped items.

## Exact proof and limits

The selected local family has cardinal-plane supports and straight, cardinal
model lines with two original planar line pcurves for every model edge. Exact
source decimals must be representable without rounding. Before the existing
planar parser can construct material, a separate exact rational check proves
both endpoints lie on each source model line and on the whole lifted owner
pcurve line. The two pcurve owner surfaces must be precisely the two actual
adjacent faces. Cardinal plane frames are required, so there is no approximate
direction normalization. Existing native exact global boundary embedding and
orientation validation then certifies material. A tiny dyadic line or pcurve
shift that would fit an older relative tolerance still fails this exact gate.

The mapping origin must be identity, all three target axes must be explicit
signed-coordinate unit vectors with X cross Y = Z, target scale must be exactly1,
and target origin must preserve the source numbers. Nested maps, reflected or
scaled targets, implicit axes, mixed unit contexts, detached geometry, missing
owners and extra source material fail. Existing parser depth/entity/value/byte
and native face/vertex/coedge bounds apply. No generic analytic or spline
mapping acceptance is claimed.

The observed pinned OCCT AP214 simple operator uses eight arguments: two empty
functional-transformation strings, an empty representation name, axis1,axis2,
origin,scale,axis3. This codec supports exactly that independently loaded form;
it does not reinterpret a six-argument record or silently discard functional
metadata. Pinned OCCT's Cartesian-operator mapped-item path ignores a nonidentity
mapping origin, which is why that case is explicitly excluded here.

`WriteMappedPlanarMaterialSource` is validated source write-back, not construction
from edited cached Body/Transform fields. It revalidates the supplied source,
retains original header records, IDs and semantic numeric tokens, then writes.
A forged/invalid source produces no output. Generic authoring or editing of this
representation is outside this milestone.

## Independent geometry-first evidence

`author_mapped_planar_box.cxx` authors a native OCCT2×3×4 box and exports its
original advanced planar model, model lines and per-owner pcurves. No Go writer
supplies geometry or numeric tokens. `author_mapped_planar_graph.py` then attaches
the explicitly specified representation-map graph to those unchanged native
geometry records, and directs the original product shape definition to it.
The two target transforms are translation(10,0,0) and
(x,y,z)->(z,x+20,y). The retained361-entity fixture is
`occt-mapped-planar-box.step`.

Pinned OCCT4dbfdcc448c0b3e565488afcd626c84483172db9 loads this graph without
load/transfer warnings. Ordinary STEPControl.TransferRoots and BRepCheck accept
both original source and source write-back: two solids,12faces,volume48,area104,
centroid(6.5,11.25,1.75). The independent `disk_bound_mass` helper measures these
results; set PLAINSOLID_OCCT_DISK_BOUND_MASS for the optional Go differential.
The local native proof separately yields volume24,area52,centroid(1,1.5,2).

Tests reject false scale, nonproper axes, nonidentity/map-cycle origins, wrong
contexts, functional metadata, orphan geometry, mismatched pcurve owners and
tiny exact model/pcurve displacements. Preflight now reports the mapped-material
entity names and the explicit reader as a structural hint only; it is not an
acceptance certificate.
