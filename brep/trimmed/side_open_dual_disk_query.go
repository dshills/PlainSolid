package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type sideOpenDualDiskSource struct {
	frame         geom.Frame
	min, max      geom.Vec2
	discs         [2]ExtrusionCircle
	height, depth float64
	outer         []ExactQuadraticUV
	owners        []int
}

// Locate classifies a represented world point in the material of the
// original cut. The signed frame and the circle/stock comparisons are
// exact; no proximity tolerance moves a point onto the boundary.
func (c SideOpenDualDiskCut) Locate(ctx context.Context, point geom.Vec3) (parametric.Location, error) {
	if !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	return c.LocateRational(ctx, [3]*big.Rat{ratInset(point.X), ratInset(point.Y), ratInset(point.Z)})
}

// LocateRational is Locate with an exact rational world point.
func (c SideOpenDualDiskCut) LocateRational(ctx context.Context, point [3]*big.Rat) (parametric.Location, error) {
	return locateSideOpenDualDisk(ctx, c.source, point)
}

func locateSideOpenDualDisk(ctx context.Context, s *sideOpenDualDiskSource, point [3]*big.Rat) (parametric.Location, error) {
	if ctx == nil || s == nil || point[0] == nil || point[1] == nil || point[2] == nil {
		return parametric.Outside, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return parametric.Outside, e
	}
	o := s.frame.Origin()
	delta := [3]*big.Rat{new(big.Rat).Sub(point[0], ratInset(o.X)), new(big.Rat).Sub(point[1], ratInset(o.Y)), new(big.Rat).Sub(point[2], ratInset(o.Z))}
	var local [3]*big.Rat
	lo, hi := [3]float64{s.min.X, s.min.Y, 0}, [3]float64{s.max.X, s.max.Y, s.height}
	stockBoundary := false
	for i, a := range [3]geom.Vec3{s.frame.XAxis(), s.frame.YAxis(), s.frame.ZAxis()} {
		for j, n := range [3]float64{a.X, a.Y, a.Z} {
			if n != 0 { // The constructor proved a signed coordinate frame.
				local[i] = new(big.Rat).Mul(delta[j], ratInset(n))
				break
			}
		}
		l, h := local[i].Cmp(ratInset(lo[i])), local[i].Cmp(ratInset(hi[i]))
		if l < 0 || h > 0 {
			return parametric.Outside, nil
		}
		stockBoundary = stockBoundary || l == 0 || h == 0
	}
	if z := local[2].Cmp(ratInset(s.depth)); z <= 0 {
		// The first boundary in the union can be inside the other circle.
		// Classify by the minimum sign of the two disk equations.
		union := 1
		dy := new(big.Rat).Sub(local[1], ratInset(s.min.Y))
		dy2 := new(big.Rat).Mul(dy, dy)
		for _, d := range s.discs {
			r := ratInset(d.Radius)
			r.Mul(r, r)
			dx := new(big.Rat).Sub(local[0], ratInset(d.Center.X))
			d2 := dx.Mul(dx, dx).Add(dx, dy2)
			union = min(union, d2.Cmp(r))
		}
		if union < 0 {
			if z < 0 || s.depth == s.height {
				return parametric.Outside, nil
			}
			return parametric.Boundary, nil // The blind floor remains material.
		}
		if union == 0 {
			return parametric.Boundary, nil
		}
	}
	if stockBoundary {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
}

// SideOpenDualDiskMeasures reports scalar measures and the world centroid.
type SideOpenDualDiskMeasures struct {
	Volume, Area float64
	Centroid     geom.Vec3
}

// Measures returns the volume, surface area and centroid of the original
// cut from closed-form circular integrals, rounded to float64. It does not
// integrate a sampled boundary or depend on a display mesh.
func (c SideOpenDualDiskCut) Measures() (SideOpenDualDiskMeasures, error) {
	s := c.source
	if s == nil {
		return SideOpenDualDiskMeasures{}, geom.ErrInvalid
	}
	lo, hi := min(s.discs[0].Center.X, s.discs[1].Center.X), max(s.discs[0].Center.X, s.discs[1].Center.X)
	r := s.discs[0].Radius
	// Split the union at its symmetry plane. The half width d and
	// normalized ordinate at the join use the exact caller geometry.
	d := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(hi), ratInset(lo)), big.NewRat(2, 1))
	a := new(big.Rat).Quo(d, ratInset(r))
	alpha, _ := a.Float64()
	h2 := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Mul(a, a))
	square, _ := h2.Float64()
	h := math.Sqrt(square)
	angle := math.Pi - math.Atan2(h, alpha)
	normArea := angle + alpha*h
	openingArea := r * (r * normArea)
	openingWidth := (2 * r) * (1 + alpha)
	arcLength := (2 * r) * angle
	width, length := s.max.X-s.min.X, s.max.Y-s.min.Y
	boxVolume := (width * length) * s.height
	removedVolume := openingArea * s.depth
	volume := boxVolume - removedVolume
	area := 2*(width*length+(width+length)*s.height) + (arcLength-openingWidth)*s.depth
	if s.depth == s.height {
		area -= 2 * openingArea
	}
	// The first moment relative to the open side is
	// r³(1+alpha-(1+alpha³)/3). The x moment is fixed by symmetry.
	diskCentroidY := r * ((1 + alpha - (1+alpha*alpha*alpha)/3) / normArea)
	stockX, diskX := s.min.X+width/2, lo+(hi-lo)/2
	ratio := removedVolume / volume
	center := s.frame.ToWorld(geom.Vec3{
		X: stockX + ratio*(stockX-diskX),
		Y: s.min.Y + (length/2 + ratio*(length/2-diskCentroidY)),
		Z: s.height/2 + ratio*((s.height-s.depth)/2),
	})
	if volume <= 0 || area <= 0 || !finite(volume) || !finite(area) || !center.IsFinite() {
		return SideOpenDualDiskMeasures{}, geom.ErrUnrepresentable
	}
	return SideOpenDualDiskMeasures{Volume: volume, Area: area, Centroid: center}, nil
}
