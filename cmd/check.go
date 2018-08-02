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
	"os"

	"github.com/clearlinux/diva/bloatcheck"
	"github.com/clearlinux/diva/diva"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/spf13/cobra"
)

var highPrioBundles = map[string]bool{"os-core": true, "os-core-update": true, "c-basic": true, "kernel": true}

type bloatCheckCmdFlags struct {
	printOutput bool
	failCap     float64
	warningCap  float64
}

var bloatFlags bloatCheckCmdFlags

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run various content and metadata checks",
	Long:  `Run various checks against distribution content or metadata`,
}

func checkSize(name *string, sizeDiff, size float64) (float64, bool) {
	if _, ok := highPrioBundles[*name]; ok {
		// High priority bundles cannot increase by more than 10% because they
		// may affect many other bundles and minimal installations -> error out
		sizeChange := size * (bloatFlags.failCap / 100.0)
		if sizeDiff > sizeChange {
			return sizeDiff, true
		}
	}
	// Increase of warning cap needs to be flagged, but not fatal
	sizeChange := size * (bloatFlags.warningCap / 100.0)
	if sizeDiff > sizeChange {
		return sizeDiff, true
	}
	return 0, false
}

func runBloatCheck(r *diva.Results, u diva.UInfo, args []string) error {
	var err error

	// Get the smallest version # passed in if it's not in order
	if len(args) == 2 {
		u.Ver = helpers.Min(args[0], args[1])
	} else {
		u.Ver = args[0]
	}

	err = diva.GetBundleAtTag(conf, allFlags.bundleURL, u.Ver)
	if err != nil {
		return err
	}
	err = diva.FetchUpdate(u)
	if err != nil {
		return err
	}

	fromBundleSizes, err := bloatcheck.GetBundleSize(u, conf.Paths.BundleDefsRepo)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		fmt.Printf("Size information for build %v\n", u.Ver)
		for bundle, size := range fromBundleSizes {
			fmt.Printf("%s: %d\n", bundle, size)
		}
		// exit so we don't try to compare build sizes
		return nil
	}

	// Get the larger of the two if it's out of order
	u.Ver = helpers.Max(args[0], args[1])

	// Need both version of bundle definitions
	err = diva.GetBundleAtTag(conf, allFlags.bundleURL, u.Ver)
	if err != nil {
		return err
	}
	err = diva.FetchUpdate(u)
	if err != nil {
		return err
	}

	toBundleSizes, err := bloatcheck.GetBundleSize(u, conf.Paths.BundleDefsRepo)
	if err != nil {
		return err
	}

	// Iterate using from because to may have new bundles
	var sizeDiff int64
	var desc string
	for bundle, size := range fromBundleSizes {
		if _, ok := toBundleSizes[bundle]; !ok {
			continue
		}
		sizeDiff = toBundleSizes[bundle] - size
		changeCap := bloatFlags.warningCap
		_, ret := checkSize(&bundle, float64(sizeDiff), float64(size))

		percentDiff := (float64(toBundleSizes[bundle]) - float64(fromBundleSizes[bundle])) / float64(fromBundleSizes[bundle]) * 100
		pChange := fmt.Sprintf("%3.2f%%", percentDiff)
		if _, ok := highPrioBundles[bundle]; ok {
			changeCap = bloatFlags.failCap
		}
		desc = fmt.Sprintf("%s size did not change by more than %2.0f%% -> %s", bundle, changeCap, pChange)
		r.Ok(!ret, desc)
	}
	return nil
}

var bloatCheckCmd = &cobra.Command{
	Use:   "bloat [version] <to version>",
	Short: "Check bundle size variation between builds",
	Long: `Check bundle size variation between 2 builds by supplying two
versions (to & from). You can omit the second "to version" to get the size
of every bundle from one build only`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Passing false to the last "recursive" flag because we don't want all manifest from minversion
		u, err := diva.GetUpstreamInfo(conf, allFlags.upstreamURL, allFlags.version, true, false)
		helpers.FailIfErr(err)

		r := diva.NewSuite("bloat check", "check bundle bloat between build versions")

		err = runBloatCheck(r, u, args)
		helpers.FailIfErr(err)

		if r.Failed > 0 {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.AddCommand(bloatCheckCmd)

	bloatCheckCmd.Flags().BoolVarP(&bloatFlags.printOutput, "print", "p", false, "Print out bundles that increased in size")
	bloatCheckCmd.Flags().Float64Var(&bloatFlags.failCap, "max", 10.0, "Set the max % a high priority bundle may increase.")
	bloatCheckCmd.Flags().Float64Var(&bloatFlags.warningCap, "warn", 20.0, "Set the % bundle size change that will emit a warning.")
}
