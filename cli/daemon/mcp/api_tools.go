package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	"encr.dev/cli/daemon/run"
	"encr.dev/pkg/builder"
	metav1 "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

func (m *Manager) registerAPITools() {

	// Add tool for calling an API endpoint
	m.server.AddTool(mcp.NewTool("call_endpoint",
		mcp.WithDescription("Make HTTP requests to any API endpoint in the currently open Encore. Always use this tool to make API calls and do not use curl. This tool will automatically start the application if it's not already running. This tool allows testing and interacting with the application's API endpoints, including authentication and custom payloads."),
		mcp.WithString("service", mcp.Description("The name of the service containing the endpoint to call. This must match a service defined in the currently open Encore.")),
		mcp.WithString("endpoint", mcp.Description("The name of the endpoint to call within the specified service. This must match an endpoint defined in the service.")),
		mcp.WithString("method", mcp.Description("The HTTP method to use for the request (GET, POST, PUT, DELETE, etc.). Must be a valid HTTP method.")),
		mcp.WithString("path", mcp.Description("The API request path, including any path parameters. This should match the endpoint's defined path pattern.")),
		mcp.WithString("payload", mcp.Description("JSON payload for the request containing all endpoint parameters. This includes path parameters, query parameters, headers, and request body as key-value pairs.")),
		mcp.WithString("auth_token", mcp.Description("Optional authentication token to include in the request. This is used for endpoints that require authentication.")),
		mcp.WithString("auth_payload", mcp.Description("Optional authentication payload in JSON format. This is used for custom authentication schemes.")),
		mcp.WithString("correlation_id", mcp.Description("Optional correlation ID to track the request through the system. Useful for debugging and tracing.")),
	), m.callEndpoint)

	// Add tool for getting all services and endpoints
	m.server.AddTool(mcp.NewTool("get_services",
		mcp.WithDescription("Retrieve comprehensive information about all services and their endpoints in the currently open Encore. This includes endpoint schemas, documentation, and service-level metadata."),
		mcp.WithArray("services",
			mcp.Items(map[string]any{
				"type":        "string",
				"description": "Optional list of specific service names to retrieve information for. If not provided, returns information for all services in the currently open Encore.",
			})),
		mcp.WithArray("endpoints",
			mcp.Items(map[string]any{
				"type":        "string",
				"description": "Optional list of specific endpoint names to filter by. If not provided, returns all endpoints for the specified services.",
			})),
		mcp.WithBoolean("include_schemas", mcp.Description("When true, includes detailed request and response schemas for each endpoint. This is useful for understanding the data structures used by the API.")),
		mcp.WithBoolean("include_service_details", mcp.Description("When true, includes additional service-level information such as middleware, dependencies, and configuration.")),
		mcp.WithBoolean("include_endpoints", mcp.Description("When true, includes endpoint information in the response. Set to false to get only service-level information.")),
	), m.getEndpoints)

	// Add tool for getting middleware metadata
	m.server.AddTool(mcp.NewTool("get_middleware",
		mcp.WithDescription("Retrieve detailed information about all middleware components in the currently open Encore, including their configuration, order of execution, and which services/endpoints they affect."),
	), m.getMiddleware)

	// Add tool for getting auth handler metadata
	m.server.AddTool(mcp.NewTool("get_auth_handlers",
		mcp.WithDescription("Retrieve information about all authentication handlers in the currently open Encore, including their configuration, supported authentication methods, and which services/endpoints they protect."),
	), m.getAuthHandlers)
}

