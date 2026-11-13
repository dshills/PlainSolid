package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"sort"
)

// WrappedTubeBody retains the two positive support descriptors; the inner wall
// is reversed in Solid. It accepts one independently certified through-bore.
type WrappedTubeBody struct {
	Name         string
	FloorName    string // Only populated by the distinct blind-bore API.
	Outer, Inner WrappedSplineFace
	CapNames     [2]string
	Solid        trimmed.Solid
	LengthUnit   float64
}

// ReadWrappedTube requires two opposed wrapped walls and two exact annular
// planar caps. It preserves the original wall geometry and proves homothetic
// nesting through NewWrappedCylinderTube, not merely shell incidence.
func ReadWrappedTube(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedTubeBody, error) {
	return readWrappedBore(r, l, o, false)
}

func readWrappedBore(r io.Reader, l Limits, o WrappedSurfaceOptions, blind bool) (WrappedTubeBody, error) {
	return readWrappedArrangement(r, l, o, blind, false)
}

func readWrappedArrangement(r io.Reader, l Limits, o WrappedSurfaceOptions, blind, shoulder bool) (WrappedTubeBody, error) {
	fail := func() (WrappedTubeBody, error) {
		return WrappedTubeBody{}, errors.New("step: unsupported wrapped bore graph")
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return WrappedTubeBody{}, e
	}
	o, e = o.defaults()
	if e != nil {
		return WrappedTubeBody{}, e
	}
	faceCount := 4
	capCount := 2
	if blind {
		faceCount = 5
		capCount = 3
	}
	if l.MaxFaces < faceCount || l.MaxEdgeUses < 12 {
		return fail()
	}
	f, e := Parse(r, l)
	if e != nil {
		return WrappedTubeBody{}, e
	}
	bodyID := int64(0)
	next := int64(1)
	for _, id := range f.Order {
		if id >= next {
			next = id + 1
		}
		if _, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2); e == nil {
			if bodyID != 0 {
				return fail()
			}
			bodyID = id
		}
	}
	if bodyID == 0 {
		return fail()
	}
	emit := func(rec Record) int64 {
		id := next
		next++
		f.Entities[id] = Entity{ID: id, Records: []Record{rec}}
		f.Order = append(f.Order, id)
		return id
	}
	set := func(id int64, rec Record) { f.Entities[id] = Entity{ID: id, Records: []Record{rec}} }
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	list := func(ids ...int64) Value {
		v := Value{Kind: Aggregate}
		for _, id := range ids {
			v.Items = append(v.Items, refv(id))
		}
		return v
	}
	truth := Value{Kind: Enumeration, Text: "T"}
	body, _ := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	shellID, _ := ref(body.Args[1])
	shell, e := simple(f, shellID, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	faces, e := listRefs(shell.Args[1], faceCount)
	if e != nil || len(faces) != faceCount {
		return fail()
	}
	larger := int64(0)
	if shoulder {
		candidates := []int64{}
		proofs := []geom.WrappedCylinderEmbedding{}
		for _, fid := range faces {
			face, e := simple(f, fid, "ADVANCED_FACE", 4)
			if e != nil {
				return fail()
			}
			sid, _ := ref(face.Args[2])
			if _, ok := component(f.Entities[sid], "B_SPLINE_SURFACE_WITH_KNOTS"); !ok {
				continue
			}
			w, e := DecodeWrappedSurface(context.Background(), f, sid, o)
			if e != nil {
				return WrappedTubeBody{}, e
			}
			p, e := geom.CertifyWrappedCylinderEmbedding(context.Background(), w.Geometry, geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil || !p.Complete {
				return fail()
			}
			candidates = append(candidates, fid)
			proofs = append(proofs, p)
		}
		if len(candidates) != 2 || proofs[0].Radius.Cmp(proofs[1].Radius) == 0 {
			return fail()
		}
		larger = candidates[0]
		if proofs[0].Radius.Cmp(proofs[1].Radius) < 0 {
			larger = candidates[1]
		}
	}
	walls := [2]int64{}
	caps := []int64{}
	edgeWall := map[int64]int{}
	for _, fid := range faces {
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		sid, _ := ref(face.Args[2])
		if _, ok := component(f.Entities[sid], "B_SPLINE_SURFACE_WITH_KNOTS"); !ok {
			caps = append(caps, fid)
			continue
		}
		sense, e := boolean(face.Args[3])
		if e != nil {
			return fail()
		}
		index := 0
		if !sense {
			index = 1
		}
		if shoulder {
			if !sense {
				return fail()
			}
			index = 1
			if fid == larger {
				index = 0
			}
		}
		if walls[index] != 0 {
			return fail()
		}
		walls[index] = fid
		bounds, e := listRefs(face.Args[1], 1)
		if e != nil || len(bounds) != 1 {
			return fail()
		}
		ent := f.Entities[bounds[0]]
		if len(ent.Records) != 1 {
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
		face.Args = append([]Value(nil), face.Args...)
		face.Args[3] = truth
		bound.Args = append([]Value(nil), bound.Args...)
		bound.Args[2] = truth
		set(fid, face)
		set(bounds[0], bound)
		_, uses, e := analyticFaceLoop(f, fid, 4)
		if e != nil {
			return fail()
		}
		for _, u := range uses {
			if owner, ok := edgeWall[u.id]; ok && owner != index {
				return fail()
			}
			edgeWall[u.id] = index
		}
		if e = normalizeWrappedIsoSplines(f, sid, o.MaxPoles); e != nil {
			return WrappedTubeBody{}, e
		}
	}
	if walls[0] == 0 || walls[1] == 0 || len(caps) != capCount {
		return fail()
	}
	capFaces := [2][]int64{}
	annuli, floors, outerDisks := 0, 0, 0
	floorName := ""
	for _, fid := range caps {
		face, _ := simple(f, fid, "ADVANCED_FACE", 4)
		sense, e := boolean(face.Args[3])
		if e != nil {
			return fail()
		}
		bounds, e := listRefs(face.Args[1], 1)
		if e != nil || len(bounds) > 2 || (!blind && len(bounds) != 2) {
			return fail()
		}
		if len(bounds) == 2 {
			annuli++
		}
		assigned := [2]bool{}
		for _, bid := range bounds {
			ent := f.Entities[bid]
			if len(ent.Records) != 1 || (ent.Records[0].Type != "FACE_BOUND" && ent.Records[0].Type != "FACE_OUTER_BOUND") {
				return fail()
			}
			bound, e := simple(f, bid, ent.Records[0].Type, 3)
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
			if e != nil {
				return fail()
			}
			eid, _ := ref(use.Args[3])
			index, ok := edgeWall[eid]
			if !ok || assigned[index] || (!blind && !shoulder && index == 1 && bound.Type == "FACE_OUTER_BOUND") {
				return fail()
			}
			us, e := boolean(use.Args[4])
			expectedUse := index == 0 || len(bounds) == 1
			if e != nil || us != expectedUse {
				return fail()
			}
			if len(bounds) == 1 {
				if index == 0 {
					outerDisks++
				} else {
					floors++
					floorName, e = textValue(face.Args[0])
					if e != nil {
						return fail()
					}
				}
			}
			assigned[index] = true
			use.Args = append([]Value(nil), use.Args...)
			use.Args[4] = truth
			uid := emit(use)
			loop.Args = append([]Value(nil), loop.Args...)
			loop.Args[1] = list(uid)
			newLoop := emit(loop)
			bound.Args = append([]Value(nil), bound.Args...)
			bound.Args[1] = refv(newLoop)
			if index == 1 && ((!shoulder && len(bounds) == 1) || (shoulder && len(bounds) == 2)) {
				value := "T"
				if sense {
					value = "F"
				}
				bound.Args[2] = Value{Kind: Enumeration, Text: value}
			}
			newBound := emit(bound)
			clone := face
			clone.Args = append([]Value(nil), face.Args...)
			clone.Args[1] = list(newBound)
			if index == 1 && ((!shoulder && len(bounds) == 1) || (shoulder && len(bounds) == 2)) {
				clone.Args[3] = bound.Args[2]
			}
			capFaces[index] = append(capFaces[index], emit(clone))
		}
		delete(f.Entities, fid)
	}
	if len(capFaces[0]) != 2 || len(capFaces[1]) != 2 {
		return fail()
	}
	if blind {
		if annuli != 1 || floors != 1 || outerDisks != 1 {
			return fail()
		}
	} else if annuli != 2 {
		return fail()
	}
	shell.Args = append([]Value(nil), shell.Args...)
	shell.Args[1] = list(walls[0], capFaces[0][0], capFaces[0][1])
	set(shellID, shell)
	innerShell := shell
	innerShell.Args = append([]Value(nil), shell.Args...)
	innerShell.Args[1] = list(walls[1], capFaces[1][0], capFaces[1][1])
	newShell := emit(innerShell)
	innerBody := body
	innerBody.Args = append([]Value(nil), body.Args...)
	innerBody.Args[1] = refv(newShell)
	newBody := emit(innerBody)
	reps := 0
	for _, id := range f.Order {
		rep, e := simple(f, id, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
		if e != nil {
			continue
		}
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return fail()
		}
		owns := false
		for _, item := range items {
			if item == bodyID {
				owns = true
			}
		}
		if owns {
			reps++
			rep.Args = append([]Value(nil), rep.Args...)
			rep.Args[1] = Value{Kind: Aggregate, Items: append(append([]Value(nil), rep.Args[1].Items...), refv(newBody))}
			set(id, rep)
		}
	}
	if reps != 1 {
		return fail()
	}
	raw := serializeWrappedProofFile(f)
	internal := l
	internal.MaxSolids = max(2, l.MaxSolids)
	internal.MaxFaces = max(6, l.MaxFaces)
	internal.MaxEdgeUses = max(12, l.MaxEdgeUses)
	m, e := ReadWrappedCylindricalBodies(bytes.NewReader(raw), internal, o)
	if e != nil {
		return WrappedTubeBody{}, e
	}
	if len(m.Bodies) != 2 {
		return fail()
	}
	outer, inner := m.Bodies[0], m.Bodies[1]
	var solid trimmed.Solid
	options := geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	if shoulder {
		if outer.Embedding.UpperZ.Cmp(inner.Embedding.LowerZ) != 0 && inner.Embedding.UpperZ.Cmp(outer.Embedding.LowerZ) != 0 {
			return fail()
		}
		solid, _, e = trimmed.CombineAxialWrappedCylinders(context.Background(), outer.Solid, inner.Solid, parametric.Union, options)
	} else if blind {
		low := outer.Embedding.LowerZ.Cmp(inner.Embedding.LowerZ) == 0
		high := outer.Embedding.UpperZ.Cmp(inner.Embedding.UpperZ) == 0
		if low == high {
			return fail()
		}
		solid, _, e = trimmed.NewWrappedCylinderBore(context.Background(), outer.Face.Geometry, inner.Face.Geometry, 1e-10, options)
	} else {
		solid, _, e = trimmed.NewWrappedCylinderTube(context.Background(), outer.Face.Geometry, inner.Face.Geometry, 1e-10, options)
	}

	if e != nil {
		return WrappedTubeBody{}, e
	}
	return WrappedTubeBody{Name: name, FloorName: floorName, Outer: outer.Face, Inner: inner.Face, CapNames: outer.CapNames, Solid: solid, LengthUnit: m.LengthUnit}, nil
}

func serializeWrappedProofFile(f *File) []byte {
	var b bytes.Buffer
	b.WriteString("ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('proof'),'2;1');\nFILE_NAME('proof','',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));\nENDSEC;\nDATA;\n")
	ids := map[int64]int64{}
	next := int64(1)
	for id := range f.Entities {
		if id >= next {
			next = id + 1
		}
		if id > 0 {
			ids[id] = id
		}
	}
	negative := []int64{}
	for id := range f.Entities {
		if id < 0 {
			negative = append(negative, id)
		}
	}
	sort.Slice(negative, func(i, j int) bool { return negative[i] < negative[j] })
	for _, id := range negative {
		ids[id] = next
		next++
	}
	for _, id := range f.Order {
		if ent, ok := f.Entities[id]; ok {
			fmt.Fprintf(&b, "#%d=%s;\n", ids[id], assemblyEntity(ent, ids))
		}
	}
	for _, id := range negative {
		fmt.Fprintf(&b, "#%d=%s;\n", ids[id], assemblyEntity(f.Entities[id], ids))
	}
	b.WriteString("ENDSEC;\nEND-ISO-10303-21;\n")
	return b.Bytes()
}
