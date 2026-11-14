package plaincadbridge

import (
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func (b *Bridge) putSteppedShaftSTEP(m step.SteppedShaftModel) (Shape, error) {
	p := m.Body
	scale := m.LengthUnit / .001
	if !finite(scale) || scale <= 0 {
		return Shape{}, geom.ErrDomain
	}
	if scale == 1 {
		return b.putSteppedShaft(p)
	}
	frame, err := geom.NewFrame(p.Frame.Origin().Scale(scale), p.Frame.XAxis(), p.Frame.ZAxis())
	if err != nil {
		return Shape{}, err
	}
	p.Frame = frame
	p.HeadRadius, p.ShaftRadius = p.HeadRadius*scale, p.ShaftRadius*scale
	p.HeadHeight, p.ShaftLength, p.SocketFloor = p.HeadHeight*scale, p.ShaftLength*scale, p.SocketFloor*scale
	for i, v := range p.Socket {
		p.Socket[i] = geom.Vec2{X: v.X * scale, Y: v.Y * scale}
	}
	return b.putSteppedShaft(p)
}
