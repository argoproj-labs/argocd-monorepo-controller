package repository

import (
	"context"
	goio "io"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	//	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	argopath "github.com/argoproj/argo-cd/v3/util/app/path"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/io"

	//	"github.com/argoproj/argo-cd/v3/util/kustomize"

	log "github.com/sirupsen/logrus"
)

func (s *Service) GetChangeRevision(_ context.Context, request *apiclient.ChangeRevisionRequest) (result *apiclient.ChangeRevisionResponse, err error) {
	startTime := time.Now()

	logCtx := log.WithFields(log.Fields{"application": request.AppName, "appNamespace": request.Namespace})
	app := request.AppName
	namespace := request.Namespace
	repo := request.GetRepo()
	currentRevision := request.GetCurrentRevision()
	previousRevision := request.GetPreviousRevision()
	refreshPaths := request.GetPaths()
	defer func() {
		s.metricsServer.ObserveGetChangeRevisionRequestDuration(time.Since(startTime),
			repo.Repo, app, namespace)
		s.metricsServer.IncGetChangeRevisionRequest(repo.Repo, err != nil, app, namespace)
	}()

	logCtx.WithFields(log.Fields{
		"repo":             repo,
		"app":              app,
		"namespace":        namespace,
		"currentRevision":  currentRevision,
		"previousRevision": previousRevision,
		"refreshPaths":     refreshPaths,
	}).Info("GetChangeRevision called")

	if repo == nil {
		return nil, status.Error(codes.InvalidArgument, "must pass a valid repo")
	}

	if len(refreshPaths) == 0 {
		return nil, status.Error(codes.InvalidArgument, "must pass a refresh path")
	}

	var gitClientOpts []git.ClientOpts
	if s.initConstants.UseCache {
		gitClientOpts = append(gitClientOpts, git.WithCache(s.cache, true))
	}
	gitClient, revision, err := s.newClientResolveRevision(repo, currentRevision, gitClientOpts...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to resolve git revision %s: %v", revision, err)
	}
	if previousRevision == "" {
		logCtx.Infof("there is no previous revision (new app or source), using current revision as change revision")
		return &apiclient.ChangeRevisionResponse{
			Revision: currentRevision,
		}, nil
	}

	s.metricsServer.IncPendingRepoRequest(repo.Repo)
	defer s.metricsServer.DecPendingRepoRequest(repo.Repo)

	closer, err := s.repoLock.Lock(gitClient.Root(), revision, true, func() (goio.Closer, error) {
		return s.checkoutRevision(gitClient, revision, false)
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to checkout git repo %s with revision %s: %v", repo.Repo, revision, err)
	}
	defer io.Close(closer)

	logCtx.Debugf("running list revisions '%s' .. '%s'", previousRevision, revision)
	revisions, err := gitClient.ListRevisions(previousRevision, revision)
	if err != nil {
		logCtx.Errorf("failed to get revisions %s..%s: %v", previousRevision, revision, err)
		s.metricsServer.IncRevListFail(repo.Repo, app, namespace)
		return nil, status.Errorf(codes.Internal, "failed to get revisions %s..%s: %v", previousRevision, revision, err)
	}
	logCtx.Debugf("got list of %d revisions: %v", len(revisions), revisions)
	if len(revisions) == 0 {
		logCtx.Infof("no path between revisions '%s' and '%s', using current revision as change revision", previousRevision, revision)
		return &apiclient.ChangeRevisionResponse{
			Revision: revision,
		}, nil
	}
	for _, rev := range revisions {
		logCtx.Debugf("checking for changes in revision '%s'", rev)
		files, err := gitClient.DiffTree(rev)
		if err != nil {
			s.metricsServer.IncDiffTreeFail(repo.Repo, app, namespace)
			logCtx.Errorf("Difftree for revision %s returned error: %s", rev, err.Error())
			return nil, status.Errorf(codes.Internal, "Difftree for revision %s returned error: %s", rev, err.Error())
		}
		logCtx.Debugf("refreshpath is '%v'", refreshPaths)
		logCtx.Debugf("files are '%v'", files)
		if len(files) == 0 {
			continue
		}
		changedFiles := argopath.AppFilesHaveChanged(refreshPaths, files)
		if changedFiles {
			logCtx.Infof("changes found in repo %s from revision %s to revision %s in revision %s", repo.Repo, previousRevision, revision, rev)
			return &apiclient.ChangeRevisionResponse{
				Revision: rev,
			}, nil
		}
	}
	logCtx.Infof("changes not found in repo %s from revision %s to revision %s", repo.Repo, previousRevision, revision)
	return &apiclient.ChangeRevisionResponse{}, nil
}
