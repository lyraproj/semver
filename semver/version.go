package semver

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// A Version represents a version as specified in "Semantic Versioning 2.0". The document
// can be found at https://semver.org
type Version interface {
	fmt.Stringer

	// CompareTo compares the receiver to another version. Return zero if the versions are equal,
	// a negative integer if the receiver is less than the given version, and a positive
	// integer if the receiver is greater than the given version.
	//
	// The build suffix is not included in the comparison.
	CompareTo(Version) int

	// Equals tests if the receiver is equal to another version.
	//
	// In contrast to CompareTo, this method will include build prefixes in the comparison.
	Equals(other Version) bool

	// TripletEquals returns true if the major, minor, and patch numbers are equal.
	TripletEquals(ov Version) bool

	// IsStable returns true when the version has no pre-release suffix.
	IsStable() bool

	// Major returns the major version number
	Major() int

	// Minor returns the minor version number
	Minor() int

	// Patch returns the patch version number
	Patch() int

	// PreRelease returns the pre-release suffix
	PreRelease() string

	// Build returns the pre-release suffix
	Build() string

	// NextPatch returns a copy of this version where the patch number is
	// incremented by one and the pre-release and build suffixes are stripped
	// off.
	NextPatch() Version

	// ToStable returs a copy of this version where the pre-release and build
	// suffixes are stripped off.
	ToStable() Version

	// ToString writes the string representation of this version onto the given
	// Writer.
	ToString(io.Writer)
}

type version struct {
	major      int
	minor      int
	patch      int
	preRelease []interface{}
	build      []interface{}
}

var minPrereleases []interface{}

var vPRPart = `(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-]+[0-9A-Za-z-]*)`
var vPRParts = vPRPart + `(?:\.` + vPRPart + `)*`
var vPart = `[0-9A-Za-z-]+`
var vParts = vPart + `(?:\.` + vPart + `)*`
var vPrerelase = `(?:-(` + vPRParts + `))?`
var vBuild = `(?:\+(` + vParts + `))?`
var vQualifier = vPrerelase + vBuild
var vNR = `(0|[1-9][0-9]*)`

var vPRPartsPattern = regexp.MustCompile(`\A` + vPRParts + `\z`)
var vPartsPattern = regexp.MustCompile(`\A` + vParts + `\z`)

var Max Version = &version{math.MaxInt64, math.MaxInt64, math.MaxInt64, nil, nil}
var Min = &version{0, 0, 0, minPrereleases, nil}
var Zero = &version{0, 0, 0, nil, nil}
var VersionPattern = regexp.MustCompile(`\A` + vNR + `\.` + vNR + `\.` + vNR + vQualifier + `\z`)

func NewVersion(major, minor, patch int) (Version, error) {
	return NewVersion3(major, minor, patch, ``, ``)
}

func NewVersion2(major, minor, patch int, preRelease string) (Version, error) {
	return NewVersion3(major, minor, patch, preRelease, ``)
}

func NewVersion3(major, minor, patch int, preRelease string, build string) (Version, error) {
	if major < 0 || minor < 0 || patch < 0 {
		return nil, fmt.Errorf(`negative numbers not accepted in version`)
	}
	ps, err := splitParts(`pre-release`, preRelease, true)
	if err != nil {
		return nil, err
	}
	bs, err := splitParts(`build`, build, false)
	if err != nil {
		return nil, err
	}
	return &version{major, minor, patch, ps, bs}, nil
}

func MustParseVersion(str string) Version {
	v, err := ParseVersion(str)
	if err != nil {
		panic(err)
	}
	return v
}

func ParseVersion(str string) (version Version, err error) {
	if group := VersionPattern.FindStringSubmatch(str); group != nil {
		major, _ := strconv.Atoi(group[1])
		minor, _ := strconv.Atoi(group[2])
		patch, _ := strconv.Atoi(group[3])
		return NewVersion3(major, minor, patch, group[4], group[5])
	}
	return nil, fmt.Errorf(`the string '%s' does not represent a valid semantic version`, str)
}

func (v *version) Build() string {
	if v.build == nil {
		return ``
	}
	bld := bytes.NewBufferString(``)
	writeParts(v.build, bld)
	return bld.String()
}

