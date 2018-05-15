package semver

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
)

// A VersionRange represents a range of semantic versions. It conforms to the specification
// used for npm. See https://docs.npmjs.com/misc/semver for a full description
type VersionRange interface {
	fmt.Stringer
	// EndVersion returns the ending version in the range if that is possible to determine, or nil otherwise
	EndVersion() Version

	// Equals compares the receiver to another range and returns true if the ranges are equal
	Equals(VersionRange) bool

	// Includes returns true if the given version is included in the receiver range
	Includes(v Version) bool

	// Intersection returns a new range that is the intersection of the receiver and the given range
	Intersection(other VersionRange) VersionRange

	// IsAsRestrictiveAs returns true if the receiver is equally or more restrictive than the given range
	IsAsRestrictiveAs(other VersionRange) bool

	// IsExcludeEnd returns true unless the end version is included in the range
	IsExcludeEnd() bool

	// IsExcludeStart returns true unless the start version is included in the range
	IsExcludeStart() bool

	// Merge returns a new range that will includes all versions included by the receiver
	// plus all versions included by the given range
	Merge(or VersionRange) VersionRange

	// NormalizedString returns the canonical string representation of this range. E.g.
	//
	// "2.x" normalized becomes ">=2.0.0 <3.0.0"
	NormalizedString() string

	// StartVersion returns the starting version in the range if that is possible to determine, or nil otherwise
	StartVersion() Version

	// ToNormalizedString writes the normalized string onto the given Writer
	ToNormalizedString(bld io.Writer)

	// ToString writes the string representation of this range onto the given writer
	ToString(bld io.Writer)
}

type abstractRange interface {
		asLowerBound() abstractRange
		asUpperBound() abstractRange
		equals(or abstractRange) bool
		includes(v Version) bool
		isAbove(v Version) bool
		isBelow(v Version) bool
		isExcludeStart() bool
		isExcludeEnd() bool
		isLowerBound() bool
		isUpperBound() bool
		start() Version
		end() Version
		testPrerelease(v Version) bool
		ToString(bld io.Writer)
	}

type simpleRange struct {
		Version
	}

type startEndRange struct {
		startCompare abstractRange
		endCompare   abstractRange
	}

type eqRange struct {
		simpleRange
	}

type gtRange struct {
		simpleRange
	}

type gtEqRange struct {
		simpleRange
	}

type ltRange struct {
		simpleRange
	}

type ltEqRange struct {
		simpleRange
	}

type versionRange struct {
	originalString string
	ranges         []abstractRange
}


var nr = `0|[1-9][0-9]*`
var xr = `(x|X|\*|` + nr + `)`

var part = `(?:[0-9A-Za-z-]+)`
var parts = part + `(?:\.` + part + `)*`
var qualifier = `(?:-(` + parts + `))?(?:\+(` + parts + `))?`

var partial = xr + `(?:\.` + xr + `(?:\.` + xr + qualifier + `)?)?`

var simple = `([<>=~^]|<=|>=|~>|~=)?(?:` + partial + `)`
var simplePattern = regexp.MustCompile(`\A` + simple + `\z`)

var orSplit = regexp.MustCompile(`\s*\|\|\s*`)
var simpleSplit = regexp.MustCompile(`\s+`)

var opWsPattern = regexp.MustCompile(`([><=~^])(?:\s+|\s*v)`)

var hyphen = `(?:` + partial + `)\s+-\s+(?:` + partial + `)`
var hyphenPattern = regexp.MustCompile(`\A` + hyphen + `\z`)

var highestLb = &gtRange{simpleRange{Max}}
var lowestLb = &gtEqRange{simpleRange{Min}}
var lowestUb = &ltRange{simpleRange{Min}}

var MatchAll VersionRange = &versionRange{`*`, []abstractRange{lowestLb}}
var MatchNone VersionRange = &versionRange{`<0.0.0`, []abstractRange{lowestUb}}

