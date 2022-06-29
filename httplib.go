// Package httplib abstracts common HTTP types, interfaces and settings
// to avoid having a single large HTTP call in each project
// to use, create a new HTTP request or client per call needed
package httplib

import (
	"bytes"
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// FormRequest contains basic fields needed for a HTTP request
// it has a method of FormRequest which returns a *http.Request
type FormRequest struct {
	BaseURL  string
	Endpoint string
	Payload  []byte
	Method   string
}

// DefaultClient provides a default client with 10s timeout
func DefaultClient(req *http.Request) (*http.Response, error) {
	c := NewClient{
		Transport:     nil,
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       10 * time.Second,
	}
	return c.DoRequest(req)
}

// FormRequest creates a new HTTP request
func (r FormRequest) FormRequest() (*http.Request, error) {
	var (
		URL    string
		req    *http.Request
		reqErr error
	)

	URL = r.BaseURL + r.Endpoint
	log.Debugf("URL: %s\n", URL)

	req, reqErr = http.NewRequest(r.Method, URL, bytes.NewBuffer(r.Payload))
	if reqErr != nil {
		log.Debugln("Error forming HTTP request")
		return nil, reqErr
	}
	return req, nil
}

// Headers sets a key, value to add to the *http.Request
// call each time a header is needed to be set.
// Used in method AddHeader()
type Headers struct {
	Key   string
	Value string
}

// AddHeader sets headers to a *http.Request
func (h Headers) AddHeader(req *http.Request) *http.Request {
	req.Header.Add(h.Key, h.Value)
	return req
}

type NewClient http.Client

// DoRequest performs the HTTP request and return the response
func (c NewClient) DoRequest(req *http.Request) (*http.Response, error) {
	client := http.Client{Transport: c.Transport, CheckRedirect: c.CheckRedirect, Jar: c.Jar, Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorln("Error performing HTTP request")
		return nil, err
	}
	return resp, nil
}

// ReadRespBody reads and return HTTP response without a buffer. Larger requests should be processed with buffers
func ReadRespBody(resp *http.Response) ([]byte, error) {
	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	_ = resp.Body.Close() // ignore err for the linter
	return body, nil
}

// ProcessStatusCode process the status codes
// 200 and 400 return a body with error
// 429 will sleep for 60s
// 500 returns only an error
// if none of the http code categories is appropriate
// assume a good response and return the body
func ProcessStatusCode(r *http.Response) ([]byte, error) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorln("error reading http body")
	}

	// switch between status code types and return body, error when necessary
	switch {
	case strings.HasPrefix(r.Status, "2"):
		return body, nil

	case strings.HasPrefix(r.Status, "4"):
		// sleep for 60s if rate limit exceeded
		if r.StatusCode == http.StatusTooManyRequests {
			time.Sleep(60 * time.Second) // sleeping now for good measure
			return nil, errors.New("rate limit exceed")
		}
		return body, errors.New("40X received; check request")

	case strings.HasPrefix(r.Status, "5"):
		return nil, errors.New("50X received; check network/service availability")

	// catch all in case of an odd status code
	default:
		return body, nil
	}

}

// DefaultRequest provides a standardized way to perform HTTP calls
func DefaultRequest(req *FormRequest, headers []Headers) ([]byte, error) {
	r, err := req.FormRequest()
	if err != nil {
		log.Errorln("Incorrect parameters set in form request")
		return nil, err
	}

	// add each header provided to the request
	for i := 0; i < len(headers); i++ {
		headers[i].AddHeader(r)
	}

	resp, err := DefaultClient(r)
	if err != nil {
		return nil, err
	}

	data, err := ProcessStatusCode(resp)

	return data, err
}
