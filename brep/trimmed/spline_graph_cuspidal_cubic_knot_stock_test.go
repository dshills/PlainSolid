package trimmed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func cuspidalCubicKnotStock(t *testing.T) [3]*geom.BSplineSurface {
	t.Helper()
	src := cuspidalCubicCommonFixture(t)
	for i := 1; i < len(src); i++ {
		p, err := geom.NewSurfacePatch(src[i], geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			p, err = p.InsertUKnot(.4375, 3, geom.SurfacePatchOptions{})
		} else {
			p, err = p.InsertVKnot(-.75, 1, geom.SurfacePatchOptions{})
		}
		if err != nil {
			t.Fatal(err)
		}
		refined, err := p.ToBSplineSurface()
		if err != nil {
			t.Fatal(err)
		}
		d := refined.Data()
		// Add the true crease (3/16)*max(0,x/3-7/16) to both roofs.
		// A uses u and B uses (1-v)/4. Greville coordinates give the
		// exact coefficients of this hinge in the refined supports.
		for u := range d.CountU {
			for v := range d.CountV {
				if i == 1 {
					x3 := d.KnotsU[u+1] + d.KnotsU[u+2] + d.KnotsU[u+3]
					d.Poles[u*d.CountV+v].Z += max(x3-3*.4375, 0) / 16
				} else {
					x := (1 - d.KnotsV[v+1]) / 4
					d.Poles[u*d.CountV+v].Z += max(x-.4375, 0) * 3 / 16
				}
			}
		}
		src[i], err = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if err != nil {
			t.Fatal(err)
		}
	}
	return src
}

func TestNativeCuspidalCubicCreasedStockMassAndReplay(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicKnotStock(t)
	// Integrate the added hinge over the full 3 by 2 footprint:
	// 6*(3/16)*(1-7/16)^2/2. As both carriers change equally,
	// neither material difference has a volume correction.
	const volumeAdded = 729.0 / 4096
	type result interface {
		Boundary() Solid
		WriteJSON(context.Context, io.Writer) error
	}
	for order := range 2 {
		a, b := src[1+order], src[2-order]
		o := CuspidalCubicCommonOptions{}
		cutVolume := [2]float64{1695.0 / 22528, 243.0 / 22528}[order]
		for op, volume := range []float64{cuspidalCommonVolume + volumeAdded, cuspidalFuseVolume + volumeAdded, cutVolume} {
			t.Run(fmt.Sprintf("%s/%d", []string{"common", "fuse", "cut"}[op], order), func(t *testing.T) {
				var (
					out   result
					read  func(io.Reader) (result, error)
					faces [2]int
					owner [2]int
				)
				switch op {
				case 0:
					x, err := CommonCuspidalCubicSplineGraphs(ctx, src[0], a, b, order, 1e-9, o)
					if err != nil {
						t.Fatal(err)
					}
					out, faces, owner = x, x.History().RoofFaces, x.History().RoofOwners
					read = func(r io.Reader) (result, error) {
						return ReadCuspidalCubicCommonJSON(ctx, r, JSONLimits{}, o)
					}
				case 1:
					x, err := FuseCuspidalCubicSplineGraphs(ctx, src[0], a, b, order, 1e-9, o)
					if err != nil {
						t.Fatal(err)
					}
					out, faces, owner = x, x.History().RoofFaces, x.History().RoofOwners
					read = func(r io.Reader) (result, error) {
						return ReadCuspidalCubicFuseJSON(ctx, r, JSONLimits{}, o)
					}
				case 2:
					x, err := CutCuspidalCubicSplineGraphs(ctx, src[0], a, b, order, 1e-9, o)
					if err != nil {
						t.Fatal(err)
					}
					out, faces, owner = x, x.History().RoofFaces, [2]int{0, 1}
					read = func(r io.Reader) (result, error) {
						return ReadCuspidalCubicCutJSON(ctx, r, JSONLimits{}, o)
					}
				}
				s := out.Boundary()
				topo := s.Topology()
				if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 {
					t.Fatal("lost closed original topology", topo)
				}
				for i, id := range faces {
					if s.faces[id].Surface != ([2]*geom.BSplineSurface{a, b})[owner[i]] {
						t.Fatal("assembly substituted original knotted support", i)
					}
				}
				p, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if err != nil || math.Abs(p.Volume-volume) > 1e-9 || !finite(p.Area) || p.Area <= 0 {
					t.Fatal("creased solid differs from independent material integral", p, volume, err)
				}
				t.Logf("volume %.16g area %.16g", p.Volume, p.Area)
				var native, replay bytes.Buffer
				if err := out.WriteJSON(ctx, &native); err != nil {
					t.Fatal(err)
				}
				again, err := read(bytes.NewReader(native.Bytes()))
				if err != nil {
					t.Fatal(err)
				}
				if err := again.WriteJSON(ctx, &replay); err != nil || !bytes.Equal(native.Bytes(), replay.Bytes()) {
					t.Fatal("native replay changed original knot, trim or cusp proof", err)
				}
			})
		}
	}
	d := src[2].Data()
	d.Poles[0].Z += 1. / 1024
	changed, err := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := CommonCuspidalCubicSplineGraphs(ctx, src[0], src[1], changed, 0, 1e-9, CuspidalCubicCommonOptions{}); !errors.Is(err, geom.ErrConvergence) || out != nil {
		t.Fatal("accepted a changed later carrier cell", err)
	}
}
