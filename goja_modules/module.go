package goja_modules

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/peekjef72/httpapi_exporter/goja_modules/console"
	"github.com/peekjef72/httpapi_exporter/goja_modules/exporter"

	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja_nodejs/require"
)

type printer struct{}

func (p *printer) Log(msg string) {
	fmt.Printf("INFO: %s\n", msg)
}
func (p *printer) Debug(msg string) {
	fmt.Printf("Debug: %s\n", msg)
}
func (p *printer) Info(msg string) {
	fmt.Printf("INFO: %s\n", msg)
}
func (p *printer) Warn(msg string) {
	fmt.Printf("WARN: %s\n", msg)
}
func (p *printer) Error(msg string) {
	fmt.Printf("ERROR: %s\n", msg)
}

type jsModExporterFunc struct {
	func_map map[string]any
}

func (e *jsModExporterFunc) GetJSFuncMap() map[string]any {
	return e.func_map
}

func getIdentifiersFromExpression(val ast.Expression, identifiers map[string]string, token string) {

	if val != nil {
		switch expr := val.(type) {

		case *ast.AwaitExpression:
			getIdentifiersFromExpression(expr.Argument, identifiers, token)

		case *ast.ArrayLiteral:
			getIdentifiersFromExpressionList(expr.Value, identifiers, token)

		case *ast.ArrayPattern:
			getIdentifiersFromExpressionList(expr.Elements, identifiers, token)
			getIdentifiersFromExpression(expr.Rest, identifiers, token)

		case *ast.AssignExpression:
			getIdentifiersFromExpression(expr.Left, identifiers, token)
			getIdentifiersFromExpression(expr.Right, identifiers, token)

		case *ast.BinaryExpression:
			getIdentifiersFromExpression(expr.Left, identifiers, token)
			getIdentifiersFromExpression(expr.Right, identifiers, token)

		case *ast.BracketExpression:
			getIdentifiersFromExpression(expr.Left, identifiers, token)
			getIdentifiersFromExpression(expr.Member, identifiers, token)

		case *ast.CallExpression:
			getIdentifiersFromExpression(expr.Callee, identifiers, token)

			getIdentifiersFromExpressionList(expr.ArgumentList, identifiers, token)

		case *ast.ConditionalExpression:
			getIdentifiersFromExpression(expr.Test, identifiers, token)
			getIdentifiersFromExpression(expr.Consequent, identifiers, token)
			getIdentifiersFromExpression(expr.Alternate, identifiers, token)

		case *ast.NewExpression:
			getIdentifiersFromExpressionList(expr.ArgumentList, identifiers, token)

		case *ast.Identifier:
			name := expr.Name.String()
			if _, found := identifiers[name]; !found {
				identifiers[name] = token
			}

		case *ast.DotExpression:
			getIdentifiersFromExpression(expr.Left, identifiers, token)

		case *ast.PrivateDotExpression:
			getIdentifiersFromExpression(expr.Left, identifiers, token)

		case *ast.ObjectLiteral:
			for _, prop := range expr.Value {
				getIdentifiersFromExpression(prop, identifiers, token)

			}
		case *ast.PropertyKeyed:
			getIdentifiersFromExpression(expr.Key, identifiers, token)
			getIdentifiersFromExpression(expr.Value, identifiers, token)

		}

	}
}

func getIdentifiersFromExpressionList(list_e []ast.Expression, identifiers map[string]string, token string) {
	for _, expr := range list_e {
		getIdentifiersFromExpression(expr, identifiers, token)
	}
}

func getIdentifierFromStatement(stmt ast.Statement, identifiers map[string]string, token string) {
	switch val := stmt.(type) {

	case *ast.BlockStatement:
		getIdentifierFromStatementList(val.List, identifiers, token)

	case *ast.CaseStatement:
		getIdentifiersFromExpression(val.Test, identifiers, token)
		getIdentifierFromStatementList(val.Consequent, identifiers, token)

	case *ast.CatchStatement:
		getIdentifierFromStatement(val.Body, identifiers, token)

	case *ast.DoWhileStatement:
		getIdentifiersFromExpression(val.Test, identifiers, token)
		getIdentifierFromStatement(val.Body, identifiers, token)

	case *ast.ExpressionStatement:
		getIdentifiersFromExpression(val.Expression, identifiers, token)

	case *ast.ForInStatement:
		getIdentifiersFromExpression(val.Source, identifiers, token)
		getIdentifierFromStatement(val.Body, identifiers, token)

	case *ast.ForOfStatement:
		getIdentifiersFromExpression(val.Source, identifiers, token)
		getIdentifierFromStatement(val.Body, identifiers, token)

	case *ast.ForStatement:
		getIdentifiersFromExpression(val.Update, identifiers, token)
		getIdentifiersFromExpression(val.Test, identifiers, token)
		getIdentifierFromStatement(val.Body, identifiers, token)

	case *ast.IfStatement:
		getIdentifiersFromExpression(val.Test, identifiers, token)
		getIdentifierFromStatement(val.Consequent, identifiers, token)
		if val.Alternate != nil {
			getIdentifierFromStatement(val.Alternate, identifiers, token)
		}

	case *ast.LabelledStatement:
		getIdentifierFromStatement(val.Statement, identifiers, token)

	case *ast.ReturnStatement:
		getIdentifiersFromExpression(val.Argument, identifiers, token)

	case *ast.SwitchStatement:
		getIdentifiersFromExpression(val.Discriminant, identifiers, token)
		// for _, stmt_tmp := range val.Body {
		// 	getIdentifierFromStatement((ast.Statement)(*stmt_tmp))
		// }
	case *ast.ThrowStatement:
		getIdentifiersFromExpression(val.Argument, identifiers, token)

	case *ast.TryStatement:
		getIdentifierFromStatement(val.Body, identifiers, token)
		getIdentifierFromStatement(val.Catch, identifiers, token)
		if val.Finally != nil {
			getIdentifierFromStatement(val.Finally, identifiers, token)
		}
	case *ast.LexicalDeclaration:
		for _, bind := range val.List {
			getIdentifiersFromExpression(bind.Target, identifiers, "const")
			getIdentifiersFromExpression(bind.Initializer, identifiers, token)
		}

	case *ast.VariableStatement:
		for _, bind := range val.List {
			getIdentifiersFromExpression(bind.Target, identifiers, "var")
			getIdentifiersFromExpression(bind.Initializer, identifiers, token)
		}

	case *ast.WhileStatement:
		getIdentifiersFromExpression(val.Test, identifiers, token)
		getIdentifierFromStatement(val.Body, identifiers, token)

	case *ast.WithStatement:
		getIdentifiersFromExpression(val.Object, identifiers, token)
		getIdentifierFromStatement(val.Body, identifiers, token)

	case *ast.FunctionDeclaration:
		getIdentifierFromStatement(val.Function.Body, identifiers, token)
	}
}

