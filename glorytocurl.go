package glorytocurl

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"

	"github.com/goccy/go-yaml"
)

func GloryToCurl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := os.LookupEnv("GLORY_TO_CURL"); !ok {
			next.ServeHTTP(w, r)
			return
		}
		var bodyBuffer bytes.Buffer
		tee := io.TeeReader(r.Body, &bodyBuffer)
		r.Body = io.NopCloser(tee)
		defer r.Body.Close()
		dw := httptest.NewRecorder()

		next.ServeHTTP(dw, r)

		// can't recover in this middleware
		if dw.Code < 400 && dw.Code >= 500 {
			for k, v := range dw.Header() {
				w.Header()[k] = v
			}
			w.Write(bodyBuffer.Bytes())
			w.WriteHeader(dw.Code)
			return
		}

		ct := r.Header.Get("Content-Type")
		var jsonBody []byte
		// recover curl's -d option to json
		switch ct {
		case "":
			r.Header.Set("Content-Type", "application/json")
			fallthrough
		case "application/json":
			body := make(map[string]any)
			// can't recover because the body is broken
			if err := yaml.Unmarshal(bodyBuffer.Bytes(), &body); err != nil {
				for k, v := range dw.Header() {
					w.Header()[k] = v
				}
				w.Write(bodyBuffer.Bytes())
				w.WriteHeader(dw.Code)
			}
			jsonBody, _ = json.Marshal(body)
		case "application/x-www-form-urlencoded":
			r.ParseForm()
			body := make(map[string]any, len(r.Form))
			for k, v := range r.Form {
				if len(v) == 1 {
					if i, err := strconv.ParseInt(v[0], 10, 64); err == nil {
						body[k] = i
					} else {
						body[k] = v[0]
					}
				} else {
					body[k] = v
				}
			}
			jsonBody, _ = json.Marshal(body)
		}
		r.Body = io.NopCloser(bytes.NewReader(jsonBody))
		r.ContentLength = int64(len(jsonBody))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Content-Length", strconv.Itoa(len(jsonBody)))
		r.Form = nil
		r.PostForm = nil
		r.MultipartForm = nil
		next.ServeHTTP(w, r)
	})
}
