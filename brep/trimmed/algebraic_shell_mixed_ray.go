package trimmed

import (
	"context"

	"plainsolid/geom"
)

// IntersectMaterialLineEvents reports the finite line transitions of the
// mixed material with original operand and face ownership. At a proved
// simultaneous crossing both original faces remain in the event. The
// sweep uses original source charts and never the display mesh.
func (s *MixedAlgebraicShellBoolean) IntersectMaterialLineEvents(ctx context.Context, origin, direction geom.Vec3, o SplineRayOptions) ([]AlgebraicShellLineEvent, error) {
	if s == nil || s.section == nil {
		return nil, geom.ErrInvalid
	}
	return s.boundary.IntersectMaterialLineEvents(ctx, origin, direction, o)
}
