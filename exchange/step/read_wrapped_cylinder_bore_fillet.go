package step

import (
	"context"
	"errors"
	"io"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/borefillet"
	"plainsolid/geom"
)

// WrappedCylinderBoreRimFilletSTEP retains the exact model-only source and
// the reconstructed native boundary. Complete circular source edges may
// supply four native quadrants. A zero vertex source ID identifies a
// cardinal vertex introduced by that exact subdivision.
type WrappedCylinderBoreRimFilletSTEP struct {
	Name                                     string
	LengthUnit                               float64
	Solid                                    trimmed.Solid
	Source                                   *File
	SourceBodyID, SourceShellID              int64
	NativeFaceSourceIDs, NativeEdgeSourceIDs []int64
	NativeVertexSourceIDs                    []int64
}

var errBoreRimFilletSTEP = errors.New("step: unsupported exact model-only circular bore rim fillet")

type boreRimSTEPFace struct {
	id           int64
	kind         borefillet.SupportKind
	center       geom.Vec3
	axis         geom.Vec3
	major, minor float64
	sense        bool
	loops        [][]analyticUse // sense and order already reflect FACE_BOUND orientation
	outerBounds  []bool
}

// ReadWrappedCylinderBoreRimFillet strictly reconstructs one analytic body
// with a circular through-bore and one or both internal quarter-toroids.
// It authenticates every model curve, vertex and directed incidence before
// returning material. It accepts full-circle and quadrant rim partitions in
// a model-only STEP graph. The returned solid remains in the original units.
func ReadWrappedCylinderBoreRimFillet(ctx context.Context, r io.Reader, l Limits, o geom.SurfacePatchOptions) (WrappedCylinderBoreRimFilletSTEP, error) {
	var out WrappedCylinderBoreRimFilletSTEP
	if ctx == nil || r == nil {
		return out, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	l, err := l.defaults()
	if err != nil {
		return out, err
	}
	l.MaxBytes = min(l.MaxBytes, 128<<10)
	l.MaxEntities = min(l.MaxEntities, 512)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, rep, _, items, err := parseCanonicalCircularExtrusions(r, l)
	if err != nil {
		return out, err
	}
	representation, _ := simple(f, rep, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	unit, err := contextUnitsWithExactPlanar(f, representation.Args[2].Ref, true)
	if err != nil {
		return out, err
	}
	bodyID, err := boreRimSTEPBody(f, rep, items)
	if err != nil {
		return out, err
	}
	body, _ := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	name, err := textValue(body.Args[0])
	if err != nil {
		return out, err
	}
	shellID, err := ref(body.Args[1])
	if err != nil {
		return out, err
	}
	shell, err := simple(f, shellID, "CLOSED_SHELL", 2)
	if err != nil {
		return out, err
	}
	ids, err := listRefs(shell.Args[1], 5)
	if err != nil || len(ids) > 6 {
		return out, errBoreRimFilletSTEP
	}
	faces, err := readBoreRimSTEPFaces(f, ids)
	if err != nil {
		return out, err
	}
	s, faceSources, err := reconstructBoreRimSTEP(ctx, faces, o)
	if err != nil {
		return out, err
	}
	edgeSources, vertexSources, err := proveBoreRimSTEPIncidence(ctx, f, faces, s, faceSources, o)
	if err != nil {
		return out, err
	}
	return WrappedCylinderBoreRimFilletSTEP{
		Name: name, LengthUnit: unit, Solid: s, Source: f, SourceBodyID: bodyID, SourceShellID: shellID,
		NativeFaceSourceIDs: faceSources, NativeEdgeSourceIDs: edgeSources, NativeVertexSourceIDs: vertexSources,
	}, nil
}

func boreRimSTEPBody(f *File, rep int64, items []int64) (int64, error) {
	var body, datum int64
	for _, id := range items {
		if _, err := simple(f, id, "MANIFOLD_SOLID_BREP", 2); err == nil && body == 0 {
			body = id
			continue
		}
		p, axis, err := boreRimSTEPFrame(f, id)
		if err != nil || datum != 0 || p != (geom.Vec3{}) || axis != (geom.Vec3{Z: 1}) {
			return 0, errBoreRimFilletSTEP
		}
		datum = id
	}
	if body == 0 {
		return 0, errBoreRimFilletSTEP
	}
	geometry, err := nauoClosure(f, []int64{rep}, 16)
	if err != nil {
		return 0, err
	}
	roots := []int64{rep}
	counts := map[string]int{}
	for _, id := range f.Order {
		for _, r := range f.Entities[id].Records {
			counts[r.Type]++
			switch r.Type {
			case "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT_RELATED_PRODUCT_CATEGORY":
				roots = append(roots, id)
			case "SHAPE_DEFINITION_REPRESENTATION":
				if len(r.Args) != 2 || r.Args[1].Kind != Reference || r.Args[1].Ref != rep {
					return 0, errBoreRimFilletSTEP
				}
				if _, err := simple(f, r.Args[0].Ref, "PRODUCT_DEFINITION_SHAPE", 3); r.Args[0].Kind != Reference || err != nil {
					return 0, errBoreRimFilletSTEP
				}
				roots = append(roots, id)
			}
		}
	}
	if counts["MANIFOLD_SOLID_BREP"] != 1 || counts["CLOSED_SHELL"] != 1 ||
		counts["SHAPE_DEFINITION_REPRESENTATION"] != 1 || counts["APPLICATION_PROTOCOL_DEFINITION"] != 1 ||
		counts["PRODUCT_RELATED_PRODUCT_CATEGORY"] > 1 {
		return 0, errBoreRimFilletSTEP
	}
	all, err := nauoClosure(f, roots, 16)
	if err != nil || len(all) != len(f.Entities) {
		return 0, errBoreRimFilletSTEP
	}
	for _, id := range f.Order {
		for _, r := range f.Entities[id].Records {
			if geometry[id] {
				switch r.Type {
				case "ADVANCED_BREP_SHAPE_REPRESENTATION", "MANIFOLD_SOLID_BREP", "CLOSED_SHELL",
					"ADVANCED_FACE", "FACE_BOUND", "FACE_OUTER_BOUND", "EDGE_LOOP", "ORIENTED_EDGE",
					"EDGE_CURVE", "VERTEX_POINT", "LINE", "CIRCLE", "CYLINDRICAL_SURFACE", "TOROIDAL_SURFACE", "PLANE",
					"AXIS2_PLACEMENT_3D", "VECTOR", "DIRECTION", "CARTESIAN_POINT":
					if len(r.Args) == 0 || r.Args[0].Kind != String {
						return 0, errBoreRimFilletSTEP
					}
					continue
				}
				if independentShareable(f.Entities[id]) {
					continue
				}
			} else {
				switch r.Type {
				case "APPLICATION_PROTOCOL_DEFINITION", "APPLICATION_CONTEXT", "SHAPE_DEFINITION_REPRESENTATION", "PRODUCT_DEFINITION_SHAPE",
					"PRODUCT_DEFINITION", "PRODUCT_DEFINITION_FORMATION", "PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE", "PRODUCT",
					"PRODUCT_CONTEXT", "PRODUCT_DEFINITION_CONTEXT", "MECHANICAL_CONTEXT", "DESIGN_CONTEXT", "PRODUCT_RELATED_PRODUCT_CATEGORY":
					continue
				}
			}
			return 0, errBoreRimFilletSTEP
		}
	}
	return body, nil
}

func boreRimSTEPFrame(f *File, id int64) (origin, axis geom.Vec3, err error) {
	r, e := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return origin, axis, e
	}
	p, e := namedReferences(r, 1, 2, 3)
	if e != nil {
		return origin, axis, e
	}
	origin, e = mappedExactCoordinate(f, p[0], "CARTESIAN_POINT", 3)
	if e != nil {
		return origin, axis, e
	}
	axis, e = mappedExactCoordinate(f, p[1], "DIRECTION", 3)
	if e != nil {
		return origin, axis, e
	}
	x, e := mappedExactCoordinate(f, p[2], "DIRECTION", 3)
	if e != nil || x != (geom.Vec3{X: 1}) {
		return origin, axis, errBoreRimFilletSTEP
	}
	return origin, axis, nil
}

func reconstructBoreRimSTEP(ctx context.Context, faces []boreRimSTEPFace, o geom.SurfacePatchOptions) (trimmed.Solid, []int64, error) {
	var caps, cylinders [2]*boreRimSTEPFace
	var blends [2]*boreRimSTEPFace
	for i := range faces {
		f := &faces[i]
		switch f.kind {
		case borefillet.Plane:
			end := 0
			if f.sense == (f.axis.Z > 0) {
				end = 1
			}
			if caps[end] != nil || len(f.loops) != 2 {
				return trimmed.Solid{}, nil, errBoreRimFilletSTEP
			}
			caps[end] = f
		case borefillet.Cylinder:
			side := 0
			if !f.sense {
				side = 1
			}
			if cylinders[side] != nil || len(f.loops) != 1 {
				return trimmed.Solid{}, nil, errBoreRimFilletSTEP
			}
			cylinders[side] = f
		}
	}
	if caps[0] == nil || caps[1] == nil || cylinders[0] == nil || cylinders[1] == nil ||
		caps[0].center.Z >= caps[1].center.Z {
		return trimmed.Solid{}, nil, errBoreRimFilletSTEP
	}
	hole := cylinders[1]
	var radii [2]float64
	for i := range faces {
		f := &faces[i]
		if f.kind != borefillet.Torus {
			continue
		}
		if !f.sense || len(f.loops) != 1 || f.center.X != hole.center.X || f.center.Y != hole.center.Y ||
			new(big.Rat).Add(sideRat(hole.major), sideRat(f.minor)).Cmp(sideRat(f.major)) != 0 {
			return trimmed.Solid{}, nil, errBoreRimFilletSTEP
		}
		end := -1
		for j, cap := range caps {
			if boreRimFacesShareEdge(f, cap) {
				if end >= 0 {
					return trimmed.Solid{}, nil, errBoreRimFilletSTEP
				}
				end = j
			}
		}
		if end < 0 || blends[end] != nil {
			return trimmed.Solid{}, nil, errBoreRimFilletSTEP
		}
		span := new(big.Rat).Sub(sideRat(f.center.Z), sideRat(caps[end].center.Z))
		if end == 1 {
			span.Neg(span)
		}
		if span.Cmp(sideRat(f.minor)) != 0 {
			return trimmed.Solid{}, nil, errBoreRimFilletSTEP
		}
		blends[end], radii[end] = f, f.minor
	}
	var walls [2]*geom.WrappedSurfacePatch
	for i, f := range cylinders {
		w, err := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: f.center.X, Y: f.center.Y}, f.major, 1, o)
		if err != nil {
			return trimmed.Solid{}, nil, err
		}
		walls[i], err = geom.ReheightWrappedCylinder(ctx, w, sideRat(caps[0].center.Z), sideRat(caps[1].center.Z), o)
		if err != nil {
			return trimmed.Solid{}, nil, err
		}
	}
	stock, _, err := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, walls[0], walls[1], 1e-9, o)
	if err != nil {
		return trimmed.Solid{}, nil, err
	}
	s, h, err := trimmed.FilletWrappedCylinderBoreRims(ctx, stock, radii[0], radii[1], o)
	if err != nil {
		return trimmed.Solid{}, nil, err
	}
	sources := make([]int64, len(faces))
	sources[h.Outer], sources[h.Bore] = cylinders[0].id, cylinders[1].id
	for end, cap := range caps {
		sources[h.Caps[end]] = cap.id
		if blends[end] != nil {
			sources[h.Blends[end]] = blends[end].id
		}
	}
	return s, sources, nil
}

func boreRimFacesShareEdge(a, b *boreRimSTEPFace) bool {
	for _, u := range a.loops[0] {
		for _, loop := range b.loops {
			for _, v := range loop {
				if u.id == v.id {
					return true
				}
			}
		}
	}
	return false
}
