package parametric

import (
	"context"
	"fmt"
	"math"

	"plainsolid/geom"
)

// IntegrationOptions bounds the surface quadrature. RelativeTolerance defaults
// to 1e-8. MaxSubdivisions defaults to 64 per parameter direction, per face.
type IntegrationOptions struct {
	RelativeTolerance float64
	MaxSubdivisions   int
	// MaxSamples bounds quadrature evaluations over all refinements. Zero
	// selects 4,000,000 samples.
	MaxSamples int
}

// Properties describes an outward-oriented solid. The change estimates are
// observed differences between consecutive refinements; they are not rigorous
// error bounds. CentroidChange is a length in model units.
type Properties struct {
	Area, Volume                             float64
	Centroid                                 geom.Vec3
	AreaChange, VolumeChange, CentroidChange float64
	// Inertia is the centroidal inertia tensor in model axes at unit density.
	Inertia       [3][3]float64
	InertiaChange float64
}

// MassProperties integrates the actual parametric boundary, independent of
// tessellation. It uses tensor Gauss quadrature and the divergence theorem for
// volume and first moments. The caller must provide a properly oriented,
// embedded boundary. Returns ErrBudget if the requested convergence is not
// observed; highly oscillatory custom surfaces can alias the quadrature.
func (s Solid) MassProperties(ctx context.Context, o IntegrationOptions) (Properties, error) {
	if len(s.faces) == 0 {
		return Properties{}, geom.ErrDegenerate
	}
	return IntegrateBoundary(ctx, s.faces, s.vertices[0], o)
}

// IntegrateBoundary applies the same quadrature to a supplied list of oriented
// rectangular parameter patches. The caller must establish that the patches
// form an embedded, consistently oriented closed boundary; this numerical
// routine does not read or validate their topology or edge references.
func IntegrateBoundary(ctx context.Context, faces []Face, origin geom.Vec3, o IntegrationOptions) (Properties, error) {
	return integrateBoundary(ctx, faces, origin, o, nil)
}

// IntegrateOrientedBoundary integrates parameter patches that may overlap or
// fold in their own charts. orientation returns the sign of the local area
// factor relative to the underlying chart, before applying Face.Reversed.
// Patches must cover the desired embedded boundary with signed multiplicity
// one. A negative sign subtracts area; normal flux already carries its sign.
func IntegrateOrientedBoundary(ctx context.Context, faces []Face, origin geom.Vec3, o IntegrationOptions, orientation func(face int, u, v float64) (int, error)) (Properties, error) {
	if orientation == nil {
		return Properties{}, geom.ErrInvalid
	}
	return integrateBoundary(ctx, faces, origin, o, orientation)
}

func integrateBoundary(ctx context.Context, faces []Face, origin geom.Vec3, o IntegrationOptions, orientation func(int, float64, float64) (int, error)) (Properties, error) {
	if ctx == nil || !finite(o.RelativeTolerance) || o.RelativeTolerance < 0 || o.RelativeTolerance >= 1 || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 512 || o.MaxSamples < 0 {
		return Properties{}, fmt.Errorf("parametric: invalid integration options")
	}
	if err := ctx.Err(); err != nil {
		return Properties{}, err
	}
	if len(faces) == 0 {
		return Properties{}, geom.ErrDegenerate
	}
	if o.RelativeTolerance == 0 {
		o.RelativeTolerance = 1e-8
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 64
	}
	remaining := o.MaxSamples
	if remaining == 0 {
		remaining = 4000000
	}
	if len(faces) > remaining/25 {
		return Properties{}, ErrBudget
	}
	remaining -= 25 * len(faces)
	if !origin.IsFinite() {
		return Properties{}, geom.ErrInvalid
	}
	scale := 0.0
	for _, f := range faces {
		if f.Surface == nil || !f.U.valid() || !f.V.valid() {
			return Properties{}, geom.ErrInvalid
		}
		if err := ctx.Err(); err != nil {
			return Properties{}, err
		}
		for i := 0; i <= 4; i++ {
			for j := 0; j <= 4; j++ {
				p, e := f.point(float64(i)/4, float64(j)/4)
				if e != nil {
					return Properties{}, e
				}
				d := p.Sub(origin)
				scale = math.Max(scale, math.Max(math.Abs(d.X), math.Max(math.Abs(d.Y), math.Abs(d.Z))))
			}
		}
	}
	if scale <= 0 || !finite(scale) {
		return Properties{}, fmt.Errorf("parametric: invalid integration scale")
	}
	var prev [11]float64
	converged := 0
	for n := 1; n <= o.MaxSubdivisions; n *= 2 {
		perFace := 16 * n * n
		if len(faces) > remaining/perFace {
			return Properties{}, ErrBudget
		}
		remaining -= len(faces) * perFace
		v, err := integral(ctx, faces, n, origin, scale, orientation)
		if err != nil {
			return Properties{}, err
		}
		if n > 1 && v[0] > 0 && v[1] > 0 && prev[0] > 0 && prev[1] > 0 {
			aDelta := math.Abs(v[0] - prev[0])
			vDelta := math.Abs(v[1] - prev[1])
			c := geom.Vec3{X: v[2] / v[1], Y: v[3] / v[1], Z: v[4] / v[1]}
			cp := geom.Vec3{X: prev[2] / prev[1], Y: prev[3] / prev[1], Z: prev[4] / prev[1]}
			cDelta := c.Sub(cp).Length()
			inertia, prevInertia := centralInertia(v, c), centralInertia(prev, cp)
			var iDelta, iScale float64
			for i := range 3 {
				for j := range 3 {
					iDelta = math.Max(iDelta, math.Abs(inertia[i][j]-prevInertia[i][j]))
					iScale = math.Max(iScale, math.Abs(inertia[i][j]))
				}
			}
			if aDelta <= o.RelativeTolerance*v[0] && vDelta <= o.RelativeTolerance*v[1] && cDelta <= o.RelativeTolerance && iDelta <= o.RelativeTolerance*iScale {
				converged++
				// Require two successive refinements to lower aliasing risk.
				if converged == 2 {
					area := v[0] * scale * scale
					volume := v[1] * scale * scale * scale
					centroid := origin.Add(c.Scale(scale))
					if area <= 0 || !finite(area) || volume <= 0 || !finite(volume) || !centroid.IsFinite() {
						return Properties{}, fmt.Errorf("parametric: mass properties outside float64 range")
					}
					scaled5 := func(x float64) float64 {
						for range 5 {
							x *= scale
						}
						return x
					}
					for i := range 3 {
						for j := range 3 {
							inertia[i][j] = scaled5(inertia[i][j])
							if !finite(inertia[i][j]) || i == j && inertia[i][j] <= 0 {
								return Properties{}, fmt.Errorf("parametric: inertia outside float64 range")
							}
						}
					}
					return Properties{
						Area: area, Volume: volume, Centroid: centroid,
						AreaChange: aDelta * scale * scale, VolumeChange: vDelta * scale * scale * scale, CentroidChange: cDelta * scale,
						Inertia: inertia, InertiaChange: scaled5(iDelta),
					}, nil
				}
			} else {
				converged = 0
			}
		}
		prev = v
	}
	return Properties{}, ErrBudget
}

