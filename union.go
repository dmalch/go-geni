package geni

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

type UnionRequest struct {
	// Marriage date and location
	Marriage *EventElement `json:"marriage,omitempty"`
	// Divorce date and location
	Divorce *EventElement `json:"divorce,omitempty"`
}

type UnionBulkResponse struct {
	Results []UnionResponse `json:"results,omitempty"`
}

type UnionResponse struct {
	// The union's id
	Id string `json:"id,omitempty"`
	// AdoptedChildren is a subset of the children array, indicating which children are adopted
	AdoptedChildren []string `json:"adopted_children,omitempty"`
	// Children is an array of children in the union (urls or ids, if requested)
	Children []string `json:"children,omitempty"`
	// FosterChildren is a subset of the children array, indicating which children are foster
	FosterChildren []string `json:"foster_children,omitempty"`
	// Partners is an array of partners in the union (urls or ids, if requested)
	Partners []string `json:"partners,omitempty"`
	// Marriage date and location
	Marriage *EventElement `json:"marriage,omitempty"`
	// Divorce date and location
	Divorce *EventElement `json:"divorce,omitempty"`
	// Status of the union (spouse|ex_spouse)
	Status string `json:"status,omitempty"`
}

func (c *Client) GetUnion(ctx context.Context, unionId string) (*UnionResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + unionId
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var union UnionResponse
	err = json.Unmarshal(body, &union)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &union, nil
}

func (c *Client) GetUnions(ctx context.Context, unionIds []string) (*UnionBulkResponse, error) {
	// Geni's bulk endpoint returns an empty results array when
	// `ids=` carries exactly one identifier — the server appears to
	// route single-id bulk requests through a search/filter path
	// rather than a fetch-by-id path. Fall back to the singular
	// endpoint and wrap the result so the caller sees a consistent
	// envelope regardless of input size.
	if len(unionIds) == 1 {
		one, err := c.GetUnion(ctx, unionIds[0])
		if err != nil {
			return nil, err
		}
		return &UnionBulkResponse{Results: []UnionResponse{*one}}, nil
	}

	url := BaseURL(c.useSandboxEnv) + "api/union"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	query := req.URL.Query()
	query.Add("ids", strings.Join(unionIds, ","))
	req.URL.RawQuery = query.Encode()

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var union UnionBulkResponse
	err = json.Unmarshal(body, &union)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &union, nil
}

// AddPartnerToUnion adds a new partner profile to an existing union
// and returns the newly-created profile. Geni's public docs describe
// the response as a union, but the live API returns the new partner
// profile (mirroring the profile-scoped [Client.AddPartner]); refetch
// the union via [Client.GetUnion] if you need the updated partner
// list.
func (c *Client) AddPartnerToUnion(ctx context.Context, unionId string) (*ProfileResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + unionId + "/add-partner"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profile ProfileResponse
	if err := json.Unmarshal(body, &profile); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	c.fixResponse(&profile)
	return &profile, nil
}

// AddChildToUnion adds a new child profile to an existing union and
// returns the newly-created profile. [WithModifier] selects "adopt" or
// "foster" to record an adopted/foster relationship — the modifier is
// stored on the union (in `adopted_children` / `foster_children`), so
// refetch via [Client.GetUnion] to confirm it took effect.
func (c *Client) AddChildToUnion(ctx context.Context, unionId string, opts ...AddOption) (*ProfileResponse, error) {
	url := BaseURL(c.useSandboxEnv) + "api/" + unionId + "/add-child"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	for _, opt := range opts {
		opt(req)
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var profile ProfileResponse
	if err := json.Unmarshal(body, &profile); err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}
	c.fixResponse(&profile)
	return &profile, nil
}

func (c *Client) UpdateUnion(ctx context.Context, unionId string, request *UnionRequest) (*UnionResponse, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		slog.Error("Error marshaling request", "error", err)
		return nil, err
	}

	jsonStr := strings.ReplaceAll(string(jsonBody), "\\\\", "\\")
	jsonStr = escapeString(jsonStr)

	url := BaseURL(c.useSandboxEnv) + "api/" + unionId + "/update"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(jsonStr))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	body, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var union UnionResponse
	err = json.Unmarshal(body, &union)
	if err != nil {
		slog.Error("Error unmarshaling response", "error", err)
		return nil, err
	}

	return &union, nil
}
