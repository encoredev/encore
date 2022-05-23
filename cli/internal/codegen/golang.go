package codegen

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	. "github.com/dave/jennifer/jen"
	"github.com/fatih/structtag"

	"encr.dev/cli/internal/version"
	"encr.dev/internal/gocodegen"
	"encr.dev/parser/encoding"
	meta "encr.dev/proto/encore/parser/meta/v1"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// goGenVersion allows us to introduce breaking changes in the generated code but behind a switch
// meaning that people with client code reliant on the old behaviour can continue to generate the
// old code.
type goGenVersion int

const (
	// GoInitial is the originally released Go client generator
	GoInitial goGenVersion = iota

	// GoExperimental can be used to lock experimental or uncompleted features in the generated code
	// It should always be the last item in the enum
	GoExperimental
)

const goGenLatestVersion = GoExperimental - 1

type golang struct {
	md               *meta.Data
	enc              *gocodegen.MarshallingCodeGenerator
	generatorVersion goGenVersion
}

func (g *golang) Generate(buf *bytes.Buffer, appSlug string, md *meta.Data) (err error) {
	g.md = md
	g.enc = gocodegen.NewMarshallingCodeGenerator("serde", true)

	namedTypes := getNamedTypes(md)

	// Create a new client file
	file := NewFile("client")

	// Generate the parent Client struct
	g.generateClient(file, appSlug, md.Svcs)

	// Generate the types and service client structs
	for _, service := range md.Svcs {
		g.generateTypeDefinitions(file, namedTypes.Decls(service.Name))

		if hasPublicRPC(service) {
			if err := g.generateServiceClient(file, service); err != nil {
				return errors.Wrapf(err, "unable to generate service client for service: %s", service)
			}
		}
	}

	// Generate the base client
	g.generateBaseClient(file)

	// Write the APIError type
	g.writeErrorType(file)

	// Generate the serializer
	g.enc.WriteToFile(file)

	// Finally, render the client
	if err := file.Render(buf); err != nil {
		return errors.Wrap(err, "unable to generate go client")
	}

	return nil
}

func (g *golang) Version() int {
	return int(g.generatorVersion)
}

func (g *golang) cleanServiceName(service *meta.Service) string {
	return strings.Title(strings.ToLower(service.Name))
}

