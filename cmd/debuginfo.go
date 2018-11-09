package cmd

import (
	"fmt"
	"strings"

	"github.com/clearlinux/diva/diva"
	"github.com/clearlinux/diva/internal/config"
	"github.com/clearlinux/diva/internal/helpers"
	"github.com/clearlinux/diva/pkginfo"
	"github.com/spf13/cobra"
)

type debuginfoCmdFlags struct {
	mixName string
	version string
	latest  bool
	verbose bool
}

var debuginfoFlags debuginfoCmdFlags

func init() {
	debuginfoCmd.Flags().StringVarP(&debuginfoFlags.mixName, "name", "n", "clear", "name of data group")
	debuginfoCmd.Flags().StringVarP(&debuginfoFlags.version, "version", "v", "0", "version to check")
	debuginfoCmd.Flags().BoolVar(&debuginfoFlags.latest, "latest", false, "get the latest version from upstreamURL")
	debuginfoCmd.Flags().BoolVar(&debuginfoFlags.verbose, "verbose", false, "lists failed package results")
}

var debuginfoCmd = &cobra.Command{
	Use:   "debuginfo",
	Short: "checks the completeness of debuginfo packages",
	Long: `checks whether all debuginfo packages contain files, and if so whether
they contain source information as well. Returns two results, the first being
packages that fail to have any files, and the second being debug packages that
have incomplete sources. The results are reported as the number of failures over
the total number of packages; to get the names of the failed rpms, pass the
--verbose flag.`,
	Run: runCheckDebuginfo,
}

func runCheckDebuginfo(cmd *cobra.Command, args []string) {
	r := diva.NewSuite("debuginfo", "Validates debuginfo")

	u := config.UInfo{
		MixName: debuginfoFlags.mixName,
		Ver:     debuginfoFlags.version,
		Latest:  debuginfoFlags.latest,
		RPMType: "debug",
	}

	repo, err := pkginfo.NewRepo(conf, &u)
	helpers.FailIfErr(err)

	helpers.PrintBegin("Populating debuginfo repo")
	err = pkginfo.PopulateRepo(&repo)
	helpers.FailIfErr(err)
	helpers.PrintComplete("Repo content populated successfully")

	validateDebuginfo(r, &repo)
}

func validateDebuginfo(r *diva.Results, repo *pkginfo.Repo) {
	emptyFiles := []string{}
	missing := []string{}

	for _, pkg := range repo.Packages {
		if len(pkg.Files) == 0 {
			emptyFiles = append(emptyFiles, pkg.Name)
			// no point in doing the other checks if the files list is empty
			continue
		}

		var srcFlag, libFlag bool
		for i := range pkg.Files {
			fname := pkg.Files[i].Name
			// a package must contain a file within "/usr/src/debug" and "/usr/lib/debug"
			// to be considered a good state, if either are missing append to missing.
			if strings.HasPrefix(fname, "/usr/src/debug") {
				srcFlag = true
			}
			if strings.HasPrefix(fname, "/usr/lib/debug") {
				libFlag = true
			}
			// no need to read all files, if both lib and src files found
			if srcFlag && libFlag {
				break
			}
		}
		// after reading all files, if there is no src or lib then add to missing
		if !srcFlag || !libFlag {
			missing = append(missing, pkg.Name)
		}
	}

	r.Ok(len(emptyFiles) == 0, "Empty debuginfo files")
	if len(emptyFiles) > 0 {
		r.Diagnostic(fmt.Sprintf("Empty debuginfo files: %d/%d\n", len(emptyFiles), len(repo.Packages)))
		if debuginfoFlags.verbose {
			r.Diagnostic(fmt.Sprintf("Empty debuginfo packages: %d/%d\n\n%s", len(emptyFiles), len(repo.Packages), strings.Join(emptyFiles, "\n")))
		}
	}

	r.Ok(len(missing) == 0, "Missing source debuginfo")
	if len(missing) > 0 {
		r.Diagnostic(fmt.Sprintf("Missing source debuginfo: %d/%d\n", len(missing), len(repo.Packages)))
		if debuginfoFlags.verbose {
			r.Diagnostic(fmt.Sprintf("Missing source debuginfo: %d/%d\n\n%s", len(missing), len(repo.Packages), strings.Join(missing, "\n")))
		}
	}
}
