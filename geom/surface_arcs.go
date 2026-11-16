package geom

import (
	"context"
	"math/big"
)

// SurfacePlaneArcOptions bounds exact certification work. Zero fields select
// endpoint tolerances 1e-8, depth 20, 16384 cells, 262144 queued poles,
// 2M rational operations, 16384 coefficient bits and 8192 root subdivisions.
type SurfacePlaneArcOptions struct {
	// IsolateQuadraticTangencies proves isolated interior contacts when the
	// homogeneous plane numerator has a definite quadratic Hessian.
	IsolateQuadraticTangencies bool
	// ResolveSubdivisionCorners accepts a corner-only cell if its exact
	// Bernstein coefficients prove there is no other contact. A global
	// trace still requires an adjacent arc to account for that point.
	ResolveSubdivisionCorners bool
	// MaxCellParameterWidth, when positive, forces each certified arc cell to
	// have U and V widths at most this value. Zero preserves adaptive defaults.
	MaxCellParameterWidth                                                                      float64
	EndpointParameterTolerance, EndpointBoxTolerance                                           float64
	MaxDepth, MaxCells, MaxQueuedPoles, MaxOperations, MaxCoefficientBits, MaxRootSubdivisions int
}

// SurfacePlaneArc certifies one regular, injective spatial arc in Domain.
// Endpoints enclose its two boundary points. EdgeRootCounts counts distinct
// roots on each edge (corners count on both); -1 denotes an entire zero edge.
// Neighboring arcs are not stitched and may share endpoints or boundary arcs.
type SurfacePlaneArc struct {
	source         *SurfacePatch
	plane          Plane
	Domain         ParameterRectangle
	Bounds         Box
	Endpoints      [2]SurfacePlaneEndpoint
	Certificate    SurfaceArcCertificate
	EdgeRootCounts [4]int
	Depth          int
	OnBoundary     bool
	BoundaryEdge   SurfaceEdge
}

// SurfacePlaneUnresolved preserves a cell whose topology could not be certified.
type SurfacePlaneUnresolved struct {
	Domain ParameterRectangle
	Bounds Box
	Depth  int
	Reason string
}

// SurfacePlaneArcs covers contact by certified arcs, proved isolated
// tangencies, exact coplanar cells, unresolved cells and pending cells.
// Complete excludes the latter two; it does
// not imply that arcs have been globally stitched or that the surface is injective.
type SurfacePlaneArcs struct {
	Arcs                                            []SurfacePlaneArc
	Tangencies                                      []SurfacePlaneTangency
	CornerContacts                                  []SurfacePlaneEndpoint
	Coplanar, Pending                               []ParameterRectangle
	Unresolved                                      []SurfacePlaneUnresolved
	VisitedCells, ExactOperations, RootSubdivisions int
	Complete                                        bool
}

func certifyArcEdges(p *SurfacePatch, data surfaceArcData, cert SurfaceArcCertificate, o SurfacePlaneArcOptions, b *polynomialBudget, subdivisions *int) (SurfacePlaneArc, bool, error) {
	arc := SurfacePlaneArc{Domain: p.Domain(), Bounds: p.bounds, Certificate: cert}
	var coefficients [4][]*big.Rat
	zeroEdge := -1
	for edge := SurfaceEdge(0); edge < 4; edge++ {
		allZero := true
		for _, index := range p.edgeIndices(edge) {
			v := data.fields[4][index]
			coefficients[edge] = append(coefficients[edge], v)
			allZero = allZero && v.Sign() == 0
		}
		if allZero {
			if zeroEdge >= 0 {
				return arc, false, nil
			}
			zeroEdge = int(edge)
		}
	}
	if zeroEdge >= 0 {
		edge := SurfaceEdge(zeroEdge)
		normalAxis := 0
		if edge >= SurfaceEdgeVMin {
			normalAxis = 1
		}
		if cert.FixedParameterAxis != normalAxis {
			return arc, false, nil
		}
		for i := 0; i < 2; i++ {
			x := big.NewRat(int64(i), 1)
			ep, ok, e := p.endpoint(edge, x, x, 1, o.EndpointParameterTolerance, o.EndpointBoxTolerance, b)
			if e != nil || !ok {
				return arc, false, e
			}
			arc.Endpoints[i] = ep
		}
		arc.EdgeRootCounts = [4]int{1, 1, 1, 1}
		arc.EdgeRootCounts[edge] = -1
		arc.EdgeRootCounts[edge^1] = 0
		arc.OnBoundary = true
		arc.BoundaryEdge = edge
		return arc, true, nil
	}
	endpoints := []SurfacePlaneEndpoint{}
	corners := map[int]bool{}
	valid := true
	for edge := SurfaceEdge(0); edge < 4; edge++ {
		poly, e := bernsteinToPower(coefficients[edge], b)
		if e != nil {
			return arc, false, e
		}
		remaining := o.MaxRootSubdivisions - *subdivisions
		if remaining < 1 {
			return arc, false, ErrConvergence
		}
		n, e := isolatePolynomialRoots(poly, big.NewRat(0, 1), big.NewRat(1, 1), b, remaining, func(a, c *big.Rat, m int) (bool, error) {
			ep, ok, err := p.endpoint(edge, a, c, m, o.EndpointParameterTolerance, o.EndpointBoxTolerance, b)
			if err != nil || !ok {
				return ok, err
			}
			ep.identity.polynomial = poly
			arc.EdgeRootCounts[edge]++
			if ep.CornerIndex >= 0 {
				enters, decidable := cornerEntersPatch(p, data, ep.CornerIndex, cert.FixedParameterAxis)
				valid = valid && enters && decidable
				if corners[ep.CornerIndex] {
					return true, nil
				}
				corners[ep.CornerIndex] = true
			} else if m != 1 {
				valid = false
			}
			endpoints = append(endpoints, ep)
			return true, nil
		})
		*subdivisions += n
		if e != nil {
			return arc, false, e
		}
	}
	if !valid || len(endpoints) != 2 {
		return arc, false, nil
	}
	copy(arc.Endpoints[:], endpoints)
	return arc, true, nil
}

