package signal

import "testing"

func TestEnvelopeRoundTrip(t *testing.T) {
	raw, err := Encode(TypePing, "1", Ping{ClientTs: 9, Seq: 3})
	if err != nil {
		t.Fatal(err)
	}
	e, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if e.Type != TypePing {
		t.Fatalf("type=%s", e.Type)
	}
	if _, err := Decode([]byte(`{}`)); err == nil {
		t.Fatal("missing type should fail")
	}
	if _, err := Decode([]byte(`not-json`)); err == nil {
		t.Fatal("expected json error")
	}
}
