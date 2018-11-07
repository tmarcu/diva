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
	"strings"

	"github.com/clearlinux/diva/diva"
	"github.com/clearlinux/diva/internal/config"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/pkginfo"
	"github.com/go-test/deep"

	"github.com/spf13/cobra"
)

type rpmConflictCmdFlags struct {
	mixName string
	version string
	latest  bool
	mash    bool
}

var conflictFlags rpmConflictCmdFlags

func init() {
	checkCmd.AddCommand(rpmFileConflictsCmd)
	rpmFileConflictsCmd.Flags().StringVarP(&conflictFlags.mixName, "name", "n", "clear", "name of data group")
	rpmFileConflictsCmd.Flags().StringVarP(&conflictFlags.version, "version", "v", "0", "version to check")
	rpmFileConflictsCmd.Flags().BoolVar(&conflictFlags.latest, "latest", false, "get the latest version from upstreamURL")
	rpmFileConflictsCmd.Flags().BoolVar(&conflictFlags.mash, "repo", false, "pass repo to use all rpms from repo, and not populate from a bundle version")
}

var rpmFileConflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "check file conflicts for bundle tag",
	Long: `check all packages within bundle tag to see if any two rpms have the
same filename, yet come from different SRPMs`,
	Run: runCheckFileConflicts,
}

func runCheckFileConflicts(cmd *cobra.Command, args []string) {
	u := config.UInfo{
		MixName: conflictFlags.mixName,
		Ver:     conflictFlags.version,
		Latest:  conflictFlags.latest,
	}

	repo, err := pkginfo.NewRepo(conf, &u)
	helpers.FailIfErr(err)

	if conflictFlags.mash {
		helpers.PrintBegin("Populating repo package content")
		err = pkginfo.PopulateRepo(&repo)
		helpers.FailIfErr(err)
	} else {
		helpers.PrintBegin("Populating bundle package content")
		var bundleInfo pkginfo.BundleInfo
		bundleInfo, err = pkginfo.NewBundleInfo(conf, &u)
		helpers.FailIfErr(err)
		err = pkginfo.PopulateRepoFromBundles(&bundleInfo, &repo)
		helpers.FailIfErr(err)
	}
	helpers.PrintComplete("Packages populated successfully")

	results, err := CheckFileConflicts(repo.Packages)
	helpers.FailIfErr(err)

	if results.Failed > 0 {
		os.Exit(1)
	}
}

// Attrs stores the file attribute information for comparison purposes. This
// type and its fields must be exported for the deep package to compare the
// values correctly
type Attrs struct {
	Permission string
	Owner      string
	Group      string
}

// fileCompareData contains the file data to compare for conflicts. The fields
// for this struct must be exported for the deep package to use them for comparison.
type fileCompareData struct {
	SRPM string
	Attrs
}

// newFileCompare creates the struct object fileCompareData with the SRPM and
// a new Attrs struct
func newFileCompare(f *pkginfo.File, srpm string) fileCompareData {
	return fileCompareData{srpm, Attrs{f.Permissions, f.Owner, f.Group}}
}

// CheckFileConflicts checks that no files conflict between packages
func CheckFileConflicts(rpms []*pkginfo.RPM) (*diva.Results, error) {
	r := diva.NewSuite("file conflicts", "check file conflicts of RPMs")

	allFiles := make(map[string]fileCompareData)
	conflictsSRPM := make(map[string][]string)
	conflictsATTR := make(map[string][]string)

	for _, rpm := range rpms {
		for i := range rpm.Files {
			// if the "file" doesn't have a hash or a symlink, it is a directory,
			// so ignore it
			if rpm.Files[i].Hash == "" && rpm.Files[i].SymlinkTarget == "" {
				continue
			}

			fname := rpm.Files[i].Name
			currentfi := newFileCompare(rpm.Files[i], rpm.SRPMName)

			// a conflict exists if two rpms have the same filename, but come from
			// different SRPMs.
			if savedfi, ok := allFiles[fname]; ok {
				if savedfi.SRPM != currentfi.SRPM {
					conflictsSRPM[fname] = append(conflictsSRPM[fname], savedfi.SRPM, currentfi.SRPM)
					// no need to compare the attrs of mismatching SRPMs
					continue
				}

				// It is also a conflict if two rpm files have the same filename and
				// SRPMs but different %attr values (permission, owner, and group)
				attrDiff := deep.Equal(savedfi.Attrs, currentfi.Attrs)
				if len(attrDiff) > 0 {
					c := fmt.Sprintf("%s, attr mismatch %s", currentfi.SRPM, strings.Join(attrDiff, ", "))
					conflictsATTR[fname] = append(conflictsATTR[fname], c)
				}
			}
			allFiles[fname] = currentfi
		}
	}

	r.Ok(len(conflictsSRPM) == 0, "SRPM mismatch file conflicts")
	for f, data := range conflictsSRPM {
		r.Diagnostic(fmt.Sprintf("%s found in packages: %s", f, strings.Join(data, ", ")))
	}

	r.Ok(len(conflictsATTR) == 0, "%attr mismatch file conflicts")
	for f, data := range conflictsATTR {
		r.Diagnostic(fmt.Sprintf("%s in %s", f, strings.Join(data, ", ")))
	}
	return r, nil
}
