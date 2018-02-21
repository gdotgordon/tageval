// Package tageval implements per-field validation for struct
// members, by adding custom tags containing expressions.  There
// are two types of validation supported: boolean JavaScript
// expressions and regular expression evaluations.
//
// The JavaScript expressions are evaluated using the "otto" embedded
// JavaScript engine, and may be applied to any struct field (including
// slices, nested structs, channels, or pointers to these types).
// Regexp evaluation may be applied to any field of type string,
// fmt.Stringer, int, uint and bool types.  Other types use the default
// fmt package representation to prduce a string vale for regexp.
//
// A simple boolean describing whether the overall valdation succeeded is
// returned, as well as a detailed list containing the results of each
// failed evaluation (and optionally successful evaluations) is returned.
package tageval

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unsafe"
)

// Struct tag names for the types of validation that can be done.
// Note a JSON tag may or may not be present.
// Example struct members
//   LastName string `json:"last_name" expr:"LastName.length<10"`
//   LastName string `expr:"LastName.length<10"`
//
//   State string `json:"state" regexp:"[A-Z]{2}"`
//   State string `regexp:"[A-Z]{2}"`
//
//   MyName string `json:"my_name" expr:"MyName.length<10" regexp:"^\p{L}.*$`
const (
	ExprTag   = "expr"
	RegexpTag = "regexp"
)

// Options supported and passed to "NewValidator()"".
const (
	// ProcessAsJSON selects whether the tag processor should
	// obey JSON semantics when processng tags.  The value for
	// this option is of type bool, with the default being 'true'.
	ProcessAsJSON = "ProcessAsJson"

	// ShowSuccesses also incudes successful validations in the
	// list returned from any Vaildate() call.  The default is
	// "false", meaning only failures are shown.
	ShowSuccesses = "ShowSuccesses"
)

// The Validator traverses a given interface{} instance to
// locate our custom tags as well as JSON tags.  It will
// validate any fields that contain validation expressions,
// either JavaScript expressions or regexps, and report back
// The results of the validation.
type Validator struct {
	processAsJSON bool
	showSuccesses bool
	eval          *evaluator
}

// A Result captures the data from a single evaluation.  The validation
// returns separate lists of successful and failed validations containing
// the following information.
type Result struct {
	Name  string
	Value interface{}
	Type  reflect.Type
	Expr  string
	Valid bool
}

// Option defines items for passing Validator configuration options.
type Option struct {
	Name  string
	Value interface{}
}

// TypeMapper delcares the signature of the function to add a
// custom type mapping.  Essentially, the string returned is a
// JavaScript fragment that creates an object that is somehwat
// equivalent or analagous to a Go type, or at least useful for
// evaluation purposes.
//
// Unfortunaltey, the built in JavaScript interpreter may treat some
// semantically meaningful types as generic structs, and because
// the fields are mostly private, they aren't too useful in JavaScript.
// It may be helpful to view the built-in mapping of time.Time
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

var (
	logger = NewLogger(os.Stderr, Off)

	timeType = reflect.TypeOf(time.Now())
)

func init() {
	mappers = make(map[reflect.Type]TypeMapper)
	mappers[timeType] = TimeMapper
}

// NewValidator returns a new item capable of traversing and
// inspecting any item (interface{}).
func NewValidator(options ...Option) (*Validator, error) {
	eval := newEvaluator()
	ret := &Validator{
		processAsJSON: true,
		showSuccesses: false,
		eval:          eval,
	}

	for _, opt := range options {
		switch opt.Name {
		case ProcessAsJSON:
			val, ok := opt.Value.(bool)
			if !ok {
				return nil,
					fmt.Errorf("bool value expected for Option %s",
						ProcessAsJSON)
			}
			ret.processAsJSON = val
		case ShowSuccesses:
			val, ok := opt.Value.(bool)
			if !ok {
				return nil,
					fmt.Errorf("bool value expected for Option %s",
						ShowSuccesses)
			}
			ret.showSuccesses = val
		default:
			return nil, fmt.Errorf("unknown option: %s", opt.Name)
		}
	}
	for k, f := range mappers {
		eval.addTypeMapping(k, f)
	}
	return ret, nil
}

