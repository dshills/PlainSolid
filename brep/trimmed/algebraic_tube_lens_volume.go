package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type AlgebraicTubeLensVolumeOptions struct {
	Patch              geom.SurfacePatchOptions
	MaxDepth, MaxCells int
}

type AlgebraicTubeLensVolumeBounds struct {
	Volume                        geom.ExactParameterInterval
	VisitedCells, ExactOperations int
}

type AlgebraicTubeLensMassBounds struct {
	AlgebraicTubeLensVolumeBounds
	// FirstMoment encloses the unnormalized first moments in model XYZ.
	// Centroid is present when the positive volume lower bound is proved.
	FirstMoment, Centroid [3]geom.ExactParameterInterval
	// Inertia is about the model origin at unit density, using the
	// conventional negative off-diagonal products.
	Inertia [3][3]geom.ExactParameterInterval
}

// BoundVolume integrates the positive part of the signed height difference
// over the common projection of rectangular faces. Complete section coverage and
// the single interior loop identify that positive part with the lens.
// Positive homogeneous weights bound each cell, and an exact projected
// Jacobian converts UV area to model area. Returned intervals include all
// rounding and need not have small width at a given depth.
func (l AlgebraicTubeLens) BoundVolume(ctx context.Context, o AlgebraicTubeLensVolumeOptions) (AlgebraicTubeLensVolumeBounds, error) {
	r, e := l.boundLensMass(ctx, o, false)
	return r.AlgebraicTubeLensVolumeBounds, e
}

// BoundMassProperties encloses volume, first moments and centroid without
// meshing or quadrature. It has the same rectangular-chart scope as
// BoundVolume. A zero lower volume leaves Centroid unavailable.
func (l AlgebraicTubeLens) BoundMassProperties(ctx context.Context, o AlgebraicTubeLensVolumeOptions) (AlgebraicTubeLensMassBounds, error) {
	return l.boundLensMass(ctx, o, true)
}

func (l AlgebraicTubeLens) boundLensMass(ctx context.Context, o AlgebraicTubeLensVolumeOptions, moments bool) (r AlgebraicTubeLensMassBounds, err error) {
	if l.graphs[0] == nil {
		return r, geom.ErrInvalid
	}
	m, e := l.graphs[0].InverseCoefficients()
	if e != nil {
		return r, e
	}
	return boundTubeLensMass(ctx, l.section, l.upper, [2]int{0, 1}, m, o, moments)
}

