# tageval
Package to validate Go struct fields via either JavaScript expressions or regular expressions.

This package implements per-field validation for Go struct members using custom field tags.  While this is not the first package to provide this, it attempts to provide a flexible and extensible API, leveraging a complete standard scripting language, the ECMA 5 version of JavaScript.  This is accomplished using the Go-based _otto_ embedded JavaScript implementation at <https://github.com/robertkrimen/otto>.  The mapping from Go types to JavaScript is generally straightforward, and where it is less so, the current _tageval_ API adds the capability to define custom Go -> JavaScript bindings to be used with _otto_, by defining custom type mapping functions in Go.

The package also supports standard Go `regexp` validation for fields where a string value can be ascertained (including items that implement the `fmt.Stringer` interface).  It would be easy to adapt the API to handle additional types of validation, but the two modes currently supported provide a good balance between standard Go mechanisms and the power of a rich and easy-to-use scripting language.

## How Does it Work?
As mentioned, additional Go struct field tags are introduced, `eval` and `regexp`.  These are designed to either work alongside JSON tags and observe JSON rules such as `omitempty` and `-`, or work as an independent validation tool.

For normal use, any item (`interface{}`) or pointer to such an item is passed in to be traversed and evaluated.

If you want to use this package for validation outside JSON *and* need to traverse private fields that are more complex than built-in data types, there is a separate function for this.  The call requires an _addressable_ reflection value to be passed in.  This is because otto requires a concrete interface{} to evaluate, and the only way to create an interface{} in these situations with complex private members is by using the _unsafe_ package with pointers.  While no data is ever modified by `tageval`, only use this mode if you really need it.  Perhaps try the standard API first and then fall back to this method.

The following declaration shows how the tags lookvarious legitimate tag formats.  Validation for virtually every Go type is supported, including channels, slices, arrays, etc., even if they would not be serialized by the Go JSON serializer.  For some types, such as channels, the user must define a custom type-mapper, but this works seamlessly with the API.

```
type MyStruct struct {
  FirstName string `json:"my_name" expr:"MyName.length<10" regexp:"^\p{L}.*$`
  LastName string  `json:"last_name" expr:"LastName.length<10"`
  City string      `expr:"City.length<10"`
  State string     `json:"state" regexp:"[A-Z]{2}"`
  Data []int       `expr:"var sum = P.reduce(function(pv, cv) { return pv + cv; }, 0); sum == 10"`
 }
 ```

## How Do I Use It?
The API itself is simple.  The expressions and regexps used are limited by the complexity of the JavaScript language and regexp language respectively.  The expressions can be as simple as a simple comparison, such as `a < 7`, or as complex as using the functional API built into JavaScript.  Examples of both will be shown.

As far as the API is concerned, the default mode is to evaluate an `interface{}` instance and run any validation tags encountered.  Also, by default, Go JSON serialization rules are obeyed.  This means private fields, fields tagged with `-`, or non-struct zero value fields are skipped.

### Quick look at expression evaluation
Here is a very simple example:
```
<imports here ...>
import "github.com/GGordonCode/tageval"

type MyStruct struct {
	A      int       `json:"a,omitempty" expr:"A > 5"`
}

func main() {
	ms1 := &MyStruct{4}
	v := tageval.NewValidator()
	ok, res, err := v.Validate(ms1)
	if err != nil {
		fmt.Printf(os.Stderr, "validation failed with error: %v", err)
    	os.Exit(1)
	}
	if !ok {
		t.Fatalf("unexpected failure result")
	}
	res.PrintResults(os.Stdout)
}
```

In general, an expression uses the Go field's name (here "A") as the name of the variable.  For more complex data types, such as structs, any or all of the contained fields may be used to build an evaluation expression (Go's struct fields are mapped to JavaScript object properties, so fields are accessed as property lookups, an example of which will be shown later).

This test shown here leads to an `ok` value of `false`, as the value of A is not greater than 5.  The `res` parameter is an object of type `Results`, which breaks down in detail the results of failures (or optionally both successes and failures) from the `Validate()` call.  This is extremely helpful for figuring out what went wrong, especially with multiple expressions, but may be ignored by assigning it to `_`.  Again, the `error` type is reserved for an execution error in the Validation, and not a validation failure.

Now we'll try a very slightly more complex example, a slice.  Assume the code from above, but change the struct definition and instance to be as follows:

```
type MyStruct struct {
	Vals []int `expr:"Vals.length >= 2 && Vals[0] > 2 && Vals[1] > 10"`
}

ms1 := MyStruct{[]int{5, 7, 32}}
```

This one fails as Vals[1] is 7, which is not > 10, and here is the `Result` item for this one:

`'Vals' (type: []int) item: '[5 7 32]', expr: 'Vals.length >= 2 && Vals[0] > 2 && Vals[1] > 10'  : failed`

With all the information in one place, the task of finding the problem should be simpler.

### Reflection
The reflection uses the `regexp` package in Go to see whether the string matches.  It does not require a complete match, and if you require that, start the regexp string with a '^' and terminate it with a '$'.  Getting the value to validate against the regexp is obvious for strings and objects implmenting `fmt.Stringer()`, as well as all the `int` and `uint` types.  The type `bool` maps to "true" or "false", and all the other types use the default format (`%v`) from the fmt package.

Here's an example assuming the rest of the program above is unchanged:

```
type SpecialInt int

func (s SpecialInt) String() string {
	return fmt.Sprintf("I'm special, my value is: %d", s)
}

type SpecialStruct struct {
Spec	SpecialInt `json:"spec" regexp:"^.*: [-]?[0-9]+$"`
}
...
ms1 := &SpecialStruct{-56}
v := tageval.NewValidator(tageval.Option{ShowSuccesses, true})
ok, res, err := v.Validate(ms1)
...
```
Note we've added an option to return successes in the list, as well as failures.  Options will be covered in the next section.  Also note that we don't need to specify the variable name in the expression.  So running this expression returns `true` for `ok`, and the detailed result is:

`'Spec' (type: SpecialInt) item: 'I'm special, my value is: -56', expr: '^.*: [-]?[0-9]+$'  : ok`



