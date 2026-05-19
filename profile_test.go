package geni

import (
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	profileapi "github.com/dmalch/go-geni/profile"
)

func ptr[T any](s T) *T {
	return &s
}

func TestCreateProfile1(t *testing.T) {
	t.Skip()
	RegisterTestingT(t)
	profileRequest := profileapi.Request{
		Gender: ptr("male"),
		Names: map[string]profileapi.NameElement{
			"en-US": {
				FirstName: ptr("1TestFirstName"),
				LastName:  ptr("1TestLastName"),
			},
			"ru": {
				FirstName:  ptr("Ф"),
				LastName:   ptr("М"),
				MiddleName: ptr("Н"),
			},
		},
		Birth: &profileapi.EventElement{
			Date: &profileapi.DateElement{
				Day:   ptr[int32](19),
				Month: ptr[int32](8),
				Year:  ptr[int32](1922),
			},
			Location: &profileapi.LocationElement{
				Country:   ptr("РСФСР"),
				County:    ptr("Спасский уезд, Кирилловская волость"),
				PlaceName: ptr("село Кирилово"),
				State:     ptr("Тамбовская губерния"),
			},
		},
		Death: &profileapi.EventElement{
			Date: &profileapi.DateElement{
				Day:   ptr[int32](25),
				Month: ptr[int32](9),
				Year:  ptr[int32](1993),
			},
		},
	}
	client := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: testAccessToken}), true)

	profile, err := client.CreateProfile(t.Context(), &profileRequest)

	Expect(err).ToNot(HaveOccurred())
	Expect(profile).ToNot(BeNil())
	Expect(profile.Id).ToNot(BeEmpty())
	Expect(profile.Guid).ToNot(BeEmpty())
	Expect(profile.FirstName).To(BeEquivalentTo("1TestFirstName"))
	Expect(profile.LastName).To(BeEquivalentTo("1TestLastName"))
	Expect(profile.Gender).To(BeEquivalentTo("male"))
	Expect(profile.Names).To(HaveKeyWithValue("en-US", profileapi.NameElement{
		FirstName: ptr("1TestFirstName"),
		LastName:  ptr("1TestLastName"),
	}))
	Expect(profile.Names).To(HaveKeyWithValue("ru", profileapi.NameElement{
		FirstName:  ptr("Ф"),
		LastName:   ptr("М"),
		MiddleName: ptr("Н"),
	}))
	Expect(profile.Birth).To(Equal(&profileapi.EventElement{
		Date: &profileapi.DateElement{
			Day:   ptr[int32](19),
			Month: ptr[int32](8),
			Year:  ptr[int32](1922),
		},
		Location: &profileapi.LocationElement{
			Country:   ptr("РСФСР"),
			County:    ptr("Спасский уезд, Кирилловская волость"),
			PlaceName: ptr("село Кирилово"),
			State:     ptr("Тамбовская губерния"),
		},
		Name: "Birth of 1TestFirstName 1TestLastName",
	}))
	Expect(profile.Death).To(Equal(&profileapi.EventElement{
		Date: &profileapi.DateElement{
			Day:   ptr[int32](25),
			Month: ptr[int32](9),
			Year:  ptr[int32](1993),
		},
		Name: "Death of 1TestFirstName 1TestLastName",
	}))
	Expect(profile.CreatedAt).ToNot(BeEmpty())
}

func TestGetProfile1(t *testing.T) {
	t.Skip()
	RegisterTestingT(t)

	profileId := "profile-5955"

	client := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: testAccessToken}), true)

	profile, err := client.GetProfile(t.Context(), profileId)

	Expect(err).ToNot(HaveOccurred())
	Expect(profile).ToNot(BeNil())
	Expect(profile.Id).To(BeEquivalentTo(profileId))
	Expect(profile.FirstName).To(BeEquivalentTo("D"))
	Expect(profile.LastName).To(BeEquivalentTo("M"))
	Expect(profile.Gender).To(BeEquivalentTo("male"))
	Expect(profile.Names).To(HaveKeyWithValue("en-US", profileapi.NameElement{
		FirstName: ptr("D"),
		LastName:  ptr("M"),
	}))
	Expect(profile.Names).To(HaveKeyWithValue("ru", profileapi.NameElement{
		FirstName:  ptr("Д"),
		LastName:   ptr("М"),
		MiddleName: ptr("В"),
	}))
	Expect(profile.Unions).To(ContainElement("union-1837"))
}