// CertifySurfacePlaneArcs uses exact rational Bernstein hulls and Sturm edge
// roots. A strictly signed partial of the plane numerator makes its zero set a
// parameter graph. Two simple entering boundary endpoints identify one arc;
// a strictly signed spatial-coordinate derivative proves spatial regularity
// and injectivity. Tangent/singular or ambiguous cells are retained explicitly.
// Exhaustion returns ErrConvergence with a conservative covering of remaining
// work. Certification applies to the exact real surface defined by input floats.
func CertifySurfacePlaneArcs(patch *SurfacePatch, plane Plane, options SurfacePlaneArcOptions) (result SurfacePlaneArcs, err error) {
	return CertifySurfacePlaneArcsContext(context.Background(), patch, plane, options)
}

// CertifySurfacePlaneArcsContext checks cancellation between bounded cells.
// Pending still covers cells that were not processed.
func CertifySurfacePlaneArcsContext(ctx context.Context, patch *SurfacePatch, plane Plane, options SurfacePlaneArcOptions) (result SurfacePlaneArcs, err error) {
	return certifySurfacePlaneArcsContext(ctx, patch, plane, options, nil)
}

type surfaceArcProjectionCertificate func(*SurfacePatch, surfaceArcData, int, *polynomialBudget) (SurfaceArcCertificate, bool, error)

