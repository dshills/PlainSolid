package step

import (
	"context"
	"errors"
	"io"
	"plainsolid/brep"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// AxisAlignedSplineShellBody retains an exact planar solid and its source
// spline faces. Only axis-aligned affine bilinear supports are accepted; the
// planar kernel separately certifies global embedding and outward orientation.
type AxisAlignedSplineShellBody struct {
	Name      string
	Faces     []OrthogonalSplineFace
	FaceHoles [][][]geom.Vec2
	FaceIDs   []int64
	Uses      []OrthogonalShellUse
	Solid     brep.Solid
}
type AxisAlignedSplineShellModel struct {
	Bodies     []AxisAlignedSplineShellBody
	LengthUnit float64
}

func axisAlignedAffineSupport(s *geom.BSplineSurface) bool {
	if s == nil {
		return false
	}
	d := s.Data()
	if d.DegreeU != 1 || d.DegreeV != 1 || d.CountU != 2 || d.CountV != 2 || len(d.KnotsU) != 4 || len(d.KnotsV) != 4 || d.KnotsU[0] != d.KnotsU[1] || d.KnotsU[2] != d.KnotsU[3] || d.KnotsV[0] != d.KnotsV[1] || d.KnotsV[2] != d.KnotsV[3] {
		return false
	}
	for _, w := range d.Weights {
		if w != d.Weights[0] {
			return false
		}
	}
	coord := func(p geom.Vec3) [3]float64 { return [3]float64{p.X, p.Y, p.Z} }
	a, b, c, e := coord(d.Poles[0]), coord(d.Poles[1]), coord(d.Poles[2]), coord(d.Poles[3])
	u, v := -1, -1
	for k := 0; k < 3; k++ {
		if c[k] != a[k] {
			if u != -1 {
				return false
			}
			u = k
		}
		if b[k] != a[k] {
			if v != -1 {
				return false
			}
			v = k
		}
	}
	if u < 0 || v < 0 || u == v {
		return false
	}
	for k := 0; k < 3; k++ {
		want := a[k]
		if k == u {
			want = c[k]
		} else if k == v {
			want = b[k]
		}
		if e[k] != want {
			return false
		}
	}
	return true
}

func axisAlignedShellBoundary(ctx context.Context, f *File, s OrthogonalSplineOpenShell) (brep.Solid, error) {
	return affineSplineShellBoundary(ctx, f, s, supportAxis)
}

func affineSplineShellBoundary(ctx context.Context, f *File, s OrthogonalSplineOpenShell, mode affineSupportMode) (brep.Solid, error) {
	fail := func() (brep.Solid, error) {
		return brep.Solid{}, errors.New("step: nonaffine or invalid planar spline shell")
	}
	if mode != supportAxis {
		if !affineSplineSupports(s.Faces, mode == supportOrthogonal) {
			return fail()
		}
	} else {
		for _, face := range s.Faces {
			if !axisAlignedAffineSupport(face.Geometry) {
				return fail()
			}
		}
	}

	vertexMap := map[int64]int{}
	edgeMap := map[int64]int{}
	vertices := []geom.Vec3{}
	edges := []brep.Edge{}
	faces := make([]brep.Face, len(s.Faces))
	for i := range faces {
		faces[i].Holes = make([][]brep.EdgeUse, len(s.FaceHoles[i]))
	}
	for _, u := range s.Uses {
		pair := [2]int{}
		for i, id := range []int64{u.StartVertexID, u.EndVertexID} {
			index, ok := vertexMap[id]
			if !ok {
				p, e := analyticVertex(f, id)
				if e != nil {
					return fail()
				}
				index = len(vertices)
				vertexMap[id] = index
				vertices = append(vertices, p)
			}
			pair[i] = index
		}
		ei, ok := edgeMap[u.EdgeID]
		if !ok {
			ei = len(edges)
			edgeMap[u.EdgeID] = ei
			edges = append(edges, brep.Edge{Start: pair[0], End: pair[1]})
		}
		use := brep.EdgeUse{Edge: ei, Reversed: !u.Forward}
		if u.Loop == 0 {
			faces[u.Face].Boundary = append(faces[u.Face].Boundary, use)
		} else {
			faces[u.Face].Holes[u.Loop-1] = append(faces[u.Face].Holes[u.Loop-1], use)
		}
	}
	solid, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return brep.Solid{}, e
	}
	if e = poly.ValidateEmbeddedBoundary(ctx, solid, poly.Options{RequireExactConversion: true}); e != nil {
		return brep.Solid{}, e
	}
	return solid, nil
}

