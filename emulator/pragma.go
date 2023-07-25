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
