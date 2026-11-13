package step

import (
	"errors"
	"io"
	"plainsolid/geom"
	"strings"
)

// RectangularSplineFace is a non-solid, positively oriented full-domain patch.
// Its four boundaries are exact clamped isocurves; arbitrary trims are excluded.
type RectangularSplineFace struct {
	Name     string
	Geometry *geom.BSplineSurface
}
type SplineFaceModel struct {
	Faces      []RectangularSplineFace
	LengthUnit float64
}

var splineFaceEnds = [4][2]int{{0, 3}, {0, 1}, {1, 2}, {3, 2}}

func certifySplineFacePatch(patch *geom.SurfacePatch) error {
	// The diagonal integer transforms are exact in homogeneous arithmetic. They
	// permit a quarter cylinder whose axis-aligned projections degenerate at ends.
	transforms := []geom.Affine{geom.Identity(), {{1, -1, 0, 0}, {0, 0, 1, 0}, {0, 1, 0, 0}}, {{1, 1, 0, 0}, {0, 0, 1, 0}, {0, 1, 0, 0}}}
	for _, transform := range transforms {
		p, e := patch.Transformed(transform)
		if e != nil {
			continue
		}
		for _, axes := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
			proof, e := geom.CertifySurfaceInjectivity(p, geom.SurfaceInjectivityOptions{ProjectionAxes: axes, MaxDepth: 4, MaxCells: 64, MaxOperations: 200000, MaxCoefficientBits: 4096})
			if e == nil && proof.ProvenInjective {
				return nil
			}
		}
	}
	return errors.New("step: unresolved rectangular patch injectivity")
}
func decodeSplineFace(f *File, id int64, maxPoints int) (RectangularSplineFace, error) {
	b, e := decodeBoundedSplineFace(f, id, maxPoints)
	if e != nil {
		return RectangularSplineFace{}, e
	}
	u0, u1, v0, v1 := b.Geometry.Domain()
	if b.Domain != (geom.ParameterRectangle{UMin: u0, UMax: u1, VMin: v0, VMax: v1}) {
		return RectangularSplineFace{}, errors.New("step: bounded trim requires ReadBoundedSplineFaces")
	}
	return RectangularSplineFace{b.Name, b.Geometry}, nil
}
func ReadBoundedSplineFaces(r io.Reader, l Limits) (BoundedSplineFaceModel, error) {
	l, e := l.defaults()
	if e != nil {
		return BoundedSplineFaceModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return BoundedSplineFaceModel{}, e
	}
	return readSplineFaceGraph(f, l, decodeBoundedSplineFace, 4)
}
func readSplineFaceGraph(f *File, l Limits, decode func(*File, int64, int) (BoundedSplineFace, error), edgeCount int) (BoundedSplineFaceModel, error) {
	m := BoundedSplineFaceModel{}
	used := map[int64]bool{}
	surfaces := map[int64]bool{}
	points := 0
	fail := func() (BoundedSplineFaceModel, error) {
		return BoundedSplineFaceModel{}, errors.New("step: unsupported spline face surface model")
	}
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" {
				return fail()
			}
			if strings.HasSuffix(rec.Type, "SHAPE_REPRESENTATION") && rec.Type != "MANIFOLD_SURFACE_SHAPE_REPRESENTATION" {
				return fail()
			}
		}
		if len(ent.Records) != 1 || ent.Records[0].Type != "MANIFOLD_SURFACE_SHAPE_REPRESENTATION" {
			continue
		}
		rep, e := simple(f, id, "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		ctx, e := ref(rep.Args[2])
		if e != nil {
			return fail()
		}
		unit, e := contextUnits(f, ctx)
		if e != nil || m.LengthUnit != 0 && unit != m.LengthUnit {
			return fail()
		}
		m.LengthUnit = unit
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return fail()
		}
		for _, item := range items {
			if _, e := simple(f, item, "AXIS2_PLACEMENT_3D", 4); e == nil {
				if _, e := placement(f, item); e != nil {
					return fail()
				}
				continue
			}
			model, e := simple(f, item, "SHELL_BASED_SURFACE_MODEL", 2)
			if e != nil {
				return fail()
			}
			shells, e := listRefs(model.Args[1], 1)
			if e != nil {
				return fail()
			}
			for _, shellID := range shells {
				shell, e := simple(f, shellID, "OPEN_SHELL", 2)
				if e != nil {
					return fail()
				}
				faces, e := listRefs(shell.Args[1], 1)
				if e != nil || len(faces) != 1 {
					return fail()
				}
				fid := faces[0]
				if used[fid] || len(m.Faces) >= l.MaxFaces || len(m.Faces) >= l.MaxSolids || edgeCount > l.MaxEdgeUses/(len(m.Faces)+1) {
					return fail()
				}
				face, e := decode(f, fid, l.MaxVertices-points)
				if e != nil {
					return BoundedSplineFaceModel{}, e
				}
				points += len(face.Geometry.Data().Poles)
				used[fid] = true
				record, _ := simple(f, fid, "ADVANCED_FACE", 4)
				sid, _ := ref(record.Args[2])
				if surfaces[sid] {
					return fail()
				}
				surfaces[sid] = true
				m.Faces = append(m.Faces, face)
			}
		}
	}
	for id, ent := range f.Entities {
		for _, rec := range ent.Records {
			if rec.Type == "ADVANCED_FACE" && !used[id] || rec.Type == "B_SPLINE_SURFACE_WITH_KNOTS" && !surfaces[id] {
				return fail()
			}
		}
	}
	if len(m.Faces) == 0 {
		return fail()
	}
	return m, nil
}

func ReadSplineFaces(r io.Reader, l Limits) (SplineFaceModel, error) {
	b, e := ReadBoundedSplineFaces(r, l)
	if e != nil {
		return SplineFaceModel{}, e
	}
	m := SplineFaceModel{LengthUnit: b.LengthUnit}
	for _, face := range b.Faces {
		u0, u1, v0, v1 := face.Geometry.Domain()
		if face.Domain != (geom.ParameterRectangle{UMin: u0, UMax: u1, VMin: v0, VMax: v1}) {
			return SplineFaceModel{}, errors.New("step: bounded trim requires ReadBoundedSplineFaces")
		}
		m.Faces = append(m.Faces, RectangularSplineFace{face.Name, face.Geometry})
	}
	return m, nil
}
