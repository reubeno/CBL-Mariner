// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"fmt"
	"os"

	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	_ "github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd/boot"
	_ "github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd/build"
	_ "github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd/check"
	_ "github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd/download"
	_ "github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd/edit"
	_ "github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd/query"
)

func main() {
	// Make sure we're not running as root.
	if os.Geteuid() == 0 {
		fmt.Fprintln(os.Stderr, "error: this tool may not be run as root; it will internally invoke sub-commands as sudo as needed")
		os.Exit(1)
	}

	cmd.Execute()
}
