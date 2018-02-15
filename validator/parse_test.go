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

type Mover interface {
	move(string) (int, error)
}

type MovingInt int

func (m MovingInt) move(string) (int, error) {
	return int(m), nil
}

func (m MovingInt) String() string {
	return strconv.Itoa(int(m))
}

type MyStruct struct {
	A int       `json:"a,omitempty" expr:"A>5"`
	B time.Time `json:"b"`
	C string    `regexp:"^[aeiou]{4}$|hello"`
	D []Another
	E *byte `expr:"E == 4"`
	F chan Another
	G Another `expr:"G[\"Fred\"].length > 2 && G[\"Location\"] == \"Oshkosh, WI\""`
	H Mover   `json:"mover" expr:"H < 400" regexp:"^[0-9]$"`
	I map[string]int
	j string
	K Mover
	L string    `regexp:"^[aeiou]{4}$|hello"`
	M float64   `expr:"M == 3.14"`
	N time.Time `expr:"N.getMonth() == new Date().getMonth()"`
	Mover
}

func (ms MyStruct) move(string) (int, error) {
	return 45, nil
}

func TestValidation(t *testing.T) {
	b1 := byte(3)
	ms1 := &MyStruct{A: 1, B: time.Now(), C: "hello",
		D: []Another{Another{"Joe", "Plano, TX"}},
		E: &b1, G: Another{"bingo", "Oshkosh, WI"},
		H: MovingInt(7), I: map[string]int{"green": 12, "blue": 93}, j: "a",
		L: "uoiea", M: 3.14, N: time.Now().Add(2 * time.Second)}
	v := NewValidator()
	res, err := v.Validate(ms1)
	fmt.Println("error: ", err)
	if err == nil {
		res.PrintResults(os.Stdout)
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
