package step

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestOriginalFinitePlanarPCurveCoverage(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/occt-planar-annular-contact.step")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	m, p, err := ReadFaceContactPair(ctx, bytes.NewReader(raw), FaceContactSTEPOptions{})
	if err != nil || len(m.Bodies) != 2 || p.SignedVolume.RatString() != "24" || len(p.WireOffsets) != 3 {
		t.Fatalf("original finite pose was not authenticated: %+v %v", p, err)
	}
	cases := []struct{ name, original, changed, reason string }{
		{"same-line-shortened", "#278 = CARTESIAN_POINT('',(3.,1.));", "#278 = CARTESIAN_POINT('',(2.999999999999999,1.));", "endpoint or model station changed"},
		{"changed-station", "(-2.5,-0.5),.PIECEWISE_BEZIER_KNOTS.", "(-2.499999999999999,-0.5),.PIECEWISE_BEZIER_KNOTS.", "endpoint or model station changed"},
		{"reversed-owner", "(#278,#279),.UNSPECIFIED.", "(#279,#278),.UNSPECIFIED.", "endpoint or model station changed"},
		{"unclamped", "(2,2),(-2.5,-0.5)", "(1,2),(-2.5,-0.5)", "not clamped"},
		{"different-model-speed", "#273 = VECTOR('',#274,1.);", "#273 = VECTOR('',#274,2.);", "endpoint or model station changed"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if !bytes.Contains(raw, []byte(test.original)) {
				t.Fatal("fixture pattern not present", test.original)
			}
			bad := bytes.Replace(raw, []byte(test.original), []byte(test.changed), 1)
			m, p, err := ReadFaceContactPair(ctx, bytes.NewReader(bad), FaceContactSTEPOptions{})
			if err == nil || !strings.Contains(err.Error(), test.reason) || m.Bodies != nil || p.SignedVolume != nil {
				t.Fatalf("corrupt finite source published a proof: %+v %v", p, err)
			}
		})
	}
}
