package literals

import (
	"go/ast"
	"go/constant"
	"reflect"

	"github.com/fatih/structtag"

	"encr.dev/v2/internals/perr"
)

func Decode[T any](errs *perr.List, literal *Struct, defaultValues *T) T {
	var decodeTo T
	dst := reflect.ValueOf(&decodeTo)
	if dst.Elem().Kind() != reflect.Struct {
		errs.Assert(errArgumentMustBeStruct.AtGoNode(literal.ast))
	}
	dst = dst.Elem()

	var defaultValue reflect.Value
	if defaultValues != nil {
		defaultValue = reflect.ValueOf(defaultValues).Elem()
	}

	fieldPaths := decodeStruct(errs, literal, dst, defaultValue)

	decodedFields := make(map[string]bool)
	for _, fp := range fieldPaths {
		decodedFields[fp] = true
	}

	// Make sure all the fields we care about have been decoded and nothing else.
	for _, f := range literal.FieldPaths() {
		if !decodedFields[f] {
			errs.Add(errUnexpectedField.AtGoNode(literal.Expr(f)))
		}
	}

	return decodeTo
}

func decodeStruct(errs *perr.List, literal *Struct, dst reflect.Value, defaultValues reflect.Value) (fieldPaths []string) {
	for i := 0; i < dst.NumField(); i++ {
		fieldType := dst.Type().Field(i)
		if fieldType.Anonymous {
			errs.Assert(errAnonymousFieldsNotSupported)
		} else if !ast.IsExported(fieldType.Name) {
			errs.Assert(errUnexportedFieldsNotSupported)
		}

		var fieldDefault reflect.Value
		if defaultValues.IsValid() {
			fieldDefault = defaultValues.Field(i)
		}

		paths := decodeField(errs, literal, fieldType, dst.Field(i), fieldDefault)
		fieldPaths = append(fieldPaths, paths...)
	}
	return fieldPaths
}

// decodeField decodes a single constant literal field.
// It reports the field path it decoded.
func decodeField(errs *perr.List, literal *Struct, fieldType reflect.StructField, field reflect.Value, defaultField reflect.Value) (fieldPaths []string) {
	// Determine the path we want to find the field at
	fieldPath := fieldType.Name

	required := false
	dynamicOK := false
	zeroValueOK := false
	useDefaultValue := false

	tag, err := structtag.Parse(string(fieldType.Tag))
	if err != nil {
		errs.Assert(errInvalidTag.AtGoNode(literal.Expr(fieldPath)).Wrapping(err))
	}
	if tagOpts, err := tag.Get("literal"); err == nil {
		if tagOpts.Name != "" {
			fieldPath = tagOpts.Name
		}
		required = !tagOpts.HasOption("optional")
		dynamicOK = tagOpts.HasOption("dynamic")
		zeroValueOK = tagOpts.HasOption("zero-ok")
		useDefaultValue = tagOpts.HasOption("default")
	}

	fieldPaths = []string{fieldPath}

	// If the field is required and isn't set, return an error.
	if isSet := literal.IsSet(fieldPath); !isSet {
		if required {
			pos := literal.Pos(fieldPath)
			errs.Add(errMissingRequiredField(fieldPath).AtGoPos(pos, pos))
			return
		} else {
			if useDefaultValue && defaultField.IsValid() {
				// If the field is optional and we have a default value, use it.
				field.Set(defaultField)
			}
			// Nothing to do
			return
		}
	}

	// If the field is not dynamic and we don't allow dynamic fields, return an error.
	isDynamic := !literal.IsConstant(fieldPath)
	if isDynamic && !dynamicOK {
		errs.Add(errIsntConstant(fieldPath).AtGoNode(literal.Expr(fieldPath)))
		return
	} else if isDynamic {
		// Make sure the type is Expr.
		if field.Type().PkgPath() != "go/ast" || field.Type().Name() != "Expr" {
			errs.Assert(errDyanmicFieldNotExpr.AtGoNode(literal.Expr(fieldPath)))
		}
		field.Set(reflect.ValueOf(literal.Expr(fieldPath)))
		return
	}

	val := literal.ConstantValue(fieldPath)
	switch fieldType.Type.Kind() {
	case reflect.String:
		if val.Kind() == constant.String {
			field.SetString(constant.StringVal(val))
		} else {
			errs.Add(errWrongDynamicType(fieldPath, "string").AtGoNode(literal.Expr(fieldPath)))
		}

	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64:
		if val.Kind() == constant.Int {
			n, _ := constant.Int64Val(val)
			field.SetInt(n)
		} else {
			errs.Add(errWrongDynamicType(fieldPath, "integer").AtGoNode(literal.Expr(fieldPath)))
		}

	case reflect.Bool:
		if val.Kind() == constant.Bool {
			field.SetBool(constant.BoolVal(val))
		} else {
			errs.Add(errWrongDynamicType(fieldPath, "boolean").AtGoNode(literal.Expr(fieldPath)))
		}

	case reflect.Struct:
		child, ok := literal.ChildStruct(fieldPath)
		if !ok {
			errs.Add(errWrongDynamicType(fieldPath, "inline struct").AtGoNode(literal.Expr(fieldPath)))
			return
		}
		childPaths := decodeStruct(errs, child, field, defaultField)
		for _, p := range childPaths {
			fieldPaths = append(fieldPaths, fieldPath+"."+p)
		}
		return fieldPaths

	default:
		errs.Assert(errUnsupportedType(fieldType.Type.Kind()).AtGoNode(literal.Expr(fieldPath)))
	}

	// Now that we've set the value, if the field is required make sure it's not the zero value.
	if required && !zeroValueOK && field.IsZero() {
		errs.Add(errZeroValue(fieldPath).AtGoNode(literal.Expr(fieldPath)))
	}

	return fieldPaths
}