func ExactVersionRange(v Version) VersionRange {
	return &versionRange{``, []abstractRange{&eqRange{simpleRange{v}}}}
}

func FromVersions(start Version, excludeStart bool, end Version, excludeEnd bool) VersionRange {
	var as abstractRange
	if excludeStart {
		as = &gtRange{simpleRange{start}}
	} else {
		as = &gtEqRange{simpleRange{start}}
	}
	var ae abstractRange
	if excludeEnd {
		ae = &ltRange{simpleRange{end}}
	} else {
		ae = &ltEqRange{simpleRange{end}}
	}
	return newVersionRange(``, []abstractRange{as, ae})
}

func MustParseVersionRange(str string) VersionRange {
	v, err := ParseVersionRange(str)
	if err != nil {
		panic(err)
	}
	return v
}

func ParseVersionRange(vr string) (result VersionRange, err error) {
	if vr == `` {
		return nil, nil
	}

	vr = opWsPattern.ReplaceAllString(vr, `$1`)
	rangeStrings := orSplit.Split(vr, -1)
	ranges := make([]abstractRange, 0, len(rangeStrings))
	if len(rangeStrings) == 0 {
		return nil, fmt.Errorf(`'%s' is not a valid version range`, vr)
	}
	for _, rangeStr := range rangeStrings {
		if rangeStr == `` {
			ranges = append(ranges, lowestLb)
			continue
		}

		if m := hyphenPattern.FindStringSubmatch(rangeStr); m != nil {
			e1, err := createGtEqRange(m, 1)
			if err != nil {
				return nil, err
			}
			e2, err := createGtEqRange(m, 6)
			if err != nil {
				return nil, err
			}
			ranges = append(ranges, intersection(e1, e2))
			continue
		}

		var simpleRange abstractRange
		for _, simple := range simpleSplit.Split(rangeStr, -1) {
			m := simplePattern.FindStringSubmatch(simple)
			if m == nil {
				return nil, fmt.Errorf(`'%s' is not a valid version range`, simple)
			}
			var rng abstractRange
			var err error
			switch m[1] {
			case `~`, `~>`:
				rng, err = createTildeRange(m, 2)
			case `^`:
				rng, err = createCaretRange(m, 2)
			case `>`:
				rng, err = createGtRange(m, 2)
			case `>=`:
				rng, err = createGtEqRange(m, 2)
			case `<`:
				rng, err = createLtRange(m, 2)
			case `<=`:
				rng, err = createLtEqRange(m, 2)
			default:
				rng, err = createXRange(m, 2)
			}
			if err != nil {
				return nil, err
			}
			if simpleRange == nil {
				simpleRange = rng
			} else {
				simpleRange = intersection(simpleRange, rng)
			}
		}
		if simpleRange != nil {
			ranges = append(ranges, simpleRange)
		}
	}
	return newVersionRange(vr, ranges), nil
}

func (r *versionRange) EndVersion() Version {
	if len(r.ranges) == 1 {
		return r.ranges[0].end()
	}
	return nil
}

func (r *versionRange) Equals(other VersionRange) bool {
	or := other.(*versionRange)
	top := len(r.ranges)
	if top != len(or.ranges) {
		return false
	}
	for idx, ar := range r.ranges {
		if !ar.equals(or.ranges[idx]) {
			return false
		}
	}
	return true
}

func (r *versionRange) Includes(v Version) bool {
	if v != nil {
		for _, ar := range r.ranges {
			if ar.includes(v) && (v.IsStable() || ar.testPrerelease(v)) {
				return true
			}
		}
	}
	return false
}

func (r *versionRange) Intersection(other VersionRange) VersionRange {
	if other != nil {
		or := other.(*versionRange)
		iscs := make([]abstractRange, 0)
		for _, ar := range r.ranges {
			for _, ao := range or.ranges {
				is := intersection(ar, ao)
				if is != nil {
					iscs = append(iscs, is)
				}
			}
		}
		if len(iscs) > 0 {
			return newVersionRange(``, iscs)
		}
	}
	return nil
}

