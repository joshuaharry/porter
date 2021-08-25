package cluster

import (
	"net/http"

	"github.com/porter-dev/porter/api/server/handlers"
	"github.com/porter-dev/porter/api/server/shared"
	"github.com/porter-dev/porter/api/server/shared/apierrors"
	"github.com/porter-dev/porter/api/types"
	"github.com/porter-dev/porter/internal/models"
)

type CreateNamespaceHandler struct {
	handlers.PorterHandlerReadWriter
	KubernetesAgentGetter
}

func NewCreateNamespaceHandler(
	config *shared.Config,
	decoderValidator shared.RequestDecoderValidator,
	writer shared.ResultWriter,
) *CreateNamespaceHandler {
	return &CreateNamespaceHandler{
		PorterHandlerReadWriter: handlers.NewDefaultPorterHandler(config, decoderValidator, writer),
		KubernetesAgentGetter:   NewDefaultKubernetesAgentGetter(config),
	}
}

func (c *CreateNamespaceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	request := &types.CreateNamespaceRequest{}

	if ok := c.DecodeAndValidate(w, r, request); !ok {
		return
	}

	cluster, _ := r.Context().Value(types.ClusterScope).(*models.Cluster)

	agent, err := c.GetAgent(cluster)

	if err != nil {
		c.HandleAPIError(w, apierrors.NewErrInternal(err))
		return
	}

	namespace, err := agent.CreateNamespace(request.Name)

	if err != nil {
		c.HandleAPIError(w, apierrors.NewErrInternal(err))
		return
	}

	res := types.CreateNamespaceResponse{
		Namespace: namespace,
	}

	c.WriteResult(w, res)
}
