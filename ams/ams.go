package ams

// AccessReviewEndpoint is the AMS access review endpoint.
const AccessReviewEndpoint = "/api/authorizations/v1/access_review"

// AccessReview models the struct expected by the AMS access review endpoint.
type AccessReview struct {
	Action          string `json:"action"`
	AccountUsername string `json:"account_username"`
	OrganizationID  string `json:"organization_id"`
	ResourceType    string `json:"resource_type"`
}