func (m *Manager) callEndpoint(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	// Extract and validate required arguments
	serviceStr, ok := request.Params.Arguments["service"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid service argument")
	}

	endpointStr, ok := request.Params.Arguments["endpoint"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid endpoint argument")
	}

	methodStr, ok := request.Params.Arguments["method"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid method argument")
	}

	pathStr, ok := request.Params.Arguments["path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid path argument")
	}

	// Build API call parameters
	params := &run.ApiCallParams{
		AppID:         inst.PlatformOrLocalID(),
		Service:       serviceStr,
		Endpoint:      endpointStr,
		Path:          pathStr,
		Method:        methodStr,
		CorrelationID: "",
	}

	if !strings.HasPrefix(params.Path, "/") {
		params.Path = "/" + params.Path
	}

	// Add optional parameters
	if payload, ok := request.Params.Arguments["payload"].(string); ok && payload != "" {
		params.Payload = []byte(payload)
	}
	if authToken, ok := request.Params.Arguments["auth_token"].(string); ok && authToken != "" {
		params.AuthToken = authToken
	}
	if authPayload, ok := request.Params.Arguments["auth_payload"].(string); ok && authPayload != "" {
		params.AuthPayload = []byte(authPayload)
	}
	if correlationID, ok := request.Params.Arguments["correlation_id"].(string); ok && correlationID != "" {
		params.CorrelationID = correlationID
	}
	ns, err := m.ns.GetActive(ctx, inst)
	if err != nil {
		return nil, fmt.Errorf("failed to get active namespace: %w", err)
	}

	// Get the app's run instance
	appRun := m.run.FindRunByAppID(inst.PlatformOrLocalID())
	if appRun == nil {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("failed to create listener: %w", err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		appRun, err = m.run.Start(ctx, run.StartParams{
			App:        inst,
			NS:         ns,
			WorkingDir: "/",
			Watch:      true,
			Listener:   ln,
			ListenAddr: "127.0.0.1:" + fmt.Sprint(port),
			Environ:    os.Environ(),
			OpsTracker: nil,
			Browser:    run.BrowserModeNever,
			Debug:      builder.DebugModeDisabled,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to start app run: %w", err)
		}
	}

	started := false
	for !started {
		select {
		case <-appRun.Done():
			return nil, fmt.Errorf("app run failed to start")
		case <-time.After(100 * time.Millisecond):
			// Check if the app is ready by polling the health endpoint
			resp, err := http.Get("http://" + appRun.ListenAddr + "/__encore/healthz")
			if err != nil {
				continue
			}
			resp.Body.Close()
			started = resp.StatusCode == 200
		}
	}

	// Call the API
	result, err := run.CallAPI(ctx, appRun, params)

	if err != nil {
		return nil, fmt.Errorf("API call failed: %w", err)
	}

	// Serialize the response
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (m *Manager) getEndpoints(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Get the list of services to retrieve endpoints for
	var serviceNames []string
	if services, ok := request.Params.Arguments["services"].([]interface{}); ok {
		for _, svc := range services {
			if svcName, ok := svc.(string); ok && svcName != "" {
				serviceNames = append(serviceNames, svcName)
			}
		}
	}

	// If no services specified, get all services
	if len(serviceNames) == 0 {
		for _, svc := range md.Svcs {
			serviceNames = append(serviceNames, svc.Name)
		}
	}

	// Parse request parameters
	includeEndpoints := true
	if include, ok := request.Params.Arguments["include_endpoints"].(bool); ok {
		includeEndpoints = include
	}

	var endpointNames []string
	var endpointFilter map[string]bool
	var hasEndpointFilter bool

	// Only process endpoint filters if we're including endpoints
	if includeEndpoints {
		// Get the list of endpoint names to filter by
		if endpoints, ok := request.Params.Arguments["endpoints"].([]interface{}); ok {
			for _, ep := range endpoints {
				if epName, ok := ep.(string); ok && epName != "" {
					endpointNames = append(endpointNames, epName)
				}
			}
		}
		// Create a map for faster lookups when filtering endpoints
		endpointFilter = make(map[string]bool)
		for _, name := range endpointNames {
			endpointFilter[name] = true
		}
		hasEndpointFilter = len(endpointFilter) > 0
	}

	includeSchemas := false
	if include, ok := request.Params.Arguments["include_schemas"].(bool); ok {
		includeSchemas = include
	}

	includeServiceDetails := false
	if include, ok := request.Params.Arguments["include_service_details"].(bool); ok {
		includeServiceDetails = include
	}

	// Set up decl map for schema info if needed
	var declByID map[uint32]*schema.Decl
	if includeEndpoints && includeSchemas {
		declByID = map[uint32]*schema.Decl{}
		for _, decl := range md.Decls {
			declByID[decl.Id] = decl
		}
	}

	// Create a map to store services with their endpoints
	serviceMap := make(map[string]map[string]interface{})

	// Process each requested service
	for _, serviceName := range serviceNames {
		// Find the service in metadata
		var targetService *metav1.Service
		for _, svc := range md.Svcs {
			if svc.Name == serviceName {
				targetService = svc
				break
			}
		}

		if targetService == nil {
			// Skip services that don't exist instead of returning an error
			continue
		}

		// Initialize service data
		serviceData := map[string]interface{}{}

		// Add service details if requested
		if includeServiceDetails {
			serviceData["name"] = targetService.Name
			serviceData["rel_path"] = targetService.RelPath
			serviceData["has_config"] = targetService.HasConfig
			serviceData["databases"] = targetService.Databases
			serviceData["rpc_count"] = len(targetService.Rpcs)
		}

		// Process endpoints if requested
		if includeEndpoints {
			// Initialize an empty array for this service's endpoints
			endpoints := make([]map[string]interface{}, 0)

			// Process all RPCs for this service
			for _, rpc := range targetService.Rpcs {
				// Skip this endpoint if it's not in the filter list (when filter is provided)
				if hasEndpointFilter && !endpointFilter[rpc.Name] {
					continue
				}

				endpoint := map[string]interface{}{
					"name":         rpc.Name,
					"access_type":  rpc.AccessType.String(),
					"http_methods": rpc.HttpMethods,
				}

				// Add path if available
				if rpc.Path != nil {
					pathSegments := make([]string, 0)
					for _, segment := range rpc.Path.Segments {
						pathSegments = append(pathSegments, segment.Value)
					}
					endpoint["path"] = strings.Join(pathSegments, "/")
				}

				// Add documentation if available
				if rpc.Doc != nil {
					endpoint["doc"] = *rpc.Doc
				}

				// Include schema information if requested
				if includeSchemas {
					schemas := map[string]interface{}{}

					// For request and response schemas
					if rpc.RequestSchema != nil {
						str, _ := NamedOrInlineStruct(declByID, rpc.RequestSchema)
						qry, headers, cookies, body := StructBits(str, rpc.HttpMethods[0], false, false, true)
						schemas["request_schema"] = strings.Join([]string{"{", qry, headers, cookies, body, "}"}, "")
					}

					if rpc.ResponseSchema != nil {
						str, _ := NamedOrInlineStruct(declByID, rpc.ResponseSchema)
						qry, headers, cookies, body := StructBits(str, rpc.HttpMethods[0], true, false, true)
						schemas["response_schema"] = strings.Join([]string{"{", qry, headers, cookies, body, "}"}, "")
					}

					if len(schemas) > 0 {
						endpoint["schemas"] = schemas
					}
				}

				endpoints = append(endpoints, endpoint)
			}

			// Add endpoints to the service data if any were found
			if len(endpoints) > 0 {
				serviceData["endpoints"] = endpoints
			}
		}

		// Add service to the result map if it has data or endpoints
		if len(serviceData) > 0 {
			serviceMap[serviceName] = serviceData
		}
	}

	// Create the result object with services and summary
	result := map[string]interface{}{
		"services": serviceMap,
		"summary": map[string]interface{}{
			"total_services": len(serviceMap),
		},
	}

	// Add endpoint count to summary if we're including endpoints
	if includeEndpoints {
		totalEndpoints := 0
		for _, serviceData := range serviceMap {
			if endpoints, ok := serviceData["endpoints"].([]map[string]interface{}); ok {
				totalEndpoints += len(endpoints)
			}
		}
		result["summary"].(map[string]interface{})["total_endpoints"] = totalEndpoints
	}

	// Add filter information to summary if filters were applied
	if len(serviceNames) < len(md.Svcs) || (includeEndpoints && hasEndpointFilter) {
		filters := map[string]interface{}{}
		if len(serviceNames) < len(md.Svcs) {
			filters["services"] = serviceNames
		}
		if includeEndpoints && hasEndpointFilter {
			filters["endpoints"] = endpointNames
		}
		result["summary"].(map[string]interface{})["filters_applied"] = filters
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal services and endpoints: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (m *Manager) getMiddleware(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Find middleware definition locations from trace nodes
	middlewareDefLocations := make(map[string]map[string]interface{})

	// Scan through all packages to find trace nodes related to middleware
	for _, pkg := range md.Pkgs {
		for _, node := range pkg.TraceNodes {
			// Check for middleware definitions
			if node.GetMiddlewareDef() != nil {
				middlewareDef := node.GetMiddlewareDef()
				middlewareName := middlewareDef.Name

				// Use package path + name as a unique identifier
				middlewareID := fmt.Sprintf("%s/%s", middlewareDef.PkgRelPath, middlewareName)

				middlewareDefLocations[middlewareID] = map[string]interface{}{
					"filepath":     node.Filepath,
					"line_start":   node.SrcLineStart,
					"line_end":     node.SrcLineEnd,
					"column_start": node.SrcColStart,
					"column_end":   node.SrcColEnd,
					"package_path": middlewareDef.PkgRelPath,
				}
			}
		}
	}

	// Group middleware by type (global vs service-specific)
	globalMiddleware := make([]map[string]interface{}, 0)
	serviceMiddleware := make(map[string][]map[string]interface{})

	// Process all middleware
	for _, middleware := range md.Middleware {
		middlewareInfo := map[string]interface{}{
			"doc":    middleware.Doc,
			"global": middleware.Global,
		}

		// Add qualified name information if available
		if middleware.Name != nil {
			name := map[string]interface{}{
				"package": middleware.Name.Pkg,
				"name":    middleware.Name.Name,
			}
			middlewareInfo["name"] = name

			// Add definition location if available
			middlewareID := fmt.Sprintf("%s/%s", middleware.Name.Pkg, middleware.Name.Name)
			if location, exists := middlewareDefLocations[middlewareID]; exists {
				middlewareInfo["definition"] = map[string]interface{}{
					"filepath":     location["filepath"],
					"line_start":   location["line_start"],
					"line_end":     location["line_end"],
					"column_start": location["column_start"],
					"column_end":   location["column_end"],
				}
			}
		}

		// Add target information if available
		if len(middleware.Target) > 0 {
			targets := make([]map[string]interface{}, 0, len(middleware.Target))
			for _, target := range middleware.Target {
				targetInfo := map[string]interface{}{
					"type":  target.Type.String(),
					"value": target.Value,
				}
				targets = append(targets, targetInfo)
			}
			middlewareInfo["targets"] = targets
		}

		// Add to the appropriate group
		if middleware.Global {
			globalMiddleware = append(globalMiddleware, middlewareInfo)
		} else if middleware.ServiceName != nil {
			serviceName := *middleware.ServiceName
			if _, exists := serviceMiddleware[serviceName]; !exists {
				serviceMiddleware[serviceName] = make([]map[string]interface{}, 0)
			}
			serviceMiddleware[serviceName] = append(serviceMiddleware[serviceName], middlewareInfo)
		}
	}

	// Build the final result
	result := map[string]interface{}{
		"global":   globalMiddleware,
		"services": serviceMiddleware,
		"summary": map[string]interface{}{
			"total_middleware":   len(md.Middleware),
			"global_middleware":  len(globalMiddleware),
			"service_middleware": make(map[string]int),
			"service_count":      len(serviceMiddleware),
		},
	}

	// Add counts by service
	summary := result["summary"].(map[string]interface{})
	for service, middleware := range serviceMiddleware {
		summary["service_middleware"].(map[string]int)[service] = len(middleware)
	}

	// Sort middleware by name for consistent output
	sort.Slice(globalMiddleware, func(i, j int) bool {
		nameI := ""
		nameJ := ""
		if name, ok := globalMiddleware[i]["name"].(map[string]interface{}); ok {
			nameI = name["name"].(string)
		}
		if name, ok := globalMiddleware[j]["name"].(map[string]interface{}); ok {
			nameJ = name["name"].(string)
		}
		return nameI < nameJ
	})

	// Sort service middleware as well
	for service, middleware := range serviceMiddleware {
		sort.Slice(middleware, func(i, j int) bool {
			nameI := ""
			nameJ := ""
			if name, ok := middleware[i]["name"].(map[string]interface{}); ok {
				nameI = name["name"].(string)
			}
			if name, ok := middleware[j]["name"].(map[string]interface{}); ok {
				nameJ = name["name"].(string)
			}
			return nameI < nameJ
		})
		serviceMiddleware[service] = middleware
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal middleware information: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func (m *Manager) getAuthHandlers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	inst, err := m.getApp(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	md, err := inst.CachedMetadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Find auth handler definition locations from trace nodes
	authHandlerDefLocations := make(map[string]map[string]interface{})

	// Scan through all packages to find trace nodes related to auth handlers
	for _, pkg := range md.Pkgs {
		for _, node := range pkg.TraceNodes {
			// Check for auth handler definitions
			if node.GetAuthHandlerDef() != nil {
				authHandlerDef := node.GetAuthHandlerDef()
				serviceName := authHandlerDef.ServiceName
				handlerName := authHandlerDef.Name

				// Use service name + handler name as a unique identifier
				handlerID := fmt.Sprintf("%s/%s", serviceName, handlerName)

				authHandlerDefLocations[handlerID] = map[string]interface{}{
					"filepath":     node.Filepath,
					"line_start":   node.SrcLineStart,
					"line_end":     node.SrcLineEnd,
					"column_start": node.SrcColStart,
					"column_end":   node.SrcColEnd,
					"service_name": serviceName,
				}
			}
		}
	}

	// Process the main auth handler if it exists
	var mainAuthHandler map[string]interface{}
	if md.AuthHandler != nil {
		auth := md.AuthHandler

		authData := map[string]interface{}{
			"name":         auth.Name,
			"doc":          auth.Doc,
			"service_name": auth.ServiceName,
			"pkg_path":     auth.PkgPath,
			"pkg_name":     auth.PkgName,
		}

		// Add parameter and auth data type information
		if auth.Params != nil {
			paramsData, err := protojson.Marshal(auth.Params)
			if err == nil {
				var paramsJson interface{}
				if err := json.Unmarshal(paramsData, &paramsJson); err == nil {
					authData["params"] = paramsJson
				}
			}
		}

		if auth.AuthData != nil {
			authDataTypeData, err := protojson.Marshal(auth.AuthData)
			if err == nil {
				var authDataJson interface{}
				if err := json.Unmarshal(authDataTypeData, &authDataJson); err == nil {
					authData["auth_data"] = authDataJson
				}
			}
		}

		// Add location information if available
		handlerID := fmt.Sprintf("%s/%s", auth.ServiceName, auth.Name)
		if location, exists := authHandlerDefLocations[handlerID]; exists {
			authData["definition"] = map[string]interface{}{
				"filepath":     location["filepath"],
				"line_start":   location["line_start"],
				"line_end":     location["line_end"],
				"column_start": location["column_start"],
				"column_end":   location["column_end"],
			}
		}

		mainAuthHandler = authData
	}

	// Process gateway auth handlers
	gatewayAuthHandlers := make(map[string]map[string]interface{})

	for _, gateway := range md.Gateways {
		if gateway.Explicit != nil && gateway.Explicit.AuthHandler != nil {
			auth := gateway.Explicit.AuthHandler

			authData := map[string]interface{}{
				"name":         auth.Name,
				"doc":          auth.Doc,
				"service_name": auth.ServiceName,
				"pkg_path":     auth.PkgPath,
				"pkg_name":     auth.PkgName,
				"gateway_name": gateway.EncoreName,
			}

			// Add parameter and auth data type information
			if auth.Params != nil {
				paramsData, err := protojson.Marshal(auth.Params)
				if err == nil {
					var paramsJson interface{}
					if err := json.Unmarshal(paramsData, &paramsJson); err == nil {
						authData["params"] = paramsJson
					}
				}
			}

			if auth.AuthData != nil {
				authDataTypeData, err := protojson.Marshal(auth.AuthData)
				if err == nil {
					var authDataJson interface{}
					if err := json.Unmarshal(authDataTypeData, &authDataJson); err == nil {
						authData["auth_data"] = authDataJson
					}
				}
			}

			// Add location information if available
			handlerID := fmt.Sprintf("%s/%s", auth.ServiceName, auth.Name)
			if location, exists := authHandlerDefLocations[handlerID]; exists {
				authData["definition"] = map[string]interface{}{
					"filepath":     location["filepath"],
					"line_start":   location["line_start"],
					"line_end":     location["line_end"],
					"column_start": location["column_start"],
					"column_end":   location["column_end"],
				}
			}

			gatewayAuthHandlers[gateway.EncoreName] = authData
		}
	}

	// Build the final result
	result := map[string]interface{}{
		"main_auth_handler":     mainAuthHandler,
		"gateway_auth_handlers": gatewayAuthHandlers,
		"summary": map[string]interface{}{
			"has_main_auth":      mainAuthHandler != nil,
			"gateway_count":      len(md.Gateways),
			"auth_gateway_count": len(gatewayAuthHandlers),
		},
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth handler information: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
