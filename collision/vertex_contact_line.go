package collision

import (
	"context"
	"math/big"
	"math/bits"
	"sort"

	"plainsolid/geom"
)

// AssemblyLineState describes the exact membership and original owner IDs of
// a line station or open span in a proved assembly. BoundaryFaces is populated
// only on the boundary. Components retain their original order.
type AssemblyLineState struct {
	Location      Location
	Components    []int
	BoundaryFaces []AssemblyFace
	// SharedEdge is set only on a complete station or open span in the
	// original shared edge of a proved edge-contact pair.
	SharedEdge bool
}

type AssemblyLineStation struct {
	Parameter *big.Rat
	AssemblyLineState
	Contacts []AssemblyVertexContact
}

type AssemblyLineSpan struct {
	Start, End int
	AssemblyLineState
}

// AssemblyLinePartition preserves exact stations, their open spans and the
// contact ledger at individual original shared vertices. Adjacent interior
// spans in separate components remain separate even if their point is shared.
type AssemblyLinePartition struct {
	Stations []AssemblyLineStation
	Spans    []AssemblyLineSpan
	Work     int
}

type componentLinePartition struct {
	id int
	r  RationalLinePartition
}

// PartitionRationalLine reports the complete ordered membership partition of
// a rational line through the prepared assembly. Component proofs and contact
// nodes are reused. Exact input, traversal, and merged cells share both the
// locator's aggregate query limit and any smaller explicit MaxWork.
func (l *VertexContactAssemblyLocator) PartitionRationalLine(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o RationalLinePartitionOptions) (AssemblyLinePartition, error) {
	var zero AssemblyLinePartition
	if l == nil || ctx == nil || o.MaxWork < 0 {
		return zero, ErrInvalid
	}
	limit := l.maxWork
	if limit == 0 {
		limit = 2000000
	}
	if o.MaxWork > 0 {
		limit = min(limit, o.MaxWork)
	}
	o.MaxWork = limit
	// The empty locator establishes the range and validates every rational
	// input even when none of the component bounds meets the finite line.
	empty := PointLocator{}
	seed, err := empty.PartitionRationalLine(ctx, origin, direction, minT, maxT, o)
	if err != nil {
		return zero, err
	}
	b := &locationBudget{ctx: ctx, limit: limit, work: seed.Work}
	parameters := make([]*big.Rat, 0, len(seed.Stations))
	for _, station := range seed.Stations {
		parameters = append(parameters, station.Parameter)
	}
	var parts []componentLinePartition
	for i, component := range l.components {
		if err := b.tick(1); err != nil {
			return zero, err
		}
		if !lineMeetsClosedBox(component.box, origin, direction, minT, maxT) {
			continue
		}
		if b.work >= b.limit {
			return zero, ErrLimit
		}
		componentOptions := o
		componentOptions.MaxWork = min(b.limit-b.work, component.loc.options.MaxWork)
		p, err := component.loc.PartitionRationalLine(ctx, origin, direction, minT, maxT, componentOptions)
		if err != nil {
			return zero, err
		}
		if err := b.tick(p.Work); err != nil {
			return zero, err
		}
		for _, station := range p.Stations {
			parameters = append(parameters, station.Parameter)
		}
		parts = append(parts, componentLinePartition{i, p})
	}
	if err := b.tick(len(parameters) * bits.Len(uint(len(parameters)))); err != nil {
		return zero, err
	}
	parameters = uniqueLineParameters(parameters)
	r := AssemblyLinePartition{Stations: make([]AssemblyLineStation, len(parameters)), Spans: make([]AssemblyLineSpan, len(parameters)-1)}
	for i, at := range parameters {
		state, err := assemblyLineStateAt(parts, at, b)
		if err != nil {
			return zero, err
		}
		s := AssemblyLineStation{Parameter: new(big.Rat).Set(at), AssemblyLineState: state}
		if len(state.Components) > 1 {
			xyz := plus(origin, scale(direction, at))
			if l.edge != nil {
				if !onSegment(xyz, l.edge[0], l.edge[1]) {
					return zero, ErrInvalid
				}
				s.SharedEdge = true
			} else {
				x, ex := xyz[0].Float64()
				y, ey := xyz[1].Float64()
				z, ez := xyz[2].Float64()
				if !ex || !ey || !ez {
					return zero, ErrInvalid // admitted vertex contacts have represented coordinates
				}
				s.Contacts = append([]AssemblyVertexContact(nil), l.contacts[geom.Vec3{X: x, Y: y, Z: z}]...)
				if len(s.Contacts) != len(state.Components)*(len(state.Components)-1)/2 {
					return zero, ErrInvalid
				}
			}
		}
		r.Stations[i] = s
		if i == 0 {
			continue
		}
		mid := div(add(parameters[i-1], at), big.NewRat(2, 1))
		state, err = assemblyLineStateAt(parts, mid, b)
		if err != nil {
			return zero, err
		}
		if len(state.Components) > 1 {
			if l.edge == nil ||
				!onSegment(plus(origin, scale(direction, parameters[i-1])), l.edge[0], l.edge[1]) ||
				!onSegment(plus(origin, scale(direction, at)), l.edge[0], l.edge[1]) {
				return zero, ErrInvalid
			}
			state.SharedEdge = true
		}
		r.Spans[i-1] = AssemblyLineSpan{i - 1, i, state}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	r.Work = b.work
	return r, nil
}

func assemblyLineStateAt(parts []componentLinePartition, at *big.Rat, b *locationBudget) (AssemblyLineState, error) {
	var r AssemblyLineState
	if err := b.tick(1); err != nil {
		return r, err
	}
	for _, p := range parts {
		if err := b.tick(bits.Len(uint(len(p.r.Stations)))); err != nil {
			return AssemblyLineState{}, err
		}
		i := sort.Search(len(p.r.Stations), func(i int) bool { return p.r.Stations[i].Parameter.Cmp(at) >= 0 })
		var location Location
		var faces []int
		if i < len(p.r.Stations) && p.r.Stations[i].Parameter.Cmp(at) == 0 {
			location, faces = p.r.Stations[i].Location, p.r.Stations[i].BoundaryFaces
		} else if i > 0 && i < len(p.r.Stations) {
			location, faces = p.r.Spans[i-1].Location, p.r.Spans[i-1].BoundaryFaces
		}
		if location == Outside {
			continue
		}
		if r.Location == Inside || location == Inside && r.Location != Outside {
			return AssemblyLineState{}, ErrInvalid
		}
		if err := b.tick(len(faces)); err != nil {
			return AssemblyLineState{}, err
		}
		r.Location = location
		r.Components = append(r.Components, p.id)
		for _, face := range faces {
			r.BoundaryFaces = append(r.BoundaryFaces, AssemblyFace{Solid: p.id, Face: face})
		}
	}
	return r, nil
}

func lineMeetsClosedBox(box geom.Box, o, d point, lo, hi *big.Rat) bool {
	a, z := exact(box.Min), exact(box.Max)
	for i := range d {
		if d[i].Sign() == 0 {
			if o[i].Cmp(a[i]) < 0 || o[i].Cmp(z[i]) > 0 {
				return false
			}
			continue
		}
		start, end := div(sub(a[i], o[i]), d[i]), div(sub(z[i], o[i]), d[i])
		if start.Cmp(end) > 0 {
			start, end = end, start
		}
		if start.Cmp(lo) > 0 {
			lo = start
		}
		if end.Cmp(hi) < 0 {
			hi = end
		}
		if lo.Cmp(hi) > 0 {
			return false
		}
	}
	return true
}
