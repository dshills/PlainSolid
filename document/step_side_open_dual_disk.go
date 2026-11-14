package document

import (
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// WriteSideOpenDualDiskSTEP exports one authored paired side-cut occurrence.
// The stock and cut are regenerated before output. Only exact signed-coordinate
// world placements and supported SI units can retain the original body.
func (s State) WriteSideOpenDualDiskSTEP(ctx context.Context, w io.Writer, nodeID string) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	node, hasNode := s.Nodes[nodeID]
	f, hasFeature := s.Features[node.Asset]
	if !hasNode || !hasFeature || f.Kind != SideOpenDualDiskCutFeature {
		return fmt.Errorf("document: node %q does not own an authored side-opening cut", nodeID)
	}
	built, e := s.EvaluateFeatures(ctx)
	if e != nil {
		return e
	}
	if e := built.Validate(); e != nil {
		return e
	}
	world, e := built.worldTransforms()
	if e != nil {
		return e
	}
	var d step.SideOpenDualDiskDescriptor
	if d, e = authoredSideOpenDualDiskDescriptor(built.Planar[f.Input], f.SideOpenDualDisk); e != nil {
		return e
	}
	d, e = placeSideOpenDualDisk(d, world[nodeID])
	if e != nil {
		return e
	}
	d.Name, d.LengthUnit = node.Name, s.LengthUnit
	if d.Name == "" {
		d.Name = nodeID
	}
	if f.SideOpenDualDisk.OffsetCenters {
		return step.WriteOffsetSideOpenDualDiskProduct(ctx, w, step.OffsetSideOpenDualDiskDescriptor(d))
	}
	if f.SideOpenDualDisk.ExactWalls {
		for _, face := range built.Trimmed[node.Asset].Faces() {
			if _, exact := face.Surface.(*geom.ExactBezierSurface); exact {
				return step.WriteExactWallSideOpenDualDiskProduct(ctx, w, step.ExactWallSideOpenDualDiskDescriptor(d))
			}
		}
	}
	if d.Discs[0].Radius == d.Discs[1].Radius {
		return step.WriteSideOpenDualDiskProduct(ctx, w, d)
	}
	if externalSideDiskRadical(d) {
		return step.WriteGeneralSideOpenDualDiskProduct(ctx, w, step.GeneralSideOpenDualDiskDescriptor(d))
	}
	return step.WriteUnequalSideOpenDualDiskProduct(ctx, w, step.UnequalSideOpenDualDiskDescriptor(d))
}

func placeSideOpenDualDisk(d step.SideOpenDualDiskDescriptor, m geom.Affine) (step.SideOpenDualDiskDescriptor, error) {
	var origin [3]float64
	used := [3]bool{}
	p := d.Frame.Origin()
	for i, row := range m {
		axis := -1
		for j, coeff := range row[:3] {
			if coeff == 0 {
				continue
			}
			if axis >= 0 || used[j] || coeff != 1 && coeff != -1 {
				return step.SideOpenDualDiskDescriptor{}, geom.ErrDomain
			}
			axis = j
		}
		if axis < 0 {
			return step.SideOpenDualDiskDescriptor{}, geom.ErrDomain
		}
		used[axis] = true
		a := new(big.Rat).SetFloat64(([3]float64{p.X, p.Y, p.Z})[axis] * row[axis])
		b := new(big.Rat).SetFloat64(row[3])
		if a == nil || b == nil {
			return step.SideOpenDualDiskDescriptor{}, geom.ErrInvalid
		}
		var exact bool
		origin[i], exact = a.Add(a, b).Float64()
		if !exact {
			return step.SideOpenDualDiskDescriptor{}, geom.ErrUnrepresentable
		}
	}
	x, z := m.ApplyVector(d.Frame.XAxis()), m.ApplyVector(d.Frame.ZAxis())
	if m.Determinant() < 0 {
		x = x.Scale(-1)
		d.Min.X, d.Max.X = -d.Max.X, -d.Min.X
		for i := range d.Discs {
			d.Discs[i].Center.X = -d.Discs[i].Center.X
		}
	}
	f, e := geom.NewFrame(geom.Vec3{X: origin[0], Y: origin[1], Z: origin[2]}, x, z)
	if e != nil || f.YAxis() != m.ApplyVector(d.Frame.YAxis()) {
		return step.SideOpenDualDiskDescriptor{}, geom.ErrDomain
	}
	d.Frame = f
	return d, nil
}
