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
	j      string `expr:"j[0] == 'P'"`
	K      Talker
	L      string        `regexp:"^[aeiou]{4}$|hello"`
	M      float64       `expr:"M == 3.14"`
	N      time.Time     `expr:"N.getMonth() == new Date().getMonth()"`
	P      []int         `expr:"var sum = P.reduce(function(pv, cv) { return pv + cv; }, 0); sum == 10"`
	Q      []interface{} `expr:"Q.length == 0"`
	Talker               // Not supported, as we don't have a name.
}

func (ms MyStruct) move(string) (int, error) {
	return 45, nil
}

func TestValidationOlio(t *testing.T) {
	b1 := byte(3)
	ms1 := &MyStruct{A: 1, B: time.Now(), C: "hello",
		D: []Another{Another{"Joe", "Plano, TX"}},
		E: &b1, G: Another{"bingo", "Oshkosh, WI"},
		H: TalkingInt(7), I: map[string]int{"green": 12, "blue": 93}, j: "Pete",
		L: "uoiea", M: 3.14, N: time.Now().Add(2 * time.Second),
		P: []int{1, 2, 3, 4}}
	v := NewValidator()
	res, err := v.Validate(ms1)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	res.PrintResults(os.Stdout)
	if len(res.Succ) != 11 {
		t.Fatalf("validation expected 11 successes, got %d",
			len(res.Succ))
	}

	if len(res.Fail) != 4 {
		t.Fatalf("validation expected 4 failures, got %d",
			len(res.Fail))
	}
}

func TestChannelExprs(t *testing.T) {
	type StructWithChan struct {
		Chan1 chan (int)         `expr:"Chan1.cap==8"`
		Chan2 chan (int)         `expr:"Chan2.cap==0"`
		Chan3 chan (interface{}) `expr:"Chan3.cap==0"`
	}

	swc := StructWithChan{
		Chan1: make(chan (int), 8),
		Chan3: make(chan (interface{})),
	}

	// JSON serialization doesn't handle channels, but we can
	// still validate channels in a non JSON context.  However,
	// we need to create custom mapping s for each channel type.
	// In this case, we'll define functions that allows us to check
	// the channel capacity by creating a js Object with one field.
	v := NewValidator()
	v.ignoreJSONTags = true
	v.AddTypeMapping(reflect.TypeOf(swc.Chan1),
		func(i interface{}) string {
			c := i.(chan (int))
			return fmt.Sprintf("new Object({cap: %d})", cap(c))
		})
	v.AddTypeMapping(reflect.TypeOf(swc.Chan3),
		func(i interface{}) string {
			c := i.(chan (interface{}))
			return fmt.Sprintf("new Object({cap: %d})", cap(c))
		})
	res, err := v.Validate(&swc)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}

	fmt.Printf("chan succ = %v\n", res.Succ)
	fmt.Printf("chan fail = %v\n", res.Fail)
	if len(res.Succ) != 3 || len(res.Fail) != 0 {
		t.Fatalf("wrong number of expected successes and failues")
	}
}

func TestMap(t *testing.T) {
	type Other struct {
		Person string
		Where  string
	}

	type MapTest struct {
		Name string
		M    map[string]int   `expr:"M[\"Jane\"] == 5"`
		N    map[string]Other `expr:"N[\"Bob\"][\"Where\"] == \"Somewhere\""`
	}

	mt := &MapTest{
		Name: "Mary",
		M:    map[string]int{"Jane": 5},
		N:    map[string]Other{"Bob": Other{"Sue", "Somewhere"}},
	}

	v := NewValidator()
	res, err := v.Validate(mt)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	fmt.Printf("res: %v\n", res)
	if len(res.Succ) != 2 || len(res.Fail) != 0 {
		t.Fatalf("wrong number of expected successes and failues")
	}

	mt = &MapTest{
		Name: "Mary",
		M:    map[string]int{"Jane": 5},
		N:    map[string]Other{"Bob": Other{"Sue", "Anywhere"}},
	}
	res, err = v.Validate(mt)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	fmt.Printf("res: %v\n", res)
	if len(res.Succ) != 1 || len(res.Fail) != 1 {
		t.Fatalf("wrong number of expected successes and failues")
	}
}

func TestPrivateFields(t *testing.T) {
	type myob struct {
		First  int
		Second int
	}

	type noyb struct {
		blah string `regexp:"^ick$" expr:"blah == \"ick\""`
	}

	type privy struct {
		name   string `expr:"name[0] == 'J'"`
		age    int    `expr:"age > 21"`
		things []int  `expr:"things[0] > 2 && things[1] > 0"`
		other  []myob `expr:"(other[0]['Second'] - other[0]['First']) == -155"`
		iptr   *int   `expr:"iptr == 75"`
		b      noyb
		z      *int `expr:"z == 5"`
	}

	ival := 75
	p := privy{"Joe", 50, []int{3, 4}, []myob{{300, 145}}, &ival, noyb{"ick"}, nil}
	rv := reflect.ValueOf(&p).Elem()
	v := NewValidator()
	v.ignoreJSONTags = true
	res, err := v.ValidateAddressable(rv)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	fmt.Println(res)
	if len(res.Succ) != 7 || len(res.Fail) != 0 {
		t.Fatalf("wrong number of expected successes and failues")
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
