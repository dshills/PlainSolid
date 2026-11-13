package step

import (
	"bytes"
	"fmt"
	"io"
	"plainsolid/geom"
	"unicode/utf8"
)

// WritePartitionedSplineLensBody splits the shared conic at exact rational knot
// events from both original charts. Nonrepresentable coefficients fail before
// output; no rounded parameter cut or geometric approximation is emitted.
func WritePartitionedSplineLensBody(w io.Writer, m SplineLensBody, o CurvedOvalOptions) error {
	return writePartitionedSplineLensBody(w, m, o, nil)
}
func writePartitionedSplineLensBody(w io.Writer, m SplineLensBody, o CurvedOvalOptions, refined [][3]*geom.BSpline, raw ...[][3]RationalLensCurve) error {
	var rawCurves [][3]RationalLensCurve
	if len(raw) > 0 {
		rawCurves = raw[0]
	}
	hasRefined := refined != nil || rawCurves != nil
	if w == nil || !validLengthUnit(m.LengthUnit) || !utf8.ValidString(m.Name) || !utf8.ValidString(m.FaceNames[0]) || !utf8.ValidString(m.FaceNames[1]) {
		return geom.ErrInvalid
	}
	var e error
	o, e = o.defaults()
	if e != nil {
		return e
	}
	if _, e = certifySplineLens(m, o); e != nil {
		return e
	}
	pieces, e := lensPiecePlan(m, o)
	if e != nil {
		return e
	}
	var refinedFiles []*File
	var refinedRoots [][3]int64
	if rawCurves != nil {
		if len(rawCurves) != len(pieces) {
			return geom.ErrInvalid
		}
		for _, cs := range rawCurves {
			f, ids, e := stageRationalLensCurves(cs, m.LengthUnit, o)
			if e != nil {
				return e
			}
			refinedFiles = append(refinedFiles, f)
			refinedRoots = append(refinedRoots, ids)
		}
	} else if refined != nil {
		if len(refined) != len(pieces) {
			return geom.ErrInvalid
		}
		for _, cs := range refined {
			f, ids, e := stageRefinedLensCurves(cs, m.LengthUnit, o)
			if e != nil {
				return e
			}
			refinedFiles = append(refinedFiles, f)
			refinedRoots = append(refinedRoots, ids)
		}
	}

	type data struct {
		up, mp []geom.Vec3
		uw, mw []float64
		start  geom.Vec3
	}
	encoded := make([]data, len(pieces))
	for i, p := range pieces {
		encoded[i].up, encoded[i].uw, e = exactBezierFloats(p.UV, o.patch())
		if e != nil {
			return e
		}
		encoded[i].mp, encoded[i].mw, e = exactBezierFloats(p.Model, o.patch())
		if e != nil {
			return e
		}
		encoded[i].start, e = p.Model.ExactPoint(0)
		if e != nil {
			return e
		}
	}
	var supports bytes.Buffer
	if e = WriteSurfaces(&supports, SurfaceModel{LengthUnit: m.LengthUnit, Surfaces: []SplineSurface{{Geometry: m.Upper, ClosedU: "F", ClosedV: "F", SelfIntersect: "F"}, {Geometry: m.Lower, ClosedU: "F", ClosedV: "F", SelfIntersect: "F"}}}); e != nil {
		return e
	}
	source, e := Parse(&supports, Limits{})
	if e != nil {
		return e
	}
	roots := []int64{}
	for _, id := range source.Order {
		if _, ok := component(source.Entities[id], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
			roots = append(roots, id)
		}
	}
	if len(roots) != 2 {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	e = writeStandalone(&out, m.LengthUnit, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		surfaces := copySplineGeometry(emit, source, roots)
		ctx := emit("(GEOMETRIC_REPRESENTATION_CONTEXT(2) PARAMETRIC_REPRESENTATION_CONTEXT() REPRESENTATION_CONTEXT('',''))")
		vertices := []int64{}
		for _, d := range encoded {
			p := d.start
			pid := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
			vertices = append(vertices, emit(fmt.Sprintf("VERTEX_POINT('',#%d)", pid)))
		}
		var uses [2][]int64
		for i, d := range encoded {
			var copied []int64
			if hasRefined {
				copied = copySplineGeometry(emit, refinedFiles[i], refinedRoots[i][:])
			}
			mid := int64(0)
			if hasRefined {
				mid = copied[0]
			} else {
				mid = emitCurvedOvalBezier(emit, d.mp, d.mw, false)
			}
			pcs := []int64{}
			for owner := 0; owner < 2; owner++ {
				uid := int64(0)
				if hasRefined {
					uid = copied[owner+1]
				} else {
					uid = emitCurvedOvalBezier(emit, d.up, d.uw, true)
				}
				rep := emit(fmt.Sprintf("DEFINITIONAL_REPRESENTATION('',(#%d),#%d)", uid, ctx))
				pcs = append(pcs, emit(fmt.Sprintf("PCURVE('',#%d,#%d)", surfaces[owner], rep)))
			}
			sc := emit(fmt.Sprintf("SURFACE_CURVE('',#%d,%s,.PCURVE_S1.)", mid, references(pcs)))
			edge := emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertices[i], vertices[(i+1)%len(vertices)], sc))
			for owner := 0; owner < 2; owner++ {
				uses[owner] = append(uses[owner], emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,.T.)", edge)))
			}
		}
		faces := []int64{}
		for owner := 0; owner < 2; owner++ {
			sense := "T"
			if owner == 1 {
				sense = "F"
			}
			loop := emit("EDGE_LOOP(''," + references(uses[owner]) + ")")
			bound := emit(fmt.Sprintf("FACE_OUTER_BOUND('',#%d,.%s.)", loop, sense))
			faces = append(faces, emit(fmt.Sprintf("ADVANCED_FACE(%s,(#%d),#%d,.%s.)", quote(m.FaceNames[owner]), bound, surfaces[owner], sense)))
		}
		shell := emit("CLOSED_SHELL(''," + references(faces) + ")")
		return []int64{emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(m.Name), shell))}
	})
	if e != nil {
		return e
	}
	if _, e = readSplineLensBody(bytes.NewReader(out.Bytes()), Limits{}, o, true, hasRefined, rawCurves != nil); e != nil {
		return fmt.Errorf("step: staged lens verification: %w", e)
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		return io.ErrShortWrite
	}
	return e
}
