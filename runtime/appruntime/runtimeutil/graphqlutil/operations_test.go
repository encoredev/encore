package graphqlutil

import (
	"reflect"
	"testing"
)

func TestGetOperations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		httpMethod  string
		contentType string
		req         string
		want        []*Op
	}{
		{
			name:       "named query via GET",
			httpMethod: "GET",
			req: `query getDogName {
  dog {
    name
    color
  }
}`,
			want: []*Op{{Query, "getDogName"}},
		},
		{
			name:        "named query via POST with application/graphql",
			httpMethod:  "POST",
			contentType: "application/graphql",
			req:         `query getDogName { dog { name }}`,
			want:        []*Op{{Query, "getDogName"}},
		},
		{
			name:       "named mutation",
			httpMethod: "GET",
			req: `  mutation 
					GetUserData 
					{
						user { name }
			}`,
			want: []*Op{{Mutation, "GetUserData"}},
		},
		{
			name:       "named subscription",
			httpMethod: "GET",
			req: `subscription NewMessages {
  newMessage(roomId: 123) {
    sender
    text
  }
}`,
			want: []*Op{{Subscription, "NewMessages"}},
		},
		{
			name:       "json encoded query",
			httpMethod: "POST",
			req: `{	
	"query": "query getDogName { dog { name color } }",
	"operationName": "getDogName"
}`,
			want: []*Op{{Query, "getDogName"}},
		},
		{
			name:       "json encoded mutation",
			httpMethod: "POST",
			req: `{
	"query": "mutation GetUserData { user { name } }",
	"operationName": "GetUserData"
}`,
			want: []*Op{{Mutation, "GetUserData"}},
		},
		{
			name:       "multiple queries query",
			httpMethod: "GET",
			req: `query getDogName {
  dog {
    name
    color
  }
}

query getCatName {
  cat {
    name
    color
  }
}


mutation deleteOwner {
  deleteOwner()
}`,
			want: []*Op{{Query, "getDogName"}, {Query, "getCatName"}, {Mutation, "deleteOwner"}},
		},
		{
			name: "some other binary",
			req:  string([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}),
			want: nil,
		},
		{
			name:        "query sent as POST without Content-Type",
			httpMethod:  "POST",
			contentType: "application/json", // not application/graphql
			req:         `query getDogName { dog { name }}`,
			want:        nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := GetOperations(tt.httpMethod, tt.contentType, []byte(tt.req)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OperationNameFromQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}
