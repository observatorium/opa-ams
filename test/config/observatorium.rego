package observatorium

import input

default allow = false

allow {
  input.permission == "read"
  input.resource == "metrics"
  input.tenant == "test-delegate-authz"
} else {
  response := http.send({"method": "post", "url": "http://127.0.0.1:8080/v1/data/observatorium/allow", "body": {"input": input}})
  response.status_code == 200
  json.unmarshal(response.raw_body).result == true
}
