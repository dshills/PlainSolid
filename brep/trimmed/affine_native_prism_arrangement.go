package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AffineNativePrismOptions bounds candidate search and cumulative exact affine
// arithmetic. Native options bound each complete cardinal proof attempt. This
// first domain requires lossless represented canonical and final native vertices.
type AffineNativePrismOptions struct {
	Native                                                    NativePrismBooleanOptions
	MaxCandidates, MaxTransformOperations, MaxCoefficientBits int
}
type AffineNativePrismBoolean struct {
	canonical        *NativePrismBoolean
	owners           [2]brep.Solid
	solids           []brep.Solid
	inverse          [3][4]*big.Rat
	volume           *big.Rat
	candidates, work int
}

func (s *AffineNativePrismBoolean) Components() []brep.Solid {
	if s == nil {
		return nil
	}
	return append([]brep.Solid(nil), s.solids...)
}
func (s *AffineNativePrismBoolean) ExactVolume() *big.Rat {
	if s == nil {
		return nil
	}
	return new(big.Rat).Set(s.volume)
}
func (s *AffineNativePrismBoolean) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil || i < 0 || i > 1 {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}
func (s *AffineNativePrismBoolean) OriginalCaps() [2][2]int {
	if s == nil {
		return [2][2]int{{-1, -1}, {-1, -1}}
	}
	return s.canonical.OriginalCaps()
}
func (s *AffineNativePrismBoolean) WallUses() []NativePrismWallUse {
	if s == nil {
		return nil
	}
	return s.canonical.WallUses()
}
func (s *AffineNativePrismBoolean) CanonicalToOriginal() [3][4]*big.Rat {
	var out [3][4]*big.Rat
	if s == nil {
		return out
	}
	for i := range out {
		for j := range out[i] {
			out[i][j] = new(big.Rat).Set(s.inverse[i][j])
		}
	}
	return out
}
func (s *AffineNativePrismBoolean) ProofWork() (candidates, transformOperations int) {
	if s == nil {
		return 0, 0
	}
	return s.candidates, s.work
}

type nativePrismAffineBudget struct {
	ctx             context.Context
	work, max, bits int
	err             error
}

