package semver_test

import (
	"fmt"

	"github.com/lyraproj/semver/semver"
)

func ExampleParseVersionRange() {
	rng, err := semver.ParseVersionRange(`1.x`)
	if err == nil {
		fmt.Println(rng)
		fmt.Println(rng.NormalizedString())
	} else {
		fmt.Println(err)
	}
	// Output:
	// 1.x
	// >=1.0.0 <2.0.0
}

func ExampleMatchAll() {
	rng, err := semver.ParseVersionRange(`*`)
	if err == nil {
		fmt.Println(rng)
		fmt.Println(rng.NormalizedString())
		fmt.Println(rng.Includes(semver.Min))
		fmt.Println(rng.Includes(semver.MustParseVersion(`1.2.3-rc1`)))
	} else {
		fmt.Println(err)
	}
	// Output:
	// *
	// >=0.0.0-
	// true
	// true
}
