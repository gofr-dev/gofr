package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

type ParsedInterface struct {
	InterfaceName string
	Methods       []ParsedMethod
}

type ParsedMethod struct {
	Name        string
	InputParams []ParsedParam
	ReturnTypes []ParsedType
}

type ParsedParam struct {
	Name string
	Type string
}

type ParsedType struct {
	Name   string
	Fields []ParsedField
}

type ParsedField struct {
	Name string
	Type string
}

func ParseInterface(file string) (*ParsedInterface, error) {
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	interfaceDecl, interfaceSpec := findInterfaceDecl(astFile)
	if interfaceDecl == nil {
		return nil, fmt.Errorf("no interface declaration found in file %s", file)
	}

	parsedInterface := &ParsedInterface{
		InterfaceName: interfaceSpec.Name.Name,
		Methods:       make([]ParsedMethod, 0),
	}

	for _, method := range interfaceDecl.Methods.List {
		parsedMethod, err := parseMethod(method)
		if err != nil {
			return nil, err
		}
		parsedInterface.Methods = append(parsedInterface.Methods, *parsedMethod)
	}

	return parsedInterface, nil
}

func findInterfaceDecl(astFile *ast.File) (*ast.InterfaceType, *ast.TypeSpec) {
	for _, decl := range astFile.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
						return interfaceType, typeSpec
					}
				}
			}
		}
	}
	return nil, nil
}

func parseMethod(method *ast.Field) (*ParsedMethod, error) {
	parsedMethod := &ParsedMethod{
		Name: method.Names[0].Name,
	}

	for _, param := range method.Type.(*ast.FuncType).Params.List {
		parsedParam, err := parseParam(param)
		if err != nil {
			return nil, err
		}
		parsedMethod.InputParams = append(parsedMethod.InputParams, *parsedParam)
	}

	for _, result := range method.Type.(*ast.FuncType).Results.List {
		parsedType, err := parseType(result.Type)
		if err != nil {
			return nil, err
		}
		parsedMethod.ReturnTypes = append(parsedMethod.ReturnTypes, *parsedType)
	}

	return parsedMethod, nil
}

func parseParam(param *ast.Field) (*ParsedParam, error) {
	parsedParam := &ParsedParam{}

	// Handle unnamed parameters
	if len(param.Names) > 0 {
		parsedParam.Name = param.Names[0].Name
	}

	parsedParam.Type = parseExpr(param.Type)

	return parsedParam, nil
}

func parseType(expr ast.Expr) (*ParsedType, error) {
	parsedType := &ParsedType{
		Name: parseExpr(expr),
	}

	if structType, ok := expr.(*ast.StructType); ok {
		parsedType.Name = "struct"
		for _, field := range structType.Fields.List {
			parsedField, err := parseField(field)
			if err != nil {
				return nil, err
			}
			parsedType.Fields = append(parsedType.Fields, *parsedField)
		}
	}

	return parsedType, nil
}

func parseField(field *ast.Field) (*ParsedField, error) {
	parsedField := &ParsedField{}

	// Handle unnamed fields
	if len(field.Names) > 0 {
		parsedField.Name = field.Names[0].Name
	}

	parsedField.Type = parseExpr(field.Type)

	return parsedField, nil
}

func parseExpr(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		return "*" + parseExpr(expr.X)
	case *ast.ArrayType:
		return "[]" + parseExpr(expr.Elt)
	case *ast.MapType:
		return "map[" + parseExpr(expr.Key) + "]" + parseExpr(expr.Value)
	case *ast.SelectorExpr:
		return parseExpr(expr.X) + "." + expr.Sel.Name
	default:
		return fmt.Sprintf("unsupported type: %T", expr)
	}
}
