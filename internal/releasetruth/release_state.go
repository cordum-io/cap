package releasetruth

import "strings"

type futureCoordinate struct {
	version string
	tag     string
	channel string
}

func checkReleaseState(m *Manifest) []Problem {
	var ps []Problem
	if m.Candidate != nil && m.Snapshot != nil {
		ps = append(ps, Problem{"releaseState", "candidate and snapshot are mutually exclusive"})
	}
	if m.Candidate != nil && m.SchemaVersion == "1.0.0" {
		ps = append(ps, Problem{"candidate.schemaVersion", "candidate requires schemaVersion >= 1.1.0"})
	}
	if m.Snapshot != nil && m.SchemaVersion != "1.2.0" {
		ps = append(ps, Problem{"snapshot.schemaVersion", "snapshot requires schemaVersion 1.2.0"})
	}
	return ps
}

func checkCandidate(m *Manifest) []Problem {
	if m.Candidate == nil {
		return nil
	}
	c := m.Candidate
	return checkFutureCoordinate("candidate", futureCoordinate{c.Version, c.Tag, c.Channel}, m.Release.Version)
}

func checkSnapshot(m *Manifest) []Problem {
	if m.Snapshot == nil {
		return nil
	}
	s := m.Snapshot
	return checkFutureCoordinate("snapshot", futureCoordinate{s.Version, s.Tag, s.Channel}, m.Release.Version)
}

func checkFutureCoordinate(field string, c futureCoordinate, published string) []Problem {
	var ps []Problem
	if !reSemver.MatchString(c.version) {
		ps = append(ps, Problem{field + ".version", "must be canonical MAJOR.MINOR.PATCH: " + c.version})
	} else if reSemver.MatchString(published) && !semverGreater(c.version, published) {
		ps = append(ps, Problem{field + ".version", "must be newer than published release " + published})
	}
	if c.tag != "v"+c.version {
		ps = append(ps, Problem{field + ".tag", "tag must equal v" + c.version + ", got " + c.tag})
	}
	if c.channel != "stable" {
		ps = append(ps, Problem{field + ".channel", "stable version/tag require channel stable, got " + c.channel})
	}
	return ps
}

func semverGreater(candidate, published string) bool {
	candidateParts := strings.Split(candidate, ".")
	publishedParts := strings.Split(published, ".")
	for i := 0; i < 3; i++ {
		if len(candidateParts[i]) != len(publishedParts[i]) {
			return len(candidateParts[i]) > len(publishedParts[i])
		}
		if candidateParts[i] != publishedParts[i] {
			return candidateParts[i] > publishedParts[i]
		}
	}
	return false
}

func checkPublishedComponentVersions(m *Manifest) []Problem {
	var ps []Problem
	for _, c := range m.Components {
		if c.Tier == "stable" && c.Version != m.Release.Version {
			ps = append(ps, Problem{"components.published.version",
				"stable component " + c.Name + " must remain at published release " + m.Release.Version})
		}
	}
	return ps
}
