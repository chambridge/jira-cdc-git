package jobs

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// JobIDGenerator generates unique job IDs
type JobIDGenerator interface {
	Generate(prefix string) string
	GenerateWithType(jobType JobType) string
	Validate(jobID string) error
}

// DefaultJobIDGenerator implements a UUID-based job ID generator
type DefaultJobIDGenerator struct{}

// NewJobIDGenerator creates a new job ID generator
func NewJobIDGenerator() JobIDGenerator {
	return &DefaultJobIDGenerator{}
}

// Generate creates a new job ID with optional prefix
func (g *DefaultJobIDGenerator) Generate(prefix string) string {
	timestamp := time.Now().Format("20060102-150405")
	randomSuffix := g.generateRandomString(8)

	if prefix != "" {
		return fmt.Sprintf("%s-%s-%s", prefix, timestamp, randomSuffix)
	}

	return fmt.Sprintf("job-%s-%s", timestamp, randomSuffix)
}

// GenerateWithType creates a job ID based on job type
func (g *DefaultJobIDGenerator) GenerateWithType(jobType JobType) string {
	return g.Generate(string(jobType))
}

// Validate checks if a job ID is valid
func (g *DefaultJobIDGenerator) Validate(jobID string) error {
	if jobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if len(jobID) < 5 {
		return fmt.Errorf("job ID too short: %s", jobID)
	}

	if len(jobID) > 63 {
		return fmt.Errorf("job ID too long (max 63 characters): %s", jobID)
	}

	// Check for invalid characters (Kubernetes naming conventions)
	if !isValidKubernetesName(jobID) {
		return fmt.Errorf("job ID contains invalid characters: %s", jobID)
	}

	return nil
}

// generateRandomString creates a random alphanumeric string
func (g *DefaultJobIDGenerator) generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)

	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based generation
		return fmt.Sprintf("%x", time.Now().UnixNano())[:length]
	}

	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}

	return string(b)
}

// isValidKubernetesName checks if a string is a valid Kubernetes resource name
func isValidKubernetesName(name string) bool {
	if name == "" {
		return false
	}

	// Must start and end with alphanumeric character
	if !isAlphaNumeric(name[0]) || !isAlphaNumeric(name[len(name)-1]) {
		return false
	}

	// Check all characters
	for _, char := range name {
		if !isAlphaNumeric(byte(char)) && char != '-' {
			return false
		}
	}

	// Cannot contain consecutive hyphens
	if strings.Contains(name, "--") {
		return false
	}

	return true
}

// isAlphaNumeric checks if a byte is alphanumeric
func isAlphaNumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// JobIDInfo extracts information from a job ID
type JobIDInfo struct {
	OriginalID string
	Prefix     string
	Timestamp  string
	Suffix     string
	JobType    JobType
}

// ParseJobID extracts components from a job ID
func ParseJobID(jobID string) (*JobIDInfo, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job ID cannot be empty")
	}

	parts := strings.Split(jobID, "-")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid job ID format: %s", jobID)
	}

	info := &JobIDInfo{
		OriginalID: jobID,
		Prefix:     parts[0],
		Suffix:     parts[len(parts)-1],
	}

	// Try to extract timestamp (second-to-last part for standard format)
	if len(parts) >= 3 {
		timestampPart := parts[len(parts)-2]
		if len(timestampPart) == 15 { // Format: 20060102-150405
			info.Timestamp = timestampPart
		}
	}

	// Determine job type from prefix
	switch info.Prefix {
	case "single":
		info.JobType = JobTypeSingle
	case "batch":
		info.JobType = JobTypeBatch
	case "jql":
		info.JobType = JobTypeJQL
	default:
		info.JobType = JobType(info.Prefix)
	}

	return info, nil
}

// FormatJobName generates a Kubernetes-compatible job name from a job ID
func FormatJobName(jobID string) string {
	// Ensure the name is lowercase and replace any invalid characters
	name := strings.ToLower(jobID)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, ".", "-")

	// Ensure it starts with alphanumeric
	if len(name) > 0 && !isAlphaNumeric(name[0]) {
		name = "job-" + name
	}

	// Truncate if too long
	if len(name) > 63 {
		name = name[:63]
	}

	// Ensure it ends with alphanumeric
	for len(name) > 0 && !isAlphaNumeric(name[len(name)-1]) {
		name = name[:len(name)-1]
	}

	return name
}
