package export

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"encore.dev/appruntime/exported/experiments"
	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/appfile"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/builder/builderimpl"
	daemonpb "encr.dev/proto/encore/daemon"
)

// MockBuilder is a mock implementation of the builder.Impl interface
type MockBuilder struct {
	mock.Mock
}

func (m *MockBuilder) Parse(ctx context.Context, params builder.ParseParams) (*builder.ParseResult, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*builder.ParseResult), args.Error(1)
}

func (m *MockBuilder) Compile(ctx context.Context, params builder.CompileParams) (*builder.CompileResult, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*builder.CompileResult), args.Error(1)
}

func (m *MockBuilder) ServiceConfigs(ctx context.Context, params builder.ServiceConfigsParams) (*builder.ServiceConfigsResult, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*builder.ServiceConfigsResult), args.Error(1)
}

func (m *MockBuilder) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockBuilder) GenUserFacing(ctx context.Context, params builder.GenUserFacingParams) error {
	args := m.Called(ctx, params)
	return args.Error(1)
}

func (m *MockBuilder) NeedsMeta() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockBuilder) RunTests(ctx context.Context, params builder.RunTestsParams) error {
	args := m.Called(ctx, params)
	return args.Error(0)
}

func (m *MockBuilder) TestSpec(ctx context.Context, params builder.TestSpecParams) (*builder.TestSpecResult, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*builder.TestSpecResult), args.Error(1)
}

func (m *MockBuilder) UseNewRuntimeConfig() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockResolveFunc is a function type for mocking builderimpl.Resolve
type MockResolveFunc func(appfile.Lang, *experiments.Set) builder.Impl

// resolveBuilder is a variable that holds the function to resolve the builder.
// We'll modify this variable in our tests instead of directly changing builderimpl.Resolve.
var resolveBuilder = builderimpl.Resolve

// SetMockResolve replaces the builderimpl.Resolve function with a mock
func SetMockResolve(mock MockResolveFunc) func() {
	original := resolveBuilder
	resolveBuilder = mock
	return func() {
		resolveBuilder = original
	}
}

func TestDocker(t *testing.T) {
	// Setup
	ctx := context.Background()
	app := &apps.Instance{}
	log := zerolog.New(zerolog.NewTestWriter(t))

	// Test cases
	tests := []struct {
		name           string
		req            *daemonpb.ExportRequest
		setupMocks     func(*MockBuilder)
		expectedResult bool
		expectedError  error
	}{
		{
			name: "Successful export",
			req: &daemonpb.ExportRequest{
				Format: &daemonpb.ExportRequest_Docker{
					Docker: &daemonpb.DockerExportParams{
						BaseImageTag:       "alpine:latest",
						LocalDaemonTag:     "myapp:latest",
						PushDestinationTag: "registry.example.com/myapp:latest",
					},
				},
				Goos:   "linux",
				Goarch: "amd64",
			},
			setupMocks: func(mb *MockBuilder) {
				mb.On("Parse", mock.Anything, mock.Anything).Return(builder.ParseResult{}, nil)
				mb.On("ServiceConfigs", mock.Anything, mock.Anything).Return(builder.ServiceConfigsResult{}, nil)
				mb.On("Compile", mock.Anything, mock.Anything).Return(&builder.CompileResult{}, nil)
				mb.On("Close").Return(nil)
				// You might need to mock GenUserFacing as well, depending on your logic
				mb.On("GenUserFacing", mock.Anything, mock.Anything).Return(builder.GenUserFacingParams{}, nil)
			},
			expectedResult: true,
			expectedError:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mb := new(MockBuilder)
			tt.setupMocks(mb)

			// Set up the mock resolve function
			restore := SetMockResolve(func(lang appfile.Lang, expSet *experiments.Set) builder.Impl {
				return mb
			})
			defer restore()

			// Run the test
			result, err := Docker(ctx, app, tt.req, log)

			// Assertions
			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			// Verify mocks
			mb.AssertExpectations(t)
		})
	}
}