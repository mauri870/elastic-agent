// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eql

//go:generate antlr4 -Dlanguage=Go -o parser Eql.g4 -visitor

// Eval takes an expression, parse and evaluate it, everytime this method is called a new
// parser is created, if you want to reuse the parsed tree see the `New` method.
// If allowMissingVars is true, then variables not found in the VarStore will
// evaluate to Null. Otherwise, they will produce an error.
// Evaluation does not use logical short circuiting: for example,
// the expression "${validVariable} or ${invalidVariable}" will generate
// an error even if ${validVariable} is true.
func Eval(expression string, store VarStore, allowMissingVars bool) (bool, error) {
	e, err := New(expression)
	if err != nil {
		return false, err
	}
	return e.Eval(store, allowMissingVars)
}
