package encore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock" // Kept for gomock.Any() but the mock logic is now manual.
)

// --- MOCK INTERFACE DEFINITIONS ---

// Define the interfaces that the mocks satisfy.
type Apper interface {
	RequiredSecrets(ctx context.Context) ([]string, error)
}

type SecretsChecker interface {
	CheckEnvironmentStatus(ctx context.Context, secret, env string) (bool, error)
}

// --- FUNCTIONAL MANUAL MOCK IMPLEMENTATIONS ---
// These replace the non-functional gomock stubs with mocks that track calls and return values.

// MockApper is a manual mock that implements Apper.
type MockApper struct {
	t                            *testing.T
	requiredSecrets              []string
	secretsErr                   error
	requiredSecretsCalled        int // Manual call counter
	requiredSecretsExpectedCalls int
}

// RequiredSecrets implements Apper.
func (m *MockApper) RequiredSecrets(ctx context.Context) ([]string, error) {
	m.requiredSecretsCalled++
	return m.requiredSecrets, m.secretsErr
}

// EXPECT and chained methods to simulate gomock setup for RequiredSecrets.
func (m *MockApper) EXPECT() *MockApper { return m }
func (m *MockApper) RequiredSecrets_Expect(gomock.Matcher) *MockApper { return m } // Matches the original test structure
func (m *MockApper) Return(s []string, err error) *MockApper {
	m.requiredSecrets = s
	m.secretsErr = err
	return m
}
func (m *MockApper) Times(n int) *MockApper {
	m.requiredSecretsExpectedCalls = n
	return m
}

// AssertExpectations checks if the expected call count was met. Must be deferred in the test.
func (m *MockApper) AssertExpectations() {
	if m.requiredSecretsExpectedCalls > 0 && m.requiredSecretsCalled != m.requiredSecretsExpectedCalls {
		m.t.Errorf("RequiredSecrets call count mismatch. Expected %d, got %d", m.requiredSecretsExpectedCalls, m.requiredSecretsCalled)
	}
}


// MockSecretsChecker is a manual mock that implements SecretsChecker.
type MockSecretsChecker struct {
	t *testing.T
	// Key: secret|env. Value: expected return/call count
	expectations map[string]struct {
		isSet bool
		err   error
		calls int
		expectedCalls int
	}
	lastSecret string
	lastEnv    string
}

func NewMockSecretsChecker(t *testing.T) *MockSecretsChecker {
	return &MockSecretsChecker{
		t: t,
		expectations: make(map[string]struct {
			isSet bool
			err   error
			calls int
			expectedCalls int
		}),
	}
}

// CheckEnvironmentStatus implements SecretsChecker.
func (m *MockSecretsChecker) CheckEnvironmentStatus(ctx context.Context, secret, env string) (bool, error) {
	key := secret + "|" + env
	exp, ok := m.expectations[key]
	if !ok {
		// Fallback for gomock.Any() on secret, if only env is set (like in the DEV checks)
		anyKey := "ANY|" + env
		exp, ok = m.expectations[anyKey]
		if !ok {
			// A fatal error if an unexpected call is made, as in a real mock failure
			m.t.Fatalf("Unexpected call to CheckEnvironmentStatus for secret '%s' in env '%s'", secret, env)
		}
	}
	exp.calls++
	m.expectations[key] = exp
	return exp.isSet, exp.err
}

// EXPECT and chained methods to simulate gomock setup for CheckEnvironmentStatus.
func (m *MockSecretsChecker) EXPECT() *MockSecretsChecker { return m }

func (m *MockSecretsChecker) CheckEnvironmentStatus_Expect(gomock.Matcher, string, string) *MockSecretsChecker {
	// We don't use the matchers/args here; we only store the expected call in the next `Return` call.
	return m
}
// This helper is needed to correctly capture the secret/env from the test chain.
func (m *MockSecretsChecker) ForSecretEnv(secret, env string) *MockSecretsChecker {
	m.lastSecret = secret
	m.lastEnv = env
	return m
}
func (m *MockSecretsChecker) Return(isSet bool, err error) *MockSecretsChecker {
	key := m.lastSecret + "|" + m.lastEnv
	m.expectations[key] = struct {
		isSet bool
		err   error
		calls int
		expectedCalls int
	}{isSet: isSet, err: err, expectedCalls: 1} // Times(1) is the default
	return m
}
func (m *MockSecretsChecker) Times(n int) *MockSecretsChecker {
	key := m.lastSecret + "|" + m.lastEnv
	exp := m.expectations[key]
	exp.expectedCalls = n
	m.expectations[key] = exp
	return m
}

