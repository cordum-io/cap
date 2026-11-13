package publicreport

import (
	"errors"
	"fmt"
)

// Badge errors. Badge also returns Validate's sentinels for an invalid report.
var (
	ErrBadgeVerdict = errors.New("publicreport: a badge needs verdict PASS")
	ErrBadgeProfile = errors.New("publicreport: no badge for this profile")
)

// badgeLabels lists the profiles that get a badge. The SVG is built only from
// these constant values, never from report bytes, so no report text reaches it.
var badgeLabels = map[string]string{
	"core":           "core",
	"cap-production": "cap-production",
}

const badgeTemplate = `<svg xmlns="http://www.w3.org/2000/svg" width="204" height="20" role="img" aria-label="cap-tck %[1]s: PASS">
<title>cap-tck %[1]s: PASS</title>
<rect width="160" height="20" fill="#555"/>
<rect x="160" width="44" height="20" fill="#2e7d32"/>
<g fill="#fff" font-family="DejaVu Sans,Verdana,sans-serif" font-size="11" text-anchor="middle">
<text x="80" y="14">cap-tck %[1]s</text>
<text x="182" y="14">PASS</text>
</g>
</svg>
`

// Badge validates a public report and returns its fixed-template SVG badge.
// Only a valid report with verdict PASS and a listed profile gets one; any
// other input returns an error and nil.
func Badge(report []byte) ([]byte, error) {
	r, err := Validate(report)
	if err != nil {
		return nil, err
	}
	if r.Verdict != VerdictPass {
		return nil, ErrBadgeVerdict
	}
	label, ok := badgeLabels[r.Profile]
	if !ok {
		return nil, ErrBadgeProfile
	}
	return fmt.Appendf(nil, badgeTemplate, label), nil
}
