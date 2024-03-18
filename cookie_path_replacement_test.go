package TraefikCookiePathReplacement

import (
	"bytes"
	"context"
	"math/rand"
	"net/http"
	"testing"
)

type TestCase struct {
	Name          string
	Config        Config
	ExpectedError bool
	Cookies       []map[string][2]string //[request] => [cookie name] => [old path, new path]
}

func TestCookiePathReplacement(t *testing.T) {
	testCases := []TestCase{
		{
			Name:   "Nothing",
			Config: Config{},
		},
		{
			Name: "SimpleReplacement",
			Config: Config{
				Replacements: []ReplacementConfig{
					{
						Original:    "/old",
						Replacement: "/new",
					},
				},
			},
			Cookies: []map[string][2]string{
				{
					"test":  [2]string{"/old", "/new"},
					"test2": [2]string{"/old", "/new"},
					"test3": [2]string{"/old/", "/old/"},
				},
			},
		},
		{
			Name: "StartAndEndTokens",
			Config: Config{
				Replacements: []ReplacementConfig{
					{
						Original:    "^/old$",
						Replacement: "/new",
					},
				},
			},
			Cookies: []map[string][2]string{
				{
					"test":  [2]string{"/old", "/new"},
					"test2": [2]string{"/old/", "/old/"},
				},
			},
		},
		{
			Name: "NamedCapture",
			Config: Config{
				Replacements: []ReplacementConfig{
					{
						Original:    "/(?P<name>[^/]+)",
						Replacement: "/new/{{name}}",
					},
				},
			},
			Cookies: []map[string][2]string{
				{
					"test":  [2]string{"/old", "/new/old"},
					"test2": [2]string{"/old/", "/old/"},
				},
			},
		},
		{
			Name: "NameMatching",
			Config: Config{
				Replacements: []ReplacementConfig{
					{
						Name:        "test",
						Original:    "/old",
						Replacement: "/new",
					},
				},
			},
			Cookies: []map[string][2]string{
				{
					"test":  [2]string{"/old", "/new"},
					"test2": [2]string{"/old", "/old"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			mockHandler := &MockHandler{}
			mware, err := New(context.Background(), mockHandler, &tc.Config, "")
			if err != nil {
				if !tc.ExpectedError {
					t.Fatalf("Expected no error, got %v", err)
				} else {
					return
				}
			}
			if tc.ExpectedError {
				t.Fatalf("Expected error, got nil")
			}
			if mware == nil {
				t.Fatalf("Expected middleware, got nil")
			}

			for _, testRequest := range tc.Cookies {
				randomValues := map[string]string{} // [cookie name] => [value]
				mockWriter := &MockResponseWriter{}

				mware.ServeHTTP(mockWriter, nil)
				rw := mockHandler.GetResponseWriter()

				for cookieName, cookiePaths := range testRequest {
					rValue := RandomString(32)
					randomValues[cookieName] = rValue
					http.SetCookie(rw, &http.Cookie{Name: cookieName, Value: rValue, Path: cookiePaths[0]})
				}

				rw.WriteHeader(240)
				if mockWriter.WriteHeaderCalled != 1 {
					t.Fatalf("Expected WriteHeader to be called once, got %d", mockWriter.WriteHeaderCalled)
				}

				if mockWriter.GetStatusCode() != 240 {
					t.Fatalf("Expected status code to be 240, got %d", mockWriter.GetStatusCode())
				}

				_, err := rw.Write([]byte("test"))
				if err != nil {
					t.Fatalf("Expected no error, got %v", err)
				}

				if mockWriter.WriteCalled != 1 {
					t.Fatalf("Expected Write to be called once, got %d", mockWriter.WriteCalled)
				}

				if mockWriter.GetBody() != "test" {
					t.Fatalf("Expected body to be 'test', got %s", mockWriter.GetBody())
				}

				cookies := mockWriter.GetCookies()
				for _, cookie := range cookies {
					if cookie.Value != randomValues[cookie.Name] {
						t.Fatalf("Expected cookie value to be %s, got %s", randomValues[cookie.Name], cookie.Value)
					}
					if cookie.Path != testRequest[cookie.Name][1] {
						t.Fatalf("Expected cookie path for %s to be %s, got %s", cookie.Name, testRequest[cookie.Name][1], cookie.Path)
					}
				}
			}
		})
	}
}

func TestResponseWriterPassThrough(t *testing.T) {
	mockWriter := &MockResponseWriter{}
	rw := responseWriter{
		writer: mockWriter,
	}

	rw.WriteHeader(240)
	if mockWriter.WriteHeaderCalled != 1 {
		t.Fatalf("Expected WriteHeader to be called once, got %d", mockWriter.WriteHeaderCalled)
	}

	if mockWriter.GetStatusCode() != 240 {
		t.Fatalf("Expected status code to be 240, got %d", mockWriter.GetStatusCode())
	}

	_, err := rw.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if mockWriter.WriteCalled != 1 {
		t.Fatalf("Expected Write to be called once, got %d", mockWriter.WriteCalled)
	}

	if mockWriter.GetBody() != "test" {
		t.Fatalf("Expected body to be 'test', got %s", mockWriter.GetBody())
	}
}

type MockResponseWriter struct {
	HeaderCalled      int
	WriteCalled       int
	WriteHeaderCalled int
	StatusCode        int
	BodyBuffer        bytes.Buffer
	MHeader           http.Header
}

func (m *MockResponseWriter) Header() http.Header {
	if m.HeaderCalled == 0 {
		m.MHeader = make(http.Header)
	}
	m.HeaderCalled++
	return m.MHeader
}

func (m *MockResponseWriter) Write(i []byte) (int, error) {
	m.WriteCalled++
	return m.BodyBuffer.Write(i)
}

func (m *MockResponseWriter) WriteHeader(statusCode int) {
	m.WriteHeaderCalled++
	m.StatusCode = statusCode
}

func (m *MockResponseWriter) GetStatusCode() int {
	return m.StatusCode
}

func (m *MockResponseWriter) GetBody() string {
	return m.BodyBuffer.String()
}

type MockCookie struct {
	Name  string
	Value string
	Path  string
}

func (m *MockResponseWriter) GetCookies() []MockCookie {
	var result []MockCookie
	resp := http.Response{Header: m.MHeader}
	cookies := resp.Cookies()
	for _, cookie := range cookies {
		result = append(result, MockCookie{Name: cookie.Name, Value: cookie.Value, Path: cookie.Path})
	}
	return result
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

type MockHandler struct {
	writer http.ResponseWriter
	req    *http.Request
}

func (m *MockHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	m.writer = rw
	m.req = req
}

func (m *MockHandler) GetResponseWriter() http.ResponseWriter {
	return m.writer
}

func (m *MockHandler) GetRequest() *http.Request {
	return m.req
}
