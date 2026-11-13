package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// MaxBits bounds exact Taylor coefficients; MaxTrigTerms is cumulative over
// all attempted grids. Other exact arithmetic has finite binary64 inputs and
// fixed polynomial degree. Defaults:512 segments,65536 vertices,262144 triangles,
// 1M predicates,262144 Taylor terms,16384 coefficient bits.
type CertifiedRevolvePolygonOptions struct {
	Deflection                                                                   float64
	MaxSegments, MaxVertices, MaxTriangles, MaxPredicates, MaxTrigTerms, MaxBits int
	Collision                                                                    collision.Options
}

// CertifiedRevolvedPolygonMesh bounds both directed distances between the
// actual stored face images of Solid and the closed display. SquaredErrors
// correspond to display triangles. IdealSweepEmbedded proves the mathematical
// sweep in the represented invertible frame; it does NOT assert exact sewing
// of Solid's independently rounded cap frames. Those cap discrepancies are
// included in the face-image bounds. Display embedding is checked separately.
type CertifiedRevolvedPolygonMesh struct {
	Solid                                           Solid
	Mesh                                            mesh.Mesh
	Profile                                         []geom.Vec2
	OriginalFace                                    []int
	SquaredErrors                                   []*big.Rat
	SourceImageSquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                       collision.Report
	Segments, TrigTerms, Predicates                 int
	IdealSweepEmbedded                              bool
}

