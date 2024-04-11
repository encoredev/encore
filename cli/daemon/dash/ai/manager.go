package ai

import (
	"context"

	"github.com/hasura/go-graphql-client"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// Manager exposes the ai functionality to the local dashboard
type Manager struct {
	aiClient *LazySubClient
}

func NewAIManager(client *graphql.SubscriptionClient) *Manager {
	return &Manager{aiClient: newLazySubClient(client)}
}

func (m *Manager) DefineEndpoints(ctx context.Context, appSlug string, sessionID AISessionID, prompt string, md *meta.Data, proposed []Service, notifier AINotifier) (string, error) {
	svcs := fns.Map(proposed, Service.GetName)
	return startAITask[struct {
		Message *AIStreamMessage `graphql:"result: defineEndpoints(appSlug: $appSlug, sessionID: $sessionID, prompt: $prompt, current: $current, proposedDesign: $proposedDesign, existingTypes: $existingTypes)"`
	}](ctx, m.aiClient, map[string]interface{}{
		"appSlug":        appSlug,
		"prompt":         prompt,
		"current":        parseServicesFromMetadata(md, svcs...),
		"proposedDesign": fns.Map(proposed, Service.GraphQL),
		"sessionID":      sessionID,
		"existingTypes":  renderTypesFromMetadata(md, svcs...),
	}, newEndpointAssemblerHandler(proposed, notifier, true))
}

func (m *Manager) ProposeSystemDesign(ctx context.Context, appSlug, prompt string, md *meta.Data, notifier AINotifier) (string, error) {
	return startAITask[struct {
		Message *AIStreamMessage `graphql:"result: proposeSystemDesign(appSlug: $appSlug, prompt: $prompt, current: $current)"`
	}](ctx, m.aiClient, map[string]interface{}{
		"appSlug": appSlug,
		"prompt":  prompt,
		"current": parseServicesFromMetadata(md),
	}, newEndpointAssemblerHandler(nil, notifier, false))
}

func (m *Manager) ModifySystemDesign(ctx context.Context, appSlug string, sessionID AISessionID, originalPrompt string, proposed []Service, newPrompt string, md *meta.Data, notifier AINotifier) (string, error) {
	return startAITask[struct {
		Message *AIStreamMessage `graphql:"result: modifySystemDesign(appSlug: $appSlug, sessionID: $sessionID, originalPrompt: $originalPrompt, proposedDesign: $proposedDesign, newPrompt: $newPrompt, current: $current)"`
	}](ctx, m.aiClient, map[string]interface{}{
		"appSlug":        appSlug,
		"originalPrompt": originalPrompt,
		"proposedDesign": fns.Map(proposed, Service.GraphQL),
		"current":        parseServicesFromMetadata(md),
		"newPrompt":      newPrompt,
		"sessionID":      sessionID,
	}, newEndpointAssemblerHandler(proposed, notifier, false))
}

func (m *Manager) ParseCode(ctx context.Context, services []Service, app *apps.Instance) (*SyncResult, error) {
	return parseCode(ctx, app, services)
}

func (m *Manager) UpdateCode(ctx context.Context, services []Service, app *apps.Instance, overwrite bool) (*SyncResult, error) {
	return updateCode(ctx, services, app, overwrite)
}

type WriteFilesResponse struct {
	FilesPaths []paths.RelSlash `json:"paths"`
}

func (m *Manager) WriteFiles(ctx context.Context, services []Service, app *apps.Instance) (*WriteFilesResponse, error) {
	files, err := writeFiles(services, app)
	return &WriteFilesResponse{FilesPaths: files}, err
}

type PreviewFile struct {
	Path    paths.RelSlash `json:"path"`
	Content string         `json:"content"`
}

type PreviewFilesResponse struct {
	Files []PreviewFile `json:"files"`
}

func (m *Manager) PreviewFiles(ctx context.Context, services []Service, app *apps.Instance) (*PreviewFilesResponse, error) {
	files, err := generateSrcFiles(services, app)
	return &PreviewFilesResponse{Files: fns.TransformMapToSlice(files, func(k paths.RelSlash, v string) PreviewFile {
		return PreviewFile{Path: k, Content: v}
	})}, err
}

func (m *Manager) Unsubscribe(id string) error {
	return m.aiClient.Unsubscribe(id)
}