func (r *versionRange) IsAsRestrictiveAs(other VersionRange) bool {
arNext:
	for _, ar := range r.ranges {
		for _, ao := range other.(*versionRange).ranges {
			is := intersection(ar, ao)
			if is != nil && asRestrictedAs(ar, ao) {
				continue arNext
			}
		}
		return false
	}
	return true
}

func (r *versionRange) IsExcludeEnd() bool {
	if len(r.ranges) == 1 {
		return r.ranges[0].isExcludeEnd()
	}
	return false
}

func (r *versionRange) IsExcludeStart() bool {
	if len(r.ranges) == 1 {
		return r.ranges[0].isExcludeStart()
	}
	return false
}

func (r *versionRange) Merge(or VersionRange) VersionRange {
	return newVersionRange(``, append(r.ranges, or.(*versionRange).ranges...))
}

func (r *versionRange) NormalizedString() string {
	bld := bytes.NewBufferString(``)
	r.ToNormalizedString(bld)
	return bld.String()
}

func (r *versionRange) StartVersion() Version {
	if len(r.ranges) == 1 {
		return r.ranges[0].start()
	}
	return nil
}

func (r *versionRange) String() string {
	bld := bytes.NewBufferString(``)
	r.ToString(bld)
	return bld.String()
}

func (r *versionRange) ToNormalizedString(bld io.Writer) {
	top := len(r.ranges)
	r.ranges[0].ToString(bld)
	for idx := 1; idx < top; idx++ {
		io.WriteString(bld, ` || `)
		r.ranges[idx].ToString(bld)
	}
}

func (r *versionRange) ToString(bld io.Writer) {
	if r.originalString == `` {
		r.ToNormalizedString(bld)
	} else {
		io.WriteString(bld, r.originalString)
	}
}

func newVersionRange(vr string, ranges []abstractRange) VersionRange {
	mergeHappened := true
	for len(ranges) > 1 && mergeHappened {
		mergeHappened = false
		result := make([]abstractRange, 0)
		for len(ranges) > 1 {
			unmerged := make([]abstractRange, 0)
			ln := len(ranges) - 1
			x := ranges[ln]
			ranges = ranges[:ln]
			for _, y := range ranges {
				merged := union(x, y)
				if merged == nil {
					unmerged = append(unmerged, y)
				} else {
					mergeHappened = true
					x = merged
				}
			}
			result = append([]abstractRange{x}, result...)
			ranges = unmerged
		}
		if len(ranges) > 0 {
			result = append(ranges, result...)
		}
		ranges = result
	}
	if len(ranges) == 0 {
		return MatchNone
	}
	return &versionRange{vr, ranges}
}

func createGtEqRange(rxGroup []string, startInMatcher int) (abstractRange, error) {
	major, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return lowestLb, nil
	}
	startInMatcher++
	minor, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		minor = 0
	}
	startInMatcher++
	patch, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		patch = 0
	}
	startInMatcher++
	preRelease := rxGroup[startInMatcher]
	startInMatcher++
	build := rxGroup[startInMatcher]
	v, err := NewVersion3(major, minor, patch, preRelease, build)
	if err != nil {
		return nil, err
	}
	return &gtEqRange{simpleRange{v}}, nil
}

func createGtRange(rxGroup []string, startInMatcher int) (abstractRange, error) {
	major, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return lowestLb, nil
	}
	startInMatcher++
	minor, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return &gtEqRange{simpleRange{&version{major + 1, 0, 0, nil, nil}}}, nil
	}
	startInMatcher++
	patch, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return &gtEqRange{simpleRange{&version{major, minor + 1, 0, nil, nil}}}, nil
	}
	startInMatcher++
	preRelease := rxGroup[startInMatcher]
	startInMatcher++
	build := rxGroup[startInMatcher]
	v, err := NewVersion3(major, minor, patch, preRelease, build)
	if err != nil {
		return nil, err
	}
	return &gtRange{simpleRange{v}}, nil
}