// RevolvePolygonCertified constructs an opt-in certified closed display for a
// signed partial sweep with 0<|angle|<=6 radians. Full turns are unsupported.
// Concave meridians and one exact axis side are supported. Exact Taylor bounds
// enclose all trig samples; an analytic Hessian interpolation bound encloses
// each ruled angular cell. Exact cap triangulation and direct affine cap
// vertex bounds account for frame normalization and display rounding. Shared
// axis points are identified only by exact radius zero. No partial certificate
// is returned on resource exhaustion or a nonembedded display.
func RevolvePolygonCertified(ctx context.Context, frame geom.Frame, profile []geom.Vec2, angle float64, o CertifiedRevolvePolygonOptions) (out CertifiedRevolvedPolygonMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedRevolvedPolygonMesh{}
		}
	}()
	if ctx == nil || !finite(angle) || angle == 0 || math.Abs(angle) > 6 || !finite(o.Deflection) || o.Deflection <= 0 {
		return out, geom.ErrInvalid
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 512
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 65536
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 262144
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 1000000
	}
	if o.MaxTrigTerms == 0 {
		o.MaxTrigTerms = 262144
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if o.MaxSegments < 1 || o.MaxSegments > 4096 || o.MaxVertices < 3 || o.MaxVertices > 1000000 || o.MaxTriangles < 1 || o.MaxTriangles > 2000000 || o.MaxPredicates < 1 || o.MaxPredicates > 100000000 || o.MaxTrigTerms < 1 || o.MaxTrigTerms > 2000000 || o.MaxBits < 64 || o.MaxBits > 65536 {
		return out, geom.ErrInvalid
	}
	p, _, e := parametric.PrepareRevolutionMeridian(frame, profile)
	if e != nil {
		return out, e
	}
	source, e := RevolvePolygon(ctx, frame, p, angle)
	if e != nil {
		return out, e
	}
	axes := [3][3]*big.Rat{exactBoundPoint(frame.XAxis()), exactBoundPoint(frame.YAxis()), exactBoundPoint(frame.ZAxis())}
	origin := exactBoundPoint(frame.Origin())
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		det.Add(det, new(big.Rat).Mul(axes[0][i], new(big.Rat).Sub(new(big.Rat).Mul(axes[1][j], axes[2][k]), new(big.Rat).Mul(axes[1][k], axes[2][j]))))
	}
	if det.Sign() <= 0 {
		return out, geom.ErrConvergence
	}
	mnorm := new(big.Rat)
	for i := 0; i < 3; i++ {
		mnorm.Add(mnorm, new(big.Rat).Add(new(big.Rat).Abs(axes[0][i]), new(big.Rat).Abs(axes[1][i])))
	}
	points := make([][2]*big.Rat, len(p))
	ring := make([]int, len(p))
	for i, q := range p {
		points[i] = [2]*big.Rat{ratInset(q.X), ratInset(q.Y)}
		ring[i] = i
	}
	proof := partitionFillProof{ctx: ctx, points: points, limit: o.MaxPredicates}
	if e := proof.simple(ring); e != nil {
		return out, e
	}
	capTriangles, e := proof.triangulate(ring)
	if e != nil {
		return out, e
	}
	// Independently check exact cap area and oriented boundary cancellation.
	chain := map[[2]int]int{}
	area := new(big.Rat)
	for _, tri := range capTriangles {
		area.Add(area, proof.area(tri[:]))
		for i, a := range tri {
			partitionChainAdd(chain, a, tri[(i+1)%3], 1)
		}
	}
	for i, a := range ring {
		partitionChainAdd(chain, a, ring[(i+1)%len(ring)], -1)
	}
	if len(chain) != 0 || area.Cmp(proof.area(ring)) != 0 {
		return out, geom.ErrConvergence
	}
	lo, hi := ratInset(math.Min(0, angle)), ratInset(math.Max(0, angle))
	span := new(big.Rat).Sub(hi, lo)
	tol := ratInset(o.Deflection)
	tol.Mul(tol, tol)
	terms := 0
	for n := 1; n <= o.MaxSegments; n *= 2 {
		if (n+1)*len(p) > o.MaxVertices || 2*n*len(p)+2*len(capTriangles) > o.MaxTriangles {
			return out, geom.ErrConvergence
		}
		du := new(big.Rat).Quo(span, big.NewRat(int64(n), 1))
		cellBounds := make([]*big.Rat, len(p))
		viable := true
		// For F(u,v), ||F_uu||<=R*M, ||F_uv||<=|dr|*M and F_vv=0.
		// Taylor expansion at the common barycentric UV point cancels the
		// weighted first derivatives. |delta u|<=du, |delta v|<=1 gives
		// B=M*(R*du²/2+|dr|*du) in both directions under that correspondence.
		for i, a := range p {
			b := p[(i+1)%len(p)]
			r := ratInset(math.Max(a.X, b.X))
			dr := new(big.Rat).Abs(new(big.Rat).Sub(ratInset(a.X), ratInset(b.X)))
			bound := new(big.Rat).Mul(mnorm, new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Mul(r, new(big.Rat).Mul(du, du)), big.NewRat(2, 1)), new(big.Rat).Mul(dr, du)))
			cellBounds[i] = new(big.Rat).Mul(bound, bound)
			if new(big.Rat).Mul(big.NewRat(4, 1), cellBounds[i]).Cmp(tol) > 0 {
				viable = false
			}
		}
		if !viable {
			continue
		}
		result := CertifiedRevolvedPolygonMesh{Solid: source, Profile: append([]geom.Vec2(nil), p...), Segments: n, Predicates: proof.work, IdealSweepEmbedded: true, SourceImageSquaredDistance: new(big.Rat)}
		ids := make([][]int, n+1)
		errors := make([][]*big.Rat, n+1)
		for k := 0; k <= n; k++ {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			u := new(big.Rat).Add(lo, new(big.Rat).Mul(du, big.NewRat(int64(k), 1)))
			sin, cos, rem, e := revolutionTrigTaylor(ctx, u, o, &terms)
			if e != nil {
				return out, e
			}
			ids[k] = make([]int, len(p))
			errors[k] = make([]*big.Rat, len(p))
			for i, q := range p {
				if q.X == 0 && k > 0 {
					ids[k][i] = ids[0][i]
					errors[k][i] = new(big.Rat).Set(errors[0][i])
					continue
				}
				r, z := ratInset(q.X), ratInset(q.Y)
				var display [3]float64
				squared := new(big.Rat)
				for axis := 0; axis < 3; axis++ {
					center := new(big.Rat).Add(origin[axis], new(big.Rat).Add(new(big.Rat).Mul(axes[2][axis], z), new(big.Rat).Mul(r, new(big.Rat).Add(new(big.Rat).Mul(axes[0][axis], cos), new(big.Rat).Mul(axes[1][axis], sin)))))
					display[axis], _ = center.Float64()
					if !finite(display[axis]) {
						return out, geom.ErrConvergence
					}
					radius := new(big.Rat).Mul(r, new(big.Rat).Mul(rem, new(big.Rat).Add(new(big.Rat).Abs(axes[0][axis]), new(big.Rat).Abs(axes[1][axis]))))
					radius.Add(radius, new(big.Rat).Abs(new(big.Rat).Sub(center, ratInset(display[axis]))))
					squared.Add(squared, new(big.Rat).Mul(radius, radius))
				}
				ids[k][i] = len(result.Mesh.Vertices)
				result.Mesh.Vertices = append(result.Mesh.Vertices, geom.Vec3{X: display[0], Y: display[1], Z: display[2]})
				errors[k][i] = squared
			}
		}
		appendTri := func(tri [3]int, face int, bound *big.Rat) {
			result.Mesh.Triangles = append(result.Mesh.Triangles, tri)
			result.OriginalFace = append(result.OriginalFace, face)
			result.SquaredErrors = append(result.SquaredErrors, new(big.Rat).Set(bound))
			if bound.Cmp(result.SourceImageSquaredDistance) > 0 {
				result.SourceImageSquaredDistance.Set(bound)
			}
		}
		accepted := true
		face := 2
		for i, a := range p {
			j := (i + 1) % len(p)
			if a.X == 0 && p[j].X == 0 {
				continue
			}
			for k := 0; k < n; k++ {
				maxError := new(big.Rat)
				for _, x := range []*big.Rat{errors[k][i], errors[k+1][i], errors[k][j], errors[k+1][j]} {
					if x.Cmp(maxError) > 0 {
						maxError.Set(x)
					}
				}
				bound := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(cellBounds[i], maxError))
				if bound.Cmp(tol) > 0 {
					accepted = false
				}
				quad := [4]int{ids[k][i], ids[k+1][i], ids[k+1][j], ids[k][j]}
				// A collapsed source UV triangle still maps within B of its
				// degenerate display edge, which belongs to the other retained
				// triangle. Thus skipping that zero-area display does not drop
				// source coverage; the cell-wide bound covers both source parts.
				for _, t := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
					tri := [3]int{quad[t[0]], quad[t[1]], quad[t[2]]}
					if tri[0] != tri[1] && tri[1] != tri[2] && tri[2] != tri[0] {
						appendTri(tri, face, bound)
					}
				}
			}
			face++
		}
		for cap := 0; cap < 2; cap++ {
			plane, ok := source.faces[cap].Surface.(geom.PlaneSurface)
			if !ok {
				return out, geom.ErrConvergence
			}
			f := plane.Frame()
			c, x, y := exactBoundPoint(f.Origin()), exactBoundPoint(f.XAxis()), exactBoundPoint(f.YAxis())
			row := cap * n
			for _, t := range capTriangles {
				bound := new(big.Rat)
				tri := [3]int{}
				for k, i := range t {
					tri[k] = ids[row][i]
					v := exactBoundPoint(result.Mesh.Vertices[tri[k]])
					squared := new(big.Rat)
					for axis := 0; axis < 3; axis++ {
						actual := new(big.Rat).Add(c[axis], new(big.Rat).Add(new(big.Rat).Mul(x[axis], points[i][0]), new(big.Rat).Mul(y[axis], points[i][1])))
						d := new(big.Rat).Sub(actual, v[axis])
						squared.Add(squared, new(big.Rat).Mul(d, d))
					}
					if squared.Cmp(bound) > 0 {
						bound.Set(squared)
					}
				}
				if cap == 1 {
					tri[1], tri[2] = tri[2], tri[1]
				}
				if bound.Cmp(tol) > 0 {
					accepted = false
				}
				appendTri(tri, cap, bound)
			}
		}
		if !accepted {
			continue
		}
		result.TrigTerms = terms
		result.SignedDisplayVolume, result.Collision, e = checkClosedPeriodicMesh(ctx, result.Mesh, o.Collision)
		if e != nil {
			return out, e
		}
		return result, nil
	}
	return out, geom.ErrConvergence
}

