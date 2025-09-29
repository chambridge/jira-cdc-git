package jobs

import (
	"context"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/fake"
)

func TestJobIDGenerator(t *testing.T) {
	generator := NewJobIDGenerator()

	t.Run("Generate", func(t *testing.T) {
		jobID := generator.Generate("test")
		if jobID == "" {
			t.Error("Expected non-empty job ID")
		}

		// Should start with prefix
		if len(jobID) < 4 || jobID[:4] != "test" {
			t.Errorf("Expected job ID to start with 'test', got: %s", jobID)
		}

		// Should be unique
		jobID2 := generator.Generate("test")
		if jobID == jobID2 {
			t.Error("Expected unique job IDs")
		}
	})

	t.Run("GenerateWithType", func(t *testing.T) {
		jobID := generator.GenerateWithType(JobTypeSingle)
		if jobID == "" {
			t.Error("Expected non-empty job ID")
		}

		// Should start with job type
		if len(jobID) < 6 || jobID[:6] != "single" {
			t.Errorf("Expected job ID to start with 'single', got: %s", jobID)
		}
	})

	t.Run("Validate", func(t *testing.T) {
		tests := []struct {
			name    string
			jobID   string
			wantErr bool
		}{
			{"valid", "single-20230101-120000-abcd1234", false},
			{"empty", "", true},
			{"too short", "abc", true},
			{"too long", "a" + string(make([]byte, 70)), true},
			{"invalid chars", "job_with_underscore", true},
			{"valid with dash", "job-with-dash", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := generator.Validate(tt.jobID)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestJobTemplateManager(t *testing.T) {
	manager := NewFileJobTemplateManager()

	t.Run("GetTemplate", func(t *testing.T) {
		// Test getting single job template
		template, err := manager.GetTemplate(JobTypeSingle)
		if err != nil {
			t.Fatalf("Failed to get single job template: %v", err)
		}

		if template.JobType != JobTypeSingle {
			t.Errorf("Expected job type %s, got %s", JobTypeSingle, template.JobType)
		}

		if template.Template == nil {
			t.Error("Expected non-nil template")
		}

		// Verify template structure
		job := template.Template
		if len(job.Spec.Template.Spec.Containers) == 0 {
			t.Error("Expected at least one container in job template")
		}

		container := job.Spec.Template.Spec.Containers[0]
		if container.Name != "sync-worker" {
			t.Errorf("Expected container name 'sync-worker', got %s", container.Name)
		}
	})

	t.Run("ValidateTemplate", func(t *testing.T) {
		template, _ := manager.GetTemplate(JobTypeBatch)
		err := manager.ValidateTemplate(template)
		if err != nil {
			t.Errorf("Expected valid template, got error: %v", err)
		}
	})

	t.Run("GetAllJobTypes", func(t *testing.T) {
		jobTypes := []JobType{JobTypeSingle, JobTypeBatch, JobTypeJQL}

		for _, jobType := range jobTypes {
			template, err := manager.GetTemplate(jobType)
			if err != nil {
				t.Errorf("Failed to get template for %s: %v", jobType, err)
			}

			if template.JobType != jobType {
				t.Errorf("Expected job type %s, got %s", jobType, template.JobType)
			}
		}
	})
}

func TestJobErrors(t *testing.T) {
	t.Run("ValidationError", func(t *testing.T) {
		err := NewValidationError("job-123", "field1", "value1", "test message")
		if err.JobID != "job-123" {
			t.Errorf("Expected job ID 'job-123', got %s", err.JobID)
		}
		if err.Type() != ErrorTypeValidation {
			t.Errorf("Expected error type %s, got %s", ErrorTypeValidation, err.Type())
		}
	})

	t.Run("ErrorSummary", func(t *testing.T) {
		err := NewKubernetesError("job-123", "create", "job", "failed to create job")
		summary := SummarizeError(err)

		if summary.Type != ErrorTypeKubernetes {
			t.Errorf("Expected error type %s, got %s", ErrorTypeKubernetes, summary.Type)
		}

		if summary.JobID != "job-123" {
			t.Errorf("Expected job ID 'job-123', got %s", summary.JobID)
		}

		if !summary.Retryable {
			t.Error("Expected Kubernetes errors to be retryable")
		}

		if len(summary.Suggestions) == 0 {
			t.Error("Expected suggestions for Kubernetes errors")
		}
	})

	t.Run("IsRetryableError", func(t *testing.T) {
		tests := []struct {
			name      string
			err       error
			retryable bool
		}{
			{"validation", NewValidationError("job", "field", "value", "msg"), false},
			{"connection", NewConnectionError("job", "target", "http", "msg"), true},
			{"timeout", NewTimeoutError("job", time.Minute, time.Minute*2, "msg"), true},
			{"kubernetes", NewKubernetesError("job", "op", "res", "msg"), true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if IsRetryableError(tt.err) != tt.retryable {
					t.Errorf("IsRetryableError() = %v, want %v", IsRetryableError(tt.err), tt.retryable)
				}
			})
		}
	})
}

func TestSyncJobOrchestrator(t *testing.T) {
	// Create a mock Kubernetes client
	fakeClient := fake.NewSimpleClientset()

	// Create scheduler with fake client
	scheduler := &KubernetesJobScheduler{
		clientset:         fakeClient,
		namespace:         "test-namespace",
		defaultImage:      "jira-sync:test",
		templateManager:   NewFileJobTemplateManager(),
		credentialsSecret: "test-credentials",
		configMapName:     "test-config",
		pvcName:           "test-pvc",
	}

	orchestrator := NewSyncJobOrchestrator(scheduler)

	t.Run("SubmitSingleIssueSync", func(t *testing.T) {
		req := &SingleIssueSyncRequest{
			IssueKey:   "PROJ-123",
			Repository: "/workspace/repo",
			SafeMode:   true,
		}

		// Note: This will fail with the fake client since we don't have
		// the required secrets/configmaps, but we can test validation
		_, err := orchestrator.SubmitSingleIssueSync(context.Background(), req)

		// Should fail due to missing Kubernetes resources, not validation
		if err != nil {
			t.Logf("Expected error due to missing Kubernetes resources: %v", err)
		}

		// Test validation failures
		invalidReq := &SingleIssueSyncRequest{
			IssueKey:   "", // Invalid: empty issue key
			Repository: "/workspace/repo",
		}

		_, err = orchestrator.SubmitSingleIssueSync(context.Background(), invalidReq)
		if err == nil {
			t.Error("Expected validation error for empty issue key")
		}

		if validationErr, ok := err.(*ValidationError); ok {
			if validationErr.Type() != ErrorTypeValidation {
				t.Errorf("Expected validation error type, got %s", validationErr.Type())
			}
		}
	})

	t.Run("SubmitBatchSync", func(t *testing.T) {
		req := &BatchSyncRequest{
			IssueKeys:   []string{"PROJ-1", "PROJ-2", "PROJ-3"},
			Repository:  "/workspace/repo",
			Concurrency: 2,
			SafeMode:    true,
		}

		_, err := orchestrator.SubmitBatchSync(context.Background(), req)
		if err != nil {
			t.Logf("Expected error due to missing Kubernetes resources: %v", err)
		}

		// Test validation
		invalidReq := &BatchSyncRequest{
			IssueKeys:  []string{}, // Invalid: empty issue list
			Repository: "/workspace/repo",
		}

		_, err = orchestrator.SubmitBatchSync(context.Background(), invalidReq)
		if err == nil {
			t.Error("Expected validation error for empty issue list")
		}
	})

	t.Run("SubmitJQLSync", func(t *testing.T) {
		req := &JQLSyncRequest{
			JQL:        "project = PROJ AND status = 'To Do'",
			Repository: "/workspace/repo",
			SafeMode:   true,
		}

		_, err := orchestrator.SubmitJQLSync(context.Background(), req)
		if err != nil {
			t.Logf("Expected error due to missing Kubernetes resources: %v", err)
		}

		// Test validation
		invalidReq := &JQLSyncRequest{
			JQL:        "", // Invalid: empty JQL
			Repository: "/workspace/repo",
		}

		_, err = orchestrator.SubmitJQLSync(context.Background(), invalidReq)
		if err == nil {
			t.Error("Expected validation error for empty JQL")
		}
	})
}

func TestJobConfiguration(t *testing.T) {
	t.Run("DefaultJobConfiguration", func(t *testing.T) {
		config := DefaultJobConfiguration()

		if config.DefaultNamespace != DefaultNamespace {
			t.Errorf("Expected namespace %s, got %s", DefaultNamespace, config.DefaultNamespace)
		}

		if config.MaxConcurrency != MaxConcurrency {
			t.Errorf("Expected max concurrency %d, got %d", MaxConcurrency, config.MaxConcurrency)
		}

		if !config.EnableSafeMode {
			t.Error("Expected safe mode to be enabled by default")
		}
	})

	t.Run("ValidateConfiguration", func(t *testing.T) {
		validConfig := DefaultJobConfiguration()
		err := ValidateConfiguration(validConfig)
		if err != nil {
			t.Errorf("Expected valid configuration, got error: %v", err)
		}

		// Test invalid configurations
		tests := []struct {
			name   string
			modify func(*JobConfiguration)
		}{
			{"empty namespace", func(c *JobConfiguration) { c.DefaultNamespace = "" }},
			{"empty image", func(c *JobConfiguration) { c.DefaultImage = "" }},
			{"zero timeout", func(c *JobConfiguration) { c.DefaultTimeout = 0 }},
			{"invalid concurrency", func(c *JobConfiguration) { c.MaxConcurrency = 0 }},
			{"excessive concurrency", func(c *JobConfiguration) { c.MaxConcurrency = 20 }},
			{"invalid batch size", func(c *JobConfiguration) { c.MaxBatchSize = 0 }},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := DefaultJobConfiguration()
				tt.modify(config)

				err := ValidateConfiguration(config)
				if err == nil {
					t.Errorf("Expected validation error for %s", tt.name)
				}
			})
		}
	})
}

func TestParseJobID(t *testing.T) {
	tests := []struct {
		name     string
		jobID    string
		wantErr  bool
		wantType JobType
	}{
		{
			name:     "single job",
			jobID:    "single-20230101-120000-abcd1234",
			wantErr:  false,
			wantType: JobTypeSingle,
		},
		{
			name:     "batch job",
			jobID:    "batch-20230101-120000-efgh5678",
			wantErr:  false,
			wantType: JobTypeBatch,
		},
		{
			name:     "jql job",
			jobID:    "jql-20230101-120000-ijkl9012",
			wantErr:  false,
			wantType: JobTypeJQL,
		},
		{
			name:    "invalid format",
			jobID:   "invalid",
			wantErr: true,
		},
		{
			name:    "empty",
			jobID:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseJobID(tt.jobID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJobID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if info.JobType != tt.wantType {
					t.Errorf("ParseJobID() job type = %v, want %v", info.JobType, tt.wantType)
				}

				if info.OriginalID != tt.jobID {
					t.Errorf("ParseJobID() original ID = %v, want %v", info.OriginalID, tt.jobID)
				}
			}
		})
	}
}

func TestFormatJobName(t *testing.T) {
	tests := []struct {
		name  string
		jobID string
		want  string
	}{
		{
			name:  "normal job ID",
			jobID: "single-20230101-120000-abcd1234",
			want:  "single-20230101-120000-abcd1234",
		},
		{
			name:  "with underscores",
			jobID: "job_with_underscores",
			want:  "job-with-underscores",
		},
		{
			name:  "with dots",
			jobID: "job.with.dots",
			want:  "job-with-dots",
		},
		{
			name:  "uppercase",
			jobID: "JOB-UPPERCASE",
			want:  "job-uppercase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatJobName(tt.jobID); got != tt.want {
				t.Errorf("FormatJobName() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark tests
func BenchmarkJobIDGeneration(b *testing.B) {
	generator := NewJobIDGenerator()

	b.Run("Generate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			generator.Generate("bench")
		}
	})

	b.Run("GenerateWithType", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			generator.GenerateWithType(JobTypeSingle)
		}
	})

	b.Run("Validate", func(b *testing.B) {
		jobID := generator.Generate("bench")
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_ = generator.Validate(jobID)
		}
	})
}

func BenchmarkTemplateRetrieval(b *testing.B) {
	manager := NewFileJobTemplateManager()

	b.Run("GetSingleTemplate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = manager.GetTemplate(JobTypeSingle)
		}
	})

	b.Run("GetBatchTemplate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = manager.GetTemplate(JobTypeBatch)
		}
	})
}
