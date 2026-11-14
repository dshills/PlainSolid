package offset

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// ConvexFootprintMeshOptions bounds the two-sided distance and outward facet
// normal error. NormalTangent is the tangent of the maximum angle in radians.
// Zero NormalTangent defaults to 1/4. Zero MaxRimIntervals defaults to 1024.
type ConvexFootprintMeshOptions struct {
	Deflection, NormalTangent float64
	MaxRimIntervals           int
	Collision                 collision.Options
}

// ConvexFootprintRimInterval identifies a closed piece of one original
// primitive. Parameters use the original rational circle map, or the affine
// segment map for a line. The two consecutive rim vertices are shared by both
// cap fans and the wall between them.
type ConvexFootprintRimInterval struct {
	Primitive, OriginalEdge, OriginalVertex int
	Lower, Upper                            *big.Rat
}

// CertifiedConvexFootprintMesh carries exact upper bounds and original face
// ownership. Face indices follow ExactSolid with bottom and top numbered 0
// and 1, then one wall per primitive. Vertices begin with the two equal-size
// rims in order, followed by their respective cap fan centers.
type CertifiedConvexFootprintMesh struct {
	Mesh                           mesh.Mesh
	OriginalFace                   []int
	Rim                            []ConvexFootprintRimInterval
	Distance, SquaredNormalTangent *big.Rat
	Collision                      collision.Report
}

// TessellateCertified bounds both directions of the error relative to the
// ideal convex rounded prism. It uses exact rational samples of the original
// circular and straight supports, and counts rounding after world placement.
// It proves coherent orientation and checks the display for unintended contact.
// Failure returns no partial mesh.
func (q ConvexFootprintDilation) TessellateCertified(ctx context.Context, o ConvexFootprintMeshOptions) (CertifiedConvexFootprintMesh, error) {
	var empty CertifiedConvexFootprintMesh
	if o.NormalTangent == 0 {
		o.NormalTangent = .25
	}
	if o.MaxRimIntervals == 0 {
		o.MaxRimIntervals = 1024
	}
	if ctx == nil || q.source == nil || !finite(o.Deflection) || o.Deflection <= 0 ||
		!finite(o.NormalTangent) || o.NormalTangent <= 0 || o.NormalTangent > 1 ||
		o.MaxRimIntervals < 1 || o.MaxRimIntervals > 4096 || o.Collision.MaxPairs < 0 || o.Collision.MaxContacts < 0 {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	deflection, angle := num(o.Deflection), num(o.NormalTangent)
	angleSquared := mul(angle, angle)
	source := q.source
	center := pt{new(big.Rat), new(big.Rat)}
	for _, p := range source.points {
		center.x.Add(center.x, p.x)
		center.y.Add(center.y, p.y)
	}
	count := big.NewRat(int64(len(source.points)), 1)
	center.x.Quo(center.x, count)
	center.y.Quo(center.y, count)
	c, err := footprintMeshPointAt(source, center)
	if err != nil {
		return empty, err
	}
	if c.rounding.Cmp(deflection) > 0 || footprintMeshLocalAxis(source.placement, c.world[0], 2) >= footprintMeshLocalAxis(source.placement, c.world[1], 2) {
		return empty, geom.ErrUnrepresentable
	}
	out := CertifiedConvexFootprintMesh{Distance: new(big.Rat).Set(c.rounding), SquaredNormalTangent: new(big.Rat)}
	var rim []footprintMeshPoint
	for i, primitive := range source.primitives {
		a, b := pt{num(primitive.Start.X), num(primitive.Start.Y)}, pt{num(primitive.End.X), num(primitive.End.Y)}
		start, err := footprintMeshPointAt(source, a)
		if err != nil {
			return empty, err
		}
		end, err := footprintMeshPointAt(source, b)
		if err != nil {
			return empty, err
		}
		arc := newFootprintMeshArc(a, b, primitive)
		var split func(lo, hi *big.Rat, start, end footprintMeshPoint, depth int) error
		split = func(lo, hi *big.Rat, start, end footprintMeshPoint, depth int) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if len(rim) == o.MaxRimIntervals {
				return geom.ErrConvergence
			}
			displayChord := footprintDifference(end.local, start.local)
			bound, turn, valid := arc.bounds(start, end, displayChord)
			inside := footprintCross(displayChord, footprintDifference(c.local, start.local)).Sign() > 0
			if valid && inside && bound.Cmp(deflection) <= 0 && turn.Cmp(angleSquared) <= 0 {
				rim = append(rim, start)
				out.Rim = append(out.Rim, ConvexFootprintRimInterval{i, primitive.OriginalEdge, primitive.OriginalVertex, lo, hi})
				if out.Distance.Cmp(bound) < 0 {
					out.Distance.Set(bound)
				}
				if out.SquaredNormalTangent.Cmp(turn) < 0 {
					out.SquaredNormalTangent.Set(turn)
				}
				return nil
			}
			if arc.radius.Sign() == 0 || depth == 32 || displayChord.x.Sign() == 0 && displayChord.y.Sign() == 0 {
				return geom.ErrConvergence
			}
			mid := mul(add(lo, hi), big.NewRat(1, 2))
			p, err := footprintMeshPointAt(source, arc.evaluate(mid))
			if err != nil {
				return err
			}
			if err := split(lo, mid, start, p, depth+1); err != nil {
				return err
			}
			return split(mid, hi, p, end, depth+1)
		}
		if err := split(new(big.Rat), big.NewRat(1, 1), start, end, 0); err != nil {
			return empty, fmt.Errorf("rounded footprint primitive %d: %w", i, err)
		}
	}
	n := len(rim)
	out.Mesh.Vertices = make([]geom.Vec3, 2*n+2)
	for i, p := range rim {
		out.Mesh.Vertices[i], out.Mesh.Vertices[n+i] = p.world[0], p.world[1]
	}
	out.Mesh.Vertices[2*n], out.Mesh.Vertices[2*n+1] = c.world[0], c.world[1]
	for i, s := range out.Rim {
		j := (i + 1) % n
		out.Mesh.Triangles = append(out.Mesh.Triangles, [3]int{2 * n, j, i}, [3]int{2*n + 1, n + i, n + j},
			[3]int{i, j, n + j}, [3]int{i, n + j, n + i})
		out.OriginalFace = append(out.OriginalFace, 0, 1, s.Primitive+2, s.Primitive+2)
	}
	if err := out.Mesh.ValidateSolid(); err != nil {
		return empty, fmt.Errorf("rounded footprint display: %w: %v", geom.ErrUnrepresentable, err)
	}
	out.Collision, err = collision.SelfIntersectionsBVHContext(ctx, out.Mesh, o.Collision)
	if err != nil {
		return empty, err
	}
	if !out.Collision.Complete || len(out.Collision.Contacts) != 0 {
		return empty, geom.ErrConvergence
	}
	return out, nil
}

