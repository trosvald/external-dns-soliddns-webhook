package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/trosvald/external-dns-soliddns-webhook/cmd/webhook/init/configuration"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"

	log "github.com/sirupsen/logrus"
)

type testCase struct {
	name                      string
	returnRecords             []*endpoint.Endpoint
	returnAdjustedEndpoints   []*endpoint.Endpoint
	returnDomainFilter        endpoint.DomainFilter
	hasError                  error
	method                    string
	path                      string
	headers                   map[string]string
	body                      string
	expectedStatusCode        int
	expectedResponseHeaders   map[string]string
	expectedBody              string
	expectedChanges           *plan.Changes
	expectedEndpointsToAdjust []*endpoint.Endpoint
	log.Ext1FieldLogger
}

var mockProvider *MockProvider

func TestMain(m *testing.M) {
	mockProvider = &MockProvider{}

	go func() {
		srv := NewServer()
		srv.StartHealth(configuration.Init())
		srv.Start(configuration.Init(), mockProvider)
	}()

	time.Sleep(300 * time.Second)
	m.Run()
}

func TestRecords(t *testing.T) {
	testCases := []testCase{
		{
			name: "valid case",
			returnRecords: []*endpoint.Endpoint{
				{
					DNSName:    "test.example.com",
					Targets:    []string{""},
					RecordType: "A",
					RecordTTL:  3600,
					Labels: map[string]string{
						"label1": "value1",
					},
				},
			},
			method:             http.MethodGet,
			headers:            map[string]string{"Accept": "application/external.dns.webhook+json;version=1"},
			path:               "/records",
			body:               "",
			expectedStatusCode: http.StatusOK,
			expectedResponseHeaders: map[string]string{
				"Content-Type": "application/external.dns.webhook+json;version=1",
			},
			expectedBody: "[{\"dnsName\":\"test.example.com\",\"targets\":[\"\"],\"recordType\":\"A\",\"recordTTL\":3600,\"labels\":{\"label1\":\"value1\"}}]",
		},
		{
			name:               "backend errpr",
			hasError:           fmt.Errorf("backend error"),
			method:             http.MethodGet,
			headers:            map[string]string{"Accept": "application/external.dns.webhook+json;version=1"},
			path:               "/records",
			body:               "",
			expectedStatusCode: http.StatusInternalServerError,
		},
	}

	executeTestCases(t, testCases)
}

func executeTestCases(t *testing.T, testCases []testCase) {
	log.SetLevel(log.DebugLevel)

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d. %s", i+1, tc.name), func(t *testing.T) {
			mockProvider.testCase = tc
			mockProvider.t = t

			var bodyReader io.Reader = strings.NewReader(tc.body)

			request, err := http.NewRequest(tc.method, "https://localhost:8888"+tc.path, bodyReader)
			if err != nil {
				t.Error(err)
			}

			for k, v := range tc.headers {
				request.Header.Set(k, v)
			}

			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Error(err)
			}

			if response.StatusCode != tc.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tc.expectedStatusCode, response.StatusCode)
			}

			for k, v := range tc.expectedResponseHeaders {
				if response.Header.Get(k) != v {
					t.Errorf("expected response header %s=%s, got %s", k, v, response.Header.Get(k))
				}
			}

			if tc.expectedBody != "" {
				body, err := io.ReadAll(response.Body)
				if err != nil {
					t.Error(err)
				}
				_ = response.Body.Close()
				actualTrimmedBody := strings.TrimSpace(string(body))
				if actualTrimmedBody != tc.expectedBody {
					t.Errorf("expected body %s, got %s", tc.expectedBody, actualTrimmedBody)
				}
			}
		})
	}
}

type MockProvider struct {
	t        *testing.T
	testCase testCase
}

func (d *MockProvider) Records(_ context.Context) ([]*endpoint.Endpoint, error) {
	return d.testCase.returnRecords, d.testCase.hasError
}

func (d *MockProvider) ApplyChanges(_ context.Context, changes *plan.Changes) error {
	if d.testCase.hasError != nil {
		return d.testCase.hasError
	}
	if !reflect.DeepEqual(changes, d.testCase.expectedChanges) {
		d.t.Errorf("expected changes '%v', got '%v'", d.testCase.expectedChanges, changes)
	}
	return nil
}

func (d *MockProvider) AdjustEndpoints(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	if !reflect.DeepEqual(endpoints, d.testCase.expectedEndpointsToAdjust) {
		d.t.Errorf("expected endpoints to adjust '%v', got '%v'", d.testCase.expectedEndpointsToAdjust, endpoints)
	}
	return d.testCase.returnAdjustedEndpoints, nil
}

func (d *MockProvider) GetDomainFilter() endpoint.DomainFilter {
	return d.testCase.returnDomainFilter
}
