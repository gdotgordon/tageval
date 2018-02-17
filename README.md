# tageval
Package to validate Go struct fields via either JavaScript expressions or regular expressions.

This package implements per-field validation for Go struct members using custom field tags.  While this is not the first package to provide this, it attempts to provide a flexible and extensible API, leveraging a complete standard scripting language, the ECMA 5 version of JavaScript.  This is accomplished using the Go-based _otto_ embedded JavaScript implmentation at <https://github.com/robertkrimen/otto>.  The mapping from Go types to JavaScript is generally straightforward, and where it is less so, the current _tageval_ API adds the capability to define custom Go -> JavaScript bindings to be used with _otto_, by defining custom type mapping functions in Go.

The package also supports standard Go `regexp` validation for fields where a string value can be ascertained (including items that implement the `fmt.Stringer` interface.  It would be easy to adapt the API to hndle additional types of validation, but the two currently supported provide a good balanace between standard Go mechanisms and the power of a rich and easy-to-use programming language.

## How Does it Work?
As mentioned, additional Go struct field tags are introdiuced, `eval` and `regexp`.  These are designed to either work alongside with JSON tgas and observe JSON rules such as `omitempty` and `-`, or work as an independent validation tool.

The following example is mostly to show how the tags look.  Validation for virtaully every Go type is supported, including channels, slices, arrays, etc., even if they would not be serialzed by the Go JSON serializer.
```type MyStruct struct {
  FirstName string ``json:"my_name" expr:"MyName.length<10" regexp:"^\p{L}.*$``
  LastName string  ``json:"last_name" expr:"LastName.length<10"``
  City string      ``expr:"City.length<10"``
  State string     ``json:"state" regexp:"[A-Z]{2}"``
  Data []int       ``expr:"var sum = P.reduce(function(pv, cv) { return pv + cv; }, 0); sum == 10"
 }```

<Work-in-progress - to be continued ...>
