// Command ellipticallens writes the exact closed difference between two
// shared-base spline graphs meeting along an interior ellipse.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func graph(c, factor float64) (*geom.BSplineSurface, error) {
	k := []float64{0, 0, 0, 1, 1, 1}
	square := [3]float64{.25, -.25, .25}
	var p []geom.Vec3
	for i := range square {
		for j := range square {
			p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2,
				Z: c + factor*(1-16*square[i]-64*square[j])})
		}
	}
	return geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
}

func run(ctx context.Context, w io.Writer) (parametric.Properties, error) {
	var surfaces [3]*geom.BSplineSurface
	for i, data := range [3][2]float64{{0, 0}, {24, 1}, {24, 0}} {
		s, e := graph(data[0], data[1])
		if e != nil {
			return parametric.Properties{}, e
		}
		surfaces[i] = s
	}
	ellipse := geom.GraphEllipseHint{
		Center:        [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)},
		RadiusSquared: [2]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 64)},
	}
	body, _, e := trimmed.CombineEllipticalContactSplineGraphs(ctx, surfaces[0], surfaces[1], surfaces[2],
		parametric.Difference, 1e-9, intersect.SplinePlaneOptions{}, ellipse, geom.GraphPointContactOptions{})
	if e != nil {
		return parametric.Properties{}, e
	}
	p, e := body.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		return parametric.Properties{}, e
	}
	return p, trimmed.WriteJSON(w, body)
}

func main() {
	output := flag.String("out", "elliptical-lens.json", "native trimmed solid output")
	flag.Parse()
	f, e := os.Create(*output)
	if e != nil {
		log.Fatal(e)
	}
	p, e := run(context.Background(), f)
	if closeErr := f.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		log.Fatal(e)
	}
	fmt.Printf("Wrote %s (volume %.9f)\n", *output, p.Volume)
}
