package cloudtrace

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
)

// gcpProjectID is the GCP Project ID for this process.
var (
	gcpProjectID   = ""
	gcpProjectLoad sync.Once
)

// GcpProjectID returns the GCP Project ID for this process.
//
// In order of precdecence:
// 1. Look for GCP_PROJECT environment variable
// 2. Look for GCLOUD_PROJECT environment variable
// 3. Look for GOOGLE_CLOUD_PROJECT environment variable
// 4. Check for GOOGLE_APPLICATION_CREDENTIALS JSON file
// 5. GCE project ID from metadata server
//
// This code is derived from: https://github.com/googleapis/google-auth-library-nodejs/blob/070ec96b78dc26791bacb452ebef13d0a5ae6b18/src/auth/googleauth.ts#L245
//
// If it fails or panics, it will return an empty string and will not attempt to load again.
func GcpProjectID() string {
	gcpProjectLoad.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				gcpProjectID = ""
			}
		}()

		gcpProjectID = gcpProjectIDFromEnv()

		if gcpProjectID == "" {
			gcpProjectID = gcpProjectIDFromCredsFile()
		}

		if gcpProjectID == "" {
			gcpProjectID = gcpProjectIDFromMetadata()
		}
	})

	return gcpProjectID
}

func gcpProjectIDFromEnv() string {
	switch {
	case os.Getenv("GCP_PROJECT") != "":
		return os.Getenv("GCP_PROJECT")
	case os.Getenv("GCLOUD_PROJECT") != "":
		return os.Getenv("GCLOUD_PROJECT")
	case os.Getenv("GOOGLE_CLOUD_PROJECT") != "":
		return os.Getenv("GOOGLE_CLOUD_PROJECT")
	case os.Getenv("gcp_project") != "":
		return os.Getenv("gcp_project")
	case os.Getenv("gcloud_project") != "":
		return os.Getenv("gcloud_project")
	case os.Getenv("google_cloud_project") != "":
		return os.Getenv("google_cloud_project")
	default:
		return ""
	}
}

func gcpProjectIDFromCredsFile() string {
	creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if creds == "" {
		creds = os.Getenv("google_application_credentials")
	}
	if creds == "" {
		return ""
	}

	// Read the file
	contents, err := os.ReadFile(creds)
	if err != nil {
		return ""
	}

	// Unmarshal the JSON
	type Creds struct {
		ProjectID      string `json:"project_id,omitempty"`
		QuotaProjectID string `json:"quota_project_id,omitempty"`
	}
	var c Creds
	if err := json.Unmarshal(contents, &c); err != nil {
		return ""
	}

	// Return the ProjectID if set
	switch {
	case c.ProjectID != "":
		return c.ProjectID
	case c.QuotaProjectID != "":
		return c.QuotaProjectID
	}

	return ""
}

func gcpProjectIDFromMetadata() string {
	const url = "http://metadata.google.internal/computeMetadata/v1/project/project-id"
	// nosemgrep
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Add("Metadata-Flavor", "Google")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	projectIDBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	return string(projectIDBytes)
}
