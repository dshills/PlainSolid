package plaincadbridge

import (
	"context"
	"errors"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type displayEdges struct {
	Positions []float64 `json:"positions"`
	EdgeUses  int       `json:"edgeUses"`
}
type displayUse struct {
	face   int
	normal func(float64) (geom.Vec3, geom.Vec3, error)
}

// nativeDisplayEdges derives display-only lines from stored native topology.
// Tangent suppression uses the same three interior normal samples as PlainCAD's
// OCCT adapter; it is not a geometric sewing or exact continuity certificate.
func nativeDisplayEdges(s stored, maxSegments int, remaining *int) (displayEdges, error) {
	out := displayEdges{Positions: []float64{}}
	if maxSegments < 0 || maxSegments > 60000 || remaining != nil && (*remaining < 0 || *remaining > 40000) {
		return out, errors.New("invalid native display budget")
	}
	if p := s.rationalStepArc; p != nil {
		local, err := nativeDisplayEdges(stored{analytic: &analyticBody{solid: p.local}}, maxSegments, remaining)
		if err != nil {
			return out, err
		}
		if err = p.roundDisplayEdges(context.Background(), local.Positions); err != nil {
			return out, err
		}
		return local, nil
	}
	if s.tangent != nil {
		maxUses := 12000
		if remaining != nil && *remaining < maxUses {
			maxUses = *remaining
		}
		r, e := s.tangent.displayEdges(maxUses, maxSegments)
		return displayEdges{Positions: r.Segments, EdgeUses: r.EdgeUses}, e
	}
	var vertices []geom.Vec3
	var edges []parametric.Edge
	var uses [][]displayUse
	transform := geom.Identity()
	add := func(edge int, use displayUse) error {
		out.EdgeUses++
		if out.EdgeUses > 12000 {
			return errors.New("Native edge traversal exceeds the display budget.")
		}
		if remaining != nil && out.EdgeUses > *remaining {
			return errors.New("Project exceeds the native edge traversal budget.")
		}
		if edge < 0 || edge >= len(uses) {
			return errors.New("invalid native edge identity")
		}
		uses[edge] = append(uses[edge], use)
		return nil
	}
	trimmedBody := func(body trimmed.Solid) error {
		vertices, edges = body.Vertices(), body.Edges()
		uses = make([][]displayUse, len(edges))
		for fi, f := range body.Faces() {
			for _, loop := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
				for _, u := range loop {
					face, use := f, u
					normal := func(t float64) (geom.Vec3, geom.Vec3, error) {
						if use.Reversed {
							t = 1 - t
						}
						uv := geom.Vec2{X: use.UVStart.X + (use.UVEnd.X-use.UVStart.X)*t, Y: use.UVStart.Y + (use.UVEnd.Y-use.UVStart.Y)*t}
						var e error
						if use.Curve != nil {
							uv, e = use.Curve.EvalUV(t)
							if e != nil {
								return geom.Vec3{}, geom.Vec3{}, e
							}
						}
						p, e := face.Surface.Eval(uv.X, uv.Y)
						if e != nil {
							return p, geom.Vec3{}, e
						}
						du, dv, e := face.Surface.Partials(uv.X, uv.Y)
						n := du.Cross(dv)
						if face.Reversed {
							n = n.Scale(-1)
						}
						return p, n, e
					}
					if e := add(u.Edge, displayUse{fi, normal}); e != nil {
						return e
					}
				}
			}
		}
		return nil
	}
	var e error
	switch {
	case s.cylinder != nil:
		body := s.cylinder.solid
		vertices, edges = body.Vertices(), body.Edges()
		uses = make([][]displayUse, len(edges))
		for fi, f := range body.Faces() {
			for side, u := range f.Boundary {
				face, use, k := f, u, side
				normal := func(t float64) (geom.Vec3, geom.Vec3, error) {
					if use.Reversed {
						t = 1 - t
					}
					a, b := face.U.Min, face.V.Min
					switch k {
					case 0:
						a += t * (face.U.Max - face.U.Min)
					case 1:
						a = face.U.Max
						b += t * (face.V.Max - face.V.Min)
					case 2:
						a = face.U.Max - t*(face.U.Max-face.U.Min)
						b = face.V.Max
					case 3:
						b = face.V.Max - t*(face.V.Max-face.V.Min)
					}
					p, e := face.Surface.Eval(a, b)
					if e != nil {
						return p, geom.Vec3{}, e
					}
					du, dv, e := face.Surface.Partials(a, b)
					n := du.Cross(dv)
					if face.Reversed {
						n = n.Scale(-1)
					}
					return p, n, e
				}
				if e = add(u.Edge, displayUse{fi, normal}); e != nil {
					return displayEdges{}, e
				}
			}
		}
	case s.analytic != nil:
		e = trimmedBody(s.analytic.solid)
	case s.bore != nil:
		e = trimmedBody(s.bore.solid)
	case s.treated != nil:
		transform = frameMap(s.treated.frame)
		e = trimmedBody(s.treated.chain.Solid)
	default:
		vertices = s.solid.Vertices()
		for _, edge := range s.solid.Edges() {
			edges = append(edges, parametric.Edge{Start: edge.Start, End: edge.End})
		}
		uses = make([][]displayUse, len(edges))
		for fi, f := range s.solid.Faces() {
			var points []geom.Vec3
			for _, u := range f.Boundary {
				edge := edges[u.Edge]
				idx := edge.Start
				if u.Reversed {
					idx = edge.End
				}
				points = append(points, vertices[idx])
			}
			n := geom.Vec3{}
			for i := 1; i+1 < len(points); i++ {
				n = n.Add(points[i].Sub(points[0]).Cross(points[i+1].Sub(points[0])))
			}
			for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
				for _, u := range loop {
					edge := edges[u.Edge]
					a, b, normalVector := vertices[edge.Start], vertices[edge.End], n
					normal := func(t float64) (geom.Vec3, geom.Vec3, error) {
						return a.Scale(1 - t).Add(b.Scale(t)), normalVector, nil
					}
					if e = add(u.Edge, displayUse{fi, normal}); e != nil {
						return displayEdges{}, e
					}
				}
			}
		}
	}
	if e != nil {
		return displayEdges{}, e
	}
	smooth := func(a, b displayUse) bool {
		for _, t := range []float64{.25, .5, .75} {
			pa, na, ea := a.normal(t)
			pb, nb, eb := b.normal(t)
			if ea != nil || eb != nil || pa.Sub(pb).Length() > 1e-6 || na.Length() <= 1e-12 || nb.Length() <= 1e-12 {
				return false
			}
			if na.Dot(nb)/(na.Length()*nb.Length()) <= .99999 {
				return false
			}
		}
		return true
	}
	for i, edge := range edges {
		if edge.Curve == nil && edge.Start == edge.End {
			continue
		}
		us := uses[i]
		seam := false
		for j := range us {
			for k := 0; k < j; k++ {
				if us[j].face == us[k].face {
					seam = true
				}
			}
		}
		if seam || len(us) == 2 && smooth(us[0], us[1]) {
			continue
		}
		divisions := 1
		if edge.Curve != nil {
			if _, ok := edge.Curve.(geom.Line); !ok {
				divisions = 64
			}
		}
		eval := func(t float64) (geom.Vec3, error) {
			if edge.Curve == nil {
				return vertices[edge.Start].Scale(1 - t).Add(vertices[edge.End].Scale(t)), nil
			}
			return edge.Curve.Eval(edge.Range.Min + t*(edge.Range.Max-edge.Range.Min))
		}
		a, e := eval(0)
		if e != nil {
			return displayEdges{}, e
		}
		a = transform.Apply(a)
		for j := 1; j <= divisions; j++ {
			b, e := eval(float64(j) / float64(divisions))
			if e != nil {
				return displayEdges{}, e
			}
			b = transform.Apply(b)
			if !a.IsFinite() || !b.IsFinite() {
				return displayEdges{}, errors.New("A native edge has nonfinite coordinates.")
			}
			if a.Sub(b).Length() > 1e-9 {
				if len(out.Positions)/6 >= maxSegments {
					return displayEdges{}, errors.New("Native edges exceed the display segment budget.")
				}
				out.Positions = append(out.Positions, a.X, a.Y, a.Z, b.X, b.Y, b.Z)
			}
			a = b
		}
	}
	return out, nil
}
