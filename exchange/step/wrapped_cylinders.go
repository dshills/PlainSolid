package step

import (
	"context"
	"errors"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type WrappedCylindricalBody struct {
	Name      string
	CapNames  [2]string // Lower and upper planar face names.
	Face      WrappedSplineFace
	Solid     trimmed.Solid
	Embedding geom.WrappedCylinderEmbedding
}
type WrappedCylindricalModel struct {
	Bodies     []WrappedCylindricalBody
	LengthUnit float64
}

// ReadWrappedCylindricalBodies requires a three-face closed shell: the exact
// wrapped wall and two planar caps carrying the identical rational rim curves.
// Native topology may split each source closed rim into quarter fragments.
func ReadWrappedCylindricalBodies(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedCylindricalModel, error) {
	m, e := readWrappedCappedBodies(r, l, o, false)
	if e != nil {
		return WrappedCylindricalModel{}, e
	}
	out := WrappedCylindricalModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		out.Bodies = append(out.Bodies, WrappedCylindricalBody{Name: b.Name, CapNames: b.CapNames, Face: b.Face, Solid: b.Solid, Embedding: b.CylinderProof})
	}
	return out, nil
}

type wrappedCappedBody struct {
	Name          string
	CapNames      [2]string
	Face          WrappedSplineFace
	Solid         trimmed.Solid
	CylinderProof geom.WrappedCylinderEmbedding
	FrustumProof  geom.WrappedFrustumEmbedding
}
type wrappedCappedModel struct {
	Bodies     []wrappedCappedBody
	LengthUnit float64
}

