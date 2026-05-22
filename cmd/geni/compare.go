package main

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/dmalch/go-geni/profile"
)

// fieldDiff is one row of a profile comparison: a named field with its
// normalised value on each side and whether they match.
type fieldDiff struct {
	Field string `json:"field"`
	A     string `json:"a"`
	B     string `json:"b"`
	Match bool   `json:"match"`
}

// compareSummary counts the matching and mismatching field rows.
type compareSummary struct {
	Matches    int `json:"matches"`
	Mismatches int `json:"mismatches"`
}

// comparedProfiles embeds the full source resources the diff was
// computed from, so the comparison response is self-contained.
type comparedProfiles struct {
	A *profile.Profile `json:"a"`
	B *profile.Profile `json:"b"`
}

// profileComparison is the result of comparing two profiles.
type profileComparison struct {
	A        string           `json:"a"`
	B        string           `json:"b"`
	Fields   []fieldDiff      `json:"fields"`
	Summary  compareSummary   `json:"summary"`
	Profiles comparedProfiles `json:"profiles"`
}

// compareProfiles builds a field-by-field diff of two profiles and
// embeds the originals under Profiles. It is pure — no I/O.
func compareProfiles(a, b *profile.Profile) *profileComparison {
	rows := []fieldDiff{
		{Field: "first_name", A: derefString(a.FirstName), B: derefString(b.FirstName)},
		{Field: "middle_name", A: derefString(a.MiddleName), B: derefString(b.MiddleName)},
		{Field: "last_name", A: derefString(a.LastName), B: derefString(b.LastName)},
		{Field: "maiden_name", A: derefString(a.MaidenName), B: derefString(b.MaidenName)},
		{Field: "display_name", A: derefString(a.DisplayName), B: derefString(b.DisplayName)},
		{Field: "gender", A: derefString(a.Gender), B: derefString(b.Gender)},
		{Field: "birth_date", A: eventDate(a.Birth), B: eventDate(b.Birth)},
		{Field: "birth_place", A: eventPlace(a.Birth), B: eventPlace(b.Birth)},
		{Field: "death_date", A: eventDate(a.Death), B: eventDate(b.Death)},
		{Field: "death_place", A: eventPlace(a.Death), B: eventPlace(b.Death)},
		{Field: "is_alive", A: strconv.FormatBool(a.IsAlive), B: strconv.FormatBool(b.IsAlive)},
		{Field: "occupation", A: a.Occupation, B: b.Occupation},
		{Field: "nicknames", A: nicknamesString(a.Nicknames), B: nicknamesString(b.Nicknames)},
	}

	var summary compareSummary
	for i := range rows {
		rows[i].Match = strings.TrimSpace(rows[i].A) == strings.TrimSpace(rows[i].B)
		if rows[i].Match {
			summary.Matches++
		} else {
			summary.Mismatches++
		}
	}

	return &profileComparison{
		A:        a.ID,
		B:        b.ID,
		Fields:   rows,
		Summary:  summary,
		Profiles: comparedProfiles{A: a, B: b},
	}
}

// runProfileCompare handles "geni profile compare <id1> <id2>" — it
// fetches both profiles and prints a field-by-field diff as JSON.
func runProfileCompare(ctx context.Context, g *globalOpts, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: geni profile compare <id1> <id2>")
	}
	c, err := newClient(g)
	if err != nil {
		return err
	}
	a, err := c.Profile().Get(ctx, args[0])
	if err != nil {
		return err
	}
	b, err := c.Profile().Get(ctx, args[1])
	if err != nil {
		return err
	}
	return render(g.stdout, compareProfiles(a, b))
}

// derefString returns the pointed-to string, or "" when the pointer
// is nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// eventDate renders an event's date as a normalised string, or "" when
// the event or its date is absent.
func eventDate(e *profile.EventElement) string {
	if e == nil {
		return ""
	}
	return dateString(e.Date)
}

// eventPlace renders an event's location as a normalised string, or ""
// when the event or its location is absent.
func eventPlace(e *profile.EventElement) string {
	if e == nil {
		return ""
	}
	return placeString(e.Location)
}

// dateString renders a date as "YYYY", "YYYY-MM", or "YYYY-MM-DD",
// omitting absent components. An absent year yields "".
func dateString(d *profile.DateElement) string {
	if d == nil || d.Year == nil {
		return ""
	}
	out := strconv.Itoa(int(*d.Year))
	if d.Month != nil {
		out += "-" + zeroPad(int(*d.Month))
		if d.Day != nil {
			out += "-" + zeroPad(int(*d.Day))
		}
	}
	return out
}

// zeroPad formats n as a two-digit, zero-padded string.
func zeroPad(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// placeString renders a location as a comma-joined "City, County,
// State, Country", dropping absent components.
func placeString(l *profile.LocationElement) string {
	if l == nil {
		return ""
	}
	var parts []string
	for _, p := range []*string{l.City, l.County, l.State, l.Country} {
		if s := derefString(p); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

// nicknamesString renders a nickname list as a sorted, comma-joined
// string so the comparison is order-independent.
func nicknamesString(n []string) string {
	if len(n) == 0 {
		return ""
	}
	sorted := append([]string(nil), n...)
	sort.Strings(sorted)
	return strings.Join(sorted, ", ")
}
