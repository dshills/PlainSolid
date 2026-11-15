package parametric

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestExactTensorSurfaceNativeCodec(t *testing.T) {
	ctx := context.Background()
	r := geom.ExactTensorSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 1, CountU: 3, CountV: 2,
		KnotsU: []float64{2, 2, 3.5, 5, 5}, KnotsV: []float64{-1, -1, 2, 2},
		Controls: [][4]string{{"0", "0", "1/3", "1"}, {"0", "1", "1/3", "1"}, {"1", "0", "1/2", "1"}, {"1", "1", "1/2", "1"}, {"2", "0", "4/3", "1"}, {"2", "1", "4/3", "1"}}}
	p, e := geom.ImportExactTensorSurfaceRecipe(ctx, r, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := p.ToBSplineSurface(); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("fixture should require exact homogeneous controls", e)
	}
	s, e := geom.NewExactTensorSurface(ctx, p, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pose := geom.Translation(geom.Vec3{X: 2, Y: -3, Z: 4})
	raw, e := EncodeSurface(MapSurface(s, pose))
	if e != nil {
		t.Fatal(e)
	}
	restored, e := DecodeSurface(raw, len(r.Controls))
	if e != nil {
		t.Fatal(e)
	}
	base, m := SurfaceMapping(restored)
	back, ok := base.(*geom.ExactTensorSurface)
	if !ok || m != pose {
		t.Fatalf("original tensor kind or occurrence lost: %T %v", base, m)
	}
	returned, e := back.ExactPatch().ExportExactTensorRecipe()
	if e != nil || !reflect.DeepEqual(r, returned) {
		t.Fatal("exact original knot vector and homogeneous data", returned, e)
	}
	for _, u := range []*big.Rat{big.NewRat(8, 3), big.NewRat(7, 2), big.NewRat(13, 3)} {
		target, e := p.EvaluateRational(ctx, u, big.NewRat(7, 9), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		got, e := back.ExactPatch().EvaluateRational(ctx, u, big.NewRat(7, 9), geom.SurfacePatchEvaluationOptions{})
		if e != nil || !reflect.DeepEqual(target, got) {
			t.Fatal("original exact jets changed", u, target, got, e)
		}
	}
	box, e := SurfaceBounds(restored, Interval{2, 5}, Interval{-1, 2})
	if e != nil || box.Min.X != 2 || box.Max.X != 4 || box.Min.Y != -3 || box.Max.Y != -2 || box.Min.Z > 13.0/3 || box.Max.Z < 16.0/3 {
		t.Fatal("outward bounds of exactly transformed multi-span support", box, e)
	}
	if _, e := DecodeSurface(raw, len(r.Controls)-1); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("native control budget", e)
	}
	raw, e = EncodeSurface(s)
	if e != nil {
		t.Fatal(e)
	}
	for _, item := range []struct{ field, value string }{
		{"KnotsU", `[2,2,3.5,3.5,5,5]`}, {"KnotsV", `[-1,null,2,2]`}, {"CountU", `1000000`},
		{"Controls", `[["0","0","1/3","-1"]]`}, {"Version", `2`}, {"ClaimedBound", `true`},
	} {
		var outer map[string]json.RawMessage
		var exact map[string]json.RawMessage
		if e := json.Unmarshal(raw, &outer); e != nil {
			t.Fatal(e)
		}
		if e := json.Unmarshal(outer["ExactTensor"], &exact); e != nil {
			t.Fatal(e)
		}
		exact[item.field] = json.RawMessage(item.value)
		outer["ExactTensor"], e = json.Marshal(exact)
		if e != nil {
			t.Fatal(e)
		}
		bad, e := json.Marshal(outer)
		if e != nil {
			t.Fatal(e)
		}
		if _, e := DecodeSurface(bad, len(r.Controls)); e == nil {
			t.Fatal("accepted invalid exact tensor data", item.field)
		}
	}
}
