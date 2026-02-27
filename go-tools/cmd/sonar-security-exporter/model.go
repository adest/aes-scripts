package main

// Issue represents a SonarCloud vulnerability from api/issues/search.
type Issue struct {
	Key          string   `json:"key"`
	Rule         string   `json:"rule"`
	Severity     string   `json:"severity"`
	Type         string   `json:"type"`
	Component    string   `json:"component"`
	Line         *int     `json:"line,omitempty"`
	Message      string   `json:"message"`
	Status       string   `json:"status"`
	Tags         []string `json:"tags,omitempty"`
	CreationDate string   `json:"creationDate"`
	UpdateDate   string   `json:"updateDate,omitempty"`
}

// Hotspot represents a SonarCloud security hotspot from api/hotspots/search.
type Hotspot struct {
	Key                      string `json:"key"`
	Component                string `json:"component"`
	SecurityCategory         string `json:"securityCategory"`
	VulnerabilityProbability string `json:"vulnerabilityProbability"`
	Status                   string `json:"status"`
	Resolution               string `json:"resolution,omitempty"`
	Line                     *int   `json:"line,omitempty"`
	Message                  string `json:"message"`
	Author                   string `json:"author,omitempty"`
	CreationDate             string `json:"creationDate"`
}

// Project represents a SonarCloud project from api/components/search.
type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// ExportResult is the root JSON object written by the export command.
type ExportResult struct {
	Project              string    `json:"project"`
	Organization         string    `json:"organization,omitempty"`
	ExportedAt           string    `json:"exported_at"`
	Branch               string    `json:"branch,omitempty"`
	PullRequest          string    `json:"pull_request,omitempty"`
	TotalVulnerabilities int       `json:"total_vulnerabilities"`
	TotalHotspots        int       `json:"total_hotspots"`
	Vulnerabilities      []Issue   `json:"vulnerabilities"`
	Hotspots             []Hotspot `json:"hotspots"`
}
