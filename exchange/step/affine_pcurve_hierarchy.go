package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
)

// AffinePCurveEvidence records an original two-pole, degree-one planar pcurve.
// Its entire knot interval has the same parameterization as the owning model
// LINE. Strings are exact rational values, not rounded observations.
type AffinePCurveEvidence struct {
	EdgeID, SurfaceCurveID, PCurveID, CurveID, PlaneID int64
	PoleIDs                                            [2]int64
	Knots                                              [2]string
	Intercept, Derivative                              [2]string
}

// AffinePCurveHierarchy retains the original source separately from a qualified
// affine LINE proof view. It admits no general spline or trimmed-curve repair.
// Source entity IDs for faces, edges, vertices and occurrences are unchanged.
type AffinePCurveHierarchy struct {
	original  string
	model     RationalSphericalHierarchy
	evidence  []AffinePCurveEvidence
	vendor    bool
	tolerance float64
}

func (m AffinePCurveHierarchy) OriginalSTEP() string { return m.original }

// UsesOCCTFrameConvention distinguishes the explicitly selected frame rule.
func (m AffinePCurveHierarchy) UsesOCCTFrameConvention() bool { return m.vendor }
func (m AffinePCurveHierarchy) AffinePCurves() []AffinePCurveEvidence {
	return append([]AffinePCurveEvidence(nil), m.evidence...)
}

// ReadAffinePCurveRationalHierarchy is an explicit extension of the neutral-child
// rational hierarchy. Only exact affine planar pcurves with exclusive ownership
// and full model-line/endpoint agreement are normalized in its private proof.
func ReadAffinePCurveRationalHierarchy(ctx context.Context, r io.Reader, l Limits, tol float64) (AffinePCurveHierarchy, error) {
	return readAffinePCurveHierarchy(ctx, r, l, tol, false)
}

// ReadOCCTAffinePCurveRationalHierarchy uses the separately qualified OCCT
// nonneutral-frame convention. The original strict readers remain unchanged.
func ReadOCCTAffinePCurveRationalHierarchy(ctx context.Context, r io.Reader, l Limits, tol float64) (AffinePCurveHierarchy, error) {
	return readAffinePCurveHierarchy(ctx, r, l, tol, true)
}
func readAffinePCurveHierarchy(ctx context.Context, r io.Reader, l Limits, tol float64, vendor bool) (AffinePCurveHierarchy, error) {
	var zero AffinePCurveHierarchy
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
	evidence, e := affinePCurveView(ctx, f)
	if e != nil {
		return zero, e
	}
	if len(evidence) == 0 {
		return zero, geom.ErrInvalid
	}
	view, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return zero, e
	}
	m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(view), l, tol, vendor)
	if e != nil {
		return zero, e
	}
	return AffinePCurveHierarchy{string(raw), m, evidence, vendor, tol}, nil
}

