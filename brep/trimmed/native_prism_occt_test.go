package trimmed_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type occtPrismInput struct {
	rings     [][]geom.Vec2
	low, high float64
}

type occtPrismCase struct {
	name, op string
	a, b     occtPrismInput
	family   trimmed.NativePrismMaterialFamily
}

func nativePrismOracleCases() []occtPrismCase {
	rect := func(x0, y0, x1, y1 float64) []geom.Vec2 {
		return []geom.Vec2{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
	}
	stock, crossing, inside := rect(0, 0, 4, 3), rect(2, -1, 5, 2), rect(1, 1, 2, 2)
	ridge, side := rect(2, -1, 3, 4), rect(4, 1, 6, 2)
	annulus := occtPrismInput{[][]geom.Vec2{rect(0, 0, 6, 6), rect(2, 2, 4, 4)}, 0, 2}
	restrict := occtPrismInput{[][]geom.Vec2{rect(1, 1, 5, 5), rect(1.5, 1.5, 4.5, 4.5)}, 2, 4}
	post := occtPrismInput{[][]geom.Vec2{rect(2, .5, 5, 2.5), rect(2.5, 1, 3, 2)}, 1, 3}
	span := func(ring []geom.Vec2, low, high float64) occtPrismInput {
		return occtPrismInput{[][]geom.Vec2{ring}, low, high}
	}
	return []occtPrismCase{
		{"matching fuse", "fuse", span(stock, 0, 3), span(crossing, 0, 3), trimmed.NativePrismFamilyMatching},
		{"matching cut", "cut", span(stock, 0, 3), span(crossing, 0, 3), trimmed.NativePrismFamilyMatching},
		{"unequal common", "common", span(stock, 0, 3), span(crossing, 1, 4), trimmed.NativePrismFamilyCommon},
		{"staggered fuse", "fuse", span(stock, 0, 3), span(crossing, 2, 5), trimmed.NativePrismFamilyCrossingFuse},
		{"staggered cut", "cut", span(stock, 0, 3), span(crossing, 2, 5), trimmed.NativePrismFamilyCrossingCut},
		{"nested crossing fuse", "fuse", span(stock, 0, 4), span(crossing, 1, 3), trimmed.NativePrismFamilyNestedFuse},
		{"nested crossing cut with retained post", "cut", span(stock, 0, 4), post, trimmed.NativePrismFamilyNestedCut},
		{"covering cut with disconnected material", "cut", span(stock, 0, 3), span(ridge, -1, 4), trimmed.NativePrismFamilyCoveringCut},
		{"abutting crossing fuse", "fuse", span(stock, 0, 2), span(crossing, 2, 5), trimmed.NativePrismFamilyAbuttingFuse},
		{"abutting identical fuse", "fuse", span(stock, 0, 2), span(stock, 2, 5), trimmed.NativePrismFamilyAbuttingFuse},
		{"abutting nested annuli fuse", "fuse", annulus, restrict, trimmed.NativePrismFamilyAbuttingFuse},
		{"tied cap fuse", "fuse", span(stock, 0, 4), span(crossing, 0, 2), trimmed.NativePrismFamilyTiedFuse},
		{"side abutting fuse", "fuse", span(stock, 0, 3), span(side, 2, 5), trimmed.NativePrismFamilySideAbuttingFuse},
		{"strict cavity", "cut", span(stock, 0, 4), span(inside, 1, 3), trimmed.NativePrismFamilyCavityCut},
		{"blind cut", "cut", span(stock, 0, 4), span(inside, 2, 5), trimmed.NativePrismFamilyBlindCut},
		{"through cut", "cut", span(stock, 0, 4), span(inside, -1, 5), trimmed.NativePrismFamilyThroughCut},
		{"boss fuse", "fuse", span(stock, 0, 4), span(inside, 2, 5), trimmed.NativePrismFamilyBossFuse},
		{"profile-covered two-slab cut", "cut", span(stock, 0, 4), span(rect(-1, -1, 5, 4), 1, 3), trimmed.NativePrismFamilyProfileCoveredCut},
		{"profile-covered bound cut", "cut", span(stock, 0, 4), span(rect(-1, -1, 5, 4), 0, 2), trimmed.NativePrismFamilyProfileCoveredCut},
		{"disjoint fuse", "fuse", span(stock, 0, 2), span(crossing, 3, 4), trimmed.NativePrismFamilyOriginal},
		{"cap-only common", "common", span(stock, 0, 2), span(crossing, 2, 5), trimmed.NativePrismFamilyCommon},
	}
}

func poseNativePrismOracleInput(input occtPrismInput, reflected bool) occtPrismInput {
	out := occtPrismInput{rings: make([][]geom.Vec2, len(input.rings)), low: input.low, high: input.high}
	for i, ring := range input.rings {
		for _, p := range ring {
			x := p.X + p.Y/4
			y := p.Y + x/8
			if reflected {
				x, y = 7-y, -3+x
			}
			out.rings[i] = append(out.rings[i], geom.Vec2{X: x, Y: y})
		}
	}
	if reflected {
		out.low, out.high = 7-input.high, 7-input.low
	}
	return out
}

func (p occtPrismInput) write(w io.Writer) {
	fmt.Fprintf(w, "%g %g %d\n", p.low, p.high, len(p.rings)-1)
	for _, ring := range p.rings {
		fmt.Fprint(w, len(ring))
		for _, v := range ring {
			fmt.Fprintf(w, " %g %g", v.X, v.Y)
		}
		fmt.Fprintln(w)
	}
}

func (p occtPrismInput) native() (brep.Solid, error) {
	var rings [][]geom.Vec3
	for _, ring := range p.rings {
		var points []geom.Vec3
		for _, v := range ring {
			points = append(points, geom.Vec3{X: v.X, Y: v.Y, Z: p.low})
		}
		rings = append(rings, points)
	}
	return brep.PrismWithHoles(rings[0], rings[1:], geom.Vec3{Z: p.high - p.low})
}

func TestNativePrismBooleanAgainstOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_NATIVE_PRISM_BOOLEAN")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_NATIVE_PRISM_BOOLEAN to the independent prism comparator")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var cases []occtPrismCase
	for _, base := range nativePrismOracleCases() {
		for _, reflected := range []bool{false, true} {
			q := base
			if reflected {
				q.name += " reflected Z and rotated section"
			}
			q.a = poseNativePrismOracleInput(q.a, reflected)
			q.b = poseNativePrismOracleInput(q.b, reflected)
			cases = append(cases, q)
		}
	}
	var request, diagnostics bytes.Buffer
	fmt.Fprintln(&request, len(cases))
	for _, c := range cases {
		fmt.Fprintln(&request, c.op)
		c.a.write(&request)
		c.b.write(&request)
	}
	command := exec.CommandContext(ctx, bin)
	command.Stdin, command.Stderr = &request, &diagnostics
	response, err := command.Output()
	if err != nil {
		t.Fatalf("independent OCCT comparator: %v\n%s", err, diagnostics.Bytes())
	}
	answers := json.NewDecoder(bytes.NewReader(response))
	operations := map[string]intersect.AffineRingOperation{
		"common": intersect.AffineRingIntersection, "fuse": intersect.AffineRingUnion, "cut": intersect.AffineRingDifference,
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var expected struct {
				Index, Solids, Shells int
				Volume, Area          float64
				Centroid              [3]float64
				Eulers                []int
			}
			if err := answers.Decode(&expected); err != nil || expected.Index != i {
				t.Fatalf("mismatched OCCT result at %d: %v", i, err)
			}
			a, err := c.a.native()
			if err != nil {
				t.Fatal(err)
			}
			b, err := c.b.native()
			if err != nil {
				t.Fatal(err)
			}
			got, err := trimmed.CombineNativePrisms(ctx, a, b, operations[c.op], trimmed.NativePrismDispatchOptions{})
			if err != nil || got.Family() != c.family || !got.Complete() {
				t.Fatalf("native dispatcher family %v, expected %v, error %v", got.Family(), c.family, err)
			}
			var volume, area float64
			var center geom.Vec3
			var shells int
			var eulers []int
			for _, s := range got.Components() {
				p, err := s.MassProperties()
				if err != nil || p.Volume <= 0 {
					t.Fatalf("invalid native material mass %v, error %v", p, err)
				}
				volume += p.Volume
				area += p.SurfaceArea
				center = center.Add(p.Centroid.Scale(p.Volume))
				top := s.Topology()
				shells += top.Shells
				eulers = append(eulers, top.EulerCharacteristic)
			}
			if volume > 0 {
				center = center.Scale(1 / volume)
			}
			slices.Sort(eulers)
			slices.Sort(expected.Eulers)
			close := func(a, b float64) bool { return math.Abs(a-b) <= 3e-10*max(1, math.Abs(b)) }
			if !close(volume, expected.Volume) || !close(area, expected.Area) || !close(center.X, expected.Centroid[0]) ||
				!close(center.Y, expected.Centroid[1]) || !close(center.Z, expected.Centroid[2]) || len(eulers) != expected.Solids ||
				shells != expected.Shells || !slices.Equal(eulers, expected.Eulers) {
				t.Fatalf("native volume=%g area=%g centroid=%v shells=%d Euler=%v, OCCT=%+v", volume, area, center, shells, eulers, expected)
			}
		})
	}
	if err := answers.Decode(new(any)); err != io.EOF {
		t.Fatalf("unexpected additional OCCT output: %v", err)
	}
}
