package triplestore

import (
	"reflect"
	"testing"
)

func TestParser(t *testing.T) {
	tcases := []struct {
		input    string
		expected []Triple
	}{
		{
			input: "<sub> <pred> <lol> .\n<sub2> <pred2> \"lol2\" .",
			expected: []Triple{
				SubjPred("sub", "pred").Resource("lol"),
				mustTriple("sub2", "pred2", "lol2"),
			},
		},
		{
			input: "<sub> <pred> \"2\"^^<myinteger> .\n<sub2> <pred2> <lol2> .",
			expected: []Triple{
				SubjPred("sub", "pred").Object(object{isLit: true, lit: literal{typ: "myinteger", val: "2"}}),
				SubjPred("sub2", "pred2").Resource("lol2"),
			},
		},
	}

	for j, tcase := range tcases {
		p := newNTParser(tcase.input)
		tris := p.parse()
		if got, want := len(tris), len(tcase.expected); got != want {
			t.Fatalf("triples size (case %d): got %d, want %d", j+1, got, want)
		}
		for i, tri := range tris {
			if got, want := tri, tcase.expected[i]; !got.Equal(want) {
				t.Fatalf("triple (%d)\ngot %v\n\nwant %v", i+1, got, want)
			}
		}
	}
}
func TestLexer(t *testing.T) {
	tcases := []struct {
		input    string
		expected []ntToken
	}{
		// single
		{"<node>", []ntToken{iriTok("node")}},
		{"# comment", []ntToken{commentTok(" comment")}},
		{"\"lit\"", []ntToken{litTok("lit")}},
		{"^^<xsd:float>", []ntToken{datatypeTok("xsd:float")}},
		{" ", []ntToken{wspaceTok}},
		{".", []ntToken{fullstopTok}},

		// escaped
		{`<no>de>`, []ntToken{iriTok("no>de")}},
		{`<no\>de>`, []ntToken{iriTok("no\\>de")}},
		{`<node\\>`, []ntToken{iriTok("node\\\\")}},
		{`"\\"`, []ntToken{litTok(`\\`)}},
		{`"quot"ed"`, []ntToken{litTok(`quot"ed`)}},
		{`"quot\"ed"`, []ntToken{litTok("quot\\\"ed")}},

		// triple
		{"<sub> <pred> \"3\"^^<xsd:integer> .", []ntToken{
			iriTok("sub"), wspaceTok, iriTok("pred"), wspaceTok, litTok("3"),
			datatypeTok("xsd:integer"), wspaceTok, fullstopTok,
		}},
		{"<sub><pred>\"3\"^^<xsd:integer>.", []ntToken{
			iriTok("sub"), iriTok("pred"), litTok("3"), datatypeTok("xsd:integer"), fullstopTok,
		}},
		{"<sub> <pred> \"lit\" . # commenting", []ntToken{
			iriTok("sub"), wspaceTok, iriTok("pred"), wspaceTok, litTok("lit"),
			wspaceTok, fullstopTok, wspaceTok, commentTok(" commenting"),
		}},
		{"<sub><pred>\"lit\".#commenting", []ntToken{
			iriTok("sub"), iriTok("pred"), litTok("lit"), fullstopTok, commentTok("commenting"),
		}},
	}

	for i, tcase := range tcases {
		l := newLexer(tcase.input)
		var toks []ntToken
		for tok := l.nextToken(); tok.kind != EOF_TOK; tok = l.nextToken() {
			toks = append(toks, tok)
		}
		if got, want := toks, tcase.expected; !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d: \ngot %#v\n\nwant %#v", i+1, got, want)
		}
	}
}

func TestLexerReadIRI(t *testing.T) {
	tcases := []struct {
		input string
		node  string
	}{
		{"<", ""},
		{">", ""},
		{"", ""},
		{"z", ""},
		{"subject>", "subject"},
		{"s  ubject>", "s  ubject"},
		{"subject>   <", "subject"},
		{"    subject>   <", "    subject"},
		{"subject><", "subject"},
		{"subje   ct><", "subje   ct"},
		{"sub>ject>", "sub>ject"},
		{"sub > ject>", "sub > ject"},
		{"sub>ject>      ", "sub>ject"},
		{"subject", ""},

		{"pred>   \"", "pred"},
		{"pred>\"", "pred"},

		{"resource>.", "resource"},
		{"resource> .", "resource"},
		{"resource>> .", "resource>"},
		{"resource>  .   ", "resource"},
	}

	for i, tcase := range tcases {
		l := newLexer(tcase.input)
		if got, want := l.readIRI(), tcase.node; got != want {
			t.Fatalf("case %d: got '%s', want '%s'", i+1, got, want)
		}
	}

}

func TestLexerReadStringLiteral(t *testing.T) {
	tcases := []struct {
		input string
		node  string
	}{
		{"", ""},
		{`"`, ""},
		{"z", ""},
		{`lit"`, "lit"},
		{`l it"`, "l it"},
		{"li\"t\"", "li\"t"},
		{"li \"t\"", "li \"t"},
		{"li\"t\" .", "li\"t"},
		{"li\"t\".", "li\"t"},
		{"li\"t\" .", "li\"t"},
		{"li\"t\"  .  ", "li\"t"},
		{"li\"t\"^", "li\"t"},
		{"li\"t\"^^", "li\"t"},
		{"li\"t\" ^", "li\"t"},
		{"li\"t\" ^^", "li\"t"},
		{"li\"t\"   ^", "li\"t"},
		{"li\"t\"     ^^", "li\"t"},
	}

	for i, tcase := range tcases {
		s := newLexer(tcase.input).readStringLiteral()
		if got, want := s, tcase.node; got != want {
			t.Fatalf("case %d: got '%s', want '%s'", i+1, got, want)
		}
	}

}

func mustTriple(s, p string, i interface{}) Triple {
	t, err := SubjPredLit(s, p, i)
	if err != nil {
		panic(err)
	}
	return t
}
