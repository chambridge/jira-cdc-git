package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// DotEnvLoader implements Provider with .env file support
type DotEnvLoader struct {
	*Loader
	envFiles []string
}

// NewDotEnvLoader creates a new configuration loader with .env file support
func NewDotEnvLoader(envFiles ...string) Provider {
	// Default to .env file in current directory if none specified
	if len(envFiles) == 0 {
		envFiles = []string{".env"}
	}

	return &DotEnvLoader{
		Loader:   &Loader{envLoader: &OSEnvLoader{}},
		envFiles: envFiles,
	}
}

// NewDotEnvLoaderWithEnv creates a loader with custom environment loader and .env support
func NewDotEnvLoaderWithEnv(envLoader EnvLoader, envFiles ...string) Provider {
	if len(envFiles) == 0 {
		envFiles = []string{".env"}
	}

	return &DotEnvLoader{
		Loader:   &Loader{envLoader: envLoader},
		envFiles: envFiles,
	}
}

// Load loads configuration from .env file(s) and environment variables
func (d *DotEnvLoader) Load() (*Config, error) {
	// Load all .env files at once to ensure proper override behavior
	existingFiles := []string{}
	for _, envFile := range d.envFiles {
		if _, err := os.Stat(envFile); err == nil {
			existingFiles = append(existingFiles, envFile)
		}
	}

	// Load all existing files at once - godotenv.Overload with multiple files
	// ensures that .env files override any existing environment variables
	if len(existingFiles) > 0 {
		if err := godotenv.Overload(existingFiles...); err != nil {
			// Get absolute path for error reporting
			absPath := existingFiles[0]
			if len(existingFiles) > 1 {
				absPath = "multiple files: " + strings.Join(existingFiles, ", ")
			}
			return nil, NewEnvFileError(absPath, err)
		}
	}

	// Load from environment variables (including those loaded from .env)
	return d.LoadFromEnv()
}

// EnvFileError represents an error loading a .env file
type EnvFileError struct {
	FilePath string
	Err      error
}

func NewEnvFileError(filePath string, err error) *EnvFileError {
	return &EnvFileError{
		FilePath: filePath,
		Err:      err,
	}
}

func (e *EnvFileError) Error() string {
	return "failed to load .env file '" + e.FilePath + "': " + e.Err.Error()
}

func (e *EnvFileError) Unwrap() error {
	return e.Err
}

// LoadWithEnvFile is a convenience function to load configuration with .env file support
func LoadWithEnvFile(envFiles ...string) (*Config, error) {
	loader := NewDotEnvLoader(envFiles...)
	return loader.Load()
}

// LoadFromCurrentDir loads configuration from .env file in current directory
func LoadFromCurrentDir() (*Config, error) {
	return LoadWithEnvFile(".env")
}