// Degree64 Taylor polynomials have the common Lagrange remainder |x|^65/65!,
// since all real derivatives of sine and cosine have magnitude at most one.
func revolutionTrigTaylor(ctx context.Context, x *big.Rat, o CertifiedRevolvePolygonOptions, work *int) (sin, cos, remainder *big.Rat, err error) {
	sin, cos = new(big.Rat), big.NewRat(1, 1)
	term := big.NewRat(1, 1)
	check := func(v *big.Rat) bool { return v.Num().BitLen() <= o.MaxBits && v.Denom().BitLen() <= o.MaxBits }
	if !check(x) {
		return nil, nil, nil, geom.ErrConvergence
	}
	for k := 1; k <= 65; k++ {
		if e := ctx.Err(); e != nil {
			return nil, nil, nil, e
		}
		if *work >= o.MaxTrigTerms {
			return nil, nil, nil, geom.ErrConvergence
		}
		*work++
		term.Mul(term, x)
		term.Quo(term, big.NewRat(int64(k), 1))
		if !check(term) {
			return nil, nil, nil, geom.ErrConvergence
		}
		if k == 65 {
			remainder = new(big.Rat).Abs(term)
			break
		}
		target := cos
		sign := (k / 2) % 2
		if k%2 == 1 {
			target = sin
			sign = ((k - 1) / 2) % 2
		}
		if sign == 0 {
			target.Add(target, term)
		} else {
			target.Sub(target, term)
		}
		if !check(target) {
			return nil, nil, nil, geom.ErrConvergence
		}
	}
	return sin, cos, remainder, nil
}