// generateClient creates the Client struct, Option type and New function
func (g *golang) generateClient(file *File, appSlug string, services []*meta.Service) {
	// List all services which have public RPCs or types we need
	fieldDef := make([]Code, 0, len(services))
	fieldInit := make(Dict)
	for _, service := range services {
		if !hasPublicRPC(service) {
			continue
		}

		name := g.cleanServiceName(service)

		fieldDef = append(fieldDef,
			Id(name).Id(fmt.Sprintf("%sClient", name)),
		)

		fieldInit[Id(name)] = Op("&").Id(fmt.Sprintf("%sClient", strings.ToLower(name))).Values(Id("base"))
	}

	// The client struct
	file.Commentf("Client is an API client for the %s Encore application.", appSlug)
	file.Add(
		Type().Id("Client").Struct(fieldDef...),
		Line(),
		Line(),
	)

	file.Comment("BaseURL is the base URL for calling the Encore application's API.")
	file.Type().Id("BaseURL").String()
	file.Line()

	file.Const().Id("Local").Id("BaseURL").Op("=").Lit("http://localhost:4000")
	file.Line()

	file.Comment("Environment returns a BaseURL for calling the cloud environment with the given name.")
	file.Func().Id("Environment").
		Params(Id("name").String()).
		Id("BaseURL").
		Block(Return(
			Id("BaseURL").Call(
				Qual("fmt", "Sprintf").Call(
					Lit(fmt.Sprintf("https://%%s-%s.encr.app", appSlug)),
					Id("name"),
				),
			),
		))

	// Option type alias
	file.Comment("Option allows you to customise the baseClient used by the Client")
	file.Add(
		Type().Id("Option").Op("=").
			Func().Params(Id("client").Op("*").Id("baseClient")).Error(),
		Line(),
		Line(),
	)

	// New Function
	file.Comment("New returns a Client for calling the public and authenticated APIs of your Encore application.")
	file.Comment("You can customize the behaviour of the client using the given Option functions, such as WithHTTPClient or WithAuthToken.")
	file.Add(
		Func().Id("New").
			Params(
				Id("target").Id("BaseURL"),
				Id("options").Id("...Option"),
			).
			Params(Op("*").Id("Client"), Error()).
			Block(
				Comment("Parse the base URL where the Encore application is being hosted"),
				List(Id("baseURL"), Err()).
					Op(":=").
					Qual("net/url", "Parse").Call(String().Call(Id("target"))),
				If(Err().Op("!=").Nil()).Block(
					Return(
						Nil(),
						Qual("fmt", "Errorf").Call(Lit("unable to parse base url: %w"), Err()),
					),
				),
				Line(),

				Comment("Create a client with sensible defaults"),
				Id("base").Op(":=").Op("&").Id("baseClient").Values(Dict{
					Id("baseURL"):    Id("baseURL"),
					Id("httpClient"): Qual("net/http", "DefaultClient"),
					Id("userAgent"):  Lit(fmt.Sprintf("%s-Generated-Go-Client (Encore/%s)", appSlug, version.Version)),
				}),
				Line(),

				Comment("Apply any given options"),
				For(List(Id("_"), Id("option")).Op(":=").Range().Id("options")).Block(
					If(
						Id("err").Op(":=").Id("option").Call(Id("base")),
						Id("err").Op("!=").Nil(),
					).Block(
						Return(
							Nil(),
							Qual("fmt", "Errorf").Call(
								Lit("unable to apply client option: %w"),
								Id("err"),
							),
						),
					),
				),
				Line(),

				Return(Op("&").Id("Client").Values(fieldInit), Nil()),
			),
	)

	// Generate the WithHttpClient function
	g.generateOptionFunc(
		file,
		"HTTPClient",
		`can be used to configure the underlying HTTP client used when making API calls. 

Defaults to http.DefaultClient`,
		&Statement{Id("client").Id("HTTPDoer")},
		&Statement{
			Id("base").Dot("httpClient").Op("=").Id("client"),
			Return(Nil()),
		},
	)

	// Generate the WithAuthToken function
	g.generateOptionFunc(
		file,
		"AuthToken",
		"allows you to set the auth token to be used for each request",
		&Statement{Id("token").String()},
		&Statement{
			Id("base").Dot("tokenGenerator").Op("=").Func().
				Params(Id("_").Qual("context", "Context")).
				Params(String(), Error()).
				Block(Return(Id("token"), Nil())),
			Return(Nil()),
		},
	)

	g.generateOptionFunc(
		file,
		"AuthFunc",
		`allows you to pass a function which is called for each request to return an access token.`,
		&Statement{
			Id("tokenGenerator").Func().
				Params(Id("ctx").Qual("context", "Context")).
				Params(String(), Error()),
		},
		&Statement{
			&Statement{Id("base").Dot("tokenGenerator").Op("=").Id("tokenGenerator")},
			Return(Nil()),
		},
	)
}

// generateOptionFunc is a helper for reducing the boilerplate we have when creating the option functions
func (g *golang) generateOptionFunc(file *File, optionName string, doc string, params *Statement, block *Statement) {
	for i, line := range strings.Split(doc, "\n") {
		if i == 0 {
			file.Commentf("With%s %s", optionName, line)
		} else {
			file.Comment(line)
		}
	}

	file.Func().
		Id(fmt.Sprintf("With%s", optionName)).
		Params(*params...).
		Id("Option").
		Block(
			Return(Func().Params(Id("base").Op("*").Id("baseClient")).Error().Block(
				*block...,
			)),
		)
}

func (g *golang) generateServiceClient(file *File, service *meta.Service) error {
	name := g.cleanServiceName(service)
	interfaceName := fmt.Sprintf("%sClient", name)
	structName := fmt.Sprintf("%sClient", strings.ToLower(name))

	// The interface
	file.Commentf("%s Provides you access to call public and authenticated APIs on %s. The concrete implementation is %s.", interfaceName, service.Name, structName)
	file.Comment("It is setup as an interface allowing you to use GoMock to create mock implementations during tests.")
	var interfaceMethods []Code
	for _, rpc := range service.Rpcs {
		if rpc.AccessType == meta.RPC_PRIVATE {
			continue
		}

		// Add the documentation for the API to the interface method
		if rpc.Doc != "" {
			// Add a newline if this is not the first method
			if len(interfaceMethods) > 0 {
				interfaceMethods = append(interfaceMethods, Line())
			}

			for _, line := range strings.Split(strings.TrimSpace(rpc.Doc), "\n") {
				interfaceMethods = append(interfaceMethods, Comment(line))
			}
		}

		interfaceMethods = append(interfaceMethods,
			Id(rpc.Name).Add(g.rpcParams(rpc)).Add(g.rpcReturnType(rpc, false)),
		)
	}
	file.Type().Id(interfaceName).Interface(interfaceMethods...)
	file.Line()

	// The struct
	file.Type().Id(structName).Struct(
		Id("base").Op("*").Id("baseClient"),
	)
	file.Line()
	file.Var().Id("_").Id(interfaceName).Op("=").Params(Op("*").Id(structName)).Params(Nil())

	// The API functions
	for _, rpc := range service.Rpcs {
		if rpc.AccessType == meta.RPC_PRIVATE {
			continue
		}

		for _, line := range strings.Split(strings.TrimSpace(rpc.Doc), "\n") {
			if line != "" {
				file.Comment(line)
			}
		}

		callSite, err := g.rpcCallSite(rpc)
		if err != nil {
			return errors.Wrapf(err, "rpc: %s", rpc.Name)
		}

		file.Func().
			Params(Id("c").Op("*").Id(structName)).
			Id(rpc.Name).
			Add(
				g.rpcParams(rpc),
				g.rpcReturnType(rpc, true),
			).Block(callSite...)
		file.Line()
	}
	return nil
}

