// The version package implements version parsing.
// It also acts as guardian of the current client Juju version number.
package version

import (
	"fmt"
	"io/ioutil"
	"labix.org/v2/mgo/bson"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// The presence and format of this constant is very important. 
// The debian/rules build recipe uses this value for the version
// number of the release package.
const version = "1.9.2"

// Current gives the current version of the system.  If the file
// "FORCE-VERSION" is present in the same directory as the running
// binary, it will override this.
var Current = Binary{
	Number: MustParse(version),
	Series: readSeries("/etc/lsb-release"), // current Ubuntu release name.  
	Arch:   ubuntuArch(runtime.GOARCH),
}

func init() {
	toolsDir := filepath.Dir(os.Args[0])
	v, err := ioutil.ReadFile(filepath.Join(toolsDir, "FORCE-VERSION"))
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		panic(fmt.Errorf("version: cannot read forced version: %v", err))
	}
	Current = MustParseBinary(strings.TrimSpace(string(v)))
}

// Number represents a juju version.  When bugs are fixed the patch
// number is incremented; when new features are added the minor number
// is incremented and patch is reset; and when compatibility is broken
// the major version is incremented and minor and patch are reset.  The
// build number is automatically assigned and has no well defined
// sequence.  If the build number is greater than zero or any of the
// other numbers are odd, it indicates that the release is still in
// development.
type Number struct {
	Major int
	Minor int
	Patch int
	Build int
}

// Binary specifies a binary version of juju.
type Binary struct {
	Number
	Series string
	Arch   string
}

func (v Binary) String() string {
	return fmt.Sprintf("%v-%s-%s", v.Number, v.Series, v.Arch)
}

// GetBSON turns v into a bson.Getter so it can be saved directly
// on a MongoDB database with mgo.
func (v Binary) GetBSON() (interface{}, error) {
	return v.String(), nil
}

// SetBSON turns v into a bson.Setter so it can be loaded directly
// from a MongoDB database with mgo.
func (vp *Binary) SetBSON(raw bson.Raw) error {
	var s string
	err := raw.Unmarshal(&s)
	if err != nil {
		return err
	}
	v, err := ParseBinary(s)
	if err != nil {
		return err
	}
	*vp = v
	return nil
}

var (
	binaryPat = regexp.MustCompile(`^(\d{1,9})\.(\d{1,9})\.(\d{1,9})(\.\d{1,9})?-([^-]+)-([^-]+)$`)
	numberPat = regexp.MustCompile(`^(\d{1,9})\.(\d{1,9})\.(\d{1,9})(\.\d{1,9})?$`)
)

// MustParse parses a version and panics if it does
// not parse correctly.
func MustParse(s string) Number {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// MustParseBinary parses a binary version and panics if it does
// not parse correctly.
func MustParseBinary(s string) Binary {
	v, err := ParseBinary(s)
	if err != nil {
		panic(err)
	}
	return v
}

// ParseBinary parses a binary version of the form "1.2.3-series-arch".
func ParseBinary(s string) (Binary, error) {
	m := binaryPat.FindStringSubmatch(s)
	if m == nil {
		return Binary{}, fmt.Errorf("invalid binary version %q", s)
	}
	var v Binary
	v.Major = atoi(m[1])
	v.Minor = atoi(m[2])
	v.Patch = atoi(m[3])
	if m[4] != "" {
		v.Build = atoi(m[4][1:])
	}
	v.Series = m[5]
	v.Arch = m[6]
	return v, nil
}

// Parse parses the version, which is of the form 1.2.3
// giving the major, minor and release versions
// respectively.
func Parse(s string) (Number, error) {
	m := numberPat.FindStringSubmatch(s)
	if m == nil {
		return Number{}, fmt.Errorf("invalid version %q", s)
	}
	var v Number
	v.Major = atoi(m[1])
	v.Minor = atoi(m[2])
	v.Patch = atoi(m[3])
	if m[4] != "" {
		v.Build = atoi(m[4][1:])
	}
	return v, nil
}

// atoi is the same as strconv.Atoi but assumes that
// the string has been verified to be a valid integer.
func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return n
}

func (v Number) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Build > 0 {
		s += fmt.Sprintf(".%d", v.Build)
	}
	return s
}

// Less returns whether v is semantically earlier in the
// version sequence than w.
func (v Number) Less(w Number) bool {
	switch {
	case v.Major != w.Major:
		return v.Major < w.Major
	case v.Minor != w.Minor:
		return v.Minor < w.Minor
	case v.Patch != w.Patch:
		return v.Patch < w.Patch
	case v.Build != w.Build:
		return v.Build < w.Build
	}
	return false
}

// GetBSON turns v into a bson.Getter so it can be saved directly
// on a MongoDB database with mgo.
func (v Number) GetBSON() (interface{}, error) {
	return v.String(), nil
}

// SetBSON turns v into a bson.Setter so it can be loaded directly
// from a MongoDB database with mgo.
func (vp *Number) SetBSON(raw bson.Raw) error {
	var s string
	err := raw.Unmarshal(&s)
	if err != nil {
		return err
	}
	v, err := Parse(s)
	if err != nil {
		return err
	}
	*vp = v
	return nil
}

func isOdd(x int) bool {
	return x%2 != 0
}

// IsDev returns whether the version represents a development
// version. A version with an odd-numbered major, minor
// or patch version is considered to be a development version.
func (v Number) IsDev() bool {
	return isOdd(v.Major) || isOdd(v.Minor) || isOdd(v.Patch) || v.Build > 0
}

func readSeries(releaseFile string) string {
	data, err := ioutil.ReadFile(releaseFile)
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		const p = "DISTRIB_CODENAME="
		if strings.HasPrefix(line, p) {
			return strings.Trim(line[len(p):], "\t '\"")
		}
	}
	return "unknown"
}

func ubuntuArch(arch string) string {
	if arch == "386" {
		arch = "i386"
	}
	return arch
}
