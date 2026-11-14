package trimmed

import (
	"fmt"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BoreFiniteCylinders removes independent through or perpendicular entry-open
// blind tools after proving separation of their retained finite padded bounds.
// Depth zero selects through. Positive depths are measured from the entry cap.
// Floors are -1 for through tools; source indices and wall histories stay stable.
func BoreFiniteCylinders(s brep.Solid, cuts []CylinderCut, depths []float64, tolerance float64) (Solid, []BoreHistory, []int, error) {
	if len(depths) != len(cuts) || len(cuts) == 0 || len(cuts) > 32 || !finite(tolerance) || tolerance <= 0 {
		return Solid{}, nil, nil, geom.ErrInvalid
	}
	r, e := FromPlanar(s, tolerance)
	if e != nil {
		return Solid{}, nil, nil, e
	}
	v, edges, faces := r.Vertices(), r.Edges(), r.Faces()
	nv, ne, nf := len(v), len(edges), len(faces)
	translate := func(a []Use, remap func(int) int) []Use {
		b := append([]Use(nil), a...)
		for i := range b {
			b[i].Edge = remap(b[i].Edge)
		}
		return b
	}
	var bounds []geom.Box
	var pending []Face
	floors := make([]int, len(cuts))
	for i := range floors {
		floors[i] = -1
	}
	history := make([]BoreHistory, 0, len(cuts))
	for i, c := range cuts {
		t, h, e := BoreCylinderAlong(s, c.Entry, c.Exit, c.AxisPoint, c.Direction, c.Radius, tolerance)
		if e != nil {
			return Solid{}, nil, nil, fmt.Errorf("trimmed: tool %d: %w", i, e)
		}
		blind := depths[i] != 0
		if !finite(depths[i]) || depths[i] < 0 {
			return Solid{}, nil, nil, geom.ErrInvalid
		}
		var floor Face
		if blind {
			faces := t.Faces()
			for _, fid := range h.WallFaces {
				for j := range faces[fid].Outer {
					u := &faces[fid].Outer[j]
					if u.Curve == nil {
						continue
					}
					c, ok := u.Curve.(EllipseUV)
					if !ok || c.Cos != (geom.Vec2{}) || c.Sin != (geom.Vec2{}) || c.Linear != (geom.Vec2{X: 1}) {
						return Solid{}, nil, nil, geom.ErrDomain
					}
					u.Curve = nil
				}
			}
			t, e = NewSolid(t.Vertices(), t.Edges(), faces, tolerance)
			if e != nil {
				return Solid{}, nil, nil, e
			}
			span := t.edges[h.WallSeamEdges[0]].Range
			delta := span.Max - span.Min - depths[i]
			var floorID int
			t, floorID, e = StopPerpendicularBore(t, h, c.Direction, delta)
			if e != nil {
				return Solid{}, nil, nil, e
			}
			floor = t.faces[floorID]
		}
		var box geom.Box
		for j, id := range h.WallFaces {
			f := t.faces[id]
			d, e := faceUVBounds(f)
			if e != nil {
				return Solid{}, nil, nil, e
			}
			b, e := parametric.SurfaceBounds(f.Surface, parametric.Interval{Min: d.UMin, Max: d.UMax}, parametric.Interval{Min: d.VMin, Max: d.VMax})
			if e != nil {
				return Solid{}, nil, nil, e
			}
			if j == 0 {
				box = b
			} else {
				box, e = box.Union(b)
				if e != nil {
					return Solid{}, nil, nil, e
				}
			}
		}
		pad := func(lo, hi float64) (float64, float64) {
			m := tolerance + 128*0x1p-52*max(math.Abs(lo), math.Abs(hi))
			return math.Nextafter(lo-m, math.Inf(-1)), math.Nextafter(hi+m, math.Inf(1))
		}
		box.Min.X, box.Max.X = pad(box.Min.X, box.Max.X)
		box.Min.Y, box.Max.Y = pad(box.Min.Y, box.Max.Y)
		box.Min.Z, box.Max.Z = pad(box.Min.Z, box.Max.Z)
		if !box.Valid() {
			return Solid{}, nil, nil, geom.ErrInvalid
		}
		for j, b := range bounds {
			if _, ok := b.Intersect(box); ok {
				return Solid{}, nil, nil, fmt.Errorf("trimmed: tools %d and %d do not have separated bounds", j, i)
			}
		}
		bounds = append(bounds, box)
		edgeOffset, vertexOffset, faceOffset := len(edges)-ne, len(v)-nv, len(faces)-nf
		remap := func(id int) int {
			if id >= ne {
				return id + edgeOffset
			}
			return id
		}
		v = append(v, t.vertices[nv:]...)
		for _, x := range t.edges[ne:] {
			x.Start += vertexOffset
			x.End += vertexOffset
			edges = append(edges, x)
		}
		caps := []int{h.Entry}
		if !blind {
			caps = append(caps, h.Exit)
		}
		for _, f := range caps {
			added := t.faces[f].Holes[len(t.faces[f].Holes)-1]
			faces[f].Holes = append(faces[f].Holes, translate(added, remap))
		}
		for j := range 4 {
			f := cloneFace(t.faces[h.WallFaces[j]])
			f.Outer = translate(f.Outer, remap)
			faces = append(faces, f)
			h.WallFaces[j] += faceOffset
			h.EntryEdges[j] = remap(h.EntryEdges[j])
			h.ExitEdges[j] = remap(h.ExitEdges[j])
			h.WallSeamEdges[j] = remap(h.WallSeamEdges[j])
		}
		if blind {
			floor.Outer = translate(floor.Outer, remap)
			floors[i] = len(pending)
			pending = append(pending, floor)
		}
		history = append(history, h)
	}
	for i := range floors {
		if floors[i] >= 0 {
			floors[i] += len(faces)
		}
	}
	faces = append(faces, pending...)
	out, e := NewSolid(v, edges, faces, tolerance)
	if e != nil {
		return Solid{}, nil, nil, e
	}
	return out, history, floors, nil
}