func certifySurfacePlaneArcsContext(ctx context.Context, patch *SurfacePatch, plane Plane, options SurfacePlaneArcOptions,
	projected surfaceArcProjectionCertificate) (result SurfacePlaneArcs, err error) {
	if ctx == nil || patch == nil || len(patch.net) == 0 || !plane.valid() {
		return result, ErrInvalid
	}
	o := options
	if o.EndpointParameterTolerance == 0 {
		o.EndpointParameterTolerance = 1e-8
	}
	if o.EndpointBoxTolerance == 0 {
		o.EndpointBoxTolerance = 1e-8
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 20
	}
	if o.MaxCells == 0 {
		o.MaxCells = 16384
	}
	if o.MaxQueuedPoles == 0 {
		o.MaxQueuedPoles = 262144
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxRootSubdivisions == 0 {
		o.MaxRootSubdivisions = 8192
	}
	if !finite(o.MaxCellParameterWidth) || o.MaxCellParameterWidth < 0 || !finite(o.EndpointParameterTolerance) || o.EndpointParameterTolerance <= 0 || !finite(o.EndpointBoxTolerance) || o.EndpointBoxTolerance <= 0 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxQueuedPoles < 4 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxRootSubdivisions < 1 {
		return result, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	type node struct {
		p     *SurfacePatch
		depth int
	}
	stack := []node{{patch, 0}}
	queued := len(patch.net)
	pending := func(p *SurfacePatch) {
		if p != nil {
			result.Pending = append(result.Pending, p.Domain())
		}
		for _, n := range stack {
			result.Pending = append(result.Pending, n.p.Domain())
		}
	}
	if queued > o.MaxQueuedPoles {
		pending(nil)
		return result, ErrConvergence
	}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			pending(nil)
			return result, e
		}
		if result.VisitedCells >= o.MaxCells {
			pending(nil)
			return result, ErrConvergence
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		p := n.p
		queued -= len(p.net)
		result.VisitedCells++
		r, e := p.planeBounds(plane, b)
		if e != nil {
			pending(p)
			return result, e
		}
		if r.Relation == PlanePatchAbove || r.Relation == PlanePatchBelow {
			continue
		}
		if r.Relation == PlanePatchCoplanar {
			result.Coplanar = append(result.Coplanar, p.Domain())
			continue
		}
		uk, hu := internalPatchKnot(p.knotsU, p.degreeU, p.countU)
		vk, hv := internalPatchKnot(p.knotsV, p.degreeV, p.countV)
		reason := "unresolved boundary topology"
		allowArc := o.MaxCellParameterWidth == 0 || patchParameterWidth(p) <= o.MaxCellParameterWidth
		if !hu && !hv && (allowArc || o.IsolateQuadraticTangencies || o.ResolveSubdivisionCorners) {
			data, e := p.arcData(plane, b)
			if e != nil {
				pending(p)
				return result, e
			}
			if o.IsolateQuadraticTangencies {
				x, isolated, e := definiteQuadraticTangency(p, data.fields[4], b)
				if e != nil {
					pending(p)
					return result, e
				}
				if isolated {
					result.Tangencies = append(result.Tangencies, x)
					continue
				}
			}
			if o.ResolveSubdivisionCorners {
				point, isolated, e := isolatedSurfaceCorner(p, data, o, b)
				if e != nil {
					pending(p)
					return result, e
				}
				if isolated {
					result.CornerContacts = append(result.CornerContacts, point)
					continue
				}
			}
			certified := false
			for axis := 0; allowArc && axis < 2; axis++ {
				cert, ok, e := data.certify(axis, b)
				if e == nil && !ok && projected != nil {
					cert, ok, e = projected(p, data, axis, b)
				}
				if e != nil {
					pending(p)
					return result, e
				}
				if !ok {
					continue
				}
				arc, ok, e := certifyArcEdges(p, data, cert, o, b, &result.RootSubdivisions)
				if e != nil {
					pending(p)
					return result, e
				}
				if ok {
					arc.source, arc.plane = p, plane
					arc.Depth = n.depth
					result.Arcs = append(result.Arcs, arc)
					certified = true
					break
				}
			}
			if certified {
				continue
			}
			if data.du[4].sign() == 0 && data.dv[4].sign() == 0 {
				reason = "plane numerator gradient not certified"
			}
		}
		d := p.Domain()
		splitU, t := true, uk
		if !hu {
			if hv {
				splitU, t = false, vk
			} else if d.UMax-d.UMin >= d.VMax-d.VMin {
				t = d.UMin + (d.UMax-d.UMin)/2
			} else {
				splitU, t = false, d.VMin+(d.VMax-d.VMin)/2
			}
		}
		if n.depth >= o.MaxDepth || (splitU && (t == d.UMin || t == d.UMax)) || (!splitU && (t == d.VMin || t == d.VMax)) {
			result.Unresolved = append(result.Unresolved, SurfacePlaneUnresolved{d, p.bounds, n.depth, reason})
			continue
		}
		l, rp, e := p.split(t, splitU, b)
		if e != nil {
			pending(p)
			return result, e
		}
		size := len(l.net) + len(rp.net)
		if size > o.MaxQueuedPoles-queued {
			pending(p)
			return result, ErrConvergence
		}
		queued += size
		stack = append(stack, node{rp, n.depth + 1}, node{l, n.depth + 1})
	}
	if len(result.Unresolved) > 0 {
		return result, ErrConvergence
	}
	result.Complete = true
	return result, nil
}

func isolatedSurfaceCorner(p *SurfacePatch, data surfaceArcData, o SurfacePlaneArcOptions, b *polynomialBudget) (SurfacePlaneEndpoint, bool, error) {
	corner, side := -1, 0
	for i, c := range data.fields[4] {
		if c.Sign() == 0 {
			if corner != -1 {
				return SurfacePlaneEndpoint{}, false, nil
			}
			corner = i
		} else if side != 0 && c.Sign() != side {
			return SurfacePlaneEndpoint{}, false, nil
		} else {
			side = c.Sign()
		}
	}
	if corner < 0 {
		return SurfacePlaneEndpoint{}, false, nil
	}
	var edge SurfaceEdge
	t := big.NewRat(0, 1)
	switch corner {
	case 0:
		edge = SurfaceEdgeUMin
	case p.countV - 1:
		edge = SurfaceEdgeUMin
		t.SetInt64(1)
	case (p.countU - 1) * p.countV:
		edge = SurfaceEdgeUMax
	case p.countU*p.countV - 1:
		edge = SurfaceEdgeUMax
		t.SetInt64(1)
	default:
		return SurfacePlaneEndpoint{}, false, nil
	}
	return p.endpoint(edge, t, t, 1, o.EndpointParameterTolerance, o.EndpointBoxTolerance, b)
}
