package trimmed

import (
	"container/heap"
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// MixedRoofDistanceOptions bounds exact branch-and-bound distance. SquaredGap
// defaults to 1/100000; MaxDepth to 32 and MaxCells to 16384.
type MixedRoofDistanceOptions struct {
	geom.SurfacePatchEvaluationOptions
	SquaredGap         *big.Rat
	MaxDepth, MaxCells int
}

// MixedRoofDistance encloses global squared distance to the original exposed
// boundary. Witness is an actual rational boundary point, possibly not a
// minimizer. Owners identify the original supports carrying that witness.
// Converged means the exact requested gap was met. A work limit may return a
// valid wider interval with ErrConvergence; cancellation publishes nothing.
type MixedRoofDistance struct {
	Squared                       geom.ExactParameterInterval
	Witness                       [3]*big.Rat
	Owners                        []MixedRoofFaceOwner
	VisitedCells, ExactOperations int
	Converged                     bool
}

type mixedRoofDistanceCell struct {
	patch *geom.SurfacePatch
	lo    *big.Rat
	owner MixedRoofFaceOwner
	depth int
}
type mixedRoofDistanceHeap []mixedRoofDistanceCell

func (h mixedRoofDistanceHeap) Len() int           { return len(h) }
func (h mixedRoofDistanceHeap) Less(i, j int) bool { return h[i].lo.Cmp(h[j].lo) < 0 }
func (h mixedRoofDistanceHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *mixedRoofDistanceHeap) Push(v any)        { *h = append(*h, v.(mixedRoofDistanceCell)) }
func (h *mixedRoofDistanceHeap) Pop() any {
	a := *h
	n := len(a) - 1
	v := a[n]
	a[n] = mixedRoofDistanceCell{}
	*h = a[:n]
	return v
}

type mixedRoofBoundBox [3]geom.ExactParameterInterval

func mixedRoofBoundHull(b geom.Box) mixedRoofBoundBox {
	return mixedRoofBoundBox{
		{Lower: ratInset(b.Min.X), Upper: ratInset(b.Max.X)},
		{Lower: ratInset(b.Min.Y), Upper: ratInset(b.Max.Y)},
		{Lower: ratInset(b.Min.Z), Upper: ratInset(b.Max.Z)},
	}
}

// roofBoxLocation excludes a support cell only by a strict interval sign.
// Intersections near the roof, side or floor stay in the global distance queue.
func (b *mixedRoofBox) roofBoxLocation(box mixedRoofBoundBox, w *mixedRoofLineWork) (parametric.Location, error) {
	d := b.domain
	begin, end := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
	if box[2].Upper.Cmp(b.floor) < 0 {
		return parametric.Outside, nil
	}
	inside := box[2].Lower.Cmp(b.floor) > 0
	var coordinate [2]geom.ExactParameterInterval
	for i := 0; i < 2; i++ {
		a, c := ratInset(begin[i]), ratInset(end[i])
		if box[i].Upper.Cmp(a) < 0 || box[i].Lower.Cmp(c) > 0 {
			return parametric.Outside, nil
		}
		inside = inside && box[i].Lower.Cmp(a) > 0 && box[i].Upper.Cmp(c) < 0
		width, err := w.rat(new(big.Rat).Sub(c, a))
		if err != nil {
			return 0, err
		}
		for j, t := range [2]*big.Rat{maxRat(a, box[i].Lower), minRat(c, box[i].Upper)} {
			n, err := w.rat(new(big.Rat).Sub(t, a))
			if err != nil {
				return 0, err
			}
			if _, err = w.rat(n.Quo(n, width)); err != nil {
				return 0, err
			}
			if j == 0 {
				coordinate[i].Lower = n
			} else {
				coordinate[i].Upper = n
			}
		}
	}
	height, err := b.roofHeightRange(coordinate, w)
	if err != nil {
		return 0, err
	}
	if box[2].Lower.Cmp(height.Upper) > 0 {
		return parametric.Outside, nil
	}
	if inside && box[2].Upper.Cmp(height.Lower) < 0 {
		return parametric.Inside, nil
	}
	return parametric.Boundary, nil // unresolved, not a claim of coincident geometry
}

// Coordinates are in [0,1]. Bound every positive monomial by its two
// endpoint products, using signed interval sums for the original polynomial.
func (b *mixedRoofBox) roofHeightRange(uv [2]geom.ExactParameterInterval, w *mixedRoofLineWork) (geom.ExactParameterInterval, error) {
	num, err := mixedRoofPowerRange(b.power, uv, w)
	if err != nil || b.minWeight == nil {
		return num, err
	}
	weight, err := mixedRoofPowerRange(b.weight, uv, w)
	if err != nil {
		return geom.ExactParameterInterval{}, err
	}
	minimum := maxRat(weight.Lower, b.minWeight)
	var out geom.ExactParameterInterval
	for _, n := range [2]*big.Rat{num.Lower, num.Upper} {
		for _, d := range [2]*big.Rat{minimum, weight.Upper} {
			z, err := w.rat(new(big.Rat).Quo(n, d))
			if err != nil {
				return geom.ExactParameterInterval{}, err
			}
			if out.Lower == nil || z.Cmp(out.Lower) < 0 {
				out.Lower = z
			}
			if out.Upper == nil || z.Cmp(out.Upper) > 0 {
				out.Upper = z
			}
		}
	}
	return out, nil
}

func mixedRoofPowerRange(power [3][3]*big.Rat, uv [2]geom.ExactParameterInterval, w *mixedRoofLineWork) (geom.ExactParameterInterval, error) {
	out := geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	var powers [2][3][2]*big.Rat
	for i, s := range uv {
		a, c := s.Lower, s.Upper
		powers[i][0] = [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}
		powers[i][1] = [2]*big.Rat{a, c}
		for j, x := range powers[i][1] {
			z, err := w.rat(new(big.Rat).Mul(x, x))
			if err != nil {
				return out, err
			}
			powers[i][2][j] = z
		}
	}
	for i, row := range power {
		for j, v := range row {
			if v.Sign() == 0 {
				continue
			}
			for k, dst := range [2]*big.Rat{out.Lower, out.Upper} {
				a := k
				if v.Sign() < 0 {
					a = 1 - a
				}
				z, err := w.rat(new(big.Rat).Mul(powers[0][i][a], powers[1][j][a]))
				if err != nil {
					return out, err
				}
				if _, err = w.rat(z.Mul(z, v)); err != nil {
					return out, err
				}
				if _, err = w.rat(dst.Add(dst, z)); err != nil {
					return out, err
				}
			}
		}
	}
	return out, nil
}

func (w *mixedRoofLineWork) pointHullDistance(query [3]*big.Rat, box mixedRoofBoundBox) (*big.Rat, error) {
	sum := new(big.Rat)
	for i, s := range box {
		var p *big.Rat
		if query[i].Cmp(s.Lower) < 0 {
			p = s.Lower
		}
		if query[i].Cmp(s.Upper) > 0 {
			p = s.Upper
		}
		if p == nil {
			continue
		}
		d, err := w.rat(new(big.Rat).Sub(query[i], p))
		if err != nil {
			return nil, err
		}
		if _, err = w.rat(d.Mul(d, d)); err != nil {
			return nil, err
		}
		if _, err = w.rat(sum.Add(sum, d)); err != nil {
			return nil, err
		}
	}
	return sum, nil
}

// BoundBoundaryDistanceRational searches all twelve independently proved
// original supports. An outward bound on the opposing roof box certifies
// omitted cells. Unresolved trim cells stay in the global lower bound and
// only points classified on the exposed material can lower the upper bound.
func (q *MixedRoofMaterialQuery) BoundBoundaryDistanceRational(ctx context.Context, point [3]*big.Rat, o MixedRoofDistanceOptions) (MixedRoofDistance, error) {
	var zero MixedRoofDistance
	if o.SquaredGap == nil {
		o.SquaredGap = big.NewRat(1, 100000)
	}
	var err error
	o.SurfacePatchEvaluationOptions, err = q.evaluationOptions(ctx, o.SurfacePatchEvaluationOptions, point[0], point[1], point[2], o.SquaredGap)
	if err != nil {
		return zero, err
	}
	if o.MaxDepth < 0 || o.MaxDepth > 48 || o.MaxCells < 0 || o.MaxCells > 262144 || o.SquaredGap.Sign() <= 0 {
		return zero, geom.ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	if o.MaxCells == 0 {
		o.MaxCells = 16384
	}
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o.SurfacePatchEvaluationOptions}}
	loc, err := q.LocateRational(ctx, point, o.SurfacePatchEvaluationOptions)
	if err != nil {
		return zero, err
	}
	w.operations = loc.ExactOperations
	if loc.Location == parametric.Boundary {
		owners, err := q.roofPointOwners(ctx, point, &w)
		if err != nil {
			return zero, err
		}
		out := MixedRoofDistance{
			Squared: geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)},
			Owners:  owners, Converged: true, ExactOperations: w.operations,
		}
		for i, x := range point {
			out.Witness[i] = new(big.Rat).Set(x)
		}
		return out, nil
	}
	queue := mixedRoofDistanceHeap{}
	push := func(p *geom.SurfacePatch, owner MixedRoofFaceOwner, depth int) error {
		b, err := p.Bounds()
		if err != nil {
			return err
		}
		lo, err := w.pointHullDistance(point, mixedRoofBoundHull(b))
		if err != nil {
			return err
		}
		heap.Push(&queue, mixedRoofDistanceCell{p, lo, owner, depth})
		return nil
	}
	for i, b := range q.box {
		for j, p := range b.patches {
			if err := push(p, MixedRoofFaceOwner{i, j}, 0); err != nil {
				return zero, err
			}
		}
	}
	out := MixedRoofDistance{}
	var upper, unresolved *big.Rat
	lower := func() *big.Rat {
		l := upper
		if unresolved != nil && (l == nil || unresolved.Cmp(l) < 0) {
			l = unresolved
		}
		if len(queue) != 0 && (l == nil || queue[0].lo.Cmp(l) < 0) {
			l = queue[0].lo
		}
		return l
	}
	finish := func(err error) (MixedRoofDistance, error) {
		if err != nil && !errors.Is(err, geom.ErrConvergence) {
			return zero, err
		}
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		out.ExactOperations = w.operations
		l := lower()
		if l != nil {
			out.Squared.Lower = new(big.Rat).Set(l)
		}
		if upper != nil {
			out.Squared.Upper = new(big.Rat).Set(upper)
			out.Converged = l != nil && new(big.Rat).Sub(upper, l).Cmp(o.SquaredGap) <= 0
		}
		if out.Converged {
			return out, nil
		}
		return out, geom.ErrConvergence
	}
	hold := func(lo *big.Rat) {
		if unresolved == nil || lo.Cmp(unresolved) < 0 {
			unresolved = lo
		}
	}
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		if upper != nil && new(big.Rat).Sub(upper, lower()).Cmp(o.SquaredGap) <= 0 {
			return finish(nil)
		}
		if out.VisitedCells >= o.MaxCells || w.operations >= w.o.MaxOperations {
			return finish(geom.ErrConvergence)
		}
		cell := heap.Pop(&queue).(mixedRoofDistanceCell)
		out.VisitedCells++
		if upper != nil && cell.lo.Cmp(upper) >= 0 {
			continue
		}
		fail := func(e error) (MixedRoofDistance, error) { hold(cell.lo); return finish(e) }
		box, err := cell.patch.Bounds()
		if err != nil {
			return fail(err)
		}
		other := q.box[cell.owner.Operand^1]
		wantInside := q.op == parametric.Intersection || q.op == parametric.Difference && cell.owner.Operand == 1
		relation, err := other.roofBoxLocation(mixedRoofBoundHull(box), &w)
		if err != nil {
			return fail(err)
		}
		want := parametric.Outside
		if wantInside {
			want = parametric.Inside
		}
		if relation != parametric.Boundary && relation != want {
			continue
		}

		d := cell.patch.Domain()
		u := new(big.Rat).Quo(new(big.Rat).Add(ratInset(d.UMin), ratInset(d.UMax)), big.NewRat(2, 1))
		v := new(big.Rat).Quo(new(big.Rat).Add(ratInset(d.VMin), ratInset(d.VMax)), big.NewRat(2, 1))
		opt := o.SurfacePatchEvaluationOptions
		opt.MaxOperations -= w.operations
		if opt.MaxOperations < 1 {
			return fail(geom.ErrConvergence)
		}
		j, err := cell.patch.EvaluateRational(ctx, u, v, opt)
		w.operations += j.ExactOperations
		if err != nil {
			return fail(err)
		}
		// Strictly retained cells need no point classifier. In unresolved
		// cells, accept only an exposed point and obtain its full owner set.
		onBoundary := relation == want
		if relation == parametric.Boundary {
			opt.MaxOperations = o.MaxOperations - w.operations
			if opt.MaxOperations < 1 {
				return fail(geom.ErrConvergence)
			}
			r, err := q.LocateRational(ctx, j.Point, opt)
			if err != nil {
				return fail(err)
			}
			w.operations += r.ExactOperations
			onBoundary = r.Location == parametric.Boundary
		}
		if onBoundary {
			p := mixedRoofBoundBox{}
			for i, z := range j.Point {
				p[i] = geom.ExactParameterInterval{Lower: z, Upper: z}
			}
			z, err := w.pointHullDistance(point, p)
			if err != nil {
				return fail(err)
			}
			if upper == nil || z.Cmp(upper) < 0 {
				owners, err := q.roofPointOwners(ctx, j.Point, &w)
				if err != nil {
					return fail(err)
				}
				matches := false
				for _, owner := range owners {
					matches = matches || owner == cell.owner
				}
				if !matches {
					return fail(geom.ErrConvergence)
				}
				upper, out.Witness, out.Owners = z, j.Point, owners
			}
		}
		if upper != nil && cell.lo.Cmp(upper) >= 0 {
			continue
		}
		if cell.depth >= o.MaxDepth {
			hold(cell.lo)
			continue
		}
		a, b := d, d
		if d.UMax-d.UMin >= d.VMax-d.VMin {
			m := d.UMin/2 + d.UMax/2
			if m <= d.UMin || m >= d.UMax {
				hold(cell.lo)
				continue
			}
			a.UMax, b.UMin = m, m
		} else {
			m := d.VMin/2 + d.VMax/2
			if m <= d.VMin || m >= d.VMax {
				hold(cell.lo)
				continue
			}
			a.VMax, b.VMin = m, m
		}
		for _, r := range [2]geom.ParameterRectangle{a, b} {
			left := o.MaxOperations - w.operations
			if left < 1 {
				return fail(geom.ErrConvergence)
			}
			p, n, err := cell.patch.TrimWithWorkLimit(r, left)
			w.operations += n
			if err != nil {
				return fail(err)
			}
			if err := push(p, cell.owner, cell.depth+1); err != nil {
				return fail(err)
			}
		}
	}
	return finish(nil)
}

