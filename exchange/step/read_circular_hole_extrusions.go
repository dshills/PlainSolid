package step

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strconv"
	"strings"
)

// ReadCircularHoleExtrusions accepts the canonical graph emitted by
// WriteCircularHoleExtrusions, including signed blind pockets. A through-only
// body is normalized to positive height from its lower cap; its original signed
// feature history is not inferable. The complete represented geometry and
// topology graph must equal re-authored, native-validated output.
// World-to-sketch recovery must be exactly representable. Equivalent alternate
// parameter frames or edge partitions are unsupported.
func ReadCircularHoleExtrusions(r io.Reader, l Limits) (CircularHoleExtrusionModel, error) {
	f, repID, unit, ids, e := parseCanonicalCircularExtrusions(r, l)
	if e != nil {
		return CircularHoleExtrusionModel{}, e
	}
	m := CircularHoleExtrusionModel{LengthUnit: unit}
	for _, id := range ids {
		b, e := recoverCircularExtrusion(f, id, nil)
		if e != nil {
			return CircularHoleExtrusionModel{}, e
		}
		m.Bodies = append(m.Bodies, b)
	}
	var out bytes.Buffer
	if e = WriteCircularHoleExtrusions(&out, m); e != nil {
		return CircularHoleExtrusionModel{}, e
	}
	if e := verifyCanonicalCircularExtrusionGraph(f, repID, &out); e != nil {
		return CircularHoleExtrusionModel{}, e
	}
	return m, nil
}

func parseCanonicalCircularExtrusions(r io.Reader, l Limits) (*File, int64, float64, []int64, error) {
	fail := func() (*File, int64, float64, []int64, error) {
		return nil, 0, 0, nil, fmt.Errorf("step: unsupported circular extrusion source graph")
	}
	var e error
	l, e = l.defaults()
	if e != nil {
		return nil, 0, 0, nil, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return nil, 0, 0, nil, e
	}
	repID := int64(0)
	counts := map[string]int{}
	for id, entity := range f.Entities {
		for _, rec := range entity.Records {
			counts[rec.Type]++
			if strings.Contains(rec.Type, "TRANSFORMATION") || strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP") || rec.Type == "MAPPED_ITEM" || rec.Type == "REPRESENTATION_MAP" || rec.Type == "NEXT_ASSEMBLY_USAGE_OCCURRENCE" {
				return fail()
			}
			if strings.HasSuffix(rec.Type, "SHAPE_REPRESENTATION") {
				if rec.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" || repID != 0 || len(entity.Records) != 1 {
					return fail()
				}
				repID = id
			}
		}
	}
	if repID == 0 || counts["ADVANCED_FACE"] > l.MaxFaces || counts["VERTEX_POINT"] > l.MaxVertices || counts["ORIENTED_EDGE"] > l.MaxEdgeUses || counts["MANIFOLD_SOLID_BREP"] > l.MaxSolids {
		return fail()
	}
	rep, e := simple(f, repID, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	ctx, e := ref(rep.Args[2])
	if e != nil {
		return fail()
	}
	unit, e := contextUnits(f, ctx)
	if e != nil {
		return fail()
	}
	ids, e := listRefs(rep.Args[1], 1)
	if e != nil || len(ids) > 32 {
		return fail()
	}
	return f, repID, unit, ids, nil
}

