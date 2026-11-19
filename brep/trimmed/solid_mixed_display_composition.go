//go:build research

package trimmed

import (
	"context"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// Then applies a further exact proper pose to the source occurrence. It uses
// the original prepared trim mesh, not intermediate rounded display vertices,
// and recomputes the bound from the original authenticated source certificate.
// Native IDs/parameters remain unchanged; fresh final contact/nesting and all
// composition/placement quotas are retained. No accumulated rounding is hidden.
func (p *PlacedStoredMixedDisplay) Then(ctx context.Context, next placement.RationalRigid, o placement.ClosedDisplayOptions) (*PlacedStoredMixedDisplay, error) {
	if p == nil || p.source == nil || p.display == nil {
		return nil, geom.ErrInvalid
	}
	world, e := p.display.Then(ctx, next, o)
	if e != nil {
		return nil, e
	}
	return p.source.bindPlacedDisplay(ctx, world)
}
