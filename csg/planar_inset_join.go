package csg

import (
	"context"
	"fmt"

	"plainsolid/brep"
	"plainsolid/collision"
)

// PlanarInsetJoinHistory maps the retained original entities. The original
// host face remains in its slot with a new inner wire; the complete original
// tool face is deleted and its wire becomes that hole.
type PlanarInsetJoinHistory struct {
	Owners  [2]PlanarFaceJoinOwner
	Contact collision.PlanarInsetContact
}

// JoinPlanarInset attaches a planar tool whose entire original contact face
// meets the strict interior of a larger host face. The host exterior face
// retains its original outer boundary and any existing holes.
func JoinPlanarInset(ctx context.Context, host, tool brep.Solid, hostFace, toolFace int, o collision.PlanarFaceContactOptions) (brep.Solid, PlanarInsetJoinHistory, error) {
	s, h, err := JoinPlanarInsets(ctx, host, tool, hostFace, []int{toolFace}, o)
	if err != nil {
		return brep.Solid{}, PlanarInsetJoinHistory{}, err
	}
	return s, PlanarInsetJoinHistory{Owners: h.Owners, Contact: h.Contact.Tools[0]}, nil
}

// PlanarInsetSetJoinHistory retains every original tool disk and maps the
// corresponding whole boundary to a separate hole in the original host face.
type PlanarInsetSetJoinHistory struct {
	Owners  [2]PlanarFaceJoinOwner
	Contact collision.PlanarInsetContactSet
}

// JoinPlanarInsets attaches a single tool across several disjoint complete
// tool faces contained strictly in the same host face.
func JoinPlanarInsets(ctx context.Context, host, tool brep.Solid, hostFace int, toolFaces []int, o collision.PlanarFaceContactOptions) (brep.Solid, PlanarInsetSetJoinHistory, error) {
	var zero PlanarInsetSetJoinHistory
	p, err := collision.ValidatePlanarInsetContactSet(ctx, host, tool, hostFace, toolFaces, o)
	if err != nil {
		return brep.Solid{}, zero, err
	}
	return joinProvedPlanarInsets(ctx, host, tool, p, o.Validation)
}

func joinProvedPlanarInsets(ctx context.Context, host, tool brep.Solid, p collision.PlanarInsetContactSet, o collision.LocationOptions) (brep.Solid, PlanarInsetSetJoinHistory, error) {
	var zero PlanarInsetSetJoinHistory
	h := PlanarInsetSetJoinHistory{Contact: p}
	hv, he, hf := host.Vertices(), host.Edges(), host.Faces()
	tv, te, tf := tool.Vertices(), tool.Edges(), tool.Faces()
	vo, eo := len(hv), len(he)
	v := append(hv, tv...)
	e := he
	for _, q := range te {
		if err := ctx.Err(); err != nil {
			return brep.Solid{}, zero, err
		}
		e = append(e, brep.Edge{Start: q.Start + vo, End: q.End + vo})
	}
	mapWire := func(w []brep.EdgeUse) {
		for i := range w {
			w[i].Edge += eo
		}
	}
	a, b := &h.Owners[0], &h.Owners[1]
	a.Vertices, a.Edges, a.Faces = make([]int, vo), make([]brep.EdgeUse, eo), make([]int, len(hf))
	b.Vertices, b.Edges, b.Faces = make([]int, len(tv)), make([]brep.EdgeUse, len(te)), make([]int, len(tf))
	for i := range a.Vertices {
		a.Vertices[i] = i
	}
	for i := range a.Edges {
		a.Edges[i].Edge = i
	}
	for i := range a.Faces {
		a.Faces[i] = i
	}
	for i := range b.Vertices {
		b.Vertices[i] = vo + i
	}
	for i := range b.Edges {
		b.Edges[i].Edge = eo + i
	}
	f := hf
	removed := make([]bool, len(tf))
	for _, q := range p.Tools {
		removed[q.ToolFace] = true
	}
	for i, face := range tf {
		if err := ctx.Err(); err != nil {
			return brep.Solid{}, zero, err
		}
		mapWire(face.Boundary)
		if removed[i] {
			b.Faces[i] = -1
			f[p.HostFace].Holes = append(f[p.HostFace].Holes, face.Boundary)
			continue
		}
		for _, w := range face.Holes {
			mapWire(w)
		}
		b.Faces[i] = len(f)
		f = append(f, face)
	}
	out, err := brep.NewSolid(v, e, f)
	if err != nil {
		return brep.Solid{}, zero, fmt.Errorf("csg: cannot add inset wire to original host face: %w", err)
	}
	loc, err := collision.NewPointLocator(ctx, out, o)
	if err != nil {
		return brep.Solid{}, zero, err
	}
	shells := loc.Shells()
	if len(shells) != 1 || shells[0].SignedVolume.Cmp(p.SignedVolume) != 0 {
		return brep.Solid{}, zero, fmt.Errorf("%w: inset boundary does not retain the complete combined material", collision.ErrInvalid)
	}
	return out, h, nil
}
