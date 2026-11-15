package fillet

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// OrthogonalPrismOptions bounds original edge and disk comparisons and the
// exact circular output validation. Zero MaxWork selects 100,000 checks.
type OrthogonalPrismOptions struct {
	MaxWork  int
	Circular trimmed.ExactCircularLoopOptions
}

type orthogonalDirection [2]int64

// PrismOrthogonalCorners retains exact circles at selected convex or
// reentrant vertical corners of a simple CCW orthogonal prism. Reentrant
// rounds add material locally and have negative reported sweep. The original
// corner disks must be strictly clear of nonincident finite edges and each
// other. No original or tangent coordinate may be snapped.
func PrismOrthogonalCorners(ctx context.Context, polygon []geom.Vec2, zMin, zMax float64, selection []CornerRadius, o OrthogonalPrismOptions) (SelectiveResult, error) {
	var zero SelectiveResult
	n := len(polygon)
	if ctx == nil || n < 4 || len(selection) == 0 || len(selection) > n || o.MaxWork < 0 ||
		!finite(zMin) || !finite(zMax) || zMax <= zMin || !finite(zMax-zMin) {
		return zero, ErrInvalid
	}
	if n > 64 || len(selection) > 64-n {
		return zero, ErrLimit
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	remaining := o.MaxWork
	if remaining == 0 {
		remaining = 100000
	}
	tick := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if remaining == 0 {
			return ErrLimit
		}
		remaining--
		return nil
	}
	dir := make([]orthogonalDirection, n)
	length := make([]q, n)
	radii := make([]float64, n)
	for _, s := range selection {
		if s.Corner < 0 || s.Corner >= n || !finite(s.Radius) || s.Radius <= 0 || radii[s.Corner] != 0 {
			return zero, ErrInvalid
		}
		radii[s.Corner] = s.Radius
	}
	for _, p := range polygon {
		if !finite(p.X) || !finite(p.Y) {
			return zero, ErrInvalid
		}
	}
	source := make(trimvalidate.Loop, n)
	area2, perimeter := new(big.Rat), new(big.Rat)
	for i, a := range polygon {
		if err := tick(); err != nil {
			return zero, err
		}
		b := polygon[(i+1)%n]
		dx, dy := qs(num(b.X), num(a.X)), qs(num(b.Y), num(a.Y))
		if (dx.Sign() == 0) == (dy.Sign() == 0) {
			return zero, fmt.Errorf("%w: original sides must be nonzero and axis aligned", ErrInvalid)
		}
		dir[i] = orthogonalDirection{int64(dx.Sign()), int64(dy.Sign())}
		length[i] = new(big.Rat).Abs(qa(dx, dy))
		perimeter.Add(perimeter, length[i])
		area2.Add(area2, qs(qm(num(a.X), num(b.Y)), qm(num(a.Y), num(b.X))))
		source[i] = trimvalidate.Segment{Start: a, End: b}
	}
	if area2.Sign() <= 0 {
		return zero, fmt.Errorf("%w: expected a CCW original polygon", ErrInvalid)
	}
	for i, d := range dir {
		prev := dir[(i+n-1)%n]
		if prev[0]*d[1]-prev[1]*d[0] == 0 {
			return zero, fmt.Errorf("%w: original orthogonal sides must turn at every corner", ErrInvalid)
		}
		kept := qs(qs(length[i], num(radii[i])), num(radii[(i+1)%n]))
		if kept.Sign() <= 0 {
			return zero, fmt.Errorf("%w: no positive-length original side remains", ErrRadius)
		}
	}
	valid, err := trimvalidate.ValidateLoops(ctx, source, nil, o.Circular.Trims)
	if err != nil {
		return zero, err
	}
	if valid.Status != trimvalidate.Certified {
		return zero, fmt.Errorf("%w: original profile face: %s", ErrInvalid, valid.Reason)
	}

	nominalArea, _ := new(big.Rat).Quo(area2, big.NewRat(2, 1)).Float64()
	nominalPerimeter, _ := perimeter.Float64()
	r := SelectiveResult{Corners: make([]*Corner, n), StraightFaces: make([]int, n), Caps: [2]int{0, 1}}
	in, out := append([]geom.Vec2(nil), polygon...), append([]geom.Vec2(nil), polygon...)
	for i, p := range polygon {
		radius := radii[i]
		if radius == 0 {
			continue
		}
		if err := tick(); err != nil {
			return zero, err
		}
		prev, next := dir[(i+n-1)%n], dir[i]
		sign := prev[0]*next[1] - prev[1]*next[0]
		rad := num(radius)
		shift := func(x float64, s int64) q { return qa(num(x), qm(rad, big.NewRat(s, 1))) }
		px, py := shift(p.X, -prev[0]), shift(p.Y, -prev[1])
		cx := qa(px, qm(rad, big.NewRat(-sign*prev[1], 1)))
		cy := qa(py, qm(rad, big.NewRat(sign*prev[0], 1)))
		a, ok1 := exactOrthogonalPoint(px, py)
		b, ok2 := exactOrthogonalPoint(shift(p.X, next[0]), shift(p.Y, next[1]))
		center, ok3 := exactOrthogonalPoint(cx, cy)
		if !ok1 || !ok2 || !ok3 {
			return zero, fmt.Errorf("%w: corner %d exact tangent points or center", geom.ErrUnrepresentable, i)
		}
		sweep := float64(sign) * math.Pi / 2
		in[i], out[i] = a, b
		r.Corners[i] = &Corner{Center: center, Incoming: a, Outgoing: b, Sweep: sweep, Setback: radius}
		nominalArea -= float64(sign) * radius * radius * (1 - math.Pi/4)
		nominalPerimeter += radius * (math.Pi/2 - 2)
		r2 := qm(rad, rad)
		for j, a := range polygon {
			if j == i || j == (i+n-1)%n {
				continue
			}
			if err := tick(); err != nil {
				return zero, err
			}
			if orthogonalSegmentDistanceSquared(center, a, polygon[(j+1)%n]).Cmp(r2) <= 0 {
				return zero, fmt.Errorf("%w: corner %d disk meets nonincident original edge %d", ErrRadius, i, j)
			}
		}
		for j, c := range r.Corners[:i] {
			if c == nil {
				continue
			}
			if err := tick(); err != nil {
				return zero, err
			}
			sum := qa(rad, num(radii[j]))
			if squaredDistance(center, c.Center).Cmp(qm(sum, sum)) <= 0 {
				return zero, fmt.Errorf("%w: original corner disks %d and %d meet", ErrRadius, j, i)
			}
		}
	}
	pieces := make([]trimmed.ExactCircularLoopSegment, 0, n+len(selection))
	for i, c := range r.Corners {
		if c != nil {
			c.CylinderFace, c.BottomArc, c.TopArc = 2+len(pieces), len(pieces), n+len(selection)+len(pieces)
			pieces = append(pieces, trimmed.ExactCircularLoopSegment{Start: in[i], End: out[i], Center: c.Center, Radius: radii[i]})
		}
		r.StraightFaces[i] = 2 + len(pieces)
		pieces = append(pieces, trimmed.ExactCircularLoopSegment{Start: out[i], End: in[(i+1)%n]})
	}
	r.Solid, err = trimmed.ExtrudeExactDirectedCircularLoop(ctx, pieces, zMin, zMax, o.Circular)
	if err != nil {
		return zero, err
	}
	r.NominalArea, r.NominalPerimeter = nominalArea, nominalPerimeter
	r.NominalVolume = nominalArea * (zMax - zMin)
	return r, nil
}

func exactOrthogonalPoint(x, y q) (geom.Vec2, bool) {
	u, exactU := x.Float64()
	v, exactV := y.Float64()
	return geom.Vec2{X: u, Y: v}, exactU && exactV && finite(u) && finite(v)
}

func orthogonalSegmentDistanceSquared(p, a, b geom.Vec2) q {
	q := geom.Vec2{X: max(min(a.X, b.X), min(p.X, max(a.X, b.X))), Y: max(min(a.Y, b.Y), min(p.Y, max(a.Y, b.Y)))}
	return squaredDistance(p, q)
}
