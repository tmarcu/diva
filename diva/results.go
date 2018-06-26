// Copyright © 2018 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package diva

import (
	"encoding/json"
	"io"
	"text/template"
)

// PFS describes the pass/fail/skip status of an individual test result
type PFS uint8

// Valid values for PFS
const (
	Pass PFS = iota
	Fail
	Skip
)

// Result holds individual test status and information
type Result struct {
	Name        string
	Description string
	Status      PFS
	Output      string
}

// Results holds the results of a test run
type Results struct {
	Name        string
	Description string
	Passed      uint
	Failed      uint
	Skipped     uint
	Total       uint
	Tests       []Result
}

const resultTemplate = `
Test Suite:  {{.Name}}
             {{.Description}}
Total:       {{.Total}}
Passed:      {{.Passed}}
Failed:      {{.Failed}}
Skipped:     {{.Skipped}}

{{range $f := .Tests}}{{if eq .Status 1}}{{.Name}}	{{.Description}}	{{.Output}}
{{end}}{{end}}
`

// Print the Results to the Writer provided using the resultTemplate as the
// format.
func (r *Results) Print(w io.Writer) error {
	t, err := template.New("resultTemplate").Parse(resultTemplate)
	if err != nil {
		return err
	}

	return t.Execute(w, r)
}

// PrintJSON prints the Results in JSON format to the Writer provided.
func (r *Results) PrintJSON(w io.Writer) error {
	resOut, err := json.Marshal(r)
	if err != nil {
		return err
	}

	_, err = io.WriteString(w, string(resOut))
	return err
}

// Add increments the appropriate counters according to err and skipped
// arguments and adds a test the result.Tests field.
func (r *Results) Add(name, description string, err error, skipped bool) {
	r.Total++
	var status PFS
	var output string
	switch {
	case skipped:
		r.Skipped++
		status = Skip
		output = "skipped: "
		if err != nil {
			output += err.Error()
		}
	case err == nil:
		r.Passed++
		status = Pass
		output = ""
	default: // err != nil
		r.Failed++
		status = Fail
		output = err.Error()
	}

	t := Result{
		Name:        name,
		Description: description,
		Status:      status,
		Output:      output,
	}
	r.Tests = append(r.Tests, t)
}
