// Command pressurecap clips a sphere with a plane and writes its exact native
// boundary and a conforming STL mesh.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"plainsolid/brep/parametric"
	"plainsolid/exchange"
	"plainsolid/geom"
)

func main() {
	native := flag.String("native", "pressure-cap.json", "path for exact native B-rep")
	stl := flag.String("stl", "pressure-cap.stl", "path for tessellation")
	flag.Parse()
	if err := run(context.Background(), *native, *stl); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(ctx context.Context, native, stl string) error {
	if native == stl {
		return fmt.Errorf("choose distinct output paths")
	}
	p, e := geom.NewPlane(geom.Vec3{Z: 4}, geom.Vec3{Z: 1})
	if e != nil {
		return e
	}
	s, e := parametric.ClipSphere(parametric.Ball{Radius: 10}, p, 1e-9)
	if e != nil {
		return e
	}
	info, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		return e
	}
	m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .04, AnalyticBound: true})
	if e != nil {
		return e
	}
	if e = write(native, func(w io.Writer) error { return parametric.WriteJSON(w, s) }); e != nil {
		return e
	}
	if e = write(stl, func(w io.Writer) error { return exchange.WriteSTL(w, m) }); e != nil {
		return e
	}
	fmt.Printf("volume %.9g, area %.9g; wrote %d triangles\n", info.Volume, info.Area, len(m.Triangles))
	return nil
}
func write(path string, f func(io.Writer) error) error {
	file, e := os.Create(path)
	if e != nil {
		return e
	}
	err := f(file)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
