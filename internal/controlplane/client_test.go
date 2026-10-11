package controlplane

import (
	"net/url"
	"testing"
)

func TestEndpointURLPreservesBasePathAndQuery(t *testing.T) {
	base, _ := url.Parse("https://control.example/root/")
	got, err := endpointURL(base, "/api/agent/v1/tasks/next?nodeId=node%2Fa")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "https://control.example/root/api/agent/v1/tasks/next?nodeId=node%2Fa" {
		t.Fatalf("unexpected endpoint URL: %s", got)
	}
}

func TestEndpointURLEscapesTaskIDAsSinglePathSegment(t *testing.T) {
	base, _ := url.Parse("https://control.example")
	got, err := endpointURL(base, "/api/agent/v1/tasks/"+url.PathEscape("task/a")+"/result")
	if err != nil {
		t.Fatal(err)
	}
	if got.EscapedPath() != "/api/agent/v1/tasks/task%2Fa/result" {
		t.Fatalf("task ID escaped incorrectly: %s", got.EscapedPath())
	}
}
