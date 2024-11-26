// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd"
	_ "github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd/boot"
	_ "github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd/build"
	_ "github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd/check"
	_ "github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd/download"
	_ "github.com/microsoft/azurelinux/toolkit/tools/azlbuild/cmd/query"
)

func main() {
	cmd.Execute()
}
