package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// NURBSPlanarSurfaceEvidence identifies an original affine bilinear chart.
// All strings are exact rational coefficients in its original parameterization.
type NURBSPlanarSurfaceEvidence struct {
	SurfaceID, FaceID int64
	PoleIDs           [4]int64
	U, V              [2]string
	Origin, DU, DV    [3]string
}

type NURBSPlanarCurveEvidence struct {
	CurveID, SurfaceCurveID, EdgeID int64
	PoleIDs                         [2]int64
	Knots                           [2]string
	Origin, Derivative              [3]string
}

// NURBSPlanarAssembly authenticates a bounded original polynomial NURBS
// assembly. Geometry is derived from an equivalent private PLANE/LINE view;
// its Source is that proof view, not the original file. Source re-export uses
// the separately retained original and re-proves it, never the mutable claims.
type NURBSPlanarAssembly struct {
	Geometry ColoredAssembly
	original string
	surfaces []NURBSPlanarSurfaceEvidence
	curves   []NURBSPlanarCurveEvidence
	pcurves  []AffinePCurveEvidence
}

func (m NURBSPlanarAssembly) OriginalSTEP() string { return m.original }
func (m NURBSPlanarAssembly) Surfaces() []NURBSPlanarSurfaceEvidence {
	return append([]NURBSPlanarSurfaceEvidence(nil), m.surfaces...)
}
func (m NURBSPlanarAssembly) Curves() []NURBSPlanarCurveEvidence {
	return append([]NURBSPlanarCurveEvidence(nil), m.curves...)
}
func (m NURBSPlanarAssembly) PCurves() []AffinePCurveEvidence {
	return append([]AffinePCurveEvidence(nil), m.pcurves...)
}

// ReadNURBSPlanarAssembly accepts degree-one model/UV curves and bilinear
// affine support charts with signed-cardinal unit derivatives. Every finite
// curve endpoint, owner chart rectangle and continuous model/UV identity is
// checked before the existing planar material/assembly proof. No fitting occurs.
func ReadNURBSPlanarAssembly(ctx context.Context, r io.Reader, l Limits) (NURBSPlanarAssembly, error) {
	var zero NURBSPlanarAssembly
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 4096)
	l.MaxValues = min(l.MaxValues, 65536)
	l.MaxDepth = min(l.MaxDepth, 16)
	l.MaxEdgeUses = min(l.MaxEdgeUses, 4096)
	l.MaxSolids = min(l.MaxSolids, 16)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrInvalid
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	s, c, e := nurbsPlanarView(ctx, f)
	if e != nil {
		return zero, e
	}
	p, e := affinePCurveView(ctx, f)
	if e != nil {
		return zero, e
	}
	if len(s) == 0 || len(c) == 0 || len(p) != 2*len(c) {
		return zero, geom.ErrInvalid
	}
	view, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return zero, e
	}
	m, e := ReadColoredAssembly(bytes.NewReader(view), l)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	for _, product := range m.Assembly.Products {
		for _, body := range m.ProductBodies[product.DefinitionID] {
			if e = poly.ValidateEmbeddedBoundary(ctx, body.Body.Solid, nurbsPlanarEmbeddingOptions()); e != nil {
				return zero, e
			}
		}
	}
	return NURBSPlanarAssembly{m, string(raw), s, c, p}, nil
}

