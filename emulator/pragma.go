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

type PragmaList []Pragma

type Pragma struct {
	Name     string
	Argument string
}

func (l PragmaList) HasPragma(name string) bool {
	for _, p := range l {
		if p.Name == name {
			return true
		}
	}
	return false
}

func (l PragmaList) ArgumentForPragma(name string) string {
	for _, p := range l {
		if p.Name == name {
			return p.Argument
		}
	}
	return ""
}

func ExtractPragmas(code string) (result PragmaList) {
	result = make(PragmaList, 0)
	r := regexp.MustCompile(`#([a-zA-Z]*)(\(\"([^\"]*?)\"\))?`)
	for _, match := range r.FindAllStringSubmatch(code, -1) {
		result = append(result, Pragma{
			Name:     match[1],
			Argument: match[3],
		})
	}
	return result
}
