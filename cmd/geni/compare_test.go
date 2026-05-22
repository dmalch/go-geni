package main

import (
	"context"
	"testing"

	"github.com/dmalch/go-geni/profile"
	. "github.com/onsi/gomega"
)

func strp(s string) *string { return &s }
func i32p(n int32) *int32   { return &n }

func TestDerefString(t *testing.T) {
	RegisterTestingT(t)
	Expect(derefString(nil)).To(Equal(""))
	Expect(derefString(strp("hi"))).To(Equal("hi"))
}

func TestZeroPad(t *testing.T) {
	RegisterTestingT(t)
	Expect(zeroPad(3)).To(Equal("03"))
	Expect(zeroPad(11)).To(Equal("11"))
}

func TestDateString(t *testing.T) {
	RegisterTestingT(t)
	Expect(dateString(nil)).To(Equal(""))
	Expect(dateString(&profile.DateElement{})).To(Equal(""), "no year → empty")
	Expect(dateString(&profile.DateElement{Year: i32p(1850)})).To(Equal("1850"))
	Expect(dateString(&profile.DateElement{Year: i32p(1850), Month: i32p(3)})).To(Equal("1850-03"))
	Expect(dateString(&profile.DateElement{Year: i32p(1850), Month: i32p(3), Day: i32p(7)})).To(Equal("1850-03-07"))
	// A day without a month is ignored — no "YYYY--DD".
	Expect(dateString(&profile.DateElement{Year: i32p(1850), Day: i32p(7)})).To(Equal("1850"))
}

func TestPlaceString(t *testing.T) {
	RegisterTestingT(t)
	Expect(placeString(nil)).To(Equal(""))
	Expect(placeString(&profile.LocationElement{})).To(Equal(""))
	Expect(placeString(&profile.LocationElement{City: strp("Moscow"), Country: strp("Russia")})).
		To(Equal("Moscow, Russia"))
	Expect(placeString(&profile.LocationElement{
		City: strp("Moscow"), County: strp("C"), State: strp("S"), Country: strp("Russia"),
	})).To(Equal("Moscow, C, S, Russia"))
}

func TestNicknamesString(t *testing.T) {
	RegisterTestingT(t)
	Expect(nicknamesString(nil)).To(Equal(""))
	// Sorted, so the comparison is order-independent.
	Expect(nicknamesString([]string{"Vanya", "Ivan"})).To(Equal("Ivan, Vanya"))
}

func TestCompareProfiles(t *testing.T) {
	t.Run("identical profiles → all match", func(t *testing.T) {
		RegisterTestingT(t)
		p := &profile.Profile{
			ID:        "profile-1",
			FirstName: strp("John"),
			LastName:  strp("Smith"),
			Birth:     &profile.EventElement{Date: &profile.DateElement{Year: i32p(1850)}},
		}
		cmp := compareProfiles(p, p)

		Expect(cmp.A).To(Equal("profile-1"))
		Expect(cmp.B).To(Equal("profile-1"))
		Expect(cmp.Summary.Mismatches).To(Equal(0))
		Expect(cmp.Summary.Matches).To(Equal(len(cmp.Fields)))
		for _, f := range cmp.Fields {
			Expect(f.Match).To(BeTrue(), "field %s should match", f.Field)
		}
	})

	t.Run("differing fields are flagged", func(t *testing.T) {
		RegisterTestingT(t)
		a := &profile.Profile{
			ID: "profile-1", FirstName: strp("John"), LastName: strp("Smith"),
			Birth: &profile.EventElement{Date: &profile.DateElement{Year: i32p(1850)}},
		}
		b := &profile.Profile{
			ID: "profile-2", FirstName: strp("John"), LastName: strp("Smyth"),
			Birth: &profile.EventElement{Date: &profile.DateElement{Year: i32p(1850), Month: i32p(3)}},
		}
		cmp := compareProfiles(a, b)

		byField := map[string]fieldDiff{}
		for _, f := range cmp.Fields {
			byField[f.Field] = f
		}
		Expect(byField["first_name"].Match).To(BeTrue())
		Expect(byField["last_name"].Match).To(BeFalse())
		Expect(byField["last_name"].A).To(Equal("Smith"))
		Expect(byField["last_name"].B).To(Equal("Smyth"))
		Expect(byField["birth_date"].Match).To(BeFalse())
		Expect(byField["birth_date"].A).To(Equal("1850"))
		Expect(byField["birth_date"].B).To(Equal("1850-03"))
		Expect(cmp.Summary.Matches + cmp.Summary.Mismatches).To(Equal(len(cmp.Fields)))
		Expect(cmp.Summary.Mismatches).To(Equal(2))
	})

	t.Run("two empty profiles → all match, originals embedded", func(t *testing.T) {
		RegisterTestingT(t)
		a := &profile.Profile{ID: "profile-1"}
		b := &profile.Profile{ID: "profile-2"}
		cmp := compareProfiles(a, b)

		Expect(cmp.Summary.Mismatches).To(Equal(0))
		Expect(cmp.Profiles.A).To(BeIdenticalTo(a))
		Expect(cmp.Profiles.B).To(BeIdenticalTo(b))
	})
}

func TestRunProfileCompare_ArgValidation(t *testing.T) {
	g := &globalOpts{}

	for _, args := range [][]string{nil, {"profile-1"}, {"profile-1", "profile-2", "profile-3"}} {
		t.Run("rejects arg count", func(t *testing.T) {
			RegisterTestingT(t)
			Expect(runProfileCompare(context.Background(), g, args)).To(HaveOccurred())
		})
	}
}
