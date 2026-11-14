package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/geom"
)

// NURBSPlanarChartScaling proves the positive diagonal parameter change
// (u,v)->(Scale[0]*u,Scale[1]*v). Source keeps the original chart coefficients;
// ProofU/ProofV are the knot ranges in the equivalent unit-derivative view.
type NURBSPlanarChartScaling struct {
	Source         NURBSPlanarSurfaceEvidence
	Scale          [2]string
	ProofU, ProofV [2]string
}

// NURBSPlanarPCurveScaling records both original and derived UV poles. Original
// model-curve and pcurve knot parameters do not change during this operation.
type NURBSPlanarPCurveScaling struct {
	PCurveID, CurveID, SurfaceID int64
	PoleIDs                      [2]int64
	SourcePoles, ProofPoles      [2][2]string
}

// ScaledNURBSPlanarAssembly retains independently authenticated original
// non-unit cardinal affine spline charts. Geometry is derived, not an editable
// source recipe. It has the same native material scope as NURBSPlanarAssembly.
type ScaledNURBSPlanarAssembly struct {
	Geometry             ColoredAssembly
	original, normalized string
	charts               []NURBSPlanarChartScaling
	pcurves              []NURBSPlanarPCurveScaling
	modelCurves          []NURBSPlanarCurveEvidence
	proofCurves          []AffinePCurveEvidence
}

func (m ScaledNURBSPlanarAssembly) OriginalSTEP() string { return m.original }

// NormalizedSTEP is the exactly equivalent NURBS source with unit-derivative
// chart parameters, retaining source entity IDs, geometry, uses and metadata.
func (m ScaledNURBSPlanarAssembly) NormalizedSTEP() string { return m.normalized }
func (m ScaledNURBSPlanarAssembly) ChartScalings() []NURBSPlanarChartScaling {
	return append([]NURBSPlanarChartScaling(nil), m.charts...)
}
func (m ScaledNURBSPlanarAssembly) PCurveScalings() []NURBSPlanarPCurveScaling {
	return append([]NURBSPlanarPCurveScaling(nil), m.pcurves...)
}
func (m ScaledNURBSPlanarAssembly) ModelCurves() []NURBSPlanarCurveEvidence {
	return append([]NURBSPlanarCurveEvidence(nil), m.modelCurves...)
}

// NormalizedPCurves uses the derived plane-chart coordinates, with unchanged
// original curve parameters and original owner/curve IDs.
func (m ScaledNURBSPlanarAssembly) NormalizedPCurves() []AffinePCurveEvidence {
	return append([]AffinePCurveEvidence(nil), m.proofCurves...)
}

// ReadScaledNURBSPlanarAssembly adds positive exact cardinal chart scaling to
// ReadNURBSPlanarAssembly. Every changed scalar must be exactly representable;
// no normalized source coefficient is rounded. Original domain, endpoint,
// continuous composition, ownership and material proofs remain mandatory.
func ReadScaledNURBSPlanarAssembly(ctx context.Context, r io.Reader, l Limits) (ScaledNURBSPlanarAssembly, error) {
	var zero ScaledNURBSPlanarAssembly
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
	charts, pc, e := scaleNURBSPlanarCharts(ctx, f)
	if e != nil {
		return zero, e
	}
	normalized, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return zero, e
	}
	proof, e := ReadNURBSPlanarAssembly(ctx, bytes.NewReader(normalized), l)
	if e != nil {
		return zero, e
	}
	if len(charts) != len(proof.surfaces) || len(pc) != len(proof.pcurves) {
		return zero, geom.ErrInvalid
	}
	byID := map[int64]NURBSPlanarSurfaceEvidence{}
	for _, s := range proof.surfaces {
		byID[s.SurfaceID] = s
	}
	for i := range charts {
		s, ok := byID[charts[i].Source.SurfaceID]
		if !ok || s.PoleIDs != charts[i].Source.PoleIDs {
			return zero, geom.ErrInvalid
		}
		charts[i].Source.FaceID = s.FaceID
		charts[i].Source.Origin = s.Origin
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return ScaledNURBSPlanarAssembly{proof.Geometry, string(raw), string(normalized), charts, pc, proof.curves, proof.pcurves}, nil
}

