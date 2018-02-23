package tageval_test

import (
	"fmt"
	"os"

	"github.com/gdotgordon/tageval"
)

func Example() {
	type MyStruct struct {
		Total int    `json:"total,omitempty" expr:"Total > 5"`
		State string `regexp:"^[A-Z]{2}$"`
	}

	ms1 := &MyStruct{4, "ARK"}
	v, err := tageval.NewValidator()
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialization failed with: %v", err)
		os.Exit(1)
	}
	ok, res, err := v.Validate(ms1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validation failed with error: %v", err)
		os.Exit(1)
	}
	if ok {
		fmt.Fprintf(os.Stderr, "unexpected success result")
		os.Exit(1)
	}
	tageval.PrintResults(os.Stdout, res)
	// Output:
	// Results:
	// 'Total' (type: int) item: '4', expr: 'Total > 5' : failed
	// 'State' (type: string) item: 'ARK', expr: '^[A-Z]{2}$' : failed

}

func ExampleValidator_ValidateAddressable() {
	type privy struct {
		things [2]int `expr:"things[0] == 37 && Math.sqrt(things[1]) == 9"`
	}
	p := privy{[2]int{37, 81}}

	// Note Options passed to NewValidator().
	v, err := tageval.NewValidator(
		tageval.Option{tageval.ShowSuccesses, true},
		tageval.Option{tageval.ProcessAsJSON, false})
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialization failed with: %v", err)
		os.Exit(1)
	}

	// Call to "Validate()" won't work due to private access.
	_, _, err = v.Validate(&p)
	if err == nil {
		fmt.Fprintf(os.Stderr, "expected error on private field access")
		os.Exit(1)
	}
	fmt.Printf("Non-addressable received error: %v\n", err)

	// Must call "ValidateAddressable()" for private access.
	fmt.Println("Adressable case:")
	ok, res, err := v.ValidateAddressable(&p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validation failed with error: %v", err)
		os.Exit(1)
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "unexpected failure result")
		os.Exit(1)
	}
	tageval.PrintResults(os.Stdout, res)
	// Output:
	// Non-addressable received error: cannot access private field: 'things'
	// Adressable case:
	// Results:
	// 'things' (type: [2]int) item: '[37 81]', expr: 'things[0] == 37 && Math.sqrt(things[1]) == 9' : ok
}
