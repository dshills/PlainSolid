package step

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

func graphValue(v Value, offset int64) string {
	switch v.Kind {
	case Omitted:
		return "$"
	case Derived:
		return "*"
	case String:
		return quote(v.Text)
	case Number:
		return v.Text
	case Enumeration:
		return "." + v.Text + "."
	case Reference:
		return fmt.Sprintf("#%d", v.Ref+offset)
	case Aggregate:
		parts := make([]string, len(v.Items))
		for i, a := range v.Items {
			parts[i] = graphValue(a, offset)
		}
		return "(" + strings.Join(parts, ",") + ")"
	case Typed:
		parts := make([]string, len(v.Items))
		for i, a := range v.Items {
			parts[i] = graphValue(a, offset)
		}
		return v.Text + "(" + strings.Join(parts, ",") + ")"
	}
	return "?"
}
func graphRecord(r Record, offset int64) string {
	parts := make([]string, len(r.Args))
	for i, v := range r.Args {
		parts[i] = graphValue(v, offset)
	}
	return r.Type + "(" + strings.Join(parts, ",") + ")"
}
func graphEntity(e Entity, offset int64) string {
	parts := make([]string, len(e.Records))
	for i, r := range e.Records {
		parts[i] = graphRecord(r, offset)
	}
	value := strings.Join(parts, " ")
	if len(parts) > 1 {
		value = "(" + value + ")"
	}
	return fmt.Sprintf("#%d=%s;\n", e.ID+offset, value)
}

const graphHeader = "ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('PlainSolid exact independent bodies'),'2;1');\nFILE_NAME('','',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));\nENDSEC;\nDATA;\n"

// JoinIndependentModels renumbers complete standalone graphs without merging
// geometric entities, representations or coordinate contexts.
func JoinIndependentModels(w io.Writer, sources [][]byte, l Limits) error {
	if len(sources) < 1 || len(sources) > 32 {
		return fmt.Errorf("step: independent model count limit")
	}
	var out bytes.Buffer
	out.WriteString(graphHeader)
	offset := int64(0)
	for _, source := range sources {
		f, e := Parse(bytes.NewReader(source), l)
		if e != nil {
			return e
		}
		maxID := int64(0)
		for _, id := range f.Order {
			if id > maxID {
				maxID = id
			}
			out.WriteString(graphEntity(f.Entities[id], offset))
		}
		if maxID > 1<<52 || offset > 1<<52-maxID {
			return fmt.Errorf("step: entity identifier limit")
		}
		offset += maxID + 1
	}
	out.WriteString("ENDSEC;\nEND-ISO-10303-21;\n")
	limits, e := l.defaults()
	if e != nil {
		return e
	}
	if int64(out.Len()) > limits.MaxBytes {
		return fmt.Errorf("step: output byte limit")
	}
	if _, e = Parse(bytes.NewReader(out.Bytes()), l); e != nil {
		return e
	}
	_, e = w.Write(out.Bytes())
	return e
}

// SplitIndependentModels validates that the file contains independent shape
// representations and returns their complete geometric reference closures,
// retaining each complete registered AP203 product envelope when present.
// Metadata-less inputs remain metadata-less. Only declarative unit/context
// nodes may be shared across models; topology and products are singly owned.
// Assembly mappings and detached geometry are rejected, never silently dropped.
func SplitIndependentModels(r io.Reader, l Limits) ([][]byte, error) {
	f, e := Parse(r, l)
	if e != nil {
		return nil, e
	}
	roots := []int64{}
	for _, id := range f.Order {
		en := f.Entities[id]
		for _, rec := range en.Records {
			if strings.HasSuffix(rec.Type, "SHAPE_REPRESENTATION") {
				if len(en.Records) != 1 || (rec.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" && rec.Type != "FACETED_BREP_SHAPE_REPRESENTATION") {
					return nil, fmt.Errorf("step: unsupported representation")
				}
				roots = append(roots, id)
			}
		}
	}
	if len(roots) == 0 || len(roots) > 32 {
		return nil, fmt.Errorf("step: independent model count limit")
	}
	all := map[int64]bool{}
	out := make([][]byte, 0, len(roots))
	for _, root := range roots {
		productRoots, err := independentProductRoots(f, root)
		if err != nil {
			return nil, err
		}
		used := map[int64]bool{}
		pending := append([]int64{root}, productRoots...)
		for len(pending) > 0 {
			id := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if used[id] {
				continue
			}
			used[id] = true
			en, ok := f.Entities[id]
			if !ok {
				return nil, fmt.Errorf("step: missing independent entity")
			}
			if all[id] && !independentShareable(en) {
				return nil, fmt.Errorf("step: cross-model ownership of #%d", id)
			}
			var values []Value
			for _, rec := range en.Records {
				values = append(values, rec.Args...)
			}
			for len(values) > 0 {
				v := values[len(values)-1]
				values = values[:len(values)-1]
				if v.Kind == Reference {
					pending = append(pending, v.Ref)
				}
				values = append(values, v.Items...)
			}
		}
		for id := range used {
			all[id] = true
		}
		var b bytes.Buffer
		b.WriteString(graphHeader)
		for _, id := range f.Order {
			if used[id] {
				b.WriteString(graphEntity(f.Entities[id], 0))
			}
		}
		b.WriteString("ENDSEC;\nEND-ISO-10303-21;\n")
		out = append(out, b.Bytes())
	}
	for id, en := range f.Entities {
		if !all[id] {
			return nil, fmt.Errorf("step: detached or unsupported entity #%d (%s)", id, en.Records[0].Type)
		}
	}
	return out, nil
}
