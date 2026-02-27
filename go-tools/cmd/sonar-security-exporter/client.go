package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const pageSize = 500

// Client is a thin SonarCloud REST API client.
// It uses HTTP Basic auth with the token as username and an empty password,
// which is the authentication scheme documented by Sonar.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{},
	}
}

func (c *Client) get(path string, params url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.token, "")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// FetchVulnerabilities retrieves all VULNERABILITY issues for a project,
// paginating automatically until all results are collected.
func (c *Client) FetchVulnerabilities(projectKey, branch, pullRequest string, tags []string) ([]Issue, error) {
	type response struct {
		Total  int     `json:"total"`
		Issues []Issue `json:"issues"`
	}

	var all []Issue
	for page := 1; ; page++ {
		params := url.Values{
			"projectKeys": {projectKey},
			"types":       {"VULNERABILITY"},
			"ps":          {strconv.Itoa(pageSize)},
			"p":           {strconv.Itoa(page)},
		}
		if branch != "" {
			params.Set("branch", branch)
		}
		if pullRequest != "" {
			params.Set("pullRequest", pullRequest)
		}
		if len(tags) > 0 {
			params.Set("tags", strings.Join(tags, ","))
		}

		body, err := c.get("/api/issues/search", params)
		if err != nil {
			return nil, fmt.Errorf("vulnerabilities page %d: %w", page, err)
		}
		var r response
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		all = append(all, r.Issues...)
		if len(all) >= r.Total {
			break
		}
	}
	return all, nil
}

// FetchHotspots retrieves all security hotspots for a project,
// paginating automatically until all results are collected.
func (c *Client) FetchHotspots(projectKey, branch, pullRequest string) ([]Hotspot, error) {
	type paging struct {
		Total int `json:"total"`
	}
	type response struct {
		Paging   paging    `json:"paging"`
		Hotspots []Hotspot `json:"hotspots"`
	}

	var all []Hotspot
	for page := 1; ; page++ {
		params := url.Values{
			"projectKey": {projectKey},
			"ps":         {strconv.Itoa(pageSize)},
			"p":          {strconv.Itoa(page)},
		}
		if branch != "" {
			params.Set("branch", branch)
		}
		if pullRequest != "" {
			params.Set("pullRequest", pullRequest)
		}

		body, err := c.get("/api/hotspots/search", params)
		if err != nil {
			return nil, fmt.Errorf("hotspots page %d: %w", page, err)
		}
		var r response
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		all = append(all, r.Hotspots...)
		if len(all) >= r.Paging.Total {
			break
		}
	}
	return all, nil
}

// FetchProjects retrieves all projects in an organization,
// paginating automatically until all results are collected.
func (c *Client) FetchProjects(org string) ([]Project, error) {
	type paging struct {
		Total int `json:"total"`
	}
	type response struct {
		Paging     paging    `json:"paging"`
		Components []Project `json:"components"`
	}

	var all []Project
	for page := 1; ; page++ {
		params := url.Values{
			"organization": {org},
			"qualifiers":   {"TRK"},
			"ps":           {"500"},
			"p":            {strconv.Itoa(page)},
		}

		body, err := c.get("/api/components/search", params)
		if err != nil {
			return nil, fmt.Errorf("projects page %d: %w", page, err)
		}
		var r response
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		all = append(all, r.Components...)
		if len(all) >= r.Paging.Total {
			break
		}
	}
	return all, nil
}
