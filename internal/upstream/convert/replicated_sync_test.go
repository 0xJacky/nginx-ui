package convert

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/model"
)

// A conversion another node replicated here must not be mirrored again. No
// database is set up, so resolving the site's sync nodes would panic: the
// sync has to return before it gets that far.
func TestReplicatedConversionIsNotMirrored(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/upstream/convert", nil)
	request.Header.Set(nodeauth.ReplicatedFromHeader, `"origin-instance"`)
	request = nodeauth.WithPrincipal(request, &nodeauth.Principal{AuthMethod: model.NodeAuthMethodLegacy})

	startSync(request.Context(), &plan{siteName: "legacy.test"}, &model.Config{Filepath: "/nowhere.conf"}, "legacy_pool", "admin")
	WaitForSync()
}
