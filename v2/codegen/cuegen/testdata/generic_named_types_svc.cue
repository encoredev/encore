// Code generated by encore. DO NOT EDIT.
//
// The contents of this file are generated from the structs used in
// conjunction with Encore's `config.Load[T]()` function. This file
// automatically be regenerated if the data types within the struct
// are changed.
//
// For more information about this file, see:
// https://encore.dev/docs/develop/config
package svc

// #Meta contains metadata about the running Encore application.
// The values in this struct will be injected by Encore upon deployment and can be
// referenced from other config values for example when configuring a callback URL:
//    CallbackURL: "\(#Meta.APIBaseURL)/webhooks.Handle`"
#Meta: {
	APIBaseURL: string @tag(APIBaseURL) // The base URL which can be used to call the API of this running application.
	Environment: {
		Name:  string                                              @tag(EnvName)   // The name of this environment
		Type:  "production" | "development" | "ephemeral" | "test" @tag(EnvType)   // The type of environment that the application is running in
		Cloud: "aws" | "azure" | "gcp" | "encore" | "local"        @tag(CloudType) // The cloud provider that the application is running in
	}
}

// #Config is the top level configuration for the application and is generated
// from the Go types you've passed into `config.Load[T]()`. Encore uses a definition
// of this struct which is closed, such that the CUE tooling can any typos of field names.
// this definition is then immediately inlined, so any fields within it are expected
// as fields at the package level.
#Config: {
	HTTP:    #DisablableOption_uint16 // The options for the HTTP server
	Another: #DisablableOption_uint64
	TCP:     #DisablableOption_uint16 // The options for the TCP server
	GRPC:    #DisablableOption_uint64 // The options for the GRPC server
	List1:   #List_string             // A list of strings
	List2:   #List_string
	List3: [...int]
	Map1: #Map_string_string
	Map2: [int]: string
	Map3: #Map_string_string
}
#Config

// Generic option which can be disbaled
#DisablableOption_uint16: {
	Option:   uint16
	Disabled: bool // True if this is disabled
}

// A nice generic map
#Map_string_string: {
	[string]: string
}

#List_string: [...string]

// Generic option which can be disbaled
#DisablableOption_uint64: {
	Option:   uint64
	Disabled: bool // True if this is disabled
}
