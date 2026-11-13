package plaincadbridge

import "testing"

func TestRevolveJoinCaptureContainedPlanarVolume(t *testing.T) {
	b := New()
	stock := box(t, b, vector{10, 0, 0}, vector{20, 10, 10})
	tool := circle(t, b, vector{}, vector{0, 1, 0}, 25, 10)
	count := len(b.shapes)
	for _, pair := range [][2]string{{stock.Handle, tool.Handle}, {tool.Handle, stock.Handle}} {
		result, err := b.overlapVolume(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
		near(t, result.(map[string]float64)["volume"], 1000)
	}
	if len(b.shapes) != count {
		t.Fatal("query published geometry")
	}
	far := box(t, b, vector{100, 0, 0}, vector{110, 10, 10})
	result, err := b.overlapVolume(far.Handle, tool.Handle)
	if err != nil {
		t.Fatal(err)
	}
	near(t, result.(map[string]float64)["volume"], 0)
}
