/*
Package tageval validates Go struct fields via either JavaScript expressions
or regular expressions.

This package implements per-field validation for Go struct members using
custom field tags. It attempts to provide a flexible and extensible API,
leveraging a complete standard scripting language, the ECMA 5 version of
JavaScript. This is accomplished using the Go-based otto embedded JavaScript
implementation at https://github.com/robertkrimen/otto. The mapping from Go
types to JavaScript is generally straightforward, and where more usable
bindings are required, the current tageval API adds the capability to define
custom Go -> JavaScript bindings to be used with otto, by defining custom type
mapping functions in Go.

The package also supports standard Go regexp validation for fields where a
string value can be ascertained (including items that implement the
fmt.Stringer interface and more). It would be easy to adapt the API to handle
additional types of validation, but the two modes currently supported provide
a good balance between standard Go mechanisms and the power of a rich and
easy-to-use scripting language.
*/
package tageval
