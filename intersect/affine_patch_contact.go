package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AffinePatchContactKind is the dimension of the complete closed-domain contact.
type AffinePatchContactKind uint8

const (
	AffinePatchContactEmpty AffinePatchContactKind = iota
	AffinePatchContactPoint
	AffinePatchContactSegment
	AffinePatchContactArea
)

// AffinePatchContactEdge indexes exact vertices. Boundary contains the original
// UMin/UMax/VMin/VMax masks (1,2,4,8) owning the entire edge, not just an endpoint.
// A transverse section edge can be interior to both owners (both masks zero).
type AffinePatchContactEdge struct {
	Start, End int
	Boundary   [2]uint8
}

// AffinePatchContact is an immutable complete intersection of actual original
// affine patches. Area contacts have one convex cycle, positive in owner A UV;
// point/segment contacts are not regularized material or a closed shell.
type AffinePatchContact struct {
	owners            [2]Patch
	exact             [2]*geom.SurfacePatch
	kind              AffinePatchContactKind
	coplanar          bool
	orientation       int
	vertices          []geom.AffineChartIntersectionEndpoint
	edges             []AffinePatchContactEdge
	area, squaredArea *big.Rat
	work              int
}

func (s *AffinePatchContact) Complete() bool {
	return s != nil && s.exact[0] != nil && s.exact[1] != nil
}
func (s *AffinePatchContact) Kind() (AffinePatchContactKind, error) {
	if !s.Complete() {
		return 0, geom.ErrInvalid
	}
	return s.kind, nil
}
func (s *AffinePatchContact) Owner(i int) (Patch, error) {
	if !s.Complete() || i < 0 || i > 1 {
		return Patch{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}
func (s *AffinePatchContact) ExactOperations() int {
	if !s.Complete() {
		return 0
	}
	return s.work
}
func (s *AffinePatchContact) Vertices() ([]geom.AffineChartIntersectionEndpoint, error) {
	if !s.Complete() {
		return nil, geom.ErrInvalid
	}
	out := make([]geom.AffineChartIntersectionEndpoint, len(s.vertices))
	for i, v := range s.vertices {
		out[i] = v
		for j := 0; j < 3; j++ {
			out[i].Point[j] = new(big.Rat).Set(v.Point[j])
		}
		for j := 0; j < 2; j++ {
			out[i].A[j] = new(big.Rat).Set(v.A[j])
			out[i].B[j] = new(big.Rat).Set(v.B[j])
		}
	}
	return out, nil
}
func (s *AffinePatchContact) Edges() ([]AffinePatchContactEdge, error) {
	if !s.Complete() {
		return nil, geom.ErrInvalid
	}
	return append([]AffinePatchContactEdge(nil), s.edges...), nil
}

// CoplanarArea returns original A-UV area and squared physical area, including
// zero for coplanar lower-dimensional contacts. Orientation compares B to A.
// A noncoplanar contact has no shared oriented area chart and returns ErrDomain.
func (s *AffinePatchContact) CoplanarArea() (areaA, squaredPhysical *big.Rat, orientation int, err error) {
	if !s.Complete() {
		return nil, nil, 0, geom.ErrInvalid
	}
	if !s.coplanar {
		return nil, nil, 0, geom.ErrDomain
	}
	return new(big.Rat).Set(s.area), new(big.Rat).Set(s.squaredArea), s.orientation, nil
}

// IntersectAffinePatches proves each actual source is globally affine, then
// computes its entire finite contact. Exact support recovery uses separately
// bounded p stages; chart identities, clipping and original vertex evaluations
// share o's cumulative arithmetic budget. No supplied certificate is trusted.
// Existing Section/Trace wire semantics are unchanged by this additive API.
func IntersectAffinePatches(ctx context.Context, a, b Patch, p geom.SurfacePatchOptions, o geom.AffineChartIntersectionOptions) (*AffinePatchContact, error) {
	if ctx == nil || !a.valid() || !b.valid() {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return nil, geom.ErrInvalid
	}
	s := &AffinePatchContact{owners: [2]Patch{a, b}}
	charts := [2]*geom.AffineSurfaceChart{}
	for i, owner := range s.owners {
		var e error
		s.exact[i], e = exactSplinePatch(owner, p)
		if e != nil {
			return nil, e
		}
		if s.work >= o.MaxOperations {
			return nil, geom.ErrConvergence
		}
		charts[i], e = geom.NewAffineSurfaceChart(ctx, s.exact[i], geom.AffineSurfaceChartOptions{MaxOperations: o.MaxOperations - s.work, MaxCoefficientBits: o.MaxCoefficientBits, MaxCells: 4096})
		if e != nil {
			return nil, e
		}
		s.work += charts[i].ExactOperations()
	}
	if s.work >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	q, e := geom.IntersectAffineSurfaceCharts(ctx, charts[0], charts[1], geom.AffineChartIntersectionOptions{MaxOperations: o.MaxOperations - s.work, MaxCoefficientBits: o.MaxCoefficientBits})
	s.work += q.ExactOperations
	if e != nil {
		return nil, e
	}
	if !q.Complete {
		return nil, ErrUnresolved
	}
	switch q.Kind {
	case geom.AffineChartsDisjoint:
		s.kind = AffinePatchContactEmpty
	case geom.AffineChartsPoint:
		s.kind = AffinePatchContactPoint
		s.vertices = append(s.vertices, q.Endpoints[0])
	case geom.AffineChartsSegment:
		s.kind = AffinePatchContactSegment
		s.vertices = append(s.vertices, q.Endpoints[:]...)
	case geom.AffineChartsCoplanar:
		if s.work >= o.MaxOperations {
			return nil, geom.ErrConvergence
		}
		clip, e := geom.IntersectCoplanarAffineSurfaceCharts(ctx, charts[0], charts[1], geom.AffineChartIntersectionOptions{MaxOperations: o.MaxOperations - s.work, MaxCoefficientBits: o.MaxCoefficientBits})
		s.work += clip.ExactOperations
		if e != nil {
			return nil, e
		}
		if !clip.Complete {
			return nil, ErrUnresolved
		}
		switch clip.Kind {
		case geom.CoplanarChartsEmpty:
			s.kind = AffinePatchContactEmpty
		case geom.CoplanarChartsPoint:
			s.kind = AffinePatchContactPoint
		case geom.CoplanarChartsSegment:
			s.kind = AffinePatchContactSegment
		case geom.CoplanarChartsArea:
			s.kind = AffinePatchContactArea
		default:
			return nil, geom.ErrInvalid
		}
		s.coplanar = true
		s.orientation = clip.RelativeOrientation
		s.area = clip.AreaA
		s.squaredArea = clip.SquaredArea
		s.vertices = clip.Vertices
	default:
		return nil, geom.ErrInvalid
	}
	// Independently corroborate every exact published model endpoint on both
	// original source patches, including their actual retained chart domains.
	for _, v := range s.vertices {
		for owner, uv := range [2][2]*big.Rat{v.A, v.B} {
			if s.work >= o.MaxOperations {
				return nil, geom.ErrConvergence
			}
			jet, e := s.exact[owner].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - s.work, MaxCoefficientBits: o.MaxCoefficientBits})
			s.work += jet.ExactOperations
			if e != nil {
				return nil, e
			}
			for axis := 0; axis < 3; axis++ {
				s.work++
				if s.work > o.MaxOperations {
					return nil, geom.ErrConvergence
				}
				if jet.Point[axis].Cmp(v.Point[axis]) != 0 {
					return nil, geom.ErrInvalid
				}
			}
		}
	}
	count := 0
	if s.kind == AffinePatchContactSegment {
		count = 1
	} else if s.kind == AffinePatchContactArea {
		count = len(s.vertices)
	}
	for i := 0; i < count; i++ {
		j := (i + 1) % len(s.vertices)
		x, y := s.vertices[i], s.vertices[j]
		edge := AffinePatchContactEdge{Start: i, End: j, Boundary: [2]uint8{x.BoundaryA & y.BoundaryA, x.BoundaryB & y.BoundaryB}}
		if s.kind == AffinePatchContactArea && edge.Boundary[0] == 0 && edge.Boundary[1] == 0 {
			return nil, geom.ErrInvalid
		}
		s.edges = append(s.edges, edge)
	}
	return s, nil
}
