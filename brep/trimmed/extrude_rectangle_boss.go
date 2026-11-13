package trimmed

import (
	"context"

	"plainsolid/geom"
)

// ExtrusionBoss is a positive-height rectangular protrusion from the upper
// cap (larger local Z) when Upper is true, or the lower cap otherwise.
type ExtrusionBoss struct {
	Footprint ExtrusionRectangle
	Height    float64
	Upper     bool
}

// ExtrudeMixedRectangleWithBoss builds a rounded extrusion with rectangular
// through pockets and circular bores/pockets, adding one rectangular boss.
// The boss must lie strictly in the inner corner-chord polygon and be
// disjoint from all holes; its footprint is removed from the supporting cap
// and shared with the four new side faces. The original body face IDs remain
// stable. The four boss sides and end cap are appended in this order.
func ExtrudeMixedRectangleWithBoss(ctx context.Context, frame geom.Frame, profile RoundedRectangle, rectangles []ExtrusionRectangle, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64, boss ExtrusionBoss) (Solid, error) {
	return extrudeMixedRectangle(ctx, frame, profile, rectangles, &boss, through, blind, height)
}

// ExtrudeMixedRectangleWithBossCertified builds the same native solid and a
// closed mesh with an exact ideal-boundary error bound and source face IDs.
func ExtrudeMixedRectangleWithBossCertified(ctx context.Context, frame geom.Frame, profile RoundedRectangle, rectangles []ExtrusionRectangle, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64, boss ExtrusionBoss, o CertifiedRevolvePolygonOptions) (CertifiedCircularHoleExtrusion, error) {
	return certifyCircularExtrusion(ctx, frame, nil, nil, &profile, rectangles, &boss, through, blind, height, o)
}
