package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"plainsolid/geom"
)

// WriteWrappedSplineFaces builds one full fundamental face per wrapped support.
// Seam and corner identities are derived from certified wrapped axes, rather
// than copied from external IDs. It emits separate surface models, never solids.
// Exact model curve coefficients and endpoints must be representable.
func WriteWrappedSplineFaces(w io.Writer, m WrappedSurfaceModel) error {
	if w == nil || len(m.Surfaces) == 0 || len(m.Surfaces) > 64 {
		return errors.New("step: invalid wrapped face model")
	}
	type staged struct {
		sf, cf  *File
		sid     int64
		cids    []int64
		corners [4]geom.Vec3
		d       geom.ParameterRectangle
		wu, wv  bool
	}
	parts := make([]staged, len(m.Surfaces))
	for i, s := range m.Surfaces {
		if s.Geometry == nil {
			return geom.ErrInvalid
		}
		p := &parts[i]
		patch := s.Geometry.FundamentalPatch()
		p.d = patch.Domain()
		periods := s.Geometry.Periods()
		p.wu, p.wv = periods[0] != nil, periods[1] != nil
		var b bytes.Buffer
		if e := WriteWrappedSurfaces(&b, WrappedSurfaceModel{Surfaces: []WrappedSplineSurface{s}, LengthUnit: m.LengthUnit}); e != nil {
			return e
		}
		var e error
		p.sf, e = Parse(&b, Limits{})
		if e != nil {
			return e
		}
		for _, id := range p.sf.Order {
			if _, ok := component(p.sf.Entities[id], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
				p.sid = id
			}
		}
		cm := CurveModel{LengthUnit: m.LengthUnit}
		for j := 0; j < 4; j++ {
			axis, fixed := 1, p.d.VMin
			switch j {
			case 1:
				axis, fixed = 0, p.d.UMax
			case 2:
				axis, fixed = 1, p.d.VMax
			case 3:
				axis, fixed = 0, p.d.UMin
			}
			c, e := patch.IsoCurve(axis, fixed, geom.SurfacePatchOptions{})
			if e != nil {
				return e
			}
			spline, e := c.ToBSpline()
			if e != nil {
				return e
			}
			closed := "F"
			if (axis == 1 && p.wu) || (axis == 0 && p.wv) {
				closed = "T"
			}
			cm.Curves = append(cm.Curves, SplineCurve{Geometry: spline, Closed: closed, SelfIntersect: "U"})
			lo, hi := c.Domain()
			t := lo
			if j >= 2 {
				t = hi
			}
			p.corners[j], e = c.ExactPoint(t)
			if e != nil {
				return e
			}
		}
		if e = WriteCurves(&b, cm); e != nil {
			return e
		}
		p.cf, e = Parse(&b, Limits{})
		if e != nil {
			return e
		}
		for _, id := range p.cf.Order {
			if _, ok := component(p.cf.Entities[id], "B_SPLINE_CURVE_WITH_KNOTS"); ok {
				p.cids = append(p.cids, id)
			}
		}
		if p.sid == 0 || len(p.cids) != 4 {
			return errors.New("step: invalid staged wrapped geometry")
		}
	}
	var b bytes.Buffer
	e := writeStandalone(&b, m.LengthUnit, "", "MANIFOLD_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
		roots := []int64{}
		for i, p := range parts {
			sid := copySplineGeometry(emit, p.sf, []int64{p.sid})[0]
			curves := copySplineGeometry(emit, p.cf, p.cids)
			canonical := func(j int) int {
				u, v := []int{0, 1, 1, 0}[j], []int{0, 0, 1, 1}[j]
				if p.wu {
					u = 0
				}
				if p.wv {
					v = 0
				}
				return 2*v + u
			}
			var vertices, pcs, edges [4]int64
			vmap := map[int]int64{}
			for j, point := range p.corners {
				k := canonical(j)
				if vmap[k] == 0 {
					pt := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(point.X), real(point.Y), real(point.Z)))
					vmap[k] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", pt))
				}
				vertices[j] = vmap[k]
			}
			for j := 0; j < 4; j++ {
				u, v, dx, dy := 0., p.d.VMin, 1., 0.
				switch j {
				case 1:
					u, v, dx, dy = p.d.UMax, 0, 0, 1
				case 2:
					v = p.d.VMax
				case 3:
					u, v, dx, dy = p.d.UMin, 0, 0, 1
				}
				pt := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s))", real(u), real(v)))
				dir := emit(fmt.Sprintf("DIRECTION('',(%s,%s))", real(dx), real(dy)))
				vec := emit(fmt.Sprintf("VECTOR('',#%d,1.)", dir))
				line := emit(fmt.Sprintf("LINE('',#%d,#%d)", pt, vec))
				rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", line, ctx))
				pcs[j] = emit(fmt.Sprintf("PCURVE('',#%d,#%d)", sid, rep))
			}
			ends := [4][2]int{{0, 1}, {1, 2}, {3, 2}, {0, 3}}
			for j := 0; j < 4; j++ {
				if j == 2 && p.wv {
					edges[j] = edges[0]
					continue
				}
				if j == 3 && p.wu {
					edges[j] = edges[1]
					continue
				}
				kind := "SURFACE_CURVE"
				pc := []int64{pcs[j]}
				if j == 0 && p.wv || j == 1 && p.wu {
					kind = "SEAM_CURVE"
					pc = append(pc, pcs[j+2])
				}
				sc := emit(fmt.Sprintf("%s('',#%d,%s,.PCURVE_S1.)", kind, curves[j], references(pc)))
				edges[j] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[ends[j][0]], vertices[ends[j][1]], sc))
			}
			uses := []int64{}
			for j, edge := range edges {
				sense := ".T."
				if j >= 2 {
					sense = ".F."
				}
				uses = append(uses, emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edge, sense)))
			}
			loop := emit("EDGE_LOOP(''," + references(uses) + ")")
			bound := emit(fmt.Sprintf("FACE_OUTER_BOUND('',#%d,.T.)", loop))
			face := emit(fmt.Sprintf("ADVANCED_FACE(%s,(#%d),#%d,.T.)", quote(m.Surfaces[i].Name), bound, sid))
			shell := emit(fmt.Sprintf("OPEN_SHELL('',(#%d))", face))
			roots = append(roots, emit(fmt.Sprintf("SHELL_BASED_SURFACE_MODEL('',(#%d))", shell)))
		}
		return roots
	})
	if e != nil {
		return e
	}
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		return e
	}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) == 1 && ent.Records[0].Type == "ADVANCED_FACE" {
			if _, e = DecodeWrappedSplineFace(context.Background(), f, id, WrappedSurfaceOptions{}); e != nil {
				return e
			}
		}
	}
	n, e := w.Write(b.Bytes())
	if e == nil && n != b.Len() {
		e = io.ErrShortWrite
	}
	return e
}
