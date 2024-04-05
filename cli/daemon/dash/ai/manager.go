package ai

import (
	"context"

	"github.com/hasura/go-graphql-client"

	"encr.dev/cli/daemon/apps"
	"encr.dev/pkg/fns"
	"encr.dev/pkg/paths"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

type Manager struct {
	client *LazySubClient
}

func NewAIManager(client *graphql.SubscriptionClient) *Manager {
	return &Manager{client: newLazyClient(client)}
}

func (m *Manager) DefineEndpoints(ctx context.Context, appSlug string, sessionID AISessionID, prompt string, md *meta.Data, proposed []Service, notifier AINotifier) (string, error) {
	return query[struct {
		StreamUpdate *StreamUpdate `graphql:"result: defineEndpoints(appSlug: $appSlug, sessionID: $sessionID, prompt: $prompt, current: $current, proposedDesign: $proposedDesign)"`
	}](ctx, m.client, map[string]interface{}{
		"appSlug":        appSlug,
		"prompt":         prompt,
		"current":        parseServicesFromMetadata(md),
		"proposedDesign": fns.Map(proposed, Service.GraphQL),
		"sessionID":      sessionID,
	}, createUpdateHandler(proposed, notifier))
}

func (m *Manager) ProposeSystemDesign(ctx context.Context, appSlug, prompt string, md *meta.Data, notifier AINotifier) (string, error) {
	return query[struct {
		StreamUpdate *StreamUpdate `graphql:"result: proposeSystemDesign(appSlug: $appSlug, prompt: $prompt, current: $current)"`
	}](ctx, m.client, map[string]interface{}{
		"appSlug": appSlug,
		"prompt":  prompt,
		"current": parseServicesFromMetadata(md),
	}, createUpdateHandler(nil, notifier))
}

func (m *Manager) ModifySystemDesign(ctx context.Context, appSlug string, sessionID AISessionID, originalPrompt string, proposed []Service, newPrompt string, md *meta.Data, notifier AINotifier) (string, error) {
	return query[struct {
		StreamUpdate *StreamUpdate `graphql:"result: modifySystemDesign(appSlug: $appSlug, sessionID: $sessionID, originalPrompt: $originalPrompt, proposedDesign: $proposedDesign, newPrompt: $newPrompt, current: $current)"`
	}](ctx, m.client, map[string]interface{}{
		"appSlug":        appSlug,
		"originalPrompt": originalPrompt,
		"proposedDesign": fns.Map(proposed, Service.GraphQL),
		"current":        parseServicesFromMetadata(md),
		"newPrompt":      newPrompt,
		"sessionID":      sessionID,
	}, createUpdateHandler(proposed, notifier))
}

func ParseCode(ctx context.Context, services []Service, app *apps.Instance) (*SyncResult, error) {
	return parseCode(ctx, app, services)
}

func UpdateCode(ctx context.Context, services []Service, app *apps.Instance, overwrite bool) (*SyncResult, error) {
	return updateCode(ctx, services, app, overwrite)
}

type WriteFilesResponse struct {
	FilesPaths []paths.RelSlash `json:"paths"`
}

func WriteFiles(ctx context.Context, services []Service, app *apps.Instance) (*WriteFilesResponse, error) {
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

func PreviewFiles(ctx context.Context, services []Service, app *apps.Instance) (*PreviewFilesResponse, error) {
	files, err := generateSrcFiles(services, app)
	return &PreviewFilesResponse{Files: fns.TransformMapToSlice(files, func(k paths.RelSlash, v string) PreviewFile {
		return PreviewFile{Path: k, Content: v}
	})}, err
}