// AddTypeMapping allows the user to declare and add their
// own type mapping to be used by the js engine.  The type
// mapping function is explained in the TypeMapper type
// declaration (above).
func (v *Validator) AddTypeMapping(t reflect.Type, tm TypeMapper) {
	v.eval.addTypeMapping(t, tm)
}

// Copy makes an effective copy of the current Validtor.  Making a copy
// for each goroutine using validation is a solution for the lack of
// concurrency in the underlying Javascript engine.  Note the caches
// of compiled expressions and regexps are not copied.
func (v *Validator) Copy() *Validator {
	return &Validator{v.processAsJSON, v.showSuccesses, v.eval.copy()}
}

// Validate a Go item (or pointer) of any kind.  If the item is not
// a struct, or does not contain or reference a struct anywhere, there
// will be nothing to evaluate, as that is where all the tags live.
// This function returns the results of all validations, or an error if
// something went wrong.  Note, failed validations do not cause an error
// to be returned.
func (v *Validator) Validate(item interface{}) (bool, []Result, error) {
	return v.doValidation(reflect.ValueOf(item))
}

// ValidateAddressable is a variant of "Validate()" that accepts a
// Go reflect.Value of any kind that is addressable.  This variant
// should only be used if it is desired to perform expression evaluation
// on private fields that are not of primitive type.  The following
// creates such an addressable item:
//	p := struct{foo: "bar"}
//	rv := reflect.ValueOf(&p).Elem()
//  ok, _, err := v.Validate(rv)
//
//If the item is not a struct, or does not contain or reference a
// struct anywhere, there will be nothing to evaluate, as that is
// where all the tags live.  This function returns the results of all
// validations, or an error if something went wrong.  Note, failed
// validations do not cause an error to be returned.
func (v *Validator) ValidateAddressable(rvAddr reflect.Value) (bool,
	[]Result, error) {
	return v.doValidation(rvAddr)
}

func (v *Validator) doValidation(rv reflect.Value) (bool, []Result, error) {
	var res []Result
	if err := v.traverse(rv, &res); err != nil {
		return false, nil, err
	}
	ok := true
	for _, rslt := range res {
		if !rslt.Valid {
			ok = false
			break
		}
	}
	return ok, res, nil
}

