package iges

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
	"strconv"
)

// ReadPartitionedRationalSplineLensSolid accepts canonical conjugate-axis ellipse seams
// partitioned at the exact union of both source charts' rational knot cuts.
func ReadPartitionedRationalSplineLensSolid(r io.Reader, l Limits) (PartitionedRationalSplineLens, error) {
	return readPartitionedLens(r, l, false)
}
func readPartitionedLens(r io.Reader, l Limits, affine bool) (PartitionedRationalSplineLens, error) {
	d, e := readTopologySource(r, l, true)
	if e != nil {
		return PartitionedRationalSplineLens{}, e
	}
	m, e := readIGESLens(d, l, true, true, affine)
	if e != nil {
		return PartitionedRationalSplineLens{}, e
	}
	out := *m.partitioned
	out.Body = m
	return out, nil
}
func rawLensBezier(x *geom.ExactSplineCurve) (RationalLensCurve, error) {
	net, _, e := x.BezierControlNet(context.Background(), curvedOvalOptions())
	if e != nil {
		return RationalLensCurve{}, e
	}
	if net.Start != 0 || net.End != 1 || len(net.Control) < 2 || len(net.Control) > 17 {
		return RationalLensCurve{}, geom.ErrInvalid
	}
	c := RationalLensCurve{Degree: len(net.Control) - 1}
	lcm := big.NewInt(1)
	for _, h := range net.Control {
		var p [3]float64
		for i := range p {
			p[i], e = geom.RepresentedExactScalar(new(big.Rat).Quo(h[i], h[3]))
			if e != nil {
				return c, e
			}
		}
		c.Poles = append(c.Poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
		den := h[3].Denom()
		g := new(big.Int).GCD(nil, nil, lcm, den)
		lcm.Mul(new(big.Int).Quo(lcm, g), den)
		if lcm.BitLen() > 8192 {
			return c, geom.ErrConvergence
		}
	}
	ints := make([]*big.Int, len(net.Control))
	gcd := new(big.Int)
	for i, h := range net.Control {
		v := new(big.Rat).Mul(h[3], new(big.Rat).SetInt(lcm))
		if !v.IsInt() {
			return c, geom.ErrUnrepresentable
		}
		ints[i] = new(big.Int).Set(v.Num())
		gcd.GCD(nil, nil, gcd, ints[i])
	}
	for _, v := range ints {
		w, e := geom.RepresentedExactScalar(new(big.Rat).SetInt(new(big.Int).Quo(v, gcd)))
		if e != nil {
			return c, e
		}
		c.Weights = append(c.Weights, w)
	}
	for i := 0; i <= c.Degree; i++ {
		c.Knots = append(c.Knots, 0)
	}
	for i := 0; i <= c.Degree; i++ {
		c.Knots = append(c.Knots, 1)
	}
	return c, nil
}

// NewPartitionedSplineLensSolid constructs lossless raw quarter-piece curves.
// Irrational knot intersections or unrepresentable poles/weights fail explicitly.
func NewPartitionedSplineLensSolid(m SplineLensSolid) (PartitionedRationalSplineLens, error) {
	return newPartitionedLens(m, false)
}
func newPartitionedLens(m SplineLensSolid, affine bool) (PartitionedRationalSplineLens, error) {
	if !affine && m.Tilt != 0 {
		return PartitionedRationalSplineLens{}, geom.ErrInvalid
	}
	if _, e := certifyIGESLens(m); e != nil {
		return PartitionedRationalSplineLens{}, e
	}
	plan, e := igesLensPlan(m)
	if e != nil {
		return PartitionedRationalSplineLens{}, e
	}
	out := PartitionedRationalSplineLens{Body: m}
	for _, p := range plan {
		uv, e := rawLensBezier(p.UV)
		if e != nil {
			return out, e
		}
		model, e := rawLensBezier(p.Model)
		if e != nil {
			return out, e
		}
		out.Curves = append(out.Curves, [3]RationalLensCurve{model, uv, uv})
		out.Pieces = append(out.Pieces, p.LensSeamPiece)
	}
	var b bytes.Buffer
	if e = writePartitionedLens(&b, out, affine); e != nil {
		return PartitionedRationalSplineLens{}, e
	}
	return readPartitionedLens(&b, Limits{}, affine)
}
func WritePartitionedRationalSplineLensSolid(w io.Writer, m PartitionedRationalSplineLens) error {
	return writePartitionedLens(w, m, false)
}
func writePartitionedLens(w io.Writer, m PartitionedRationalSplineLens, affine bool) error {
	if !affine && m.Body.Tilt != 0 {
		return geom.ErrInvalid
	}
	if w == nil {
		return geom.ErrInvalid
	}
	if _, e := certifyIGESLens(m.Body); e != nil {
		return e
	}
	plan, e := igesLensPlan(m.Body)
	if e != nil {
		return e
	}
	n := len(plan)
	if len(m.Curves) != n || len(m.Pieces) != n {
		return geom.ErrInvalid
	}
	for i, p := range m.Pieces {
		q := plan[i]
		if p.Start == nil || p.End == nil || p.Quarter != q.Quarter || p.Start.Cmp(q.Start) != 0 || p.End.Cmp(q.End) != 0 || p.Cells != q.Cells {
			return geom.ErrInvalid
		}
	}
	a := topologyAuthor{}
	surfaces := [2]int{}
	for i, s := range []*geom.BSplineSurface{m.Body.Upper, m.Body.Lower} {
		surfaces[i] = a.entity(Entity{Surface: &Surface{Geometry: s, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1}})
	}
	models := make([]int, n)
	owners := [2][]int{make([]int, n), make([]int, n)}
	var points []geom.Vec3
	for i, row := range m.Curves {
		for owner, c := range row {
			data, e := encodeRationalIGESCurve(c, owner > 0)
			if e != nil {
				return e
			}
			id := a.add(126, 0, data)
			if owner == 0 {
				models[i] = id
				x, e := rationalIGESCurve(c)
				if e != nil {
					return e
				}
				p, e := x.ExactPoint(0)
				if e != nil {
					return e
				}
				points = append(points, p)
			} else {
				owners[owner-1][i] = id
			}
		}
	}
	values := []string{"502", strconv.Itoa(n)}
	for _, p := range points {
		values = append(values, number(p.X), number(p.Y), number(p.Z))
	}
	vid := a.add(502, 1, joinTopologyValues(values))
	edges := []int{504, n}
	for i, id := range models {
		edges = append(edges, id, vid, i+1, vid, (i+1)%n+1)
	}
	eid := a.add(504, 1, topologyNumbers(edges...))
	var faces [2]int
	for f := 0; f < 2; f++ {
		loop := []int{508, n}
		for i, id := range owners[f] {
			loop = append(loop, 0, eid, i+1, 1, 1, 0, id)
		}
		lid := a.add(508, 1, topologyNumbers(loop...))
		faces[f] = a.add(510, 1, topologyNumbers(510, surfaces[f], 1, 1, lid))
	}
	shell := a.add(514, 1, topologyNumbers(514, 2, faces[0], 1, faces[1], 0))
	a.add(186, 0, topologyNumbers(186, shell, 1, 0))
	data, e := a.file(m.Body.LengthUnit)
	if e != nil {
		return e
	}
	checked, e := readPartitionedLens(bytes.NewReader(data), Limits{}, affine)
	if e != nil {
		return e
	}
	if checked.Body.Center != m.Body.Center || checked.Body.Radii != m.Body.Radii || checked.Body.Shear != m.Body.Shear || checked.Body.Tilt != m.Body.Tilt {
		return geom.ErrInvalid
	}
	written, e := w.Write(data)
	if e == nil && written != len(data) {
		return io.ErrShortWrite
	}
	return e
}
