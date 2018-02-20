# tageval
Package to validate Go struct fields via either JavaScript expressions or regular expressions.

This package implements per-field validation for Go struct members using custom field tags.  While this is not the first package to provide this, it attempts to provide a flexible and extensible API, leveraging a complete standard scripting language, the ECMA 5 version of JavaScript.  This is accomplished using the Go-based _otto_ embedded JavaScript implementation at <https://github.com/robertkrimen/otto>.  The mapping from Go types to JavaScript is generally straightforward, and where more usable bindings are required, the current _tageval_ API adds the capability to define custom Go -> JavaScript bindings to be used with _otto_, by defining custom type mapping functions in Go.

The package also supports standard Go `regexp` validation for fields where a string value can be ascertained (including items that implement the `fmt.Stringer` interface).  It would be easy to adapt the API to handle additional types of validation, but the two modes currently supported provide a good balance between standard Go mechanisms and the power of a rich and easy-to-use scripting language.

## How Does it Work?
As mentioned, additional Go struct field tags are introduced, `eval` and `regexp`.  These are designed to either work alongside JSON tags and observe JSON rules such as `omitempty` and `-`, or work as an independent validation tool.

For normal use, any item (`interface{}`) or pointer to such an item is passed in to be traversed and evaluated.

If you want to use this package for validation outside JSON *and* need to traverse private fields that are more complex than built-in data types, there is a separate function for this.  The call requires an _addressable_ reflection value to be passed in.  This is because otto requires a concrete interface{} to evaluate, and the only way to create an interface{} in these situations with complex private members is by using the _unsafe_ package with pointers.  While no data is ever modified by `tageval`, only use this mode if you really need it.  Perhaps try the standard API first and then fall back to this method.

The following declaration shows various legitimate tag formats.  Validation for virtually every Go type is supported, including channels, slices, arrays, etc., even if they would not be serialized by the Go JSON serializer.  For some types, such as channels, the user must define a custom type-mapper, but this works seamlessly with the API.

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
The API itself is simple.  The expressions used may be as complex as allowed by the JavaScript language and regexp language.  The expressions can be a simple comparison, such as `a < 7`, or an invocation of the functional API built into JavaScript.  Examples of both will be shown.  In fact, the field `Data` shown above uses a functional JavaScript expression to check the sum of the elements of an array.

As far as the API is concerned, the default mode is to evaluate an `interface{}` instance and run any validation tags encountered.  Also, by default, Go JSON serialization rules are obeyed.  This means private fields, fields tagged with `-`, or non-struct zero value fields are skipped.

### Quick look at expression evaluation
Here is a very simple example:
```
<imports here ...>
import "github.com/gdotgordon/tageval"

type MyStruct struct {
    A      int       `json:"a,omitempty" expr:"A > 5"`
}

func main() {
    ms1 := &MyStruct{4}
    v, err := tageval.NewValidator()
    if err != nil {
       fmt.Printf("initialization failed with: %v", err)
       os.Exit(1)
    }
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

In general, an expression uses the Go field's name (here "A") as the name of the variable.  For more complex data types, such as structs, any or all of the contained fields may be used to build an evaluation expression (Go's struct fields are mapped to JavaScript object properties, so fields are accessed as property lookups, an example of which will be shown later).  All expressions are compiled by _otto_ and then memoized by _tageval_ for faster execution.

Back to the sample above, the test shown here leads to an `ok` return value of `false`, meaning the validation failed, as the value of A is not greater than 5.  The second parameter, `res` is an object of type `[]Result`, which breaks down in detail the results of failures (or optionally both successes and failures) from the `Validate()` call.  This is extremely helpful for figuring out what went wrong, especially with multiple expressions, but may be ignored by assigning it to `_`.  Again, the `error` type is reserved for an execution error in the Validation, and not a validation failure.

One final note: for simple relational expressions such as `A > 5`, these may be abbreviated, here as `> 5`.  This works for expressions beginning with '<', '<=', '>', '>=', '==' and '!='.  This allows for a change of the field name without having to edit the expression, among other things.  So the example above could have been written as:

```
type MyStruct struct {
    A      int       `json:"a,omitempty" expr:"> 5"`
}
```

Now let's look at a slightly more complex example, a slice.  Assume the code from above, but change the struct definition and instance to be as follows:

```
type MyStruct struct {
    Vals []int `expr:"Vals.length >= 2 && Vals[0] > 2 && Vals[1] > 10"`
}

