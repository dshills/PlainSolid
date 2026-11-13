// Package step implements bounded ISO 10303-21 syntax and a deliberately narrow
// planar B-rep exchange subset. It is not a general AP203/AP214/AP242 translator.
package step

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Limits bounds buffered input, parsed values, nesting and imported topology.
// Zero fields select defaults. MaxDepth may not exceed 256.
type Limits struct {
	MaxBytes                                                                        int64
	MaxEntities, MaxValues, MaxDepth, MaxVertices, MaxFaces, MaxSolids, MaxEdgeUses int
}

func (l Limits) defaults() (Limits, error) {
	if l.MaxBytes == 0 {
		l.MaxBytes = 64 << 20
	}
	if l.MaxEntities == 0 {
		l.MaxEntities = 1_000_000
	}
	if l.MaxValues == 0 {
		l.MaxValues = 4_000_000
	}
	if l.MaxDepth == 0 {
		l.MaxDepth = 64
	}
	if l.MaxVertices == 0 {
		l.MaxVertices = 1_000_000
	}
	if l.MaxFaces == 0 {
		l.MaxFaces = 100_000
	}
	if l.MaxEdgeUses == 0 {
		l.MaxEdgeUses = 1_000_000
	}
	if l.MaxSolids == 0 {
		l.MaxSolids = 1000
	}
	if l.MaxBytes < 1 || l.MaxBytes > 1<<30 || l.MaxEntities < 1 || l.MaxValues < 1 || l.MaxDepth < 1 || l.MaxDepth > 256 || l.MaxVertices < 1 || l.MaxFaces < 1 || l.MaxSolids < 1 || l.MaxEdgeUses < 1 {
		return l, errors.New("step: invalid limits")
	}
	return l, nil
}

// Kind discriminates a Part 21 parameter. Number.Text preserves its lexeme.
type Kind uint8

const (
	Omitted Kind = iota
	Derived
	Number
	String
	Enumeration
	Reference
	Aggregate
	Typed
)

type Value struct {
	Kind  Kind
	Text  string
	Ref   int64
	Items []Value
}
type Record struct {
	Type string
	Args []Value
}

// Entity has one record for a simple instance, multiple for a complex instance.
type Entity struct {
	ID      int64
	Records []Record
}

// File preserves entity IDs and encounter order. All references resolve.
// Parsing checks syntax and resources, not EXPRESS schema constraints.
type File struct {
	Header   []Record
	Entities map[int64]Entity
	Order    []int64
}
type token struct {
	kind byte
	text string
	pos  int
}
type parser struct {
	data   []byte
	pos    int
	peeked *token
	limits Limits
	values int
}

func (p *parser) err(msg string) error { return fmt.Errorf("step: byte %d: %s", p.pos, msg) }
func ident(c byte) bool                { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_' }
func digit(c byte) bool                { return c >= '0' && c <= '9' }
func (p *parser) lex() (token, error) {
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == ' ' || c == '\r' || c == '\n' || c == '\t' {
			p.pos++
			continue
		}
		if c == '/' && p.pos+1 < len(p.data) && p.data[p.pos+1] == '*' {
			end := bytes.Index(p.data[p.pos+2:], []byte("*/"))
			if end < 0 {
				return token{}, p.err("unterminated comment")
			}
			p.pos += end + 4
			continue
		}
		break
	}
	start := p.pos
	if start == len(p.data) {
		return token{kind: 0, pos: start}, nil
	}
	c := p.data[p.pos]
	p.pos++
	switch c {
	case '(', ')', ',', ';', '=', '$', '*':
		return token{kind: c, text: string(c), pos: start}, nil
	case '#':
		for p.pos < len(p.data) && digit(p.data[p.pos]) {
			p.pos++
		}
		if p.pos == start+1 {
			return token{}, p.err("empty reference")
		}
		return token{kind: '#', text: string(p.data[start+1 : p.pos]), pos: start}, nil
	case '\'':
		var b strings.Builder
		for p.pos < len(p.data) {
			c = p.data[p.pos]
			p.pos++
			if c == '\'' {
				if p.pos < len(p.data) && p.data[p.pos] == '\'' {
					p.pos++
					b.WriteByte('\'')
					continue
				}
				return token{kind: 's', text: b.String(), pos: start}, nil
			}
			if c == '\\' {
				if p.pos < len(p.data) && p.data[p.pos] == '\\' {
					p.pos++
					b.WriteByte('\\')
					continue
				}
				if p.pos+2 < len(p.data) && p.data[p.pos] == 'X' && (p.data[p.pos+1] == '2' || p.data[p.pos+1] == '4') && p.data[p.pos+2] == '\\' {
					width := int(p.data[p.pos+1]-'0') * 2
					p.pos += 3
					end := bytes.Index(p.data[p.pos:], []byte("\\X0\\"))
					if end < 0 || end%width != 0 {
						return token{}, p.err("invalid Unicode escape")
					}
					raw := string(p.data[p.pos : p.pos+end])
					for i := 0; i < len(raw); i += width {
						n, e := strconv.ParseUint(raw[i:i+width], 16, 32)
						if e != nil || !utf8.ValidRune(rune(n)) {
							return token{}, p.err("invalid Unicode scalar")
						}
						b.WriteRune(rune(n))
					}
					p.pos += end + 4
					continue
				}
				return token{}, p.err("unsupported STEP string escape")
			}
			if c < 32 || c > 126 {
				return token{}, p.err("non-ASCII string byte; use X2/X4 escapes")
			}
			b.WriteByte(c)
		}
		return token{}, p.err("unterminated string")
	}
	if c == '.' && p.pos < len(p.data) && ident(p.data[p.pos]) {
		for p.pos < len(p.data) && (ident(p.data[p.pos]) || digit(p.data[p.pos])) {
			p.pos++
		}
		if p.pos >= len(p.data) || p.data[p.pos] != '.' {
			return token{}, p.err("unterminated enumeration")
		}
		p.pos++
		return token{kind: 'e', text: strings.ToUpper(string(p.data[start+1 : p.pos-1])), pos: start}, nil
	}
	if ident(c) {
		for p.pos < len(p.data) && (ident(p.data[p.pos]) || digit(p.data[p.pos]) || p.data[p.pos] == '-') {
			p.pos++
		}
		return token{kind: 'i', text: strings.ToUpper(string(p.data[start:p.pos])), pos: start}, nil
	}
	if digit(c) || c == '+' || c == '-' || c == '.' {
		for p.pos < len(p.data) {
			c = p.data[p.pos]
			if digit(c) || c == '.' || c == 'E' || c == 'e' || c == '+' || c == '-' {
				p.pos++
			} else {
				break
			}
		}
		s := string(p.data[start:p.pos])
		v, e := strconv.ParseFloat(s, 64)
		if !numberToken(s) || e != nil || math.IsInf(v, 0) || math.IsNaN(v) {
			return token{}, p.err("invalid finite number")
		}
		return token{kind: 'n', text: s, pos: start}, nil
	}
	return token{}, p.err("unexpected byte")
}

