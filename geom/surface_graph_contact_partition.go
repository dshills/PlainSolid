package geom

import (
	"context"
	"math/big"
)

// GraphContactPartitionCell is a knot-aligned chart cell. Coincident means its
// entire closed rectangle agrees. Otherwise Side proves strict A.Z-B.Z sign
// on its open interior; closed-boundary zeros are exactly ZeroSides/ZeroCorners.
type GraphContactPartitionCell struct {
	Domain      ParameterRectangle
	Coincident  bool
	Side        int
	ZeroSides   [4]int
	ZeroCorners [4]int
}
type GraphContactCellAdjacency struct {
	Cells        [2]int
	Edges        [2]SurfaceEdge
	ZeroBoundary int
}

// GraphContactZeroSide is a canonical exact UV segment. FixedAxis is 0 for a
// fixed U line, 1 for fixed V; Interval is the other coordinate. Cells/Edges
// retain its incidence on one or two accepted cells. With Complete, one means
// outer boundary; an incomplete report may instead border a Pending cell.
type GraphContactZeroSide struct {
	FixedAxis int
	Fixed     *big.Rat
	Interval  [2]*big.Rat
	Cells     []int
	Edges     []SurfaceEdge
}

// GraphContactZeroCorner identifies an exact shared knot vertex, including zero
// vertices lying on zero sides. Corners use local order LL,HL,HH,LH.
type GraphContactZeroCorner struct {
	Parameters [2]*big.Rat
	Cells      []int
	Corners    []int
}

// SplineGraphContactPartition gives an exhaustive common-XY contact set only
// when Complete. Coincident cells, zero sides and zero corners then cover every
// model contact. Exact projected injectivity excludes off-diagonal UV contacts.
// Pending preserves cells whose mixed coefficients do not prove a strict side.
type SplineGraphContactPartition struct {
	Complete        bool
	Cells           []GraphContactPartitionCell
	Adjacencies     []GraphContactCellAdjacency
	ZeroSides       []GraphContactZeroSide
	ZeroCorners     []GraphContactZeroCorner
	Pending         []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// PartitionSplineGraphContact builds a sufficient exact partial-coincidence
// partition over the union of both knot grids. Positive rational weights and
// weak Bernstein signs prove each accepted cell. Different degrees/weights are
// handled by exact homogeneous products. No sampling or tolerance changes zeros.
func PartitionSplineGraphContact(ctx context.Context, a, c *SurfacePatch, o GraphPointContactOptions) (r SplineGraphContactPartition, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	patches, e := graphContactBezierCells(ctx, difference, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += r.Graph.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Graph.ProvenInjective {
		return r, ErrConvergence
	}
	type edgeRecord struct {
		cell          int
		edge          SurfaceEdge
		zero          bool
		axis          int
		fixed, lo, hi *big.Rat
	}
	edges := map[string][]edgeRecord{}
	order := []string{}
	corners := map[string]int{}
	for index, p := range patches {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		d := p.Domain()
		cell := GraphContactPartitionCell{Domain: d, ZeroSides: [4]int{-1, -1, -1, -1}, ZeroCorners: [4]int{-1, -1, -1, -1}}
		positive, negative, nonzero := true, true, false
		for _, h := range p.net {
			sign := h[2].Sign()
			positive = positive && sign >= 0
			negative = negative && sign <= 0
			nonzero = nonzero || sign != 0
		}
		if !nonzero {
			cell.Coincident = true
		} else if positive {
			cell.Side = 1
		} else if negative {
			cell.Side = -1
		} else {
			r.Pending = append(r.Pending, d)
			r.Cells = append(r.Cells, cell)
			continue
		}
		r.Cells = append(r.Cells, cell)
		for edge := SurfaceEdge(0); edge < 4; edge++ {
			zero := true
			for _, i := range p.edgeIndices(edge) {
				zero = zero && p.net[i][2].Sign() == 0
			}
			axis := 0
			fixed, lo, hi := d.UMin, d.VMin, d.VMax
			switch edge {
			case SurfaceEdgeUMax:
				fixed = d.UMax
			case SurfaceEdgeVMin:
				axis = 1
				fixed, lo, hi = d.VMin, d.UMin, d.UMax
			case SurfaceEdgeVMax:
				axis = 1
				fixed, lo, hi = d.VMax, d.UMin, d.UMax
			}
			f, l, h := new(big.Rat).SetFloat64(fixed), new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(hi)
			prefix := "u:"
			if axis == 1 {
				prefix = "v:"
			}
			key := prefix + f.RatString() + ":" + l.RatString() + ":" + h.RatString()
			if _, ok := edges[key]; !ok {
				order = append(order, key)
			}
			edges[key] = append(edges[key], edgeRecord{index, edge, zero, axis, f, l, h})
		}
		indices := []int{0, (p.countU - 1) * p.countV, len(p.net) - 1, p.countV - 1}
		points := [4][2]float64{{d.UMin, d.VMin}, {d.UMax, d.VMin}, {d.UMax, d.VMax}, {d.UMin, d.VMax}}
		for corner, k := range indices {
			if p.net[k][2].Sign() != 0 {
				continue
			}
			u, v := new(big.Rat).SetFloat64(points[corner][0]), new(big.Rat).SetFloat64(points[corner][1])
			key := u.RatString() + "," + v.RatString()
			id, ok := corners[key]
			if !ok {
				id = len(r.ZeroCorners)
				corners[key] = id
				r.ZeroCorners = append(r.ZeroCorners, GraphContactZeroCorner{Parameters: [2]*big.Rat{u, v}})
			}
			r.ZeroCorners[id].Cells = append(r.ZeroCorners[id].Cells, index)
			r.ZeroCorners[id].Corners = append(r.ZeroCorners[id].Corners, corner)
			r.Cells[index].ZeroCorners[corner] = id
		}
	}
	for _, key := range order {
		records := edges[key]
		if len(records) > 2 {
			return r, ErrConvergence
		}
		id := -1
		allZero := true
		for _, record := range records {
			allZero = allZero && record.zero
		}
		if len(records) == 2 && records[0].zero != records[1].zero {
			return r, ErrConvergence
		}
		if allZero {
			first := records[0]
			id = len(r.ZeroSides)
			side := GraphContactZeroSide{FixedAxis: first.axis, Fixed: first.fixed, Interval: [2]*big.Rat{first.lo, first.hi}}
			for _, record := range records {
				side.Cells = append(side.Cells, record.cell)
				side.Edges = append(side.Edges, record.edge)
				r.Cells[record.cell].ZeroSides[record.edge] = id
			}
			r.ZeroSides = append(r.ZeroSides, side)
		}
		if len(records) == 2 {
			r.Adjacencies = append(r.Adjacencies, GraphContactCellAdjacency{Cells: [2]int{records[0].cell, records[1].cell}, Edges: [2]SurfaceEdge{records[0].edge, records[1].edge}, ZeroBoundary: id})
		}
	}
	if len(r.Pending) > 0 {
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}
