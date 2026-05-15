package geni

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestCreateVideo_Request(t *testing.T) {
	t.Run("POSTs multipart with title + file", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"video-1","title":"Reunion 1972"}`)

		video, err := c.CreateVideo(
			context.Background(),
			"Reunion 1972",
			"reunion.mp4",
			bytes.NewReader([]byte("mp4-bytes")),
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(video.Id).To(Equal("video-1"))
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video/add"))

		fields, fileName, fileBody := readMultipart(t, ft.lastRequest)
		Expect(fields["title"]).To(Equal("Reunion 1972"))
		Expect(fileName).To(Equal("reunion.mp4"))
		Expect(fileBody).To(Equal([]byte("mp4-bytes")))
	})

	t.Run("file is optional — multipart body omits the file part", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"video-1"}`)

		_, err := c.CreateVideo(context.Background(), "External link", "", nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))

		fields, fileName, fileBody := readMultipart(t, ft.lastRequest)
		Expect(fields["title"]).To(Equal("External link"))
		Expect(fileName).To(BeEmpty())
		Expect(fileBody).To(BeEmpty())
	})

	t.Run("WithVideoDescription + WithVideoDate are set as form fields", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"video-1"}`)

		_, err := c.CreateVideo(
			context.Background(),
			"Title",
			"v.mp4",
			strings.NewReader("bytes"),
			WithVideoDescription("a description"),
			WithVideoDate("2026-05-15"),
		)
		Expect(err).ToNot(HaveOccurred())

		fields, _, _ := readMultipart(t, ft.lastRequest)
		Expect(fields).To(HaveKeyWithValue("description", "a description"))
		Expect(fields).To(HaveKeyWithValue("date", "2026-05-15"))
	})

	t.Run("empty title returns an error before sending the request", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{}`)

		_, err := c.CreateVideo(context.Background(), "", "v.mp4", strings.NewReader("x"))

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("title is required"))
		Expect(ft.lastRequest).To(BeNil(), "no HTTP request should have been sent")
	})
}

func TestGetVideo_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"video-9","title":"X"}`)

	video, err := c.GetVideo(context.Background(), "video-9")

	Expect(err).ToNot(HaveOccurred())
	Expect(video.Id).To(Equal("video-9"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodGet))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-9"))
}

func TestGetVideos_SingleIdFallback(t *testing.T) {
	t.Run("1 id → /api/<id>", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"id":"video-9","title":"X"}`)

		res, err := c.GetVideos(context.Background(), []string{"video-9"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-9"))
		Expect(ft.lastRequest.URL.Query().Has("ids")).To(BeFalse())
		Expect(res.Results).To(HaveLen(1))
		Expect(res.Results[0].Id).To(Equal("video-9"))
	})

	t.Run("2 ids → /api/video?ids=…", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"video-1"},{"id":"video-2"}]}`)

		_, err := c.GetVideos(context.Background(), []string{"video-1", "video-2"})

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video"))
		Expect(ft.lastRequest.URL.Query().Get("ids")).To(Equal("video-1,video-2"))
	})
}

func TestUpdateVideo_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"video-1","title":"After"}`)

	video, err := c.UpdateVideo(context.Background(), "video-1", &VideoRequest{
		Title:       "After",
		Description: "updated",
	})

	Expect(err).ToNot(HaveOccurred())
	Expect(video.Title).To(Equal("After"))
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/update"))

	got, _ := io.ReadAll(ft.lastRequest.Body)
	Expect(string(got)).To(ContainSubstring(`"title":"After"`))
	Expect(string(got)).To(ContainSubstring(`"description":"updated"`))
}

func TestDeleteVideo_Request(t *testing.T) {
	t.Run("POSTs to /api/<videoId>/delete", func(t *testing.T) {
		RegisterTestingT(t)
		c, ft := newFakeClient(http.StatusOK, `{"result":"ok"}`)

		err := c.DeleteVideo(context.Background(), "video-9")

		Expect(err).ToNot(HaveOccurred())
		Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
		Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-9/delete"))
	})

	t.Run("404 maps to ErrResourceNotFound", func(t *testing.T) {
		RegisterTestingT(t)
		c, _ := newFakeClient(http.StatusNotFound, ``)

		err := c.DeleteVideo(context.Background(), "video-9")

		Expect(err).To(MatchError(ErrResourceNotFound))
	})
}

func TestTagVideo_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"video-1","tags":["profile-9"]}`)

	_, err := c.TagVideo(context.Background(), "video-1", "profile-9")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/tag/profile-9"))
}

func TestUntagVideo_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"id":"video-1","tags":[]}`)

	_, err := c.UntagVideo(context.Background(), "video-1", "profile-9")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/untag/profile-9"))
}

func TestGetVideoTags_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"profile-1"}],"page":1}`)

	res, err := c.GetVideoTags(context.Background(), "video-1", 1)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/tags"))
	Expect(ft.lastRequest.URL.Query().Get("page")).To(Equal("1"))
	Expect(res.Results).To(HaveLen(1))
}

func TestGetVideoComments_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[{"id":"c-1","comment":"hi"}]}`)

	res, err := c.GetVideoComments(context.Background(), "video-1", 0)

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/comments"))
	Expect(res.Results).To(HaveLen(1))
}

func TestAddVideoComment_Request(t *testing.T) {
	RegisterTestingT(t)
	c, ft := newFakeClient(http.StatusOK, `{"results":[]}`)

	_, err := c.AddVideoComment(context.Background(), "video-1", "nice clip", "")

	Expect(err).ToNot(HaveOccurred())
	Expect(ft.lastRequest.Method).To(Equal(http.MethodPost))
	Expect(ft.lastRequest.URL.Path).To(HaveSuffix("/api/video-1/comment"))
	Expect(ft.lastRequest.URL.Query().Get("text")).To(Equal("nice clip"))
}
