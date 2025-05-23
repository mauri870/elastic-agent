// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eql

import (
	"fmt"
	"os"
	"testing"

	"github.com/antlr4-go/antlr/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-agent/internal/pkg/eql/parser"
)

var showDebug = lookupEnvOrDefault("DEBUG", "0")

type testVarStore struct {
	vars map[string]interface{}
}

func (s *testVarStore) Lookup(v string) (interface{}, bool) {
	val, ok := s.vars[v]
	return val, ok
}

func TestEqlNewReportsBadSyntax(t *testing.T) {
	// Some malformed antlr expressions can produce an error when evaluated
	// because they cause a nil pointer reference or similar unhelpful
	// error. These test cases confirm that eql.New reports these errors
	// during the initial parsing of the expression, so things don't get
	// that far.
	testCases := []string{
		"asdf",
		"${***}",
		"${",
		"{}{}{}",
		"1+=2",
		"1.23f == ''",
		"${asdf}...",
		"${...}",
		"${a.b.}",
		"${a..b}",
	}
	for _, expression := range testCases {
		_, err := New(expression)
		assert.Error(t, err, "malformed EQL expression \"%v\" should produce an error", expression)
	}
}

func TestEql(t *testing.T) {
	testcases := []struct {
		expression       string
		allowMissingVars bool
		result           bool
		err              bool
	}{
		// escaped variables (not allowed)
		{expression: "$${env.HOSTNAME|host.name|'fallback'}", err: true},
		{expression: "$${env.HOSTNAME}", err: true},

		// variables
		{expression: "${env.HOSTNAME|host.name|'fallback'} == 'my-hostname'", result: true},
		{expression: "${env.MISSING|host.name|'fallback'} == 'host-name'", result: true},
		{expression: "${env.MISSING|host.MISSING|'fallback'} == 'fallback'", result: true},
		{expression: "${env.MISSING|host.MISSING|2} == 2", result: true},
		{expression: "${env.MISSING|host.MISSING|2.0} == 2.0", result: true},
		{expression: "${env.MISSING|host.MISSING|true} == true", result: true},
		{expression: "${env.MISSING|host.MISSING|false} == false", result: true},
		{expression: "${env.MISSING|host.MISSING}", err: true},
		{expression: "${'constant'} == 'constant'", result: true},
		{expression: "${data.with-dash} == 'dash-value'", result: true},
		{expression: "${'dash-value'} == 'dash-value'", result: true},
		{expression: "${data.with/slash} == 'some/path'", result: true},

		// constant of variables
		{expression: "'${env.HOSTNAME}' == '${env.HOSTNAME}'", result: true},
		{expression: "'${env.HOSTNAME}' == '${env.HOSTSAME}'", result: false},
		{expression: "'$${env.HOSTNAME}' == '$${env.HOSTNAME}'", result: true},
		{expression: "'$${env.HOSTNAME}' == '$${env.HOSTSAME}'", result: false},

		// boolean
		{expression: "true", result: true},
		{expression: "false", result: false},

		// equal
		{expression: "'hello' == 'hello'", result: true},
		{expression: "'hello' == 'other'", result: false},
		{expression: "'other' == 'hello'", result: false},
		{expression: "1 == 1", result: true},
		{expression: "1 == 2", result: false},
		{expression: "2 == 1", result: false},
		{expression: "1.0 == 1", result: true},
		{expression: "1.1 == 1", result: false},
		{expression: "1 == 1.1", result: false},
		{expression: "true == true", result: true},
		{expression: "true == false", result: false},
		{expression: "false == false", result: true},
		{expression: "true == false", result: false},
		{expression: "${missing} == ${missing}", allowMissingVars: true, result: true},
		{expression: "${missing} == false", allowMissingVars: true, result: false},
		{expression: "false == ${missing}", allowMissingVars: true, result: false},
		{expression: "${missing} == ${missing}", err: true},
		{expression: "${missing} == false", err: true},
		{expression: "false == ${missing}", err: true},

		// not equal
		{expression: "'hello' != 'hello'", result: false},
		{expression: "'hello' != 'other'", result: true},
		{expression: "'other' != 'hello'", result: true},
		{expression: "1 != 1", result: false},
		{expression: "1 != 2", result: true},
		{expression: "2 != 1", result: true},
		{expression: "1.0 != 1", result: false},
		{expression: "1.1 != 1", result: true},
		{expression: "1 != 1.1", result: true},
		{expression: "true != true", result: false},
		{expression: "true != false", result: true},
		{expression: "false != false", result: false},
		{expression: "true != false", result: true},
		{expression: "${missing} != ${missing}", allowMissingVars: true, result: false},
		{expression: "${missing} != false", allowMissingVars: true, result: true},
		{expression: "false != ${missing}", allowMissingVars: true, result: true},
		{expression: "${missing} != ${missing}", err: true},
		{expression: "${missing} != false", err: true},
		{expression: "false != ${missing}", err: true},

		// gt
		{expression: "1 > 5", result: false},
		{expression: "10 > 5", result: true},
		{expression: "10 > 10", result: false},
		{expression: "1.1 > 5", result: false},
		{expression: "10.1 > 5", result: true},
		{expression: "1 > 5.0", result: false},
		{expression: "10 > 5.0", result: true},
		{expression: "10.1 > 10.1", result: false},

		// lt
		{expression: "1 < 5", result: true},
		{expression: "10 < 5", result: false},
		{expression: "10 < 10", result: false},
		{expression: "1.1 < 5", result: true},
		{expression: "10.1 < 5", result: false},
		{expression: "1 < 5.0", result: true},
		{expression: "10 < 5.0", result: false},
		{expression: "10.1 < 10.1", result: false},

		// gte
		{expression: "1 >= 5", result: false},
		{expression: "10 >= 5", result: true},
		{expression: "10 >= 10", result: true},
		{expression: "1.1 >= 5", result: false},
		{expression: "10.1 >= 5", result: true},
		{expression: "1 >= 5.0", result: false},
		{expression: "10 >= 5.0", result: true},
		{expression: "10.1 >= 10.1", result: true},

		// lte
		{expression: "1 <= 5", result: true},
		{expression: "10 <= 5", result: false},
		{expression: "10 <= 10", result: true},
		{expression: "1.1 <= 5", result: true},
		{expression: "10.1 <= 5", result: false},
		{expression: "1 <= 5.0", result: true},
		{expression: "10 <= 5.0", result: false},
		{expression: "10.1 <= 10.1", result: true},

		// math (pemdas)
		{expression: "4 * (5 + 3) == 32", result: true},
		{expression: "4 * 5 + 3 == 23", result: true},
		{expression: "2 + 5 * 3 == 17", result: true},
		{expression: "(2 + 5) * 3 == 21", result: true},
		{expression: "30 / 5 * 3 == 18", result: true},
		{expression: "30 / (5 * 3) == 2", result: true},
		{expression: "(18 / 6 * 5) - 14 / 7 == 13", result: true},
		{expression: "(18 / 6 * 5) - 14 / 7 == 13", result: true},
		{expression: "1.0 / 2 * 6 == 3", result: true},
		{expression: "24.0 / (-2 * -6) == 2", result: true},
		{expression: "24.0 / 0 == 0", err: true},
		{expression: "-4 * (5 + 3) == -32", result: true},
		{expression: "-4 * 5 + 3 == -17", result: true},
		{expression: "-24.0 / (2 * 6) == -2", result: true},
		{expression: "-24.0 / (5 % 3) == -12", result: true},
		{expression: "-24 % 5 * 3 == -12", result: true},

		// not
		{expression: "not false", result: true},
		{expression: "not true", result: false},
		{expression: "not (1 == 1)", result: false},
		{expression: "not (1 != 1)", result: true},
		{expression: "NOT false", result: true},
		{expression: "NOT true", result: false},
		{expression: "NOT (1 == 1)", result: false},
		{expression: "NOT (1 != 1)", result: true},

		// and
		{expression: "(1 == 1) and (2 == 2)", result: true},
		{expression: "(1 == 4) and (2 == 2)", result: false},
		{expression: "(1 == 1) and (2 == 3)", result: false},
		{expression: "(1 == 5) and (2 == 3)", result: false},
		{expression: "(1 == 1) AND (2 == 2)", result: true},
		{expression: "(1 == 4) AND (2 == 2)", result: false},
		{expression: "(1 == 1) AND (2 == 3)", result: false},
		{expression: "(1 == 5) AND (2 == 3)", result: false},
		{expression: "1 == 1 AND 2 == 2", result: true},
		{expression: "1 == 4 AND 2 == 2", result: false},
		{expression: "1 == 1 AND 2 == 3", result: false},
		{expression: "1 == 5 AND 2 == 3", result: false},
		{expression: "1 == 1 and 2 == 2", result: true},
		{expression: "1 == 4 and 2 == 2", result: false},
		{expression: "1 == 1 and 2 == 3", result: false},
		{expression: "1 == 5 and 2 == 3", result: false},

		// or
		{expression: "(1 == 1) OR (2 == 2)", result: true},
		{expression: "(1 == 1) OR (3 == 2)", result: true},
		{expression: "(1 == 2) OR (2 == 2)", result: true},
		{expression: "(1 == 2) OR (2 == 2)", result: true},
		{expression: "(1 == 2) OR (1 == 2)", result: false},
		{expression: "(1 == 1) or (2 == 2)", result: true},
		{expression: "(1 == 1) or (3 == 2)", result: true},
		{expression: "(1 == 2) or (2 == 2)", result: true},
		{expression: "(1 == 2) or (2 == 2)", result: true},
		{expression: "(1 == 2) or (1 == 2)", result: false},

		// mixed
		{expression: "((1 == 1) AND (2 == 2)) OR (2 != 3)", result: true},
		{expression: "(1 == 1 OR 2 == 2) AND 2 != 3", result: true},
		{expression: "((1 == 1) AND (2 == 2)) OR (2 != 3)", result: true},
		{expression: "1 == 1 OR 2 == 2 AND 2 != 3", result: true},

		// evaluation doesn't use logical short-circuits
		{expression: "${host.name} == 'asdf'", result: false},
		{expression: "${host.name} == 'asdf' AND ${missing} == 'qwer'", err: true},
		{expression: "${host.name} == 'asdf' AND ${missing} == 'qwer'", allowMissingVars: true, result: false},
		{expression: "${host.name} == 'host-name'", result: true},
		{expression: "${host.name} == 'host-name' OR ${missing} == 'qwer'", err: true},
		{expression: "${host.name} == 'host-name' OR ${missing} == 'qwer'", allowMissingVars: true, result: true},

		// arrays
		{expression: "[true, false, 1, 1.0, 'test'] == [true, false, 1, 1.0, 'test']", result: true},
		{expression: "[true, false, 1, 1.0, 'test'] == [true, false, 1, 1.1, 'test']", result: false},
		{expression: "[true, false, 1, 1.0, 'test'] != [true, false, 1, 1.0, 'test']", result: false},
		{expression: "[true, false, 1, 1.0, 'test'] != [true, false, 1, 1.1, 'test']", result: true},

		// dict
		{expression: `{bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "test"} == {bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "test"}`, result: true},
		{expression: `{bt: true, bf: false, number: 1, float: 1.0, st: 'test'} == {bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "test"}`, result: false},
		{expression: `{bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "other"} == {bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "test"}`, result: false},
		{expression: `{bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt2: "test"} == {bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "test"}`, result: false},
		{expression: `{bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "test"} != {bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "test"}`, result: false},
		{expression: `{bt: true, bf: false, number: 1, float: 1.0, st: 'test'} != {bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "test"}`, result: true},
		{expression: `{bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "other"} != {bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "test"}`, result: true},
		{expression: `{bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt2: "test"} != {bt: true, bf: false, number: 1, float: 1.0, st: 'test', dt: "test"}`, result: true},

		// methods array
		{expression: "arrayContains([true, 1, 3.5, 'str'], 1)", result: true},
		{expression: "arrayContains([true, 1, 3.5, 'str'], 2)", result: false},
		{expression: "arrayContains([true, 1, 3.5, 'str'], 'str')", result: true},
		{expression: "arrayContains([true, 1, 3.5, 'str'], 'str2')", result: false},
		{expression: "arrayContains([true, 1, 3.5, 'str'], 'str2', 3.5)", result: true},
		{expression: "arrayContains(${null.data}, 'str2', 3.5)", allowMissingVars: true, result: false},
		{expression: "arrayContains(${null.data}, 'str2', 3.5)", err: true},
		{expression: "arrayContains(${data.array}, 'array5', 'array2')", result: true},
		{expression: "arrayContains('not array', 'str2')", err: true},

		// methods dict
		{expression: "hasKey({key1: 'val1', key2: 'val2'}, 'key2')", result: true},
		{expression: "hasKey({key1: 'val1', key2: 'val2'}, 'other', 'key1')", result: true},
		{expression: "hasKey({key1: 'val1', key2: 'val2'}, 'missing', 'still')", result: false},
		{expression: "hasKey(${data.dict}, 'key3', 'still')", result: true},
		{expression: "hasKey(${null}, 'key3', 'still')", allowMissingVars: true, result: false},
		{expression: "hasKey(${null}, 'key3', 'still')", err: true},
		{expression: "hasKey(${data.dict})", err: true},
		{expression: "hasKey(${data.array}, 'not present')", err: true},

		// methods length
		{expression: "length('hello') == 5", result: true},
		{expression: "length([true, 1, 3.5, 'str']) == 4", result: true},
		{expression: "length({key: 'data', other: '2'}) == 2", result: true},
		{expression: "length(${data.dict}) == 3", result: true},
		{expression: "length(${null}) == 0", allowMissingVars: true, result: true},
		{expression: "length(${null}) == 0", err: true},
		{expression: "length(4) == 2", err: true},
		{expression: "length('hello', 'too many args') == 2", err: true},

		// methods math
		{expression: "add(2, 2) == 4", result: true},
		{expression: "add(2.2, 2.2) == 4.4", result: true},
		{expression: "add(2) == 4", err: true},
		{expression: "add(2, 2, 2) == 4", err: true},
		{expression: "add('str', 'str') == 4", err: true},
		{expression: "subtract(2, 2) == 0", result: true},
		{expression: "subtract(2.2, 2.2) == 0", result: true},
		{expression: "subtract(2) == 0", err: true},
		{expression: "subtract(2, 2, 2) == 0", err: true},
		{expression: "subtract('str', 'str') == 0", err: true},
		{expression: "multiply(4, 2) == 8", result: true},
		{expression: "multiply(4.2, 2) == 8.4", result: true},
		{expression: "multiply(4) == 4", err: true},
		{expression: "multiply(2, 2, 2) == 4", err: true},
		{expression: "multiply('str', 'str') == 4", err: true},
		{expression: "divide(8, 2) == 4", result: true},
		{expression: "divide(4.2, 2) == 2.1", result: true},
		{expression: "divide(4.2, 0) == 2.1", err: true},
		{expression: "divide(4) == 4", err: true},
		{expression: "divide(2, 2, 2) == 4", err: true},
		{expression: "divide('str', 'str') == 4", err: true},
		{expression: "modulo(8, 3) == 2", result: true},
		{expression: "modulo(8, 0) == 2", err: true},
		{expression: "modulo(4.2, 2) == 1.2", err: true},
		{expression: "modulo(4) == 4", err: true},
		{expression: "modulo(2, 2, 2) == 4", err: true},
		{expression: "modulo('str', 'str') == 4", err: true},

		// methods str
		{expression: "concat('hello ', 2, ' the world') == 'hello 2 the world'", result: true},
		{expression: "concat('h', 2, 2.0, ['a', 'b'], true, {key: 'value'}) == 'h22E+00[a,b]true{key:value}'", result: true},
		{expression: "endsWith('hello world', 'world')", result: true},
		{expression: "endsWith('hello world', 'wor')", result: false},
		{expression: "endsWith('hello world', 'world', 'too many args')", err: true},
		{expression: "endsWith('not enough')", err: true},
		{expression: "indexOf('elastic.co', '.') == 7", result: true},
		{expression: "indexOf('elastic-agent.elastic.co', '.', 15) == 21", result: true},
		{expression: "indexOf('elastic-agent.elastic.co', '.', 15.2) == 21", err: true},
		{expression: "indexOf('elastic-agent.elastic.co', '.', 'not int') == 21", err: true},
		{expression: "indexOf('elastic-agent.elastic.co', '.', '15, 'too many args') == 21", err: true},
		{expression: "match('elastic.co', '[a-z]+.[a-z]{2}')", result: true},
		{expression: "match('elastic.co', '[a-z]+', '[a-z]+.[a-z]{2}')", result: true},
		{expression: "match('not enough')", err: true},
		{expression: "match('elastic.co', '[a-z')", err: true},
		{expression: "number('002020') == 2020", result: true},
		{expression: "number('0xbeef', 16) == 48879", result: true},
		{expression: "number('not a number') == 'not'", err: true},
		{expression: "number('0xbeef', 16, 2) == 'too many args'", err: true},
		{expression: "startsWith('hello world', 'hello')", result: true},
		{expression: "startsWith('hello world', 'llo')", result: false},
		{expression: "startsWith('hello world', 'hello', 'too many args')", err: true},
		{expression: "startsWith('not enough')", err: true},
		{expression: "string('str') == 'str'", result: true},
		{expression: "string(2) == '2'", result: true},
		{expression: "string(2.0) == '2E+00'", result: true},
		{expression: "string(true) == 'true'", result: true},
		{expression: "string(false) == 'false'", result: true},
		{expression: "string(['a', 'b']) == '[a,b]'", result: true},
		{expression: "string({key:'value'}) == '{key:value}'", result: true},
		{expression: "string(2, 'too many') == '2'", err: true},
		{expression: "stringContains('hello world', 'o w')", result: true},
		{expression: "stringContains('hello world', 'rol')", result: false},
		{expression: "stringContains('hello world', 'o w', 'too many')", err: true},
		{expression: "stringContains(0, 'o w', 'too many')", err: true},
		{expression: "stringContains('hello world', 0)", result: false},

		// Bad expression and malformed expression
		{expression: "length('hello')", err: true},
		{expression: "length()", err: true},
		{expression: "donotexist()", err: true},
		{expression: "${***} != ${~~~}", err: true},
		{expression: "false asdf!@#$", err: true},
		{expression: "length('something' 345) > 1000", err: true},
	}

	store := &testVarStore{
		vars: map[string]interface{}{
			"env.HOSTNAME":    "my-hostname",
			"env.HOSTSAME":    "my-hostname",
			"host.name":       "host-name",
			"data.array":      []interface{}{"array1", "array2", "array3"},
			"data.with-dash":  "dash-value",
			"data.with/slash": "some/path",
			"data.dict": map[string]interface{}{
				"key1": "dict1",
				"key2": "dict2",
				"key3": "dict3",
			},
		},
	}

	for _, test := range testcases {
		var title string
		if test.err {
			title = fmt.Sprintf("%s failed parsing", test.expression)
		} else {
			title = fmt.Sprintf("%s => return %v", test.expression, test.result)
		}
		t.Run(title, func(t *testing.T) {
			if showDebug == "1" {
				debug(t, test.expression)
			}

			r, err := Eval(test.expression, store, test.allowMissingVars)

			if test.err {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.result, r)
		})
	}
}

func debug(t *testing.T, expression string) {
	raw := antlr.NewInputStream(expression)

	lexer := parser.NewEqlLexer(raw)
	for {
		token := lexer.NextToken()
		if token.GetTokenType() == antlr.TokenEOF {
			break
		}
		t.Logf("%s (%q)\n",
			lexer.SymbolicNames[token.GetTokenType()], token.GetText())
	}
}

var result bool

func BenchmarkEval(b *testing.B) {
	expression, _ := New("(length('hello') == 5) AND (length('Hi') == 2)")

	var r bool
	for n := 0; n < b.N; n++ {
		r, _ = expression.Eval(nil, true)
	}
	result = r
}

func lookupEnvOrDefault(name, d string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return d
}