func (b *nativePrismAffineBudget) check(q *big.Rat) {
	if b.err != nil {
		return
	}
	if e := b.ctx.Err(); e != nil {
		b.err = e
		return
	}
	b.work++
	if q == nil {
		b.err = geom.ErrInvalid
	} else if b.work > b.max || q.Num().BitLen() > b.bits || q.Denom().BitLen() > b.bits {
		b.err = geom.ErrConvergence
	}
}
func (b *nativePrismAffineBudget) op(op byte, x, y *big.Rat) *big.Rat {
	q := new(big.Rat)
	if b.err != nil {
		return q
	}
	switch op {
	case '+':
		q.Add(x, y)
	case '-':
		q.Sub(x, y)
	case '*':
		q.Mul(x, y)
	case '/':
		if y.Sign() == 0 {
			b.err = geom.ErrInvalid
			return q
		}
		q.Quo(x, y)
	}
	b.check(q)
	return q
}
func (b *nativePrismAffineBudget) mapSolid(s brep.Solid, m [3][4]*big.Rat) (brep.Solid, error) {
	vertices := s.Vertices()
	for i, v := range vertices {
		p := [3]*big.Rat{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
		xyz := [3]float64{}
		for axis := 0; axis < 3; axis++ {
			q := new(big.Rat).Set(m[axis][3])
			for j := 0; j < 3; j++ {
				q = b.op('+', q, b.op('*', m[axis][j], p[j]))
			}
			if b.err != nil {
				return brep.Solid{}, b.err
			}
			x, exact := q.Float64()
			if !exact || math.IsInf(x, 0) || math.IsNaN(x) {
				return brep.Solid{}, geom.ErrDomain
			}
			xyz[axis] = x
		}
		vertices[i] = geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	}
	return brep.NewSolid(vertices, s.Edges(), s.Faces())
}

// CombineAffineNativePrisms infers cap/direction candidates from original native
// face anchors and incident edges. An exact orientation-preserving affine map
// turns each candidate into coordinate-axis extrusion, where the complete
// original cap/loop/wall authenticator is rerun. Thus a proposed direction is
// never itself proof. Shared cap planes and translation must match for both
// operands. Deterministic bounded search handles multiple valid prism directions.
// No generic dispatcher or cardinal API is changed; unrepresentable canonical
// vertices, unresolved topology and exhausted search remain explicit errors.
func CombineAffineNativePrisms(ctx context.Context, a, c brep.Solid, operation intersect.AffineRingOperation, o AffineNativePrismOptions) (*AffineNativePrismBoolean, error) {
	if ctx == nil || operation > intersect.AffineRingDifference {
		return nil, geom.ErrInvalid
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 16
	}
	if o.MaxTransformOperations == 0 {
		o.MaxTransformOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCandidates < 1 || o.MaxCandidates > 64 || o.MaxTransformOperations < 1 || o.MaxTransformOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return nil, geom.ErrInvalid
	}
	b := &nativePrismAffineBudget{ctx: ctx, max: o.MaxTransformOperations, bits: o.MaxCoefficientBits}
	limit := o.Native.Native.MaxVertices
	if limit == 0 {
		limit = 4096
	}
	if limit < 6 || limit > 16384 {
		return nil, geom.ErrInvalid
	}
	av, cv := a.Vertices(), c.Vertices()
	if len(av) < 6 || len(cv) < 6 || len(av) > limit || len(cv) > limit {
		return nil, geom.ErrDomain
	}
	edges, faces := a.Edges(), a.Faces()
	if len(edges) > 3*limit || len(faces) > limit || len(c.Edges()) > 3*limit || len(c.Faces()) > limit {
		return nil, geom.ErrDomain
	}
	exact := make([][3]*big.Rat, len(av))
	for i, v := range av {
		exact[i] = [3]*big.Rat{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
		for _, q := range exact[i] {
			b.check(q)
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	attempts := 0
	seen := map[string]bool{}
	lastError := error(geom.ErrDomain)
	for _, face := range faces {
		ids, e := nativePrismLoopIndices(face.Boundary, edges)
		if e != nil {
			return nil, e
		}
		anchor := ids[0]
		origin := exact[anchor]
		normal := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		for i := 1; i+1 < len(ids); i++ {
			u, v := [3]*big.Rat{}, [3]*big.Rat{}
			for j := 0; j < 3; j++ {
				u[j] = b.op('-', exact[ids[i]][j], origin[j])
				v[j] = b.op('-', exact[ids[i+1]][j], origin[j])
			}
			for j := 0; j < 3; j++ {
				k, l := (j+1)%3, (j+2)%3
				normal[j] = b.op('-', b.op('*', u[k], v[l]), b.op('*', u[l], v[k]))
			}
			if normal[0].Sign() != 0 || normal[1].Sign() != 0 || normal[2].Sign() != 0 {
				break
			}
		}
		if b.err != nil {
			return nil, b.err
		}
		r := -1
		for j := 0; j < 3; j++ {
			if normal[j].Sign() != 0 {
				r = j
				break
			}
		}
		if r < 0 {
			continue
		}
		onFace := map[int]bool{}
		for _, loop := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
			ring, e := nativePrismLoopIndices(loop, edges)
			if e != nil {
				return nil, e
			}
			for _, id := range ring {
				onFace[id] = true
			}
		}
		for _, edge := range edges {
			other := -1
			if edge.Start == anchor {
				other = edge.End
			} else if edge.End == anchor {
				other = edge.Start
			}
			if other < 0 || onFace[other] {
				continue
			}
			d := [3]*big.Rat{}
			h := new(big.Rat)
			for j := 0; j < 3; j++ {
				d[j] = b.op('-', exact[other][j], origin[j])
				h = b.op('+', h, b.op('*', normal[j], d[j]))
			}
			if b.err != nil {
				return nil, b.err
			}
			if h.Sign() == 0 {
				continue
			}
			j, k := (r+1)%3, (r+2)%3
			forward, inverse := [3][4]*big.Rat{}, [3][4]*big.Rat{}
			for row := 0; row < 3; row++ {
				for col := 0; col < 4; col++ {
					forward[row][col] = new(big.Rat)
					inverse[row][col] = new(big.Rat)
				}
			}
			determinant := b.op('/', h, normal[r])
			flip := big.NewRat(1, 1)
			if determinant.Sign() < 0 {
				flip.SetInt64(-1)
			}
			for axis := 0; axis < 3; axis++ {
				forward[2][axis] = b.op('/', normal[axis], h)
				forward[0][axis] = b.op('-', new(big.Rat), b.op('*', d[j], forward[2][axis]))
				forward[1][axis] = b.op('-', new(big.Rat), b.op('*', d[k], forward[2][axis]))
			}
			forward[0][j] = b.op('+', forward[0][j], big.NewRat(1, 1))
			forward[1][k] = b.op('+', forward[1][k], big.NewRat(1, 1))
			for axis := 0; axis < 3; axis++ {
				forward[0][axis] = b.op('*', forward[0][axis], flip)
			}
			for row := 0; row < 3; row++ {
				for axis := 0; axis < 3; axis++ {
					forward[row][3] = b.op('-', forward[row][3], b.op('*', forward[row][axis], origin[axis]))
				}
			}
			inverse[j][0] = new(big.Rat).Set(flip)
			inverse[k][1] = big.NewRat(1, 1)
			inverse[r][0] = b.op('*', flip, b.op('/', b.op('-', new(big.Rat), normal[j]), normal[r]))
			inverse[r][1] = b.op('/', b.op('-', new(big.Rat), normal[k]), normal[r])
			for axis := 0; axis < 3; axis++ {
				inverse[axis][2] = new(big.Rat).Set(d[axis])
				inverse[axis][3] = new(big.Rat).Set(origin[axis])
			}
			if b.err != nil {
				return nil, b.err
			}
			// Reprove the complete rational affine inverse, including translation.
			for row := 0; row < 3; row++ {
				for col := 0; col < 4; col++ {
					value := new(big.Rat)
					if col == 3 {
						value.Set(forward[row][3])
					}
					for axis := 0; axis < 3; axis++ {
						value = b.op('+', value, b.op('*', forward[row][axis], inverse[axis][col]))
					}
					want := new(big.Rat)
					if row == col {
						want.SetInt64(1)
					}
					if b.err != nil {
						return nil, b.err
					}
					if value.Cmp(want) != 0 {
						return nil, geom.ErrInvalid
					}
				}
			}
			signature := ""
			for _, row := range forward {
				for _, q := range row {
					signature += q.RatString() + ","
				}
			}
			if seen[signature] {
				continue
			}
			seen[signature] = true
			if attempts >= o.MaxCandidates {
				return nil, geom.ErrConvergence
			}
			attempts++
			ca, e := b.mapSolid(a, forward)
			if e != nil {
				if b.err != nil {
					return nil, b.err
				}
				lastError = e
				continue
			}
			cc, e := b.mapSolid(c, forward)
			if e != nil {
				if b.err != nil {
					return nil, b.err
				}
				lastError = e
				continue
			}
			proof, e := CombineMatchingNativePrisms(ctx, ca, cc, operation, o.Native)
			if e != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				lastError = e
				continue
			}
			out := &AffineNativePrismBoolean{canonical: proof, owners: [2]brep.Solid{a, c}, inverse: inverse, candidates: attempts}
			good := true
			for _, solid := range proof.Material().Components() {
				world, e := b.mapSolid(solid, inverse)
				if e != nil {
					if b.err != nil {
						return nil, b.err
					}
					lastError = e
					good = false
					break
				}
				out.solids = append(out.solids, world)
			}
			if !good {
				continue
			}
			out.volume = b.op('*', proof.Material().ExactVolume(), new(big.Rat).Abs(determinant))
			if b.err != nil {
				return nil, b.err
			}
			out.work = b.work
			return out, nil
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	return nil, lastError
}
