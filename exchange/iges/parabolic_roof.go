package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ParabolicRoofDescriptor describes an axis-aligned box below
// z=Origin.Z+Height+Rise*((x-Origin.X)/Width)^2. All lengths are model units.
// Width, Depth, Height and Rise are strictly positive; translation is explicit.
type ParabolicRoofDescriptor struct {
	Origin                     geom.Vec3
	Width, Depth, Height, Rise float64
}
type roofRoles struct {
	surfaces, faces               [6]int
	models                        [12]int
	pc                            [6][4]int
	solid, shell, vertices, edges int
}

// ParabolicRoofSolid preserves the original canonical-role IGES graph and a
// certified native curved material; source geometry is never fitted.
type ParabolicRoofSolid struct {
	Descriptor                                                         ParabolicRoofDescriptor
	LengthUnit                                                         float64
	Solid                                                              trimmed.Solid
	Geometry                                                           map[int]Entity
	Metadata                                                           map[int]CylinderDatumMetadata
	SourceSurfaceIDs                                                   [6]int
	SourceModelCurveIDs                                                [12]int
	SourcePCurveIDs                                                    [6][4]int
	SourceFaceIDs                                                      [6]int
	SourceSolidID, SourceShellID, SourceVertexListID, SourceEdgeListID int
	Embedding                                                          trimmed.SplineRectangleEmbedding
}

func roofFailure() error { return fmt.Errorf("iges: unsupported exact canonical parabolic roof solid") }
func roofAuthor(d ParabolicRoofDescriptor) (topologyAuthor, roofRoles, error) {
	var a topologyAuthor
	var roles roofRoles
	if !d.Origin.IsFinite() {
		return a, roles, roofFailure()
	}
	for _, x := range []float64{d.Width, d.Depth, d.Height, d.Rise} {
		if !finite(x) || x <= 0 {
			return a, roles, roofFailure()
		}
	}
	scalar := func(origin float64, terms ...float64) (float64, error) {
		r := tetraRat(origin)
		for i := 0; i < len(terms); i += 2 {
			r.Add(r, new(big.Rat).Mul(tetraRat(terms[i]), tetraRat(terms[i+1])))
		}
		x, exact := r.Float64()
		if !exact {
			return 0, geom.ErrUnrepresentable
		}
		return x, nil
	}
	point := func(u, v, h, k float64) (geom.Vec3, error) {
		x, e := scalar(d.Origin.X, d.Width, u)
		if e != nil {
			return geom.Vec3{}, e
		}
		y, e := scalar(d.Origin.Y, d.Depth, v)
		if e != nil {
			return geom.Vec3{}, e
		}
		z, e := scalar(d.Origin.Z, d.Height, h, d.Rise, k)
		return geom.Vec3{X: x, Y: y, Z: z}, e
	}
	var vertices [8]geom.Vec3
	vp := []string{"502", "8"}
	for i := range vertices {
		p, e := point(float64(i&1), float64((i>>1)&1), float64(i>>2), float64((i>>2)*(i&1)))
		if e != nil {
			return a, roles, e
		}
		vertices[i] = p
		vp = append(vp, number(p.X), number(p.Y), number(p.Z))
	}
	roles.vertices = a.add(502, 1, joinTopologyValues(vp))
	curve := func(p []geom.Vec3) (int, error) {
		k := []float64{0, 0, 1, 1}
		if len(p) == 3 {
			k = []float64{0, 0, 0, 1, 1, 1}
		}
		g, e := geom.NewBSpline(len(p)-1, p, k, nil)
		if e != nil {
			return 0, e
		}
		return a.entity(Entity{Spline: &Spline{Curve: g, Start: 0, End: 1, Polynomial: true}}), nil
	}
	edges := [][2]int{}
	for i := 0; i < 8; i++ {
		for _, bit := range []int{1, 2, 4} {
			if j := i ^ bit; j > i {
				edges = append(edges, [2]int{i, j})
			}
		}
	}
	ev := []int{504, 12}
	for i, e := range edges {
		p := []geom.Vec3{vertices[e[0]], vertices[e[1]]}
		if e[0] >= 4 && e[1]-e[0] == 1 {
			mid, err := point(.5, float64((e[0]>>1)&1), 1, 0)
			if err != nil {
				return a, roles, err
			}
			p = []geom.Vec3{p[0], mid, p[1]}
		}
		id, err := curve(p)
		if err != nil {
			return a, roles, err
		}
		roles.models[i] = id
		ev = append(ev, id, roles.vertices, e[0]+1, roles.vertices, e[1]+1)
	}
	roles.edges = a.add(504, 1, topologyNumbers(ev...))
	quads := [][4]int{{0, 2, 3, 1}, {4, 5, 7, 6}, {0, 1, 5, 4}, {2, 6, 7, 3}, {0, 4, 6, 2}, {1, 3, 7, 5}}
	fv := []int{514, 6}
	for fi, q := range quads {
		cu, cv := 2, 2
		if fi == 1 || fi == 2 {
			cu = 3
		}
		if fi == 3 {
			cv = 3
		}
		poles := make([]geom.Vec3, cu*cv)
		for u := 0; u < cu; u++ {
			for v := 0; v < cv; v++ {
				var x, y, h, k float64
				switch fi {
				case 0:
					x, y = float64(v), float64(u)
				case 1:
					x, y, h = float64(u)/2, float64(v), 1
					if u == 2 {
						k = 1
					}
				case 2:
					x, h = float64(u)/2, float64(v)
					if u == 2 {
						k = float64(v)
					}
				case 3:
					x, y, h = float64(v)/2, 1, float64(u)
					if v == 2 {
						k = float64(u)
					}
				case 4:
					y, h = float64(v), float64(u)
				case 5:
					x, y, h, k = 1, float64(u), float64(v), float64(v)
				}
				p, e := point(x, y, h, k)
				if e != nil {
					return a, roles, e
				}
				poles[u*cv+v] = p
			}
		}
		ku, kv := []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}
		if cu == 3 {
			ku = []float64{0, 0, 0, 1, 1, 1}
		}
		if cv == 3 {
			kv = []float64{0, 0, 0, 1, 1, 1}
		}
		g, e := geom.NewBSplineSurface(cu-1, cv-1, cu, cv, poles, ku, kv, nil)
		if e != nil {
			return a, roles, e
		}
		roles.surfaces[fi] = a.entity(Entity{Surface: &Surface{Geometry: g, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1, Polynomial: true}})
		uv := map[int]geom.Vec3{q[0]: {}, q[1]: {X: 1}, q[2]: {X: 1, Y: 1}, q[3]: {Y: 1}}
		lv := []int{508, 4}
		for j, start := range q {
			end := q[(j+1)%4]
			for i, edge := range edges {
				if min(start, end) == edge[0] && max(start, end) == edge[1] {
					sense := 0
					if start == edge[0] {
						sense = 1
					}
					pc, e := curve([]geom.Vec3{uv[edge[0]], uv[edge[1]]})
					if e != nil {
						return a, roles, e
					}
					roles.pc[fi][j] = pc
					lv = append(lv, 0, roles.edges, i+1, sense, 1, 0, pc)
				}
			}
		}
		loop := a.add(508, 1, topologyNumbers(lv...))
		roles.faces[fi] = a.add(510, 1, topologyNumbers(510, roles.surfaces[fi], 1, 1, loop))
		fv = append(fv, roles.faces[fi], 1)
	}
	roles.shell = a.add(514, 1, topologyNumbers(fv...))
	roles.solid = a.add(186, 0, topologyNumbers(186, roles.shell, 1, 0))
	return a, roles, nil
}

