package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
)

// CylinderPCurveEvidence retains verified source graph links and exact U
// monotonicity/range evidence. It does not prove pointwise model/pcurve matching.
type CylinderPCurveEvidence struct {
	EdgeID, SurfaceCurveID, ModelCurveID, CylinderID, PCurveID, ParameterCurveID, ShellID, BodyID int64
	OwnerFaces                                                                                    [2]int64
	EdgeSameSense                                                                                 bool
	IncreasingU, DecreasingU                                                                      bool
	ParameterStart, ParameterEnd, UStart, UEnd                                                    *big.Rat
	DerivativeSpans                                                                               []PolynomialSphereResidualSpan
	// PositiveTurnDefect encloses (UEnd-UStart)-2*pi, in exact radians.
	PositiveTurnDefect   [2]*big.Rat
	MasterRepresentation string
	MasterPCurveID       int64
}

// InspectCylinderPCurve verifies a two-owner SURFACE_CURVE edge in one closed
// shell, exactly one associated pcurve per owner and one cylindrical owner.
// The simple polynomial cylinder pcurve must use a 2D context and the same exact
// source parameter domain as its 3D polynomial model. U' is enclosed exactly.
// The unique owning body representation must declare unscaled radians.
// No periodic closure, winding, inverse matching or material acceptance follows.
func InspectCylinderPCurve(r io.Reader, l Limits, edgeID int64) (out CylinderPCurveEvidence, err error) {
	fail := func() (CylinderPCurveEvidence, error) {
		return CylinderPCurveEvidence{}, fmt.Errorf("step: unsupported cylinder pcurve graph or proof budget")
	}
	l, e := l.defaults()
	if e != nil || r == nil {
		return fail()
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return out, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return fail()
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return out, e
	}
	edge, e := simple(f, edgeID, "EDGE_CURVE", 5)
	if e != nil {
		return fail()
	}
	same, e := boolean(edge.Args[4])
	if e != nil {
		return fail()
	}
	scID, e := ref(edge.Args[3])
	if e != nil {
		return fail()
	}
	sc, e := simple(f, scID, "SURFACE_CURVE", 4)
	if e != nil {
		return fail()
	}
	model, e := ref(sc.Args[1])
	if e != nil {
		return fail()
	}
	pcs, e := listRefs(sc.Args[2], 2)
	if e != nil || len(pcs) != 2 || pcs[0] == pcs[1] {
		return fail()
	}
	if sc.Args[3].Kind != Enumeration || sc.Args[3].Text != "CURVE_3D" && sc.Args[3].Text != "PCURVE_S1" && sc.Args[3].Text != "PCURVE_S2" {
		return fail()
	}
	ownerSurfaces := map[int64]int64{}
	var faces []int64
	var senses []bool
	uses := 0
	faceCount := 0
	selectedUses := map[int64]bool{}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "ADVANCED_FACE" {
			continue
		}
		faceCount++
		if faceCount > l.MaxFaces {
			return fail()
		}
		face, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return fail()
		}
		if _, e := boolean(face.Args[3]); e != nil {
			return fail()
		}
		support, e := ref(face.Args[2])
		if e != nil {
			return fail()
		}
		bounds, e := listRefs(face.Args[1], 1)
		if e != nil {
			return fail()
		}
		for _, bid := range bounds {
			bound, e := simple(f, bid, "FACE_BOUND", 3)
			if e != nil {
				bound, e = simple(f, bid, "FACE_OUTER_BOUND", 3)
			}
			if e != nil {
				return fail()
			}
			sense, e := boolean(bound.Args[2])
			if e != nil {
				return fail()
			}
			loopID, e := ref(bound.Args[1])
			if e != nil {
				return fail()
			}
			loop, e := simple(f, loopID, "EDGE_LOOP", 2)
			if e != nil {
				return fail()
			}
			oes, e := listRefs(loop.Args[1], 1)
			if e != nil {
				return fail()
			}
			uses += len(oes)
			if uses > l.MaxEdgeUses {
				return fail()
			}
			for _, oid := range oes {
				oe, e := simple(f, oid, "ORIENTED_EDGE", 5)
				if e != nil {
					return fail()
				}
				if oe.Args[1].Kind != Derived || oe.Args[2].Kind != Derived {
					return fail()
				}
				eid, e := ref(oe.Args[3])
				if e != nil {
					return fail()
				}
				if eid != edgeID {
					continue
				}
				forward, e := boolean(oe.Args[4])
				if e != nil || ownerSurfaces[support] != 0 {
					return fail()
				}
				selectedUses[oid] = true
				ownerSurfaces[support] = id
				faces = append(faces, id)
				senses = append(senses, forward == sense)
			}
		}
	}
	if len(faces) != 2 || senses[0] == senses[1] {
		return fail()
	}
	var cylinder, pcID, param int64
	for _, pid := range pcs {
		pc, e := simple(f, pid, "PCURVE", 3)
		if e != nil {
			return fail()
		}
		sid, e := ref(pc.Args[1])
		if e != nil || ownerSurfaces[sid] == 0 {
			return fail()
		}
		delete(ownerSurfaces, sid)
		if _, ok := f.Entities[sid]; !ok {
			return fail()
		}
		if _, e := simple(f, sid, "CYLINDRICAL_SURFACE", 3); e != nil {
			continue
		}
		if cylinder != 0 {
			return fail()
		}
		cylinder, pcID = sid, pid
		rid, e := ref(pc.Args[2])
		if e != nil {
			return fail()
		}
		rep, e := simple(f, rid, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil {
			return fail()
		}
		items, e := listRefs(rep.Args[1], 1)
		if e != nil || len(items) != 1 {
			return fail()
		}
		param = items[0]
		contextID, e := ref(rep.Args[2])
		if e != nil {
			return fail()
		}
		context, ok := f.Entities[contextID]
		if !ok {
			return fail()
		}
		dimension := false
		for _, rec := range context.Records {
			if rec.Type == "GEOMETRIC_REPRESENTATION_CONTEXT" && len(rec.Args) == 1 {
				n, e := numeric(rec.Args[0])
				dimension = e == nil && n == 2
			}
		}
		if !dimension {
			return fail()
		}
	}
	if cylinder == 0 || len(ownerSurfaces) != 0 {
		return fail()
	}
	shell, body := int64(0), int64(0)
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 {
			continue
		}
		rec := en.Records[0]
		if rec.Type == "ORIENTED_EDGE" && len(rec.Args) == 5 {
			if eid, e := ref(rec.Args[3]); e == nil && eid == edgeID && !selectedUses[id] {
				return fail()
			}
		}
		if (rec.Type == "SURFACE_CURVE" || rec.Type == "SEAM_CURVE") && id != scID && len(rec.Args) == 4 {
			for _, v := range rec.Args[2].Items {
				if v.Kind == Reference && (v.Ref == pcs[0] || v.Ref == pcs[1]) {
					return fail()
				}
			}
		}
		if rec.Type == "EDGE_CURVE" && id != edgeID && len(rec.Args) == 5 {
			if x, e := ref(rec.Args[3]); e == nil && x == scID {
				return fail()
			}
		}
		if rec.Type == "CLOSED_SHELL" && len(rec.Args) == 2 {
			ids, e := listRefs(rec.Args[1], 1)
			if e != nil {
				return fail()
			}
			countA, countB := 0, 0
			for _, fid := range ids {
				if fid == faces[0] {
					countA++
				}
				if fid == faces[1] {
					countB++
				}
			}
			if countA+countB > 0 {
				if countA != 1 || countB != 1 || shell != 0 {
					return fail()
				}
				shell = id
			}
		}
	}
	if shell == 0 {
		return fail()
	}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "MANIFOLD_SOLID_BREP" && len(en.Records[0].Args) == 2 {
			sid, e := ref(en.Records[0].Args[1])
			if e == nil && sid == shell {
				if body != 0 {
					return fail()
				}
				body = id
			}
		}
	}
	if body == 0 || !liftRadianContext(f, body) {
		return fail()
	}
	p, poles, knots, e := readResidualPolynomialDimension(bytes.NewReader(raw), l, param, 2)
	if e != nil {
		return fail()
	}
	mp, modelPoles, modelKnots, e := readResidualPolynomial(bytes.NewReader(raw), l, model)
	if e != nil {
		return fail()
	}
	if knots[p].Cmp(modelKnots[mp]) != 0 || knots[len(poles)].Cmp(modelKnots[len(modelPoles)]) != 0 {
		return fail()
	}
	out = CylinderPCurveEvidence{EdgeID: edgeID, SurfaceCurveID: scID, ModelCurveID: model, CylinderID: cylinder, PCurveID: pcID, ParameterCurveID: param, ShellID: shell, BodyID: body, OwnerFaces: [2]int64{faces[0], faces[1]}, EdgeSameSense: same, IncreasingU: true, DecreasingU: true, ParameterStart: new(big.Rat).Set(knots[p]), ParameterEnd: new(big.Rat).Set(knots[len(poles)]), UStart: new(big.Rat).Set(poles[0][0]), UEnd: new(big.Rat).Set(poles[len(poles)-1][0])}
	derivative := make([][3]*big.Rat, len(poles)-1)
	for i := range derivative {
		den := new(big.Rat).Sub(knots[i+p+1], knots[i+1])
		if den.Sign() == 0 {
			return fail()
		}
		for c := 0; c < 3; c++ {
			derivative[i][c] = new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(int64(p), 1), new(big.Rat).Sub(poles[i+1][c], poles[i][c])), den)
		}
	}
	n := p - 1
	var inverse [][]*big.Rat
	if n > 0 {
		inverse, e = residualBernsteinInverse(n)
		if e != nil {
			return fail()
		}
	}
	for k := p; k < len(poles); k++ {
		a, b := knots[k], knots[k+1]
		if a.Cmp(b) == 0 {
			continue
		}
		values := make([]*big.Rat, n+1)
		for j := 0; j <= n; j++ {
			t := new(big.Rat).Set(a)
			if n > 0 {
				t.Add(t, new(big.Rat).Mul(new(big.Rat).Sub(b, a), big.NewRat(int64(j), int64(n))))
			}
			q, e := residualDeBoor(n, derivative, knots[1:len(knots)-1], k-1, t)
			if e != nil {
				return fail()
			}
			values[j] = q[0]
		}
		var lo, hi *big.Rat
		for i := 0; i <= n; i++ {
			q := new(big.Rat)
			if n == 0 {
				q.Set(values[0])
			} else {
				for j := 0; j <= n; j++ {
					q.Add(q, new(big.Rat).Mul(inverse[i][j], values[j]))
				}
			}
			if !residualBits(q) {
				return fail()
			}
			if lo == nil || q.Cmp(lo) < 0 {
				lo = new(big.Rat).Set(q)
			}
			if hi == nil || q.Cmp(hi) > 0 {
				hi = new(big.Rat).Set(q)
			}
		}
		out.DerivativeSpans = append(out.DerivativeSpans, PolynomialSphereResidualSpan{new(big.Rat).Set(a), new(big.Rat).Set(b), lo, hi})
		out.IncreasingU = out.IncreasingU && lo.Sign() > 0
		out.DecreasingU = out.DecreasingU && hi.Sign() < 0
	}
	out.MasterRepresentation = sc.Args[3].Text
	if out.MasterRepresentation == "PCURVE_S1" {
		out.MasterPCurveID = pcs[0]
	} else if out.MasterRepresentation == "PCURVE_S2" {
		out.MasterPCurveID = pcs[1]
	}
	piLo, piHi := pcurvePiBounds()
	delta := new(big.Rat).Sub(out.UEnd, out.UStart)
	out.PositiveTurnDefect = [2]*big.Rat{new(big.Rat).Sub(delta, new(big.Rat).Mul(big.NewRat(2, 1), piHi)), new(big.Rat).Sub(delta, new(big.Rat).Mul(big.NewRat(2, 1), piLo))}
	return out, nil
}

// Machin's identity with alternating-series rational enclosures.
func pcurvePiBounds() (*big.Rat, *big.Rat) {
	atan := func(d int64) (*big.Rat, *big.Rat) {
		sum := new(big.Rat)
		power := big.NewInt(d)
		square := big.NewInt(d * d)
		for k := int64(0); k < 32; k++ {
			term := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Mul(big.NewInt(2*k+1), power))
			if k%2 == 0 {
				sum.Add(sum, term)
			} else {
				sum.Sub(sum, term)
			}
			power.Mul(power, square)
		}
		next := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Mul(big.NewInt(65), power))
		return new(big.Rat).Set(sum), new(big.Rat).Add(sum, next)
	}
	a, b := atan(5)
	c, d := atan(239)
	return new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(16, 1), a), new(big.Rat).Mul(big.NewRat(4, 1), d)), new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(16, 1), b), new(big.Rat).Mul(big.NewRat(4, 1), c))
}