func createLtEqRange(rxGroup []string, startInMatcher int) (abstractRange, error) {
	major, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return lowestUb, nil
	}
	startInMatcher++
	minor, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return &ltRange{simpleRange{&version{major + 1, 0, 0, nil, nil}}}, nil
	}
	startInMatcher++
	patch, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return &ltRange{simpleRange{&version{major, minor + 1, 0, nil, nil}}}, nil
	}
	startInMatcher++
	preRelease := rxGroup[startInMatcher]
	startInMatcher++
	build := rxGroup[startInMatcher]
	v, err := NewVersion3(major, minor, patch, preRelease, build)
	if err != nil {
		return nil, err
	}
	return &ltEqRange{simpleRange{v}}, nil
}

func createLtRange(rxGroup []string, startInMatcher int) (abstractRange, error) {
	major, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return lowestUb, nil
	}
	startInMatcher++
	minor, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		minor = 0
	}
	startInMatcher++
	patch, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		patch = 0
	}
	startInMatcher++
	preRelease := rxGroup[startInMatcher]
	startInMatcher++
	build := rxGroup[startInMatcher]
	v, err := NewVersion3(major, minor, patch, preRelease, build)
	if err != nil {
		return nil, err
	}
	return &ltRange{simpleRange{v}}, nil
}

func createTildeRange(rxGroup []string, startInMatcher int) (abstractRange, error) {
	return allowPatchUpdates(rxGroup, startInMatcher, true)
}

func createCaretRange(rxGroup []string, startInMatcher int)  (abstractRange, error) {
	major, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return lowestLb, nil
	}
	if major == 0 {
		return allowPatchUpdates(rxGroup, startInMatcher, true)
	}
	startInMatcher++
	return allowMinorUpdates(rxGroup, major, startInMatcher)
}

func createXRange(rxGroup []string, startInMatcher int)  (abstractRange, error) {
	return allowPatchUpdates(rxGroup, startInMatcher, false)
}

func allowPatchUpdates(rxGroup []string, startInMatcher int, tildeOrCaret bool) (abstractRange, error) {
	major, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return lowestLb, nil
	}
	startInMatcher++
	minor, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return &startEndRange{
			&gtEqRange{simpleRange{&version{major, 0, 0, nil, nil}}},
			&ltRange{simpleRange{&version{major + 1, 0, 0, nil, nil}}}}, nil
	}
	startInMatcher++
	patch, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		return &startEndRange{
			&gtEqRange{simpleRange{&version{major, minor, 0, nil, nil}}},
			&ltRange{simpleRange{&version{major, minor + 1, 0, nil, nil}}}}, nil
	}
	startInMatcher++
	preRelease := rxGroup[startInMatcher]
	startInMatcher++
	build := rxGroup[startInMatcher]
	v, err := NewVersion3(major, minor, patch, preRelease, build)
	if err != nil {
		return nil, err
	}
	if tildeOrCaret {
		return &startEndRange{
			&gtEqRange{simpleRange{v}},
			&ltRange{simpleRange{&version{major, minor + 1, 0, nil, nil}}}}, nil
	}
	return &eqRange{simpleRange{v}}, nil
}

func allowMinorUpdates(rxGroup []string, major int, startInMatcher int) (abstractRange, error) {
	minor, ok, err := xDigit(rxGroup[startInMatcher])
	if !ok {
		minor = 0
	}
	startInMatcher++
	patch, ok, err := xDigit(rxGroup[startInMatcher])
	if err != nil {
		return nil, err
	}
	if !ok {
		patch = 0
	}
	startInMatcher++
	preRelease := rxGroup[startInMatcher]
	startInMatcher++
	build := rxGroup[startInMatcher]
	v, err := NewVersion3(major, minor, patch, preRelease, build)
	if err != nil {
		return nil, err
	}
	return &startEndRange{
		&gtEqRange{simpleRange{v}},
		&ltRange{simpleRange{&version{major + 1, 0, 0, nil, nil}}}}, nil
}

