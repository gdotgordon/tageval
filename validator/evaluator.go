package validator

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/robertkrimen/otto"
)

// The evauator is capable of running either a JavaScript
// or regexp valdaiton.
type evaluator struct {
	vm      *otto.Otto
	regexps map[string]*regexp.Regexp
	mapping map[reflect.Type]typeMapper
}

type typeMapper func(interface{}) (*otto.Object, error)

func newEvaluator() *evaluator {
	return &evaluator{
		vm:      otto.New(),
		regexps: make(map[string]*regexp.Regexp),
		mapping: make(map[reflect.Type]typeMapper),
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

	res, err := e.vm.Run(expr)
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