func (g *golang) rpcParams(rpc *meta.RPC) Code {
	params := []Code{
		Id("ctx").Qual("context", "Context"),
	}

	if rpc.Path != nil && len(rpc.Path.Segments) > 0 {
		for _, segment := range rpc.Path.Segments {
			if segment.Type == meta.PathSegment_LITERAL {
				continue
			}

			// We'll default to strings for most things
			typ := String()

			switch segment.ValueType {
			case meta.PathSegment_BOOL:
				typ = Bool()

			case meta.PathSegment_INT8:
				typ = Int8()

			case meta.PathSegment_INT16:
				typ = Int16()

			case meta.PathSegment_INT32:
				typ = Int32()

			case meta.PathSegment_INT64:
				typ = Int64()

			case meta.PathSegment_INT:
				typ = Int()

			case meta.PathSegment_UINT8:
				typ = Uint8()

			case meta.PathSegment_UINT16:
				typ = Uint16()

			case meta.PathSegment_UINT32:
				typ = Uint32()

			case meta.PathSegment_UINT64:
				typ = Uint64()

			case meta.PathSegment_UINT:
				typ = Uint()
			}

			params = append(params,
				Id(segment.Value).Add(typ),
			)
		}
	}

	if rpc.Proto == meta.RPC_RAW {
		params = append(params, Id("request").Op("*").Qual("net/http", "Request"))
	} else {
		if rpc.RequestSchema != nil {
			params = append(params, Id("params").Add(g.getType(rpc.RequestSchema)))
		}
	}

	return Params(params...)
}

func (g *golang) rpcReturnType(rpc *meta.RPC, concreteImpl bool) Code {
	if rpc.Proto == meta.RPC_RAW {
		return Params(Op("*").Qual("net/http", "Response"), Error())
	}

	if rpc.ResponseSchema == nil {
		return Error()
	}

	if concreteImpl {
		// For the concrete implementation we want the response type to be named so we can
		// refer to it without having to define a variable.
		return Params(Id("resp").Add(g.getType(rpc.ResponseSchema)), Err().Error())
	} else {
		return Params(g.getType(rpc.ResponseSchema), Error())
	}
}