// WriteNURBSPlanarAssemblySource revalidates and emits the retained original
// records. It does not author edited geometry or normalize source knot bases.
func WriteNURBSPlanarAssemblySource(ctx context.Context, w io.Writer, m NURBSPlanarAssembly, l Limits) error {
	if w == nil {
		return geom.ErrInvalid
	}
	checked, e := ReadNURBSPlanarAssembly(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := io.WriteString(w, checked.original)
	if e == nil && n != len(checked.original) {
		return io.ErrShortWrite
	}
	return e
}

func nurbsPlanarView(ctx context.Context, f *File) ([]NURBSPlanarSurfaceEvidence, []NURBSPlanarCurveEvidence, error) {
	fail := func() error {
		return fmt.Errorf("step: unsupported exact NURBS planar owner/domain: %w", geom.ErrInvalid)
	}
	incoming := map[int64][]int64{}
	var maxID int64
	var walk func(int64, Value)
	walk = func(id int64, v Value) {
		if v.Kind == Reference {
			incoming[v.Ref] = append(incoming[v.Ref], id)
		}
		for _, q := range v.Items {
			walk(id, q)
		}
	}
	for id, en := range f.Entities {
		maxID = max(maxID, id)
		for _, r := range en.Records {
			for _, v := range r.Args {
				walk(id, v)
			}
		}
	}
	// Keep occurrence transforms exact: original frames are cardinal, and the
	// child mapping frame is neutral. No floating inverse/rephasing is needed.
	for _, id := range f.Order {
		if _, e := simple(f, id, "AXIS2_PLACEMENT_3D", 4); e == nil {
			if _, e = mappedExactFrame(f, id); e != nil {
				return nil, nil, e
			}
		}
		if r, e := simple(f, id, "ITEM_DEFINED_TRANSFORMATION", 4); e == nil {
			fid, e := ref(r.Args[2])
			if e != nil {
				return nil, nil, e
			}
			frame, e := mappedExactFrame(f, fid)
			if e != nil {
				return nil, nil, e
			}
			if frame.Origin() != (geom.Vec3{}) || frame.XAxis() != (geom.Vec3{X: 1}) || frame.ZAxis() != (geom.Vec3{Z: 1}) {
				return nil, nil, fail()
			}
		}
	}
	if maxID > 1<<60 {
		return nil, nil, fail()
	}
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	ratval := func(q *big.Rat) (Value, error) {
		s, e := extrusionDirectionDecimal(q)
		if e != nil {
			return Value{}, e
		}
		v := Value{Kind: Number, Text: s}
		_, e = wrappedIsoSourceNumber(v)
		return v, e
	}
	putcoord := func(id int64, kind string, p [3]*big.Rat) error {
		a := make([]Value, 3)
		for i := range a {
			v, e := ratval(p[i])
			if e != nil {
				return e
			}
			a[i] = v
		}
		f.Entities[id] = Entity{ID: id, Records: []Record{{Type: kind, Args: []Value{{Kind: String}, {Kind: Aggregate, Items: a}}}}}
		return nil
	}
	emit := func(r Record) int64 {
		maxID++
		f.Order = append(f.Order, maxID)
		f.Entities[maxID] = Entity{ID: maxID, Records: []Record{r}}
		return maxID
	}
	coord := func(id int64, owner int64) ([3]*big.Rat, error) {
		var p [3]*big.Rat
		if len(incoming[id]) != 1 || incoming[id][0] != owner {
			return p, fail()
		}
		r, e := simple(f, id, "CARTESIAN_POINT", 2)
		if e != nil || r.Args[1].Kind != Aggregate || len(r.Args[1].Items) != 3 {
			return p, fail()
		}
		if _, e = textValue(r.Args[0]); e != nil {
			return p, e
		}
		for i, v := range r.Args[1].Items {
			if _, e = wrappedIsoSourceNumber(v); e != nil {
				return p, e
			}
			p[i], e = liftSourceRat(v)
			if e != nil {
				return p, e
			}
		}
		return p, nil
	}
	unit := func(p [3]*big.Rat) bool {
		n := 0
		for _, v := range p {
			if v.Sign() != 0 {
				n++
				if new(big.Rat).Abs(v).Cmp(big.NewRat(1, 1)) != 0 {
					return false
				}
			}
		}
		return n == 1
	}
	knots := func(m, k Value) ([2]*big.Rat, error) {
		var a [2]*big.Rat
		if m.Kind != Aggregate || k.Kind != Aggregate || len(m.Items) != 2 || len(k.Items) != 2 {
			return a, fail()
		}
		for i := range a {
			n, e := integer(m.Items[i])
			if e != nil || n != 2 {
				return a, fail()
			}
			if _, e = wrappedIsoSourceNumber(k.Items[i]); e != nil {
				return a, e
			}
			a[i], e = liftSourceRat(k.Items[i])
			if e != nil {
				return a, e
			}
		}
		if a[0].Cmp(a[1]) >= 0 {
			return a, fail()
		}
		return a, nil
	}
	flags := func(r Record, inds ...int) bool {
		for _, i := range inds {
			b, e := boolean(r.Args[i])
			if e != nil || b {
				return false
			}
		}
		return true
	}
	enum := func(v Value, s string) bool { return v.Kind == Enumeration && v.Text == s }
	type domain struct{ u, v [2]*big.Rat }
	domains := map[int64]domain{}
	var surfaces []NURBSPlanarSurfaceEvidence
	order := append([]int64(nil), f.Order...)
	for _, id := range order {
		if e := ctx.Err(); e != nil {
			return nil, nil, e
		}
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "B_SPLINE_SURFACE_WITH_KNOTS" {
			continue
		}
		r, e := simple(f, id, "B_SPLINE_SURFACE_WITH_KNOTS", 13)
		if e != nil {
			return nil, nil, e
		}
		a, e := integer(r.Args[1])
		b, e2 := integer(r.Args[2])
		if e != nil || e2 != nil || a != 1 || b != 1 || !flags(r, 5, 6, 7) || !enum(r.Args[4], "UNSPECIFIED") || !enum(r.Args[12], "PIECEWISE_BEZIER_KNOTS") {
			return nil, nil, fail()
		}
		if _, e = textValue(r.Args[0]); e != nil {
			return nil, nil, e
		}
		u, e := knots(r.Args[8], r.Args[10])
		if e != nil {
			return nil, nil, e
		}
		v, e := knots(r.Args[9], r.Args[11])
		if e != nil {
			return nil, nil, e
		}
		if r.Args[3].Kind != Aggregate || len(r.Args[3].Items) != 2 {
			return nil, nil, fail()
		}
		var ids [4]int64
		var p [4][3]*big.Rat
		for i := 0; i < 2; i++ {
			row, e := listRefs(r.Args[3].Items[i], 2)
			if e != nil || len(row) != 2 {
				return nil, nil, fail()
			}
			for j := 0; j < 2; j++ {
				k := 2*i + j
				ids[k] = row[j]
				p[k], e = coord(ids[k], id)
				if e != nil {
					return nil, nil, e
				}
			}
		}
		var origin, du, dv [3]*big.Rat
		for j := 0; j < 3; j++ {
			du[j] = new(big.Rat).Quo(new(big.Rat).Sub(p[2][j], p[0][j]), new(big.Rat).Sub(u[1], u[0]))
			dv[j] = new(big.Rat).Quo(new(big.Rat).Sub(p[1][j], p[0][j]), new(big.Rat).Sub(v[1], v[0]))
			if new(big.Rat).Add(p[0][j], p[3][j]).Cmp(new(big.Rat).Add(p[1][j], p[2][j])) != 0 {
				return nil, nil, fail()
			}
			origin[j] = new(big.Rat).Sub(p[0][j], new(big.Rat).Add(new(big.Rat).Mul(u[0], du[j]), new(big.Rat).Mul(v[0], dv[j])))
		}
		if !unit(du) || !unit(dv) || mappedRationalDot(du, dv).Sign() != 0 {
			return nil, nil, fail()
		}
		// The support is owned only by its face and original pcurves.
		faces := 0
		var faceID int64
		for _, owner := range incoming[id] {
			if q, e := simple(f, owner, "ADVANCED_FACE", 4); e == nil && q.Args[2].Kind == Reference && q.Args[2].Ref == id {
				faces++
				faceID = owner
				continue
			}
			q, e := simple(f, owner, "PCURVE", 3)
			if e != nil || q.Args[1].Kind != Reference || q.Args[1].Ref != id {
				return nil, nil, fail()
			}
		}
		if faces != 1 {
			return nil, nil, fail()
		}
		proof := NURBSPlanarSurfaceEvidence{SurfaceID: id, FaceID: faceID, PoleIDs: ids}
		for i := 0; i < 2; i++ {
			proof.U[i] = u[i].RatString()
			proof.V[i] = v[i].RatString()
		}
		for j := 0; j < 3; j++ {
			proof.Origin[j] = origin[j].RatString()
			proof.DU[j] = du[j].RatString()
			proof.DV[j] = dv[j].RatString()
		}
		if e = putcoord(ids[0], "CARTESIAN_POINT", origin); e != nil {
			return nil, nil, e
		}
		if e = putcoord(ids[1], "DIRECTION", du); e != nil {
			return nil, nil, e
		}
		if e = putcoord(ids[2], "DIRECTION", mappedRationalCross(du, dv)); e != nil {
			return nil, nil, e
		}
		f.Entities[ids[3]] = Entity{ID: ids[3], Records: []Record{{Type: "AXIS2_PLACEMENT_3D", Args: []Value{{Kind: String}, refv(ids[0]), refv(ids[2]), refv(ids[1])}}}}
		f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "PLANE", Args: []Value{r.Args[0], refv(ids[3])}}}}
		domains[id] = domain{u, v}
		surfaces = append(surfaces, proof)
	}
	// Every original pcurve must stay inside its original finite chart rectangle.
	for _, id := range order {
		pc, e := simple(f, id, "PCURVE", 3)
		if e != nil {
			continue
		}
		sid, e := ref(pc.Args[1])
		d, ok := domains[sid]
		if e != nil || !ok {
			return nil, nil, fail()
		}
		rid, e := ref(pc.Args[2])
		if e != nil {
			return nil, nil, e
		}
		rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil {
			return nil, nil, e
		}
		cs, e := listRefs(rep.Args[1], 1)
		if e != nil || len(cs) != 1 {
			return nil, nil, fail()
		}
		c, e := simple(f, cs[0], "B_SPLINE_CURVE_WITH_KNOTS", 9)
		if e != nil {
			return nil, nil, e
		}
		ps, e := listRefs(c.Args[2], 2)
		if e != nil || len(ps) != 2 {
			return nil, nil, fail()
		}
		for _, pid := range ps {
			p, e := simple(f, pid, "CARTESIAN_POINT", 2)
			if e != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 2 {
				return nil, nil, fail()
			}
			for j, v := range p.Args[1].Items {
				x, e := liftSourceRat(v)
				if e != nil {
					return nil, nil, e
				}
				b := d.u
				if j == 1 {
					b = d.v
				}
				if x.Cmp(b[0]) < 0 || x.Cmp(b[1]) > 0 {
					return nil, nil, fail()
				}
			}
		}
	}
	owners := map[int64]map[int64]bool{}
	for _, id := range order {
		if _, e := simple(f, id, "ADVANCED_FACE", 4); e != nil {
			continue
		}
		face, e := readCircularSourceFace(f, id)
		if e != nil {
			return nil, nil, e
		}
		if _, ok := domains[face.surface]; !ok {
			return nil, nil, fail()
		}
		for _, loop := range face.loops {
			for _, u := range loop {
				if owners[u.id] == nil {
					owners[u.id] = map[int64]bool{}
				}
				if owners[u.id][face.surface] {
					return nil, nil, fail()
				}
				owners[u.id][face.surface] = true
			}
		}
	}
	var curves []NURBSPlanarCurveEvidence
	for _, id := range order {
		sc, e := simple(f, id, "SURFACE_CURVE", 4)
		if e != nil {
			continue
		}
		cid, e := ref(sc.Args[1])
		if e != nil {
			return nil, nil, e
		}
		if len(incoming[cid]) != 1 || incoming[cid][0] != id || len(incoming[id]) != 1 {
			return nil, nil, fail()
		}
		eid := incoming[id][0]
		edge, e := simple(f, eid, "EDGE_CURVE", 5)
		if e != nil || edge.Args[3].Kind != Reference || edge.Args[3].Ref != id {
			return nil, nil, fail()
		}
		pcs, e := listRefs(sc.Args[2], 2)
		if e != nil || len(pcs) != 2 || len(owners[eid]) != 2 {
			return nil, nil, fail()
		}
		used := map[int64]bool{}
		for _, pcid := range pcs {
			pc, e := simple(f, pcid, "PCURVE", 3)
			if e != nil {
				return nil, nil, e
			}
			sid, e := ref(pc.Args[1])
			if e != nil || !owners[eid][sid] || used[sid] {
				return nil, nil, fail()
			}
			used[sid] = true
		}
		c, e := simple(f, cid, "B_SPLINE_CURVE_WITH_KNOTS", 9)
		if e != nil {
			return nil, nil, e
		}
		deg, e := integer(c.Args[1])
		if e != nil || deg != 1 || !flags(c, 4, 5) || !enum(c.Args[3], "UNSPECIFIED") || !enum(c.Args[8], "PIECEWISE_BEZIER_KNOTS") {
			return nil, nil, fail()
		}
		k, e := knots(c.Args[6], c.Args[7])
		if e != nil {
			return nil, nil, e
		}
		ps, e := listRefs(c.Args[2], 2)
		if e != nil || len(ps) != 2 {
			return nil, nil, fail()
		}
		a, e := coord(ps[0], cid)
		if e != nil {
			return nil, nil, e
		}
		b, e := coord(ps[1], cid)
		if e != nil {
			return nil, nil, e
		}
		var o, d [3]*big.Rat
		for j := 0; j < 3; j++ {
			d[j] = new(big.Rat).Quo(new(big.Rat).Sub(b[j], a[j]), new(big.Rat).Sub(k[1], k[0]))
			o[j] = new(big.Rat).Sub(a[j], new(big.Rat).Mul(k[0], d[j]))
		}
		if !unit(d) {
			return nil, nil, fail()
		}
		proof := NURBSPlanarCurveEvidence{CurveID: cid, SurfaceCurveID: id, EdgeID: eid, PoleIDs: [2]int64{ps[0], ps[1]}, Knots: [2]string{k[0].RatString(), k[1].RatString()}}
		for j := 0; j < 3; j++ {
			proof.Origin[j] = o[j].RatString()
			proof.Derivative[j] = d[j].RatString()
		}
		// Exact authored vertex values must be the full model spline endpoints.
		same, e := boolean(edge.Args[4])
		if e != nil {
			return nil, nil, e
		}
		for i, want := range [][3]*big.Rat{a, b} {
			j := i
			if !same {
				j = 1 - i
			}
			vid, e := ref(edge.Args[1+j])
			if e != nil {
				return nil, nil, e
			}
			vr, e := simple(f, vid, "VERTEX_POINT", 2)
			if e != nil {
				return nil, nil, e
			}
			pid, e := ref(vr.Args[1])
			if e != nil {
				return nil, nil, e
			}
			pr, e := simple(f, pid, "CARTESIAN_POINT", 2)
			if e != nil || pr.Args[1].Kind != Aggregate || len(pr.Args[1].Items) != 3 {
				return nil, nil, fail()
			}
			for h, v := range pr.Args[1].Items {
				x, e := liftSourceRat(v)
				if e != nil || x.Cmp(want[h]) != 0 {
					return nil, nil, fail()
				}
			}
		}
		if e = putcoord(ps[0], "CARTESIAN_POINT", o); e != nil {
			return nil, nil, e
		}
		if e = putcoord(ps[1], "DIRECTION", d); e != nil {
			return nil, nil, e
		}
		vec := emit(Record{Type: "VECTOR", Args: []Value{{Kind: String}, refv(ps[1]), {Kind: Number, Text: "1."}}})
		f.Entities[cid] = Entity{ID: cid, Records: []Record{{Type: "LINE", Args: []Value{c.Args[0], refv(ps[0]), refv(vec)}}}}
		curves = append(curves, proof)
	}
	seenEdges := map[int64]bool{}
	for _, c := range curves {
		seenEdges[c.EdgeID] = true
	}
	for _, id := range order {
		if r, e := simple(f, id, "ADVANCED_FACE", 4); e == nil {
			sid, e := ref(r.Args[2])
			if _, ok := domains[sid]; e != nil || !ok {
				return nil, nil, fail()
			}
		}
		if _, e := simple(f, id, "EDGE_CURVE", 5); e == nil && !seenEdges[id] {
			return nil, nil, fail()
		}
	}
	return surfaces, curves, nil
}

func nurbsPlanarEmbeddingOptions() poly.Options {
	return poly.Options{RequireExactConversion: true, MaxInputTriangles: 512, MaxCollisionPairs: 32768, MaxWork: 250000, MaxBits: 4096}
}
