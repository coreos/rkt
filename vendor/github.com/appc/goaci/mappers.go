package main

import (
	"flag"
	"os/exec"
	"sort"
	"strings"

	"github.com/appc/goaci/proj2aci"
)

// stringSliceWrapper is an implementation of flag.Value
// interface. It is basically a proxy that append strings to already
// existing strings slice.
type stringSliceWrapper struct {
	vector *[]string
}

func (wrapper *stringSliceWrapper) String() string {
	if len(*wrapper.vector) > 0 {
		return `["` + strings.Join(*wrapper.vector, `" "`) + `"]`
	}
	return "[]"
}

func (wrapper *stringSliceWrapper) Set(str string) error {
	*wrapper.vector = append(*wrapper.vector, str)
	return nil
}

// commonParameterMapper maps command line parameters to
// proj2aci.CommonConfiguration.
type commonParameterMapper struct {
	custom       proj2aci.BuilderCustomizations
	config       *proj2aci.CommonConfiguration
	execWrapper  stringSliceWrapper
	assetWrapper stringSliceWrapper
}

func (mapper *commonParameterMapper) setupCommonParameters(parameters *flag.FlagSet) {
	// --exec
	mapper.execWrapper.vector = &mapper.config.Exec
	parameters.Var(&mapper.execWrapper, "exec", "Parameters passed to app, can be used multiple times")

	// --use-binary
	parameters.StringVar(&mapper.config.UseBinary, "use-binary", "", "Which executable to put in ACI image")

	// --asset
	mapper.assetWrapper.vector = &mapper.config.Assets
	parameters.Var(&mapper.assetWrapper, "asset", "Additional assets, can be used multiple times; format: "+proj2aci.GetAssetString("<path in ACI rootfs>", "<local path>")+"; available placeholders for use: "+mapper.getPlaceholders())

	// --keep-tmp-dir
	parameters.BoolVar(&mapper.config.KeepTmpDir, "keep-tmp-dir", false, "Do not delete temporary directory used for creating ACI")

	// --tmp-dir
	parameters.StringVar(&mapper.config.TmpDir, "tmp-dir", "", "Use this directory for build a project and an ACI image")

	// --reuse-tmp-dir
	parameters.StringVar(&mapper.config.ReuseTmpDir, "reuse-tmp-dir", "", "Use this already existing directory with built project to build an ACI image; ACI specific contents in this directory are removed before reuse")
}

func (mapper *commonParameterMapper) getPlaceholders() string {
	mapping := mapper.custom.GetPlaceholderMapping()
	placeholders := make([]string, 0, len(mapping))
	for p := range mapping {
		placeholders = append(placeholders, p)
	}
	sort.Strings(placeholders)
	return strings.Join(placeholders, ", ")
}

func (mapper *commonParameterMapper) Name() string {
	return mapper.custom.Name()
}

func (mapper *commonParameterMapper) GetBuilderCustomizations() proj2aci.BuilderCustomizations {
	return mapper.custom
}

// goParameterMapper maps command line parameters to
// proj2aci.GoConfiguration.
type goParameterMapper struct {
	commonParameterMapper

	goCustom *proj2aci.GoCustomizations
}

func newGoParameterMapper() parameterMapper {
	custom := &proj2aci.GoCustomizations{}
	return &goParameterMapper{
		commonParameterMapper: commonParameterMapper{
			custom: custom,
			config: custom.GetCommonConfiguration(),
		},
		goCustom: custom,
	}
}

func (mapper *goParameterMapper) SetupParameters(parameters *flag.FlagSet) {
	// common params
	mapper.setupCommonParameters(parameters)

	// --go-binary
	goDefaultBinaryDesc := "Go binary to use"
	gocmd, err := exec.LookPath("go")
	if err != nil {
		goDefaultBinaryDesc += " (default: none found in $PATH, so it must be provided)"
	} else {
		goDefaultBinaryDesc += " (default: whatever go in $PATH)"
	}
	parameters.StringVar(&mapper.goCustom.Configuration.GoBinary, "go-binary", gocmd, goDefaultBinaryDesc)

	// --go-path
	parameters.StringVar(&mapper.goCustom.Configuration.GoPath, "go-path", "", "Custom GOPATH (default: a temporary directory)")
}

// cmakeParameterMapper maps command line parameters to
// proj2aci.CmakeConfiguration.
type cmakeParameterMapper struct {
	commonParameterMapper

	cmakeCustom       *proj2aci.CmakeCustomizations
	cmakeParamWrapper stringSliceWrapper
}

func newCmakeParameterMapper() parameterMapper {
	custom := &proj2aci.CmakeCustomizations{}
	return &cmakeParameterMapper{
		commonParameterMapper: commonParameterMapper{
			custom: custom,
			config: custom.GetCommonConfiguration(),
		},
		cmakeCustom: custom,
	}
}

func (mapper *cmakeParameterMapper) SetupParameters(parameters *flag.FlagSet) {
	// common params
	mapper.setupCommonParameters(parameters)

	// --binary-dir
	parameters.StringVar(&mapper.cmakeCustom.Configuration.BinDir, "binary-dir", "", "Look for binaries in this directory (relative to install path, eg passing /usr/local/mysql/bin would look for a binary in <tmpdir>/install/usr/local/mysql/bin")

	// --reuse-src-dir
	parameters.StringVar(&mapper.cmakeCustom.Configuration.ReuseSrcDir, "reuse-src-dir", "", "Instead of downloading a project, use this path with already downloaded sources")

	// --cmake-param
	mapper.cmakeParamWrapper.vector = &mapper.cmakeCustom.Configuration.CmakeParams
	parameters.Var(&mapper.cmakeParamWrapper, "cmake-param", "Parameters passed to cmake, can be used multiple times")
}