func (g *golang) rpcCallSite(rpc *meta.RPC) (code []Code, err error) {
	// Work out how we're going to encode and call this RPC
	rpcEncoding, err := encoding.DescribeRPC(g.md, rpc)
	if err != nil {
		return nil, errors.Wrapf(err, "rpc %s", rpc.Name)
	}

	// Raw end points just pass through the request
	// and need no further code generation
	if rpc.Proto == meta.RPC_RAW {
		code = append(
			code,
			List(Id("path"), Err()).Op(":=").Qual("net/url", "Parse").Call(g.createApiPath(rpc, false)),
			If(Err().Op("!=").Nil()).Block(
				Return(
					Nil(),
					Qual("fmt", "Errorf").Call(Lit("unable to parse api url: %w"), Err()),
				),
			),
			Id("request").Op("=").Id("request").Dot("WithContext").Call(Id("ctx")),
			If(Id("request").Dot("Method").Op("==").Lit("")).Block(
				Id("request").Dot("Method").Op("=").Lit(rpcEncoding.DefaultRequestEncoding.HTTPMethods[0]),
			),
			Id("request").Dot("URL").Op("=").Id("path"),
			Line(),
			Return(Id("c").Dot("base").Dot("Do").Call(Id("request"))),
		)

		return
	}

	headers := Nil()
	body := Nil()
	withQueryString := false

	// Work out how we encode the Request Schema
	if rpc.RequestSchema != nil {
		headerFields, queryFields, bodyFields, err := toFieldLists(rpcEncoding.DefaultRequestEncoding.Fields)
		if err != nil {
			return nil, err
		}

		if len(headerFields) > 0 || len(queryFields) > 0 {
			code = append(code, Comment("Convert our params into the objects we need for the request"))
		}

		enc := g.enc.NewPossibleInstance("reqEncoder")

		// Generate the headers
		if len(headerFields) > 0 {
			values := Dict{}

			for _, field := range headerFields {
				slice, err := enc.ToStringSlice(
					field.Field.Typ,
					Id("params").Dot(field.Field.Name),
				)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to encode header %s", field.Field.Name)
				}
				values[Lit(field.Name)] = slice
			}

			headers = Id("headers")
			enc.Add(Id("headers").Op(":=").Qual("net/http", "Header").Values(values), Line())
		}

		// Generate the query string
		if len(queryFields) > 0 {
			withQueryString = true
			values := Dict{}

			// Check the request schema for fields we can put in the query string
			for _, field := range queryFields {
				slice, err := enc.ToStringSlice(
					field.Field.Typ,
					Id("params").Dot(field.Field.Name),
				)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to encode query fields %s", field.Field.Name)
				}

				values[Lit(field.Name)] = slice
			}

			enc.Add(Id("queryString").Op(":=").Qual("net/url", "Values").Values(values), Line())
		}

		if rpc.ResponseSchema != nil {
			code = append(code, enc.Finalize(
				Id("err").Op("=").Qual("fmt", "Errorf").Call(
					Lit("unable to marshal parameters: %w"),
					enc.LastError(),
				),
				Return(),
			)...)
		} else {
			code = append(code, enc.Finalize(
				Return(Qual("fmt", "Errorf").Call(
					Lit("unable to marshal parameters: %w"),
					enc.LastError(),
				)),
			)...)
		}

		// Generate the body
		if len(bodyFields) > 0 {
			if len(headerFields) == 0 && len(queryFields) == 0 {
				// In the simple case we can just encode the params as the body directly
				body = Id("params")
			} else {
				// Else we need a new struct called "body"
				body = Id("body")

				types, err := g.generateAnonStructTypes(bodyFields, "json")
				if err != nil {
					return nil, err
				}

				values := Dict{}
				for _, field := range bodyFields {
					values[Id(field.Field.Name)] = Id("params").Dot(field.Field.Name)
				}

				code = append(code,
					Comment("Construct the body with only the fields which we want encoded within the body (excluding query string or header fields)"),
					Id("body").Op(":=").Struct(types...).Values(values),
					Line(),
				)
			}
		}
	}

	// Make the request
	resp := Nil()
	apiCallCode := func() Code {
		return Id("callAPI").Call(
			Id("ctx"),
			Id("c").Dot("base"),
			Lit(rpcEncoding.DefaultRequestEncoding.HTTPMethods[0]),
			g.createApiPath(rpc, withQueryString),
			headers,
			body,
			resp,
		)
	}

	// If there's no response schema, we can just return the call to the API directly
	if rpc.ResponseSchema == nil {
		code = append(code,
			List(Id("_"), Err()).Op(":=").Add(apiCallCode()),
			Return(Err()),
		)
		return
	}

	hasAnonResponseStruct := false
	headerFields, queryFields, bodyFields, err := toFieldLists(rpcEncoding.ResponseEncoding.Fields)
	if err != nil {
		return nil, err
	}

	// If we have a response object, we need
	if len(bodyFields) > 0 {
		if len(headerFields) == 0 && len(queryFields) == 0 {
			// If there are no other fields, we can just take the return type and pass it straight through
			resp = Op("&").Id("resp")
		} else {
			hasAnonResponseStruct = true
			resp = Op("&").Id("respBody")

			// we need to construct an anonymous struct
			types, err := g.generateAnonStructTypes(bodyFields, "json")
			if err != nil {
				return nil, errors.Wrap(err, "response unmarshal")
			}

			code = append(code,
				Comment("We only want the response body to marshal into these fields and none of the header fields,"),
				Comment("so we'll construct a new struct with only those fields."),
				Id("respBody").Op(":=").Struct(types...).Values(),
				Line(),
			)
		}
	}

	// The API Call itself
	code = append(code, Comment("Now make the actual call to the API"))

	headersId := "_"
	if len(headerFields) > 0 {
		headersId = "respHeaders"
		code = append(code, Var().Id(headersId).Qual("net/http", "Header"))
	}

	code = append(code,
		List(Id(headersId), Err()).Op("=").Add(apiCallCode()),
		If(Err().Op("!=").Nil()).Block(
			Return(),
		),
		Line(),
	)

	// In we have an anonymous response struct, we need to copy the results into the full response struct
	if hasAnonResponseStruct || len(headerFields) > 0 {
		code = append(code, Comment("Copy the unmarshalled response body into our response struct"))

		enc := g.enc.NewPossibleInstance("respDecoder")
		for _, field := range rpcEncoding.ResponseEncoding.Fields {
			switch field.Location {
			case encoding.Header:
				str, err := enc.FromString(
					field.Field.Typ,
					field.Field.Name,
					Id(headersId).Dot("Get").Call(Lit(field.Name)),
					Id(headersId).Dot("Values").Call(Lit(field.Name)),
					false,
				)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to convert %s to string in response header", field.Name)
				}

				enc.Add(Id("resp").Dot(field.Field.Name).Op("=").Add(str))
			case encoding.Body:
				enc.Add(Id("resp").Dot(field.Field.Name).Op("=").Id("respBody").Dot(field.Field.Name))
			default:
				return nil, errors.Newf("unsupported response location: %+v", field.Location)
			}
		}

		code = append(code, enc.Finalize(
			Id("err").Op("=").Qual("fmt", "Errorf").Call(
				Lit("unable to unmarshal headers: %w"),
				enc.LastError(),
			),
			Return(),
		)...)
		code = append(code, Line())
	}

	code = append(code, Return())

	return code, err
}

