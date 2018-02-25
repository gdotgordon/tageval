package tageval

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

type checker struct {
	name  string
	valid bool
}

type Another struct {
	Fred     string `json:"fred" expr:"Fred.length<10"`
	Location string `json:"location" expr:"Location.indexOf('TX') != -1"`
}

func (a *Another) String() string {
	return fmt.Sprintf("Fred: '%s', Location: '%s'", a.Fred, a.Location)
}

type Talker interface {
	Talk() string
}

type TalkingInt int

func (t *TalkingInt) Talk() string {
	return fmt.Sprintf("Hi, let me introudce myself, I am '%d'", t)
}

func (t *TalkingInt) String() string {
	return strconv.Itoa(int(*t))
}

type MyStruct struct {
	A      int       `json:"a,omitempty" expr:"A>5"`
	B      time.Time `json:"b"`
	C      string    `regexp:"^[aeiou]{4}$|hello"`
	D      []Another `json:"d,omitempty" expr:"D.length == 1"`
	E      *byte     `expr:"E == 4"`
	F      chan Another
	G      Another `expr:"G['Fred'].length > 2 && G['Location'] == 'Oshkosh, WI'"`
	H      Talker  `json:"talker" expr:"H < 400" regexp:"^[0-9]$"`
	I      map[string]int
	j      string `expr:"j[0] == 'P'"`
	K      Talker
	L      string        `regexp:"^[aeiou]{4}$|hello"`
	M      float64       `expr:"== 3.14"`
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
	ti := TalkingInt(7)
	ms1 := &MyStruct{A: 1, B: time.Now(), C: "hello",
		D: []Another{Another{"Joe", "Plano, TX"}},
		E: &b1, G: Another{"bingo", "Oshkosh, WI"},
		H: &ti, I: map[string]int{"green": 12, "blue": 93}, j: "Pete",
		L: "uoiea", M: 3.14, N: time.Now().Add(2 * time.Second),
		P: []int{1, 2, 3, 4}}
	v, err := NewValidator(Option{ShowSuccesses, true})
	if err != nil {
		t.Fatalf("error creating validator: %v", err)
	}

	ok, res, err := v.Validate(ms1)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	if ok {
		t.Fatalf("unexpected success result")
	}

	PrintResults(os.Stdout, res)
	correlate(t, res, []checker{
		{"A", false},
		{"C", true},
		{"D", true},
		{"Fred", true},
		{"Location", true},
		{"E", false},
		{"G", true},
		{"Fred", true},
		{"Location", false},
		{"H", true},
		{"H", true},
		{"L", false},
		{"M", true},
		{"N", true},
		{"P", true},
		{"Q", true},
	})
}

func TestZeroValuesOlio(t *testing.T) {
	ms1 := &MyStruct{}
	v, err := NewValidator(Option{ShowSuccesses, true})
	if err != nil {
		t.Fatalf("error creating validator: %v", err)
	}
	ok, res, err := v.Validate(ms1)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	if ok {
		t.Fatalf("unexpected success result")
	}

	PrintResults(os.Stdout, res)
	correlate(t, res, []checker{
		{"C", false},
		{"G", false},
		{"Fred", true},
		{"Location", false},
		{"L", false},
		{"M", false},
		{"N", false},
		{"P", false},
		{"Q", true},
	})
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
	v, err := NewValidator(Option{ShowSuccesses, true})
	if err != nil {
		t.Fatalf("error creating validator: %v", err)
	}
	v.processAsJSON = false
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
	ok, res, err := v.Validate(&swc)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	if !ok {
		t.Fatalf("unexpected failure result")
	}

	PrintResults(os.Stdout, res)
	expected := []checker{checker{"Chan1", true}, checker{"Chan2", true},
		checker{"Chan3", true}}
	correlate(t, res, expected)
}

