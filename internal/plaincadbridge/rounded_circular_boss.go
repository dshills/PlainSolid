package plaincadbridge

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func (b *Bridge) fuseRoundedCircularBoss(stock *analyticBody, base roundedStock, tool *cylinderBody) (Shape, bool, error) {
	normal, axis := base.frame.ZAxis(), tool.frame.ZAxis()
	if !cardinalFrame(base.frame) || axis != normal && axis != normal.Scale(-1) {
		return Shape{}, false, nil
	}
	origin := tool.frame.Origin()
	start := base.frame.ToLocal(origin)
	if start.X != 0 || start.Y != 0 || base.frame.ToWorld(start) != origin {
		return Shape{}, false, nil
	}
	exact := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	datum := exact(normal.Dot(base.frame.Origin()))
	lo := new(big.Rat).Sub(exact(normal.Dot(origin)), datum)
	hi := new(big.Rat).Add(lo, exact(tool.height*axis.Dot(normal)))
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	cap := exact(base.height)
	if lo.Cmp(exact(base.chamfer)) < 0 || lo.Cmp(cap) > 0 || hi.Cmp(cap) <= 0 {
		return Shape{}, false, nil
	}
	top, err := geom.RepresentedExactScalar(hi)
	if err != nil {
		return Shape{}, true, err
	}
	height, err := geom.RepresentedExactScalar(new(big.Rat).Sub(hi, cap))
	if err != nil {
		return Shape{}, true, err
	}
	ctx := context.Background()
	frame, err := geom.NewFrame(base.frame.ToWorld(geom.Vec3{Z: base.height}), base.frame.XAxis(), normal)
	if err != nil {
		return Shape{}, true, err
	}
	if exact(normal.Dot(frame.Origin())).Cmp(new(big.Rat).Add(datum, cap)) != 0 ||
		exact(normal.Dot(frame.ToWorld(geom.Vec3{Z: height}))).Cmp(new(big.Rat).Add(datum, hi)) != 0 {
		return Shape{}, true, geom.ErrUnrepresentable
	}
	layer, err := trimmed.ExtrudeDiskWithHoles(ctx, frame, trimmed.ExtrusionCircle{Radius: tool.radius}, nil, nil, height)
	if err != nil {
		return Shape{}, true, err
	}
	// The sew proves the disk fits the inset upper cap. The overlapping part
	// lies above the bottom chamfer where the stock sections are at least as wide.
	solid, upper, err := trimmed.SewPlanarCapPrism(ctx, stock.solid, layer, 1, 0, 1, false)
	if err != nil {
		return Shape{}, true, err
	}
	base.stack = &roundedStack{top: top, upper: upper, pocketCap: -1, centerBoss: true}
	out, err := b.putAnalyticBody(&analyticBody{solid: solid, rounded: &base,
		volume: stock.volume + math.Pi*tool.radius*tool.radius*height,
		area:   stock.area + 2*math.Pi*tool.radius*height})
	return out, true, err
}
