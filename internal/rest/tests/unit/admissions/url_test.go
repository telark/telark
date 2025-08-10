package admissions_test

import (
    "strings"
    "testing"

    "github.com/plsyro/rest/base"
    requestutils "github.com/plsyro/rest/utils/request"
)

func TestAdmissionsURL_UsesHTTPSImplicit443(t *testing.T) {
    req := requestutils.CreateGenericRequest(
        base.Get,
        base.AdmissionOperator,
        base.V1,
        base.Endpoint("admissions/validation/test"),
    )

    url, err := req.GenerateURL()
    if err != nil {
        t.Fatalf("GenerateURL() error = %v", err)
    }

    if !strings.HasPrefix(url, string(base.HTTPS)) {
        t.Fatalf("expected HTTPS scheme, got: %s", url)
    }

    if strings.Contains(url, ":443/") {
        t.Fatalf("expected implicit 443 without explicit port, got: %s", url)
    }

    if !strings.Contains(url, "/"+string(base.V1)+"/") {
        t.Fatalf("expected API version segment '/%s/' in URL, got: %s", base.V1, url)
    }
}

func TestNonAdmissionsURL_UsesHTTPWith8080(t *testing.T) {
    req := requestutils.CreateGenericRequest(
        base.Get,
        base.Configurator,
        base.V1,
        base.Endpoint("health"),
    )

    url, err := req.GenerateURL()
    if err != nil {
        t.Fatalf("GenerateURL() error = %v", err)
    }

    if !strings.HasPrefix(url, string(base.HTTP)) {
        t.Fatalf("expected HTTP scheme, got: %s", url)
    }

    if !strings.Contains(url, ":8080/") {
        t.Fatalf("expected explicit ':8080' in URL, got: %s", url)
    }

    if !strings.Contains(url, "/"+string(base.V1)+"/") {
        t.Fatalf("expected API version segment '/%s/' in URL, got: %s", base.V1, url)
    }
}


