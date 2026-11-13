package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
)

// WrappedStationBoreBody is a strict equal-height circular through-bore with
// independently banded outer and inner station lofts and two annular caps.
// At least one wall must have multiple bands; the other may be one frustum.
// Original wall charts are retained; cached Solid and source IDs are not input proof.
type WrappedStationBoreBody struct {
	Name         string
	Outer, Inner WrappedStationLoftBody
	CapNames     [2]string
	Solid        trimmed.Solid
	LengthUnit   float64
}

func WrappedStationBoreFromSolid(ctx context.Context, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions) (WrappedStationBoreBody, error) {
	return wrappedAxialBoreFromSolid(ctx, s, name, unit, o, false)
}
func wrappedAxialBoreFromSolid(ctx context.Context, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions, single bool) (WrappedStationBoreBody, error) {
	p, e := s.ValidateWrappedCircularStationBore(ctx, o)
	if e != nil {
		return WrappedStationBoreBody{}, e
	}
	if !p.Complete {
		return WrappedStationBoreBody{}, geom.ErrConvergence
	}
	if (len(p.OuterWalls) == 1 && len(p.InnerWalls) == 1) != single {
		return WrappedStationBoreBody{}, geom.ErrConvergence
	}
	out := WrappedStationBoreBody{Name: name, Solid: s, LengthUnit: unit}
	faces := s.Faces()
	for i, ids := range [][]int{p.OuterWalls, p.InnerWalls} {
		walls := []*geom.WrappedSurfacePatch{}
		for _, id := range ids {
			w, ok := faces[id].Surface.(*geom.WrappedSurfacePatch)
			if !ok {
				return WrappedStationBoreBody{}, geom.ErrConvergence
			}
			walls = append(walls, w)
		}
		local, _, e := trimmed.NewWrappedCircularStationLoft(ctx, walls, 1e-10, o)
		if e != nil {
			return WrappedStationBoreBody{}, e
		}
		b, e := stationBoreLoftFromSolid(ctx, local, "", unit, o)
		if e != nil {
			return WrappedStationBoreBody{}, e
		}
		if i == 0 {
			out.Outer = b
		} else {
			out.Inner = b
		}
	}
	if single && !wrappedAxialBoreTapered(out) {
		return WrappedStationBoreBody{}, geom.ErrConvergence
	}
	return out, nil
}