func (q *MixedRoofMaterialQuery) roofPointOwners(ctx context.Context, p [3]*big.Rat, w *mixedRoofLineWork) ([]MixedRoofFaceOwner, error) {
	var owners []MixedRoofFaceOwner
	for i, b := range q.box {
		loc, sides := b.footprintSides([2]*big.Rat{p[0], p[1]})
		if loc == parametric.Outside || p[2].Cmp(b.floor) < 0 {
			continue
		}
		o := w.o.SurfacePatchEvaluationOptions
		o.MaxOperations -= w.operations
		if o.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		j, err := b.roof.EvaluateRational(ctx, p[0], p[1], o)
		w.operations += j.ExactOperations
		if err != nil {
			return nil, err
		}
		cmp := p[2].Cmp(j.Point[2])
		if cmp > 0 {
			continue
		}
		if p[2].Cmp(b.floor) == 0 {
			owners = append(owners, MixedRoofFaceOwner{i, b.floorFace})
		}
		if cmp == 0 {
			owners = append(owners, MixedRoofFaceOwner{i, b.roofFace})
		}
		for k, f := range b.sideFaces {
			if sides&(1<<k) != 0 {
				owners = append(owners, MixedRoofFaceOwner{i, f})
			}
		}
	}
	if len(owners) == 0 {
		return nil, geom.ErrConvergence
	}
	return owners, nil
}
