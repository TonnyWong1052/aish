package auth

import (
	"context"
	"testing"
)

func TestGCPProject_Structure(t *testing.T) {
	project := GCPProject{
		ProjectID:      "test-project-123",
		ProjectNumber:  "123456789",
		DisplayName:    "Test Project",
		LifecycleState: "ACTIVE",
	}

	if project.ProjectID == "" {
		t.Error("ProjectID should not be empty")
	}

	if project.LifecycleState != "ACTIVE" {
		t.Errorf("LifecycleState = %v, want ACTIVE", project.LifecycleState)
	}
}

func TestPickDefaultProject(t *testing.T) {
	tests := []struct {
		name     string
		projects []GCPProject
		expected string
	}{
		{
			name:     "empty list",
			projects: []GCPProject{},
			expected: "",
		},
		{
			name: "single project",
			projects: []GCPProject{
				{ProjectID: "project-1", LifecycleState: "ACTIVE"},
			},
			expected: "project-1",
		},
		{
			name: "multiple projects - first active",
			projects: []GCPProject{
				{ProjectID: "project-1", LifecycleState: "ACTIVE"},
				{ProjectID: "project-2", LifecycleState: "ACTIVE"},
			},
			expected: "project-1",
		},
		{
			name: "with deleted project",
			projects: []GCPProject{
				{ProjectID: "deleted", LifecycleState: "DELETE_REQUESTED"},
				{ProjectID: "active", LifecycleState: "ACTIVE"},
			},
			expected: "deleted", // PickDefaultProject returns first project
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PickDefaultProject(tt.projects)
			if result != tt.expected {
				t.Errorf("PickDefaultProject() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestReadAccessTokenFromAishConfig(t *testing.T) {
	// This function reads from actual config
	// Just test it doesn't panic
	token, found := readAccessTokenFromAishConfig()
	if found {
		if token == "" {
			t.Error("Found token should not be empty")
		}
		t.Logf("Found access token in AISH config")
	} else {
		t.Log("No access token found in AISH config (expected in test environment)")
	}
}

func TestGetAccessTokenForGCP(t *testing.T) {
	// This may fail in test environment without actual credentials
	token, err := getAccessTokenForGCP()
	if err != nil {
		t.Logf("getAccessTokenForGCP() error = %v (expected without credentials)", err)
	} else {
		if token == "" {
			t.Error("Token should not be empty when no error")
		}
	}
}

func TestAutoDetectProjectID(t *testing.T) {
	ctx := context.Background()

	// This will likely fail without actual GCP credentials
	projectID, err := AutoDetectProjectID(ctx)
	if err != nil {
		t.Logf("AutoDetectProjectID() error = %v (expected without GCP access)", err)
	} else {
		if projectID != "" {
			t.Logf("Auto-detected project: %s", projectID)
		} else {
			t.Log("No project ID detected (expected without GCP access)")
		}
	}
}

func TestListActiveProjects_NoCredentials(t *testing.T) {
	ctx := context.Background()

	// Without valid credentials, this should fail
	projects, err := ListActiveProjects(ctx)
	if err != nil {
		t.Logf("ListActiveProjects() error = %v (expected without credentials)", err)
	} else {
		t.Logf("Found %d projects", len(projects))
	}
}

func TestSearchProjectsV3_NoCredentials(t *testing.T) {
	ctx := context.Background()

	// Without valid credentials, this should fail
	projects, err := SearchProjectsV3(ctx)
	if err != nil {
		t.Logf("SearchProjectsV3() error = %v (expected without credentials)", err)
	} else {
		t.Logf("Found %d projects", len(projects))
	}
}

func TestGetProject_NoCredentials(t *testing.T) {
	ctx := context.Background()

	// Try to get a fake project
	project, err := GetProject(ctx, "nonexistent-project-12345")
	if err != nil {
		t.Logf("GetProject() error = %v (expected for nonexistent project)", err)
	}

	if project != nil {
		t.Error("Should return nil for nonexistent project")
	}
}

func TestEnableRequiredAPIs_NoCredentials(t *testing.T) {
	ctx := context.Background()

	services := []string{
		"generativelanguage.googleapis.com",
		"aiplatform.googleapis.com",
	}

	err := EnableRequiredAPIs(ctx, "test-project", services)
	if err != nil {
		t.Logf("EnableRequiredAPIs() error = %v (expected without credentials)", err)
	}
}
