package helper

import "github.com/Masterminds/semver/v3"

func VersionGreaterOrEqual125(v *semver.Version) bool {
	constraint, err := semver.NewConstraint(">=v1.25.0")
	if err != nil {
		return false
	}

	return constraint.Check(v)
}
