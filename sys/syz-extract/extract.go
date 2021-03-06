// Copyright 2016 syzkaller project authors. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/syzkaller/pkg/ast"
	"github.com/google/syzkaller/pkg/compiler"
	"github.com/google/syzkaller/pkg/osutil"
)

var (
	flagLinux    = flag.String("linux", "", "path to linux kernel source checkout")
	flagLinuxBld = flag.String("linuxbld", "", "path to linux kernel build directory")
	flagArch     = flag.String("arch", "", "arch to generate")
	flagV        = flag.Int("v", 0, "verbosity")
)

type Arch struct {
	CARCH            []string
	KernelHeaderArch string
	KernelInclude    string
	CFlags           []string
}

var archs = map[string]*Arch{
	"amd64":   {[]string{"__x86_64__"}, "x86", "asm/unistd.h", []string{"-m64"}},
	"386":     {[]string{"__i386__"}, "x86", "asm/unistd.h", []string{"-m32"}},
	"arm64":   {[]string{"__aarch64__"}, "arm64", "asm/unistd.h", []string{}},
	"arm":     {[]string{"__arm__"}, "arm", "asm/unistd.h", []string{"-D__LINUX_ARM_ARCH__=6", "-m32"}},
	"ppc64le": {[]string{"__ppc64__", "__PPC64__", "__powerpc64__"}, "powerpc", "asm/unistd.h", []string{"-D__powerpc64__"}},
}

func main() {
	flag.Parse()
	if *flagLinux == "" {
		failf("provide path to linux kernel checkout via -linux flag (or make extract LINUX= flag)")
	}
	if *flagLinuxBld == "" {
		logf(1, "No kernel build directory provided, assuming in-place build")
		*flagLinuxBld = *flagLinux
	}
	if *flagArch == "" {
		failf("-arch flag is required")
	}
	if archs[*flagArch] == nil {
		failf("unknown arch %v", *flagArch)
	}
	if len(flag.Args()) != 1 {
		failf("usage: syz-extract -linux=/linux/checkout -arch=arch sys/input_file.txt")
	}

	inname := flag.Args()[0]
	outname := strings.TrimSuffix(inname, ".txt") + "_" + *flagArch + ".const"

	indata, err := ioutil.ReadFile(inname)
	if err != nil {
		failf("failed to read input file: %v", err)
	}

	desc := ast.Parse(indata, filepath.Base(inname), nil)
	if desc == nil {
		os.Exit(1)
	}

	consts := compileConsts(archs[*flagArch], desc)

	data := compiler.SerializeConsts(consts)
	if err := osutil.WriteFile(outname, data); err != nil {
		failf("failed to write output file: %v", err)
	}
}

func compileConsts(arch *Arch, desc *ast.Description) map[string]uint64 {
	info := compiler.ExtractConsts(desc, nil)
	if info == nil {
		os.Exit(1)
	}
	if len(info.Consts) == 0 {
		return nil
	}
	consts, err := fetchValues(arch.KernelHeaderArch, info.Consts,
		append(info.Includes, arch.KernelInclude), info.Incdirs, info.Defines, arch.CFlags)
	if err != nil {
		failf("%v", err)
	}
	return consts
}

func failf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func logf(v int, msg string, args ...interface{}) {
	if *flagV >= v {
		fmt.Fprintf(os.Stderr, msg+"\n", args...)
	}
}
