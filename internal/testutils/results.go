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

package testutils

import (
	"encoding/json"
	"io"
	"text/template"
)

// Results holds the results of a test run
type Results struct {
	Passed   uint
	Failed   uint
	Skipped  uint
	Total    uint
	Failures []string
}

const resultTemplate = `
Total:    {{.Total}}
Passed:   {{.Passed}}
Failed:   {{.Failed}}
Skipped:  {{.Skipped}}

{{range $f := .Failures}}
{{.f}}
{{end}}
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

// AddFailure increments the Failed and Total counts and appends the error text
// to the Failures slice
func (r *Results) AddFailure(err error) {
	r.Failed++
	r.Total++
	r.Failures = append(r.Failures, err.Error())
}

// AddPassed increments the Passed and Total counts
func (r *Results) AddPassed() {
	r.Passed++
	r.Total++
}

// AddSkipped increments the Skipped and Total counts
func (r *Results) AddSkipped() {
	r.Skipped++
	r.Total++
}