func readWrappedCappedBodies(r io.Reader, l Limits, o WrappedSurfaceOptions, frustum bool) (wrappedCappedModel, error) {
	fail := func() (wrappedCappedModel, error) {
		return wrappedCappedModel{}, errors.New("step: unsupported capped wrapped spline body")
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return wrappedCappedModel{}, e
	}
	o, e = o.defaults()
	if e != nil {
		return wrappedCappedModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return wrappedCappedModel{}, e
	}
	count := 0
	for _, ent := range f.Entities {
		for _, rec := range ent.Records {
			if rec.Type == "MANIFOLD_SOLID_BREP" {
				count++
			}
		}
	}
	if count < 1 || count > o.MaxSurfaces || count > l.MaxSolids || count > l.MaxFaces/3 || count > l.MaxEdgeUses/6 {
		return fail()
	}
	o.MaxOperations /= count * 12
	if o.MaxOperations < 1 {
		return wrappedCappedModel{}, geom.ErrConvergence
	}
	view := *f
	view.Entities = map[int64]Entity{}
	for id, ent := range f.Entities {
		view.Entities[id] = ent
	}
	names := map[int64]string{}
	capNames := map[int64][2]string{}
	caps := map[int64]bool{}
	shells := map[int64]bool{}
	for _, id := range f.Order {
		ent := f.Entities[id]
		for _, rec := range ent.Records {
			switch rec.Type {
			case "OPEN_SHELL", "SHELL_BASED_SURFACE_MODEL", "BREP_WITH_VOIDS", "FACETED_BREP", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION":
				return fail()
			}
		}
		if len(ent.Records) != 1 {
			continue
		}
		rec := ent.Records[0]
		if rec.Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			rec.Type = "MANIFOLD_SURFACE_SHAPE_REPRESENTATION"
			view.Entities[id] = Entity{ID: id, Records: []Record{rec}}
		}
		if rec.Type != "MANIFOLD_SOLID_BREP" {
			continue
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
		if e != nil || shells[shellID] {
			return fail()
		}
		shell, e := simple(f, shellID, "CLOSED_SHELL", 2)
		if e != nil {
			return fail()
		}
		fs, e := listRefs(shell.Args[1], 3)
		if e != nil || len(fs) != 3 {
			return fail()
		}
		wall := int64(0)
		capIDs := []int64{}
		for _, fid := range fs {
			face, e := simple(f, fid, "ADVANCED_FACE", 4)
			if e != nil {
				return fail()
			}
			sid, e := ref(face.Args[2])
			if e != nil {
				return fail()
			}
			if _, ok := component(f.Entities[sid], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
				if wall != 0 {
					return fail()
				}
				wall = fid
			} else {
				capIDs = append(capIDs, fid)
			}
		}
		if wall == 0 || len(capIDs) != 2 {
			return fail()
		}
		if _, ok := names[wall]; ok {
			return fail()
		}
		face, uses, e := analyticFaceLoop(f, wall, 4)
		if e != nil {
			return fail()
		}
		sid, _ := ref(face.Args[2])
		rimCaps := map[int64]int64{}
		for _, cid := range capIDs {
			edge, e := wrappedCylinderCap(f, cid, o)
			if e != nil {
				return wrappedCappedModel{}, e
			}
			if rimCaps[edge] != 0 || caps[cid] {
				return fail()
			}
			rimCaps[edge] = cid
			caps[cid] = true
			delete(view.Entities, cid)
		}
		for _, use := range uses {
			_, _, g, e := analyticEdge(f, use.id)
			if e != nil {
				return fail()
			}
			sc, e := simple(f, g, "SURFACE_CURVE", 4)
			if e != nil {
				continue
			}
			if rimCaps[use.id] == 0 {
				return fail()
			}
			pcs, e := listRefs(sc.Args[2], 2)
			if e != nil || len(pcs) != 2 {
				return fail()
			}
			selected := int64(0)
			for _, pcid := range pcs {
				pc, e := simple(f, pcid, "PCURVE", 3)
				if e != nil {
					return fail()
				}
				owner, _ := ref(pc.Args[1])
				if owner == sid {
					if selected != 0 {
						return fail()
					}
					selected = pcid
				}
			}
			if selected == 0 {
				return fail()
			}
			sc.Args = append([]Value(nil), sc.Args...)
			sc.Args[2] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: selected}}}
			view.Entities[g] = Entity{ID: g, Records: []Record{sc}}
		}
		// Exact wall decoder identifies bottom/top and their shared source IDs.
		decoded, e := DecodeWrappedSplineFace(context.Background(), &view, wall, o)
		if e != nil {
			return wrappedCappedModel{}, e
		}
		for side, index := range []int{0, 2} {
			capID := rimCaps[decoded.SideEdges[index]]
			if capID == 0 {
				return fail()
			}
			cap, _ := simple(f, capID, "ADVANCED_FACE", 4)
			sense, _ := boolean(cap.Args[3])
			cn, e := textValue(cap.Args[0])
			if e != nil {
				return fail()
			}
			pair := capNames[wall]
			pair[side] = cn
			capNames[wall] = pair
			if sense != (side == 1) {
				return fail()
			}
		}
		names[wall] = name
		shells[shellID] = true
		shell.Type = "OPEN_SHELL"
		shell.Args = append([]Value(nil), shell.Args...)
		shell.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: wall}}}
		view.Entities[shellID] = Entity{ID: shellID, Records: []Record{shell}}
		body.Type = "SHELL_BASED_SURFACE_MODEL"
		body.Args = append([]Value(nil), body.Args...)
		body.Args[1] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: shellID}}}
		view.Entities[id] = Entity{ID: id, Records: []Record{body}}
	}
	for id, ent := range f.Entities {
		for _, r := range ent.Records {
			if r.Type == "CLOSED_SHELL" && !shells[id] {
				return fail()
			}
		}
	}
	// Remove cap IDs from proof-view ordering as well as its entity map.
	view.Order = nil
	for _, id := range f.Order {
		if !caps[id] {
			view.Order = append(view.Order, id)
		}
	}
	m, e := readWrappedSplineFacesFile(context.Background(), &view, l, o)
	if e != nil {
		return wrappedCappedModel{}, e
	}
	if len(m.Faces) != len(names) {
		return fail()
	}
	out := wrappedCappedModel{LengthUnit: m.LengthUnit}
	for i, face := range m.Faces {
		name, ok := names[m.FaceIDs[i]]
		if !ok {
			return fail()
		}
		b := wrappedCappedBody{Name: name, CapNames: capNames[m.FaceIDs[i]], Face: face}
		options := geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
		if frustum {
			b.Solid, b.FrustumProof, e = trimmed.NewWrappedFrustum(context.Background(), face.Geometry, 1e-10, options)
			if e == nil && !b.FrustumProof.Complete {
				return fail()
			}
		} else {
			b.Solid, b.CylinderProof, e = trimmed.NewWrappedCylinder(context.Background(), face.Geometry, 1e-10, options)
			if e == nil && !b.CylinderProof.Complete {
				return fail()
			}
		}
		if e != nil {
			return wrappedCappedModel{}, e
		}
		out.Bodies = append(out.Bodies, b)

	}
	return out, nil
}