func TestMap(t *testing.T) {
	type Other struct {
		Person string
		Where  string
	}

	type MapTest struct {
		Name string
		M    map[string]int   `expr:"M['Jane'] == 5"`
		N    map[string]Other `expr:"N['Bob']['Where'] == 'Somewhere'"`
	}

	mt := &MapTest{
		Name: "Mary",
		M:    map[string]int{"Jane": 5},
		N:    map[string]Other{"Bob": Other{"Sue", "Somewhere"}},
	}

	v, err := NewValidator(Option{ShowSuccesses, true})
	if err != nil {
		t.Fatalf("error creating validator: %v", err)
	}
	ok, res, err := v.Validate(mt)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	if !ok {
		t.Fatalf("unexpected failure result")
	}

	expected := []checker{checker{"M", true}, checker{"N", true}}
	correlate(t, res, expected)

	mt = &MapTest{
		Name: "Mary",
		M:    map[string]int{"Jane": 5},
		N:    map[string]Other{"Bob": Other{"Sue", "Anywhere"}},
	}
	ok, res, err = v.Validate(mt)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	if ok {
		t.Fatalf("unexpected success result")
	}

	PrintResults(os.Stdout, res)
	expected = []checker{checker{"M", true}, checker{"N", false}}
	correlate(t, res, expected)
}

type slint []int64

func (s slint) String() string {
	var sum int64
	for _, v := range s {
		sum += v
	}
	return strconv.FormatInt(sum, 10)
}

func TestPrivateFields(t *testing.T) {
	type myob struct {
		First  int
		Second int
	}

	type noyb struct {
		blah string  `regexp:"^ick$" expr:"blah == 'ick'"`
		bval bool    `expr:"!bval"`
		f    float64 `expr:"Math.sqrt(f) > 5"`
		g    uint64  `expr:"g > 0"`
	}

	type privy struct {
		name   string `expr:"name[0] == 'J'"`
		age    int    `expr:"age > 21"`
		things [2]int `expr:"things[0] > 2 && things[1] > 0"`
		other  []myob `expr:"(other[0]['Second'] - other[0]['First']) == -155"`
		iptr   *int   `expr:"iptr == 75"`
		b      noyb
		y      int  `expr:"!= 5"`
		z      *int `expr:"z == 5"`
	}

	ival := 75
	p := privy{"Joe", 50, [2]int{3, 4}, []myob{{300, 145}}, &ival,
		noyb{"ick", 1 > 2, 45.1, 3}, 0, nil}
	v, err := NewValidator(Option{ProcessAsJSON, false},
		Option{ShowSuccesses, true})
	if err != nil {
		t.Fatalf("error creating validator: %v", err)
	}

	// Not addressable inside.
	ok, res, err := v.Validate(p)
	if err == nil {
		t.Fatalf("did not receive expected error")
	}

	// Not addressable from the start.
	ok, res, err = v.ValidateAddressable(p)
	if err == nil {
		t.Fatalf("did not receive expected error")
	}

	// Good to go.
	ok, res, err = v.ValidateAddressable(&p)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	if !ok {
		t.Fatalf("unexpected failure result")
	}

	PrintResults(os.Stdout, res)
	correlate(t, res, []checker{
		{"name", true},
		{"age", true},
		{"things", true},
		{"other", true},
		{"iptr", true},
		{"blah", true},
		{"blah", true},
		{"bval", true},
		{"f", true},
		{"g", true},
		{"y", true},
	})

	// This is a bizarre case, but it works!  We have a pointer to
	// an interface, with an eval based on the concrete type, plus
	// it's a private member, so in the end we have to dig it out
	// with an unsafe pointer.  The weirdness actually begins with
	// the fact that we're even using a pointer to an interface here.
	type stinger struct {
		s *fmt.Stringer `expr:"s[1] == 7"`
	}
	var stgr fmt.Stringer
	stgr = &slint{3, 7}
	s := stinger{&stgr}
	ok, res, err = v.ValidateAddressable(&s)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	PrintResults(os.Stdout, res)
	if !ok {
		t.Fatalf("unexpected failure result")
	}
}