// ReadParabolicRoofSolid requires the documented canonical role order and exact
// polynomial source coefficients. Dimensions are recovered from actual source
// vertices, not a recipe marker. Original charts still require whole-curve
// model/pcurve identity and a complete native global material certificate.
func ReadParabolicRoofSolid(ctx context.Context, r io.Reader, l Limits) (ParabolicRoofSolid, error) {
	fail := func() (ParabolicRoofSolid, error) { return ParabolicRoofSolid{}, roofFailure() }
	if ctx == nil {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return ParabolicRoofSolid{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 58)
	l.MaxRecords = min(l.MaxRecords, 1024)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxControlPoints = min(l.MaxControlPoints, 104)
	doc, e := readTopologySource(r, l, true)
	if e != nil {
		return ParabolicRoofSolid{}, e
	}
	if len(doc.entities) != 58 || doc.entities[0].kind != 502 || l.MaxControlPoints < 104 {
		return fail()
	}
	// Appearance/level references are outside this material-only dialect.
	for _, line := range bytes.Split(doc.bytes, []byte("\n")) {
		if len(line) == 80 && line[72] == 'D' {
			seq, err := field(string(line[73:]))
			if err != nil {
				return fail()
			}
			indexes := []int{1, 2}
			if seq%2 == 1 {
				kind, err := field(string(line[:8]))
				if err != nil {
					return fail()
				}
				status, err := field(string(line[64:72]))
				if err != nil {
					return fail()
				}
				want := 10000
				if kind == 186 {
					want = 0
				}
				if kind == 504 {
					want = 10001
				}
				if status != want {
					return fail()
				}
				indexes = []int{3, 4}
			}
			for _, j := range indexes {
				v, err := field(string(line[j*8 : j*8+8]))
				if err != nil || v != 0 {
					return fail()
				}
			}
		}
	}
	vr := doc.entities[0]
	if len(vr.values) < 26 || noPointers(vr.values[26:]) != nil {
		return fail()
	}
	for _, p := range doc.entities {
		if e = compositeSourceNumbers(110, p.values); e != nil {
			return fail()
		}
	}
	var v [8]geom.Vec3
	for i := range v {
		var xyz [3]float64
		for j := range xyz {
			xyz[j], e = realValue(vr.values[2+3*i+j])
			if e != nil {
				return fail()
			}
		}
		v[i] = geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	}
	diff := func(a, b float64) (float64, error) {
		x, exact := new(big.Rat).Sub(tetraRat(a), tetraRat(b)).Float64()
		if !exact {
			return 0, geom.ErrUnrepresentable
		}
		return x, nil
	}
	d := ParabolicRoofDescriptor{Origin: v[0]}
	d.Width, e = diff(v[1].X, v[0].X)
	if e != nil {
		return fail()
	}
	d.Depth, e = diff(v[2].Y, v[0].Y)
	if e != nil {
		return fail()
	}
	d.Height, e = diff(v[4].Z, v[0].Z)
	if e != nil {
		return fail()
	}
	d.Rise, e = diff(v[5].Z, v[4].Z)
	if e != nil {
		return fail()
	}
	expected, roles, e := roofAuthor(d)
	if e != nil {
		return fail()
	}
	geometry := map[int]Entity{}
	metadata := map[int]CylinderDatumMetadata{}
	for i, source := range doc.entities {
		want := expected.records[i]
		if source.id != 2*i+1 || source.kind != want.kind {
			return fail()
		}
		data := want.data
		if source.kind == 126 || source.kind == 128 {
			en, err := decode(source.kind, source.values, l.MaxControlPoints)
			if err != nil || en.sourceWeightsRounded {
				return fail()
			}
			if err = en.validate(); err != nil {
				return fail()
			}
			_, actual := encode(en)
			if actual != data {
				return fail()
			}
			geometry[source.id] = en
		} else {
			budget := 4096
			values, err := parseParameters(data, ',', ';', &budget)
			if err != nil {
				return fail()
			}
			if len(source.values) < len(values) || noPointers(source.values[len(values):]) != nil {
				return fail()
			}
			for j, p := range values {
				if source.kind != 502 || j < 2 {
					x, err := intValue(p)
					if err != nil {
						return fail()
					}
					y, err := intValue(source.values[j])
					if err != nil || x != y {
						return fail()
					}
					continue
				}
				x, err := realValue(p)
				if err != nil {
					return fail()
				}
				y, err := realValue(source.values[j])
				if err != nil || x != y {
					return fail()
				}
			}
		}
		metadata[source.id] = CylinderDatumMetadata{source.label, source.subscript}
	}
	surfaces := make([]*geom.BSplineSurface, 6)
	options := curvedOvalOptions()
	for fi, sid := range roles.surfaces {
		if e = ctx.Err(); e != nil {
			return ParabolicRoofSolid{}, e
		}
		surfaces[fi] = geometry[sid].Surface.Geometry
		patch, e := geom.NewSurfacePatch(surfaces[fi], options)
		if e != nil {
			return fail()
		}
		// Canonical loop role order binds each independent owner pcurve to its
		// source authoritative model edge; no near-endpoint lookup is used.
		loop := doc.entities[(roles.faces[fi]-1)/2-1]
		lv, e := topologyInts(loop, 30)
		if e != nil {
			return fail()
		}
		for j, pcid := range roles.pc[fi] {
			idx := lv[4+7*j] - 1
			uv, e := geom.NewExactSplineCurve(geometry[pcid].Spline.Curve, options)
			if e != nil {
				return fail()
			}
			lift, e := patch.ComposeParameterCurve(ctx, uv, options)
			if e != nil {
				return ParabolicRoofSolid{}, e
			}
			model, e := geom.NewExactSplineCurve(geometry[roles.models[idx]].Spline.Curve, options)
			if e != nil {
				return fail()
			}
			proof, e := geom.CompareExactCurvesContext(ctx, lift, model, geom.ExactCurveComparisonOptions{})
			if e != nil {
				return ParabolicRoofSolid{}, e
			}
			if !proof.Complete || !proof.Equal {
				return fail()
			}
		}
	}
	solid, _, e := trimmed.SewSplineRectangles(ctx, surfaces, make([]geom.ParameterRectangle, 6), make([]bool, 6), trimmed.SewOptions{Tolerance: 1e-10, MaxFaces: 6, MaxUses: 24})
	if e != nil {
		return ParabolicRoofSolid{}, e
	}
	embedding, e := solid.ValidateSplineRectangleEmbedding(ctx, trimmed.SplineRectangleEmbeddingOptions{MaxFaces: 6})
	if e != nil {
		return ParabolicRoofSolid{}, e
	}
	if !embedding.Complete {
		return fail()
	}
	return ParabolicRoofSolid{Descriptor: d, LengthUnit: doc.unit, Solid: solid, Geometry: geometry, Metadata: metadata, SourceFaceIDs: roles.faces, SourceSurfaceIDs: roles.surfaces, SourceModelCurveIDs: roles.models, SourcePCurveIDs: roles.pc, SourceSolidID: roles.solid, SourceShellID: roles.shell, SourceVertexListID: roles.vertices, SourceEdgeListID: roles.edges, Embedding: embedding}, nil
}

// WriteParabolicRoofSolid emits the exact canonical graph only after the same
// original-chart source and material proofs pass. No BRep approximation is made.
func WriteParabolicRoofSolid(ctx context.Context, w io.Writer, d ParabolicRoofDescriptor, unit float64) error {
	if ctx == nil || w == nil {
		return roofFailure()
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	a, _, e := roofAuthor(d)
	if e != nil {
		return e
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadParabolicRoofSolid(ctx, bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
