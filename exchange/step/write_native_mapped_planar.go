package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"strconv"
	"unicode/utf8"
)

// NativeMappedPlanarUse selects a zero-based local product and an explicit proper
// signed-cardinal placement. Repeated products remain shared representation maps.
type NativeMappedPlanarUse struct {
	Name      string
	Product   int
	Transform geom.Affine
}

// NativeMappedPlanarModel authors two to eight independently owned cardinal
// planar products and at most sixteen mapped uses. It is not a NAUO assembly.
type NativeMappedPlanarModel struct {
	Name       string
	Products   []Body
	Uses       []NativeMappedPlanarUse
	LengthUnit float64
}

// WriteNativeMappedPlanarMaterials generates original model and per-owner planar
// pcurves from native topology. Only cardinal straight edges and plane frames are
// accepted. Every generated decimal preserves the represented native coordinate;
// nonrepresentable UV subtraction fails, rather than rounding a pcurve. Complete
// material, ownership and placement proof is repeated before any output is sent.
func WriteNativeMappedPlanarMaterials(w io.Writer, m NativeMappedPlanarModel, l Limits) error {
	fail := mappedPlanarFailure
	if w == nil || len(m.Products) < 2 || len(m.Products) > 8 || len(m.Uses) < len(m.Products) || len(m.Uses) > 16 || !utf8.ValidString(m.Name) || len(m.Name) > 1024 {
		return fail()
	}
	lim, e := l.defaults()
	if e != nil {
		return e
	}
	// Bound native input before the general staging writer allocates its graph.
	vertices, faces, uses := 0, 0, 0
	for _, p := range m.Products {
		if len(p.Name) > 1024 {
			return fail()
		}
		vertices += len(p.Solid.Vertices())
		fs := p.Solid.Faces()
		faces += len(fs)
		for _, f := range fs {
			uses += len(f.Boundary)
			for _, h := range f.Holes {
				uses += len(h)
			}
		}
	}
	if vertices > lim.MaxVertices || faces > lim.MaxFaces || uses > lim.MaxEdgeUses || len(m.Products) > lim.MaxSolids {
		return fail()
	}
	var staged bytes.Buffer
	if e = WriteAdvanced(&staged, Model{Bodies: m.Products, LengthUnit: m.LengthUnit}); e != nil {
		return e
	}
	f, e := Parse(&staged, l)
	if e != nil {
		return e
	}
	var next int64
	reps := []int64{}
	sdrs := []int64{}
	for _, id := range f.Order {
		if id > next {
			next = id
		}
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		r := &en.Records[0]
		switch r.Type {
		case "MECHANICAL_CONTEXT":
			r.Type = "PRODUCT_CONTEXT"
		case "DESIGN_CONTEXT":
			r.Type = "PRODUCT_DEFINITION_CONTEXT"
		case "ADVANCED_BREP_SHAPE_REPRESENTATION":
			reps = append(reps, id)
		case "SHAPE_DEFINITION_REPRESENTATION":
			sdrs = append(sdrs, id)
		case "CARTESIAN_POINT", "DIRECTION":
			for i, v := range r.Args[1].Items {
				x, err := strconv.ParseFloat(v.Text, 64)
				if err != nil {
					return err
				}
				q := new(big.Rat).SetFloat64(x)
				if q == nil {
					return fail()
				}
				r.Args[1].Items[i] = Value{Kind: Number, Text: q.FloatString(q.Denom().BitLen() - 1)}
			}
		}
		f.Entities[id] = en
	}
	str := func(x string) Value { return Value{Kind: String, Text: x} }
	refv := func(x int64) Value { return Value{Kind: Reference, Ref: x} }
	agg := func(ids ...int64) Value {
		vs := []Value{}
		for _, id := range ids {
			vs = append(vs, refv(id))
		}
		return Value{Kind: Aggregate, Items: vs}
	}
	emit := func(kind string, args ...Value) int64 {
		next++
		f.Order = append(f.Order, next)
		f.Entities[next] = Entity{ID: next, Records: []Record{{Type: kind, Args: args}}}
		return next
	}
	coord := func(kind string, xs ...float64) int64 {
		vs := []Value{}
		for _, x := range xs {
			q := new(big.Rat).SetFloat64(x)
			vs = append(vs, Value{Kind: Number, Text: q.FloatString(q.Denom().BitLen() - 1)})
		}
		return emit(kind, str(""), Value{Kind: Aggregate, Items: vs})
	}
	// One independent 2D context per product; each original edge has exactly its
	// two actual face owners, never a nearest-plane assignment.
	for _, rid := range reps {
		rep := f.Entities[rid].Records[0]
		bodyID := rep.Args[1].Items[0].Ref
		body := f.Entities[bodyID].Records[0]
		shell := f.Entities[body.Args[1].Ref].Records[0]
		next++
		ctx := next
		f.Order = append(f.Order, ctx)
		f.Entities[ctx] = Entity{ID: ctx, Records: []Record{{Type: "GEOMETRIC_REPRESENTATION_CONTEXT", Args: []Value{{Kind: Number, Text: "2"}}}, {Type: "REPRESENTATION_CONTEXT", Args: []Value{str(""), str("")}}}}
		owners := map[int64][]int64{}
		edgeOrder := []int64{}
		for _, fv := range shell.Args[1].Items {
			face := f.Entities[fv.Ref].Records[0]
			sid := face.Args[2].Ref
			plane := f.Entities[sid].Records[0]
			if _, e = mappedExactFrame(f, plane.Args[1].Ref); e != nil {
				return e
			}
			for _, bv := range face.Args[1].Items {
				bound := f.Entities[bv.Ref].Records[0]
				loop := f.Entities[bound.Args[1].Ref].Records[0]
				for _, ov := range loop.Args[1].Items {
					oe := f.Entities[ov.Ref].Records[0]
					eid := oe.Args[3].Ref
					if len(owners[eid]) == 0 {
						edgeOrder = append(edgeOrder, eid)
					}
					owners[eid] = append(owners[eid], sid)
				}
			}
		}
		for _, eid := range edgeOrder {
			own := owners[eid]
			if len(own) != 2 || own[0] == own[1] {
				return fail()
			}
			edge := f.Entities[eid]
			lineID := edge.Records[0].Args[3].Ref
			line := f.Entities[lineID].Records[0]
			origin, e := mappedExactCoordinate(f, line.Args[1].Ref, "CARTESIAN_POINT", 3)
			if e != nil {
				return e
			}
			vec := f.Entities[line.Args[2].Ref].Records[0]
			direction, e := mappedExactCoordinate(f, vec.Args[1].Ref, "DIRECTION", 3)
			if e != nil || !cylinderSupportUnit(direction) {
				return fail()
			}
			pcs := []int64{}
			for _, sid := range own {
				plane := f.Entities[sid].Records[0]
				frame, e := mappedExactFrame(f, plane.Args[1].Ref)
				if e != nil {
					return e
				}
				project := func(axis geom.Vec3) (float64, error) {
					sum := new(big.Rat)
					a, b := mappedRatVec(origin), mappedRatVec(frame.Origin())
					for i, c := range []float64{axis.X, axis.Y, axis.Z} {
						sum.Add(sum, new(big.Rat).Mul(new(big.Rat).Sub(a[i], b[i]), new(big.Rat).SetFloat64(c)))
					}
					v, exact := sum.Float64()
					if !exact {
						return 0, fail()
					}
					return v, nil
				}
				u, e := project(frame.XAxis())
				if e != nil {
					return e
				}
				v, e := project(frame.YAxis())
				if e != nil {
					return e
				}
				p := coord("CARTESIAN_POINT", u, v)
				d := coord("DIRECTION", direction.Dot(frame.XAxis()), direction.Dot(frame.YAxis()))
				vector := emit("VECTOR", str(""), refv(d), Value{Kind: Number, Text: "1."})
				ln := emit("LINE", str(""), refv(p), refv(vector))
				dr := emit("DEFINITIONAL_REPRESENTATION", str(""), agg(ln), refv(ctx))
				pcs = append(pcs, emit("PCURVE", str(""), refv(sid), refv(dr)))
			}
			sc := emit("SURFACE_CURVE", str(""), refv(lineID), agg(pcs...), Value{Kind: Enumeration, Text: "PCURVE_S1"})
			edge.Records[0].Args[3] = refv(sc)
			f.Entities[eid] = edge
		}
	}
	maps := []int64{}
	for _, rid := range reps {
		p := coord("CARTESIAN_POINT", 0, 0, 0)
		x := coord("DIRECTION", 1, 0, 0)
		z := coord("DIRECTION", 0, 0, 1)
		origin := emit("AXIS2_PLACEMENT_3D", str(""), refv(p), refv(z), refv(x))
		en := f.Entities[rid]
		body := en.Records[0].Args[1].Items[0].Ref
		en.Records[0].Args[1] = agg(origin, body)
		f.Entities[rid] = en
		maps = append(maps, emit("REPRESENTATION_MAP", refv(origin), refv(rid)))
	}
	seen := map[int]bool{}
	items := []int64{}
	for _, u := range m.Uses {
		if u.Product < 0 || u.Product >= len(maps) || !utf8.ValidString(u.Name) || len(u.Name) > 1024 || !u.Transform.IsFinite() {
			return fail()
		}
		seen[u.Product] = true
		t := u.Transform
		x, y, z := geom.Vec3{X: t[0][0], Y: t[1][0], Z: t[2][0]}, geom.Vec3{X: t[0][1], Y: t[1][1], Z: t[2][1]}, geom.Vec3{X: t[0][2], Y: t[1][2], Z: t[2][2]}
		if !cylinderSupportUnit(x) || !cylinderSupportUnit(y) || !cylinderSupportUnit(z) || x.Cross(y) != z {
			return fail()
		}
		xi := coord("DIRECTION", x.X, x.Y, x.Z)
		yi := coord("DIRECTION", y.X, y.Y, y.Z)
		zi := coord("DIRECTION", z.X, z.Y, z.Z)
		p := coord("CARTESIAN_POINT", t[0][3], t[1][3], t[2][3])
		target := emit("CARTESIAN_TRANSFORMATION_OPERATOR_3D", str(""), str(""), str(""), refv(xi), refv(yi), refv(p), Value{Kind: Number, Text: "1."}, refv(zi))
		items = append(items, emit("MAPPED_ITEM", str(u.Name), refv(maps[u.Product]), refv(target)))
	}
	if len(seen) != len(maps) {
		return fail()
	}
	context := f.Entities[reps[0]].Records[0].Args[2]
	root := emit("SHAPE_REPRESENTATION", str(m.Name), agg(items...), context)
	for i, id := range sdrs {
		if i == 0 {
			en := f.Entities[id]
			en.Records[0].Args[1] = refv(root)
			f.Entities[id] = en
		} else {
			delete(f.Entities, id)
		}
	}
	order := []int64{}
	for _, id := range f.Order {
		if _, ok := f.Entities[id]; ok {
			order = append(order, id)
		}
	}
	f.Order = order
	f, e = mappedPlanarPartition(f, root, items, lim)
	if e != nil {
		return e
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	proved, e := ReadMappedPlanarMaterials(bytes.NewReader(raw), l)
	if e != nil {
		return fmt.Errorf("native mapped proof: %w", e)
	}
	for i, mid := range proved.MapOrder {
		if !nativeMappedSameBoundary(proved.Materials[mid].Body.Solid, m.Products[i].Solid) {
			return fail()
		}
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

// Decoding assigns vertex/edge indices by traversal. Bind oriented face loops
// by exact points rather than requiring incidental allocation order equality.
func nativeMappedSameBoundary(a, b brep.Solid) bool {
	af, bf := a.Faces(), b.Faces()
	if len(af) != len(bf) {
		return false
	}
	points := func(s brep.Solid, loop []brep.EdgeUse) []geom.Vec3 {
		v, e := s.Vertices(), s.Edges()
		out := []geom.Vec3{}
		for _, u := range loop {
			i := e[u.Edge].Start
			if u.Reversed {
				i = e[u.Edge].End
			}
			out = append(out, v[i])
		}
		return out
	}
	same := func(x, y []geom.Vec3) bool {
		if len(x) != len(y) {
			return false
		}
		for shift := range y {
			ok := true
			for i := range x {
				if x[i] != y[(i+shift)%len(y)] {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
		return false
	}
	for i, f := range af {
		g := bf[i]
		if !same(points(a, f.Boundary), points(b, g.Boundary)) || len(f.Holes) != len(g.Holes) {
			return false
		}
		for j, h := range f.Holes {
			if !same(points(a, h), points(b, g.Holes[j])) {
				return false
			}
		}
	}
	return true
}