func (g *golang) declToID(decl *schema.Decl) *Statement {
	return Id(fmt.Sprintf("%s%s", strings.Title(decl.Loc.PkgName), strings.Title(decl.Name)))
}

func (g *golang) getType(typ *schema.Type) Code {
	switch typ := typ.Typ.(type) {
	case *schema.Type_Named:
		decl := g.md.Decls[typ.Named.Id]

		named := g.declToID(decl)

		if len(typ.Named.TypeArguments) == 0 {
			return named
		}

		// Add the type arguments
		types := make([]Code, len(typ.Named.TypeArguments))
		for i, t := range typ.Named.TypeArguments {
			types[i] = g.getType(t)
		}

		return named.Types(types...)

	case *schema.Type_List:
		return Index().Add(g.getType(typ.List.Elem))

	case *schema.Type_Map:
		return Map(g.getType(typ.Map.Key)).Add(g.getType(typ.Map.Value))

	case *schema.Type_Builtin:
		switch typ.Builtin {
		case schema.Builtin_ANY:
			return Any()
		case schema.Builtin_BOOL:
			return Bool()
		case schema.Builtin_INT:
			return Int()
		case schema.Builtin_INT8:
			return Int8()
		case schema.Builtin_INT16:
			return Int16()
		case schema.Builtin_INT32:
			return Int32()
		case schema.Builtin_INT64:
			return Int64()
		case schema.Builtin_UINT:
			return Uint()
		case schema.Builtin_UINT8:
			return Uint8()
		case schema.Builtin_UINT16:
			return Uint16()
		case schema.Builtin_UINT32:
			return Uint32()
		case schema.Builtin_UINT64:
			return Uint64()
		case schema.Builtin_FLOAT32:
			return Float32()
		case schema.Builtin_FLOAT64:
			return Float64()
		case schema.Builtin_STRING:
			return String()
		case schema.Builtin_BYTES:
			return Index().Byte()
		case schema.Builtin_TIME:
			return Qual("time", "Time")
		case schema.Builtin_JSON:
			return Qual("encoding/json", "RawMessage")
		case schema.Builtin_UUID, schema.Builtin_USER_ID:
			// we don't want to add any custom depdancies, so these come in as strings
			return String()
		default:
			return Any()
		}

	case *schema.Type_Struct:
		fields := make([]Code, 0, len(typ.Struct.Fields))

		for _, field := range typ.Struct.Fields {
			// Skip over hidden fields
			if field.JsonName == "-" || field.QueryStringName == "-" {
				continue
			}

			// The base field name and type
			fieldTyp := Id(field.Name).Add(g.getType(field.Typ))

			// Add the field tags
			if field.RawTag != "" {
				tags, err := structtag.Parse(field.RawTag)
				if err != nil {
					panic("raw tags failed to parse") // This shouldn't happen at runtime, because the parser should have caught this
				}

				tagsForJen := make(map[string]string)
				for _, tag := range tags.Tags() {
					tagsForJen[tag.Key] = tag.Value()
				}

				fieldTyp = fieldTyp.Tag(tagsForJen)
			}

			// Add the docs for the field
			if field.Doc != "" {
				lines := strings.Split(strings.TrimSpace(field.Doc), "\n")

				if len(lines) == 1 {
					fieldTyp = fieldTyp.Comment(lines[0])
				} else {
					fields = append(fields, Line())
					for _, line := range lines {
						fields = append(fields, Comment(line))
					}
				}
			}

			// Finally, add the field to the list of fields on the struct
			fields = append(fields, fieldTyp)
		}

		return Struct(fields...)

	case *schema.Type_TypeParameter:
		decl := g.md.Decls[typ.TypeParameter.DeclId]
		typeParam := decl.TypeParams[typ.TypeParameter.ParamIdx]

		return Id(typeParam.Name)

	default:
		return Any()
	}
}

