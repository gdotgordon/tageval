package validator

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/robertkrimen/otto"
)

// The evauator is capable of running either a JavaScript
// or regexp validation.  It allows custom mapping
// functions for mapping Go types to JavaScript types.
// This is useful for items such as time.Time, where otto
// by default treats it as a generic JS Object, but using a
// JS Date() is a far better mapping.  In fact, the
// aformentioned mapping is already done, but the user
// may add additional such functions.
type evaluator struct {
	vm      *otto.Otto
	regexps map[string]*regexp.Regexp
	mapping map[reflect.Type]internalTypeMapper
	scripts map[string]*otto.Script
}

// The internalTypeMapper takes the user defined and publicd TypeMapper
// function, and does the final step of creating an otto.Object to
// hold the user's type declaration JavaScript.
type internalTypeMapper func(interface{}) (*otto.Object, error)

func newEvaluator() *evaluator {
	return &evaluator{
		vm:      otto.New(),
		regexps: make(map[string]*regexp.Regexp),
		mapping: make(map[reflect.Type]internalTypeMapper),
		scripts: make(map[string]*otto.Script),
	}
}

func (e *evaluator) addTypeMapping(t reflect.Type,
	f func(interface{}) string) {
	tmf := func(i interface{}) (*otto.Object, error) {
		obj, err := e.vm.Object(f(i))
		if err != nil {
			return nil, fmt.Errorf(
				"Custom object creation error for %v: %s",
				reflect.TypeOf(i), err)
		}
		return obj, nil
	}
	e.mapping[t] = tmf
}

// Evaluate a boolean JavaScript expression.
func (e *evaluator) evalBoolExpr(name string, val interface{}, expr string) (
	bool, error) {

	// First check if the type has a custom mapping function.
	f, ok := e.mapping[reflect.TypeOf(val)]
	if ok {
		var err error
		val, err = f(val)
		if err != nil {
			return false, err
		}
	}

	// Set the name of the variable (i.e. the field name) to
	// its value, which is either it's current Go value, or
	// the crresponding custom js type.
	err := e.vm.Set(name, val)
	if err != nil {
		return false, err
	}

	// Memoize the expression into a Sript object if it's not
	// already there.
	script := e.scripts[expr]
	if script == nil {
		script, err = e.vm.Compile("", expr)
		if err != nil {
			return false, err
		}
		e.scripts[expr] = script
	}

	// Run the thing and get the boolean result (or capature any error).
	// Note, an error should not happen undernormal circumstances, as it
	// is distinct frmo a validation function evaluating to "false".
	res, err := e.vm.Run(script)
	if err != nil {
		return false, err
	}

	b, err := res.ToBoolean()
	if err != nil {
		return false, err
	}
	return b, nil
}

// Evaluate a regular expression using the built-in Go mechanism.
// Again, the compiled expression is memoized for efficiency.
func (e *evaluator) evalRegexp(val string, pattern string) (bool, error) {
	rexp := e.regexps[pattern]
	if rexp == nil {
		rexp = regexp.MustCompile(pattern)
		e.regexps[pattern] = rexp
	}
	return regexp.Match(pattern, []byte(val))
}
