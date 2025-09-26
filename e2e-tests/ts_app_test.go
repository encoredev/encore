//go:build e2e

package tests

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestTSEndToEndWithApp(t *testing.T) {
	c := qt.New(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	nodePath, ok := getNodeJSPath().Get()
	if !ok {
		c.Fatal("Could not find nodejs binary, it is needed to run typescript apps")
	}

	appRoot := filepath.Join(wd, "testdata", "tsapp")
	app := RunApp(c, appRoot, nil, []string{"PATH=" + nodePath})
	run := app.Run

	// Test basic hello endpoint
	c.Run("typescript hello endpoint", func(c *qt.C) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/hello/world", nil)
		run.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, 200)
		c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]string{
			"message": "Hello world",
		})
	})

	// Test middleware functionality
	c.Run("middleware demo endpoint", func(c *qt.C) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/middleware-test", nil)
		run.ServeHTTP(w, req)

		// status modified by mw
		c.Assert(w.Code, qt.Equals, 201)

		// header set by mw
		c.Assert(w.Header().Get("X-Test-Header"), qt.Equals, "hello")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		c.Assert(err, qt.IsNil)

		// Verify middleware data is present
		c.Assert(response["message"], qt.Equals, "Hello")
		c.Assert(response["middlewareMsg"], qt.Equals, "Hello from middleware!")
	})

	// Test custom HTTP status - 404 Not Found
	c.Run("custom HTTP status", func(c *qt.C) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test-custom-status", nil)
		run.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, 202)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		c.Assert(err, qt.IsNil)

		c.Assert(response["message"], qt.Equals, "I accept!")
	})

	// Test service2 greeting endpoint directly
	c.Run("service2 greeting - formal style", func(c *qt.C) {
		requestBody := `{"name": "Alice", "style": "formal"}`
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/greet", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		run.ServeHTTP(w, req)

		c.Assert(w.Code, qt.Equals, 200)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		c.Assert(err, qt.IsNil)

		c.Assert(response["greeting"], qt.Equals, "Good day, Alice. I trust you are well.")
		c.Assert(response["timestamp"], qt.Not(qt.IsNil))
	})

	// Test service2 greeting endpoint - casual style
	c.Run("service2 greeting - casual style", func(c *qt.C) {
		requestBody := `{"name": "Bob", "style": "casual"}`
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/greet", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		run.ServeHTTP(w, req)

		c.Assert(w.Code, qt.Equals, 200)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		c.Assert(err, qt.IsNil)

		c.Assert(response["greeting"], qt.Equals, "Hey Bob! How's it going?")
	})

	// Test service-to-service call - get greeting via service1
	c.Run("service-to-service call - get greeting", func(c *qt.C) {
		requestBody := `{"name": "Charlie", "style": "excited"}`
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/get-greeting", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		run.ServeHTTP(w, req)

		c.Assert(w.Code, qt.Equals, 200)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		c.Assert(err, qt.IsNil)

		c.Assert(response["message"], qt.Equals, "Greeting retrieved successfully via service-to-service call")
		c.Assert(response["greeting"], qt.Equals, "OMG Charlie!!! So great to see you!!!")
	})

	// Test input validation - valid message processing
	c.Run("service2 message processing - valid", func(c *qt.C) {
		requestBody := `{"message": "Hello world", "priority": "high", "recipient": "test@example.com"}`
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/process-message", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		run.ServeHTTP(w, req)

		c.Assert(w.Code, qt.Equals, 201)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		c.Assert(err, qt.IsNil)

		c.Assert(response["message"], qt.Equals, "Message processed with high priority")
		c.Assert(response["processed"], qt.Equals, true)
	})

	// Test input validation - message too short
	c.Run("service2 message processing - too short", func(c *qt.C) {
		requestBody := `{"message": "Hi"}`
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/process-message", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		run.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		c.Assert(err, qt.IsNil)

		c.Assert(response["message"], qt.Contains, "message: length too short")

		// Should return validation error
		c.Assert(w.Code, qt.Equals, 400)
	})

	// Test input validation - invalid email
	c.Run("service2 message processing - invalid email", func(c *qt.C) {
		requestBody := `{"message": "Valid message", "recipient": "not-an-email"}`
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/process-message", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		run.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		c.Assert(err, qt.IsNil)

		c.Assert(response["message"], qt.Contains, "value is not an email")

		// Should return validation error
		c.Assert(w.Code, qt.Equals, 400)
	})

	// Test error handling
	c.Run("service2 api errors", func(c *qt.C) {
		expected := map[string]map[string]interface{}{
			"no-details-no-cause": {
				"code":             "canceled",
				"details":          nil,
				"internal_message": nil,
				"message":          "the error",
			},
			"with-details-no-cause": {
				"code": "canceled",
				"details": map[string]interface{}{
					"a": "detail",
				},
				"internal_message": nil,
				"message":          "the error",
			},
			"no-details-with-cause": {
				"code":             "canceled",
				"details":          nil,
				"internal_message": nil,
				"message":          "the error: Error: this is the cause: this is the cause",
			},
			"with-details-with-cause": {
				"code": "canceled",
				"details": map[string]interface{}{
					"a": "detail",
				},
				"internal_message": nil,
				"message":          "the error: Error: this is the cause: Error: this is the cause: this is the cause",
			},
		}

		for path, expected := range expected {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test-api-error/"+path, nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 499)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			c.Assert(err, qt.IsNil)

			c.Assert(response["code"], qt.Equals, expected["code"])
			if expected["details"] == nil {
				c.Assert(response["details"], qt.IsNil)
			} else {
				c.Assert(response["details"], qt.DeepEquals, expected["details"])
			}
			c.Assert(response["internal_message"], qt.Equals, expected["internal_message"])
			c.Assert(response["message"], qt.Equals, expected["message"])
		}

	})

	c.Run("service2 other errors", func(c *qt.C) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test-other-error/", nil)
		run.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, 500)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		c.Assert(err, qt.IsNil)

		c.Assert(response["code"], qt.Equals, "internal")
		c.Assert(response["details"], qt.IsNil)
		c.Assert(response["internal_message"], qt.Equals, "This is a test error")
		c.Assert(response["message"], qt.Equals, "an internal error occurred")
	})

	// Run TypeScript tests
	c.Run("run TypeScript tests", func(c *qt.C) {
		err := RunTests(c.TB, appRoot, os.Stdout, os.Stderr, nil)
		c.Assert(err, qt.IsNil)
	})
}
