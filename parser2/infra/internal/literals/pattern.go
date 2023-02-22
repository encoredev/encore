package literals

import (
	"fmt"
	"go/ast"
	"go/constant"
	"reflect"

	"github.com/fatih/structtag"

	"encr.dev/parser2/internal/perr"
)

func Decode[T any](errs *perr.List, literal *Struct) T {
	var decodeTo T
	dst := reflect.ValueOf(&decodeTo)
	if dst.Elem().Kind() != reflect.Struct {
		panic("literals.Decode: type argument must be a struct")
	}

	decodedFields := make(map[string]bool)

	dst = dst.Elem()
	for i := 0; i < dst.NumField(); i++ {
		fieldType := dst.Type().Field(i)
		if fieldType.Anonymous {
			panic("literals.Decode: anonymous fields not supported")
		} else if !ast.IsExported(fieldType.Name) {
			panic("literals.Decode: non-exported fields not supported")
		}

		fieldPath := decodeField(errs, literal, fieldType, dst.Field(i))
		decodedFields[fieldPath] = true
	}

	// Make sure all the fields we care about have been decoded and nothing else.
	for _, f := range literal.FieldPaths() {
		if !decodedFields[f] {
			errs.Addf(literal.Pos(f), "unexpected field: %s", f)
		}
	}

	return decodeTo
}

// decodeField decodes a single constant literal field.
// It reports the field path it decoded.
func decodeField(errs *perr.List, literal *Struct, fieldType reflect.StructField, field reflect.Value) (fieldPath string) {
	// Determine the path we want to find the field at
	fieldPath = fieldType.Name
	required := false
	dynamicOK := false
	zeroValueOK := false

	tag, err := structtag.Parse(string(fieldType.Tag))
	if err != nil {
		panic(fmt.Sprintf("literals.Decode: invalid tag on field %s: %v", fieldType.Name, err))
	}
	if tagOpts, err := tag.Get("literal"); err == nil {
		if tagOpts.Name != "" {
			fieldPath = tagOpts.Name
		}
		required = !tagOpts.HasOption("optional")
		dynamicOK = tagOpts.HasOption("dynamic")
		zeroValueOK = tagOpts.HasOption("zero-ok")
	}

	// If the field is required and isn't set, return an error.
	if isSet := literal.IsSet(fieldPath); !isSet {
		if required {
			errs.Addf(literal.Pos(fieldPath), "missing required field: %s", fieldPath)
			return
		} else {
			// Nothing to do
			return
		}
	}

	// If the field is not dynamic and we don't allow dynamic fields, return an error.
	isDynamic := !literal.IsConstant(fieldPath)
	if isDynamic && !dynamicOK {
		errs.Addf(literal.Pos(fieldPath), "field %s must be a constant literal", fieldPath)
		return
	} else if isDynamic {
		// Make sure the type is Expr.
		if field.Type().PkgPath() != "go/ast" || field.Type().Name() != "Expr" {
			panic(fmt.Sprintf("literals.Decode: dynamic field %s must be of type ast.Expr", fieldType.Name))
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
			errs.Addf(literal.Pos(fieldPath), "field %s must be a string literal", fieldPath)
		}

	case reflect.Int, reflect.Uint:
		if val.Kind() == constant.Int {
			n, _ := constant.Int64Val(val)
			field.SetInt(n)
		} else {
			errs.Addf(literal.Pos(fieldPath), "field %s must be an integer literal", fieldPath)
		}

	case reflect.Bool:
		if val.Kind() == constant.Bool {
			field.SetBool(constant.BoolVal(val))
		} else {
			errs.Addf(literal.Pos(fieldPath), "field %s must be a boolean literal", fieldPath)
		}

	default:
		panic(fmt.Sprintf("literals.Decode: unsupported field type: %s", fieldType.Type.Kind()))
	}

	// Now that we've set the value, if the field is required make sure it's not the zero value.
	if required && !zeroValueOK && field.IsZero() {
		errs.Addf(literal.Pos(fieldPath), "field %s must not be the zero value", fieldPath)
	}

	return
}