func xDigit(str string) (int, bool, error) {
	if str == `` || str == `x` || str == `X` || str == `*` {
		return 0, false, nil
	}
	if i, err := strconv.ParseInt(str, 10, 64); err == nil {
		return int(i), true, nil
	}
	return 0, false, fmt.Errorf(`illegal version triplet`)
}

func isOverlap(ra, rb abstractRange) bool {
	cmp := ra.start().CompareTo(rb.end())
	if cmp < 0 || cmp == 0 && !(ra.isExcludeStart() || rb.isExcludeEnd()) {
		cmp := rb.start().CompareTo(ra.end())
		return cmp < 0 || cmp == 0 && !(rb.isExcludeStart() || ra.isExcludeEnd())
	}
	return false
}

func asRestrictedAs(ra, vr abstractRange) bool {
	cmp := vr.start().CompareTo(ra.start())
	if cmp > 0 || (cmp == 0 && !ra.isExcludeStart() && vr.isExcludeStart()) {
		return false
	}

	cmp = vr.end().CompareTo(ra.end())
	return !(cmp < 0 || (cmp == 0 && !ra.isExcludeEnd() && vr.isExcludeEnd()))
}

func intersection(ra, rb abstractRange) abstractRange {
	cmp := ra.start().CompareTo(rb.end())
	if cmp > 0 {
		return nil
	}

	if cmp == 0 {
		if ra.isExcludeStart() || rb.isExcludeEnd() {
			return nil
		}
		return &eqRange{simpleRange{ra.start()}}
	}

	cmp = rb.start().CompareTo(ra.end())
	if cmp > 0 {
		return nil
	}

	if cmp == 0 {
		if rb.isExcludeStart() || ra.isExcludeEnd() {
			return nil
		}
		return &eqRange{simpleRange{rb.start()}}
	}

	cmp = ra.start().CompareTo(rb.start())
	var start abstractRange
	if cmp < 0 {
		start = rb
	} else if cmp > 0 {
		start = ra
	} else if ra.isExcludeStart() {
		start = ra
	} else {
		start = rb
	}

	cmp = ra.end().CompareTo(rb.end())
	var end abstractRange
	if cmp > 0 {
		end = rb
	} else if cmp < 0 {
		end = ra
	} else if ra.isExcludeEnd() {
		end = ra
	} else {
		end = rb
	}

	if !end.isUpperBound() {
		return start
	}

	if !start.isLowerBound() {
		return end
	}

	return &startEndRange{start.asLowerBound(), end.asUpperBound()}
}

func fromTo(ra, rb abstractRange) abstractRange {
	var startR abstractRange
	if ra.isExcludeStart() {
		startR = &gtRange{simpleRange{ra.start()}}
	} else {
		startR = &gtEqRange{simpleRange{ra.start()}}
	}
	var endR abstractRange
	if rb.isExcludeEnd() {
		endR = &ltRange{simpleRange{rb.end()}}
	} else {
		endR = &ltEqRange{simpleRange{rb.end()}}
	}
	return &startEndRange{startR, endR}
}

