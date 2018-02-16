// Package validator implements per-field validation for
// for struct members, by adding custom tags containing expressions.
// There are two types of validation supported: boolean JavaScript
// expressions for the current field (or contained structure members)
// using the "otto" embedded JavaScript engine, and regexp evaluations
// where this can be dome (strings or any Stringer objects).  A list
// of all failed and optionally successful validations is returned.
package validator

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Tag names for the types of validation that can be done.
// Note a JSON tag may or may not be present.
// Example struct members
// LastName string `json:"last_name" expr:"LastName.length<10"`
// LastName string `expr:"LastName.length<10"`
//
// State string `json:"state" regexp:"[A-Z]{2}"`
// State string `regexp:"[A-Z]{2}"`
//
// MyName string `json:"my_name" expr:"MyName.length<10" regexp:"^\p{L}.*$`
const (
	ExprTag   = "expr"
	RegexpTag = "regexp"
)

var (
	logger = NewLogger(os.Stderr, Trace)
)

// The Validator traverses the value of any interface{}
// to locate our custom tags as well as JSON tags.  It will
// validate any fields that contain validation expressions,
// either JavaScript expressions or regexps, and report back
// The results of the validation.
type Validator struct {
	ignoreJSONTags bool
	eval           *evaluator
}

// A Result captures the data from a single evaluation, including,
// most importantly, whether the validation succeeded.
type Result struct {
	Name  string
	Value interface{}
	Expr  string
}

// Results are the entirety of a single validation run.  It
// contians a list of successful validaions, and a list of
// failed ones.
type Results struct {
	Succ []*Result
	Fail []*Result
}

// TypeMapper delcares the signature of the function to add a
// custom type mapping.  Essentially, the string returned is a
// JavaScript fragment that creates an object that is somehwat
// equivalent or analagous to a Go type.
//
//The issue is that the built in js interpreter may treat some
// semantically menaingful types as generic structs, and because
// the field are mostly prviate, they aren't too useful to use in.
// js.  It may be helpful to view the built-in mapping of time.Time
// to a js Date object (below), where the string returned is a js
// code fragment that invokes "new Date()".
//
// This can be done for any type desired, using an exemplar of the
// type as the incoming interface{} parameter.
type TypeMapper func(interface{}) string

var (
	mappers map[reflect.Type]TypeMapper

	// TimeMapper is the default mapper from time.Time -> js Date.
	TimeMapper = func(i interface{}) string {
		t := i.(time.Time)
		us := (t.UnixNano() / 1000000) // need ms for js
		return fmt.Sprintf("new Date(%d)", us)
	}
)

func init() {
	mappers = make(map[reflect.Type]TypeMapper)
	mappers[reflect.TypeOf(time.Now())] = TimeMapper
}

// NewValidator returns a new item capable of traversing and
// inspecting any item (interface{}).
func NewValidator() *Validator {
	eval := newEvaluator()
	for k, f := range mappers {
		eval.addTypeMapping(k, f)
	}
	return &Validator{eval: eval}
}

// AddTypeMapping allows the user to declare and add their
// own type mapping to be used by the js engine.  The type
// mapping function is explained in the TypeMapper type
// declaation (above).
func (v *Validator) AddTypeMapping(t reflect.Type, tm TypeMapper) {
	v.eval.addTypeMapping(t, tm)
}

// Validate a Go item of any kind.  If the item is not a struct,
// or does not contain a struct anywhere, there will be nothing
// to evaluate.  Returns the results of all vaidations, or an
// error if something went wrong.  Note, failed validations do
// not cause an error to be set.
func (v *Validator) Validate(item interface{}) (*Results, error) {
	res := &Results{}
	if err := v.traverse(reflect.ValueOf(item), res); err != nil {
		return nil, err
	}
	return res, nil
}

// The main processing loop is invoked recursively as we
// traverse the value, eventually landing on a struct type,
// which is where the tags ar efound.  Types such as built-ins
// and channels require no further processing, so no acton happens.
// Any bulit-ins that were struct fields were already processed when
// handling the struct.
func (v Validator) traverse(val reflect.Value, res *Results) error {
	var err error
	t := val.Type()
	if t == reflect.TypeOf(time.Now()) {
		return nil
	}

	logger.Trace("Incoming: %v, %v\n", t, t.Kind())
	switch t.Kind() {

	// For slice and array, traverse each entry individually.
	case reflect.Slice, reflect.Array:
		for i := 0; i < val.Len(); i++ {
			if err = v.traverse(val.Index(i), res); err != nil {
				return err
			}
		}

	// Dereference the pointer if not nil.
	case reflect.Ptr:
		if val.Pointer() != 0 {
			if err = v.traverse(val.Elem(), res); err != nil {
				return err
			}
		}

	// For a map, traverse all the keys and values.
	case reflect.Map:
		keys := val.MapKeys()
		for _, key := range keys {
			if err = v.traverse(key, res); err != nil {
				return err
			}
			if err = v.traverse(val.MapIndex(key), res); err != nil {
				return err
			}
		}

	// Get the concrete type/value of the interface to process,
	// as this may be a type that has tagged fields.
	case reflect.Interface:
		if val.CanInterface() {
			iface := val.Interface()
			if iface != nil {
				if err = v.traverse(val.Elem(), res); err != nil {
					return err
				}
			}
		}

	// All tags are found on struct fields.
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			logger.Trace("Field (%d) name: %s type: %v kind: %v\n",
				i+1, f.Name, f.Type.Name(), f.Type.Kind())

			// Cannot get the interface{} value of unexported fields, so
			// cannot validate for those fields.
			var first rune
			for _, c := range f.Name {
				first = c
				break
			}
			if unicode.IsUpper(first) {
				if err = v.processTag(f, val.Field(i), res); err != nil {
					return err
				}
			}
			if err = v.traverse(val.Field(i), res); err != nil {
				return err
			}
		}
	}
	return nil
}

