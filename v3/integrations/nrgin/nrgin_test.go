package nrgin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/internal"
	"github.com/newrelic/go-agent/v3/internal/integrationsupport"
	newrelic "github.com/newrelic/go-agent/v3/newrelic"
)

var (
	pkg = "github.com/newrelic/go-agent/v3/integrations/nrgin"
)

func hello(c *gin.Context) {
	c.Writer.WriteString("hello response")
}

func TestBasicRoute(t *testing.T) {
	app := integrationsupport.NewBasicTestApp()
	router := gin.Default()
	router.Use(Middleware(app.Application))
	router.GET("/hello", hello)

	response := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	router.ServeHTTP(response, req)
	if respBody := response.Body.String(); respBody != "hello response" {
		t.Error("wrong response body", respBody)
	}
	app.ExpectTxnMetrics(t, internal.WantTxn{
		Name:  "GET " + pkg + ".hello",
		IsWeb: true,
	})
}

func TestRouterGroup(t *testing.T) {
	app := integrationsupport.NewBasicTestApp()
	router := gin.Default()
	router.Use(Middleware(app.Application))
	group := router.Group("/group")
	group.GET("/hello", hello)

	response := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/group/hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	router.ServeHTTP(response, req)
	if respBody := response.Body.String(); respBody != "hello response" {
		t.Error("wrong response body", respBody)
	}
	app.ExpectTxnMetrics(t, internal.WantTxn{
		Name:  "GET " + pkg + ".hello",
		IsWeb: true,
	})
}

func TestAnonymousHandler(t *testing.T) {
	app := integrationsupport.NewBasicTestApp()
	router := gin.Default()
	router.Use(Middleware(app.Application))
	router.GET("/anon", func(c *gin.Context) {
		c.Writer.WriteString("anonymous function handler")
	})

	response := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/anon", nil)
	if err != nil {
		t.Fatal(err)
	}
	router.ServeHTTP(response, req)
	if respBody := response.Body.String(); respBody != "anonymous function handler" {
		t.Error("wrong response body", respBody)
	}
	app.ExpectTxnMetrics(t, internal.WantTxn{
		Name:  "GET " + pkg + ".TestAnonymousHandler.func1",
		IsWeb: true,
	})
}

func multipleWriteHeader(c *gin.Context) {
	// Unlike http.ResponseWriter, gin.ResponseWriter does not immediately
	// write the first WriteHeader.  Instead, it gets buffered until the
	// first Write call.
	c.Writer.WriteHeader(200)
	c.Writer.WriteHeader(500)
	c.Writer.WriteString("multipleWriteHeader")
}

func TestMultipleWriteHeader(t *testing.T) {
	app := integrationsupport.NewBasicTestApp()
	router := gin.Default()
	router.Use(Middleware(app.Application))
	router.GET("/header", multipleWriteHeader)

	response := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/header", nil)
	if err != nil {
		t.Fatal(err)
	}
	router.ServeHTTP(response, req)
	if respBody := response.Body.String(); respBody != "multipleWriteHeader" {
		t.Error("wrong response body", respBody)
	}
	if response.Code != 500 {
		t.Error("wrong response code", response.Code)
	}
	// Error metrics test the 500 response code capture.
	app.ExpectTxnMetrics(t, internal.WantTxn{
		Name:      "GET " + pkg + ".multipleWriteHeader",
		IsWeb:     true,
		NumErrors: 1,
	})
}

func accessTransactionGinContext(c *gin.Context) {
	txn := Transaction(c)
	txn.NoticeError(errors.New("problem"))
	c.Writer.WriteString("accessTransactionGinContext")
}

