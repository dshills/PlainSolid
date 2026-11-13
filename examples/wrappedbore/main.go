// wrappedbore builds a rational spline cylinder with a blind coaxial bore,
// then writes the exact material boundary to STEP and native JSON.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func build(ctx context.Context) (step.WrappedBlindBoreBody, error) {
	o := geom.SurfacePatchOptions{}
	outer, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 20, 40, o)
	if e != nil {
		return step.WrappedBlindBoreBody{}, e
	}
	inner, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{Z: 25}, 8, 15, o)
	if e != nil {
		return step.WrappedBlindBoreBody{}, e
	}
	s, _, e := trimmed.NewWrappedCylinderBore(ctx, outer, inner, 1e-8, o)
	if e != nil {
		return step.WrappedBlindBoreBody{}, e
	}
	return step.WrappedBlindBoreBody{
		Name: "Bearing pocket", Outer: step.WrappedSplineFace{Name: "Outside wall", Geometry: outer},
		Inner:    step.WrappedSplineFace{Name: "Pocket wall", Geometry: inner},
		CapNames: [2]string{"Base", "Opening"}, FloorName: "Pocket floor",
		Solid: s, LengthUnit: .001,
	}, nil
}

func writeFile(path string, write func(io.Writer) error) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	e = write(f)
	closeErr := f.Close()
	if e != nil {
		return e
	}
	return closeErr
}

func main() {
	prefix := flag.String("out", "wrappedbore", "output basename")
	flag.Parse()
	m, e := build(context.Background())
	if e == nil {
		e = writeFile(*prefix+".step", func(w io.Writer) error { return step.WriteWrappedBlindBore(w, m) })
	}
	if e == nil {
		e = writeFile(*prefix+".json", func(w io.Writer) error { return trimmed.WriteJSON(w, m.Solid) })
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
