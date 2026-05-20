package acceptance

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dmalch/go-geni"
	"github.com/dmalch/go-geni/document"
	"github.com/dmalch/go-geni/photo"
	"github.com/dmalch/go-geni/profile"
	"github.com/dmalch/go-geni/video"
)

// tinyPngBase64 returns a 1×1 PNG encoded as Base64 — usable as the
// JSON `file` field on AddProfilePhoto / AddProfileMugshot. Mirrors
// inlineTinyPng but for the Base64-JSON paths.
func tinyPngBase64() string {
	GinkgoHelper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.Black)
	var buf bytes.Buffer
	Expect(png.Encode(&buf, img)).To(Succeed())
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func ptr[T any](v T) *T { return &v }

var _ = Describe("Profile actions API", func() {
	var (
		ctx    context.Context
		client *geni.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = newTestClient()
	})

	Describe("FollowProfile / UnfollowProfile", func() {
		It("round-trips follow then unfollow", func() {
			target := createFixtureProfile(ctx, client, "FollowTarget")

			followed, err := client.Profile().Follow(ctx, target.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(followed.ID).To(Equal(target.ID))

			unfollowed, err := client.Profile().Unfollow(ctx, target.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(unfollowed.ID).To(Equal(target.ID))
		})
	})

	Describe("CompareProfiles", func() {
		It("returns immediate-family graphs for both profiles", func() {
			a := createFixtureProfile(ctx, client, "CompareA")
			b := createFixtureProfile(ctx, client, "CompareB")

			res, err := client.Tree().Compare(ctx, a.ID, b.ID)

			Expect(err).ToNot(HaveOccurred())
			Expect(res.Results).To(HaveLen(2))
			Expect(res.Results[0].Focus).ToNot(BeNil())
			Expect(res.Results[0].Focus.ID).To(Equal(a.ID))
			Expect(res.Results[1].Focus.ID).To(Equal(b.ID))
		})
	})

	Describe("AddParent", func() {
		It("creates and attaches a new parent profile", func() {
			child := createFixtureProfile(ctx, client, "ParentChild")

			first := "ParentOf"
			last := "Acceptance"
			parent, err := client.Profile().AddParent(ctx, child.ID, &profile.Request{
				Names: map[string]profile.NameElement{
					"en-US": {FirstName: &first, LastName: &last},
				},
				IsAlive: false,
				Public:  true,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(parent.ID).To(HavePrefix("profile-"))
			DeferCleanup(func() { _ = client.Profile().Delete(context.Background(), parent.ID) })

			Expect(parent.FirstName).ToNot(BeNil())
			Expect(*parent.FirstName).To(Equal("ParentOf"))
		})
	})

	Describe("UpdateProfileBasics", func() {
		It("updates the first name and the change round-trips", func() {
			created := createFixtureProfile(ctx, client, "BasicsBefore")

			afterFirst := "BasicsAfter"
			afterLast := "Acceptance"
			updated, err := client.Profile().UpdateBasics(ctx, created.ID, &profile.Request{
				Names: map[string]profile.NameElement{
					"en-US": {FirstName: &afterFirst, LastName: &afterLast},
				},
				IsAlive: false,
				Public:  true,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(updated.FirstName).ToNot(BeNil())
			Expect(*updated.FirstName).To(Equal("BasicsAfter"))
		})
	})

	Describe("AddProfileDocument", func() {
		It("attaches a new document to the profile and returns it", func() {
			profile := createFixtureProfile(ctx, client, "AddDocOwner")

			text := "attached via add-document"
			doc, err := client.Document().AddToProfile(ctx, profile.ID, &document.Request{
				Title: fmt.Sprintf("AccAddProfileDoc-%d", time.Now().UnixNano()),
				Text:  &text,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(doc.ID).To(HavePrefix("document-"))
			DeferCleanup(func() { _ = client.Document().Delete(context.Background(), doc.ID) })
		})
	})

	Describe("AddProfilePhoto", func() {
		It("attaches a new photo to the profile (Base64 JSON path)", func() {
			profile := createFixtureProfile(ctx, client, "AddPhotoOwner")

			b64 := tinyPngBase64()
			photo, err := client.Photo().AddToProfile(ctx, profile.ID, &photo.Request{
				Title: fmt.Sprintf("AccAddProfilePhoto-%d", time.Now().UnixNano()),
				File:  &b64,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(photo.ID).To(HavePrefix("photo-"))
			DeferCleanup(func() { _ = client.Photo().Delete(context.Background(), photo.ID) })
		})
	})

	Describe("AddProfileVideo", func() {
		// Same Skip as the rest of the video sandbox tier: Geni
		// runs uploads through ffmpeg validation; placeholder
		// payloads fail. We don't ship a real video fixture.
		It("attaches a new video to the profile", func() {
			Skip("requires a real video fixture — see CreateVideo godoc")

			profile := createFixtureProfile(ctx, client, "AddVideoOwner")
			b64 := "" // placeholder
			video, err := client.Video().AddToProfile(ctx, profile.ID, &video.Request{
				Title: fmt.Sprintf("AccAddProfileVideo-%d", time.Now().UnixNano()),
				File:  &b64,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(video.ID).To(HavePrefix("video-"))
			DeferCleanup(func() { _ = client.Video().Delete(context.Background(), video.ID) })
		})
	})

	Describe("AddProfileMugshot", func() {
		It("sets a mugshot by referencing an existing photo id", func() {
			profile := createFixtureProfile(ctx, client, "MugshotOwner")

			// Create the source photo via the multipart /photo/add
			// endpoint so the mugshot setter has a real photo_id to
			// reference. Using PhotoId avoids needing a Base64
			// fixture in the request body.
			img := image.NewRGBA(image.Rect(0, 0, 1, 1))
			img.Set(0, 0, color.Black)
			var buf bytes.Buffer
			Expect(png.Encode(&buf, img)).To(Succeed())

			source, err := client.Photo().Create(ctx,
				fmt.Sprintf("AccMugshotSource-%d", time.Now().UnixNano()),
				"mugshot.png", &buf)
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() { _ = client.Photo().Delete(context.Background(), source.ID) })

			mug, err := client.Photo().AddMugshotToProfile(ctx, profile.ID, &photo.MugshotRequest{
				PhotoId: ptr(source.ID),
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(mug.ID).To(HavePrefix("photo-"))
			// Mugshot may be the same source photo or a new
			// derived one — either is fine; we just want a
			// non-empty id back.
		})
	})
})
