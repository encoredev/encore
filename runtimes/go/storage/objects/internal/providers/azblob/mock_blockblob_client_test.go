//go:build !encore_no_azure

package azblob

import (
	"context"
	"io"
	"reflect"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/golang/mock/gomock"
)

// MockblockBlobClient is a hand-written mock of the blockBlobClient interface,
// following the same pattern as the generated S3 mock.
type MockblockBlobClient struct {
	ctrl     *gomock.Controller
	recorder *MockblockBlobClientMockRecorder
}

// MockblockBlobClientMockRecorder records expected calls.
type MockblockBlobClientMockRecorder struct {
	mock *MockblockBlobClient
}

// NewMockblockBlobClient creates a new mock instance.
func NewMockblockBlobClient(ctrl *gomock.Controller) *MockblockBlobClient {
	mock := &MockblockBlobClient{ctrl: ctrl}
	mock.recorder = &MockblockBlobClientMockRecorder{mock}
	return mock
}

// EXPECT returns the recorder for expected calls.
func (m *MockblockBlobClient) EXPECT() *MockblockBlobClientMockRecorder {
	return m.recorder
}

// Upload mocks blockBlobClient.Upload.
func (m *MockblockBlobClient) Upload(ctx context.Context, body io.ReadSeekCloser, options *blockblob.UploadOptions) (blockblob.UploadResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Upload", ctx, body, options)
	ret0, _ := ret[0].(blockblob.UploadResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Upload records an expected Upload call.
func (mr *MockblockBlobClientMockRecorder) Upload(ctx, body, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Upload",
		reflect.TypeOf((*MockblockBlobClient)(nil).Upload), ctx, body, options)
}

// StageBlock mocks blockBlobClient.StageBlock.
func (m *MockblockBlobClient) StageBlock(ctx context.Context, base64BlockID string, body io.ReadSeekCloser, options *blockblob.StageBlockOptions) (blockblob.StageBlockResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StageBlock", ctx, base64BlockID, body, options)
	ret0, _ := ret[0].(blockblob.StageBlockResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StageBlock records an expected StageBlock call.
func (mr *MockblockBlobClientMockRecorder) StageBlock(ctx, base64BlockID, body, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StageBlock",
		reflect.TypeOf((*MockblockBlobClient)(nil).StageBlock), ctx, base64BlockID, body, options)
}

// CommitBlockList mocks blockBlobClient.CommitBlockList.
func (m *MockblockBlobClient) CommitBlockList(ctx context.Context, base64BlockIDs []string, options *blockblob.CommitBlockListOptions) (blockblob.CommitBlockListResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitBlockList", ctx, base64BlockIDs, options)
	ret0, _ := ret[0].(blockblob.CommitBlockListResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CommitBlockList records an expected CommitBlockList call.
func (mr *MockblockBlobClientMockRecorder) CommitBlockList(ctx, base64BlockIDs, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitBlockList",
		reflect.TypeOf((*MockblockBlobClient)(nil).CommitBlockList), ctx, base64BlockIDs, options)
}