ms1 := MyStruct{[]int{5, 7, 32}}
```

This one fails as Vals[1] is 7, which is not > 10, and here is the `Result` item for this one:

`'Vals' (type: []int) item: '[5 7 32]', expr: 'Vals.length >= 2 && Vals[0] > 2 && Vals[1] > 10'  : failed`

With all the information in one place, the task of finding the problem should be simpler.

### Regexp
The pattern matching validation uses the `regexp` package in Go to determine whether the string matches.  It does not require a complete match to succeed, but if you require a complete match, start the regexp string with a '^' and terminate it with a '$'.  Getting the value to validate against the regexp is obvious for strings and objects implementing `fmt.Stringer()`, as well as all the `int` and `uint` types.  The type `bool` maps to "true" or "false", and all the other types use the default format (`%v`) from the fmt package.

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
v, rrr := tageval.NewValidator(tageval.Option{ShowSuccesses, true})
if err != nil {
    fmt.Printf("initialization failed with: %v", err)
    os.Exit(1)
}
ok, res, err := v.Validate(ms1)
...
```
Note we've added an option to return successes in the list, as well as failures.  Options will be covered in the next section.  Also note that unlike JavaScript expressions, we don't need to specify the variable name in the expression.  So running this expression returns `true` for `ok`, and the detailed result is:

`'Spec' (type: SpecialInt) item: 'I'm special, my value is: -56', expr: '^.*: [-]?[0-9]+$'  : ok`

## Options
We saw the option to include successes in addition to failures above.  The `NewValidator()` function is _variadic_  with the signature: `func NewValidator(options ...Option) *Validator`.  An option takes a name and value as follows:

```
type Option struct {
    Name  string
    Value interface{}
}
```

The options currently supported are:
* ProcessAsJSON - a `bool` which says to obey the JSON rules, as explained above, with default of true.  You'd set this to false if you want to validate every field, regardless of whether it would be serialized to JSON.
* ShowSuccesses - by default, only failures are returned in the `[]Result`.  Setting this to `true` shows successes and failures.

## JavaScript Mappings and Debugging Tips
The biggest source of confusion is likely to be in the mappings performed from Go to JavaScript by _otto_.  As mentioned, Go structs and slices generally map to JavaScript Objects, meaning they have property maps.  Slices become Objects with members indexed by offset, and structs map to Objects indexed by struct member name.  For example, consider the following structs and note how the field names of the inner struct may be accessed to do a validation on the entire struct from the outer struct:

```
type Inner struct {
    Name     string `json:"name"`
    Location string `json:"location"`
}

type Outer struct {
    G      Inner `expr:"G[\"Name\"].length > 2 && G[\"Location\"] == \"Oshkosh, WI\""`
}
```

One way to debug situations such as this, when you are not certain of the mappings, is to dump the results using the JavaScript function `console.log()`, as in:

```
type Outer struct {
    G      Inner `expr:"console.log('Fields of G:' + Object.keys(G)) "`
}
```
In the example above, you could use a ";" to still include your validation expression after the console log.  In general, an expression can consist of multiple ";" statements.

## More Detailed Use Cases
Please see the unit tests for some more advanced examples and ideas.  One interesting concept is how a struct member that is an Interface is handled with regard to its concrete value.

## Concurrency
As the _otto_ JavaScript engine does not support concurrent access (as per the documentation), you should create a `Validator` for each concurrently running goroutine.  These items are low overhead.  The other design alternative is for _tageval_ to add mutexes around each JavaScript evaluation, and this, along with other alternatives, are currently being explored.