func ReadWrappedStationBore(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedStationBoreBody, error) {
	return readWrappedAxialBore(r, l, o, false)
}
func readWrappedAxialBore(r io.Reader, l Limits, o WrappedSurfaceOptions, single bool) (WrappedStationBoreBody, error) {
	fail := func() (WrappedStationBoreBody, error) {
		return WrappedStationBoreBody{}, fmt.Errorf("step: unsupported wrapped axial bore graph")
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return WrappedStationBoreBody{}, e
	}
	o, e = o.defaults()
	if e != nil {
		return WrappedStationBoreBody{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return WrappedStationBoreBody{}, e
	}
	root, repID := int64(0), int64(0)
	unit := 0.
	for _, id := range f.Order {
		for _, rec := range f.Entities[id].Records {
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" || strings.HasSuffix(rec.Type, "SHAPE_REPRESENTATION") && rec.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" {
				return fail()
			}
		}
		rep, err := simple(f, id, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
		if err != nil {
			continue
		}
		if repID != 0 {
			return fail()
		}
		repID = id
		ctx, err := ref(rep.Args[2])
		if err != nil {
			return fail()
		}
		unit, e = contextUnits(f, ctx)
		if e != nil {
			return fail()
		}
		items, err := listRefs(rep.Args[1], 1)
		if err != nil {
			return fail()
		}
		for _, bid := range items {
			if _, err := simple(f, bid, "AXIS2_PLACEMENT_3D", 4); err == nil {
				if _, err = placement(f, bid); err != nil {
					return fail()
				}
				continue
			}
			if root != 0 {
				return fail()
			}
			root = bid
		}
	}
	if root == 0 || unit == 0 {
		return fail()
	}
	body, e := simple(f, root, "MANIFOLD_SOLID_BREP", 2)
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
	minFaces := 5
	if single {
		minFaces = 4
	}
	faces, e := listRefs(shell.Args[1], minFaces)
	if e != nil || len(faces) > 130 || len(faces) > l.MaxFaces {
		return fail()
	}
	if len(faces)-2 > o.MaxSurfaces {
		return fail()
	}
	points := 0
	for _, id := range f.Order {
		if _, err := simple(f, id, "CARTESIAN_POINT", 2); err == nil {
			points++
		}
	}
	if points > l.MaxVertices {
		return fail()
	}
	o.MaxOperations /= 2
	if o.MaxOperations < 1 {
		return WrappedStationBoreBody{}, geom.ErrConvergence
	}
	original := cloneOvalFile(f)
	next := int64(1)
	for _, id := range f.Order {
		if id >= next {
			next = id + 1
		}
	}
	emit := func(r Record) int64 {
		id := next
		next++
		f.Entities[id] = Entity{ID: id, Records: []Record{r}}
		f.Order = append(f.Order, id)
		return id
	}
	rv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	list := func(ids []int64) Value {
		v := Value{Kind: Aggregate}
		for _, id := range ids {
			v.Items = append(v.Items, rv(id))
		}
		return v
	}
	truth := Value{Kind: Enumeration, Text: "T"}
	walls := [2][]int64{}
	caps := []int64{}
	edgeSide := map[int64]int{}
	edgeUses := map[int64]int{}
	ownedFaces := map[int64]bool{}
	for _, fid := range faces {
		if ownedFaces[fid] {
			return fail()
		}
		ownedFaces[fid] = true
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		if _, ok := component(f.Entities[sid], "B_SPLINE_SURFACE_WITH_KNOTS"); !ok {
			if _, e = simple(f, sid, "PLANE", 2); e != nil {
				return fail()
			}
			caps = append(caps, fid)
			continue
		}
		sense, e := boolean(face.Args[3])
		if e != nil {
			return fail()
		}
		side := 0
		if !sense {
			side = 1
		}
		walls[side] = append(walls[side], fid)
		bs, e := listRefs(face.Args[1], 1)
		if e != nil || len(bs) != 1 {
			return fail()
		}
		bound, e := simple(f, bs[0], "FACE_BOUND", 3)
		if e != nil {
			bound, e = simple(f, bs[0], "FACE_OUTER_BOUND", 3)
		}
		if e != nil {
			return fail()
		}
		senseB, e := boolean(bound.Args[2])
		if e != nil || senseB != sense {
			return fail()
		}
		face.Args = append([]Value(nil), face.Args...)
		face.Args[3] = truth
		bound.Args = append([]Value(nil), bound.Args...)
		bound.Args[2] = truth
		ovalSet(f, fid, face)
		ovalSet(f, bs[0], bound)
		_, uses, e := analyticFaceLoop(f, fid, 4)
		if e != nil {
			return fail()
		}
		for _, u := range uses {
			if old, ok := edgeSide[u.id]; ok && old != side {
				return fail()
			}
			edgeSide[u.id] = side
			edgeUses[u.id]++
		}
		if e = normalizeWrappedIsoSplines(f, sid, o.MaxPoles); e != nil {
			return WrappedStationBoreBody{}, e
		}
	}
	if len(caps) != 2 || len(walls[0]) < 1 || len(walls[1]) < 1 || (!single && len(walls[0])+len(walls[1]) < 3) || (single && (len(walls[0]) != 1 || len(walls[1]) != 1)) || len(walls[0]) > 64 || len(walls[1]) > 64 {
		return fail()
	}
	capFaces := [2][]int64{}
	for _, fid := range caps {
		face, _ := simple(f, fid, "ADVANCED_FACE", 4)
		sense, e := boolean(face.Args[3])
		if e != nil {
			return fail()
		}
		bounds, e := listRefs(face.Args[1], 2)
		if e != nil || len(bounds) != 2 {
			return fail()
		}
		assigned := [2]bool{}
		for _, bid := range bounds {
			bound, e := simple(f, bid, "FACE_BOUND", 3)
			if e != nil {
				bound, e = simple(f, bid, "FACE_OUTER_BOUND", 3)
			}
			if e != nil {
				return fail()
			}
			bs, e := boolean(bound.Args[2])
			if e != nil || bs != sense {
				return fail()
			}
			lid, e := ref(bound.Args[1])
			if e != nil {
				return fail()
			}
			loop, e := simple(f, lid, "EDGE_LOOP", 2)
			if e != nil {
				return fail()
			}
			uses, e := listRefs(loop.Args[1], 1)
			if e != nil || len(uses) != 1 {
				return fail()
			}
			use, e := simple(f, uses[0], "ORIENTED_EDGE", 5)
			if e != nil {
				return fail()
			}
			eid, e := ref(use.Args[3])
			if e != nil {
				return fail()
			}
			side, ok := edgeSide[eid]
			if !ok || assigned[side] || side == 1 && bound.Type == "FACE_OUTER_BOUND" {
				return fail()
			}
			assigned[side] = true
			us, e := boolean(use.Args[4])
			if e != nil || us != (side == 0) {
				return fail()
			}
			edgeUses[eid]++
			use.Args = append([]Value(nil), use.Args...)
			use.Args[4] = truth
			uid := emit(use)
			loop.Args = append([]Value(nil), loop.Args...)
			loop.Args[1] = list([]int64{uid})
			nl := emit(loop)
			bound.Args = append([]Value(nil), bound.Args...)
			bound.Args[1] = rv(nl)
			nb := emit(bound)
			clone := face
			clone.Args = append([]Value(nil), face.Args...)
			clone.Args[1] = list([]int64{nb})
			capFaces[side] = append(capFaces[side], emit(clone))
		}
	}
	if len(edgeUses)*2 > l.MaxEdgeUses {
		return fail()
	}
	for _, n := range edgeUses {
		if n != 2 {
			return fail()
		}
	}
	// Source graph ownership is checked before constructing smaller proof views.
	sourceRoots := append([]int64(nil), faces...)
	owned := map[int64]bool{}
	active := map[int64]bool{}
	var visit func(int64, int) error
	visit = func(id int64, depth int) error {
		if active[id] || depth >= l.MaxDepth {
			return geom.ErrConvergence
		}
		if owned[id] {
			return nil
		}
		en, ok := original.Entities[id]
		if !ok {
			return geom.ErrInvalid
		}
		active[id] = true
		var value func(Value) error
		value = func(v Value) error {
			if v.Kind == Reference {
				return visit(v.Ref, depth+1)
			}
			for _, c := range v.Items {
				if e := value(c); e != nil {
					return e
				}
			}
			return nil
		}
		for _, rec := range en.Records {
			for _, v := range rec.Args {
				if e := value(v); e != nil {
					return e
				}
			}
		}
		active[id] = false
		owned[id] = true
		return nil
	}
	for _, id := range sourceRoots {
		if e = visit(id, 0); e != nil {
			return WrappedStationBoreBody{}, e
		}
	}
	for _, id := range original.Order {
		for _, rec := range original.Entities[id].Records {
			switch rec.Type {
			case "MANIFOLD_SOLID_BREP":
				if id != root {
					return fail()
				}
			case "CLOSED_SHELL":
				if id != shellID {
					return fail()
				}
			case "BREP_WITH_VOIDS", "ORIENTED_CLOSED_SHELL", "OPEN_SHELL":
				return fail()
			case "SURFACE_CURVE", "SEAM_CURVE", "PCURVE", "B_SPLINE_CURVE", "B_SPLINE_CURVE_WITH_KNOTS", "ADVANCED_FACE", "EDGE_CURVE", "VERTEX_POINT", "ORIENTED_EDGE", "EDGE_LOOP", "FACE_BOUND", "FACE_OUTER_BOUND", "B_SPLINE_SURFACE", "B_SPLINE_SURFACE_WITH_KNOTS":
				if !owned[id] {
					return fail()
				}
			}
		}
	}
	local := [2]WrappedStationLoftBody{}
	for side := 0; side < 2; side++ {
		kept := append(append([]int64(nil), walls[side]...), capFaces[side]...)
		var raw bytes.Buffer
		var remap map[int64]int64
		e = writeStandalone(&raw, unit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(put func(string) int64) []int64 {
			ids, mapping := copySplineGeometryWithIDs(put, f, kept)
			remap = mapping
			sh := put("CLOSED_SHELL(''," + references(ids) + ")")
			return []int64{put(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(name), sh))}
		})
		if e != nil {
			return WrappedStationBoreBody{}, e
		}
		internal := l
		internal.MaxBytes = max(l.MaxBytes, int64(raw.Len()))
		if len(walls[side]) == 1 {
			var model WrappedFrustumModel
			model, e = ReadWrappedFrusta(&raw, internal, o)
			if e == nil {
				if len(model.Bodies) != 1 {
					return fail()
				}
				local[side], e = stationBoreSingleDescriptor(model.Bodies[0], unit)
			}
		} else {
			local[side], e = ReadWrappedStationLoft(&raw, internal, o)
		}
		if e != nil {
			return WrappedStationBoreBody{}, e
		}
		reverse := map[int64]int64{}
		for source, target := range remap {
			reverse[target] = source
		}
		for j := range local[side].Walls {
			for k, id := range local[side].Walls[j].SideEdges {
				source, ok := reverse[id]
				if !ok {
					return fail()
				}
				local[side].Walls[j].SideEdges[k] = source
			}
			for k, id := range local[side].Walls[j].CornerVertices {
				source, ok := reverse[id]
				if !ok {
					return fail()
				}
				local[side].Walls[j].CornerVertices[k] = source
			}
		}

	}
	if single && !wrappedAxialBoreTapered(WrappedStationBoreBody{Outer: local[0], Inner: local[1]}) {
		return fail()
	}
	options := geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	solid, _, e := trimmed.CombineCoaxialWrappedStationLofts(context.Background(), local[0].Solid, local[1].Solid, parametric.Difference, options)
	if e != nil {
		return WrappedStationBoreBody{}, e
	}
	proof, e := solid.ValidateWrappedCircularStationBore(context.Background(), options)
	if e != nil || !proof.Complete {
		if e != nil {
			return WrappedStationBoreBody{}, e
		}
		return fail()
	}
	return WrappedStationBoreBody{Name: name, Outer: local[0], Inner: local[1], CapNames: local[0].CapNames, Solid: solid, LengthUnit: unit}, nil
}
