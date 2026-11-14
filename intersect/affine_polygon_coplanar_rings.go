package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AffineRingOperation selects the closed planar material boundary; no volume
// or sewing is asserted. Cross-owner boundary contact is unsupported here.
type AffineRingOperation uint8

const (
	AffineRingIntersection AffineRingOperation = iota
	AffineRingUnion
	AffineRingDifference
)

// AffineSelectedRing retains its authored vertex order and original UV. Direction
// is +1 or -1 relative to that order, giving material on the left in owner A's
// chart. Reversal does not renumber original edges.
type AffineSelectedRing struct {
	Owner, Ring, Direction int
	UV                     [][2]*big.Rat
	Model                  [][3]*big.Rat
}
type AffineCoplanarRings struct {
	source *AffinePatchContact
	rings  []AffineSelectedRing
	work   int
}

func (s *AffineCoplanarRings) Complete() bool { return s != nil && s.source != nil }
func (s *AffineCoplanarRings) OriginalContact() *AffinePatchContact {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *AffineCoplanarRings) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}
func (s *AffineCoplanarRings) Rings() []AffineSelectedRing {
	if s == nil {
		return nil
	}
	out := make([]AffineSelectedRing, len(s.rings))
	for i, r := range s.rings {
		out[i] = r
		out[i].UV = make([][2]*big.Rat, len(r.UV))
		out[i].Model = make([][3]*big.Rat, len(r.Model))
		for j := range r.UV {
			for k := 0; k < 2; k++ {
				out[i].UV[j][k] = new(big.Rat).Set(r.UV[j][k])
			}
			for k := 0; k < 3; k++ {
				out[i].Model[j][k] = new(big.Rat).Set(r.Model[j][k])
			}
		}
	}
	return out
}

// SelectNoncontactingAffinePolygonRings proves complete coplanar boundary cycles
// for simple polygon trims with holes when NO cross-owner boundaries meet.
// Each whole ring then lies in one component of the other trim's complement;
// its exact membership determines inclusion and orientation. Crossing, touching
// or coincident boundaries fail atomically instead of assuming an arrangement.
func SelectNoncontactingAffinePolygonRings(ctx context.Context, a, c Patch, trims [2]AffinePolygonTrim, operation AffineRingOperation, p geom.SurfacePatchOptions, o AffinePolygonContactOptions) (*AffineCoplanarRings, error) {
	if ctx == nil || operation > AffineRingDifference {
		return nil, geom.ErrInvalid
	}
	if o.MaxEdges == 0 {
		o.MaxEdges = 64
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxEdges < 3 || o.MaxEdges > 256 || o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return nil, geom.ErrInvalid
	}
	b := &affineTrimBudget{ctx: ctx, max: o.MaxOperations, bits: o.MaxCoefficientBits}
	rings, e := validateAffinePolygonTrims(ctx, [2]Patch{a, c}, trims, o, b)
	if e != nil {
		return nil, e
	}
	if b.work >= b.max {
		return nil, geom.ErrConvergence
	}
	source, e := IntersectAffinePatches(ctx, a, c, p, geom.AffineChartIntersectionOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: b.bits})
	if e != nil {
		return nil, e
	}
	b.work += source.work
	if !source.coplanar {
		return nil, geom.ErrDomain
	}
	if b.work >= b.max {
		return nil, geom.ErrConvergence
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, source.exact[0], geom.AffineSurfaceChartOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: b.bits, MaxCells: 4096})
	if e != nil {
		return nil, e
	}
	b.work += chart.ExactOperations()
	projected := [2][][][2]*big.Rat{}
	models := [2][][][3]*big.Rat{}
	for owner, loops := range rings {
		for _, ring := range loops {
			uvs := [][2]*big.Rat{}
			xyz := [][3]*big.Rat{}
			for _, uv := range ring {
				if b.work >= b.max {
					return nil, geom.ErrConvergence
				}
				jet, e := source.exact[owner].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: b.bits})
				b.work += jet.ExactOperations
				if e != nil {
					return nil, e
				}
				xyz = append(xyz, jet.Point)
				if owner == 0 {
					uvs = append(uvs, uv)
				} else {
					if b.work >= b.max {
						return nil, geom.ErrConvergence
					}
					mapped, e := chart.InvertExactContext(ctx, jet.Point, geom.AffineChartInverseOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: b.bits})
					b.work += mapped.ExactOperations
					if e != nil {
						return nil, e
					}
					if !mapped.OnPlane {
						return nil, geom.ErrDomain
					}
					uvs = append(uvs, [2]*big.Rat{mapped.U, mapped.V})
				}
			}
			projected[owner] = append(projected[owner], uvs)
			models[owner] = append(models[owner], xyz)
		}
	}
	// Exact orientation tests plus closed bounding boxes include endpoints and
	// collinear overlap. A true absence proof is required for EVERY edge pair.
	for _, ra := range projected[0] {
		for i, x := range ra {
			y := ra[(i+1)%len(ra)]
			for _, rb := range projected[1] {
				for j, u := range rb {
					v := rb[(j+1)%len(rb)]
					s0, s1 := b.side(x, y, u).Sign(), b.side(x, y, v).Sign()
					t0, t1 := b.side(u, v, x).Sign(), b.side(u, v, y).Sign()
					if b.err != nil {
						return nil, b.err
					}
					if s0*s1 > 0 || t0*t1 > 0 {
						continue
					}
					apart := false
					for axis := 0; axis < 2; axis++ {
						lo, hi := x[axis], y[axis]
						if lo.Cmp(hi) > 0 {
							lo, hi = hi, lo
						}
						clo, chi := u[axis], v[axis]
						if clo.Cmp(chi) > 0 {
							clo, chi = chi, clo
						}
						if hi.Cmp(clo) < 0 || chi.Cmp(lo) < 0 {
							apart = true
						}
					}
					if !apart {
						return nil, ErrUnresolved
					}
				}
			}
		}
	}
	inside := func(q [2]*big.Rat, loops [][][2]*big.Rat) bool {
		wind := 0
		for _, ring := range loops {
			for i, v := range ring {
				w := ring[(i+1)%len(ring)]
				side := b.side(v, w, q).Sign()
				if v[1].Cmp(q[1]) <= 0 && w[1].Cmp(q[1]) > 0 && side > 0 {
					wind++
				}
				if v[1].Cmp(q[1]) > 0 && w[1].Cmp(q[1]) <= 0 && side < 0 {
					wind--
				}
			}
		}
		return wind != 0
	}
	out := &AffineCoplanarRings{source: source}
	for owner, loops := range projected {
		for ri, ring := range loops {
			in := inside(ring[0], projected[1-owner])
			keep := in
			direction := 1
			if owner == 1 {
				direction = source.orientation
			}
			switch operation {
			case AffineRingUnion:
				keep = !in
			case AffineRingDifference:
				if owner == 0 {
					keep = !in
				} else {
					direction = -direction
				}
			}
			if keep {
				out.rings = append(out.rings, AffineSelectedRing{Owner: owner, Ring: ri, Direction: direction, UV: rings[owner][ri], Model: models[owner][ri]})
			}
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	// Detach every retained authored coordinate before publication.
	out.rings = out.Rings()
	out.work = b.work
	return out, nil
}
