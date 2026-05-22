package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	. "github.com/onsi/gomega"
)

func TestCallbackHandle(t *testing.T) {
	t.Run("Accepts token when state matches", func(t *testing.T) {
		RegisterTestingT(t)

		handler := &callback{
			expectedState: "valid-state",
			shutdownCh:    make(chan error, 1),
		}

		q := make(url.Values)
		q.Set("state", "valid-state")
		q.Set("access_token", "my-token")
		q.Set("expires_in", "3600")
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?"+q.Encode(), nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		shutdownErr := <-handler.shutdownCh
		Expect(shutdownErr).ToNot(HaveOccurred())
		Expect(handler.accessToken).To(Equal("my-token"))
		Expect(handler.expiresIn).To(Equal("3600"))
		Expect(rec.Body.String()).To(ContainSubstring("Login was successful"))
	})

	t.Run("Rejects callback when state does not match", func(t *testing.T) {
		RegisterTestingT(t)

		handler := &callback{
			expectedState: "valid-state",
			shutdownCh:    make(chan error, 1),
		}

		q := make(url.Values)
		q.Set("state", "wrong-state")
		q.Set("access_token", "my-token")
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?"+q.Encode(), nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		shutdownErr := <-handler.shutdownCh
		Expect(shutdownErr).To(HaveOccurred())
		Expect(shutdownErr.Error()).To(ContainSubstring("state parameter mismatch"))
		Expect(handler.accessToken).To(BeEmpty())
		Expect(rec.Body.String()).To(ContainSubstring("CSRF"))
	})

	t.Run("Rejects callback when state is missing", func(t *testing.T) {
		RegisterTestingT(t)

		handler := &callback{
			expectedState: "valid-state",
			shutdownCh:    make(chan error, 1),
		}

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?access_token=my-token", nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		shutdownErr := <-handler.shutdownCh
		Expect(shutdownErr).To(HaveOccurred())
		Expect(handler.accessToken).To(BeEmpty())
	})

	t.Run("Reports unsuccessful login when token is empty but state matches", func(t *testing.T) {
		RegisterTestingT(t)

		handler := &callback{
			expectedState: "valid-state",
			shutdownCh:    make(chan error, 1),
		}

		q := make(url.Values)
		q.Set("state", "valid-state")
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?"+q.Encode(), nil)
		rec := httptest.NewRecorder()

		handler.handle(rec, req)

		shutdownErr := <-handler.shutdownCh
		Expect(shutdownErr).ToNot(HaveOccurred())
		Expect(handler.accessToken).To(BeEmpty())
		Expect(rec.Body.String()).To(ContainSubstring("Login was not successful"))
	})
}

func TestNewAuthTokenSource(t *testing.T) {
	t.Run("Creates token source with config", func(t *testing.T) {
		RegisterTestingT(t)

		src := NewAuthTokenSource(nil)
		Expect(src).ToNot(BeNil())
		Expect(src.config).To(BeNil())
	})
}