func TestGetProfile2(t *testing.T) {
	t.Skip()
	RegisterTestingT(t)

	profileId := "profile-5957"

	client := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: testAccessToken}), true)

	profile, err := client.GetProfile(t.Context(), profileId)

	Expect(err).ToNot(HaveOccurred())
	Expect(profile).ToNot(BeNil())
	Expect(profile.Id).To(BeEquivalentTo(profileId))
	Expect(profile.Guid).To(BeEquivalentTo("598352"))
	Expect(profile.FirstName).To(BeEquivalentTo("F"))
	Expect(profile.LastName).To(BeEquivalentTo("M"))
	Expect(profile.Gender).To(BeEquivalentTo("male"))
	Expect(profile.Names).To(HaveKeyWithValue("en-US", profileapi.NameElement{
		FirstName: ptr("F"),
		LastName:  ptr("M"),
	}))
	Expect(profile.Names).To(HaveKeyWithValue("ru", profileapi.NameElement{
		FirstName:  ptr("Ф"),
		LastName:   ptr("М"),
		MiddleName: ptr("Н"),
	}))
	Expect(profile.Birth).To(Equal(&profileapi.EventElement{
		Date: &profileapi.DateElement{
			Day:   ptr[int32](19),
			Month: ptr[int32](8),
			Year:  ptr[int32](1922),
		},
		Location: &profileapi.LocationElement{
			Country:   ptr("РСФСР"),
			County:    ptr("Спасский уезд, Кирилловская волость"),
			PlaceName: ptr("село Кирилово"),
			State:     ptr("Тамбовская губерния"),
		},
		Name: "Birth of F M",
	}))
	Expect(profile.Death).To(Equal(&profileapi.EventElement{
		Date: &profileapi.DateElement{
			Day:   ptr[int32](25),
			Month: ptr[int32](9),
			Year:  ptr[int32](1993),
		},
		Name: "Death of F M",
	}))
	Expect(profile.CreatedAt).To(BeEquivalentTo("1741860385"))
}

func TestUpdateProfile1(t *testing.T) {
	t.Skip()
	RegisterTestingT(t)
	profileRequest := profileapi.Request{
		Gender: ptr("male"),
		Names: map[string]profileapi.NameElement{
			"en-US": {
				FirstName: ptr("1TestFirstName"),
				LastName:  ptr("1TestLastName"),
			},
			"ru": {
				FirstName:  ptr("Ф"),
				LastName:   ptr("М"),
				MiddleName: ptr("Н"),
			},
		},
		Birth: &profileapi.EventElement{
			Date: &profileapi.DateElement{
				Day:   ptr[int32](19),
				Month: ptr[int32](8),
				Year:  ptr[int32](1922),
			},
			Location: &profileapi.LocationElement{
				Country:   ptr("РСФСР"),
				County:    ptr("Спасский уезд, Кирилловская волость"),
				PlaceName: ptr("село Кирилово"),
				State:     ptr("Тамбовская губерния"),
			},
		},
		Death: &profileapi.EventElement{
			Date: &profileapi.DateElement{
				Day:   ptr[int32](25),
				Month: ptr[int32](9),
				Year:  ptr[int32](1993),
			},
		},
	}

	client := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: testAccessToken}), true)

	profile, err := client.CreateProfile(t.Context(), &profileRequest)
	Expect(err).ToNot(HaveOccurred())

	profileRequest.Title = "2TestFirstName"
	updatedProfile, err := client.UpdateProfile(t.Context(), profile.Id, &profileRequest)
	Expect(err).ToNot(HaveOccurred())

	Expect(updatedProfile).ToNot(BeNil())
	Expect(updatedProfile.Id).ToNot(BeEmpty())
	Expect(updatedProfile.Guid).ToNot(BeEmpty())
	Expect(updatedProfile.FirstName).To(BeEquivalentTo("2TestFirstName"))
	Expect(updatedProfile.LastName).To(BeEquivalentTo("1TestLastName"))
	Expect(updatedProfile.Gender).To(BeEquivalentTo("male"))
	Expect(updatedProfile.Names).To(HaveKeyWithValue("en-US", profileapi.NameElement{
		FirstName: ptr("2TestFirstName"),
		LastName:  ptr("1TestLastName"),
	}))
	Expect(updatedProfile.Names).To(HaveKeyWithValue("ru", profileapi.NameElement{
		FirstName:  ptr("Ф"),
		LastName:   ptr("М"),
		MiddleName: ptr("Н"),
	}))
	Expect(updatedProfile.Birth).To(Equal(&profileapi.EventElement{
		Date: &profileapi.DateElement{
			Day:   ptr[int32](19),
			Month: ptr[int32](8),
			Year:  ptr[int32](1922),
		},
		Location: &profileapi.LocationElement{
			Country:   ptr("РСФСР"),
			County:    ptr("Спасский уезд, Кирилловская волость"),
			PlaceName: ptr("село Кирилово"),
			State:     ptr("Тамбовская губерния"),
		},
		Name: "Birth of 2TestFirstName 1TestLastName",
	}))
	Expect(updatedProfile.Death).To(Equal(&profileapi.EventElement{
		Date: &profileapi.DateElement{
			Day:   ptr[int32](25),
			Month: ptr[int32](9),
			Year:  ptr[int32](1993),
		},
		Name: "Death of 2TestFirstName 1TestLastName",
	}))
	Expect(updatedProfile.CreatedAt).ToNot(BeEmpty())
}

func TestDeleteProfile1(t *testing.T) {
	t.Skip()
	RegisterTestingT(t)

	client := NewClient(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: testAccessToken}), true)

	err := client.DeleteProfile(t.Context(), "profile-g599969")

	Expect(err).ToNot(HaveOccurred())
}