func affinePCurveView(ctx context.Context, f *File) ([]AffinePCurveEvidence, error) {
	fail := func() error {
		return fmt.Errorf("step: affine planar pcurve ownership or exact identity: %w", geom.ErrInvalid)
	}
	incoming := map[int64][]int64{}
	var walk func(int64, Value)
	walk = func(owner int64, v Value) {
		if v.Kind == Reference {
			incoming[v.Ref] = append(incoming[v.Ref], owner)
		}
		for _, q := range v.Items {
			walk(owner, q)
		}
	}
	var maxID int64
	for id, en := range f.Entities {
		maxID = max(maxID, id)
		for _, r := range en.Records {
			for _, v := range r.Args {
				walk(id, v)
			}
		}
	}
	unique := func(id int64, kind string, n int) (int64, Record, error) {
		a := incoming[id]
		if len(a) != 1 {
			return 0, Record{}, fail()
		}
		r, e := simple(f, a[0], kind, n)
		return a[0], r, e
	}
	refv := func(id int64) Value { return Value{Kind: Reference, Ref: id} }
	number := func(q *big.Rat) (Value, error) {
		s, e := extrusionDirectionDecimal(q)
		if e != nil {
			return Value{}, e
		}
		v := Value{Kind: Number, Text: s}
		_, e = cylinderSupportNumber(v)
		return v, e
	}
	coord := func(id int64) ([3]*big.Rat, error) {
		v, e := mappedExactCoordinate(f, id, "CARTESIAN_POINT", 3)
		if e != nil {
			return [3]*big.Rat{}, e
		}
		return mappedRatVec(v), nil
	}
	var out []AffinePCurveEvidence
	for _, id := range append([]int64(nil), f.Order...) {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "B_SPLINE_CURVE_WITH_KNOTS" {
			continue
		}
		s, e := simple(f, id, "B_SPLINE_CURVE_WITH_KNOTS", 9)
		if e != nil {
			return nil, e
		}
		// This mode owns every spline leaf: unrelated splines cannot be hidden by normalization.
		deg, e := liftSourceRat(s.Args[1])
		if e != nil || deg.Cmp(big.NewRat(1, 1)) != 0 {
			return nil, fail()
		}
		poles, e := listRefs(s.Args[2], 2)
		if e != nil || len(poles) != 2 || poles[0] == poles[1] {
			return nil, fail()
		}
		if s.Args[3].Kind != Enumeration || s.Args[3].Text != "UNSPECIFIED" || s.Args[4].Kind != Enumeration || s.Args[4].Text != "F" || s.Args[5].Kind != Enumeration || s.Args[5].Text != "F" {
			return nil, fail()
		}
		if s.Args[8].Kind != Enumeration || (s.Args[8].Text != "PIECEWISE_BEZIER_KNOTS" && s.Args[8].Text != "UNSPECIFIED") {
			return nil, fail()
		}
		if s.Args[6].Kind != Aggregate || len(s.Args[6].Items) != 2 || s.Args[7].Kind != Aggregate || len(s.Args[7].Items) != 2 {
			return nil, fail()
		}
		var knots [2]*big.Rat
		var p [2][2]*big.Rat
		for i, pid := range poles {
			mu, err := liftSourceRat(s.Args[6].Items[i])
			if err != nil || mu.Cmp(big.NewRat(2, 1)) != 0 {
				return nil, fail()
			}
			knots[i], e = liftSourceRat(s.Args[7].Items[i])
			if e != nil {
				return nil, e
			}
			if len(incoming[pid]) != 1 || incoming[pid][0] != id {
				return nil, fail()
			}
			pr, err := simple(f, pid, "CARTESIAN_POINT", 2)
			if err != nil || pr.Args[1].Kind != Aggregate || len(pr.Args[1].Items) != 2 {
				return nil, fail()
			}
			for j := 0; j < 2; j++ {
				p[i][j], e = liftSourceRat(pr.Args[1].Items[j])
				if e != nil {
					return nil, e
				}
			}
		}
		if knots[0].Cmp(knots[1]) >= 0 {
			return nil, fail()
		}
		drID, dr, e := unique(id, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil {
			return nil, e
		}
		refs, e := listRefs(dr.Args[1], 1)
		if e != nil || len(refs) != 1 || refs[0] != id {
			return nil, fail()
		}
		pcID, pc, e := unique(drID, "PCURVE", 3)
		if e != nil {
			return nil, e
		}
		if pc.Args[2].Kind != Reference || pc.Args[2].Ref != drID {
			return nil, fail()
		}
		planeID, e := ref(pc.Args[1])
		if e != nil {
			return nil, e
		}
		plane, e := simple(f, planeID, "PLANE", 2)
		if e != nil {
			return nil, e
		}
		placement, e := ref(plane.Args[1])
		if e != nil {
			return nil, e
		}
		frame, e := mappedExactFrame(f, placement)
		if e != nil {
			return nil, e
		}
		scID, sc, e := unique(pcID, "SURFACE_CURVE", 4)
		if e != nil {
			return nil, e
		}
		pcs, e := listRefs(sc.Args[2], 2)
		if e != nil || len(pcs) != 2 || (pcs[0] != pcID && pcs[1] != pcID) {
			return nil, fail()
		}
		edgeID, edge, e := unique(scID, "EDGE_CURVE", 5)
		if e != nil {
			return nil, e
		}
		if edge.Args[3].Kind != Reference || edge.Args[3].Ref != scID {
			return nil, fail()
		}
		same, e := boolean(edge.Args[4])
		if e != nil {
			return nil, e
		}
		lineID, e := ref(sc.Args[1])
		if e != nil {
			return nil, e
		}
		line, e := simple(f, lineID, "LINE", 3)
		if e != nil {
			return nil, e
		}
		originID, e := ref(line.Args[1])
		if e != nil {
			return nil, e
		}
		origin, e := coord(originID)
		if e != nil {
			return nil, e
		}
		vectorID, e := ref(line.Args[2])
		if e != nil {
			return nil, e
		}
		vector, e := simple(f, vectorID, "VECTOR", 3)
		if e != nil {
			return nil, e
		}
		magnitude, e := liftSourceRat(vector.Args[2])
		if e != nil || magnitude.Sign() <= 0 {
			return nil, fail()
		}
		dirID, e := ref(vector.Args[1])
		if e != nil {
			return nil, e
		}
		direction, e := mappedExactCoordinate(f, dirID, "DIRECTION", 3)
		if e != nil || !cylinderSupportUnit(direction) {
			return nil, fail()
		}
		d := mappedRatVec(direction)
		var intercept, slope [2]*big.Rat
		nonzero := -1
		delta := new(big.Rat).Sub(knots[1], knots[0])
		for j := 0; j < 2; j++ {
			slope[j] = new(big.Rat).Quo(new(big.Rat).Sub(p[1][j], p[0][j]), delta)
			intercept[j] = new(big.Rat).Sub(p[0][j], new(big.Rat).Mul(knots[0], slope[j]))
			if slope[j].Sign() != 0 {
				if nonzero >= 0 {
					return nil, fail()
				}
				nonzero = j
			}
		}
		if nonzero < 0 {
			return nil, fail()
		}
		fo, x, y := mappedRatVec(frame.Origin()), mappedRatVec(frame.XAxis()), mappedRatVec(frame.YAxis())
		for j := 0; j < 3; j++ {
			o := new(big.Rat).Add(fo[j], new(big.Rat).Add(new(big.Rat).Mul(x[j], intercept[0]), new(big.Rat).Mul(y[j], intercept[1])))
			v := new(big.Rat).Add(new(big.Rat).Mul(x[j], slope[0]), new(big.Rat).Mul(y[j], slope[1]))
			if o.Cmp(origin[j]) != 0 || v.Cmp(new(big.Rat).Mul(d[j], magnitude)) != 0 {
				return nil, fail()
			}
		}
		for i := 0; i < 2; i++ {
			k := i
			if !same {
				k = 1 - i
			}
			vertexID, e := ref(edge.Args[1+k])
			if e != nil {
				return nil, e
			}
			vertex, e := simple(f, vertexID, "VERTEX_POINT", 2)
			if e != nil {
				return nil, e
			}
			pid, e := ref(vertex.Args[1])
			if e != nil {
				return nil, e
			}
			point, e := coord(pid)
			if e != nil {
				return nil, e
			}
			for j := 0; j < 3; j++ {
				q := new(big.Rat).Add(origin[j], new(big.Rat).Mul(knots[i], new(big.Rat).Mul(d[j], magnitude)))
				if q.Cmp(point[j]) != 0 {
					return nil, fail()
				}
			}
		}
		claim := AffinePCurveEvidence{EdgeID: edgeID, SurfaceCurveID: scID, PCurveID: pcID, CurveID: id, PlaneID: planeID, PoleIDs: [2]int64{poles[0], poles[1]}}
		var iv, dv []Value
		for j := 0; j < 2; j++ {
			claim.Knots[j] = knots[j].RatString()
			claim.Intercept[j] = intercept[j].RatString()
			claim.Derivative[j] = slope[j].RatString()
			v, e := number(intercept[j])
			if e != nil {
				return nil, e
			}
			iv = append(iv, v)
			dv = append(dv, Value{Kind: Number, Text: fmt.Sprint(slope[j].Sign())})
		}
		mag, e := number(new(big.Rat).Abs(slope[nonzero]))
		if e != nil {
			return nil, e
		}
		if maxID >= 1<<62 {
			return nil, fail()
		}
		maxID++
		f.Entities[poles[0]] = Entity{ID: poles[0], Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: iv}}}}}
		f.Entities[poles[1]] = Entity{ID: poles[1], Records: []Record{{Type: "DIRECTION", Args: []Value{{Kind: String}, {Kind: Aggregate, Items: dv}}}}}
		f.Entities[maxID] = Entity{ID: maxID, Records: []Record{{Type: "VECTOR", Args: []Value{{Kind: String}, refv(poles[1]), mag}}}}
		f.Order = append(f.Order, maxID)
		f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "LINE", Args: []Value{{Kind: String}, refv(poles[0]), refv(maxID)}}}}
		out = append(out, claim)
	}
	return out, nil
}