func boundTubeLensMass(ctx context.Context, section AlgebraicTrimmedPairSection, upper int, axes [2]int,
	m [2][3]*big.Rat, o AlgebraicTubeLensVolumeOptions, moments bool) (r AlgebraicTubeLensMassBounds, err error) {
	source := section.runSource()
	if ctx == nil || source == nil || o.MaxDepth < 0 || o.MaxDepth > 20 ||
		o.MaxCells < 0 || o.MaxCells > 1048576 {
		return r, geom.ErrInvalid
	}
	defer func() {
		if err != nil {
			r.Volume = geom.ExactParameterInterval{}
			r.FirstMoment, r.Centroid = [3]geom.ExactParameterInterval{}, [3]geom.ExactParameterInterval{}
			r.Inertia = [3][3]geom.ExactParameterInterval{}
		}
	}()
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 10
	}
	if o.MaxCells == 0 {
		o.MaxCells = 32768
	}
	if o.Patch.MaxOperations == 0 {
		o.Patch.MaxOperations = 64000000
	}
	if o.Patch.MaxCoefficientBits == 0 {
		o.Patch.MaxCoefficientBits = 16384
	}
	if o.Patch.MaxOperations < 1 || o.Patch.MaxCoefficientBits < 64 {
		return r, geom.ErrInvalid
	}
	child := func() geom.SurfacePatchOptions {
		p := o.Patch
		p.MaxOperations -= r.ExactOperations
		return p
	}
	transform := func(s *geom.SurfacePatch, placement geom.Affine) (*geom.SurfacePatch, error) {
		remaining := child().MaxOperations
		if remaining < 1 {
			return nil, geom.ErrConvergence
		}
		out, work, e := s.TransformedWithWorkLimit(placement, remaining)
		r.ExactOperations += work
		return out, e
	}
	trim := func(s *geom.SurfacePatch, d geom.ParameterRectangle) (*geom.SurfacePatch, error) {
		if s.Domain() == d {
			return s, nil
		}
		remaining := child().MaxOperations
		if remaining < 1 {
			return nil, geom.ErrConvergence
		}
		out, work, e := s.TrimWithWorkLimit(d, remaining)
		r.ExactOperations += work
		return out, e
	}
	var patches [2]*geom.SurfacePatch
	order := [3]int{axes[0], axes[1], 3 - axes[0] - axes[1]}
	for i, f := range source.faces {
		d, e := faceUVBounds(f)
		if e != nil {
			return r, e
		}
		if !splineDistanceRectangle(f, d) {
			return r, geom.ErrConvergence
		}
		p := child()
		if p.MaxOperations < 1 {
			return r, geom.ErrConvergence
		}
		base, placement := parametric.SurfaceMapping(f.Surface)
		s, e := exactSplineSupportPatch(base, p)
		if e != nil {
			return r, e
		}
		r.ExactOperations += s.ExactOperations()
		if placement != geom.Identity() {
			s, e = transform(s, placement)
			if e != nil {
				return r, e
			}
		}
		patches[i], e = trim(s, d)
		if e != nil {
			return r, e
		}
		if axes != [2]int{0, 1} {
			var move geom.Affine
			for i, k := range order {
				move[i][k] = 1
			}
			q, e := transform(patches[i], move)
			if e != nil {
				return r, e
			}
			patches[i] = q
		}
		if r.ExactOperations >= o.Patch.MaxOperations {
			return r, geom.ErrConvergence
		}
	}
	align := func() (*geom.AffineXYChartAlignment, error) {
		p := child()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		a, work, e := geom.AlignAffineXYChartsWithWork(ctx, patches[0], patches[1], p)
		r.ExactOperations += work
		return a, e
	}
	alignment, e := align()
	if errors.Is(e, geom.ErrDomain) {
		// Both supports were proved graph charts and the lens lies in
		// their common projection. Only the shared rectangle can contribute
		// to the material integral. Verify its original trimmed supports.
		p := child()
		if p.MaxOperations < 1 {
			return r, geom.ErrConvergence
		}
		clip, clipErr := geom.ClipAffineXYGraphOverlap(ctx, patches[0], patches[1], p)
		r.ExactOperations += min(clip.ExactOperations, p.MaxOperations)
		if clipErr != nil || clip.Disjoint {
			if clipErr == nil {
				clipErr = geom.ErrConvergence
			}
			return r, clipErr
		}
		for i, bounds := range [2]geom.ParameterRectangle{clip.A, clip.B} {
			var e error
			patches[i], e = trim(patches[i], bounds)
			if e != nil {
				return r, e
			}
		}
		alignment, e = align()
	}
	if e != nil {
		return r, e
	}
	p := child()
	if p.MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	d, e := geom.NewGeneralSurfaceHeightDifferencePatch(ctx, patches[0], alignment.TargetOnSource(),
		geom.GeneralSurfaceHeightOptions{MaxPoles: p.MaxPoles, MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits})
	if e != nil {
		return r, e
	}
	r.ExactOperations += d.ExactOperations()
	det := new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).Mul(m[0][0], m[1][1]), new(big.Rat).Mul(m[0][1], m[1][0])))
	jacobian := new(big.Rat).Inv(det)
	type cell struct {
		p          *geom.SurfacePatch
		s          [2]*geom.SurfacePatch
		depth      int
		lo, hi     *big.Rat
		coordinate [3]geom.ExactParameterInterval
		height     [2]geom.ExactParameterInterval
	}
	r.Volume = geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	if moments {
		for i := range r.FirstMoment {
			r.FirstMoment[i] = geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
			for j := range r.Inertia[i] {
				r.Inertia[i][j] = geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
			}
		}
	}
	queue := []cell{{p: d, s: [2]*geom.SurfacePatch{patches[0], alignment.TargetOnSource()}}}
	for len(queue) > 0 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		if r.VisitedCells >= o.MaxCells {
			return r, geom.ErrConvergence
		}
		x := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		r.VisitedCells++
		b, e := x.p.Bounds()
		if e != nil {
			return r, e
		}
		if !finite(b.Min.Z) || !finite(b.Max.Z) {
			return r, geom.ErrConvergence
		}
		lo, hi := ratInset(b.Min.Z), ratInset(b.Max.Z)
		if upper == 1 {
			lo, hi = new(big.Rat).Neg(hi), new(big.Rat).Neg(lo)
		}
		if x.lo != nil && lo.Cmp(x.lo) < 0 {
			lo = x.lo
		}
		if x.hi != nil && hi.Cmp(x.hi) > 0 {
			hi = x.hi
		}
		if hi.Sign() <= 0 {
			continue
		}
		if lo.Sign() < 0 {
			lo = new(big.Rat)
		}
		if lo.Cmp(hi) > 0 {
			return r, geom.ErrConvergence
		}
		box := x.p.Domain()
		coordinates := x.coordinate
		heights := x.height
		if moments {
			for axis, a := range [2][2]float64{{b.Min.X, b.Max.X}, {b.Min.Y, b.Max.Y}} {
				if !finite(a[0]) || !finite(a[1]) {
					return r, geom.ErrConvergence
				}
				coordinates[axis] = lensNestedInterval(coordinates[axis], ratInset(a[0]), ratInset(a[1]))
			}
			var z [2]geom.Box
			for i, s := range x.s {
				z[i], e = s.Bounds()
				if e != nil {
					return r, e
				}
				if !finite(z[i].Min.Z) || !finite(z[i].Max.Z) {
					return r, geom.ErrConvergence
				}
				heights[i] = lensNestedInterval(heights[i], ratInset(z[i].Min.Z), ratInset(z[i].Max.Z))
			}
			average := func(a, b float64) *big.Rat {
				return new(big.Rat).Quo(new(big.Rat).Add(ratInset(a), ratInset(b)), big.NewRat(2, 1))
			}
			coordinates[2] = lensNestedInterval(coordinates[2], average(z[0].Min.Z, z[1].Min.Z), average(z[0].Max.Z, z[1].Max.Z))
		}
		if x.depth < o.MaxDepth && lo.Cmp(hi) != 0 {
			left, right := box, box
			if box.UMax-box.UMin >= box.VMax-box.VMin {
				m := box.UMin/2 + box.UMax/2
				left.UMax, right.UMin = m, m
			} else {
				m := box.VMin/2 + box.VMax/2
				left.VMax, right.VMin = m, m
			}
			if !left.Valid() || !right.Valid() {
				return r, geom.ErrConvergence
			}
			for _, b := range [2]geom.ParameterRectangle{right, left} {
				p, e := trim(x.p, b)
				if e != nil {
					return r, e
				}
				next := cell{p: p, depth: x.depth + 1, lo: lo, hi: hi, coordinate: coordinates, height: heights}
				if moments {
					for i, s := range x.s {
						q, e := trim(s, b)
						if e != nil {
							return r, e
						}
						next.s[i] = q
					}
				}
				queue = append(queue, next)
			}
			continue
		}
		width := new(big.Rat).Sub(ratInset(box.UMax), ratInset(box.UMin))
		height := new(big.Rat).Sub(ratInset(box.VMax), ratInset(box.VMin))
		area := new(big.Rat).Mul(new(big.Rat).Mul(width, height), jacobian)
		v := geom.ExactParameterInterval{Lower: new(big.Rat).Mul(area, lo), Upper: new(big.Rat).Mul(area, hi)}
		r.Volume.Lower.Add(r.Volume.Lower, v.Lower)
		r.Volume.Upper.Add(r.Volume.Upper, v.Upper)
		if moments {
			for i, c := range coordinates {
				p := lensIntervalProduct(v, c)
				r.FirstMoment[i].Lower.Add(r.FirstMoment[i].Lower, p.Lower)
				r.FirstMoment[i].Upper.Add(r.FirstMoment[i].Upper, p.Upper)
			}
			squared := [3]geom.ExactParameterInterval{lensIntervalSquare(coordinates[0]), lensIntervalSquare(coordinates[1]), {}}
			// Integral z² dz divided by thickness equals
			// (upper² + upper*lower + lower²)/3.
			a, b := lensIntervalSquare(heights[0]), lensIntervalSquare(heights[1])
			c := lensIntervalProduct(heights[0], heights[1])
			squared[2] = lensIntervalScale(lensIntervalAdd(lensIntervalAdd(a, b), c), big.NewRat(1, 3))
			if squared[2].Lower.Sign() < 0 {
				squared[2].Lower = new(big.Rat)
			}
			for i := range r.Inertia {
				for j := i; j < 3; j++ {
					var term geom.ExactParameterInterval
					if i == j {
						term = lensIntervalAdd(squared[(i+1)%3], squared[(i+2)%3])
					} else {
						term = lensIntervalScale(lensIntervalProduct(coordinates[i], coordinates[j]), big.NewRat(-1, 1))
					}
					p := lensIntervalProduct(v, term)
					r.Inertia[i][j].Lower.Add(r.Inertia[i][j].Lower, p.Lower)
					r.Inertia[i][j].Upper.Add(r.Inertia[i][j].Upper, p.Upper)
				}
			}
		}
	}
	if moments && r.Volume.Lower.Sign() > 0 {
		inv := geom.ExactParameterInterval{Lower: new(big.Rat).Inv(r.Volume.Upper), Upper: new(big.Rat).Inv(r.Volume.Lower)}
		for i, m := range r.FirstMoment {
			r.Centroid[i] = lensIntervalProduct(m, inv)
		}
	}
	if moments {
		for i := range r.Inertia {
			for j := 0; j < i; j++ {
				r.Inertia[i][j] = geom.ExactParameterInterval{Lower: new(big.Rat).Set(r.Inertia[j][i].Lower), Upper: new(big.Rat).Set(r.Inertia[j][i].Upper)}
			}
		}
		if axes != [2]int{0, 1} {
			a, b, c := r.FirstMoment, r.Centroid, r.Inertia
			for i, k := range order {
				r.FirstMoment[k], r.Centroid[k] = a[i], b[i]
				for j, l := range order {
					r.Inertia[k][l] = c[i][j]
				}
			}
		}
	}
	return r, nil
}

