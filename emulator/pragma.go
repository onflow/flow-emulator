/*
 * Flow Emulator
 *
 * Copyright 2019 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package emulator

import (
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/parser"
)

var (
	PragmaDebug      = "debug"
	PragmaSourceFile = "sourceFile"
)

type PragmaList []Pragma

type Pragma interface {
	Name() string
	Argument() string
}

var _ Pragma = &BasicPragma{}

type BasicPragma struct {
	name     string
	argument string
}

func (p *BasicPragma) Name() string {
	return p.name
}

func (p *BasicPragma) Argument() string {
	return p.argument
}

func (l PragmaList) FilterByName(name string) (result PragmaList) {
	result = PragmaList{}
	for _, p := range l {
		if p.Name() == name {
			result = append(result, p)
		}
	}
	return result
}

func (l PragmaList) First() Pragma {
	if len(l) == 0 {
		return nil
	}
	return l[0]
}

func (l PragmaList) Contains(name string) bool {
	for _, p := range l {
		if p.Name() == name {
			return true
		}
	}
	return false
}

func (l PragmaList) Count(name string) int {
	c := 0
	for _, p := range l {
		if p.Name() == name {
			c = c + 1
		}
	}
	return c
}

func ExtractPragmas(code string) (result PragmaList) {

	program, err := parser.ParseProgram(nil, []byte(code), parser.Config{})
	if err != nil {
		return PragmaList{}
	}

	program.Walk(func(el ast.Element) {
		if el.ElementType() == ast.ElementTypePragmaDeclaration {
			pragmaDeclaration, ok := el.(*ast.PragmaDeclaration)
			if !ok {
				return
			}
			expression, ok := pragmaDeclaration.Expression.(*ast.InvocationExpression)
			if !ok {
				return
			}

			if len(expression.Arguments) > 1 {
				return
			}
			var argument = ""
			if len(expression.Arguments) == 1 {
				stringParameter, ok := expression.Arguments[0].Expression.(*ast.StringExpression)
				if !ok {
					return
				}
				argument = stringParameter.Value
			}

			result = append(result, &BasicPragma{
				name:     expression.InvokedExpression.String(),
				argument: argument,
			})

		}
	})

	return result
}