func integral(ctx context.Context, faces []Face, n int, origin geom.Vec3, scale float64, orientation func(int, float64, float64) (int, error)) ([11]float64, error) {
	// Four-point Gauss-Legendre nodes mapped from [-1,1] to [0,1].
	nodes := [4]float64{.06943184420297371, .33000947820757187, .6699905217924281, .9305681557970262}
	weights := [4]float64{.17392742256872693, .32607257743127307, .32607257743127307, .17392742256872693}
	var sum, correction [11]float64
	div := func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: p.X / scale, Y: p.Y / scale, Z: p.Z / scale} }
	for index, f := range faces {
		du, dv := (f.U.Max-f.U.Min)/float64(n), (f.V.Max-f.V.Min)/float64(n)
		for i := 0; i < n; i++ {
			if err := ctx.Err(); err != nil {
				return sum, err
			}
			for j := 0; j < n; j++ {
				for k, x := range nodes {
					for l, y := range nodes {
						u, v := f.U.at((float64(i)+x)/float64(n)), f.V.at((float64(j)+y)/float64(n))
						p, err := f.Surface.Eval(u, v)
						if err != nil {
							return sum, err
						}
						a, b, err := f.Surface.Partials(u, v)
						if err != nil {
							return sum, err
						}
						r := div(p.Sub(origin))
						normal := div(a).Scale(du * weights[k]).Cross(div(b).Scale(dv * weights[l]))
						if f.Reversed {
							normal = normal.Scale(-1)
						}
						flux := r.Dot(normal)
						if !r.IsFinite() || !normal.IsFinite() || !finite(flux) {
							return sum, fmt.Errorf("parametric: nonfinite surface integrand")
						}
						area := normal.Length()
						if orientation != nil {
							sign, e := orientation(index, u, v)
							if e != nil {
								return sum, e
							}
							if sign < -1 || sign > 1 || sign == 0 && area != 0 {
								return sum, geom.ErrInvalid
							}
							area *= float64(sign)
						}
						terms := [11]float64{area, flux / 3, r.X * flux / 4, r.Y * flux / 4, r.Z * flux / 4,
							r.X * r.X * flux / 5, r.Y * r.Y * flux / 5, r.Z * r.Z * flux / 5,
							r.X * r.Y * flux / 5, r.X * r.Z * flux / 5, r.Y * r.Z * flux / 5}
						for z, w := range terms {
							y := w - correction[z]
							t := sum[z] + y
							correction[z] = (t - sum[z]) - y
							sum[z] = t
						}
					}
				}
			}
		}
	}
	for _, v := range sum {
		if !finite(v) {
			return sum, fmt.Errorf("parametric: nonfinite surface integral")
		}
	}
	return sum, nil
}

func centralInertia(v [11]float64, c geom.Vec3) [3][3]float64 {
	xx, yy, zz := v[5]-v[1]*c.X*c.X, v[6]-v[1]*c.Y*c.Y, v[7]-v[1]*c.Z*c.Z
	xy, xz, yz := v[1]*c.X*c.Y-v[8], v[1]*c.X*c.Z-v[9], v[1]*c.Y*c.Z-v[10]
	return [3][3]float64{{yy + zz, xy, xz}, {xy, xx + zz, yz}, {xz, yz, xx + yy}}
}