func (g *golang) createApiPath(rpc *meta.RPC, withQueryString bool) (urlPath *Statement) {
	var url strings.Builder
	params := make([]Code, 0)

	for _, segment := range rpc.Path.Segments {
		url.WriteByte('/')

		if segment.Type == meta.PathSegment_LITERAL {
			url.WriteString(segment.Value)
		} else {
			switch segment.ValueType {
			case meta.PathSegment_STRING, meta.PathSegment_UUID:
				url.WriteString("%s")
			case meta.PathSegment_BOOL:
				url.WriteString("%t")
			case meta.PathSegment_INT8, meta.PathSegment_INT16, meta.PathSegment_INT32, meta.PathSegment_INT64, meta.PathSegment_INT,
				meta.PathSegment_UINT8, meta.PathSegment_UINT16, meta.PathSegment_UINT32, meta.PathSegment_UINT64, meta.PathSegment_UINT:
				url.WriteString("%d")
			default:
				url.WriteString("%v")
			}

			params = append(params, Id(segment.Value))
		}
	}

	// Construct the query string
	if withQueryString {
		url.WriteString("?%s")
		params = append(params, Id("queryString").Dot("Encode").Call())
	}

	if len(params) == 0 {
		urlPath = Lit(url.String())
	} else {
		// Prepend the string format
		params = append([]Code{Lit(url.String())}, params...)
		urlPath = Qual("fmt", "Sprintf").Call(params...)
	}

	return urlPath
}

func (g *golang) generateTypeDefinitions(file *File, decls []*schema.Decl) {
	sort.Slice(decls, func(i, j int) bool {
		return decls[i].Name < decls[j].Name
	})

	for _, decl := range decls {
		// Write the docs
		if decl.Doc != "" {
			for _, line := range strings.Split(strings.TrimSpace(decl.Doc), "\n") {
				file.Comment(line)
			}
		} else {
			file.Line()
		}

		// Create the base type definition; `type X[T]`
		typ := file.Type().Add(g.declToID(decl))
		if len(decl.TypeParams) > 0 {
			types := make([]Code, len(decl.TypeParams))

			for i, param := range decl.TypeParams {
				types[i] = Id(param.Name).Any()
			}

			typ = typ.Types(types...)
		}

		// All types which are not structs should be aliases
		if decl.Type.GetStruct() == nil && len(decl.TypeParams) == 0 {
			typ = typ.Op("=")
		}

		// Add the type
		typ.Add(g.getType(decl.Type))
	}
}

func (g *golang) generateAnonStructTypes(fields []*encoding.ParameterEncoding, encodingTag string) (types []Code, err error) {
	for _, field := range fields {
		var tagValue strings.Builder
		tagValue.WriteString(field.Name)

		// Parse the tags and extract the encoding tag
		tags, err := structtag.Parse(field.Field.RawTag)
		if err != nil {
			return nil, errors.Wrapf(err, "parse tags: %s", field.Field.Name)
		}
		if tag, err := tags.Get(encodingTag); err == nil {
			options := strings.Join(tag.Options, ",")
			if options != "" {
				tagValue.WriteRune(',')
				tagValue.WriteString(options)
			}
		}

		types = append(
			types,
			Id(field.Field.Name).Add(g.getType(field.Field.Typ)).Tag(map[string]string{encodingTag: tagValue.String()}),
		)
	}

	return
}

