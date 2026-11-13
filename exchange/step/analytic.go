package step

import (
	"errors"
	"io"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
)

// DecodeAnalyticSurface maps an elementary STEP surface without importing its
// face trims or topology. Supported kinds are PLANE, CYLINDRICAL_SURFACE,
// SPHERICAL_SURFACE, CONICAL_SURFACE and ring TOROIDAL_SURFACE. Units remain raw coordinates.
func DecodeAnalyticSurface(f *File, id int64) (geom.Surface, error) {
	if f == nil {
		return nil, errors.New("step: nil file")
	}
	ent, ok := f.Entities[id]
	if !ok || len(ent.Records) != 1 {
		return nil, errors.New("step: expected simple analytic surface")
	}
	r := ent.Records[0]
	n := 0
	switch r.Type {
	case "PLANE":
		n = 2
	case "CYLINDRICAL_SURFACE", "SPHERICAL_SURFACE":
		n = 3
	case "TOROIDAL_SURFACE", "CONICAL_SURFACE":
		n = 4
	default:
		return nil, errors.New("step: unsupported analytic surface")
	}
	if len(r.Args) != n {
		return nil, errors.New("step: wrong analytic attributes")
	}
	if _, e := textValue(r.Args[0]); e != nil {
		return nil, e
	}
	pid, e := ref(r.Args[1])
	if e != nil {
		return nil, e
	}
	frame, e := placement(f, pid)
	if e != nil {
		return nil, e
	}
	a, b := 0.0, 0.0
	if n >= 3 {
		a, e = numeric(r.Args[2])
		if e != nil {
			return nil, e
		}
	}
	if n == 4 {
		b, e = numeric(r.Args[3])
		if e != nil {
			return nil, e
		}
	}
	switch r.Type {
	case "PLANE":
		return geom.NewPlaneSurface(frame)
	case "CYLINDRICAL_SURFACE":
		return geom.NewCylinderSurface(frame, a)
	case "SPHERICAL_SURFACE":
		return geom.NewSphereSurface(frame, a)
	case "CONICAL_SURFACE":
		if a < 0 || b <= 0 || b >= math.Pi/2 {
			return nil, errors.New("step: invalid conical surface")
		}
		return ConicalSurface{frame, a, b}, nil
	default:
		return geom.NewTorusSurface(frame, a, b)
	}
}

// AnalyticBody is a validated exact curved solid. Import supports complete spheres with
// minimal pole topology, ring tori with full double-seam topology, and finite
// cylinders/conical frusta with circular rims and plane caps, including cone apices,
// and full concentric tubes with annular plane caps.
type AnalyticBody struct {
	Name  string
	Solid parametric.Solid
}
type AnalyticModel struct {
	Bodies     []AnalyticBody
	LengthUnit float64
}