func (v *version) CompareTo(other Version) int {
	o := other.(*version)
	cmp := v.major - o.major
	if cmp == 0 {
		cmp = v.minor - o.minor
		if cmp == 0 {
			cmp = v.patch - o.patch
			if cmp == 0 {
				cmp = comparePreReleases(v.preRelease, o.preRelease)
			}
		}
	}
	return cmp
}

func (v *version) Equals(other Version) bool {
	ov := other.(*version)
	return v.tripletEquals(ov) && equalSegments(v.preRelease, ov.preRelease) && equalSegments(v.build, ov.build)
}

func (v *version) IsStable() bool {
	return v.preRelease == nil
}

func (v *version) Major() int {
	return v.major
}

func (v *version) Minor() int {
	return v.minor
}

func (v *version) NextPatch() Version {
	return &version{v.major, v.minor, v.patch + 1, nil, nil}
}

func (v *version) Patch() int {
	return v.patch
}

func (v *version) PreRelease() string {
	if v.preRelease == nil {
		return ``
	}
	bld := bytes.NewBufferString(``)
	writeParts(v.preRelease, bld)
	return bld.String()
}

func (v *version) String() string {
	bld := bytes.NewBufferString(``)
	v.ToString(bld)
	return bld.String()
}

func (v *version) ToStable() Version {
	return &version{v.major, v.minor, v.patch, nil, v.build}
}

func (v *version) ToString(bld io.Writer) {
	fmt.Fprintf(bld, `%d.%d.%d`, v.major, v.minor, v.patch)
	if v.preRelease != nil {
		bld.Write([]byte(`-`))
		writeParts(v.preRelease, bld)
	}
	if v.build != nil {
		bld.Write([]byte(`+`))
		writeParts(v.build, bld)
	}
}

func (v *version) TripletEquals(other Version) bool {
	return v.tripletEquals(other.(*version))
}

func (v *version) tripletEquals(ov *version) bool {
	return v.major == ov.major && v.minor == ov.minor && v.patch == ov.patch
}

func writeParts(parts []interface{}, bld io.Writer) {
	top := len(parts)
	if top > 0 {
		fmt.Fprintf(bld, `%v`, parts[0])
		for idx := 1; idx < top; idx++ {
			bld.Write([]byte(`.`))
			fmt.Fprintf(bld, `%v`, parts[idx])
		}
	}
}

func comparePreReleases(p1, p2 []interface{}) int {
	if p1 == nil {
		if p2 == nil {
			return 0
		}
		return 1
	}
	if p2 == nil {
		return -1
	}

	p1Size := len(p1)
	p2Size := len(p2)
	commonMax := p1Size
	if p1Size > p2Size {
		commonMax = p2Size
	}
	for idx := 0; idx < commonMax; idx++ {
		v1 := p1[idx]
		v2 := p2[idx]
		if i1, ok := v1.(int); ok {
			if i2, ok := v2.(int); ok {
				cmp := i1 - i2
				if cmp != 0 {
					return cmp
				}
				continue
			}
			return -1
		}

		if _, ok := v2.(int); ok {
			return 1
		}

		cmp := strings.Compare(v1.(string), v2.(string))
		if cmp != 0 {
			return cmp
		}
	}
	return p1Size - p2Size
}

func equalSegments(a, b []interface{}) bool {
	if a == nil {
		if b == nil {
			return true
		}
		return false
	}
	top := len(a)
	if b == nil || top != len(b) {
		return false
	}
	for idx := 0; idx < top; idx++ {
		if a[idx] != b[idx] {
			return false
		}
	}
	return true
}

func mungePart(part string) interface{} {
	if i, err := strconv.ParseInt(part, 10, 64); err == nil {
		return int(i)
	}
	return part
}

func splitParts(tag, str string, stringToInt bool) ([]interface{}, error) {
	if str == `` {
		return nil, nil
	}

	pattern := vPartsPattern
	if stringToInt {
		pattern = vPRPartsPattern
	}
	if !pattern.MatchString(str) {
		return nil, fmt.Errorf(`Illegal characters in %s`, tag)
	}

	parts := strings.Split(str, `.`)
	result := make([]interface{}, len(parts))
	for idx, sp := range parts {
		if stringToInt {
			result[idx] = mungePart(sp)
		} else {
			result[idx] = sp
		}
	}
	return result, nil
}
