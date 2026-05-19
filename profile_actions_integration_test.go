package geni

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/dmalch/go-geni/profile"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ptrTo is a tiny helper so the spec bodies can construct
// *string / *bool literals inline without a dedicated `s := "x"; &s`
// dance. Kept private to this file.
func ptrTo[T any](v T) *T { return &v }

var _ = Describe("Profile actions endpoints", func() {
	var (
		ctx      context.Context
		server   *httptest.Server
		client   *Client
		recorded *http.Request
		body     []byte
	)

	BeforeEach(func() {
		ctx = context.Background()
		recorded = nil
		body = nil
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	serve := func(status int, respBody []byte, wantMethod, wantPath string) {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorded = r.Clone(r.Context())
			b, _ := io.ReadAll(r.Body)
			body = b
			Expect(r.Method).To(Equal(wantMethod))
			Expect(r.URL.Path).To(Equal(wantPath))
			Expect(r.URL.Query().Get("access_token")).To(Equal("acc-test"))
			w.WriteHeader(status)
			_, _ = w.Write(respBody)
		}))
		client = newClientFor(server)
	}

	Describe("FollowProfile / UnfollowProfile", func() {
		It("FollowProfile POSTs to /follow and decodes the targeted profile", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"profile-1","first_name":"A","public":true}`),
				http.MethodPost, "/api/profile-1/follow")

			p, err := client.FollowProfile(ctx, "profile-1")

			Expect(err).ToNot(HaveOccurred())
			Expect(p.Id).To(Equal("profile-1"))
			Expect(p.FirstName).ToNot(BeNil())
			Expect(*p.FirstName).To(Equal("A"))
		})

		It("UnfollowProfile targets /unfollow", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"profile-1"}`),
				http.MethodPost, "/api/profile-1/unfollow")

			_, err := client.UnfollowProfile(ctx, "profile-1")

			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("CompareProfiles", func() {
		It("GETs /compare/<other> and decodes both FamilyResponse entries", func() {
			serve(http.StatusOK,
				[]byte(`{"results":[
					{"focus":{"id":"profile-1","first_name":"A"},"nodes":{"profile-1":{"id":"profile-1"}}},
					{"focus":{"id":"profile-2","first_name":"B"},"nodes":{"profile-2":{"id":"profile-2"}}}
				]}`),
				http.MethodGet, "/api/profile-1/compare/profile-2")

			cmp, err := client.CompareProfiles(ctx, "profile-1", "profile-2")

			Expect(err).ToNot(HaveOccurred())
			Expect(cmp.Results).To(HaveLen(2))
			Expect(cmp.Results[0].Focus).ToNot(BeNil())
			Expect(cmp.Results[0].Focus.Id).To(Equal("profile-1"))
			Expect(cmp.Results[1].Focus.Id).To(Equal("profile-2"))
			Expect(cmp.Results[0].Nodes).To(HaveKey("profile-1"))
			Expect(cmp.Results[1].Nodes).To(HaveKey("profile-2"))
		})
	})

	Describe("AddParent", func() {
		It("POSTs the ProfileRequest body to /add-parent and returns the new profile", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"profile-parent","first_name":"Mom","public":true}`),
				http.MethodPost, "/api/profile-1/add-parent")

			parent, err := client.AddParent(ctx, "profile-1", &profile.Request{
				Names: map[string]profile.NameElement{
					"en-US": {FirstName: ptrTo("Mom"), LastName: ptrTo("Smith")},
				},
				IsAlive: false,
				Public:  true,
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(parent.Id).To(Equal("profile-parent"))
			Expect(string(body)).To(ContainSubstring(`"first_name":"Mom"`))
			Expect(string(body)).To(ContainSubstring(`"last_name":"Smith"`))
		})

		It("forwards WithModifier('adopt') as a query param", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"profile-parent"}`),
				http.MethodPost, "/api/profile-1/add-parent")

			_, err := client.AddParent(ctx, "profile-1", &profile.Request{}, WithModifier("adopt"))

			Expect(err).ToNot(HaveOccurred())
			Expect(recorded.URL.Query().Get("relationship_modifier")).To(Equal("adopt"))
		})
	})

	Describe("UpdateProfileBasics", func() {
		It("POSTs the basics body to /update-basics and decodes the updated profile", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"profile-1","first_name":"After","public":true}`),
				http.MethodPost, "/api/profile-1/update-basics")

			updated, err := client.UpdateProfileBasics(ctx, "profile-1", &profile.Request{
				Names: map[string]profile.NameElement{
					"en-US": {FirstName: ptrTo("After"), LastName: ptrTo("Smith")},
				},
				IsAlive: false,
				Public:  true,
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(updated.FirstName).ToNot(BeNil())
			Expect(*updated.FirstName).To(Equal("After"))
			Expect(string(body)).To(ContainSubstring(`"first_name":"After"`))
		})
	})

	Describe("AddProfilePhoto", func() {
		It("POSTs a JSON body with the Base64 file to /add-photo and decodes the new Photo", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"photo-9","title":"Snapshot","sizes":{"small":"https://photos.geni.test/p-9/small.jpg"}}`),
				http.MethodPost, "/api/profile-1/add-photo")

			b64 := "aGVsbG8="
			photo, err := client.AddProfilePhoto(ctx, "profile-1", &PhotoRequest{
				Title: "Snapshot",
				File:  &b64,
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(photo.Id).To(Equal("photo-9"))
			Expect(photo.Sizes).To(HaveKeyWithValue("small", "https://photos.geni.test/p-9/small.jpg"))
			Expect(string(body)).To(ContainSubstring(`"title":"Snapshot"`))
			Expect(string(body)).To(ContainSubstring(`"file":"aGVsbG8="`))
			// Important: this is NOT multipart — it's a JSON body
			// distinct from the /photo/add path used by CreatePhoto.
			Expect(recorded.Header.Get("Content-Type")).To(HavePrefix("application/json"))
		})
	})

	Describe("AddProfileVideo", func() {
		It("POSTs a JSON body to /add-video and decodes the new Video", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"video-9","title":"Reel"}`),
				http.MethodPost, "/api/profile-1/add-video")

			_, err := client.AddProfileVideo(ctx, "profile-1", &VideoRequest{Title: "Reel"})

			Expect(err).ToNot(HaveOccurred())
			Expect(string(body)).To(ContainSubstring(`"title":"Reel"`))
			Expect(recorded.Header.Get("Content-Type")).To(HavePrefix("application/json"))
		})
	})

	Describe("AddProfileDocument", func() {
		It("POSTs a text-body DocumentRequest to /add-document and decodes the new Document", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"document-9","title":"Letter"}`),
				http.MethodPost, "/api/profile-1/add-document")

			text := "Lorem ipsum"
			doc, err := client.AddProfileDocument(ctx, "profile-1", &DocumentRequest{
				Title: "Letter",
				Text:  &text,
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(doc.Id).To(Equal("document-9"))
			Expect(string(body)).To(ContainSubstring(`"text":"Lorem ipsum"`))
		})

		It("POSTs a source_url-body DocumentRequest", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"document-10"}`),
				http.MethodPost, "/api/profile-1/add-document")

			src := "https://example.org/cert.pdf"
			_, err := client.AddProfileDocument(ctx, "profile-1", &DocumentRequest{
				Title:     "Source",
				SourceUrl: &src,
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(string(body)).To(ContainSubstring(`"source_url":"https://example.org/cert.pdf"`))
		})
	})

	Describe("AddProfileMugshot", func() {
		It("sends File when only File is set (PhotoId omitted)", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"photo-9"}`),
				http.MethodPost, "/api/profile-1/add-mugshot")

			b64 := "aGVsbG8="
			_, err := client.AddProfileMugshot(ctx, "profile-1", &MugshotRequest{File: &b64})

			Expect(err).ToNot(HaveOccurred())

			// Round-trip the body through json.Unmarshal to assert
			// PhotoId really wasn't in the wire payload.
			var sent map[string]any
			Expect(json.Unmarshal(body, &sent)).To(Succeed())
			Expect(sent).To(HaveKeyWithValue("file", "aGVsbG8="))
			Expect(sent).ToNot(HaveKey("photo_id"))
		})

		It("sends PhotoId when only PhotoId is set (File omitted)", func() {
			serve(http.StatusOK,
				[]byte(`{"id":"photo-9"}`),
				http.MethodPost, "/api/profile-1/add-mugshot")

			existing := "photo-100"
			_, err := client.AddProfileMugshot(ctx, "profile-1", &MugshotRequest{PhotoId: &existing})

			Expect(err).ToNot(HaveOccurred())

			var sent map[string]any
			Expect(json.Unmarshal(body, &sent)).To(Succeed())
			Expect(sent).To(HaveKeyWithValue("photo_id", "photo-100"))
			Expect(sent).ToNot(HaveKey("file"))
		})
	})
})