// Check the tags to see if there is something we need to validate.
// Validation can also only occur if our custom tags are present,
// although the json tag need not be present.
func (v Validator) processTag(f reflect.StructField,
	val reflect.Value, res *Results) error {

	// Our expression eval tag.
	exprTag := f.Tag.Get("expr")
	regexpTag := f.Tag.Get("regexp")
	if exprTag == "" && regexpTag == "" {
		return nil
	}

	var jtag string
	if f.Tag != "" {
		jtag, _ = f.Tag.Lookup("json")
		if !v.ignoreJSONTags && jtag == "-" {
			// This one won't get serialized to JSON, so skip.
			return nil
		}
	}

	// Get the underlying or concrete value.
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface:
		val = val.Elem()
	}

	// If the value is something like a nil interface
	// concrete object, forget it.
	if !val.IsValid() || !val.CanInterface() {
		return nil
	}
	iface := val.Interface()

	// Check whether this is the zero value for the type.  If
	// We are serializing to JSON, this is won't be processed.
	if !v.ignoreJSONTags &&
		(jtag != "" && strings.HasSuffix(jtag, ",omitempty")) {
		isZero := reflect.DeepEqual(iface,
			reflect.Zero(reflect.TypeOf(iface)).Interface())
		if isZero {
			logger.Info("Skip zero value for %s, '%v'\n", f.Name, iface)
			return nil
		}
	}

	// Game on!  Lets validate.
	if exprTag != "" {
		bv, err := v.eval.evalBoolExpr(f.Name, iface, exprTag)
		if err != nil {
			return err
		}
		r := &Result{
			Name:  f.Name,
			Value: iface,
			Expr:  exprTag,
		}
		if bv {
			res.Succ = append(res.Succ, r)
		} else {
			res.Fail = append(res.Fail, r)
		}
		logger.Trace("result for '%s', '%s', value: '%v': %t\n",
			exprTag, f.Name, iface, bv)
	}

	if regexpTag != "" {
		str := v.iToStr(iface)
		bv, err := v.eval.evalRegexp(str, regexpTag)
		if err != nil {
			return err
		}
		r := &Result{
			Name:  f.Name,
			Value: iface,
			Expr:  regexpTag,
		}
		if bv {
			res.Succ = append(res.Succ, r)
		} else {
			res.Fail = append(res.Fail, r)
		}
		logger.Trace("result for '%s', '%s', value: '%v': %t\n",
			regexpTag, f.Name, iface, bv)
	}
	return nil
}

// For validation, use a reasonable string value if we can
// determine one for the type, otherwise use the default
// "fmt" stirng conversion.
func (v Validator) iToStr(i interface{}) string {
	switch i.(type) {
	case string:
		return i.(string)
	case fmt.Stringer:
		return i.(fmt.Stringer).String()
	case bool:
		return fmt.Sprintf("%t", i.(bool))
	case int, int8, int16, int32, int64:
		return strconv.FormatInt(reflect.ValueOf(i).Int(), 10)
	case uint, uint8, uint16, uint32, uint64:
		return strconv.FormatUint(reflect.ValueOf(i).Uint(), 10)
	default:
		return fmt.Sprint(i)
	}
}

func (res *Result) String() string {
	tn := reflect.TypeOf(res.Value)
	var tstr string
	switch tn.Kind() {
	case reflect.Slice:
		var name string
		if tn.Elem().Kind() == reflect.Interface {
			name = "interface{}"
		} else {
			name = tn.Elem().Name()
		}
		tstr = "[]" + name
	case reflect.Array:
		tstr = fmt.Sprintf("[%d]%s", tn.Size(), tn.Elem().Name())
	case reflect.Map:
		tstr = fmt.Sprintf("map[%s]%s", tn.Key(), tn.Elem())
	case reflect.Chan:
		var name string
		if tn.Elem().Kind() == reflect.Interface {
			name = "interface{}"
		} else {
			name = tn.Elem().Name()
		}
		tstr = "chan(" + name + ")"
	default:
		tstr = tn.Name()
	}
	return fmt.Sprintf("'%s': expr: '%s' item: '%+v' (type: %v)",
		res.Name, res.Expr, res.Value, tstr)
}

// PrintResults shows the lists of successful and unsuccessful
// validations.
func (r *Results) PrintResults(w io.Writer) {
	fmt.Fprintln(w, "Results:")
	// Trying to avoid copying an item, so using iterative loop.
	for i := 0; i < len(r.Succ); i++ {
		if i == 0 {
			fmt.Fprintln(w, "Successes:")
		}
		fmt.Fprintln(w, r.Succ[i])
	}
	for i := 0; i < len(r.Fail); i++ {
		if i == 0 {
			fmt.Fprintln(w, "Failures:")
		}
		fmt.Fprintln(w, r.Fail[i])
	}
}
