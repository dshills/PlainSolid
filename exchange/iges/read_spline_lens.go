package iges

import (
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
)

// ReadSplineLensSolid validates one type186 manifold solid with a closed shell,
// opposite graph-face orientations, four common conic edges and exact material.
func ReadSplineLensSolid(r io.Reader, l Limits) (SplineLensSolid, error) {
	d, e := readTopologySource(r, l, true)
	if e != nil {
		return SplineLensSolid{}, e
	}
	return readIGESLens(d, l)
}
func readIGESLens(d topologySource, l Limits, rawMode ...bool) (SplineLensSolid, error) {
	raw := len(rawMode) > 0 && rawMode[0]
	partitioned := len(rawMode) > 1 && rawMode[1]
	affine := len(rawMode) > 2 && rawMode[2]
	n := 4
	if partitioned {
		n = (len(d.entities) - 10) / 3
	}
	if n < 4 || n > 32 {
		return SplineLensSolid{}, geom.ErrInvalid
	}
	var pieces []LensSeamPiece
	rawCurves := make([][3]RationalLensCurve, n)
	modelIDs := make([]int, n)
	ownerIDs := [2][]int{make([]int, n), make([]int, n)}
	exacts := map[int]*geom.ExactSplineCurve{}
	descriptors := map[int]RationalLensCurve{}
	fail := func() (SplineLensSolid, error) {
		return SplineLensSolid{}, fmt.Errorf("iges: unsupported or inconsistent closed spline lens")
	}
	if n < 4 || n > 32 || len(d.entities) != 10+3*n {
		return fail()
	}
	limits, e := l.defaults()
	if e != nil {
		return SplineLensSolid{}, e
	}
	m := SplineLensSolid{LengthUnit: d.unit}
	records := map[int]topologyRecord{}
	types := map[int][]int{}
	geometry := map[int]Entity{}
	used := map[int]bool{}
	controls := 0
	for _, r := range d.entities {
		records[r.id] = r
		types[r.kind] = append(types[r.kind], r.id)
		if r.kind == 126 || r.kind == 128 {
			var en Entity
			var e error
			if raw && r.kind == 126 {
				var c RationalLensCurve
				var x *geom.ExactSplineCurve
				c, x, en, e = decodeRationalIGESCurve(r, limits.MaxControlPoints-controls)
				exacts[r.id] = x
				descriptors[r.id] = c
			} else {
				en, e = decode(r.kind, r.values, limits.MaxControlPoints-controls)
			}
			if e != nil {
				return SplineLensSolid{}, e
			}
			if en.sourceWeightsRounded {
				return SplineLensSolid{}, geom.ErrUnrepresentable
			}
			if e = en.validate(); e != nil {
				return SplineLensSolid{}, e
			}
			if en.Surface != nil {
				controls += len(en.Surface.Geometry.Data().Poles)
			} else {
				controls += len(en.Spline.Curve.Poles())
			}
			geometry[r.id] = en
		}
	}
	for kind, n := range map[int]int{186: 1, 514: 1, 510: 2, 508: 2, 504: 1, 502: 1, 128: 2, 126: 3 * n} {
		if len(types[kind]) != n {
			return fail()
		}
	}
	get := func(id, kind int) (topologyRecord, bool) {
		r, ok := records[id]
		if !ok || r.kind != kind {
			return r, false
		}
		used[id] = true
		return r, true
	}
	m.SourceSolidID = types[186][0]
	r, _ := get(m.SourceSolidID, 186)
	v, e := topologyInts(r, 4)
	if e != nil || v[2] != 1 || v[3] != 0 {
		return fail()
	}
	m.SourceShellID = v[1]
	r, ok := get(v[1], 514)
	if !ok {
		return fail()
	}
	v, e = topologyInts(r, 6)
	if e != nil || v[1] != 2 || v[3] != 1 || v[5] != 0 || v[2] == v[4] {
		return fail()
	}
	m.SourceFaceIDs = [2]int{v[2], v[4]}
	m.SourceEdgeListID = types[504][0]
	m.SourceVertexListID = types[502][0]
	r, _ = get(m.SourceVertexListID, 502)
	if len(r.values) < 2+3*n {
		return fail()
	}
	if n, e := intValue(r.values[1]); e != nil || n != len(modelIDs) {
		return fail()
	}
	if k, e := intValue(r.values[0]); e != nil || k != 502 {
		return fail()
	}
	if e = noPointers(r.values[2+3*n:]); e != nil {
		return fail()
	}
	vertices := make([]geom.Vec3, n)
	unique := map[geom.Vec3]bool{}
	for i := 0; i < n; i++ {
		var p [3]float64
		for j := 0; j < 3; j++ {
			p[j], e = realValue(r.values[2+3*i+j])
			if e != nil {
				return fail()
			}
		}
		vertices[i] = geom.Vec3{X: p[0], Y: p[1], Z: p[2]}
		if unique[vertices[i]] {
			return fail()
		}
		unique[vertices[i]] = true
	}
	r, _ = get(m.SourceEdgeListID, 504)
	v, e = topologyInts(r, 2+5*n)
	if e != nil || v[1] != n {
		return fail()
	}
	modelSeen := map[int]bool{}
	models := make([]*geom.ExactSplineCurve, n)
	for i := 0; i < n; i++ {
		a := v[2+5*i : 7+5*i]
		if a[1] != m.SourceVertexListID || a[3] != m.SourceVertexListID || a[2] != i+1 || a[4] != (i+1)%n+1 || modelSeen[a[0]] {
			return fail()
		}
		modelSeen[a[0]] = true
		if _, ok = get(a[0], 126); !ok {
			return fail()
		}
		c := geometry[a[0]].Spline
		if c == nil || c.Curve.Degree() > 16 || (!raw && len(c.Curve.Poles()) > 32) || len(c.Curve.Poles()) > 64 {
			return fail()
		}
		x, e := geom.NewExactSplineCurve(c.Curve, curvedOvalOptions())
		if raw {
			x = exacts[a[0]]
			e = nil
		}
		if e != nil {
			return fail()
		}
		start, e := x.ExactPoint(c.Start)
		if e != nil {
			return fail()
		}
		end, e := x.ExactPoint(c.End)
		if e != nil || start != vertices[i] || end != vertices[(i+1)%n] {
			return fail()
		}
		x, e = x.Restrict(c.Start, c.End, curvedOvalOptions())
		if e != nil {
			return fail()
		}
		models[i] = x
		modelIDs[i] = a[0]
		if !partitioned {
			m.SourceModelCurveIDs[i] = a[0]
		}
		if raw {
			rawCurves[i][0] = descriptors[a[0]]
		}
	}
	ownerUV := [2][]*geom.ExactSplineCurve{make([]*geom.ExactSplineCurve, n), make([]*geom.ExactSplineCurve, n)}
	var supports [2]*Surface
	seenUV := map[int]bool{}
	seenSurface := map[int]bool{}
	seenLoop := map[int]bool{}
	for f, id := range m.SourceFaceIDs {
		r, ok = get(id, 510)
		if !ok {
			return fail()
		}
		v, e = topologyInts(r, 5)
		if e != nil || v[2] != 1 || v[3] != 1 || seenSurface[v[1]] || seenLoop[v[4]] {
			return fail()
		}
		seenSurface[v[1]] = true
		seenLoop[v[4]] = true
		if _, ok = get(v[1], 128); !ok {
			return fail()
		}
		supports[f] = geometry[v[1]].Surface
		if supports[f] == nil {
			return fail()
		}
		r, ok = get(v[4], 508)
		if !ok {
			return fail()
		}
		lv, e := topologyInts(r, 2+7*n)
		if e != nil || lv[1] != n {
			return fail()
		}
		for i := 0; i < n; i++ {
			a := lv[2+7*i : 9+7*i]
			if a[0] != 0 || a[1] != m.SourceEdgeListID || a[2] != i+1 || a[3] != 1 || a[4] != 1 || a[5] != 0 || seenUV[a[6]] || modelSeen[a[6]] {
				return fail()
			}
			seenUV[a[6]] = true
			if _, ok = get(a[6], 126); !ok {
				return fail()
			}
			c := geometry[a[6]].Spline
			if c == nil || c.Curve.Degree() != 2 || (!raw && len(c.Curve.Poles()) != 3) || (raw && len(c.Curve.Poles()) > 64) {
				return fail()
			}
			for _, p := range c.Curve.Poles() {
				s := supports[f]
				if p.Z != 0 || p.X < s.UStart || p.X > s.UEnd || p.Y < s.VStart || p.Y > s.VEnd {
					return fail()
				}
			}
			x, e := geom.NewExactSplineCurve(c.Curve, curvedOvalOptions())
			if raw {
				x = exacts[a[6]]
				e = nil
			}
			if e != nil {
				return fail()
			}
			x, e = x.Restrict(c.Start, c.End, curvedOvalOptions())
			if e != nil {
				return fail()
			}
			x, e = x.Reparameterize(0, 1, false, curvedOvalOptions())
			if e != nil {
				return fail()
			}
			ownerUV[f][i] = x
			ownerIDs[f][i] = a[6]
			if !partitioned {
				m.SourceParameterCurveIDs[f][i] = a[6]
			}
			if raw {
				rawCurves[i][f+1] = descriptors[a[6]]
			}
		}
	}
	if len(used) != len(records) {
		return fail()
	}
	if partitioned {
		// Canonical ordering starts at the positive first conjugate axis.
		// The Y extrema identify the second conjugate axis even under X shear.
		// These candidates are accepted only after whole-curve equality below.
		var points []geom.Vec3
		for _, x := range ownerUV[0] {
			p, e := x.ExactPoint(0)
			if e != nil {
				return fail()
			}
			points = append(points, p)
		}
		if affine {
			first := rawCurves[0][1].Poles
			tx, e := lensExact(first[1].X, first[0].X, -1)
			if e != nil {
				return fail()
			}
			ty, e := lensExact(first[1].Y, first[0].Y, -1)
			if e != nil {
				return fail()
			}
			var err error
			m.Center, m.Radii, m.Shear, m.Tilt, err = recoverIGESLensBasis(points, geom.Vec3{X: tx, Y: ty})
			if err != nil {
				return fail()
			}
		} else {
			east, north, south := points[0], points[0], points[0]
			for _, p := range points {
				if p.Y > north.Y {
					north = p
				}
				if p.Y < south.Y {
					south = p
				}
			}
			cx, e := lensAverage(north.X, south.X)
			if e != nil {
				return fail()
			}
			cy, e := lensAverage(north.Y, south.Y)
			if e != nil {
				return fail()
			}
			rx, e := lensExact(east.X, cx, -1)
			if e != nil {
				return fail()
			}
			ry, e := lensExact(north.Y, cy, -1)
			if e != nil {
				return fail()
			}
			m.Center = geom.Vec2{X: cx, Y: cy}
			m.Radii = geom.Vec2{X: rx, Y: ry}
			m.Shear, e = lensExact(north.X, cx, -1)
			if e != nil {
				return fail()
			}

		}
		m.Upper, m.Lower = supports[0].Geometry, supports[1].Geometry
		plan, e := igesLensPlan(m)
		if e != nil {
			return SplineLensSolid{}, e
		}
		if len(plan) != n {
			return fail()
		}
		for i, p := range plan {
			for f := 0; f < 2; f++ {
				eq, e := geom.CompareExactCurvesContext(context.Background(), p.UV, ownerUV[f][i], geom.ExactCurveComparisonOptions{NormalizeDomains: true, MaxOperations: 1000000, MaxCoefficientBits: 8192})
				if e != nil || !eq.Complete || !eq.Equal {
					return fail()
				}
			}
			eq, e := geom.CompareExactCurvesContext(context.Background(), p.Model, models[i], geom.ExactCurveComparisonOptions{NormalizeDomains: true, MaxOperations: 1000000, MaxCoefficientBits: 8192})
			if e != nil || !eq.Complete || !eq.Equal {
				return fail()
			}
			p.ModelCurveID = modelIDs[i]
			p.ParameterCurveIDs = [2]int{ownerIDs[0][i], ownerIDs[1][i]}
			pieces = append(pieces, p.LensSeamPiece)
		}
	} else {
		east, e := ownerUV[0][0].ExactPoint(0)
		if e != nil {
			return fail()
		}
		north, e := ownerUV[0][0].ExactPoint(1)
		if e != nil {
			return fail()
		}
		west, e := ownerUV[0][2].ExactPoint(0)
		if e != nil {
			return fail()
		}
		cx, e := lensAverage(east.X, west.X)
		if e != nil {
			return fail()
		}
		m.Center = geom.Vec2{X: cx, Y: east.Y}
		rx, e := lensExact(east.X, m.Center.X, -1)
		if e != nil {
			return fail()
		}
		ry, e := lensExact(north.Y, east.Y, -1)
		if e != nil {
			return fail()
		}
		m.Radii = geom.Vec2{X: rx, Y: ry}
		m.Shear, e = lensExact(north.X, m.Center.X, -1)
		if e != nil {
			return fail()
		}
		canonical, e := lensUV(m.Center, m.Radii, m.Shear)
		if e != nil {
			return fail()
		}
		m.Upper, m.Lower = supports[0].Geometry, supports[1].Geometry
		for f, s := range supports {
			patch, e := geom.NewSurfacePatch(s.Geometry, curvedOvalOptions())
			if e != nil {
				return fail()
			}
			for i := 0; i < n; i++ {
				expected, e := geom.NewExactSplineCurve(canonical[i], curvedOvalOptions())
				if e != nil {
					return fail()
				}
				eq, e := geom.CompareExactCurvesContext(context.Background(), expected, ownerUV[f][i], geom.ExactCurveComparisonOptions{NormalizeDomains: true, MaxOperations: 1000000, MaxCoefficientBits: 8192, MaxSpans: 32})
				if e != nil || !eq.Complete || !eq.Equal {
					return fail()
				}
				curve, cell, e := composeIGESLens(patch, expected, s.Geometry.Data().CountU != 3 || s.Geometry.Data().CountV != 3)
				if e != nil {
					return SplineLensSolid{}, e
				}
				m.SourceKnotCells[f][i] = cell
				eq, e = geom.CompareExactCurvesContext(context.Background(), curve, models[i], geom.ExactCurveComparisonOptions{NormalizeDomains: true, MaxOperations: 1000000, MaxCoefficientBits: 8192, MaxSpans: 32})
				if e != nil || !eq.Complete || !eq.Equal {
					return fail()
				}
			}
		}
	}
	m.Solid, e = certifyIGESLens(m)
	if e != nil {
		return SplineLensSolid{}, e
	}
	if partitioned {
		m.partitioned = &PartitionedRationalSplineLens{Pieces: pieces, Curves: rawCurves}
	} else if raw {
		copy(m.rawCurves[:], rawCurves)
	}
	return m, nil
}