func (g *golang) generateBaseClient(file *File) {
	// Add the interface
	file.Comment("HTTPDoer is an interface which can be used to swap out the default")
	file.Comment("HTTP client (http.DefaultClient) with your own custom implementation.")
	file.Comment("This can be used to inject middleware or mock responses during unit tests.")
	file.Type().Id("HTTPDoer").Interface(
		Id("Do").
			Params(Id("req").Op("*").Qual("net/http", "Request")).
			Params(Op("*").Qual("net/http", "Response"), Error()),
	)

	// Add the base client struct
	file.Line()
	file.Comment("baseClient holds all the information we need to make requests to an Encore application")
	file.Type().Id("baseClient").Struct(
		Id("tokenGenerator").Func().
			Params(Id("ctx").Qual("context", "Context")).
			Params(String(), Error()).
			Comment("The function which will add the bearer token to the requests"),

		Id("httpClient").Id("HTTPDoer").
			Comment("The HTTP client which will be used for all API requests"),

		Id("baseURL").Op("*").Qual("net/url", "URL").
			Comment("The base URL which API requests will be made against"),

		Id("userAgent").String().
			Commentf("What user agent we will use in the API requests"),
	)

	// Add the Do method for th base client
	file.Line()
	file.Comment("Do sends the req to the Encore application adding the authorization token as required.")
	file.Func().
		Params(Id("b").Op("*").Id("baseClient")).
		Id("Do").
		Params(Id("req").Op("*").Qual("net/http", "Request")).
		Params(Op("*").Qual("net/http", "Response"), Error()).
		Block(
			Id("req").Dot("Header").Dot("Set").Call(
				Lit("Content-Type"),
				Lit("application/json"),
			),
			Id("req").Dot("Header").Dot("Set").Call(
				Lit("User-Agent"),
				Id("b").Dot("userAgent"),
			),
			Line(),

			Comment("If a authorization token generator is present, call it and add the returned token to the request"),
			If(Id("b").Dot("tokenGenerator").Op("!=").Nil()).Block(
				If(
					List(Id("token"), Err()).Op(":=").
						Id("b").Dot("tokenGenerator").Call(
						Id("req").Dot("Context").Call(),
					),
					Err().Op("!=").Nil(),
				).Block(
					Return(Nil(), Qual("fmt", "Errorf").Call(Lit("unable to create authorization token for api request: %w"), Err())),
				).Else().If(Id("token").Op("!=").Lit("")).Block(
					Id("req").Dot("Header").Dot("Set").Call(
						Lit("Authorization"),
						Qual("fmt", "Sprintf").
							Call(Lit("Bearer %s"), Id("token")),
					),
				),
			),
			Line(),

			Comment("Merge the base URL and the API URL"),
			Id("req").Dot("URL").Op("=").
				Id("b").Dot("baseURL").Dot("ResolveReference").Call(Id("req").Dot("URL")),
			Id("req").Dot("Host").Op("=").Id("req").Dot("URL").Dot("Host"),
			Line(),

			Comment("Finally, make the request via the configured HTTP Client"),
			Return(
				Id("b").Dot("httpClient").Dot("Do").Call(Id("req")),
			),
		)

	// Add the call API function
	file.Line()
	file.Comment("callAPI is used by each generated API method to actually make request and decode the responses")
	file.Func().
		Id("callAPI").
		Params(
			Id("ctx").Qual("context", "Context"),
			Id("client").Op("*").Id("baseClient"),
			Id("method"),
			Id("path").String(),
			Id("headers").Qual("net/http", "Header"),
			List(Id("body"), Id("resp")).Any(),
		).
		Params(Qual("net/http", "Header"), Error()).
		Block(
			Comment("Encode the API body"),
			Var().Id("bodyReader").Qual("io", "Reader"),
			If(Id("body").Op("!=").Nil()).Block(
				List(Id("bodyBytes"), Err()).Op(":=").
					Qual("encoding/json", "Marshal").
					Call(Id("body")),
				If(Err().Op("!=").Nil()).Block(
					Return(Nil(), Qual("fmt", "Errorf").Call(Lit("marshal request: %w"), Err())),
				),

				Id("bodyReader").Op("=").Qual("bytes", "NewReader").Call(Id("bodyBytes")),
			),
			Line(),

			Comment("Create the request"),
			List(Id("req"), Err()).Op(":=").
				Qual("net/http", "NewRequestWithContext").
				Call(
					Id("ctx"), Id("method"), Id("path"), Id("bodyReader"),
				),
			If(Err().Op("!=").Nil()).Block(
				Return(Nil(), Qual("fmt", "Errorf").Call(Lit("create request: %w"), Err())),
			),
			Line(),

			Comment("Add any headers to the request"),
			For(List(Id("header"), Id("values")).Op(":=").Range().Id("headers")).Block(
				For(List(Id("_"), Id("value")).Op(":=").Range().Id("values")).Block(
					Id("req").Dot("Header").Dot("Add").Call(Id("header"), Id("value")),
				),
			),
			Line(),

			Comment("Make the request via the base client"),
			List(Id("rawResponse"), Err()).Op(":=").
				Id("client").Dot("Do").Call(Id("req")),
			If(Err().Op("!=").Nil()).Block(
				Return(Nil(), Qual("fmt", "Errorf").Call(Lit("request failed: %w"), Err())),
			),
			Defer().Func().Params().Block(
				Id("_").Op("=").Id("rawResponse").Dot("Body").Dot("Close").Call(),
			).Call(),
			If(Id("rawResponse").Dot("StatusCode").Op(">=").Lit(400)).Block(
				Comment("Read the full body sent back"),
				List(Id("body"), Err()).Op(":=").Qual("io/ioutil", "ReadAll").Call(Id("rawResponse").Dot("Body")),
				If(Err().Op("!=").Nil()).Block(
					Return(
						Nil(),
						Op("&").Id("APIError").Values(Dict{
							Id("Code"):    Id("ErrUnknown"),
							Id("Message"): Qual("fmt", "Sprintf").Call(Lit("got error response without readable body: %s"), Id("rawResponse").Dot("Status")),
						}),
					),
				),
				Line(),

				Comment("Attempt to decode the error response as a structured APIError"),
				Id("apiError").Op(":=").Op("&").Id("APIError").Block(),
				If(
					Err().Op(":=").Qual("encoding/json", "Unmarshal").
						Call(Id("body"), Id("apiError")),
					Err().Op("!=").Nil(),
				).Block(
					Comment("If the error is not a parsable as an APIError, then return an error with the raw body"),
					Return(
						Nil(),
						Op("&").Id("APIError").Values(Dict{
							Id("Code"):    Id("ErrUnknown"),
							Id("Message"): Qual("fmt", "Sprintf").Call(Lit("got error response: %s"), String().Call(Id("body"))),
						}),
					),
				),
				Return(Nil(), Id("apiError")),
			),
			Line(),

			Comment("Decode the response"),
			If(
				Id("resp").Op("!=").Nil(),
			).Block(
				If(
					Err().Op(":=").Qual("encoding/json", "NewDecoder").
						Call(Id("rawResponse").Dot("Body")).
						Dot("Decode").
						Call(Id("resp")),
					Err().Op("!=").Nil(),
				).Block(
					Return(Nil(), Qual("fmt", "Errorf").Call(Lit("decode response: %w"), Err())),
				),
			),
			Return(
				Id("rawResponse").Dot("Header"),
				Nil(),
			),
		)
}

