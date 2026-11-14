package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// ObliqueTorusCavityMeshOptions prescribes a common grid (1..128 intervals per
// quarter). All evaluations are bounded separately by Evaluation; there are at
// most 16*(Subdivisions+1)^2 torus evaluations. Collision has its own cumulative
// work limit. MaxTriangles defaults to 100000; no adaptive retries are hidden.
type ObliqueTorusCavityMeshOptions struct {
	Deflection                 float64
	Subdivisions, MaxTriangles int
	Evaluation                 geom.SurfacePatchEvaluationOptions
	Collision                  collision.Options
}

// CertifiedObliqueTorusCavityMesh preserves native face and torus-edge ancestry.
// TorusEdges indexes each original torus model edge in its stored direction;
// cylinder periodic/pole edges are intentionally absent. BothDirections bounds
// the entire boundary image, not material volume error. Collision certifies
// represented display embedding independently of the exact native source.
type CertifiedObliqueTorusCavityMesh struct {
	Mesh                mesh.Mesh
	OriginalFace        []int
	TorusEdges          map[int][]int
	DistanceUpper       *big.Rat
	SignedDisplayVolume *big.Rat
	Collision           collision.Report
	BothDirections      bool
}

// TessellateObliqueTorusCavityCertified re-proves the native source and builds
// both oriented shells. Shared torus indices require equal exact model points;
// rounded coordinates are never used as a welding criterion.
//
// For quarter parameters angle=2atan(t), circle first/second derivative norms
// are at most 2 and 8. Torus Hessian norms are bounded by 8(R+r), 4r, 8r for uu,uv,vv.
// Taylor interpolation on each grid triangle therefore gives 4(R+3r)/n².
// Cylinder chords, ruled sides and radial cap sectors have bound 2c/n².
// Proper rational rotation preserves norms; the maximum actual vertex rounding
// L1 error is added. Parameter triangles/sectors cover both source and display,
// establishing both directions without a sampled coverage argument.
func TessellateObliqueTorusCavityCertified(ctx context.Context, R, r, c, lower, upper float64, pose [3][4]*big.Rat, tolerance float64, o ObliqueTorusCavityMeshOptions) (out CertifiedObliqueTorusCavityMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedObliqueTorusCavityMesh{}
		}
	}()
	n := o.Subdivisions
	if n < 1 || n > 128 || !finite(o.Deflection) || o.Deflection <= 0 {
		return out, geom.ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1000000 {
		return out, geom.ErrInvalid
	}
	if 32*n*n+16*n > o.MaxTriangles || 32*n*n+16*n > collision.MaxTriangles {
		return out, geom.ErrConvergence
	}
	s, q, e := ObliqueTorusCavity(ctx, R, r, c, lower, upper, pose, tolerance, geom.SurfacePatchOptions{})
	if e != nil {
		return out, e
	}
	out.TorusEdges = map[int][]int{}
	rounding := new(big.Rat)
	appendPoint := func(p [3]*big.Rat) (int, error) {
		v, e := rationalDisplayPoint(p)
		if e != nil {
			return 0, e
		}
		represented := exactBoundPoint(v)
		d := new(big.Rat)
		for k := 0; k < 3; k++ {
			x := new(big.Rat).Sub(p[k], represented[k])
			d.Add(d, x.Abs(x))
		}
		if d.Cmp(rounding) > 0 {
			rounding.Set(d)
		}
		id := len(out.Mesh.Vertices)
		out.Mesh.Vertices = append(out.Mesh.Vertices, v)
		return id, nil
	}
	tri := func(a, b, c, face int) {
		out.Mesh.Triangles = append(out.Mesh.Triangles, [3]int{a, b, c})
		out.OriginalFace = append(out.OriginalFace, face)
	}
	circle := func(i int) [2]*big.Rat {
		quad, t := i/n, big.NewRat(int64(i%n), int64(n))
		t2 := new(big.Rat).Mul(t, t)
		den := new(big.Rat).Add(big.NewRat(1, 1), t2)
		x := new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(1, 1), t2), den)
		y := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(2, 1), t), den)
		for k := 0; k < quad; k++ {
			x, y = new(big.Rat).Neg(y), x
		}
		return [2]*big.Rat{x, y}
	}
	m := 4 * n
	for j := 0; j < 2; j++ {
		z := ratInset(lower)
		if j == 1 {
			z = ratInset(upper)
		}
		for i := 0; i < m; i++ {
			v := circle(i)
			_, e := appendPoint([3]*big.Rat{new(big.Rat).Mul(ratInset(c), v[0]), new(big.Rat).Mul(ratInset(c), v[1]), z})
			if e != nil {
				return out, e
			}
		}
	}
	bottom, e := appendPoint([3]*big.Rat{new(big.Rat), new(big.Rat), ratInset(lower)})
	if e != nil {
		return out, e
	}
	top, e := appendPoint([3]*big.Rat{new(big.Rat), new(big.Rat), ratInset(upper)})
	if e != nil {
		return out, e
	}
	for i := 0; i < m; i++ {
		j := (i + 1) % m
		tri(i, j, m+j, 0)
		tri(i, m+j, m+i, 0)
		tri(bottom, j, i, 1)
		tri(top, m+i, m+j, 2)
	}
	type sample struct {
		id int
		p  [3]*big.Rat
	}
	shared := map[[2]int]sample{}
	for u := 0; u < 4; u++ {
		for v := 0; v < 4; v++ {
			face := 3 + 4*u + v
			surface, e := q.TorusQuadrant(ctx, u, v, geom.SurfacePatchOptions{})
			if e != nil {
				return out, e
			}
			ids := make([]int, (n+1)*(n+1))
			for i := 0; i <= n; i++ {
				for j := 0; j <= n; j++ {
					jet, e := surface.ExactPatch().EvaluateRational(ctx, big.NewRat(int64(i), int64(n)), big.NewRat(int64(j), int64(n)), o.Evaluation)
					if e != nil {
						return out, e
					}
					key := [2]int{(u*n + i) % m, (v*n + j) % m}
					old, ok := shared[key]
					if ok {
						if !sameRationalVertex(old.p, jet.Point) {
							return out, geom.ErrDomain
						}
					} else {
						id, e := appendPoint(jet.Point)
						if e != nil {
							return out, e
						}
						old = sample{id, jet.Point}
						shared[key] = old
					}
					ids[i*(n+1)+j] = old.id
				}
			}
			for i := 0; i < n; i++ {
				for j := 0; j < n; j++ {
					a, b := ids[i*(n+1)+j], ids[(i+1)*(n+1)+j]
					c, d := ids[(i+1)*(n+1)+j+1], ids[i*(n+1)+j+1]
					tri(a, c, b, face)
					tri(a, d, c, face)
				}
			}
			for side, use := range s.faces[face].Outer {
				row := make([]int, n+1)
				for k := 0; k <= n; k++ {
					i, j := k, 0
					switch side {
					case 1:
						i, j = n, k
					case 2:
						i, j = n-k, n
					case 3:
						i, j = 0, n-k
					}
					at := k
					if use.Reversed {
						at = n - k
					}
					row[at] = ids[i*(n+1)+j]
				}
				if prior, ok := out.TorusEdges[use.Edge]; ok {
					for k := range row {
						if row[k] != prior[k] {
							return out, geom.ErrDomain
						}
					}
				} else {
					out.TorusEdges[use.Edge] = row
				}
			}
		}
	}
	bound := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Add(ratInset(R), new(big.Rat).Mul(big.NewRat(3, 1), ratInset(r))))
	cylinder := new(big.Rat).Mul(big.NewRat(2, 1), ratInset(c))
	if cylinder.Cmp(bound) > 0 {
		bound = cylinder
	}
	bound.Quo(bound, big.NewRat(int64(n*n), 1))
	bound.Add(bound, rounding)
	if bound.Cmp(ratInset(o.Deflection)) > 0 {
		return out, geom.ErrConvergence
	}
	out.DistanceUpper = bound
	out.SignedDisplayVolume, out.Collision, err = checkClosedPeriodicMesh(ctx, out.Mesh, o.Collision)
	if err != nil {
		return out, err
	}
	out.BothDirections = true
	return out, nil
}
