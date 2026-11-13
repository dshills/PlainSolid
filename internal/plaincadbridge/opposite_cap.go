package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type filletedCapStock struct {
	frame  geom.Frame
	size   geom.Vec3
	radius float64
}

func (b *Bridge) chamferOppositeCap(s stored, q request, normal geom.Vec3) (Shape, error) {
	p := s.analytic.filletedCap
	reject := func() (Shape, error) {
		return Shape{}, unsupportedEdge("opposite cap perimeter was lost or is ambiguous")
	}
	if normal != p.frame.ZAxis().Scale(-1) {
		return reject()
	}
	w, d, h, r, c := p.size.X, p.size.Y, p.size.Z, p.radius, float64(q.Distance)
	corners := [4]geom.Vec3{{}, {X: w}, {X: w, Y: d}, {Y: d}}
	seen := [4]bool{}
	for _, edge := range q.Edges {
		if edge.Start == nil || edge.End == nil {
			return reject()
		}
		found := false
		for i, corner := range corners {
			if !seen[i] && sameEnds(edge.Start.vec(), edge.End.vec(), p.frame.ToWorld(corner), p.frame.ToWorld(corners[(i+1)%4])) {
				seen[i], found = true, true
				break
			}
		}
		if !found {
			return reject()
		}
	}
	solid, err := trimmed.TopFilletedBottomChamferedBox(context.Background(), p.frame, w, d, h, r, c, 1e-9)
	if err != nil {
		return Shape{}, err
	}
	volume := s.analytic.volume - (w+d)*c*c + 4*c*c*c/3
	area := (w-2*c)*(d-2*c) + (w-2*r)*(d-2*r) + 2*(w+d)*(h-r-c) +
		math.Pi*r*(w+d-4*r) + 8*r*r + 2*math.Sqrt2*c*(w+d-2*c)
	return b.putAnalytic(solid, volume, area)
}

func (p *filletedCapStock) retainedLowerEdge(a, b geom.Vec3) bool {
	w, d := p.size.X, p.size.Y
	corners := [4]geom.Vec3{{}, {X: w}, {X: w, Y: d}, {Y: d}}
	for i, corner := range corners {
		if sameEnds(a, b, p.frame.ToWorld(corner), p.frame.ToWorld(corners[(i+1)%4])) {
			return true
		}
	}
	return false
}
