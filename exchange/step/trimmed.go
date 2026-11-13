package step

import (
	"errors"
	"io"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
	"strings"
)

// TrimmedBody is an exactly reconstructed solid with explicit surface trims.
type TrimmedBody struct {
	Name  string
	Solid trimmed.Solid
}
type TrimmedModel struct {
	Bodies     []TrimmedBody
	LengthUnit float64
}

// CompleteCylinder recovers the full, two-cap cylinder produced by the
// analytic reader. It verifies the trimmed topology against the native
// constructor before returning the descriptor.
func (b TrimmedBody) CompleteCylinder() (CylinderBody, bool) {
	faces := b.Solid.Faces()
	if len(faces) != 3 {
		return CylinderBody{}, false
	}
	side, lateral := faces[0].Surface.(geom.CylinderSurface)
	_, bottom := parametric.PolarPlaneFrame(faces[1].Surface)
	_, top := parametric.PolarPlaneFrame(faces[2].Surface)
	if !lateral || !bottom || !top || len(faces[0].Outer) != 4 {
		return CylinderBody{}, false
	}
	uv := faces[0].Outer
	height := uv[2].UVStart.Y
	if height <= 0 || !finiteNumber(height) {
		return CylinderBody{}, false
	}
	points := [4]geom.Vec2{{}, {X: 2 * math.Pi}, {X: 2 * math.Pi, Y: height}, {Y: height}}
	for i, use := range uv {
		if use.Curve != nil || use.UVStart != points[i] || use.UVEnd != points[(i+1)%4] {
			return CylinderBody{}, false
		}
	}
	frame, radius := side.Parameters()
	cylinder, err := parametric.Cylinder(frame, radius, height)
	if err != nil {
		return CylinderBody{}, false
	}
	want, err := trimmed.FromParametric(cylinder)
	if err != nil || !reflect.DeepEqual(b.Solid.Vertices(), want.Vertices()) ||
		!reflect.DeepEqual(b.Solid.Edges(), want.Edges()) || !reflect.DeepEqual(faces, want.Faces()) {
		return CylinderBody{}, false
	}
	return CylinderBody{Name: b.Name, Frame: frame, Radius: radius, Height: height}, true
}

// ReadTrimmedBodies reads the strict full analytic body graphs supported by
// ReadAnalyticBodies, plus a full cylinder capped by one oblique plane and one
// perpendicular plane. The latter requires direct complete ellipse/circle rims
// and a shared axial seam; p-curves and partial ellipses are unsupported.
// Analytic patches may be subdivided exactly while rebuilding shared topology.
func ReadTrimmedBodies(r io.Reader, l Limits) (TrimmedModel, error) {
	l, e := l.defaults()
	if e != nil {
		return TrimmedModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return TrimmedModel{}, e
	}
	m := TrimmedModel{}
	used := map[int64]bool{}
	totalVertices, totalFaces, totalUses := 0, 0, 0
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" || rec.Type == "NEXT_ASSEMBLY_USAGE_OCCURRENCE" {
				return TrimmedModel{}, errors.New("step: trimmed assembly placement unsupported")
			}
		}
		if len(ent.Records) != 1 {
			for _, r := range ent.Records {
				if strings.HasSuffix(r.Type, "SHAPE_REPRESENTATION") {
					return TrimmedModel{}, errors.New("step: complex trimmed representation unsupported")
				}
			}
			continue
		}
		kind := ent.Records[0].Type
		if strings.HasSuffix(kind, "SHAPE_REPRESENTATION") && kind != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			return TrimmedModel{}, errors.New("step: unsupported trimmed representation")
		}
		if kind != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			continue
		}
		rep, e := simple(f, id, kind, 3)
		if e != nil {
			return TrimmedModel{}, e
		}
		if _, e = textValue(rep.Args[0]); e != nil {
			return TrimmedModel{}, e
		}
		ctx, e := ref(rep.Args[2])
		if e != nil {
			return TrimmedModel{}, e
		}
		unit, e := contextUnits(f, ctx)
		if e != nil {
			return TrimmedModel{}, e
		}
		if m.LengthUnit != 0 && m.LengthUnit != unit {
			return TrimmedModel{}, errors.New("step: mixed trimmed units")
		}
		m.LengthUnit = unit
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return TrimmedModel{}, e
		}
		for _, bid := range items {
			if x := f.Entities[bid]; len(x.Records) == 1 && x.Records[0].Type == "AXIS2_PLACEMENT_3D" {
				if _, e = placement(f, bid); e != nil {
					return TrimmedModel{}, e
				}
				continue
			}
			if used[bid] || len(m.Bodies) >= l.MaxSolids {
				return TrimmedModel{}, errors.New("step: repeated trimmed body or resource limit")
			}
			body, e := trimmedBody(f, bid)
			if e != nil {
				return TrimmedModel{}, e
			}
			nv, nf := len(body.Solid.Vertices()), len(body.Solid.Faces())
			if nv > l.MaxVertices-totalVertices || nf > l.MaxFaces-totalFaces || nf > (l.MaxEdgeUses-totalUses)/4 {
				return TrimmedModel{}, errors.New("step: trimmed topology limit")
			}
			totalVertices += nv
			totalFaces += nf
			totalUses += nf * 4
			used[bid] = true
			m.Bodies = append(m.Bodies, body)
		}
	}
	for id, ent := range f.Entities {
		for _, r := range ent.Records {
			if (strings.HasSuffix(r.Type, "_BREP") || r.Type == "BREP_WITH_VOIDS") && !used[id] {
				return TrimmedModel{}, errors.New("step: unrepresented trimmed body")
			}
		}
	}
	if len(m.Bodies) == 0 {
		return TrimmedModel{}, errors.New("step: no supported trimmed bodies")
	}
	return m, nil
}

func trimmedBody(f *File, id int64) (TrimmedBody, error) {
	if a, e := analyticBody(f, id); e == nil {
		s, e := trimmed.FromParametric(a.Solid)
		return TrimmedBody{a.Name, s}, e
	}
	return obliqueCylinderBody(f, id)
}
