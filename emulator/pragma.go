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
	"regexp"
)

var (
	PragmaDebug      = "debug"
	PragmaSourceFile = "sourceFile"
)

type PragmaList []Pragma

var pragmaRegexp = regexp.MustCompile(`#([a-zA-Z_]([0-9a-zA-Z_]*)?)(\(\"([^\"]*?)\"\))?`)

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
	result = make(PragmaList, 0)
	for _, match := range pragmaRegexp.FindAllStringSubmatch(code, -1) {
		result = append(result, &BasicPragma{
			name:     match[1],
			argument: match[4],
		})
	}
	return result
}