func TestContextTransaction(t *testing.T) {
	app := integrationsupport.NewBasicTestApp()
	router := gin.Default()
	router.Use(Middleware(app.Application))
	router.GET("/txn", accessTransactionGinContext)

	response := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/txn", nil)
	if err != nil {
		t.Fatal(err)
	}
	router.ServeHTTP(response, req)
	if respBody := response.Body.String(); respBody != "accessTransactionGinContext" {
		t.Error("wrong response body", respBody)
	}
	if response.Code != 200 {
		t.Error("wrong response code", response.Code)
	}
	app.ExpectTxnMetrics(t, internal.WantTxn{
		Name:      "GET " + pkg + ".accessTransactionGinContext",
		IsWeb:     true,
		NumErrors: 1,
	})
}

func TestNilApp(t *testing.T) {
	var app *newrelic.Application
	router := gin.Default()
	router.Use(Middleware(app))
	router.GET("/hello", hello)

	response := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	router.ServeHTTP(response, req)
	if respBody := response.Body.String(); respBody != "hello response" {
		t.Error("wrong response body", respBody)
	}
}

func errorStatus(c *gin.Context) {
	c.String(500, "an error happened")
}

const (
	// The Gin.Context.Status method behavior changed with this pull
	// request: https://github.com/gin-gonic/gin/pull/1606.  This change
	// affects our ability to instrument the response code. In Gin v1.4.0
	// and below, we always recorded a 200 status, whereas with newer Gin
	// versions we now correctly capture the status.
	statusFixVersion = "v1.5.0"
)

func TestStatusCodes(t *testing.T) {
	// Test that we are correctly able to collect status code.
	expectCode := 200
	if gin.Version == statusFixVersion {
		expectCode = 500
	}
	app := integrationsupport.NewBasicTestApp()
	router := gin.Default()
	router.Use(Middleware(app.Application))
	router.GET("/err", errorStatus)

	response := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/err", nil)
	if err != nil {
		t.Fatal(err)
	}
	router.ServeHTTP(response, req)
	if respBody := response.Body.String(); respBody != "an error happened" {
		t.Error("wrong response body", respBody)
	}
	if response.Code != 500 {
		t.Error("wrong response code", response.Code)
	}
	app.ExpectTxnEvents(t, []internal.WantEvent{{
		Intrinsics: map[string]interface{}{
			"name":             "WebTransaction/Go/GET " + pkg + ".errorStatus",
			"nr.apdexPerfZone": internal.MatchAnything,
		},
		UserAttributes: map[string]interface{}{},
		AgentAttributes: map[string]interface{}{
			"httpResponseCode":             expectCode,
			"http.statusCode":              expectCode,
			"request.method":               "GET",
			"request.uri":                  "/err",
			"response.headers.contentType": "text/plain; charset=utf-8",
		},
	}})
}

func noBody(c *gin.Context) {
	c.Status(500)
}

func TestNoResponseBody(t *testing.T) {
	// Test that when no response body is sent (i.e. c.Writer.Write is never
	// called) that we still capture status code.
	expectCode := 200
	if gin.Version == statusFixVersion {
		expectCode = 500
	}
	app := integrationsupport.NewBasicTestApp()
	router := gin.Default()
	router.Use(Middleware(app.Application))
	router.GET("/nobody", noBody)

	response := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/nobody", nil)
	if err != nil {
		t.Fatal(err)
	}
	router.ServeHTTP(response, req)
	if respBody := response.Body.String(); respBody != "" {
		t.Error("wrong response body", respBody)
	}
	if response.Code != 500 {
		t.Error("wrong response code", response.Code)
	}
	app.ExpectTxnEvents(t, []internal.WantEvent{{
		Intrinsics: map[string]interface{}{
			"name":             "WebTransaction/Go/GET " + pkg + ".noBody",
			"nr.apdexPerfZone": internal.MatchAnything,
		},
		UserAttributes: map[string]interface{}{},
		AgentAttributes: map[string]interface{}{
			"httpResponseCode": expectCode,
			"http.statusCode":  expectCode,
			"request.method":   "GET",
			"request.uri":      "/nobody",
		},
	}})
}