func numberToken(s string) bool {
	i := 0
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		i++
	}
	start := i
	for i < len(s) && digit(s[i]) {
		i++
	}
	if i == start {
		return false
	}
	if i == len(s) {
		return true
	}
	if s[i] != '.' {
		return false
	}
	i++
	for i < len(s) && digit(s[i]) {
		i++
	}
	if i == len(s) {
		return true
	}
	if s[i] != 'E' {
		return false
	}
	i++
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	start = i
	for i < len(s) && digit(s[i]) {
		i++
	}
	return i > start && i == len(s)
}

func (p *parser) peek() (token, error) {
	if p.peeked != nil {
		return *p.peeked, nil
	}
	t, e := p.lex()
	if e == nil {
		p.peeked = &t
	}
	return t, e
}
func (p *parser) take() (token, error) { t, e := p.peek(); p.peeked = nil; return t, e }
func (p *parser) expect(k byte, s string) error {
	t, e := p.take()
	if e != nil {
		return e
	}
	if t.kind != k || (s != "" && t.text != s) {
		return p.err("expected " + s + string(k))
	}
	return nil
}
func (p *parser) list(depth int) ([]Value, error) {
	if depth > p.limits.MaxDepth {
		return nil, p.err("nesting limit exceeded")
	}
	if e := p.expect('(', ""); e != nil {
		return nil, e
	}
	var v []Value
	t, e := p.peek()
	if e != nil {
		return nil, e
	}
	if t.kind == ')' {
		_, e = p.take()
		return v, e
	}
	for {
		a, e := p.value(depth)
		if e != nil {
			return nil, e
		}
		v = append(v, a)
		t, e = p.take()
		if e != nil {
			return nil, e
		}
		if t.kind == ')' {
			return v, nil
		}
		if t.kind != ',' {
			return nil, p.err("expected comma or closing parenthesis")
		}
	}
}
func (p *parser) value(depth int) (Value, error) {
	p.values++
	if p.values > p.limits.MaxValues {
		return Value{}, p.err("value limit exceeded")
	}
	t, e := p.peek()
	if e != nil {
		return Value{}, e
	}
	if t.kind == '(' {
		v, e := p.list(depth + 1)
		return Value{Kind: Aggregate, Items: v}, e
	}
	_, _ = p.take()
	v := Value{Text: t.text}
	switch t.kind {
	case '$':
		v.Kind = Omitted
	case '*':
		v.Kind = Derived
	case 's':
		v.Kind = String
	case 'n':
		v.Kind = Number
	case 'e':
		v.Kind = Enumeration
	case '#':
		v.Kind = Reference
		v.Ref, e = strconv.ParseInt(t.text, 10, 64)
		if e != nil || v.Ref <= 0 {
			return Value{}, p.err("invalid entity reference")
		}
	case 'i':
		v.Kind = Typed
		v.Items, e = p.list(depth + 1)
	default:
		return Value{}, p.err("invalid parameter")
	}
	return v, e
}
func (p *parser) record() (Record, error) {
	t, e := p.take()
	if e != nil {
		return Record{}, e
	}
	if t.kind != 'i' || strings.Contains(t.text, "-") || strings.HasPrefix(t.text, "_") {
		return Record{}, p.err("expected entity name")
	}
	v, e := p.list(1)
	return Record{t.text, v}, e
}

