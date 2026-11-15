package collision

import (
	"context"
	"math/big"

	"plainsolid/brep"
	"plainsolid/geom"
)

// PlanarEdgeContactLocator reuses the exact original part proofs for point
// and rational line queries. Its snapshots are immutable.
type PlanarEdgeContactLocator struct {
	core  *VertexContactAssemblyLocator
	proof PlanarEdgeContact
}

// NewPlanarEdgeContactLocator verifies a single complete common edge before
// publishing an exact classifier for the two independent bodies.
func NewPlanarEdgeContactLocator(ctx context.Context, a, b brep.Solid, o PlanarEdgeContactOptions) (*PlanarEdgeContactLocator, error) {
	var prepared [2]vertexContactComponent
	p, err := discoverPlanarEdgeContact(ctx, a, b, o, &prepared)
	if err != nil {
		return nil, err
	}
	core := &VertexContactAssemblyLocator{
		components: []vertexContactLocatedComponent{{prepared[0].loc, prepared[0].box}, {prepared[1].loc, prepared[1].box}},
		maxWork:    o.MaxQueryWork,
		edge:       &[2]point{exact(p.Start), exact(p.End)},
	}
	return &PlanarEdgeContactLocator{core: core, proof: p}, nil
}

// Report returns the original edge, its owners and exact summed material volume.
func (l *PlanarEdgeContactLocator) Report() PlanarEdgeContact {
	if l == nil || l.core == nil {
		return PlanarEdgeContact{}
	}
	p := l.proof
	p.SignedVolume = new(big.Rat).Set(p.SignedVolume)
	return p
}

// Locate reports exact material membership and both body owners when the
// point is on the original shared edge.
func (l *PlanarEdgeContactLocator) Locate(ctx context.Context, p geom.Vec3) (VertexContactAssemblyLocation, error) {
	if l == nil {
		return VertexContactAssemblyLocation{}, ErrInvalid
	}
	return l.core.Locate(ctx, p)
}

// PartitionRationalLine retains both owners for shared stations and open spans
// on the original edge. Those stations may have nonbinary model coordinates.
func (l *PlanarEdgeContactLocator) PartitionRationalLine(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o RationalLinePartitionOptions) (AssemblyLinePartition, error) {
	if l == nil {
		return AssemblyLinePartition{}, ErrInvalid
	}
	return l.core.PartitionRationalLine(ctx, origin, direction, minT, maxT, o)
}
