package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/geom"
)

type planarSewChart struct {
	origin, u, v, normal [3]*big.Rat
	offset               *big.Rat
}

func newPlanarSewChart(surface geom.PlaneSurface) (planarSewChart, error) {
	f := surface.Frame()
	if !f.Origin().IsFinite() || !f.XAxis().IsFinite() || !f.YAxis().IsFinite() {
		return planarSewChart{}, geom.ErrInvalid
	}
	c := planarSewChart{origin: exactBoundPoint(f.Origin()), u: exactBoundPoint(f.XAxis()), v: exactBoundPoint(f.YAxis())}
	c.normal = ddCross(c.u, c.v)
	if ratVectorZero(c.normal) {
		return planarSewChart{}, geom.ErrInvalid
	}
	c.offset = ddDot(c.normal, c.origin)
	return c, nil
}

func (c planarSewChart) point(uv geom.Vec2) (p [3]*big.Rat) {
	u, v := ratInset(uv.X), ratInset(uv.Y)
	for i := range p {
		p[i] = new(big.Rat).Mul(c.u[i], u)
		p[i].Add(p[i], new(big.Rat).Mul(c.v[i], v)).Add(p[i], c.origin[i])
	}
	return p
}

func (c planarSewChart) parameters(p [3]*big.Rat) (geom.Vec2, error) {
	if ddDot(c.normal, p).Cmp(c.offset) != 0 {
		return geom.Vec2{}, geom.ErrConvergence
	}
	d := ddSub(p, c.origin)
	uu, uv, vv := ddDot(c.u, c.u), ddDot(c.u, c.v), ddDot(c.v, c.v)
	du, dv := ddDot(d, c.u), ddDot(d, c.v)
	component := func(a, b, c, d *big.Rat) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).Mul(a, b), new(big.Rat).Mul(c, d))
	}
	det := component(uu, vv, uv, uv)
	u, err := geom.RepresentedExactScalar(new(big.Rat).Quo(component(du, vv, dv, uv), det))
	if err != nil {
		return geom.Vec2{}, err
	}
	v, err := geom.RepresentedExactScalar(new(big.Rat).Quo(component(dv, uu, du, uv), det))
	if err != nil {
		return geom.Vec2{}, err
	}
	return geom.Vec2{X: u, Y: v}, nil
}

func planarSewReconcile(ctx context.Context, weld exactVertexWeld, charts []planarSewChart, vertices []planarSewSample, squareTolerance *big.Rat) ([]geom.Vec3, []PlanarSewnVertex, int, *big.Rat, error) {
	groups := make([][]int, len(vertices))
	for id := range vertices {
		root := weld.root(id)
		groups[root] = append(groups[root], id)
	}
	var points []geom.Vec3
	result := make([]PlanarSewnVertex, len(vertices))
	moved, maximum := 0, new(big.Rat)
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, 0, nil, err
		}
		p, err := reconcilePlanarVertex(charts, vertices, group)
		if err != nil {
			return nil, nil, 0, nil, fmt.Errorf("trimmed: planar seam vertex at source slot %d: %w", group[0], err)
		}
		pos, err := geom.RepresentedExactPoint(p)
		if err != nil {
			return nil, nil, 0, nil, err
		}
		for _, id := range group {
			d := ddSub(p, vertices[id].point)
			squared := ddDot(d, d)
			if squared.Cmp(squareTolerance) > 0 {
				return nil, nil, 0, nil, fmt.Errorf("trimmed: planar seam displacement at source slot %d exceeds tolerance: %w", id, geom.ErrConvergence)
			}
			if squared.Sign() > 0 {
				moved++
			}
			if squared.Cmp(maximum) > 0 {
				maximum.Set(squared)
			}
			result[id] = PlanarSewnVertex{Vertex: len(points), SquaredDisplacement: squared}
		}
		points = append(points, pos)
	}
	return points, result, moved, maximum, nil
}

func reconcilePlanarVertex(charts []planarSewChart, vertices []planarSewSample, group []int) ([3]*big.Rat, error) {
	var empty [3]*big.Rat
	center := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	var basis []planarSewChart
	seen := make(map[int]bool, len(group))
	for _, id := range group {
		sample := vertices[id]
		if seen[sample.face] {
			return empty, geom.ErrConvergence
		}
		seen[sample.face] = true
		for axis, x := range sample.point {
			center[axis].Add(center[axis], x)
		}
		plane := charts[sample.face]
		switch len(basis) {
		case 0:
			basis = append(basis, plane)
		case 1:
			if !ratVectorZero(ddCross(basis[0].normal, plane.normal)) {
				basis = append(basis, plane)
			}
		case 2:
			if ddDot(ddCross(basis[0].normal, basis[1].normal), plane.normal).Sign() != 0 {
				basis = append(basis, plane)
			}
		}
	}
	n := big.NewRat(int64(len(group)), 1)
	for _, x := range center {
		x.Quo(x, n)
	}
	// Project the mean onto the affine intersection. Independent normals give
	// a positive definite Gram matrix even when there are only one or two.
	var m [3][4]*big.Rat
	for i, a := range basis {
		for j, b := range basis {
			m[i][j] = ddDot(a.normal, b.normal)
		}
		m[i][len(basis)] = new(big.Rat).Sub(a.offset, ddDot(a.normal, center))
	}
	for i := range basis {
		pivot := new(big.Rat).Set(m[i][i])
		if pivot.Sign() == 0 {
			return empty, geom.ErrConvergence
		}
		for column := i; column <= len(basis); column++ {
			m[i][column].Quo(m[i][column], pivot)
		}
		for row := range basis {
			if row == i {
				continue
			}
			factor := new(big.Rat).Set(m[row][i])
			for column := i; column <= len(basis); column++ {
				m[row][column].Sub(m[row][column], new(big.Rat).Mul(factor, m[i][column]))
			}
		}
	}
	for i, plane := range basis {
		coefficient := m[i][len(basis)]
		for axis, a := range plane.normal {
			center[axis].Add(center[axis], new(big.Rat).Mul(coefficient, a))
		}
	}
	for _, id := range group {
		plane := charts[vertices[id].face]
		if ddDot(plane.normal, center).Cmp(plane.offset) != 0 {
			return empty, fmt.Errorf("incident original planes do not meet: %w", geom.ErrConvergence)
		}
	}
	return center, nil
}

func ratVectorZero(p [3]*big.Rat) bool {
	return p[0].Sign() == 0 && p[1].Sign() == 0 && p[2].Sign() == 0
}
