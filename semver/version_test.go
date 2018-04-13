package semver_test

import (
	"fmt"
	"github.com/puppetlabs/go-semver/semver"
)

func ExampleParseVersion() {
	v, err := semver.ParseVersion(`1.0.0`)
	if err == nil {
		fmt.Println(v)
	} else {
		fmt.Println(err)
	}
	// Output:
	// 1.0.0
}

func ExampleVersion_NextPatch() {
	v, err := semver.ParseVersion(`1.0.0`)
	if err == nil {
		fmt.Println(v)
		fmt.Println(v.NextPatch())
	} else {
		fmt.Println(err)
	}
	// Output:
	// 1.0.0
	// 1.0.1
}

func ExampleVersion_ToStable() {
	v, err := semver.ParseVersion(`1.0.0-rc1`)
	if err == nil {
		fmt.Println(v)
		fmt.Println(v.ToStable())
	} else {
		fmt.Println(err)
	}
	// Output:
	// 1.0.0-rc1
	// 1.0.0
}