func verifyCanonicalCircularExtrusionGraph(f *File, repID int64, out io.Reader) error {
	fail := func() error { return fmt.Errorf("step: unsupported circular extrusion source graph") }
	expected, e := Parse(out, Limits{})
	if e != nil {
		return fail()
	}
	expectedRep := int64(0)
	for id, en := range expected.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
			expectedRep = id
		}
	}
	used, e := equalCircularExtrusionGraph(f, repID, expected, expectedRep)
	if e != nil {
		return e
	}
	// Only the producer's non-geometric product metadata may remain outside
	// the represented geometry closure; detached or unknown geometry fails closed.
	metadata := map[string]bool{"APPLICATION_CONTEXT": true, "APPLICATION_PROTOCOL_DEFINITION": true, "MECHANICAL_CONTEXT": true, "DESIGN_CONTEXT": true, "PRODUCT": true, "PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE": true, "PRODUCT_DEFINITION": true, "PRODUCT_DEFINITION_SHAPE": true, "SHAPE_DEFINITION_REPRESENTATION": true}
	for id, en := range f.Entities {
		if used[id] {
			continue
		}
		for _, rec := range en.Records {
			if !metadata[rec.Type] {
				return fmt.Errorf("step: detached circular extrusion entity #%d (%s)", id, rec.Type)
			}
		}
	}

	return nil
}
func recoverCircularExtrusion(f *File, id int64, diskOut *trimmed.ExtrusionCircle) (CircularHoleExtrusionBody, error) {
	fail := func() (CircularHoleExtrusionBody, error) {
		return CircularHoleExtrusionBody{}, fmt.Errorf("step: unsupported canonical circular extrusion body")
	}
	body, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	if e != nil {
		return fail()
	}
	name, e := textValue(body.Args[0])
	if e != nil {
		return fail()
	}
	sid, e := ref(body.Args[1])
	if e != nil {
		return fail()
	}
	shell, e := simple(f, sid, "CLOSED_SHELL", 2)
	if e != nil {
		return fail()
	}
	faces, e := listRefs(shell.Args[1], 3)
	if e != nil || len(faces) > 98 {
		return fail()
	}
	var caps [2]geom.Frame
	for j := 0; j < 2; j++ {
		face, e := simple(f, faces[len(faces)-2+j], "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		s, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		caps[j], e = planeFrame(f, s)
		if e != nil {
			return fail()
		}
	}
	if caps[0].XAxis() != caps[1].XAxis() || caps[0].ZAxis() != caps[1].ZAxis() {
		return fail()
	}
	delta := caps[0].ToLocal(caps[1].Origin())
	if delta.X != 0 || delta.Y != 0 || delta.Z <= 0 {
		return fail()
	}
	height := delta.Z
	top, _ := simple(f, faces[len(faces)-1], "ADVANCED_FACE", 4)
	bounds, e := listRefs(top.Args[1], 1)
	if e != nil {
		return fail()
	}
	outer, e := simple(f, bounds[0], "FACE_OUTER_BOUND", 3)
	if e != nil {
		return fail()
	}
	lid, e := ref(outer.Args[1])
	if e != nil {
		return fail()
	}
	loop, e := simple(f, lid, "EDGE_LOOP", 2)
	if e != nil {
		return fail()
	}
	uses, e := listRefs(loop.Args[1], 1)
	if e != nil || len(uses) > 64 {
		return fail()
	}
	m := CircularHoleExtrusionBody{Name: name, Frame: caps[0], Height: height}
	if diskOut == nil {
		if len(faces) < 6 || len(uses) < 3 {
			return fail()
		}
		for _, uid := range uses {
			u, e := simple(f, uid, "ORIENTED_EDGE", 5)
			if e != nil {
				return fail()
			}
			eid, e := ref(u.Args[3])
			if e != nil {
				return fail()
			}
			a, z, _, e := analyticEdge(f, eid)
			if e != nil {
				return fail()
			}
			forward, e := boolean(u.Args[4])
			if e != nil {
				return fail()
			}
			if !forward {
				a = z
			}
			p, e := analyticVertex(f, a)
			if e != nil {
				return fail()
			}
			q := caps[1].ToLocal(p)
			if q.Z != 0 {
				return fail()
			}
			m.Outer = append(m.Outer, geom.Vec2{X: q.X, Y: q.Y})
		}
	} else {
		if len(uses) != 1 {
			return fail()
		}
		use, e := simple(f, uses[0], "ORIENTED_EDGE", 5)
		if e != nil {
			return fail()
		}
		edge, e := ref(use.Args[3])
		if e != nil {
			return fail()
		}
		rim, radius, e := canonicalExtrusionRim(f, edge)
		if e != nil {
			return fail()
		}
		q := caps[1].ToLocal(rim.Origin())
		if q.Z != 0 {
			return fail()
		}
		*diskOut = trimmed.ExtrusionCircle{Center: geom.Vec2{X: q.X, Y: q.Y}, Radius: radius}
	}
	entry := -1
	for _, fid := range faces[:len(faces)-2] {
		face, e := simple(f, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		surfID, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		surface, e := DecodeAnalyticSurface(f, surfID)
		if e != nil {
			return fail()
		}
		cyl, ok := surface.(geom.CylinderSurface)
		if !ok {
			if _, ok = surface.(geom.PlaneSurface); !ok {
				return fail()
			}
			continue
		}
		cf, radius := cyl.Parameters()
		c := caps[0].ToLocal(cf.Origin())
		h := trimmed.ExtrusionCircle{Center: geom.Vec2{X: c.X, Y: c.Y}, Radius: radius}
		exterior := diskOut != nil && fid == faces[0]
		if exterior && h != *diskOut {
			return fail()
		}
		_, edgeUses, e := analyticFaceLoop(f, fid, 4)
		if e != nil {
			return fail()
		}
		zs := []float64{}
		for _, u := range edgeUses {
			a, z, _, e := analyticEdge(f, u.id)
			if e != nil {
				return fail()
			}
			if a == z {
				fr, _, e := canonicalExtrusionRim(f, u.id)
				if e != nil {
					return fail()
				}
				zs = append(zs, caps[0].ToLocal(fr.Origin()).Z)
			}
		}
		if len(zs) != 2 {
			return fail()
		}
		low, high := math.Min(zs[0], zs[1]), math.Max(zs[0], zs[1])
		if low < 0 || high > height || low >= high {
			return fail()
		}
		if exterior {
			if low != 0 || high != height {
				return fail()
			}
			continue
		}
		if low == 0 && high == height {
			m.Holes = append(m.Holes, h)
		} else {
			side := -1
			if low == 0 {
				side = 0
			} else if high == height {
				side = 1
			}
			if side < 0 || entry >= 0 && entry != side {
				return fail()
			}
			entry = side
			m.Blind = append(m.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: h, Depth: high - low})
		}
		if len(m.Holes)+len(m.Blind) > 16 {
			return fail()
		}
	}
	if entry == 1 {
		m.Frame = caps[1]
		m.Height = -height
	}
	return m, nil
}
func canonicalExtrusionRim(f *File, edge int64) (geom.Frame, float64, error) {
	fail := func() (geom.Frame, float64, error) {
		return geom.Frame{}, 0, fmt.Errorf("step: unsupported circular extrusion rim")
	}
	a, z, g, e := analyticEdge(f, edge)
	if e != nil || a != z {
		return fail()
	}
	sc, e := simple(f, g, "SURFACE_CURVE", 4)
	if e != nil {
		return fail()
	}
	curve, e := ref(sc.Args[1])
	if e != nil {
		return fail()
	}
	circle, e := simple(f, curve, "CIRCLE", 3)
	if e != nil {
		return fail()
	}
	id, e := ref(circle.Args[1])
	if e != nil {
		return fail()
	}
	fr, e := placement(f, id)
	if e != nil {
		return fail()
	}
	radius, e := numeric(circle.Args[2])
	if e != nil || radius <= 0 {
		return fail()
	}
	return fr, radius, nil
}

// A bidirectional entity map preserves sharing, owner identity and repeated
// seam references; equal values alone cannot authorize cloned/unowned topology.
func equalCircularExtrusionGraph(a *File, ar int64, b *File, br int64) (map[int64]bool, error) {
	forward, back := map[int64]int64{}, map[int64]int64{}
	used := map[int64]bool{}
	work := 0
	fail := func() error {
		return fmt.Errorf("step: circular extrusion source geometry/topology differs from certified reconstruction")
	}
	var entity func(int64, int64, int) error
	var value func(Value, Value, int) error
	value = func(x, y Value, depth int) error {
		work++
		if work > 2000000 || depth > 128 || x.Kind != y.Kind {
			return fail()
		}
		switch x.Kind {
		case Reference:
			return entity(x.Ref, y.Ref, depth+1)
		case Number:
			number := func(s string) (*big.Rat, bool) {
				if len(s) > 128 {
					return nil, false
				}
				if i := strings.IndexAny(s, "Ee"); i >= 0 {
					n, e := strconv.Atoi(s[i+1:])
					if e != nil || n < -400 || n > 400 {
						return nil, false
					}
				}
				return new(big.Rat).SetString(s)
			}
			p, ok := number(x.Text)
			q, yes := number(y.Text)
			if !ok || !yes || p.Cmp(q) != 0 {
				return fail()
			}
		default:
			if x.Text != y.Text || len(x.Items) != len(y.Items) {
				return fail()
			}
			for i := range x.Items {
				if e := value(x.Items[i], y.Items[i], depth+1); e != nil {
					return e
				}
			}
		}
		return nil
	}
	entity = func(x, y int64, depth int) error {
		if z, ok := forward[x]; ok {
			if z != y {
				return fail()
			}
			return nil
		}
		if _, ok := back[y]; ok {
			return fail()
		}
		p, ok := a.Entities[x]
		q, yes := b.Entities[y]
		if !ok || !yes || len(p.Records) != len(q.Records) {
			return fail()
		}
		forward[x], back[y], used[x] = y, x, true
		for i, r := range p.Records {
			s := q.Records[i]
			if r.Type != s.Type || len(r.Args) != len(s.Args) {
				return fail()
			}
			for j := range r.Args {
				if e := value(r.Args[j], s.Args[j], depth+1); e != nil {
					return e
				}
			}
		}
		return nil
	}
	if e := entity(ar, br, 0); e != nil {
		return nil, e
	}
	return used, nil
}
