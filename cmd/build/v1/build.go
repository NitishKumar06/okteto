// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

// OktetoBuilderInterface runs the build of an image
type OktetoBuilderInterface interface {
	Run(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.IOController) error
}

type oktetoRegistryInterface interface {
	GetImageTagWithDigest(imageTag string) (string, error)
	HasGlobalPushAccess() (bool, error)
}

// OktetoBuilder builds the images
type OktetoBuilder struct {
	Builder  OktetoBuilderInterface
	Registry oktetoRegistryInterface
	IoCtrl   *io.IOController
}

// NewBuilder creates a new okteto builder
func NewBuilder(builder OktetoBuilderInterface, registry oktetoRegistryInterface, ioCtrl *io.IOController) *OktetoBuilder {
	return &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
		IoCtrl:   ioCtrl,
	}
}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch(ioCtrl *io.IOController) *OktetoBuilder {
	builder := &build.OktetoBuilder{}
	registry := registry.NewOktetoRegistry(okteto.Config{})
	return NewBuilder(builder, registry, ioCtrl)
}

// IsV1 returns true since it is a builder v1
func (*OktetoBuilder) IsV1() bool {
	return true
}

// Build builds the images defined by a Dockerfile
func (bc *OktetoBuilder) Build(ctx context.Context, options *types.BuildOptions) error {
	path := "."
	if options.Path != "" {
		path = options.Path
	}
	if len(options.CommandArgs) == 1 {
		path = options.CommandArgs[0]
	}

	if err := utils.CheckIfDirectory(path); err != nil {
		return fmt.Errorf("invalid build context: %s", err.Error())
	}
	options.Path = path

	if options.File == "" {
		options.File = filepath.Join(path, "Dockerfile")
	}

	if exists := filesystem.FileExistsAndNotDir(options.File, afero.NewOsFs()); !exists {
		return fmt.Errorf("%s: '%s' is not a regular file", oktetoErrors.InvalidDockerfile, options.File)
	}

	buildMsg := fmt.Sprintf("Building '%s'", options.File)
	if okteto.Context().Builder == "" {
		bc.IoCtrl.Out().Infof("%s using your local docker daemon...", buildMsg)
	} else {
		bc.IoCtrl.Out().Infof("%s in %s...", buildMsg, okteto.Context().Builder)
	}

	var err error
	options.Tag, err = env.ExpandEnv(options.Tag)
	if err != nil {
		return err
	}

	if err := bc.Builder.Run(ctx, options, bc.IoCtrl); err != nil {
		analytics.TrackBuild(false)
		return err
	}

	if options.Tag == "" {
		bc.IoCtrl.Out().Success("Build succeeded")
		bc.IoCtrl.Out().Infof("Your image won't be pushed. To push your image specify the flag '-t'.")
	} else {
		displayTag := options.Tag
		if options.DevTag != "" {
			displayTag = options.DevTag
		}
		bc.IoCtrl.Out().Success("Image '%s' successfully pushed", displayTag)
	}

	analytics.TrackBuild(true)
	return nil
}