type footprintMeshPoint struct {
	exact, local pt
	world        [2]geom.Vec3
	rounding     *big.Rat
}

func footprintMeshPointAt(s *convexFootprintSource, p pt) (footprintMeshPoint, error) {
	r := footprintMeshPoint{exact: p, rounding: new(big.Rat)}
	for level, z := range s.z {
		var rounded [3]float64
		sum := new(big.Rat)
		for axis, row := range s.placement {
			exact := num(row[3])
			for col, value := range [3]*big.Rat{p.x, p.y, z} {
				if row[col] > 0 {
					exact.Add(exact, value)
				} else if row[col] < 0 {
					exact.Sub(exact, value)
				}
			}
			f, _ := exact.Float64()
			if !finite(f) {
				return footprintMeshPoint{}, geom.ErrUnrepresentable
			}
			rounded[axis] = f
			delta := sub(exact, num(f))
			sum.Add(sum, delta.Abs(delta))
		}
		r.world[level] = geom.Vec3{X: rounded[0], Y: rounded[1], Z: rounded[2]}
		if r.rounding.Cmp(sum) < 0 {
			r.rounding.Set(sum)
		}
	}
	r.local = pt{num(footprintMeshLocalAxis(s.placement, r.world[0], 0)), num(footprintMeshLocalAxis(s.placement, r.world[0], 1))}
	return r, nil
}

func footprintMeshLocalAxis(pose geom.Affine, p geom.Vec3, axis int) float64 {
	// A proper signed cardinal placement has exactly one nonzero summand.
	return pose[0][axis]*p.X + pose[1][axis]*p.Y + pose[2][axis]*p.Z
}

type footprintMeshArc struct {
	center, start   pt
	radius, tangent *big.Rat
}

func newFootprintMeshArc(a, b pt, primitive ConvexOffsetPrimitive) footprintMeshArc {
	arc := footprintMeshArc{radius: num(primitive.Radius)}
	if arc.radius.Sign() > 0 {
		arc.center = pt{num(primitive.Center.X), num(primitive.Center.Y)}
		arc.start = footprintDifference(a, arc.center)
		end := footprintDifference(b, arc.center)
		dot := footprintDot(arc.start, end)
		arc.tangent = new(big.Rat).Quo(footprintCross(arc.start, end), add(mul(arc.radius, arc.radius), dot))
	}
	return arc
}

func (a footprintMeshArc) evaluate(parameter *big.Rat) pt {
	t := mul(a.tangent, parameter)
	t2 := mul(t, t)
	one := big.NewRat(1, 1)
	w := add(one, t2)
	c, s := new(big.Rat).Quo(sub(one, t2), w), new(big.Rat).Quo(mul(big.NewRat(2, 1), t), w)
	return pt{add(a.center.x, sub(mul(c, a.start.x), mul(s, a.start.y))), add(a.center.y, add(mul(s, a.start.x), mul(c, a.start.y)))}
}

func (a footprintMeshArc) bounds(lo, hi footprintMeshPoint, displayChord pt) (distance, tangent2 *big.Rat, valid bool) {
	chord := footprintDifference(hi.exact, lo.exact)
	bound := lo.rounding
	if bound.Cmp(hi.rounding) < 0 {
		bound = hi.rounding
	}
	distance = new(big.Rat).Set(bound)
	normal := pt{displayChord.y, new(big.Rat).Neg(displayChord.x)}
	sourceNormals := []pt{{chord.y, new(big.Rat).Neg(chord.x)}}
	if a.radius.Sign() > 0 {
		// Orthogonal projection is a bijection onto the chord for a minor
		// arc. Its maximum distance is c²/(4*(r+sqrt(r²-c²/4))). This
		// upper bound uses c²/(4*r), then adds the endpoint rounding.
		distance.Add(distance, new(big.Rat).Quo(footprintDot(chord, chord), mul(big.NewRat(4, 1), a.radius)))
		sourceNormals = []pt{footprintDifference(lo.exact, a.center), footprintDifference(hi.exact, a.center)}
	}
	tangent2 = new(big.Rat)
	for _, target := range sourceNormals {
		dot := footprintDot(normal, target)
		if dot.Sign() <= 0 {
			return distance, tangent2, false
		}
		// The source radial directions sweep monotonically between the
		// endpoints. Positive dot products at both endpoints keep that
		// entire interval in the front-facing hemisphere.
		t := footprintCross(normal, target)
		ratio := new(big.Rat).Quo(mul(t, t), mul(dot, dot))
		if tangent2.Cmp(ratio) < 0 {
			tangent2.Set(ratio)
		}
	}
	return distance, tangent2, true
}
