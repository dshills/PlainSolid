// Command splineenvelope writes a shared-base spline union as native geometry
// and a conforming STL mesh.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func graph(baseHeight, xSquared float64) (*geom.BSplineSurface, error) {
	knots := []float64{0, 0, 0, 1, 1, 1}
	var poles []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			z := baseHeight
			if i == 2 {
				z += xSquared
			}
			poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
		}
	}
	return geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
}

func build(ctx context.Context) (trimmed.Solid, trimmed.SplineGraphBooleanHistory, error) {
	var surfaces [3]*geom.BSplineSurface
	for i, p := range [][2]float64{{0, 0}, {.75, 1}, {1, 0}} {
		s, e := graph(p[0], p[1])
		if e != nil {
			return trimmed.Solid{}, trimmed.SplineGraphBooleanHistory{}, e
		}
		surfaces[i] = s
	}
	return trimmed.CombineSplineGraphSolids(ctx, surfaces[0], surfaces[1], surfaces[2], parametric.Union, 1e-8, intersect.SplinePlaneOptions{})
}

func write(name string, encode func(io.Writer) error) error {
	f, e := os.Create(name)
	if e != nil {
		return e
	}
	e = encode(f)
	if err := f.Close(); e == nil {
		e = err
	}
	return e
}

func run(ctx context.Context, native, stl string) error {
	s, h, e := build(ctx)
	if e != nil {
		return e
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		return e
	}
	m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .015})
	if e != nil {
		return e
	}
	if e = write(native, func(w io.Writer) error { return trimmed.WriteJSON(w, s) }); e != nil {
		return e
	}
	if e = write(stl, func(w io.Writer) error { return exchange.WriteSTL(w, m) }); e != nil {
		return e
	}
	fmt.Printf("Volume %.9f; %d shared section edges; %d mesh triangles\n", p.Volume, len(h.ContourEdges), len(m.Triangles))
	return nil
}

func main() {
	native := flag.String("native", "spline-envelope.json", "native trimmed B-rep output")
	stl := flag.String("stl", "spline-envelope.stl", "binary STL output")
	flag.Parse()
	if e := run(context.Background(), *native, *stl); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
