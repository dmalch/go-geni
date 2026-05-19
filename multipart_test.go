package geni

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

// readMultipart parses a request body the client built and returns
// the recorded form values + the file part's filename and raw bytes.
// Kept in root only while the video resource still lives here —
// moves into video/ alongside the methods later.
func readMultipart(t *testing.T, req *http.Request) (fields map[string]string, fileName string, fileBody []byte) {
	t.Helper()
	ct := req.Header.Get("Content-Type")
	Expect(ct).To(HavePrefix("multipart/form-data;"))

	_, params, err := mime.ParseMediaType(ct)
	Expect(err).ToNot(HaveOccurred())
	boundary, ok := params["boundary"]
	Expect(ok).To(BeTrue())

	mr := multipart.NewReader(req.Body, boundary)
	fields = map[string]string{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		Expect(err).ToNot(HaveOccurred())
		buf, err := io.ReadAll(part)
		Expect(err).ToNot(HaveOccurred())
		if part.FileName() != "" {
			fileName = part.FileName()
			fileBody = buf
		} else {
			fields[part.FormName()] = string(buf)
		}
	}
	return
}