func union(ra, rb abstractRange) abstractRange {
	if ra.includes(rb.start()) || rb.includes(ra.start()) {
		var start Version
		var excludeStart bool
		cmp := ra.start().CompareTo(rb.start())
		if cmp < 0 {
			start = ra.start()
			excludeStart = ra.isExcludeStart()
		} else if cmp > 0 {
			start = rb.start()
			excludeStart = rb.isExcludeStart()
		} else {
			start = ra.start()
			excludeStart = ra.isExcludeStart() && rb.isExcludeStart()
		}

		var end Version
		var excludeEnd bool
		cmp = ra.end().CompareTo(rb.end())
		if cmp > 0 {
			end = ra.end()
			excludeEnd = ra.isExcludeEnd()
		} else if cmp < 0 {
			end = rb.end()
			excludeEnd = rb.isExcludeEnd()
		} else {
			end = ra.end()
			excludeEnd = ra.isExcludeEnd() && rb.isExcludeEnd()
		}

		var startR abstractRange
		if excludeStart {
			startR = &gtRange{simpleRange{start}}
		} else {
			startR = &gtEqRange{simpleRange{start}}
		}
		var endR abstractRange
		if excludeEnd {
			endR = &ltRange{simpleRange{end}}
		} else {
			endR = &ltEqRange{simpleRange{end}}
		}
		return &startEndRange{startR, endR}
	}
	if ra.isExcludeStart() && rb.isExcludeStart() && ra.start().CompareTo(rb.start()) == 0 {
		return fromTo(ra, rb)
	}
	if ra.isExcludeEnd() && !rb.isExcludeStart() && ra.end().CompareTo(rb.start()) == 0 {
		return fromTo(ra, rb)
	}
	if rb.isExcludeEnd() && !ra.isExcludeStart() && rb.end().CompareTo(ra.start()) == 0 {
		return fromTo(rb, ra)
	}
	if !ra.isExcludeEnd() && !rb.isExcludeStart() && ra.end().NextPatch().CompareTo(rb.start()) == 0 {
		return fromTo(ra, rb)
	}
	if !rb.isExcludeEnd() && !ra.isExcludeStart() && rb.end().NextPatch().CompareTo(ra.start()) == 0 {
		return fromTo(rb, ra)
	}
	return nil
}

func (r *startEndRange) asLowerBound() abstractRange {
	return r.startCompare
}

func (r *startEndRange) asUpperBound() abstractRange {
	return r.endCompare
}

func (r *startEndRange) equals(o abstractRange) bool {
	if or, ok := o.(*startEndRange); ok {
		return r.startCompare.equals(or.startCompare) && r.endCompare.equals(or.endCompare)
	}
	return false
}

func (r *startEndRange) includes(v Version) bool {
	return r.startCompare.includes(v) && r.endCompare.includes(v)
}

func (r *startEndRange) isAbove(v Version) bool {
	return r.startCompare.isAbove(v)
}

func (r *startEndRange) isBelow(v Version) bool {
	return r.endCompare.isBelow(v)
}

func (r *startEndRange) isExcludeStart() bool {
	return r.startCompare.isExcludeStart()
}

func (r *startEndRange) isExcludeEnd() bool {
	return r.endCompare.isExcludeEnd()
}

func (r *startEndRange) isLowerBound() bool {
	return r.startCompare.isLowerBound()
}

func (r *startEndRange) isUpperBound() bool {
	return r.endCompare.isUpperBound()
}

func (r *startEndRange) start() Version {
	return r.startCompare.start()
}

func (r *startEndRange) end() Version {
	return r.endCompare.end()
}

func (r *startEndRange) testPrerelease(v Version) bool {
	return r.startCompare.testPrerelease(v) || r.endCompare.testPrerelease(v)
}

func (r *startEndRange) ToString(bld io.Writer) {
	r.startCompare.ToString(bld)
	bld.Write([]byte(` `))
	r.endCompare.ToString(bld)
}

func (r *simpleRange) asLowerBound() abstractRange {
	return highestLb
}

func (r *simpleRange) asUpperBound() abstractRange {
	return lowestUb
}

func (r *simpleRange) isAbove(v Version) bool {
	return false
}

func (r *simpleRange) isBelow(v Version) bool {
	return false
}

func (r *simpleRange) isExcludeStart() bool {
	return false
}

func (r *simpleRange) isExcludeEnd() bool {
	return false
}

func (r *simpleRange) isLowerBound() bool {
	return false
}

func (r *simpleRange) isUpperBound() bool {
	return false
}

func (r *simpleRange) start() Version {
	return Min
}

func (r *simpleRange) end() Version {
	return Max
}

func (r *simpleRange) testPrerelease(v Version) bool {
	return !r.IsStable() && r.TripletEquals(v)
}

// Equals
func (r *eqRange) asLowerBound() abstractRange {
	return r
}

func (r *eqRange) asUpperBound() abstractRange {
	return r
}