func TestEmptyInterface(t *testing.T) {
	type DoGooder interface {
		DoGoodThings()
	}
	type IfaceOnly struct {
		DoGood DoGooder
	}

	v, err := NewValidator(Option{ShowSuccesses, true})
	if err != nil {
		t.Fatalf("error creating validator: %v", err)
	}

	ok, res, err := v.Validate(IfaceOnly{})
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	if !ok {
		t.Fatalf("unexpected failure result")
	}
	PrintResults(os.Stdout, res)
}

func TestValidatorError(t *testing.T) {
	type Cracked struct {
		BadEgg string `expr:"this omelet has no !*@&^% mushrooms"`
	}

	v, err := NewValidator(Option{ShowSuccesses, true})
	if err != nil {
		t.Fatalf("error creating validator: %v", err)
	}

	_, _, err = v.Validate(Cracked{"Jumbo"})
	if err == nil {
		t.Fatalf("expected validation error did not occur.")
	}
}

func TestCopyValidator(t *testing.T) {
	type CopyTest struct {
		A int    `expr:"== 8"`
		B string `regexp:"^hello$"`
		C int    `expr:"== 9"`
		D string `regexp:"^goodbye$"`
	}
	v, err := NewValidator(Option{ShowSuccesses, true})
	if err != nil {
		t.Fatalf("error creating validator: %v", err)
	}
	vc := v.Copy()
	ct := CopyTest{8, "hello", 10, "adios"}
	ok, res, err := vc.Validate(ct)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	if ok {
		t.Fatalf("unexpected success result")
	}
	PrintResults(os.Stdout, res)
	expected := []checker{
		checker{"A", true},
		checker{"B", true},
		checker{"C", false},
		checker{"D", false},
	}
	correlate(t, res, expected)
}

func TestRegexpStringTypes(t *testing.T) {
	type RegTest struct {
		A int    `regexp:"^[-]?[0-9]{1,}$""`
		B uint32 `regexp:"^[0-9]{3}$"`
		C bool   `regexp:"^false$"`
		D string `regexp:"^goodbye$"`
		E string `json:"-" regexp:"hi"`
	}
	v, err := NewValidator(Option{ShowSuccesses, true})
	if err != nil {
		t.Fatalf("error creating validator: %v", err)
	}
	rt := RegTest{-84, 345, 5 > 4, "au revoir", "hi"}
	ok, res, err := v.Validate(rt)
	if err != nil {
		t.Fatalf("validation failed with error: %v", err)
	}
	if ok {
		t.Fatalf("unexpected success result")
	}
	PrintResults(os.Stdout, res)
	expected := []checker{
		checker{"A", true},
		checker{"B", true},
		checker{"C", false},
		checker{"D", false},
	}
	correlate(t, res, expected)
}

func TestNewOptions(t *testing.T) {
	_, err := NewValidator(Option{ShowSuccesses, 3})
	if err == nil {
		t.Fatalf("expected error not received")
	}

	_, err = NewValidator(Option{ProcessAsJSON, "hi"})
	if err == nil {
		t.Fatalf("expected error not received")
	}

	_, err = NewValidator(Option{"invalid option", true})
	if err == nil {
		t.Fatalf("expected error not received")
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

	expr = "b + * & < 7.01"
	val = reflect.ValueOf(7)
	v1 = val.Int()

	_, err = v.evalBoolExpr("b", v1, expr)
	if err == nil {
		t.Fatalf("Did not get expected evaluation error")
	}
}

func correlate(t *testing.T, results []Result, expected []checker) {
	if len(results) != len(expected) {
		t.Fatalf("Expected %d results, got %d", len(expected), len(results))
	}
	for i, v := range results {
		if v.Name != expected[i].name || v.Valid != expected[i].valid {
			t.Fatalf("Expected result %d to be for '%+v'\n", i, expected[i])
		}
	}
}
