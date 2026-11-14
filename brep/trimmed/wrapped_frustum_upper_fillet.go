package trimmed

import (
	"context"
	"plainsolid/geom"
)

// WrappedFrustumRimFilletHistory is the shared exact meridian proof and native
// ancestry for either supported end. Tangency/center coordinates are model Z,
// not a reflected scratch chart. The existing lower-end type remains compatible.
type WrappedFrustumRimFilletHistory = WrappedFrustumLowerFilletHistory

// FilletWrappedFrustumUpperRim rounds the original upper rim of a descending
// canonical circular frustum. For slope s<0 and L=sqrt(1+s²), the rolling-circle
// center is (Rupper-radius*(L+s), zupper-radius), and cone normal (1/L,-s/L).
// Tangency is center+radius*normal. The first-quadrant torus endpoint parameter
// t=-s/(L+1) must be represented exactly; the retained arc is V=[t,1].
// These identities prove tangency to both complete revolved supports, while
// native shared-edge agreement independently proves the whole circular joins.
// Major radius must exceed ball radius and side tangency lie strictly inside
// the original height interval. Other domain/work/representation restrictions
// match FilletWrappedFrustumLowerRim. No reflected or rounded source is used.
func FilletWrappedFrustumUpperRim(ctx context.Context, s Solid, radius float64, o geom.SurfacePatchOptions) (Solid, WrappedFrustumRimFilletHistory, error) {
	return filletWrappedFrustumEndRim(ctx, s, radius, true, o)
}