// The main processing loop is invoked recursively as we
// traverse the value, eventually landing on a struct type,
// which is where the tags are found.  Types such as built-ins
// and channels require no further processing, so no action happens.
func (v *Validator) traverse(val reflect.Value, res *[]Result) error {
	var err error
	t := val.Type()

	if t == timeType {
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
		rv := reflect.Indirect(val)
		if rv.Kind() != reflect.Invalid {
			if err = v.traverse(reflect.Indirect(val), res); err != nil {
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
		if val.IsValid() && !val.IsNil() {
			if err = v.traverse(val.Elem(), res); err != nil {
				return err
			}
		}

	// All tags are found on struct fields.
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)

			// If following JSON serialization rules, skip
			// any private fields.
			handleTags := true
			if v.processAsJSON {
				var first rune
				for _, c := range f.Name {
					first = c
					break
				}
				if !unicode.IsUpper(first) {
					handleTags = false
				}
			}

			if handleTags {
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
func (v *Validator) processTag(f reflect.StructField,
	val reflect.Value, res *[]Result) error {

	// Our expression eval tags.
	exprTag := f.Tag.Get("expr")
	regexpTag := f.Tag.Get("regexp")
	if exprTag == "" && regexpTag == "" {
		return nil
	}

	var jtag string
	if f.Tag != "" {
		jtag, _ = f.Tag.Lookup("json")
		if v.processAsJSON && jtag == "-" {
			// This one won't get serialized to JSON, so skip.
			return nil
		}
	}

	logger.Trace("Process tag, name: %s type: %v kind: %v\n",
		f.Name, f.Type.Name(), f.Type.Kind())

	// Get the underlying or concrete value.
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface:
		val = val.Elem()
	}

	// If the value is something like a nil interface concrete object,
	// forget it.
	var iface interface{}

	if !val.IsValid() {
		return nil
	}

	if val.CanInterface() {
		iface = val.Interface()
	} else {
		// Handle private builtin primitive types for starters.
		switch val.Kind() {
		case reflect.String:
			iface = val.String()
		case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
			iface = val.Int()
		case reflect.Float32, reflect.Float64:
			iface = val.Float()
		case reflect.Complex64, reflect.Complex128:
			iface = val.Complex()
		case reflect.Bool:
			iface = val.Bool()
		default:
			// Been beat up and battered 'round
			// Been sent up, and I've been shot down
			// You're the best thing that I've ever found
			// Handle me with care
			rf := reflect.NewAt(val.Type(),
				unsafe.Pointer(val.UnsafeAddr())).Elem()
			iface = rf.Interface()
		}
	}

	// Check whether this is the zero value for the type.  If
	// We are serializing to JSON, this is won't be processed.
	// Note: references (not pointers) to structs are serialized
	// to JSON in Go even if they are empty.
	if v.processAsJSON && f.Type.Kind() != reflect.Struct {
		if strings.HasSuffix(jtag, ",omitempty") {
			isZero := reflect.DeepEqual(iface,
				reflect.Zero(reflect.TypeOf(iface)).Interface())
			if isZero {
				logger.Info("Skip zero value for %s, '%v'\n", f.Name, iface)
				return nil
			}
		}
	}

	// Game on!  Let's validate.
	var bv bool
	var err error
	if exprTag != "" {

		// Support shortcuts for simple relational expressions, i.e.
		// "<= 7" is a synonym for "<current field name> <= 7".
		expr := exprTag
		ts := strings.TrimSpace(expr)
		switch ts[0] {
		case '!':
			// '!' could be a simple negation, so check "!=".
			if len(ts) < 2 || ts[1] != '=' {
				break
			}
			fallthrough
		case '<', '>', '=':
			// Must be start of right-hand side of expr or syntax error.
			var buffer bytes.Buffer
			buffer.WriteString(f.Name)
			buffer.WriteString(" ")
			buffer.WriteString(exprTag)
			expr = buffer.String()
		}

		bv, err = v.eval.evalBoolExpr(f.Name, iface, expr)
		if err != nil {
			return err
		}

		if !bv || v.showSuccesses {
			r := Result{
				Name:  f.Name,
				Value: iface,
				Type:  f.Type,
				Expr:  expr,
				Valid: bv,
			}
			*res = append(*res, r)
		}
	}

	if regexpTag != "" {
		str := v.iToStr(iface)
		bv, err = v.eval.evalRegexp(str, regexpTag)
		if err != nil {
			return err
		}
		if !bv || v.showSuccesses {
			r := Result{
				Name:  f.Name,
				Value: iface,
				Type:  f.Type,
				Expr:  regexpTag,
				Valid: bv,
			}
			*res = append(*res, r)
		}
	}

	logger.Trace("result for '%s', '%s', value: '%v': %t\n",
		regexpTag, f.Name, iface, bv)
	return nil
}

// For regexps, use a reasonable string value if we can
// determine one for the type, otherwise use the default
// "fmt" string conversion.
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

	valid := "ok"
	if !res.Valid {
		valid = "failed"
	}
	return fmt.Sprintf("'%s' (type: %v) item: '%+v', expr: '%s'  : %s",
		res.Name, tstr, res.Value, res.Expr, valid)
}

// PrintResults shows the lists of successful and unsuccessful
// validations.
func PrintResults(w io.Writer, res []Result) {
	fmt.Fprintln(w, "Results:")
	for i := 0; i < len(res); i++ {
		fmt.Fprintln(w, res[i].String())
	}
}
