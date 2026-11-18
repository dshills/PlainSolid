package plaincadbridge

import (
	"encoding/base64"
	"fmt"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestBlindTripleCrossedBoreScopeAndSTEP(t *testing.T) {
	slowtest.Skip(t)
	for axis := 0; axis < 3; axis++ {
		for _, side := range []float64{1, -1} {
			t.Run(fmt.Sprintf("%d/%v", axis, side), func(t *testing.T) {
				b := New()
				min, max, center := vector{2, 4, -2}, vector{34, 40, 38}, vector{17, 19, 16}
				stock := box(t, b, min, max)
				var handles []string
				var floor, normal geom.Vec3
				sum := 108.
				for i := 0; i < 3; i++ {
					entry, direction := center, vector{}
					entry[i], direction[i] = min[i], 1
					depth := max[i] - min[i]
					if i == axis {
						if side < 0 {
							entry[i], direction[i] = max[i], side
						}
						depth -= 5.2
						sum -= 5.2
						stop := entry
						stop[i] += side * depth
						floor = stop.vec()
						normal = direction.vec().Scale(-1)
					}
					c := circle(t, b, entry, direction, 3, depth)
					handles = append(handles, c.Handle)
				}
				out, err := b.cutMany(stock.Handle, []string{handles[2], handles[0], handles[1]})
				if err != nil {
					t.Fatal(err)
				}
				wantVolume := 32.*36*40 - sum*9*math.Pi + 216*math.Sqrt2
				wantArea := 2.*(32*36+32*40+36*40) + (sum*6-36)*math.Pi - 216*math.Sqrt2
				near(t, out.GeometryAssertions.Volume, wantVolume)
				near(t, out.GeometryAssertions.SurfaceArea, wantArea)
				saved, err := b.exportSTEP([]string{out.Handle})
				if err != nil {
					t.Fatal(err)
				}
				raw, err := base64.StdEncoding.DecodeString(saved.Base64)
				if err != nil {
					t.Fatal(err)
				}
				got, err := b.importSTEP(string(raw))
				if err != nil || got.Bounds != stock.Bounds {
					t.Fatal(got, err)
				}
				near(t, got.GeometryAssertions.Volume, wantVolume)
				near(t, got.GeometryAssertions.SurfaceArea, wantArea)
				f, err := b.shapes[got.Handle].bore.planarFace(floor, normal)
				if err != nil || f.Face != 6 || f.Normal != coordinates(normal) {
					t.Fatal(f, err)
				}
				near(t, f.Area, 9*math.Pi)
				for _, h := range handles {
					near(t, overlapMeasure(t, b, got.Handle, h), 0)
				}
			})
		}
	}
}

func TestMultipleBlindTripleCrossedBoreScopeAndSTEP(t *testing.T) {
	slowtest.Skip(t)
	for _, depths := range [][3]float64{{28.5, 30, 0}, {0, 30, 32}, {28.5, 30, 32}} {
		b := New()
		stock := box(t, b, vector{}, vector{40, 40, 40})
		center := vector{20, 20, 20}
		tools := make([]string, 3)
		count, span := 0, 0.
		for axis, stop := range depths {
			origin, direction := center, vector{}
			origin[axis], direction[axis] = 0, 1
			if axis == 1 {
				origin[axis], direction[axis] = 40, -1
			}
			depth := 40.
			if stop > 0 {
				depth = stop
				count++
			}
			span += depth
			tools[axis] = circle(t, b, origin, direction, 3, depth).Handle
		}
		out, err := b.cutMany(stock.Handle, tools)
		if err != nil {
			t.Fatal(depths, err)
		}
		pair := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tools[0]})
		pair = shape(t, b, map[string]any{"op": "cut", "a": pair.Handle, "b": tools[1]})
		sequential := shape(t, b, map[string]any{"op": "cut", "a": pair.Handle, "b": tools[2]})
		if out.GeometryAssertions != sequential.GeometryAssertions {
			t.Fatal(depths, out, sequential)
		}
		wantVolume := 64000 - span*9*math.Pi + 216*math.Sqrt2
		wantArea := 9600 + (span*6-float64(3-count)*18)*math.Pi - 216*math.Sqrt2
		near(t, out.GeometryAssertions.Volume, wantVolume)
		near(t, out.GeometryAssertions.SurfaceArea, wantArea)
		file, err := b.exportSTEP([]string{out.Handle})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.StdEncoding.DecodeString(file.Base64)
		if err != nil {
			t.Fatal(err)
		}
		got, err := b.importSTEP(string(raw))
		if err != nil || got.Bounds != stock.Bounds {
			t.Fatal(depths, got, err)
		}
		near(t, got.GeometryAssertions.Volume, wantVolume)
		near(t, got.GeometryAssertions.SurfaceArea, wantArea)
		index := 6
		for _, axis := range [3]int{2, 0, 1} {
			depth := depths[axis]
			if depth == 0 {
				continue
			}
			p, normal := center, vector{}
			p[axis], normal[axis] = depth, -1
			if axis == 1 {
				p[axis], normal[axis] = 40-depth, 1
			}
			face, err := b.shapes[got.Handle].bore.planarFace(p.vec(), normal.vec())
			if err != nil || face.Face != index || face.Normal != normal {
				t.Fatal(depths, axis, face, err)
			}
			near(t, face.Area, 9*math.Pi)
			index++
		}
		for _, tool := range tools {
			near(t, overlapMeasure(t, b, got.Handle, tool), 0)
		}
		if count != 3 {
			continue
		}
		deeper := circle(t, b, vector{0, 20, 20}, vector{1, 0, 0}, 3, 34)
		near(t, overlapMeasure(t, b, got.Handle, deeper.Handle), 5.5*9*math.Pi)
		extended := shape(t, b, map[string]any{"op": "cut", "a": got.Handle, "b": deeper.Handle})
		near(t, extended.GeometryAssertions.Volume, wantVolume-5.5*9*math.Pi)
		near(t, extended.GeometryAssertions.SurfaceArea, wantArea+5.5*6*math.Pi)
		for _, p := range []struct {
			origin, normal geom.Vec3
			id             int
		}{
			{geom.Vec3{X: 20, Y: 20, Z: 32}, geom.Vec3{Z: -1}, 6},
			{geom.Vec3{X: 34, Y: 20, Z: 20}, geom.Vec3{X: -1}, 7},
			{geom.Vec3{X: 20, Y: 10, Z: 20}, geom.Vec3{Y: 1}, 8},
		} {
			face, err := b.shapes[extended.Handle].bore.planarFace(p.origin, p.normal)
			if err != nil || face.Face != p.id {
				t.Fatal(p, face, err)
			}
		}
		through := circle(t, b, vector{40, 20, 20}, vector{-1, 0, 0}, 3, 40)
		near(t, overlapMeasure(t, b, extended.Handle, through.Handle), 6*9*math.Pi)
		opened := shape(t, b, map[string]any{"op": "cut", "a": extended.Handle, "b": through.Handle})
		near(t, opened.GeometryAssertions.Volume, wantVolume-11.5*9*math.Pi)
		near(t, opened.GeometryAssertions.SurfaceArea, wantArea+11.5*6*math.Pi-18*math.Pi)
		want, err := b.cutMany(stock.Handle, []string{through.Handle, tools[1], tools[2]})
		if err != nil {
			t.Fatal(err)
		}
		gotSTEP, err := b.exportSTEP([]string{opened.Handle})
		if err != nil {
			t.Fatal(err)
		}
		wantSTEP, err := b.exportSTEP([]string{want.Handle})
		if err != nil || gotSTEP.Base64 != wantSTEP.Base64 {
			t.Fatal("through recut differs from fresh STEP", err)
		}
	}
}
