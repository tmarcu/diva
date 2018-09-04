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

package cmd

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"strings"
	"time"

	rpm "github.com/cavaliercoder/go-rpm"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(rpminfoCmd)
	rpminfoCmd.Flags().StringVarP(&queryFormat, "query", "f", "", "template for query")
}

// rpminfoCmd is the command to call rpminfo
var rpminfoCmd = &cobra.Command{
	Use:   "rpminfo",
	Short: "Print info on rpm packages",
	Long: "Print info on rpm packages using this default template:\n" + defaultQueryFormat +
		"\nSee methods on PackageFile at https://godoc.org/github.com/cavaliercoder/go-rpm#PackageFile\n",
	Run: func(cmd *cobra.Command, args []string) {
		rpminfo(os.Stdout, args)
	},
}

const defaultQueryFormat = `Name        : {{ .Name }}
Version     : {{ .Version }}
Release     : {{ .Release }}
Architecture: {{ .Architecture }}
Group       : {{ .Groups | join }}
Size        : {{ .Size }}
License     : {{ .License }}
Signature   : {{ .GPGSignature }}
Source RPM  : {{ .SourceRPM }}
Build Date  : {{ .BuildTime | timestamp }}
Build Host  : {{ .BuildHost }}
Packager    : {{ .Packager }}
Vendor      : {{ .Vendor }}
URL         : {{ .URL }}
Summary     : {{ .Summary }}
Description :
{{ .Description }}
`

var queryFormat string

func rpminfo(w io.Writer, args []string) {

	if queryFormat == "" {
		queryFormat = defaultQueryFormat
	}
	qf, err := queryformat(queryFormat)
	if err != nil {
		log.Fatal(err)
	}

	// debugging: fmt.Fprintf(w, "Using %v\n", queryFormat)

	for i, path := range args {
		if i > 0 {
			_, _ = fmt.Fprintf(w, "\n")
		}

		p, err := rpm.OpenPackageFile(path)
		if err != nil {
			_, _ = fmt.Fprintf(w, "error reading %s: %v\n", path, err)
			continue
		}

		if err := qf.Execute(w, p); err != nil {
			_, _ = fmt.Fprintf(w, "error formatting %s: %v\n", path, err)
			continue
		}
	}
}

func queryformat(tmpl string) (*template.Template, error) {
	return template.New("queryformat").
		Funcs(template.FuncMap{
			"join": func(a []string) string {
				return strings.Join(a, ", ")
			},
			"timestamp": func(t time.Time) rpm.Time {
				return rpm.Time(t)
			},
		}).
		Parse(tmpl)
}
