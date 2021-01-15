// +build !supportedBuild

package main

import (
	"context"

	"github.com/sirupsen/logrus"
)

const supportedBuildTag = ""

func initializeBuild(ctx context.Context, logger logrus.FieldLogger) error {
	return nil
}