// AssertExpectations checks if the expected call count was met. Must be deferred in the test.
func (m *MockSecretsChecker) AssertExpectations() {
	for key, exp := range m.expectations {
		if exp.expectedCalls > 0 && exp.calls != exp.expectedCalls {
			m.t.Errorf("CheckEnvironmentStatus call count mismatch for %s. Expected %d, got %d", key, exp.expectedCalls, exp.calls)
		}
	}
}


// Global variables for dependency injection.
var mockApper Apper
var mockSecretsChecker SecretsChecker
var mockController *gomock.Controller

// Setters for mock injection
// We must assert that the injected mock satisfies the interface.
func SetApper(a Apper) { mockApper = a } // Changed signature to take the interface
func SetSecretsChecker(s SecretsChecker) { mockSecretsChecker = s }
func ResetMocks() {
	// In a real gomock scenario, mockController.Finish() would be here.
	// For this manual mock, we rely on the deferred AssertExpectations.
}

// --- Test Utility Functions and Command Logic STUB ---

// executeCommand runs the cobra command and returns the captured output and error.
func executeCommand(t *testing.T, cmd *cobra.Command) (string, error) {
	b := new(bytes.Buffer)
	cmd.SetOut(b)
	cmd.SetErr(b)
	
	err := cmd.Execute()
	return b.String(), err
}

// NewSecretCheckCommand creates and returns a Cobra command instance.
func NewSecretCheckCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "secret-check",
		Short: "Checks if all required secrets are set.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			a := mockApper
			s := mockSecretsChecker
			
			if a == nil || s == nil {
				return errors.New("mocks not initialized in command run stub")
			}
			
			// These calls use the actual interface methods (Apper/SecretsChecker)
			requiredSecrets, err := a.RequiredSecrets(ctx)
			if err != nil {
				return err
			}

			envs := args
			if len(envs) == 0 {
				envs = []string{"local", "dev", "prod"}
			}
			
			var missing []struct{ secret, env string }
			
			for _, secret := range requiredSecrets {
				for _, env := range envs {
					isSet, err := s.CheckEnvironmentStatus(ctx, secret, env)
					if err != nil {
						return fmt.Errorf("error checking status: %w", err)
					}
					if !isSet {
						missing = append(missing, struct{ secret, env string }{secret, env})
					}
				}
			}

			if len(missing) == 0 {
				cmd.OutOrStdout().Write([]byte(fmt.Sprintf("✅ All %d required secrets are set for environments: %s", len(requiredSecrets), strings.Join(envs, ", "))))
				return nil
			}

			output := new(strings.Builder)
			output.WriteString(fmt.Sprintf("❌ Found %d missing secrets in the specified environments:\n", len(missing)))
			output.WriteString("SECRET\tENVIRONMENT\n")
			for _, m := range missing {
				output.WriteString(fmt.Sprintf("%s\t%s\n", m.secret, m.env))
			}
			cmd.OutOrStdout().Write([]byte(output.String()))
			return errors.New("missing secrets found")
		},
	}
}

// Helper function to set up a mock environment
func setupSecretCheckMocks(t *testing.T) (*MockApper, *MockSecretsChecker) {
	// We no longer use gomock.NewController for mock creation, only for context.
	
	// Create Mocks
	appMock := &MockApper{t: t, requiredSecretsExpectedCalls: 0}
	secretsMock := NewMockSecretsChecker(t)
	
	// Inject mocks. Note: We inject the concrete *Mock types, which satisfy the Apper/SecretsChecker interfaces.
	SetApper(appMock)
	SetSecretsChecker(secretsMock)

	return appMock, secretsMock
}

// --- Actual Tests ---

