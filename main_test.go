package main

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/satur-io/estoraje/lib/hashing"
	"github.com/stretchr/testify/assert"
)

var (
	router = setupRouter()
)

func init() {
	gin.SetMode("release")
	ring = hashing.New()
	ring.Add("test")
	myhost := "test"
	host = &myhost
	lockKey = func(key string) (unlock func() error, err error) {
		unlock = func() error {
			return nil
		}
		return
	}
}

func TestGetNotFoundRoute(t *testing.T) {
	realReadFile := readFile
	defer func() { readFile = realReadFile }()

	readFile = func(string) ([]byte, error) {
		return []byte(""), errors.New("Not found")
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/my-key", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
	assert.Equal(t, "", w.Body.String())
}

func TestGetValueRoute(t *testing.T) {
	router := setupRouter()

	realReadFile := readFile
	defer func() { readFile = realReadFile }()

	readFile = func(string) ([]byte, error) {
		return []byte("my-value"), nil
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/my-key", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "my-value", w.Body.String())
}

func TestPostValueErrorRoute(t *testing.T) {
	router := setupRouter()

	realWriteFile := writeFile
	defer func() { writeFile = realWriteFile }()

	writeFile = func(name string, data []byte, perm fs.FileMode) error {
		return errors.New("Error writing")
	}

	realUpdateStatus := updateStatus
	defer func() { updateStatus = realUpdateStatus }()

	updateStatus = func(node string, status int8) {}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/my-key", strings.NewReader("my-value"))
	router.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
	assert.Equal(t, "Write failed on too much nodes, dissmissing...", w.Body.String())
}

func TestPostValueRoute(t *testing.T) {
	router := setupRouter()

	realWriteFile := writeFile
	defer func() { writeFile = realWriteFile }()

	writeFile = func(name string, data []byte, perm fs.FileMode) error {
		return nil
	}

	realUpdateStatus := updateStatus
	defer func() { updateStatus = realUpdateStatus }()

	updateStatus = func(node string, status int8) {}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/my-key", strings.NewReader("my-value"))
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "", w.Body.String())
}

func TestDeleteErrorRoute(t *testing.T) {
	router := setupRouter()

	realRemoveFile := removeFile
	defer func() { removeFile = realRemoveFile }()

	removeFile = func(name string) error {
		return errors.New("Error deleting")
	}

	realUpdateStatus := updateStatus
	defer func() { updateStatus = realUpdateStatus }()

	updateStatus = func(node string, status int8) {}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/my-key", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
	assert.Equal(t, "Write failed on too much nodes, dissmissing...", w.Body.String())
}

func TestDeleteValueRoute(t *testing.T) {
	router := setupRouter()

	realRemoveFile := removeFile
	defer func() { removeFile = realRemoveFile }()

	removeFile = func(name string) error {
		return nil
	}

	realUpdateStatus := updateStatus
	defer func() { updateStatus = realUpdateStatus }()

	updateStatus = func(node string, status int8) {}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/my-key", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "", w.Body.String())
}

func BenchmarkWriting(b *testing.B) {
	router := setupRouter()

	w := httptest.NewRecorder()
	for _, tt := range tests {
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("POST", "/"+tt.key, strings.NewReader(tt.value))
			router.ServeHTTP(w, req)
		}
	}

}

func BenchmarkReading(b *testing.B) {
	router := setupRouter()

	w := httptest.NewRecorder()
	for _, tt := range tests {
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", "/"+tt.key, nil)
			router.ServeHTTP(w, req)
		}
	}

}

func BenchmarkDeleting(b *testing.B) {
	router := setupRouter()

	w := httptest.NewRecorder()
	for _, tt := range tests {
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("DELETE", "/"+tt.key, strings.NewReader(tt.value))
			router.ServeHTTP(w, req)
		}
	}

}

var (
	tests = map[int]struct {
		key   string
		value string
	}{
		0: {key: "33333333333", value: "33333333333value"},
		1: {key: "33333333334", value: "33333333334value"},
		2: {key: "33333333335", value: "33333333335value"},
		3: {key: "33333333336", value: "33333333336value"},
	}
)
