package governance

import "testing"

func signedCommit() Commit {
	return Commit{
		Hash: "abc123", AuthorName: "Yaron Torgeman", AuthorEmail: "yaront1111@gmail.com",
		Message: "docs: something\n\nSigned-off-by: Yaron Torgeman <yaront1111@gmail.com>\n",
	}
}

func TestValidSignoffPasses(t *testing.T) {
	if v := CheckDCO([]Commit{signedCommit()}, DefaultBotAllowlist); len(v) != 0 {
		t.Fatalf("valid signoff should pass, got %+v", v)
	}
}

func TestMissingSignoffFails(t *testing.T) {
	c := signedCommit()
	c.Message = "docs: no signoff here"
	v := CheckDCO([]Commit{c}, DefaultBotAllowlist)
	if len(v) != 1 {
		t.Fatalf("missing signoff must produce one violation, got %+v", v)
	}
}

func TestSignoffMustMatchAuthor(t *testing.T) {
	c := signedCommit()
	c.Message = "x\n\nSigned-off-by: Someone Else <someone@else.com>\n"
	if v := CheckDCO([]Commit{c}, DefaultBotAllowlist); len(v) != 1 {
		t.Fatalf("a signoff not matching the author must fail, got %+v", v)
	}
}

func TestSignoffEmailCaseInsensitive(t *testing.T) {
	c := signedCommit()
	c.Message = "x\n\nSigned-off-by: Yaron Torgeman <YARONT1111@Gmail.com>\n"
	if v := CheckDCO([]Commit{c}, DefaultBotAllowlist); len(v) != 0 {
		t.Fatalf("email comparison should be case-insensitive, got %+v", v)
	}
}

func TestCoAuthorNeedsMatchingSignoff(t *testing.T) {
	c := signedCommit()
	c.Message = "x\n\nCo-authored-by: Pat Dev <pat@dev.com>\n" +
		"Signed-off-by: Yaron Torgeman <yaront1111@gmail.com>\n"
	v := CheckDCO([]Commit{c}, DefaultBotAllowlist)
	if len(v) != 1 {
		t.Fatalf("a co-author without a matching signoff must fail, got %+v", v)
	}
}

func TestCoAuthorWithSignoffPasses(t *testing.T) {
	c := signedCommit()
	c.Message = "x\n\nCo-authored-by: Pat Dev <pat@dev.com>\n" +
		"Signed-off-by: Yaron Torgeman <yaront1111@gmail.com>\n" +
		"Signed-off-by: Pat Dev <pat@dev.com>\n"
	if v := CheckDCO([]Commit{c}, DefaultBotAllowlist); len(v) != 0 {
		t.Fatalf("co-author with matching signoff should pass, got %+v", v)
	}
}

func TestBotCommitsAreExempt(t *testing.T) {
	c := Commit{Hash: "d1", AuthorName: "dependabot[bot]",
		AuthorEmail: "49699333+dependabot[bot]@users.noreply.github.com",
		Message:     "chore(deps): bump x"}
	if v := CheckDCO([]Commit{c}, DefaultBotAllowlist); len(v) != 0 {
		t.Fatalf("allowlisted bot must be exempt, got %+v", v)
	}
}

func TestNonAllowlistedBotNotExempt(t *testing.T) {
	c := Commit{Hash: "d1", AuthorName: "randobot[bot]", AuthorEmail: "rando@bot.com",
		Message: "chore: sneaky"}
	if v := CheckDCO([]Commit{c}, DefaultBotAllowlist); len(v) != 1 {
		t.Fatalf("a non-allowlisted bot must still require signoff, got %+v", v)
	}
}

func TestSpoofedBotNameDoesNotBypass(t *testing.T) {
	// A human sets their display name to a bot's, but keeps a human email and no signoff.
	c := Commit{Hash: "d1", AuthorName: "dependabot[bot]", AuthorEmail: "human@example.com",
		Message: "chore: pretending to be a bot"}
	if v := CheckDCO([]Commit{c}, DefaultBotAllowlist); len(v) != 1 {
		t.Fatalf("a spoofed bot display name with a human email must NOT bypass DCO, got %+v", v)
	}
}