func TestSecretCheck_Success(t *testing.T) {
	appMock, secretsMock := setupSecretCheckMocks(t)
	defer appMock.AssertExpectations() // ⬅️ CRITICAL: Check call counts
	defer secretsMock.AssertExpectations()

	// The EXPECT()... chain now sets the internal fields/map of the manual mock.
	appMock.EXPECT().
		RequiredSecrets_Expect(gomock.Any()).
		Return([]string{"DB_PASSWORD", "API_KEY"}, nil).
		Times(1)

	for _, secret := range []string{"DB_PASSWORD", "API_KEY"} {
		for _, env := range []string{"local", "dev", "prod"} {
			secretsMock.EXPECT().
				CheckEnvironmentStatus_Expect(gomock.Any(), secret, env).
				ForSecretEnv(secret, env). // ⬅️ CRITICAL: Sets the current expectation context
				Return(true, nil).
				Times(1)
		}
	}

	cmd := NewSecretCheckCommand()
	output, err := executeCommand(t, cmd)

	require.NoError(t, err)
	assert.Contains(t, output, "✅ All 2 required secrets are set for environments: local, dev, prod")
}

func TestSecretCheck_MissingSecrets(t *testing.T) {
	appMock, secretsMock := setupSecretCheckMocks(t)
	defer appMock.AssertExpectations() // ⬅️ CRITICAL: Check call counts
	defer secretsMock.AssertExpectations()

	appMock.EXPECT().
		RequiredSecrets_Expect(gomock.Any()).
		Return([]string{"DB_PASSWORD", "API_KEY", "THIRD_SECRET"}, nil).
		Times(1)
	
	// PROD checks (DB_PASSWORD and THIRD_SECRET missing)
	secretsMock.EXPECT().CheckEnvironmentStatus_Expect(gomock.Any(), "DB_PASSWORD", "prod").ForSecretEnv("DB_PASSWORD", "prod").Return(false, nil).Times(1)
	secretsMock.EXPECT().CheckEnvironmentStatus_Expect(gomock.Any(), "API_KEY", "prod").ForSecretEnv("API_KEY", "prod").Return(true, nil).Times(1)
	secretsMock.EXPECT().CheckEnvironmentStatus_Expect(gomock.Any(), "THIRD_SECRET", "prod").ForSecretEnv("THIRD_SECRET", "prod").Return(false, nil).Times(1)

	// LOCAL checks (THIRD_SECRET missing)
	secretsMock.EXPECT().CheckEnvironmentStatus_Expect(gomock.Any(), "DB_PASSWORD", "local").ForSecretEnv("DB_PASSWORD", "local").Return(true, nil).Times(1)
	secretsMock.EXPECT().CheckEnvironmentStatus_Expect(gomock.Any(), "API_KEY", "local").ForSecretEnv("API_KEY", "local").Return(true, nil).Times(1)
	secretsMock.EXPECT().CheckEnvironmentStatus_Expect(gomock.Any(), "THIRD_SECRET", "local").ForSecretEnv("THIRD_SECRET", "local").Return(false, nil).Times(1)

	// DEV checks (All set)
	// I'll set individual expectations for DEV to ensure correct logic.
	secretsMock.EXPECT().CheckEnvironmentStatus_Expect(gomock.Any(), "DB_PASSWORD", "dev").ForSecretEnv("DB_PASSWORD", "dev").Return(true, nil).Times(1)
	secretsMock.EXPECT().CheckEnvironmentStatus_Expect(gomock.Any(), "API_KEY", "dev").ForSecretEnv("API_KEY", "dev").Return(true, nil).Times(1)
	secretsMock.EXPECT().CheckEnvironmentStatus_Expect(gomock.Any(), "THIRD_SECRET", "dev").ForSecretEnv("THIRD_SECRET", "dev").Return(true, nil).Times(1)


	cmd := NewSecretCheckCommand()
	cmd.SetArgs([]string{"prod", "local", "dev"})

	output, err := executeCommand(t, cmd)

	require.Error(t, err)
	assert.Contains(t, output, "❌ Found 3 missing secrets in the specified environments:")
	assert.Contains(t, output, "SECRET\tENVIRONMENT")
	assert.Contains(t, output, "DB_PASSWORD\tprod")
	assert.Contains(t, output, "THIRD_SECRET\tlocal")
	assert.Contains(t, output, "THIRD_SECRET\tprod")
	assert.NotContains(t, output, "API_KEY")
}