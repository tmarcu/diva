package cmd

import (
	"debug/elf"
	"errors"
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

		errs := runCanaryCheck(r, u, args)
		r.Ok(len(errs) == 0, "All ELF binaries pass stack canary check")
		if len(errs) > 0 {
			r.Diagnostic(fmt.Sprint(len(errs)) + " Canaries Missing:\n" + strings.Join(errs, "\n"))
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
		return fmt.Errorf("%s for: %s", err.Error(), name)
	}
	// This is the canary we need to check for
	for _, s := range dynSymbols {
		if s.Name == canaryString {
			return nil
		}
	}
	return fmt.Errorf("No canary found for: %s", name)
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

func readCanary(oldfile, newfile string) error {
	var err error
	newFile, err := os.OpenFile(newfile, os.O_RDONLY, 0)
	if os.IsNotExist(err) {
		// File no longer exists, so nothing to check
		return nil
	} else if err != nil {
		return err
	}
	defer func() {
		_ = newFile.Close()
	}()

	// Don't fail on non-elf files, so check before trying to parse
	var ret bool
	ret, err = isElf(newFile)
	if err != nil {
		return err
	}
	// It's just not an elf file, so ignore trying to check for canary
	if !ret {
		return nil
	}

	// Check the old file now since new does exist and is a valid ELF file
	prev, err := os.OpenFile(oldfile, os.O_RDONLY, 0)
	// Only exit if the error was something other than IsNotExist()
	// because prev will be set to nil if it's not there, and we still
	// want to check newfile if it was newly added in the build.
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	defer func() {
		_ = prev.Close()
	}()

	// Check both files if they exist to make sure the canary exists and was
	// not lost between the two build versions.
	var returnErr string
	var prevElf *elf.File
	var newElf *elf.File
	if prev != nil {
		ret, err = isElf(prev)
		if err != nil {
			return err
		}
		// Only check for canary if it is a valid ELF file
		if ret {
			// Get ELF specific struct to read symbols from
			prevElf, err = elf.NewFile(prev)
			// If we get an err here, it IS an elf file, but couldn't be parsed
			if err != nil {
				return err
			}
			if err = findCanary(oldfile+"\n", prevElf); err != nil {
				returnErr += err.Error()
			}
		}
	}
	newElf, err = elf.NewFile(newFile)
	if err != nil {
		return err
	}
	if err = findCanary(newfile, newElf); err != nil {
		returnErr += err.Error()
	}
	if returnErr != "" {
		return errors.New(returnErr)
	}
	return nil
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
func runCanaryCheck(r *diva.Results, u diva.UInfo, args []string) (errs []string) {
	oldBuild := args[0]
	newBuild := args[1]
	var filesList []string
	err := filepath.Walk(newBuild, getFiles(&filesList))
	if err != nil {
		return []string{err.Error()}
	}

	// Check using new full chroot only since it covers new/deleted files
	for _, newfile := range filesList {
		oldfile := filepath.Join(oldBuild, strings.TrimPrefix(newfile, newBuild))
		if err := readCanary(oldfile, newfile); err != nil {
			errs = append(errs, err.Error())
		}
	}
	return errs
}

var chirpFlag bool

func init() {
	chirpCmd.Flags().BoolVar(&chirpFlag, "chirp", false, "Print number of errors in chirps")
	_ = chirpCmd.Flags().MarkHidden("chirp")
}