func wrappedCylinderCap(f *File, id int64, o WrappedSurfaceOptions) (int64, error) {
	fail := func() (int64, error) { return 0, errors.New("step: unsupported exact rational cylinder cap") }
	face, e := simple(f, id, "ADVANCED_FACE", 4)
	if e != nil {
		return fail()
	}
	sense, e := boolean(face.Args[3])
	if e != nil {
		return fail()
	}
	bounds, e := listRefs(face.Args[1], 1)
	if e != nil || len(bounds) != 1 {
		return fail()
	}
	ent := f.Entities[bounds[0]]
	if len(ent.Records) != 1 || (ent.Records[0].Type != "FACE_BOUND" && ent.Records[0].Type != "FACE_OUTER_BOUND") {
		return fail()
	}
	bound, e := simple(f, bounds[0], ent.Records[0].Type, 3)
	if e != nil {
		return fail()
	}
	bs, e := boolean(bound.Args[2])
	if e != nil || bs != sense {
		return fail()
	}
	lid, _ := ref(bound.Args[1])
	loop, e := simple(f, lid, "EDGE_LOOP", 2)
	if e != nil {
		return fail()
	}
	uses, e := listRefs(loop.Args[1], 1)
	if e != nil || len(uses) != 1 {
		return fail()
	}
	use, e := simple(f, uses[0], "ORIENTED_EDGE", 5)
	if e != nil || use.Args[1].Kind != Derived || use.Args[2].Kind != Derived {
		return fail()
	}
	forward, e := boolean(use.Args[4])
	if e != nil || !forward {
		return fail()
	}
	eid, _ := ref(use.Args[3])
	a, b, g, e := analyticEdge(f, eid)
	if e != nil || a != b {
		return fail()
	}
	sid, _ := ref(face.Args[2])
	plane, e := planeFrame(f, sid)
	if e != nil || plane.XAxis() != (geom.Vec3{X: 1}) || plane.ZAxis() != (geom.Vec3{Z: 1}) || plane.Origin().X != 0 || plane.Origin().Y != 0 {
		return fail()
	}
	sc, e := simple(f, g, "SURFACE_CURVE", 4)
	if e != nil {
		return fail()
	}
	pcs, e := listRefs(sc.Args[2], 2)
	if e != nil || len(pcs) != 2 {
		return fail()
	}
	pcid := int64(0)
	for _, p := range pcs {
		pc, e := simple(f, p, "PCURVE", 3)
		if e != nil {
			return fail()
		}
		owner, _ := ref(pc.Args[1])
		if owner == sid {
			if pcid != 0 {
				return fail()
			}
			pcid = p
		}
	}
	if pcid == 0 {
		return fail()
	}
	pc, _ := simple(f, pcid, "PCURVE", 3)
	rid, _ := ref(pc.Args[2])
	rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	ctxID, e := ref(rep.Args[2])
	if e != nil {
		return fail()
	}
	seen := map[string]bool{}
	for _, record := range f.Entities[ctxID].Records {
		if seen[record.Type] {
			return fail()
		}
		seen[record.Type] = true
		switch record.Type {
		case "GEOMETRIC_REPRESENTATION_CONTEXT":
			if len(record.Args) != 1 {
				return fail()
			}
			n, e := integer(record.Args[0])
			if e != nil || n != 2 {
				return fail()
			}
		case "PARAMETRIC_REPRESENTATION_CONTEXT":
			if len(record.Args) != 0 {
				return fail()
			}
		case "REPRESENTATION_CONTEXT":
			if len(record.Args) != 2 || record.Args[0].Kind != String || record.Args[1].Kind != String {
				return fail()
			}
		default:
			return fail()
		}
	}
	if len(seen) != 3 {
		return fail()
	}
	curves, e := listRefs(rep.Args[1], 1)
	if e != nil || len(curves) != 1 {
		return fail()
	}
	cid := curves[0]
	// Lift every source 2D control exactly into the canonical cap plane.
	local := *f
	local.Entities = map[int64]Entity{}
	for k, v := range f.Entities {
		local.Entities[k] = v
	}
	r, ok := component(f.Entities[cid], "B_SPLINE_CURVE")
	if !ok {
		r, ok = component(f.Entities[cid], "B_SPLINE_CURVE_WITH_KNOTS")
		if !ok || len(r.Args) < 3 {
			return fail()
		}
		r.Args = r.Args[1:]
	}
	if len(r.Args) < 2 {
		return fail()
	}
	pids, e := listRefs(r.Args[1], 2)
	if e != nil || len(pids) > o.MaxPoles {
		return fail()
	}
	for _, pid := range pids {
		p, e := simple(f, pid, "CARTESIAN_POINT", 2)
		if e != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 2 {
			return fail()
		}
		p.Args = append([]Value(nil), p.Args...)
		p.Args[1] = Value{Kind: Aggregate, Items: append(append([]Value(nil), p.Args[1].Items...), Value{Kind: Number, Text: real(plane.Origin().Z)})}
		local.Entities[pid] = Entity{ID: pid, Records: []Record{p}}
	}
	modelID, _ := ref(sc.Args[1])
	for _, id := range []int64{cid, modelID} {
		if e = exactSplineWeightNormalization(f, id, "RATIONAL_B_SPLINE_CURVE"); e != nil {
			return 0, e
		}
	}
	uv, e := DecodeBSplineCurve(&local, cid, o.MaxPoles)
	if e != nil {
		return 0, e
	}
	model, e := DecodeBSplineCurve(f, modelID, o.MaxPoles)
	if e != nil {
		return 0, e
	}
	options := geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	u, e := geom.NewExactSplineCurve(uv.Geometry, options)
	if e != nil {
		return 0, e
	}
	v, e := geom.NewExactSplineCurve(model.Geometry, options)
	if e != nil {
		return 0, e
	}
	proof, e := geom.CompareExactCurves(u, v, geom.ExactCurveComparisonOptions{MaxSpans: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return 0, e
	}
	if !proof.Complete || !proof.Equal {
		return fail()
	}
	return eid, nil
}