// ReadAxisAlignedSplineShells accepts closed MANIFOLD_SOLID_BREP graphs only
// after exact source curve agreement, closed manifold incidence, affine support
// proof and global planar embedding/orientation validation. No curved surface
// is flattened and no sampled acceptance or tolerance weld is performed.
func ReadAxisAlignedSplineShells(r io.Reader, l Limits) (AxisAlignedSplineShellModel, error) {
	return ReadAxisAlignedSplineShellsContext(context.Background(), r, l)
}
func ReadAxisAlignedSplineShellsContext(ctx context.Context, r io.Reader, l Limits) (AxisAlignedSplineShellModel, error) {
	return readAffineSplineShellsContext(ctx, r, l, supportAxis)
}

func readAffineSplineShellsContext(ctx context.Context, r io.Reader, l Limits, mode affineSupportMode) (AxisAlignedSplineShellModel, error) {
	fail := func() (AxisAlignedSplineShellModel, error) {
		return AxisAlignedSplineShellModel{}, errors.New("step: unsupported affine spline solid graph")
	}
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return AxisAlignedSplineShellModel{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return AxisAlignedSplineShellModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return AxisAlignedSplineShellModel{}, e
	}
	view := *f
	view.Entities = make(map[int64]Entity, len(f.Entities))
	for id, ent := range f.Entities {
		view.Entities[id] = ent
	}
	owned := map[int64]bool{}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) != 1 {
			continue
		}
		rec := ent.Records[0]
		switch rec.Type {
		case "OPEN_SHELL", "BREP_WITH_VOIDS", "FACETED_BREP", "SHELL_BASED_SURFACE_MODEL", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION":
			return fail()
		case "ADVANCED_BREP_SHAPE_REPRESENTATION":
			rec.Type = "MANIFOLD_SURFACE_SHAPE_REPRESENTATION"
			view.Entities[id] = Entity{ID: id, Records: []Record{rec}}
		case "MANIFOLD_SOLID_BREP":
			if len(rec.Args) != 2 {
				return fail()
			}
			sid, e := ref(rec.Args[1])
			if e != nil || owned[sid] {
				return fail()
			}
			owned[sid] = true
			shell, e := simple(f, sid, "CLOSED_SHELL", 2)
			if e != nil {
				return fail()
			}
			shell.Type = "OPEN_SHELL"
			shell.Args = append([]Value(nil), shell.Args...)
			shell.Args[0] = rec.Args[0]
			view.Entities[sid] = Entity{ID: sid, Records: []Record{shell}}
			rec.Type = "SHELL_BASED_SURFACE_MODEL"
			rec.Args = append([]Value(nil), rec.Args...)
			rec.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: sid}}}
			view.Entities[id] = Entity{ID: id, Records: []Record{rec}}
		}
	}
	if len(owned) == 0 {
		return fail()
	}
	for _, id := range f.Order {
		if _, e := simple(f, id, "CLOSED_SHELL", 2); e == nil && !owned[id] {
			return fail()
		}
	}
	if mode != supportAxis {
		if e := normalizeAffineIsoPCurves(&view, l.MaxVertices); e != nil {
			return AxisAlignedSplineShellModel{}, e
		}
	}
	model, e := readOrthogonalSplineShellFile(&view, l, true)
	if e != nil {
		return AxisAlignedSplineShellModel{}, e
	}
	out := AxisAlignedSplineShellModel{LengthUnit: model.LengthUnit}
	for _, s := range model.Shells {
		solid, e := affineSplineShellBoundary(ctx, f, s, mode)
		if e != nil {
			return AxisAlignedSplineShellModel{}, e
		}
		out.Bodies = append(out.Bodies, AxisAlignedSplineShellBody{Name: s.Name, Faces: s.Faces, FaceHoles: s.FaceHoles, FaceIDs: s.FaceIDs, Uses: s.Uses, Solid: solid})
	}
	return out, nil
}

// WriteAxisAlignedSplineShells retains the exact source supports, loops and
// explicit topology as CLOSED_SHELL / MANIFOLD_SOLID_BREP. Faces, FaceHoles,
// FaceIDs and Uses are authoritative; Solid is derived by the reader and is
// not serialized. The full staged output must pass exact closed-solid validation
// before any bytes are emitted. Numeric entity IDs may be regenerated.
func WriteAxisAlignedSplineShells(w io.Writer, m AxisAlignedSplineShellModel) error {
	graph := OrthogonalSplineOpenShellModel{LengthUnit: m.LengthUnit}
	for _, body := range m.Bodies {
		graph.Shells = append(graph.Shells, OrthogonalSplineOpenShell{Name: body.Name, Faces: body.Faces, FaceHoles: body.FaceHoles, FaceIDs: body.FaceIDs, Uses: body.Uses})
	}
	return writeOrthogonalSplineShells(w, graph, true, supportAxis)
}