// Parse reads one HEADER and one DATA section (Part 21 editions 1/2 subset).
// Complex instances, typed values, comments, forward references, quoted strings
// and X2/X4 Unicode escapes are supported. ANCHOR/REFERENCE/signature sections,
// binary literals and legacy code-page escapes are explicitly unsupported.
func Parse(r io.Reader, l Limits) (*File, error) {
	l, e := l.defaults()
	if e != nil {
		return nil, e
	}
	if r == nil {
		return nil, errors.New("step: nil reader")
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > l.MaxBytes {
		return nil, errors.New("step: byte limit exceeded")
	}
	p := parser{data: b, limits: l}
	for _, s := range []string{"ISO-10303-21", "HEADER"} {
		if e = p.expect('i', s); e != nil {
			return nil, e
		}
		if e = p.expect(';', ""); e != nil {
			return nil, e
		}
	}
	f := &File{Entities: map[int64]Entity{}}
	for {
		t, e := p.peek()
		if e != nil {
			return nil, e
		}
		if t.kind == 'i' && t.text == "ENDSEC" {
			_, _ = p.take()
			break
		}
		if len(f.Header) >= 64 {
			return nil, p.err("header record limit exceeded")
		}
		rec, e := p.record()
		if e != nil {
			return nil, e
		}
		f.Header = append(f.Header, rec)
		if e = p.expect(';', ""); e != nil {
			return nil, e
		}
	}
	if e = p.expect(';', ""); e != nil {
		return nil, e
	}
	if len(f.Header) < 3 || f.Header[0].Type != "FILE_DESCRIPTION" || f.Header[1].Type != "FILE_NAME" || f.Header[2].Type != "FILE_SCHEMA" {
		return nil, p.err("missing required header records")
	}
	if e = p.expect('i', "DATA"); e != nil {
		return nil, e
	}
	if e = p.expect(';', ""); e != nil {
		return nil, e
	}
	for {
		t, e := p.peek()
		if e != nil {
			return nil, e
		}
		if t.kind == 'i' && t.text == "ENDSEC" {
			_, _ = p.take()
			break
		}
		if len(f.Order) >= l.MaxEntities {
			return nil, p.err("entity limit exceeded")
		}
		t, e = p.take()
		if e != nil {
			return nil, e
		}
		if t.kind != '#' {
			return nil, p.err("expected entity ID")
		}
		id, e := strconv.ParseInt(t.text, 10, 64)
		if e != nil || id <= 0 {
			return nil, p.err("invalid entity ID")
		}
		if _, ok := f.Entities[id]; ok {
			return nil, p.err("duplicate entity ID")
		}
		if e = p.expect('=', ""); e != nil {
			return nil, e
		}
		ent := Entity{ID: id}
		t, e = p.peek()
		if e != nil {
			return nil, e
		}
		if t.kind == '(' {
			_, _ = p.take()
			for {
				t, e = p.peek()
				if e != nil {
					return nil, e
				}
				if t.kind == ')' {
					_, _ = p.take()
					break
				}
				rec, e := p.record()
				if e != nil {
					return nil, e
				}
				for _, prior := range ent.Records {
					if prior.Type == rec.Type {
						return nil, p.err("duplicate complex component")
					}
				}
				ent.Records = append(ent.Records, rec)
				if len(ent.Records) > 256 {
					return nil, p.err("complex component limit exceeded")
				}
			}
			if len(ent.Records) < 2 {
				return nil, p.err("complex instance needs multiple components")
			}
		} else {
			rec, e := p.record()
			if e != nil {
				return nil, e
			}
			ent.Records = []Record{rec}
		}
		if e = p.expect(';', ""); e != nil {
			return nil, e
		}
		f.Entities[id] = ent
		f.Order = append(f.Order, id)
	}
	if e = p.expect(';', ""); e != nil {
		return nil, e
	}
	if e = p.expect('i', "END-ISO-10303-21"); e != nil {
		return nil, e
	}
	if e = p.expect(';', ""); e != nil {
		return nil, e
	}
	if t, e := p.take(); e != nil || t.kind != 0 {
		return nil, p.err("trailing content")
	}
	var refs func([]Value) error
	refs = func(v []Value) error {
		for _, a := range v {
			if a.Kind == Reference {
				if _, ok := f.Entities[a.Ref]; !ok {
					return fmt.Errorf("step: missing reference #%d", a.Ref)
				}
			}
			if e := refs(a.Items); e != nil {
				return e
			}
		}
		return nil
	}
	for _, rec := range f.Header {
		if e = refs(rec.Args); e != nil {
			return nil, e
		}
	}
	for _, ent := range f.Entities {
		for _, r := range ent.Records {
			if e = refs(r.Args); e != nil {
				return nil, e
			}
		}
	}
	return f, nil
}
