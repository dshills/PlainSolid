package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicTubeRunMeshOptions separates partition proof limits from cumulative
// display work. Endpoint overrides are deliberately not accepted: only freshly
// rederived immutable section provenance supplies endpoint model identities.
type AlgebraicTubeRunMeshOptions struct {
	Partition AlgebraicTubeFacePartitionOptions
	Display   RationalTubePolylineOptions
	MaxPieces int
}

type AlgebraicTubeRunSample struct {
	Free     *geom.ExactAlgebraicRoot // Original free units of this piece; never a rounded cut.
	Rational *big.Rat                 // Non-nil for an interior rational graph sample.
	Point    geom.Vec3
	Model    geom.AlgebraicCurveEndpointBounds
}
type CertifiedAlgebraicTubeRun struct {
	Partition       AlgebraicTubeFacePartition
	Vertices        []geom.Vec3
	PieceVertices   [][]int // Run orientation; junction indices shared by provenance.
	Samples         [][]AlgebraicTubeRunSample
	SquaredErrors   [][]*big.Rat
	ExactOperations int // Display work only; partition reports its own work.
}

// TessellateTubeRunCertified certifies a continuous model-space polyline for
// one freshly proved face partition. Whole-branch boxes prove both directed
// distances, including represented endpoint error. It does not certify fill,
// embedding, or independently supplied public cut/event bindings. No partial
// mesh is returned on failure. Free parameters can differ between pieces.
func (r AlgebraicTrimmedPairSection) TessellateTubeRunCertified(ctx context.Context, chart, branch, run int, o AlgebraicTubeRunMeshOptions) (CertifiedAlgebraicTubeRun, error) {
	var empty CertifiedAlgebraicTubeRun
	d := o.Display
	if ctx == nil || !finite(d.Deflection) || d.Deflection <= 0 || d.EndpointPoints != nil || !finite(d.Point.Tolerance) || d.Point.Tolerance < 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 256
	}
	if o.MaxPieces < 1 || o.MaxPieces > 4096 {
		return empty, geom.ErrInvalid
	}
	if d.MaxSegments == 0 {
		d.MaxSegments = 4096
	}
	if d.MaxDepth == 0 {
		d.MaxDepth = 32
	}
	if d.MaxOperations == 0 {
		d.MaxOperations = 8000000
	}
	if d.MaxInputBits == 0 {
		d.MaxInputBits = 4096
	}
	if d.MaxSegments < 1 || d.MaxSegments > 65536 || d.MaxDepth < 1 || d.MaxDepth > 64 || d.MaxOperations < 1 || d.MaxOperations > 1000000000 || d.MaxInputBits < 64 || d.MaxInputBits > 65536 {
		return empty, geom.ErrInvalid
	}
	p, e := r.PartitionFaceByTubeRun(ctx, chart, branch, run, o.Partition)
	if e != nil {
		return empty, e
	}
	if len(p.Cut) == 0 || len(p.Cut) > o.MaxPieces {
		return empty, geom.ErrConvergence
	}
	out := CertifiedAlgebraicTubeRun{Partition: p}
	remaining := func() (int, error) {
		n := d.MaxOperations - out.ExactOperations
		if n < 1 {
			return 0, geom.ErrConvergence
		}
		return n, nil
	}
	rootOptions := func() (geom.ExactCurvePlaneEventOptions, error) {
		n, e := remaining()
		return geom.ExactCurvePlaneEventOptions{MaxOperations: n, MaxCoefficientBits: d.MaxInputBits}, e
	}
	endpoints := make(map[*geom.ExactComposedCurveEvent]int)
	models := make(map[*geom.ExactComposedCurveEvent]geom.AlgebraicCurveEndpointBounds)
	getEndpoint := func(event *geom.ExactComposedCurveEvent) (geom.AlgebraicCurveEndpointBounds, int, error) {
		if i, ok := endpoints[event]; ok {
			return models[event], i, nil
		}
		n, e := remaining()
		if e != nil {
			return geom.AlgebraicCurveEndpointBounds{}, 0, e
		}
		tol := d.Deflection / 16
		if tol == 0 {
			tol = d.Deflection
		}
		if d.Point.Tolerance > 0 && d.Point.Tolerance < tol {
			tol = d.Point.Tolerance
		}
		b, e := event.Enclose(ctx, geom.ComposedCurveEndpointOptions{Tolerance: tol, ExactCurvePlaneEventOptions: geom.ExactCurvePlaneEventOptions{MaxOperations: n, MaxCoefficientBits: d.MaxInputBits}})
		out.ExactOperations += b.ExactOperations
		if e != nil {
			return b, 0, e
		}
		if !b.Converged || !b.HasRepresentative {
			return b, 0, geom.ErrConvergence
		}
		for _, v := range out.Vertices {
			if v == b.Point {
				return b, 0, geom.ErrDegenerate
			}
		}
		i := len(out.Vertices)
		out.Vertices = append(out.Vertices, b.Point)
		endpoints[event] = i
		models[event] = b
		return b, i, nil
	}
	count := 0
	for pieceIndex, piece := range p.Cut {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		first, fi, e := getEndpoint(piece.StartPoint)
		if e != nil {
			return empty, e
		}
		last, li, e := getEndpoint(piece.EndPoint)
		if e != nil {
			return empty, e
		}
		ro, e := rootOptions()
		if e != nil {
			return empty, e
		}
		order, e := piece.FreeStart.Compare(ctx, piece.FreeEnd, ro)
		out.ExactOperations += order.Operations
		if e != nil {
			return empty, e
		}
		if !order.Proven || order.Order == 0 {
			return empty, geom.ErrDegenerate
		}
		a := AlgebraicTubeRunSample{Free: piece.FreeStart, Point: first.Point, Model: first}
		b := AlgebraicTubeRunSample{Free: piece.FreeEnd, Point: last.Point, Model: last}
		type task struct {
			a, b  AlgebraicTubeRunSample
			depth int
		}
		stack := []task{{a, b, 0}}
		samples := []AlgebraicTubeRunSample{a}
		ids := []int{fi}
		var bounds []*big.Rat
		_, domain, _, e := piece.Segment.ExactFreeDomain()
		if e != nil {
			return empty, e
		}
		for len(stack) > 0 {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			x := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if count+len(stack) >= d.MaxSegments {
				return empty, geom.ErrConvergence
			}
			width := new(big.Rat).Sub(domain.Upper, domain.Lower)
			width.Quo(width, new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(x.depth+16))))
			intervals := [2]geom.ExactParameterInterval{}
			for i, s := range []AlgebraicTubeRunSample{x.a, x.b} {
				if s.Rational != nil {
					intervals[i] = geom.ExactParameterInterval{Lower: s.Rational, Upper: s.Rational}
					continue
				}
				ro, e := rootOptions()
				if e != nil {
					return empty, e
				}
				en, e := s.Free.Enclose(ctx, width, ro)
				out.ExactOperations += en.Operations
				if e != nil {
					return empty, e
				}
				if !en.Converged {
					return empty, geom.ErrConvergence
				}
				intervals[i] = en.Interval
			}
			low, high := intervals[0], intervals[1]
			if order.Order > 0 {
				low, high = high, low
			}
			if low.Upper.Cmp(high.Lower) >= 0 {
				return empty, fmt.Errorf("piece %d overlapping free brackets: %w", pieceIndex, geom.ErrConvergence)
			}
			lo, _ := low.Lower.Float64()
			hi, _ := high.Upper.Float64()
			if !finite(lo) || !finite(hi) {
				return empty, geom.ErrUnrepresentable
			}
			if ratInset(lo).Cmp(low.Lower) > 0 {
				lo = math.Nextafter(lo, math.Inf(-1))
			}
			if ratInset(hi).Cmp(high.Upper) < 0 {
				hi = math.Nextafter(hi, math.Inf(1))
			}
			n, e := remaining()
			if e != nil {
				return empty, e
			}
			io := d.Interval
			if io.MaxOperations == 0 || io.MaxOperations > n {
				io.MaxOperations = n
			}
			en, e := piece.Segment.EncloseFreeInterval(ctx, lo, hi, io)
			out.ExactOperations += en.ExactOperations
			if e != nil && !(errors.Is(e, geom.ErrConvergence) && en.HasBounds) {
				return empty, fmt.Errorf("piece %d interval [%g,%g]: %w", pieceIndex, lo, hi, e)
			}
			if !en.HasBounds || !en.Bounds.Valid() {
				return empty, geom.ErrConvergence
			}
			squared := tubeBoxChordSquared(en.Bounds, x.a.Point, x.b.Point)
			target := ratInset(d.Deflection)
			target.Mul(target, target)
			if squared.Cmp(target) <= 0 {
				if x.a.Point == x.b.Point {
					return empty, geom.ErrDegenerate
				}
				id := li
				if len(stack) > 0 {
					id = len(out.Vertices)
					out.Vertices = append(out.Vertices, x.b.Point)
				}
				samples = append(samples, x.b)
				ids = append(ids, id)
				bounds = append(bounds, squared)
				count++
				continue
			}
			if x.depth >= d.MaxDepth {
				return empty, geom.ErrConvergence
			}
			mid := new(big.Rat).Quo(new(big.Rat).Add(low.Upper, high.Lower), big.NewRat(2, 1))
			if mid.Num().BitLen() > d.MaxInputBits || mid.Denom().BitLen() > d.MaxInputBits {
				return empty, geom.ErrConvergence
			}
			n, e = remaining()
			if e != nil {
				return empty, e
			}
			po := d.Point
			if po.MaxOperations == 0 || po.MaxOperations > n {
				po.MaxOperations = n
			}
			cert, e := piece.Segment.EvaluateRationalProjectedGraph(ctx, mid, r.partitionSource.projection, po)
			out.ExactOperations += cert.ExactOperations
			if e != nil {
				return empty, e
			}
			if !cert.Proven || cert.ModelEvent == nil {
				return empty, geom.ErrConvergence
			}
			n, e = remaining()
			if e != nil {
				return empty, e
			}
			tol := d.Deflection / 16
			if tol == 0 {
				tol = d.Deflection
			}
			if d.Point.Tolerance > 0 && d.Point.Tolerance < tol {
				tol = d.Point.Tolerance
			}
			model, e := cert.ModelEvent.Enclose(ctx, geom.ComposedCurveEndpointOptions{Tolerance: tol, ExactCurvePlaneEventOptions: geom.ExactCurvePlaneEventOptions{MaxOperations: n, MaxCoefficientBits: d.MaxInputBits}})
			out.ExactOperations += model.ExactOperations
			if e != nil {
				return empty, e
			}
			if !model.Converged || !model.HasRepresentative {
				return empty, geom.ErrConvergence
			}
			m := AlgebraicTubeRunSample{Rational: mid, Point: model.Point, Model: model}
			stack = append(stack, task{m, x.b, x.depth + 1}, task{x.a, m, x.depth + 1})
		}
		out.Samples = append(out.Samples, samples)
		out.PieceVertices = append(out.PieceVertices, ids)
		out.SquaredErrors = append(out.SquaredErrors, bounds)
	}
	return out, nil
}

func tubeBoxChordSquared(box geom.Box, points ...geom.Vec3) *big.Rat {
	maximum := new(big.Rat)
	for _, point := range points {
		sum := new(big.Rat)
		p, a, b := exactBoundPoint(point), exactBoundPoint(box.Min), exactBoundPoint(box.Max)
		for axis := 0; axis < 3; axis++ {
			x := new(big.Rat).Sub(a[axis], p[axis])
			x.Mul(x, x)
			y := new(big.Rat).Sub(b[axis], p[axis])
			y.Mul(y, y)
			if y.Cmp(x) > 0 {
				x = y
			}
			sum.Add(sum, x)
		}
		if sum.Cmp(maximum) > 0 {
			maximum.Set(sum)
		}
	}
	return maximum
}
