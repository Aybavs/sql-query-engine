package lexer

import "testing"

func kinds(toks []Token) []Kind {
	ks := make([]Kind, len(toks))
	for i, t := range toks {
		ks[i] = t.Kind
	}
	return ks
}

func TestLexSelect(t *testing.T) {
	toks, err := Lex("SELECT id, name FROM users WHERE age >= 18")
	if err != nil {
		t.Fatalf("Lex: %v", err)
	}
	wantKinds := []Kind{Keyword, Ident, Comma, Ident, Keyword, Ident, Keyword, Ident, Op, Int, EOF}
	got := kinds(toks)
	if len(got) != len(wantKinds) {
		t.Fatalf("token count = %d, want %d (%v)", len(got), len(wantKinds), toks)
	}
	for i := range wantKinds {
		if got[i] != wantKinds[i] {
			t.Fatalf("token %d kind = %v, want %v", i, got[i], wantKinds[i])
		}
	}
	if toks[0].Text != "SELECT" {
		t.Fatalf("keyword text = %q, want SELECT", toks[0].Text)
	}
}

func TestLexLiterals(t *testing.T) {
	toks, _ := Lex("1 1.5 'hi' <>")
	if toks[0].Kind != Int || toks[1].Kind != Float || toks[2].Kind != String || toks[2].Text != "hi" {
		t.Fatalf("literal kinds wrong: %v", toks)
	}
	if toks[3].Kind != Op || toks[3].Text != "<>" {
		t.Fatalf("operator wrong: %v", toks[3])
	}
}

func TestLexUnterminatedString(t *testing.T) {
	if _, err := Lex("'oops"); err == nil {
		t.Fatal("expected error for unterminated string")
	}
}
