package docker

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types/container"
	dockererrdefs "github.com/docker/docker/errdefs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePathStatClient struct {
	statErr      error
	inspectErr   error
	inspectCalls int
}

func (f *fakePathStatClient) ContainerStatPath(context.Context, string, string) (container.PathStat, error) {
	return container.PathStat{}, f.statErr
}

func (f *fakePathStatClient) ContainerInspect(context.Context, string) (container.InspectResponse, error) {
	f.inspectCalls++
	return container.InspectResponse{}, f.inspectErr
}

func TestStatPath(t *testing.T) {
	notFoundErr := dockererrdefs.NotFound(errors.New("not found"))

	tests := []struct {
		name             string
		statErr          error
		inspectErr       error
		wantExists       bool
		wantErr          bool
		wantInspectCalls int
	}{
		{
			name:       "path exists",
			wantExists: true,
		},
		{
			name:             "path missing in existing container",
			statErr:          notFoundErr,
			wantInspectCalls: 1,
		},
		{
			name:             "container missing",
			statErr:          notFoundErr,
			inspectErr:       notFoundErr,
			wantErr:          true,
			wantInspectCalls: 1,
		},
		{
			name:    "stat request fails",
			statErr: errors.New("request failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &fakePathStatClient{
				statErr:    tt.statErr,
				inspectErr: tt.inspectErr,
			}

			exists, err := statPath(context.Background(), cli, "nginx", "/run/nginx.pid")

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantExists, exists)
			assert.Equal(t, tt.wantInspectCalls, cli.inspectCalls)
		})
	}
}