func getIdentifierFromStatementList(stmts []ast.Statement, identifiers map[string]string, token string) {
	for _, stmt := range stmts {
		getIdentifierFromStatement(stmt, identifiers, token)
	}
}

// func GetIdentifiers(p *ast.Program) []string {
func GetIdentifiers(p *ast.Program) map[string]string {
	var identifiers = make(map[string]string)
	getIdentifierFromStatementList(p.Body, identifiers, "global")
	for _, decl := range p.DeclarationList {
		for _, var_def := range decl.List {
			if ident := var_def.Target.(*ast.Identifier); ident != nil {
				delete(identifiers, ident.Name.String())

			}
		}
	}
	return identifiers

}

//********************************************************************

// to store info on a js prog
type JSCode struct {
	registry *require.Registry
	prog     *goja.Program
	vm       *goja.Runtime
	logger   *slog.Logger
	idents   map[string]string
	func_map map[string]any
}

func NewJSCode(code string, func_map map[string]any) (*JSCode, error) {
	jscode := &JSCode{}

	ast_prog, err := goja.Parse("field", code)
	if err != nil {
		return nil, fmt.Errorf("parse failure in js code %s: %s", code, err)
	}

	jscode.idents = GetIdentifiers(ast_prog)
	prog, err := goja.CompileAST(ast_prog, false)
	if err != nil {
		return nil, fmt.Errorf("field js code %s is invalid: %s", code, err)
	}
	jscode.prog = prog
	jscode.func_map = func_map

	jscode.initRunTime()

	return jscode, nil
}

func (jscode *JSCode) initRunTime() {
	var print console.Printer
	jscode.vm = goja.New()

	jscode.registry = new(require.Registry)
	jscode.registry.Enable(jscode.vm)
	if jscode.logger == nil {
		print = &printer{}
	} else {
		print = &logger_printer{
			logger: jscode.logger,
		}

	}
	func_val := console.RequireWithPrinter(print)
	jscode.registry.RegisterNativeModule(console.ModuleName, func_val)
	console.Enable(jscode.vm)
	mod := &jsModExporterFunc{
		func_map: jscode.func_map,
	}
	jscode.registry.RegisterNativeModule(exporter.ModuleName, exporter.RequireWithJSModFuncMap(mod))
	exporter.Enable(jscode.vm)
}

type logger_printer struct {
	logger *slog.Logger
}

func (p *logger_printer) Log(msg string) {
	p.logger.Info(msg)
}
func (p *logger_printer) Debug(msg string) {
	p.logger.Debug(msg)
}
func (p *logger_printer) Info(msg string) {
	p.logger.Info(msg)
}
func (p *logger_printer) Warn(msg string) {
	p.logger.Warn(msg)
}
func (p *logger_printer) Error(msg string) {
	p.logger.Error(msg)
}
func (js *JSCode) SetSymbolTable(symtab map[string]any, logger *slog.Logger) {

	// set console.xx to logger.xx function
	if logger != js.logger {
		js.logger = logger
		js.initRunTime()
	}

	// set only values for key required for program and found in symtab
	for key := range js.idents {
		if _, ok := symtab[key]; ok {
			js.vm.Set(key, symtab[key])
		}
	}
}

func (js *JSCode) Run(symtab map[string]any, logger *slog.Logger) (val any, err error) {

	var code_val goja.Value
	js.SetSymbolTable(symtab, logger)

	defer func() {
		// res and err are named out parameters, so if we set value for them in defer
		// set the returned values
		ok := false
		if r := recover(); r != nil {
			if err, ok = r.(error); !ok {
				err = errors.New("panic in GetValueString from js code with undefined error")
			}
			val = ""
		} else {
			// trap goja error message
			if err != nil {
				err_str := err.Error()
				if strings.HasPrefix(err_str, "GoError") {
					err_str = strings.TrimPrefix(err_str, "GoError")
					err = errors.New(err_str)
				}
			}
		}
	}()

	code_val, err = js.vm.RunProgram(js.prog)
	if err != nil {
		return "", err
	}
	val = code_val.Export()

	return
}
