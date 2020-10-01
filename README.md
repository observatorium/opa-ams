# OPA-AMS

[![CircleCI](https://circleci.com/gh/observatorium/opa-ams.svg?style=svg)](https://circleci.com/gh/observatorium/opa-ams)
[![Go Report Card](https://goreportcard.com/badge/github.com/observatorium/opa-ams)](https://goreportcard.com/report/github.com/observatorium/opa-ams)

`opa-ams` provides an Open Policy Agent (OPA) -compatible API for making access review requests against the OpenShift Account Management System (AMS) API.

## API

### POST /v1/data/{package}/{rule}

The `opa-ams` HTTP server exposes a single endpoint of the [OPA Data API](https://www.openpolicyagent.org/docs/latest/rest-api/#data-api) and fullfills requests by translating them into [AMS access reviews](https://api.openshift.com/?urls.primaryName=Accounts%20management%20service#/default/post_api_authorizations_v1_access_review).
This endpoint expects an OPA [Input Document](https://www.openpolicyagent.org/docs/latest/kubernetes-primer/#input-document) in the body of the request with the following structure:

```json
{
    "input": {
        "groups": ["string"],
        "permission": "string",
        "resource": "string",
        "subject": "string",
        "tenant": "string"
    }
}
```

It returns a response with the following structure:

```json
{
    "result": boolean
}
```

## Usage

[embedmd]:# (tmp/help.txt)
```txt
Usage of ./opa-ams:
      --ams-url string                An AMS URL against which to authorize client requests.
      --debug.name string             A name to add as a prefix to log lines. (default "opa-ams")
      --log.format string             The log format to use. Options: 'logfmt', 'json'. (default "logfmt")
      --log.level string              The log filtering level. Options: 'error', 'warn', 'info', 'debug'. (default "info")
      --memcached strings             One or more Memcached server addresses.
      --memcached.expire int32        Time after which keys stored in Memcached should expire, given in seconds. (default 3600)
      --memcached.interval int32      The interval at which to update the Memcached DNS, given in seconds; use 0 to disable. (default 10)
      --oidc.audience string          The audience for whom the access token is intended, see https://openid.net/specs/openid-connect-core-1_0.html#IDToken.
      --oidc.client-id string         The OIDC client ID, see https://tools.ietf.org/html/rfc6749#section-2.3.
      --oidc.client-secret string     The OIDC client secret, see https://tools.ietf.org/html/rfc6749#section-2.3.
      --oidc.issuer-url string        The OIDC issuer URL, see https://openid.net/specs/openid-connect-discovery-1_0.html#IssuerDiscovery.
      --opa.package string            The name of the OPA package that opa-ams should implement, see https://www.openpolicyagent.org/docs/latest/policy-language/#packages.
      --opa.rule string               The name of the OPA rule for which opa-ams should provide a result, see https://www.openpolicyagent.org/docs/latest/policy-language/#rules. (default "allow")
      --resource-type-prefix string   A prefix to add to the resource name in AMS access review requests.
      --web.internal.listen string    The address on which the internal server listens. (default ":8081")
      --web.listen string             The address on which the public server listens. (default ":8080")
pflag: help requested
```
