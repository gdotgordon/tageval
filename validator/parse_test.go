package validator

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

type Another struct {
	Fred     string `json:"fred" expr:"Fred.length<10"`
	Location string `json:"location" expr:"Location.indexOf('TX') != -1"`
}

func (a Another) String() string {
	return "hello"
}

type Talker interface {
	Talk() string
}

type TalkingInt int

func (t TalkingInt) Talk() string {
	return fmt.Sprintf("Hi, let me introudce myself, I am '%d'", t)
}

func (t TalkingInt) String() string {
	return strconv.Itoa(int(t))
}

type MyStruct struct {
	A      int       `json:"a,omitempty" expr:"A>5"`
	B      time.Time `json:"b"`
	C      string    `regexp:"^[aeiou]{4}$|hello"`
	D      []Another
	E      *byte `expr:"E == 4"`
	F      chan Another
	G      Another `expr:"G[\"Fred\"].length > 2 && G[\"Location\"] == \"Oshkosh, WI\""`
	H      Talker  `json:"talker" expr:"H < 400" regexp:"^[0-9]$"`
	I      map[string]int
	j      string
	K      Talker
	L      string    `regexp:"^[aeiou]{4}$|hello"`
	M      float64   `expr:"M == 3.14"`
	N      time.Time `expr:"N.getMonth() == new Date().getMonth()"`
	P      []int     `expr:"var sum = P.reduce(function(pv, cv) { return pv + cv; }, 0); sum == 10"`
	Talker           // Not supported, as we don't have a name.
}

func (ms MyStruct) move(string) (int, error) {
	return 45, nil
}

func TestValidation(t *testing.T) {
	b1 := byte(3)
	ms1 := &MyStruct{A: 1, B: time.Now(), C: "hello",
		D: []Another{Another{"Joe", "Plano, TX"}},
		E: &b1, G: Another{"bingo", "Oshkosh, WI"},
		H: TalkingInt(7), I: map[string]int{"green": 12, "blue": 93}, j: "a",
		L: "uoiea", M: 3.14, N: time.Now().Add(2 * time.Second),
		P: []int{1, 2, 3, 4}}
	v := NewValidator()
	res, err := v.Validate(ms1)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	res.PrintResults(os.Stdout)
	if len(res.Succ) != 10 {
		t.Fatalf("validation expected 9 successes, got %d",
			len(res.Succ))
	}

	if len(res.Fail) != 4 {
		t.Fatalf("validation expected 4 failures, got %d",
			len(res.Fail))
	}
}

func TestEvaluation(t *testing.T) {
	v := newEvaluator()

	expr := "b < 7.01"
	val := reflect.ValueOf(7)
	v1 := val.Int()

	res, err := v.evalBoolExpr("b", v1, expr)
	if err != nil {
		t.Fatalf("Unexpected evaluation error: %s", err)
	}
	if !res {
		t.Fatalf("Unexpected false resut for: '%s'", expr)
	}

	expr = "s.length > 10"
	val = reflect.ValueOf("hello")
	vs := val.String()
	res, err = v.evalBoolExpr("s", vs, expr)
	if err != nil {
		t.Fatalf("Unexpected evaluation error: %s", err)
	}
	if res {
		t.Fatalf("Unexpected true resut for: '%s'", expr)
	}

	n := time.Now().Add(-5 * time.Minute)
	v.addTypeMapping(reflect.TypeOf(time.Now()), TimeMapper)
	expr = "console.log('d1 = ' + d1); d2 = new Date(); d1 < d2"
	res, err = v.evalBoolExpr("d1", n, expr)
	if err != nil {
		t.Fatalf("Unexpected evaluation error: %s", err)
	}
	if !res {
		t.Fatalf("Unexpected false resut for: '%s'", expr)
	}
}