func (r *eqRange) equals(o abstractRange) bool {
	if or, ok := o.(*eqRange); ok {
		return r.Equals(or.Version)
	}
	return false
}

func (r *eqRange) includes(v Version) bool {
	return r.CompareTo(v) == 0
}

func (r *eqRange) isAbove(v Version) bool {
	return r.CompareTo(v) > 0
}

func (r *eqRange) isBelow(v Version) bool {
	return r.CompareTo(v) < 0
}

func (r *eqRange) isLowerBound() bool {
	return !r.Equals(Min)
}

func (r *eqRange) isUpperBound() bool {
	return !r.Equals(Max)
}

func (r *eqRange) start() Version {
	return r.Version
}

func (r *eqRange) end() Version {
	return r.Version
}

// GreaterEquals
func (r *gtEqRange) asLowerBound() abstractRange {
	return r
}

func (r *gtEqRange) equals(o abstractRange) bool {
	if or, ok := o.(*gtEqRange); ok {
		return r.Equals(or.Version)
	}
	return false
}

func (r *gtEqRange) includes(v Version) bool {
	return r.CompareTo(v) <= 0
}

func (r *gtEqRange) isAbove(v Version) bool {
	return r.CompareTo(v) > 0
}

func (r *gtEqRange) isLowerBound() bool {
	return !r.Equals(Min)
}

func (r *gtEqRange) start() Version {
	return r.Version
}

func (r *gtEqRange) ToString(bld io.Writer) {
	bld.Write([]byte(`>=`))
	r.Version.ToString(bld)
}

// Greater
func (r *gtRange) asLowerBound() abstractRange {
	return r
}

func (r *gtRange) equals(o abstractRange) bool {
	if or, ok := o.(*gtRange); ok {
		return r.Equals(or.Version)
	}
	return false
}

func (r *gtRange) includes(v Version) bool {
	return r.CompareTo(v) < 0
}

func (r *gtRange) isAbove(v Version) bool {
	if r.IsStable() {
		v = v.ToStable()
	}
	return r.CompareTo(v) >= 0
}

func (r *gtRange) isExcludeStart() bool {
	return true
}

func (r *gtRange) isLowerBound() bool {
	return true
}

func (r *gtRange) start() Version {
	return r.Version
}

func (r *gtRange) ToString(bld io.Writer) {
	bld.Write([]byte(`>`))
	r.Version.ToString(bld)
}

// Less Equal
func (r *ltEqRange) asUpperBound() abstractRange {
	return r
}

func (r *ltEqRange) equals(o abstractRange) bool {
	if or, ok := o.(*ltEqRange); ok {
		return r.Equals(or.Version)
	}
	return false
}

func (r *ltEqRange) includes(v Version) bool {
	return r.CompareTo(v) >= 0
}

func (r *ltEqRange) isBelow(v Version) bool {
	return r.CompareTo(v) < 0
}

func (r *ltEqRange) isUpperBound() bool {
	return !r.Equals(Max)
}

func (r *ltEqRange) end() Version {
	return r.Version
}

func (r *ltEqRange) ToString(bld io.Writer) {
	bld.Write([]byte(`<=`))
	r.Version.ToString(bld)
}

// Less
func (r *ltRange) asUpperBound() abstractRange {
	return r
}

func (r *ltRange) equals(o abstractRange) bool {
	if or, ok := o.(*ltRange); ok {
		return r.Equals(or.Version)
	}
	return false
}

func (r *ltRange) includes(v Version) bool {
	return r.CompareTo(v) > 0
}

func (r *ltRange) isBelow(v Version) bool {
	if r.IsStable() {
		v = v.ToStable()
	}
	return r.CompareTo(v) <= 0
}

func (r *ltRange) isUpperBound() bool {
	return true
}

func (r *ltRange) end() Version {
	return r.Version
}

func (r *ltRange) ToString(bld io.Writer) {
	bld.Write([]byte(`<`))
	r.Version.ToString(bld)
}
