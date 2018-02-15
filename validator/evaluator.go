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
	mapping map[reflect.Type]typeMapper
	scripts map[string]*otto.Script
}

type typeMapper func(interface{}) (*otto.Object, error)

func newEvaluator() *evaluator {
	return &evaluator{
		vm:      otto.New(),
		regexps: make(map[string]*regexp.Regexp),
		mapping: make(map[reflect.Type]typeMapper),
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

// Evaluate a boolean Javascript expression.
func (e *evaluator) evalBoolExpr(name string, val interface{}, expr string) (
	bool, error) {
	f, ok := e.mapping[reflect.TypeOf(val)]
	if ok {
		var err error
		val, err = f(val)
		if err != nil {
			return false, err
		}
	}

	err := e.vm.Set(name, val)
	if err != nil {
		return false, err
	}

	script := e.scripts[expr]
	if script == nil {
		script, err = e.vm.Compile("", expr)
		if err != nil {
			return false, err
		}
		e.scripts[expr] = script
	}

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

func (e *evaluator) evalRegexp(val string, pattern string) (bool, error) {
	rexp := e.regexps[pattern]
	if rexp == nil {
		rexp = regexp.MustCompile(pattern)
		e.regexps[pattern] = rexp
	}
	return regexp.Match(pattern, []byte(val))
}