func lensIntervalAdd(a, b geom.ExactParameterInterval) geom.ExactParameterInterval {
	return geom.ExactParameterInterval{Lower: new(big.Rat).Add(a.Lower, b.Lower), Upper: new(big.Rat).Add(a.Upper, b.Upper)}
}

func lensIntervalScale(a geom.ExactParameterInterval, x *big.Rat) geom.ExactParameterInterval {
	p, q := new(big.Rat).Mul(a.Lower, x), new(big.Rat).Mul(a.Upper, x)
	if x.Sign() < 0 {
		p, q = q, p
	}
	return geom.ExactParameterInterval{Lower: p, Upper: q}
}

func lensIntervalSquare(a geom.ExactParameterInterval) geom.ExactParameterInterval {
	x, y := new(big.Rat).Mul(a.Lower, a.Lower), new(big.Rat).Mul(a.Upper, a.Upper)
	lo, hi := minRat(x, y), maxRat(x, y)
	if a.Lower.Sign() <= 0 && a.Upper.Sign() >= 0 {
		lo = new(big.Rat)
	}
	return geom.ExactParameterInterval{Lower: lo, Upper: hi}
}

func lensNestedInterval(parent geom.ExactParameterInterval, lo, hi *big.Rat) geom.ExactParameterInterval {
	if parent.Lower != nil {
		lo = maxRat(lo, parent.Lower)
		hi = minRat(hi, parent.Upper)
	}
	return geom.ExactParameterInterval{Lower: lo, Upper: hi}
}

func lensIntervalProduct(a, b geom.ExactParameterInterval) geom.ExactParameterInterval {
	x := new(big.Rat).Mul(a.Lower, b.Lower)
	lo, hi := x, x
	for _, z := range [3][2]*big.Rat{{a.Lower, b.Upper}, {a.Upper, b.Lower}, {a.Upper, b.Upper}} {
		x := new(big.Rat).Mul(z[0], z[1])
		lo, hi = minRat(lo, x), maxRat(hi, x)
	}
	return geom.ExactParameterInterval{Lower: lo, Upper: hi}
}
