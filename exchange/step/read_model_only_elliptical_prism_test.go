package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func modelOnlyPrismFixture(t testing.TB) []byte {
	t.Helper()
	var b bytes.Buffer
	if e := WriteEllipticalPrismMaterialSTEP(context.Background(), &b, nativeEllipsePrism(t), EllipticalPrismSTEPOptions{}); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestReadModelOnlyEllipticalPrism(t *testing.T) {
	t.Parallel()
	original := nativeEllipsePrism(t)
	for _, delta := range []geom.Vec3{{}, {X: 8, Y: -2, Z: 4}} {
		var b bytes.Buffer
		if e := WriteEllipticalPrismMaterialSTEP(context.Background(), &b, original, EllipticalPrismSTEPOptions{Name: "Exact direct model", Translation: delta}); e != nil {
			t.Fatal(e)
		}
		m, e := ReadModelOnlyEllipticalPrism(context.Background(), bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		if m.Name != "Exact direct model" || m.LengthUnit != .001 || m.SourceFaceIDs != ([3]int64{17, 108, 112}) || m.SourceEdgeIDs != ([3]int64{21, 54, 82}) || len(m.Solid.Faces()) != 6 {
			t.Fatal(m)
		}
		proof, e := m.Solid.ValidateEdgeAgreement(context.Background(), edgecheck.Options{})
		if e != nil || !proof.Complete {
			t.Fatal(e, proof)
		}
		for _, p := range proof.Uses {
			if p.Report.SquaredUpperBound == nil || p.Report.SquaredUpperBound.Sign() != 0 {
				t.Fatal(p)
			}
		}
		var native bytes.Buffer
		if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		props, e := loaded.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(props.Volume-24*math.Pi) > 1e-7 || props.Centroid.Distance((geom.Vec3{X: 1, Y: 3.5, Z: 3}).Add(delta)) > 1e-7 {
			t.Fatal(props, e)
		}
		if delta == (geom.Vec3{}) {
			a, _ := prismNativeBytes(original.Solid)
			c, _ := prismNativeBytes(m.Solid)
			if !bytes.Equal(a, c) {
				t.Fatal("different exact native quarter basis")
			}
		}
		var recipe bytes.Buffer
		if e = WriteModelOnlyEllipticalPrismJSON(context.Background(), &recipe, m); e != nil {
			t.Fatal(e)
		}
		saved := append([]byte(nil), recipe.Bytes()...)
		reloaded, e := ReadModelOnlyEllipticalPrismJSON(context.Background(), &recipe, Limits{})
		if e != nil || reloaded.NativeRimEdges != m.NativeRimEdges {
			t.Fatal(e)
		}
		var again bytes.Buffer
		if e = WriteModelOnlyEllipticalPrismJSON(context.Background(), &again, reloaded); e != nil || !bytes.Equal(saved, again.Bytes()) {
			t.Fatal("native byte stability", e)
		}
		var out bytes.Buffer
		if e = WriteModelOnlyEllipticalPrismSource(context.Background(), &out, reloaded.Source, Limits{}, 1e-9); e != nil {
			t.Fatal(e)
		}
		if _, e = ReadModelOnlyEllipticalPrism(context.Background(), &out, Limits{}, 1e-9); e != nil {
			t.Fatal(e)
		}
		if _, e = ReadEllipticalPrismMaterialJSON(context.Background(), bytes.NewReader(saved), Limits{}); e == nil {
			t.Fatal("repair format admitted exact dialect")
		}
		for _, key := range []string{"Native", "FaceIDs", "RimEdges", "NativeTolerance", "version"} {
			var q map[string]any
			_ = json.Unmarshal(saved, &q)
			q[key] = 0
			bad, _ := json.Marshal(q)
			v, e := ReadModelOnlyEllipticalPrismJSON(context.Background(), bytes.NewReader(bad), Limits{})
			if e == nil || v.Source != nil {
				t.Fatal("forged recipe", key)
			}
		}
	}
}
func TestModelOnlyEllipticalPrismGraphMutations(t *testing.T) {
	t.Parallel()
	raw := string(modelOnlyPrismFixture(t))
	for _, x := range [][2]string{{"#21=EDGE_CURVE('',#22,#24,#27,.T.);", "#21=EDGE_CURVE('',#22,#24,#27,.F.);"}, {"#80=ORIENTED_EDGE('',*,*,#21,.F.);", "#80=ORIENTED_EDGE('',*,*,#21,.T.);"}, {"#111=ORIENTED_EDGE('',*,*,#82,.T.);", "#111=ORIENTED_EDGE('',*,*,#54,.T.);"}, {"#56=ELLIPSE('',#57,4.,2.);", "#56=ELLIPSE('',#57,5.,2.);"}, {"#71=CARTESIAN_POINT('',(1,5,3));", "#71=CARTESIAN_POINT('',(1,4,3));"}, {"#39=DIRECTION('',(-0.,-1.,-0.));", "#39=DIRECTION('',(0.,1.,0.));"}, {"#108=ADVANCED_FACE('',(#109),#97,.F.);", "#108=ADVANCED_FACE('',(#109),#97,.T.);"}, {"(#17,#108,#112)", "(#17,#108,#108)"}, {".MILLI.,.METRE.", ".CENTI.,.METRE."}} {
		bad := strings.Replace(raw, x[0], x[1], 1)
		if bad == raw {
			t.Fatal("stale mutation", x[0])
		}
		m, e := ReadModelOnlyEllipticalPrism(context.Background(), strings.NewReader(bad), Limits{}, 1e-9)
		if e == nil || m.Source != nil {
			t.Fatal("accepted mutation", x)
		}
	}
	if _, e := ReadModelOnlyEllipticalPrism(context.Background(), bytes.NewReader(ellipticalPrismFixture(t)), Limits{}, 1e-9); e == nil {
		t.Fatal("rounded source admitted")
	}
	for _, l := range []Limits{{MaxBytes: 128}, {MaxEntities: 75}, {MaxValues: 64}, {MaxDepth: 1}} {
		if _, e := ReadModelOnlyEllipticalPrism(context.Background(), strings.NewReader(raw), l, 1e-9); e == nil {
			t.Fatal("budget")
		}
	}
	re := regexp.MustCompile(`#([0-9]+)`)
	changed := re.ReplaceAllStringFunc(raw, func(s string) string { n, _ := strconv.Atoi(s[1:]); return fmt.Sprintf("#%d", n+2000) })
	f, e := Parse(strings.NewReader(changed), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	slices.Reverse(f.Order)
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadModelOnlyEllipticalPrism(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	if e != nil || m.SourceFaceIDs != ([3]int64{2017, 2108, 2112}) {
		t.Fatal(e, m.SourceFaceIDs)
	}
}
func FuzzReadModelOnlyEllipticalPrism(f *testing.F) {
	f.Add(modelOnlyPrismFixture(f))
	f.Add(ellipticalPrismFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		_, _ = ReadModelOnlyEllipticalPrism(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 76, MaxValues: 4096, MaxDepth: 16}, 1e-9)
	})
}
