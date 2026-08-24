package main

import (
	"go/ast"
	"go/token"
	"strings"
)

// removeModuleFromManager removes a module registration from module.NewManager(...)
// or NewManager(...). e.g., removes `ica.NewAppModule(...)` from the argument list.
func removeModuleFromManager(file *ast.File, moduleName string) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Handle both module.NewManager and NewManager (plain function call)
		var isNewManager bool
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if sel.Sel.Name == "NewManager" {
				isNewManager = true
			}
		} else if ident, ok := call.Fun.(*ast.Ident); ok {
			if ident.Name == "NewManager" {
				isNewManager = true
			}
		}
		if !isNewManager {
			return true
		}

		var newArgs []ast.Expr
		for _, arg := range call.Args {
			if isModuleRegistration(arg, moduleName) {
				continue // skip this module
			}
			newArgs = append(newArgs, arg)
		}
		call.Args = newArgs
		return false
	})
}

// isModuleRegistration checks if an expression is a module registration like
// `ica.NewAppModule(...)` or `ibc.NewAppModule(...)`.
func isModuleRegistration(expr ast.Expr, moduleName string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	// Check if it's moduleName.NewAppModule or moduleName.NewIBCModule etc.
	if x, ok := sel.X.(*ast.Ident); ok && x.Name == moduleName {
		return true
	}
	return false
}

// removeModuleNameFromOrdering removes a module name from ordering functions like
// SetOrderBeginBlockers, SetOrderEndBlockers, SetOrderPreBlockers.
// It handles both string literals and selector expressions.
func removeModuleNameFromOrdering(file *ast.File, funcName, moduleName string) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != funcName {
			return true
		}

		var newArgs []ast.Expr
		for _, arg := range call.Args {
			if isModuleNameArg(arg, moduleName) {
				continue
			}
			newArgs = append(newArgs, arg)
		}
		call.Args = newArgs
		return false
	})
}

// isModuleNameArg checks if an argument represents the given module name.
// Handles: "ibc-transfer" (string literal), ibctransfertypes.ModuleName (selector), etc.
func isModuleNameArg(arg ast.Expr, moduleName string) bool {
	// String literal: "ibc-transfer"
	if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return strings.Trim(lit.Value, `"`) == moduleName
	}
	// Selector expression: ibctransfertypes.ModuleName
	if sel, ok := arg.(*ast.SelectorExpr); ok {
		if sel.Sel.Name == "ModuleName" {
			if x, ok := sel.X.(*ast.Ident); ok {
				// Map module name to type prefix
				expectedPrefix := moduleToTypePrefix(moduleName)
				return x.Name == expectedPrefix
			}
		}
	}
	return false
}

// moduleToTypePrefix maps module name to its types package prefix.
// e.g., "ibc-transfer" -> "ibctransfertypes"
func moduleToTypePrefix(moduleName string) string {
	switch moduleName {
	case "ibc-transfer":
		return "ibctransfertypes"
	case "ibc":
		return "ibcexported"
	case "ica":
		return "icatypes"
	case "wasm":
		return "wasmtypes"
	case "evidence":
		return "evidencetypes"
	case "feegrant":
		return "feegrant"
	case "authz":
		return "authzkeeper"
	case "upgrade":
		return "upgradetypes"
	case "vesting":
		return "vestingtypes"
	case "mint":
		return "minttypes"
	case "distribution":
		return "distrtypes"
	case "slashing":
		return "slashingtypes"
	case "staking":
		return "stakingtypes"
	case "gov":
		return "govtypes"
	case "bank":
		return "banktypes"
	case "auth":
		return "authtypes"
	case "genutil":
		return "genutiltypes"
	case "consensus":
		return "consensusparamtypes"
	default:
		return moduleName + "types"
	}
}

// removeModuleNameFromSlice removes a module name from slice variables like
// genesisModuleOrder and exportModuleOrder.
func removeModuleNameFromSlice(file *ast.File, varName, moduleName string) {
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		// Look for varName := []string{...} or varName = []string{...}
		if len(assign.Lhs) != 1 {
			return true
		}
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || lhs.Name != varName {
			return true
		}

		if len(assign.Rhs) != 1 {
			return true
		}
		compLit, ok := assign.Rhs[0].(*ast.CompositeLit)
		if !ok {
			return true
		}

		var newElts []ast.Expr
		for _, elt := range compLit.Elts {
			if isModuleNameArg(elt, moduleName) {
				continue
			}
			newElts = append(newElts, elt)
		}
		compLit.Elts = newElts
		return false
	})
}

// removeStoreKeys removes store keys from the NewKVStoreKeys call.
func removeStoreKeys(file *ast.File, keys ...string) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "NewKVStoreKeys" {
			return true
		}

		var newArgs []ast.Expr
		for _, arg := range call.Args {
			if isStoreKeyArg(arg, keys) {
				continue
			}
			newArgs = append(newArgs, arg)
		}
		call.Args = newArgs
		return false
	})
}

// isStoreKeyArg checks if an argument matches one of the store keys to remove.
func isStoreKeyArg(arg ast.Expr, keys []string) bool {
	// Selector expression: ibcexported.StoreKey
	if sel, ok := arg.(*ast.SelectorExpr); ok && sel.Sel.Name == "StoreKey" {
		if x, ok := sel.X.(*ast.Ident); ok {
			for _, k := range keys {
				if x.Name+"."+sel.Sel.Name == k {
					return true
				}
			}
		}
	}
	return false
}

// removeMaccPerm removes a module account permission entry from maccPerms map
// or any map literal containing the module. Handles both string literal keys
// ("ibctransfertypes.ModuleName") and selector expression keys
// (ibctransfertypes.ModuleName).
func removeMaccPerm(file *ast.File, moduleName string) {
	ast.Inspect(file, func(n ast.Node) bool {
		// Look for map literals that might be maccPerms
		compLit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		// Check if it's a map type
		if compLit.Type == nil {
			return true
		}

		var newElts []ast.Expr
		changed := false
		for _, elt := range compLit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				newElts = append(newElts, elt)
				continue
			}
			// Check if key matches moduleName
			keyMatches := false
			// String literal: "ibctransfertypes.ModuleName"
			if lit, ok := kv.Key.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if strings.Trim(lit.Value, `"`) == moduleName {
					keyMatches = true
				}
			}
			// Selector expression: ibctransfertypes.ModuleName
			if sel, ok := kv.Key.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "ModuleName" {
					if x, ok := sel.X.(*ast.Ident); ok {
						if x.Name+".ModuleName" == moduleName {
							keyMatches = true
						}
					}
				}
			}
			if keyMatches {
				changed = true
				continue // skip this entry
			}
			newElts = append(newElts, elt)
		}
		if changed {
			compLit.Elts = newElts
		}
		return true // continue inspecting
	})
}
