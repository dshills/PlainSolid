package plaincadbridge

import (
	"errors"
	"math"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type drawingBore struct {
	Diameter  float64 `json:"diameter"`
	Length    float64 `json:"length"`
	Start     vector  `json:"start"`
	Direction vector  `json:"direction"`
}

type drawingGeometry struct {
	Bounds   Bounds        `json:"bounds"`
	Bores    []drawingBore `json:"bores"`
	Warnings []string      `json:"warnings"`
}

type boreBand struct {
	frame          geom.Frame
	radius, z0, z1 float64
	mapping        geom.Affine
}

func (s stored) drawingGeometry() (drawingGeometry, error) {
	out := drawingGeometry{Bounds: s.result.Bounds, Bores: []drawingBore{}, Warnings: []string{}}
	var faces []trimmed.Face
	mapping := geom.Identity()
	switch {
	case s.tangent != nil:
		h := s.tangent
		out.Bores = append(out.Bores, drawingBore{
			Diameter:  2 * h.radius,
			Length:    h.depth,
			Start:     coordinates(h.placement.Apply(geom.Vec3{})),
			Direction: coordinates(h.placement.ApplyVector(geom.Vec3{Z: 1})),
		})
		out.Warnings = append(out.Warnings, "The bore touches a stock side along a non-manifold contact edge.")
		return out, nil
	case s.bore != nil:
		faces = s.bore.solid.Faces()
	case s.treated != nil:
		faces, mapping = s.treated.chain.Solid.Faces(), frameMap(s.treated.frame)
	case s.analytic != nil:
		faces = s.analytic.solid.Faces()
	default:
		return out, nil
	}
	if len(faces) > 4096 {
		return out, &failureError{"LIMIT", "Drawing exceeds the 4096-face native inspection limit"}
	}
	bands := map[boreBand][]parametric.Interval{}
	warn := false
	for _, f := range faces {
		base, m := parametric.SurfaceMapping(f.Surface)
		cylinder, ok := base.(geom.CylinderSurface)
		// A mapped reflection also flips the stored face orientation. Its
		// physical material side follows the two signs together.
		if !ok || f.Reversed == (m.Determinant() < 0) {
			continue
		}
		u, z, ok := rectangularCylinderDomain(f)
		if !ok {
			warn = true
			continue
		}
		frame, r := cylinder.Parameters()
		band := boreBand{frame, r, z.Min, z.Max, mapping.Mul(m)}
		bands[band] = append(bands[band], u)
	}
	for band, ranges := range bands {
		sort.Slice(ranges, func(i, j int) bool { return ranges[i].Min < ranges[j].Min })
		last := ranges[0].Min
		complete := true
		for _, interval := range ranges {
			complete = complete && math.Abs(interval.Min-last) <= 1e-10
			last = interval.Max
		}
		if !complete || math.Abs(last-ranges[0].Min-2*math.Pi) > 1e-10 {
			warn = true
			continue
		}
		if err := rigid(band.mapping); err != nil {
			return out, errors.New("drawing bore requires a rigid cylindrical chart")
		}
		frame := band.frame
		// Match the inward cylindrical chart convention used for drawing
		// callouts. Reorient both the start and the axis together.
		n, err := band.mapping.ApplyVector(frame.ZAxis().Scale(-1)).Unit()
		if err != nil {
			return out, err
		}
		out.Bores = append(out.Bores, drawingBore{2 * band.radius, band.z1 - band.z0,
			coordinates(band.mapping.Apply(frame.ToWorld(geom.Vec3{Z: band.z1}))), coordinates(n)})
	}
	if len(out.Bores) > 64 {
		return out, &failureError{"LIMIT", "Drawing exceeds the 64-bore callout limit"}
	}
	sort.Slice(out.Bores, func(i, j int) bool {
		a, b := out.Bores[i], out.Bores[j]
		for k := range a.Start {
			if a.Start[k] != b.Start[k] {
				return a.Start[k] < b.Start[k]
			}
		}
		if a.Diameter != b.Diameter {
			return a.Diameter < b.Diameter
		}
		return a.Length < b.Length
	})
	if warn {
		out.Warnings = append(out.Warnings, "Trimmed internal cylindrical faces need manual dimensions; incomplete bore callouts are omitted.")
	}
	return out, nil
}

func rectangularCylinderDomain(f trimmed.Face) (u, z parametric.Interval, ok bool) {
	if len(f.Outer) != 4 || len(f.Holes) != 0 {
		return
	}
	p := f.Outer[0].UVStart
	u, z = parametric.Interval{Min: p.X, Max: p.X}, parametric.Interval{Min: p.Y, Max: p.Y}
	for _, trim := range f.Outer {
		a, b := trim.UVStart, trim.UVEnd
		if trim.Curve != nil || (a.X == b.X) == (a.Y == b.Y) {
			return u, z, false
		}
		u.Min, u.Max = min(u.Min, a.X), max(u.Max, a.X)
		z.Min, z.Max = min(z.Min, a.Y), max(z.Max, a.Y)
	}
	for _, trim := range f.Outer {
		a := trim.UVStart
		if a.X != u.Min && a.X != u.Max || a.Y != z.Min && a.Y != z.Max {
			return u, z, false
		}
	}
	return u, z, u.Min < u.Max && z.Max-z.Min > 1e-7
}