func analyticBody(f *File, id int64) (AnalyticBody, error) {
	fail := func() (AnalyticBody, error) {
		return AnalyticBody{}, errors.New("step: only outward complete single-pole spherical solids supported")
	}
	body, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	shellID, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	shell, e := simple(f, shellID, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	if _, e = textValue(shell.Args[0]); e != nil {
		return fail()
	}
	faces, e := listRefs(shell.Args[1], 1)
	if e == nil && len(faces) >= 7 && len(faces)%2 == 1 {
		return axialProfileBody(f, name, faces)
	}
	if e == nil && len(faces) == 5 {
		cylinderCount := 0
		for _, fid := range faces {
			face, err := simple(f, fid, "ADVANCED_FACE", 4)
			if err != nil {
				continue
			}
			sid, err := ref(face.Args[2])
			if err != nil {
				continue
			}
			if _, err := simple(f, sid, "CYLINDRICAL_SURFACE", 3); err == nil {
				cylinderCount++
			}
			if _, err := simple(f, sid, "CONICAL_SURFACE", 4); err == nil {
				cylinderCount++
			}
		}
		if cylinderCount == 1 {
			return sectorBody(f, name, faces)
		}
		return steppedCylinderBody(f, name, faces)
	}
	if e == nil && len(faces) == 4 {
		return tubeBody(f, name, faces)
	}
	if e == nil && (len(faces) == 2 || len(faces) == 3) {
		for _, fid := range faces {
			face, err := simple(f, fid, "ADVANCED_FACE", 4)
			if err != nil {
				continue
			}
			sid, err := ref(face.Args[2])
			if err == nil {
				ent := f.Entities[sid]
				if len(ent.Records) == 1 && ent.Records[0].Type == "SPHERICAL_SURFACE" {
					if len(faces) == 3 {
						return sphericalZoneBody(f, name, faces)
					}
					return sphericalCapBody(f, name, faces)
				}
			}
		}
	}
	if e == nil && len(faces) == 3 {
		for _, fid := range faces {
			r, err := simple(f, fid, "ADVANCED_FACE", 4)
			if err != nil {
				continue
			}
			sid, err := ref(r.Args[2])
			if err == nil {
				ent := f.Entities[sid]
				if len(ent.Records) == 1 && ent.Records[0].Type == "TOROIDAL_SURFACE" {
					return torusElbowBody(f, name, faces)
				}
			}
		}
	}
	if e == nil && (len(faces) == 3 || len(faces) == 2) {
		return cylinderBody(f, name, faces)
	}
	if e != nil || len(faces) != 1 {
		return fail()
	}
	face, e := simple(f, faces[0], "ADVANCED_FACE", 4)
	if e != nil {
		return fail()
	}
	if _, e = textValue(face.Args[0]); e != nil {
		return fail()
	}
	sense, e := boolean(face.Args[3])
	if e != nil || !sense {
		return fail()
	}
	sid, e := ref(face.Args[2])
	if e != nil {
		return fail()
	}
	surface, e := DecodeAnalyticSurface(f, sid)
	if e != nil {
		return fail()
	}
	if torus, ok := surface.(geom.TorusSurface); ok {
		return torusBody(f, name, face, sid, torus)
	}
	sphere, ok := surface.(geom.SphereSurface)
	if !ok {
		return fail()
	}
	frame, radius := sphere.Parameters()
	bounds, e := listRefs(face.Args[1], 1)
	if e != nil || len(bounds) != 1 {
		return fail()
	}
	be := f.Entities[bounds[0]]
	if len(be.Records) != 1 || (be.Records[0].Type != "FACE_BOUND" && be.Records[0].Type != "FACE_OUTER_BOUND") {
		return fail()
	}
	bound, e := simple(f, bounds[0], be.Records[0].Type, 3)
	if e != nil {
		return fail()
	}
	if _, e = textValue(bound.Args[0]); e != nil {
		return fail()
	}
	sense, e = boolean(bound.Args[2])
	if e != nil || !sense {
		return fail()
	}
	loopID, e := ref(bound.Args[1])
	if e != nil {
		return fail()
	}
	loop, e := simple(f, loopID, "VERTEX_LOOP", 2)
	if e != nil {
		return fail()
	}
	if _, e = textValue(loop.Args[0]); e != nil {
		return fail()
	}
	vid, e := ref(loop.Args[1])
	if e != nil {
		return fail()
	}
	vertex, e := simple(f, vid, "VERTEX_POINT", 2)
	if e != nil {
		return fail()
	}
	if _, e = textValue(vertex.Args[0]); e != nil {
		return fail()
	}
	pointID, e := ref(vertex.Args[1])
	if e != nil {
		return fail()
	}
	point, e := triple(f, pointID, "CARTESIAN_POINT")
	if e != nil {
		return fail()
	}
	// A sole pole vertex has no trimming arc: it represents the natural sphere.
	// Require either parametrization pole, not an arbitrary point on the sphere.
	local := frame.ToLocal(point).Scale(1 / radius)
	if !local.IsFinite() || math.Hypot(local.X, local.Y) > 1e-10 || math.Abs(math.Abs(local.Z)-1) > 1e-10 {
		return fail()
	}
	solid, e := parametric.Sphere(frame.Origin(), radius)
	if e != nil {
		return AnalyticBody{}, e
	}
	return AnalyticBody{name, solid}, nil
}

// ReadAnalyticBodies reads complete spheres, ring tori, capped cylinders and cones/frusta.
// Spheres and tori require one outward analytic face. Spheres require one
// pole VERTEX_LOOP; tori require canonical circular seams and full-period UV
// pcurves. Cylinders require a cylindrical side, two shared circular rims, a
// longitudinal seam and two correctly oriented plane caps. Other trims, void
// shells and assembly placements are rejected.
// Reconstruction retains the exact spherical locus, canonicalizing its seam.
func ReadAnalyticBodies(r io.Reader, l Limits) (AnalyticModel, error) {
	l, e := l.defaults()
	if e != nil {
		return AnalyticModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return AnalyticModel{}, e
	}
	m := AnalyticModel{}
	used := map[int64]bool{}
	totalVertices, totalFaces, totalUses := 0, 0, 0
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" || rec.Type == "NEXT_ASSEMBLY_USAGE_OCCURRENCE" {
				return AnalyticModel{}, errors.New("step: analytic assembly placement unsupported")
			}
		}
		if len(ent.Records) != 1 {
			for _, r := range ent.Records {
				if strings.HasSuffix(r.Type, "SHAPE_REPRESENTATION") {
					return AnalyticModel{}, errors.New("step: complex analytic representation unsupported")
				}
			}
			continue
		}
		kind := ent.Records[0].Type
		if strings.HasSuffix(kind, "SHAPE_REPRESENTATION") && kind != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			return AnalyticModel{}, errors.New("step: unsupported analytic representation")
		}
		if kind != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			continue
		}
		rep, e := simple(f, id, kind, 3)
		if e != nil {
			return AnalyticModel{}, e
		}
		if _, e = textValue(rep.Args[0]); e != nil {
			return AnalyticModel{}, e
		}
		ctx, e := ref(rep.Args[2])
		if e != nil {
			return AnalyticModel{}, e
		}
		unit, e := contextUnits(f, ctx)
		if e != nil {
			return AnalyticModel{}, e
		}
		if m.LengthUnit != 0 && m.LengthUnit != unit {
			return AnalyticModel{}, errors.New("step: mixed analytic units")
		}
		m.LengthUnit = unit
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return AnalyticModel{}, e
		}
		for _, bid := range items {
			if x := f.Entities[bid]; len(x.Records) == 1 && x.Records[0].Type == "AXIS2_PLACEMENT_3D" {
				if _, e = placement(f, bid); e != nil {
					return AnalyticModel{}, e
				}
				continue
			}
			if used[bid] || len(m.Bodies) >= l.MaxSolids {
				return AnalyticModel{}, errors.New("step: repeated analytic body or resource limit")
			}
			body, e := analyticBody(f, bid)
			if e != nil {
				return AnalyticModel{}, e
			}
			nv, nf := len(body.Solid.Vertices()), len(body.Solid.Faces())
			if nv > l.MaxVertices-totalVertices || nf > l.MaxFaces-totalFaces || nf > (l.MaxEdgeUses-totalUses)/4 {
				return AnalyticModel{}, errors.New("step: analytic topology limit")
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
				return AnalyticModel{}, errors.New("step: unrepresented analytic body")
			}
		}
	}
	if len(m.Bodies) == 0 {
		return AnalyticModel{}, errors.New("step: no supported analytic bodies")
	}
	return m, nil
}
