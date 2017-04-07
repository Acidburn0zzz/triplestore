package triplestore

import (
	"testing"
	"time"
)

type TestStruct struct {
	Name     string    `predicate:"name"`
	Age      int       `predicate:"age"`
	Size     int64     `predicate:"size"`
	Male     bool      `predicate:"male"`
	Birth    time.Time `predicate:"birth"`
	Surnames []string  `predicate:"surnames"`
	Counts   []int     `predicate:"counts"`

	// special cases that should be ignored
	NoTag       string
	Unsupported complex64   `predicate:"complex"`
	Pointer     *string     `predicate:"ptr"`
	Slice       []complex64 `predicate:"complexes"`
	PtrSlice    []*string   `predicate:"strptr"`
	unexported  string
}

type MainStruct struct {
	Name string   `predicate:"name"`
	Age  int      `predicate:"age"`
	E    Embedded `subject:"rand"`
}

type Embedded struct {
	Size int64 `predicate:"size"`
	Male bool  `predicate:"male"`
}

func TestEmbeddedStructToTriple(t *testing.T) {
	e := Embedded{Size: 186, Male: true}
	s := MainStruct{Name: "donald", Age: 32, E: e}

	tris := TriplesFromStruct("me", s)
	src := NewSource()
	src.Add(tris...)
	snap := src.Snapshot()

	if got, want := snap.Count(), 4; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	all := snap.WithPredObj("size", IntegerLiteral(186))
	if got, want := len(all), 1; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
	all = snap.WithPredObj("male", BooleanLiteral(true))
	if got, want := len(all), 1; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}

func TestSimpleStructToTriple(t *testing.T) {
	now := time.Now()
	s := TestStruct{
		Name: "donald", Age: 32, Size: 186,
		Male: true, Birth: now,
		Surnames: []string{"one", "two", "three"},
		Counts:   []int{1, 2, 3},
	}

	exp := []Triple{
		SubjPred("me", "name").StringLiteral("donald"),
		SubjPred("me", "age").IntegerLiteral(32),
		SubjPred("me", "size").IntegerLiteral(186),
		SubjPred("me", "male").BooleanLiteral(true),
		SubjPred("me", "birth").DateTimeLiteral(now),
		SubjPred("me", "surnames").StringLiteral("one"),
		SubjPred("me", "surnames").StringLiteral("two"),
		SubjPred("me", "surnames").StringLiteral("three"),
		SubjPred("me", "counts").IntegerLiteral(1),
		SubjPred("me", "counts").IntegerLiteral(2),
		SubjPred("me", "counts").IntegerLiteral(3),
	}

	tris := TriplesFromStruct("me", s)
	if got, want := Triples(tris), Triples(exp); !got.Equal(want) {
		t.Fatalf("got %s\n\n want %s", got, want)
	}

	tris = TriplesFromStruct("me", &s)
	if got, want := Triples(tris), Triples(exp); !got.Equal(want) {
		t.Fatalf("got %s\n\n want %s", got, want)
	}
}

func TestReturnEmptyTriplesOnNonStructElem(t *testing.T) {
	var ptr *string
	tcases := []struct {
		val interface{}
	}{
		{true}, {"any"}, {ptr},
	}

	for i, tc := range tcases {
		tris := TriplesFromStruct("", tc.val)
		if len(tris) != 0 {
			t.Fatalf("case %d: expected no triples", i+1)
		}
	}
}
