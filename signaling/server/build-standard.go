// +build !supportedBuild

package server

import (
	"context"
)

func bootstrapBuild(context.Context, *Server) error {
	// Do nothing.
	return nil
}