func (g *golang) writeErrorType(file *File) {
	const ErrPrefix = "Err"

	// Create the error type
	file.Line()
	file.Comment("APIError is the error type returned by the API")
	file.Type().Id("APIError").Struct(
		Id("Code").Id("ErrCode").Tag(map[string]string{"json": "code"}),
		Id("Message").String().Tag(map[string]string{"json": "message"}),
		Id("Details").Any().Tag(map[string]string{"json": "details"}),
	)
	file.Func().Params(Id("e").Op("*").Id("APIError")).Id("Error").Params().String().Block(
		Return(Qual("fmt", "Sprintf").Call(Lit("%s: %s"), Id("e").Dot("Code"), Id("e").Dot("Message"))),
	)
	file.Line()
	file.Line()

	// Create the ErrCode type and list
	file.Type().Id("ErrCode").Int()
	errTypes := make([]Code, 0)
	for i, err := range errorCodes {
		for _, line := range strings.Split(strings.TrimSpace(err.Comment), "\n") {
			// Fix the comment with the prefix
			if strings.HasPrefix(line, err.Name) {
				line = ErrPrefix + line
			}
			errTypes = append(errTypes, Comment(line))
		}

		errTypes = append(
			errTypes,
			Id(ErrPrefix+err.Name).Id("ErrCode").Op("=").Lit(i),
			Line(),
		)
	}
	file.Const().Defs(errTypes...)

	// Create the functions to convert an error to string
	file.Comment("// String returns the string representation of the error code")
	file.Func().Params(Id("c").Id("ErrCode")).Id("String").Params().String().Block(
		Switch(Id("c")).BlockFunc(func(g *Group) {
			for _, err := range errorCodes {
				g.Case(Id(ErrPrefix + err.Name)).Block(
					Return(Lit(convertIdentifierTo(err.Name, SnakeCase))),
				)
			}

			g.Default().Block(
				Return(Lit("unknown")),
			)
		}),
	)
	file.Line()

	file.Comment("MarshalJSON converts the error code to a human-readable string")
	file.Func().Params(Id("c").Id("ErrCode")).Id("MarshalJSON").Params().Params(Index().Byte(), Error()).Block(
		Return(Index().Byte().Call(Qual("fmt", "Sprintf").Call(Lit("\"%s\""), Id("c"))), Nil()),
	)
	file.Line()

	file.Comment("UnmarshalJSON converts the human-readable string to an error code")
	file.Func().Params(Id("c").Op("*").Id("ErrCode")).Id("UnmarshalJSON").Params(Id("b").Index().Byte()).Error().Block(
		Switch(String().Call(Id("b"))).BlockFunc(func(g *Group) {
			for _, err := range errorCodes {
				g.Case(Lit(fmt.Sprintf("\"%s\"", convertIdentifierTo(err.Name, SnakeCase)))).Block(
					Op("*").Id("c").Op("=").Id(ErrPrefix + err.Name),
				)
			}

			g.Default().Block(
				Op("*").Id("c").Op("=").Id(ErrPrefix + "Unknown"),
			)
		}),

		Return(Nil()),
	)
}
