package geni

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Iter* helpers (end-to-end pagination over a stub server)", func() {
	var (
		ctx    context.Context
		server *httptest.Server
		client *Client
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	// Serve N+1 pages of single-item ProfileBulkResponse bodies for a
	// chosen base URL path. Each page sets next_page on all but the
	// last, so the iterator should stop after `lastPage`.
	servePaginatedProfiles := func(wantPath string, lastPage int) {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal(wantPath))
			pageStr := r.URL.Query().Get("page")
			if pageStr == "" {
				pageStr = "1"
			}
			page := asInt(pageStr)
			body := map[string]any{
				"results": []map[string]any{
					{"id": fmt.Sprintf("profile-p%d-1", page)},
					{"id": fmt.Sprintf("profile-p%d-2", page)},
				},
				"page": page,
			}
			if page < lastPage {
				body["next_page"] = fmt.Sprintf("…?page=%d", page+1)
			}
			_ = json.NewEncoder(w).Encode(body)
		}))
		client = newClientFor(server)
	}

	It("IterManagedProfiles walks every page and yields every profile", func() {
		servePaginatedProfiles("/api/user/managed-profiles", 3)

		var ids []string
		for p, err := range client.IterManagedProfiles(ctx) {
			Expect(err).ToNot(HaveOccurred())
			ids = append(ids, p.Id)
		}

		Expect(ids).To(Equal([]string{
			"profile-p1-1", "profile-p1-2",
			"profile-p2-1", "profile-p2-2",
			"profile-p3-1", "profile-p3-2",
		}))
	})

	It("IterProjectProfiles stops the moment the caller breaks", func() {
		servePaginatedProfiles("/api/project-7/profiles", 5)

		var ids []string
		for p, err := range client.IterProjectProfiles(ctx, "project-7") {
			Expect(err).ToNot(HaveOccurred())
			ids = append(ids, p.Id)
			if len(ids) == 3 {
				break
			}
		}

		Expect(ids).To(HaveLen(3))
	})
})

// asInt is a tiny helper for the fixture body builder.
func asInt(s string) int {
	switch s {
	case "1":
		return 1
	case "2":
		return 2
	case "3":
		return 3
	case "4":
		return 4
	case "5":
		return 5
	}
	return 0
}
