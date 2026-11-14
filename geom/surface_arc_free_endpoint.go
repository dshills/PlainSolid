package geom

import "math/big"

// ExactFreeEndpoint returns the original free coordinate of a certified arc
// endpoint. An irrational dependent coordinate need not be reconstructed when
// the private boundary identity fixes the free coordinate exactly. This does
// not claim a rational UV or model point. Returned values are independent copies.
func (a SurfacePlaneArc) ExactFreeEndpoint(index int, o ArcExactEndpointOptions) (*big.Rat, error) {
	if a.source == nil || index < 0 || index > 1 {
		return nil, ErrInvalid
	}
	if o.MaxOperations < 0 || (o.MaxCoefficientBits != 0 && o.MaxCoefficientBits < 64) {
		return nil, ErrInvalid
	}
	free := 1 - a.Certificate.FixedParameterAxis
	id := a.Endpoints[index].identity
	if (free == 0 || free == 1) && id != nil && id.axis == free && id.fixed != nil {
		bits := o.MaxCoefficientBits
		if bits == 0 {
			bits = 16384
		}
		if id.fixed.Num().BitLen() > bits || id.fixed.Denom().BitLen() > bits {
			return nil, ErrConvergence
		}
		return new(big.Rat).Set(id.fixed), nil
	}
	p, e := a.ExactEndpoint(index, o)
	if e != nil {
		return nil, e
	}
	return p.FreeParameter, nil
}
