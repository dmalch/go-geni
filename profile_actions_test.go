package geni

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/dmalch/go-geni/document"
	"github.com/dmalch/go-geni/photo"
	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/video"
	. "github.com/onsi/gomega"
)

func TestFollowProfile_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"profile-1","first_name":"A"}`)

	_, err := c.FollowProfile(context.Background(), "profile-1")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/follow"))
}

func TestUnfollowProfile_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"profile-1"}`)

	_, err := c.UnfollowProfile(context.Background(), "profile-1")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/unfollow"))
}

func TestCompareProfiles_Request(t *testing.T) {
	RegisterTestingT(t)
	body := `{"results":[
		{"focus":{"id":"profile-1"},"nodes":{}},
		{"focus":{"id":"profile-2"},"nodes":{}}
	]}`
	c, ft := newFakeClient(http.StatusOK, body)

	res, err := c.CompareProfiles(context.Background(), "profile-1", "profile-2")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/compare/profile-2"))
	Expect(res.Results).To(HaveLen(2))
	Expect(res.Results[0].Focus).ToNot(BeNil())
	Expect(res.Results[0].Focus.Id).To(Equal("profile-1"))
	Expect(res.Results[1].Focus.Id).To(Equal("profile-2"))
}

func TestAddParent_Request(t *testing.T) {
	t.Run("POSTs JSON to /api/<profileId>/add-parent", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-parent","first_name":"Mom"}`)

		first := "Mom"
		_, err := c.AddParent(context.Background(), "profile-1", &profile.Request{
			Names: map[string]profile.NameElement{
				"en-US": {FirstName: &first},
			},
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-parent"))
		got, _ := io.ReadAll(ft.lastRequest.Body)
		Expect(string(got)).To(ContainSubstring(`"first_name":"Mom"`))
	})

	t.Run("profile.WithModifier sets the relationship_modifier query param", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"profile-parent"}`)

		_, err := c.AddParent(context.Background(), "profile-1", &profile.Request{}, profile.WithModifier("adopt"))

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Query().Get("relationship_modifier")).To(Equal("adopt"))
	})
}

func TestUpdateProfileBasics_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"profile-1","first_name":"After"}`)

	first := "After"
	_, err := c.UpdateProfileBasics(context.Background(), "profile-1", &profile.Request{
		Names: map[string]profile.NameElement{
			"en-US": {FirstName: &first},
		},
	})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/update-basics"))
}

func TestAddProfilePhoto_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"photo-9","title":"Snapshot"}`)

	b64 := "aGVsbG8="
	res, err := c.AddProfilePhoto(context.Background(), "profile-1", &photo.Request{
		Title: "Snapshot",
		File:  &b64,
	})

	Expect(err).ToNot(HaveOccurred())
	Expect(res.Id).To(Equal("photo-9"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-photo"))
	got, _ := io.ReadAll(ft.lastRequest.Body)
	Expect(string(got)).To(ContainSubstring(`"title":"Snapshot"`))
	Expect(string(got)).To(ContainSubstring(`"file":"aGVsbG8="`))
}

func TestAddProfileVideo_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"video-9","title":"Reel"}`)

	_, err := c.AddProfileVideo(context.Background(), "profile-1", &video.Request{Title: "Reel"})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-video"))
}

func TestAddProfileDocument_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"document-9","title":"Letter"}`)

	text := "Lorem ipsum"
	_, err := c.AddProfileDocument(context.Background(), "profile-1", &document.Request{
		Title: "Letter",
		Text:  &text,
	})

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-document"))
	got, _ := io.ReadAll(ft.lastRequest.Body)
	Expect(string(got)).To(ContainSubstring(`"text":"Lorem ipsum"`))
}

func TestAddProfileMugshot_Request(t *testing.T) {
	t.Run("File path sets file in body", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-9"}`)

		b64 := "aGVsbG8="
		_, err := c.AddProfileMugshot(context.Background(), "profile-1", &MugshotRequest{
			File: &b64,
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/profile-1/add-mugshot"))
		got, _ := io.ReadAll(ft.lastRequest.Body)
		Expect(string(got)).To(ContainSubstring(`"file":"aGVsbG8="`))
		Expect(string(got)).ToNot(ContainSubstring(`"photo_id"`),
			"photo_id should be omitted when only File is set")
	})

	t.Run("PhotoId path sets photo_id in body", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"photo-9"}`)

		existing := "photo-100"
		_, err := c.AddProfileMugshot(context.Background(), "profile-1", &MugshotRequest{
			PhotoId: &existing,
		})

		Expect(err).ToNot(HaveOccurred())
		got, _ := io.ReadAll(ft.lastRequest.Body)
		Expect(string(got)).To(ContainSubstring(`"photo_id":"photo-100"`))
		Expect(string(got)).ToNot(ContainSubstring(`"file"`),
			"file should be omitted when only PhotoId is set")
	})
}

func TestFollowProfile_ErrorMapping(t *testing.T) {
	t.Run("404 → ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)
		_, err := c.FollowProfile(context.Background(), "profile-1")
		Expect(err).To(MatchError(ErrResourceNotFound))
	})
	t.Run("403 → ErrAccessDenied", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusForbidden, ``)
		_, err := c.FollowProfile(context.Background(), "profile-1")
		Expect(err).To(MatchError(ErrAccessDenied))
	})
}
