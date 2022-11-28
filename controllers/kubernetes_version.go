package controllers

import "github.com/Masterminds/semver"

func VersionGreaterOrEqual125(v *semver.Version) bool {
	constraint, err := semver.NewConstraint(">=v1.25.0")
	if err != nil {
		return false
	}

	return constraint.Check(v)
}
