package environment

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/go-github/v41/github"
	"github.com/porter-dev/porter/api/server/authz"
	"github.com/porter-dev/porter/api/server/handlers"
	"github.com/porter-dev/porter/api/server/shared"
	"github.com/porter-dev/porter/api/server/shared/apierrors"
	"github.com/porter-dev/porter/api/server/shared/config"
	"github.com/porter-dev/porter/api/types"
	"github.com/porter-dev/porter/internal/models"
	"gorm.io/gorm"
)

type EnablePullRequestHandler struct {
	handlers.PorterHandlerReadWriter
	authz.KubernetesAgentGetter
}

func NewEnablePullRequestHandler(
	config *config.Config,
	decoderValidator shared.RequestDecoderValidator,
	writer shared.ResultWriter,
) *EnablePullRequestHandler {
	return &EnablePullRequestHandler{
		PorterHandlerReadWriter: handlers.NewDefaultPorterHandler(config, decoderValidator, writer),
		KubernetesAgentGetter:   authz.NewOutOfClusterAgentGetter(config),
	}
}

func (c *EnablePullRequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	project, _ := r.Context().Value(types.ProjectScope).(*models.Project)
	cluster, _ := r.Context().Value(types.ClusterScope).(*models.Cluster)

	request := &types.PullRequest{}

	if ok := c.DecodeAndValidate(w, r, request); !ok {
		return
	}

	env, err := c.Repo().Environment().ReadEnvironmentByOwnerRepoName(project.ID, cluster.ID, request.RepoOwner, request.RepoName)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.HandleAPIError(w, r, apierrors.NewErrNotFound(fmt.Errorf("environment not found in cluster and project")))
			return
		}

		c.HandleAPIError(w, r, apierrors.NewErrInternal(err))
		return
	}

	envType := env.ToEnvironmentType()

	if len(envType.GitRepoBranches) > 0 {
		found := false

		for _, branch := range env.ToEnvironmentType().GitRepoBranches {
			if branch == request.BranchInto {
				found = true
				break
			}
		}

		if !found {
			c.HandleAPIError(w, r, apierrors.NewErrPassThroughToClient(
				fmt.Errorf("base branch '%s' is not enabled for this preview environment, please enable it "+
					"in the settings page to continue", request.BranchInto), http.StatusBadRequest,
			))
			return
		}
	} else if len(envType.GitDeployBranches) > 0 {
		found := false

		for _, branch := range env.ToEnvironmentType().GitDeployBranches {
			if branch == request.BranchFrom {
				found = true
				break
			}
		}

		if found {
			c.HandleAPIError(w, r, apierrors.NewErrPassThroughToClient(
				fmt.Errorf("head branch '%s' is enabled for branch deploys for this preview environment, "+
					"please disable it in the settings page to continue", request.BranchInto), http.StatusBadRequest,
			))
			return
		}
	}

	client, err := getGithubClientFromEnvironment(c.Config(), env)
	if err != nil {
		c.HandleAPIError(w, r, apierrors.NewErrInternal(err))
		return
	}

	// add an extra check that the installation has permission to read this pull request
	pr, _, err := client.PullRequests.Get(r.Context(), env.GitRepoOwner, env.GitRepoName, int(request.Number))
	if err != nil {
		c.HandleAPIError(w, r, apierrors.NewErrPassThroughToClient(fmt.Errorf("%v: %w", errGithubAPI, err),
			http.StatusConflict))
		return
	}

	if pr.GetState() == "closed" {
		c.HandleAPIError(w, r, apierrors.NewErrPassThroughToClient(fmt.Errorf("cannot enable deployment for closed PR"),
			http.StatusConflict))
		return
	}

	ghResp, err := client.Actions.CreateWorkflowDispatchEventByFileName(
		r.Context(), env.GitRepoOwner, env.GitRepoName, fmt.Sprintf("porter_%s_env.yml", env.Name),
		github.CreateWorkflowDispatchEventRequest{
			Ref: request.BranchFrom,
			Inputs: map[string]interface{}{
				"pr_number":      strconv.FormatUint(uint64(request.Number), 10),
				"pr_title":       pr.GetTitle(),
				"pr_branch_from": request.BranchFrom,
				"pr_branch_into": request.BranchInto,
			},
		},
	)

	if ghResp != nil {
		if ghResp.StatusCode == 404 {
			c.HandleAPIError(w, r, apierrors.NewErrPassThroughToClient(
				fmt.Errorf(
					"please make sure the preview environment workflow files are present in PR branch %s and are up to"+
						" date with the default branch", request.BranchFrom,
				), http.StatusConflict),
			)
			return
		} else if ghResp.StatusCode == 422 {
			c.HandleAPIError(w, r, apierrors.NewErrPassThroughToClient(
				fmt.Errorf(
					"please make sure the workflow files in PR branch %s are up to date with the default branch",
					request.BranchFrom,
				), http.StatusConflict),
			)
			return
		}
	}

	if err != nil {
		c.HandleAPIError(w, r, apierrors.NewErrInternal(err))
		return
	}

	// create the deployment
	depl, err := c.Repo().Environment().CreateDeployment(&models.Deployment{
		EnvironmentID: env.ID,
		Namespace:     "",
		Status:        types.DeploymentStatusCreating,
		PullRequestID: request.Number,
		RepoOwner:     request.RepoOwner,
		RepoName:      request.RepoName,
		PRName:        request.Title,
		PRBranchFrom:  request.BranchFrom,
		PRBranchInto:  request.BranchInto,
	})
	if err != nil {
		c.HandleAPIError(w, r, apierrors.NewErrInternal(err))
		return
	}

	c.WriteResult(w, r, depl.ToDeploymentType())
}
