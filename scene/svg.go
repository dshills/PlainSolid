package scene

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"

	"plainsolid/geom"
)

// WriteSVG writes projected visible spans as an SVG line drawing. Width and
// height are CSS pixels. Coordinates are normalized with Y up at input;
// output uses SVG's Y-down viewport. IDs are escaped as data-edge attributes.
func WriteSVG(w io.Writer, spans []VisibleSpan, width, height int) error {
	if w == nil || width < 1 || height < 1 || width > 1000000 || height > 1000000 {
		return fmt.Errorf("scene: invalid SVG dimensions or writer")
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n<g fill=\"none\" stroke=\"currentColor\">\n", width, height, width, height)
	project := func(p geom.Vec2) (float64, float64, error) {
		if !finite(p.X) || !finite(p.Y) || math.Abs(p.X) > 1+1e-12 || math.Abs(p.Y) > 1+1e-12 {
			return 0, 0, fmt.Errorf("scene: SVG point outside normalized viewport")
		}
		x, y := math.Max(-1, math.Min(1, p.X)), math.Max(-1, math.Min(1, p.Y))
		return (x + 1) * float64(width) / 2, (1 - y) * float64(height) / 2, nil
	}
	for _, v := range spans {
		x0, y0, e := project(v.Start)
		if e != nil {
			return e
		}
		x1, y1, e := project(v.End)
		if e != nil {
			return e
		}
		fmt.Fprintf(&b, "  <line x1=\"%.12g\" y1=\"%.12g\" x2=\"%.12g\" y2=\"%.12g\" data-edge=\"", x0, y0, x1, y1)
		if e = xml.EscapeText(&b, []byte(v.ID)); e != nil {
			return e
		}
		b.WriteString("\"/>\n")
	}
	b.WriteString("</g>\n</svg>\n")
	n, e := w.Write(b.Bytes())
	if e == nil && n != b.Len() {
		return io.ErrShortWrite
	}
	return e
}
