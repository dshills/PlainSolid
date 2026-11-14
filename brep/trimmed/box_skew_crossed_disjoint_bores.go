package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

const MaxSkewCrossedDisjointBores = 2

// BoxSkewCrossedBoresWithDisjoint adds at most two independent cardinal through
// bores to a qualified unequal/covered-skew crossing. Full projected padded
// envelopes must separate every companion from the core and other companions.
func BoxSkewCrossedBoresWithDisjoint(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, xRadius, zRadius, offset float64, additional []CardinalThroughBore, tolerance float64) (BoxCrossedBoresResult, error) {
	if ctx == nil || len(additional) == 0 || len(additional) > MaxSkewCrossedDisjointBores || !finite(tolerance) || tolerance <= 0 {
		return BoxCrossedBoresResult{}, geom.ErrInvalid
	}
	core, e := BoxSkewCrossedBores(ctx, frame, dimensions, crossing, xRadius, zRadius, offset, tolerance)
	if e != nil {
		return BoxCrossedBoresResult{}, e
	}
	return appendCrossedDisjointBores(ctx, core, frame, dimensions, skewCrossedCoreEnvelopes(crossing, xRadius, zRadius, offset), additional, tolerance)
}

// BoxFiniteSkewCrossedBoresWithDisjoint preserves original input X/Z floor IDs
// and appends companion wall groups in canonical axis/center/radius order.
// Companion tools are through; the core uses the finite crossing's existing
// complete-intersection floor restriction. Full projections prove separation.
func BoxFiniteSkewCrossedBoresWithDisjoint(ctx context.Context, frame geom.Frame, dimensions, crossing geom.Vec3, xRadius, zRadius, offset, xDepth, zDepth float64, xOpposite, zOpposite bool, additional []CardinalThroughBore, tolerance float64) (FiniteSkewCrossedBoresResult, error) {
	if ctx == nil || len(additional) == 0 || len(additional) > MaxSkewCrossedDisjointBores || !finite(tolerance) || tolerance <= 0 {
		return FiniteSkewCrossedBoresResult{}, geom.ErrInvalid
	}
	core, e := BoxFiniteSkewCrossedBores(ctx, frame, dimensions, crossing, xRadius, zRadius, offset, xDepth, zDepth, xOpposite, zOpposite, tolerance)
	if e != nil {
		return FiniteSkewCrossedBoresResult{}, e
	}
	merged, e := appendCrossedDisjointBores(ctx, core.BoxCrossedBoresResult, frame, dimensions, skewCrossedCoreEnvelopes(crossing, xRadius, zRadius, offset), additional, tolerance)
	if e != nil {
		return FiniteSkewCrossedBoresResult{}, e
	}
	core.BoxCrossedBoresResult = merged
	return core, nil
}
func skewCrossedCoreEnvelopes(c geom.Vec3, xRadius, zRadius, offset float64) []CardinalThroughBore {
	return []CardinalThroughBore{{Axis: 0, Center: geom.Vec3{Y: c.Y, Z: c.Z}, Radius: xRadius}, {Axis: 2, Center: geom.Vec3{X: c.X, Y: c.Y + offset}, Radius: zRadius}}
}

// TessellateSkewCrossedWithDisjoint uses the retained radical crossing trims
// and exact companion supports without changing the original mesh budgets.
func (s Solid) TessellateSkewCrossedWithDisjoint(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	if o.AnalyticBound {
		return mesh.Mesh{}, parametric.ErrDeflectionUnsupported
	}
	if e := ctx.Err(); e != nil {
		return mesh.Mesh{}, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 128
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	count := len(s.faces)
	if count < 20 || count > 18+4*MaxSkewCrossedDisjointBores || (count-16)%4 == 3 {
		return mesh.Mesh{}, geom.ErrDomain
	}
	prefix := 16 + (count-16)%4
	if !skewCompanionMeshFamily(s, prefix) {
		return mesh.Mesh{}, geom.ErrDomain
	}
	found := false
	for i, f := range s.faces {
		base, _ := parametric.SurfaceMapping(f.Surface)
		switch base.(type) {
		case geom.PlaneSurface, geom.CylinderSurface:
		default:
			return mesh.Mesh{}, geom.ErrDomain
		}
		if i >= prefix {
			if _, ok := base.(geom.CylinderSurface); !ok || len(f.Outer) != 4 || len(f.Holes) != 0 {
				return mesh.Mesh{}, geom.ErrDomain
			}
		}
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				switch u.Curve.(type) {
				case unequalBoreTrim:
					found = true
				case nil, EllipseUV, unionArcUV:
				default:
					return mesh.Mesh{}, geom.ErrDomain
				}
			}
		}
	}
	if !found {
		return mesh.Mesh{}, geom.ErrDomain
	}
	return s.tessellateAnalyticAdaptive(ctx, o)
}

// Identify only the normalized unequal core and the canonically appended
// rectangular companion groups produced by the constructors above.
func skewCompanionMeshFamily(s Solid, prefix int) bool {
	cylinder := func(f Face) (geom.Frame, float64, bool) {
		base, _ := parametric.SurfaceMapping(f.Surface)
		c, ok := base.(geom.CylinderSurface)
		if !ok {
			return geom.Frame{}, 0, false
		}
		frame, r := c.Parameters()
		return frame, r, true
	}
	sf, sr, ok := cylinder(s.faces[6])
	if !ok {
		return false
	}
	lf, lr, ok := cylinder(s.faces[14])
	if !ok || sr >= lr || sf.ZAxis().Dot(lf.ZAxis()) != 0 {
		return false
	}
	for i, f := range s.faces[:prefix] {
		if i < 6 || i >= 16 {
			base, _ := parametric.SurfaceMapping(f.Surface)
			if _, ok := base.(geom.PlaneSurface); !ok || len(f.Outer) != 4 || f.Reversed != (i >= 16) {
				return false
			}
			if i >= 16 && len(f.Holes) != 0 {
				return false
			}
			continue
		}
		frame, r, ok := cylinder(f)
		if !ok || !f.Reversed {
			return false
		}
		if i < 14 {
			if frame != sf || r != sr || len(f.Outer) != 4 || len(f.Holes) != 0 {
				return false
			}
			n := 0
			for _, u := range f.Outer {
				if _, ok := u.Curve.(unequalBoreTrim); ok {
					n++
				} else if u.Curve != nil {
					return false
				}
			}
			if n != 1 {
				return false
			}
		} else {
			if frame != lf || r != lr || len(f.Outer) != 6 || len(f.Holes) != 1 || len(f.Holes[0]) != 4 {
				return false
			}
			for _, u := range f.Holes[0] {
				if _, ok := u.Curve.(unequalBoreTrim); !ok {
					return false
				}
			}
		}
	}
	for group := prefix; group < len(s.faces); group += 4 {
		cf, cr, ok := cylinder(s.faces[group])
		if !ok {
			return false
		}
		var low, high float64
		for j := 0; j < 4; j++ {
			f := s.faces[group+j]
			frame, r, ok := cylinder(f)
			if !ok || frame != cf || r != cr || !f.Reversed || len(f.Outer) != 4 || len(f.Holes) != 0 {
				return false
			}
			for _, u := range f.Outer {
				if u.Curve != nil {
					return false
				}
			}
			b, e := faceUVBounds(f)
			if e != nil || b.UMin != float64(j)*math.Pi/2 || b.UMax != float64(j+1)*math.Pi/2 {
				return false
			}
			if j == 0 {
				low, high = b.VMin, b.VMax
			} else if low != b.VMin || high != b.VMax {
				return false
			}
		}
	}
	return true
}