// WriteScaledNURBSPlanarAssemblySource validates and emits the retained original
// source. Derived native geometry/evidence mutations are never serialized.
func WriteScaledNURBSPlanarAssemblySource(ctx context.Context, w io.Writer, m ScaledNURBSPlanarAssembly, l Limits) error {
	if w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadScaledNURBSPlanarAssembly(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := io.WriteString(w, p.original)
	if e == nil && n != len(p.original) {
		return io.ErrShortWrite
	}
	return e
}

func scaleNURBSPlanarCharts(ctx context.Context, f *File) ([]NURBSPlanarChartScaling, []NURBSPlanarPCurveScaling, error) {
	fail := func() error {
		return fmt.Errorf("step: nonrepresentable or unsupported exact NURBS chart scaling: %w", geom.ErrInvalid)
	}
	exact := func(v Value) (*big.Rat, error) {
		if _, e := wrappedIsoSourceNumber(v); e != nil {
			return nil, e
		}
		return liftSourceRat(v)
	}
	value := func(q *big.Rat) (Value, error) {
		if q.Num().BitLen() > 1024 || q.Denom().BitLen() > 1024 {
			return Value{}, fail()
		}
		s, e := extrusionDirectionDecimal(q)
		if e != nil {
			return Value{}, e
		}
		v := Value{Kind: Number, Text: s}
		if _, e = wrappedIsoSourceNumber(v); e != nil {
			return Value{}, e
		}
		return v, nil
	}
	// Work on private copied records; shared source point ownership is subsequently
	// checked by the complete original-domain proof, not repaired by copying.
	scales := map[int64][2]*big.Rat{}
	var charts []NURBSPlanarChartScaling
	for _, id := range f.Order {
		if e := ctx.Err(); e != nil {
			return nil, nil, e
		}
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "B_SPLINE_SURFACE_WITH_KNOTS" {
			continue
		}
		s, e := simple(f, id, "B_SPLINE_SURFACE_WITH_KNOTS", 13)
		if e != nil {
			return nil, nil, e
		}
		for _, i := range []int{1, 2} {
			d, e := integer(s.Args[i])
			if e != nil || d != 1 {
				return nil, nil, fail()
			}
		}
		if s.Args[3].Kind != Aggregate || len(s.Args[3].Items) != 2 {
			return nil, nil, fail()
		}
		var ids [4]int64
		var p [4][3]*big.Rat
		for i := 0; i < 2; i++ {
			row, e := listRefs(s.Args[3].Items[i], 2)
			if e != nil || len(row) != 2 {
				return nil, nil, fail()
			}
			for j := 0; j < 2; j++ {
				k := 2*i + j
				ids[k] = row[j]
				r, e := simple(f, ids[k], "CARTESIAN_POINT", 2)
				if e != nil || r.Args[1].Kind != Aggregate || len(r.Args[1].Items) != 3 {
					return nil, nil, fail()
				}
				for h, v := range r.Args[1].Items {
					p[k][h], e = exact(v)
					if e != nil {
						return nil, nil, e
					}
				}
			}
		}
		c := NURBSPlanarChartScaling{Source: NURBSPlanarSurfaceEvidence{SurfaceID: id, PoleIDs: ids}}
		var factors [2]*big.Rat
		for axis, ki := range []int{10, 11} {
			k := s.Args[ki]
			if k.Kind != Aggregate || len(k.Items) != 2 {
				return nil, nil, fail()
			}
			var ends [2]*big.Rat
			for j, v := range k.Items {
				ends[j], e = exact(v)
				if e != nil {
					return nil, nil, e
				}
			}
			delta := new(big.Rat).Sub(ends[1], ends[0])
			if delta.Sign() <= 0 {
				return nil, nil, fail()
			}
			pole := 2
			if axis == 1 {
				pole = 1
			}
			active := -1
			var derivative [3]string
			for j := 0; j < 3; j++ {
				d := new(big.Rat).Quo(new(big.Rat).Sub(p[pole][j], p[0][j]), delta)
				derivative[j] = d.RatString()
				if d.Sign() != 0 {
					if active >= 0 {
						return nil, nil, fail()
					}
					active = j
					factors[axis] = new(big.Rat).Abs(d)
				}
			}
			if active < 0 {
				return nil, nil, fail()
			}
			c.Scale[axis] = factors[axis].RatString()
			var sourceKnots, proofKnots [2]string
			for j := 0; j < 2; j++ {
				q := new(big.Rat).Mul(ends[j], factors[axis])
				v, e := value(q)
				if e != nil {
					return nil, nil, e
				}
				s.Args[ki].Items[j] = v
				sourceKnots[j] = ends[j].RatString()
				proofKnots[j] = q.RatString()
			}
			if axis == 0 {
				c.Source.U = sourceKnots
				c.ProofU = proofKnots
				c.Source.DU = derivative
			} else {
				c.Source.V = sourceKnots
				c.ProofV = proofKnots
				c.Source.DV = derivative
			}
		}
		scales[id] = factors
		charts = append(charts, c)
	}
	var curves []NURBSPlanarPCurveScaling
	seen := map[int64]bool{}
	for _, id := range f.Order {
		if e := ctx.Err(); e != nil {
			return nil, nil, e
		}
		pc, e := simple(f, id, "PCURVE", 3)
		if e != nil {
			continue
		}
		sid, e := ref(pc.Args[1])
		scale, ok := scales[sid]
		if e != nil || !ok {
			return nil, nil, fail()
		}
		repID, e := ref(pc.Args[2])
		if e != nil {
			return nil, nil, e
		}
		rep, e := simple(f, repID, "DEFINITIONAL_REPRESENTATION", 3)
		if e != nil {
			return nil, nil, e
		}
		cs, e := listRefs(rep.Args[1], 1)
		if e != nil || len(cs) != 1 {
			return nil, nil, fail()
		}
		curve, e := simple(f, cs[0], "B_SPLINE_CURVE_WITH_KNOTS", 9)
		if e != nil {
			return nil, nil, e
		}
		degree, e := integer(curve.Args[1])
		if e != nil || degree != 1 {
			return nil, nil, fail()
		}
		ps, e := listRefs(curve.Args[2], 2)
		if e != nil || len(ps) != 2 {
			return nil, nil, fail()
		}
		c := NURBSPlanarPCurveScaling{PCurveID: id, CurveID: cs[0], SurfaceID: sid, PoleIDs: [2]int64{ps[0], ps[1]}}
		for j, pid := range ps {
			if seen[pid] {
				return nil, nil, fail()
			}
			seen[pid] = true
			p, e := simple(f, pid, "CARTESIAN_POINT", 2)
			if e != nil || p.Args[1].Kind != Aggregate || len(p.Args[1].Items) != 2 {
				return nil, nil, fail()
			}
			for k, v := range p.Args[1].Items {
				q, e := exact(v)
				if e != nil {
					return nil, nil, e
				}
				c.SourcePoles[j][k] = q.RatString()
				q.Mul(q, scale[k])
				c.ProofPoles[j][k] = q.RatString()
				out, e := value(q)
				if e != nil {
					return nil, nil, e
				}
				p.Args[1].Items[k] = out
			}
		}
		curves = append(curves, c)
	}
	return charts, curves, nil
}
