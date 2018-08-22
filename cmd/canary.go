package cmd

import (
	"debug/elf"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/clearlinux/diva/diva"
)

const canaryString = "__stack_chk_fail"

var chirpCmd = &cobra.Command{
	Use:   "canary <full chroot1> <full chroot2>",
	Short: "Check that stack canary exists between files in the given build versions",
	Long: `Check that stack canary exists between files in the given build versions.
This means that the binary was compiled with stack protection enabled, which should persist
between builds, and only warn if it does not exist at all in any build.`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		// Passing false to the last "recursive" flag because we don't want all manifest from minversion
		u, divaErr := diva.GetUpstreamInfo(conf, conf.UpstreamURL, bloatFlags.version, true, false)
		if divaErr != nil {
			os.Exit(1)
		}

		r := diva.NewSuite("canary check", "check that stack canary exists in all ELF files")

		errs, warnings := runCanaryCheck(r, u, args)
		r.Ok(len(errs) == 0, "No regressions since last build, all ELF binaries pass stack canary check")
		if len(errs) > 0 {
			r.Diagnostic(fmt.Sprint(len(errs)) + " Canaries Missing:\n" + strings.Join(errs, "\n"))
		}
		r.Ok(len(warnings) == 0, "No issues since last build, all existing ELF issues resolved")
		if len(warnings) > 0 {
			r.Diagnostic(fmt.Sprint(len(warnings)) + " Warnings:\n" + strings.Join(warnings, "\n"))
		}

		if chirpFlag {
			for i := 0; i < len(errs); i++ {
				fmt.Printf("chirp ")
			}
		}
	},
}

func findCanary(name string, file *elf.File) error {
	dynSymbols, err := file.DynamicSymbols()
	if err != nil {
		return fmt.Errorf("\n%s for: %s ", err.Error(), name)
	}
	// This is the canary we need to check for
	for _, s := range dynSymbols {
		if s.Name == canaryString {
			return nil
		}
	}
	return fmt.Errorf("\nNo canary found for: %s", name)
}

// Taken directly from the implementation of elf.NewFile()...
// Read and decode ELF identifier so we can quit early if not ELF file
func isElf(r io.ReaderAt) (bool, error) {
	var ident [16]uint8
	if _, err := r.ReadAt(ident[0:], 0); err != nil {
		// If this fails with EOF it means the header doesn't even exist in the file
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}
	if ident[0] != '\x7f' || ident[1] != 'E' || ident[2] != 'L' || ident[3] != 'F' {
		return false, nil
	}

	return true, nil
}

func getFileCanary(file string) error {
	fd, err := os.OpenFile(file, os.O_RDONLY, 0)
	if os.IsNotExist(err) {
		// File no longer exists, so nothing to check
		return nil
	} else if err != nil {
		return err
	}
	defer func() {
		_ = fd.Close()
	}()

	// Don't fail on non-elf files, so check before trying to parse
	var isELFfile bool
	isELFfile, err = isElf(fd)
	if err != nil || !isELFfile {
		return err
	}

	var fileELF *elf.File
	fileELF, err = elf.NewFile(fd)
	// If we get an err here, it IS an elf file but couldn't be parsed.
	// We need to check if the previous was an elf file and had a canary.
	if err != nil {
		return err
	}
	return findCanary(file, fileELF)
}

func readCanary(oldfile, newfile string) (string, error) {
	var err error
	// If newfile exists AND has a canary, don't waste time parsing previous
	if err = getFileCanary(newfile); err == nil {
		return "", nil
	}

	returnErr := err.Error()

	// Check both files if they exist to make sure the canary exists and was
	// not lost between the two build versions.
	if err = getFileCanary(oldfile); err == nil {
		return fmt.Sprint("old file contains canary: "), fmt.Errorf("%s", returnErr)
	}
	// Neither old nor new have canaries, return warning
	return fmt.Sprintf("%s%s", err.Error(), returnErr), nil
}

func getFiles(files *[]string) filepath.WalkFunc {
	// Only add valid regular files to the list to check
	return func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsRegular() {
			*files = append(*files, path)
		}
		return nil
	}
}

// Checks if __stack_chk_fail is in the dynamic symbols table of binary
func runCanaryCheck(r *diva.Results, u diva.UInfo, args []string) (errs, warnings []string) {
	oldBuild := args[0]
	newBuild := args[1]
	var filesList []string
	err := filepath.Walk(newBuild, getFiles(&filesList))
	if err != nil {
		return []string{}, []string{err.Error()}
	}

	// Check using new full chroot only since it covers new/deleted files
	for _, newfile := range filesList {
		oldfile := filepath.Join(oldBuild, strings.TrimPrefix(newfile, newBuild))
		errString, err := readCanary(oldfile, newfile)
		if err != nil {
			errs = append(errs, errString)
		} else if errString != "" {
			warnings = append(warnings, errString)
		}
	}
	return errs, warnings
}

var chirpFlag bool

func init() {
	chirpCmd.Flags().BoolVar(&chirpFlag, "chirp", false, "Print number of errors in chirps")
	_ = chirpCmd.Flags().MarkHidden("chirp")
}
