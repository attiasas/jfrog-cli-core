package transferfiles

import (
	"fmt"
	"path"
	"time"

	"github.com/jfrog/gofrog/parallel"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Manages the phase of performing a full transfer of the repository.
// This phase is only executed once per repository if its completed.
// Transfer is performed by treating every folder as a task, and searching for it's content in a flat AQL.
// New folders found are handled as a separate task, and files are uploaded in chunks and polled on for status.
type fullTransferPhase struct {
	phaseBase
	transferManager *transferManager
}

func (m *fullTransferPhase) initProgressBar() error {
	if m.progressBar == nil {
		return nil
	}
	tasks, err := m.repoSummary.FilesCount.Int64()
	if err != nil {
		return err
	}
	m.progressBar.AddPhase1(tasks)
	return nil
}

func (m *fullTransferPhase) getPhaseName() string {
	return "Full Transfer Phase"
}

func (m *fullTransferPhase) phaseStarted() error {
	m.startTime = time.Now()
	return m.stateManager.SetRepoFullTransferStarted(m.repoKey, m.startTime)
}

func (m *fullTransferPhase) phaseDone() error {
	// If the phase stopped gracefully, don't mark the phase as completed
	if !m.ShouldStop() {
		if err := m.stateManager.SetRepoFullTransferCompleted(m.repoKey); err != nil {
			return err
		}
	}

	if m.progressBar != nil {
		return m.progressBar.DonePhase(m.phaseId)
	}
	return nil
}

func (m *fullTransferPhase) shouldSkipPhase() (bool, error) {
	repoTransferred, err := m.stateManager.IsRepoTransferred(m.repoKey)
	if err != nil {
		return false, err
	}
	if repoTransferred {
		m.skipPhase()
	}
	return repoTransferred, nil
}

func (m *fullTransferPhase) skipPhase() {
	// Init progress bar as "done" with 0 tasks.
	if m.progressBar != nil {
		m.progressBar.AddPhase1(0)
	}
}

func (m *fullTransferPhase) run() error {
	m.transferManager = newTransferManager(m.phaseBase, getDelayUploadComparisonFunctions(m.repoSummary.PackageType))
	action := func(pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunkData, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
			return nil
		}
		folderHandler := m.createFolderFullTransferHandlerFunc(*pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
		_, err := pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: m.repoKey, relativePath: "."}), pcWrapper.errorsQueue.AddError)
		return err
	}
	delayAction := consumeDelayFilesIfNoErrors
	return m.transferManager.doTransferWithProducerConsumer(action, delayAction)
}

type folderFullTransferHandlerFunc func(params folderParams) parallel.TaskFunc

type folderParams struct {
	repoKey      string
	relativePath string
}

func (m *fullTransferPhase) createFolderFullTransferHandlerFunc(pcWrapper producerConsumerWrapper, uploadChunkChan chan UploadedChunkData,
	delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) folderFullTransferHandlerFunc {
	return func(params folderParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			return m.transferFolder(params, logMsgPrefix, pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
		}
	}
}

func (m *fullTransferPhase) transferFolder(params folderParams, logMsgPrefix string, pcWrapper producerConsumerWrapper,
	uploadChunkChan chan UploadedChunkData, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) (err error) {
	log.Debug(logMsgPrefix+"Visited folder:", path.Join(params.repoKey, params.relativePath))

	curUploadChunk := UploadChunk{
		TargetAuth:                createTargetAuth(m.targetRtDetails, m.proxyKey),
		CheckExistenceInFilestore: m.checkExistenceInFilestore,
	}

	var result *servicesUtils.AqlSearchResult
	paginationI := 0
	for {
		if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
			return
		}
		result, err = m.getDirectoryContentsAql(params.repoKey, params.relativePath, paginationI)
		if err != nil {
			return err
		}

		// Empty folder. Add it as candidate.
		if paginationI == 0 && len(result.Results) == 0 {
			curUploadChunk.appendUploadCandidateIfNeeded(FileRepresentation{Repo: params.repoKey, Path: params.relativePath}, m.buildInfoRepo)
			break
		}

		for _, item := range result.Results {
			if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
				return
			}
			if item.Name == "." {
				continue
			}
			switch item.Type {
			case "folder":
				newRelativePath := item.Name
				if params.relativePath != "." {
					newRelativePath = path.Join(params.relativePath, newRelativePath)
				}
				folderHandler := m.createFolderFullTransferHandlerFunc(pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
				_, err = pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: params.repoKey, relativePath: newRelativePath}), pcWrapper.errorsQueue.AddError)
				if err != nil {
					return err
				}
			case "file":
				file := FileRepresentation{Repo: item.Repo, Path: item.Path, Name: item.Name}
				delayed, stopped := delayHelper.delayUploadIfNecessary(m.phaseBase, file)
				if stopped {
					return
				}
				if delayed {
					continue
				}
				curUploadChunk.appendUploadCandidateIfNeeded(file, m.buildInfoRepo)
				if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
					_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(&m.phaseBase, curUploadChunk, uploadChunkChan, errorsChannelMng), pcWrapper.errorsQueue.AddError)
					if err != nil {
						return
					}
					// Empty the uploaded chunk.
					curUploadChunk.UploadCandidates = []FileRepresentation{}
				}
			}
		}

		if len(result.Results) < AqlPaginationLimit {
			break
		}
		paginationI++
	}

	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(&m.phaseBase, curUploadChunk, uploadChunkChan, errorsChannelMng), pcWrapper.errorsQueue.AddError)
		if err != nil {
			return
		}
	}
	log.Debug(logMsgPrefix+"Done transferring folder:", path.Join(params.repoKey, params.relativePath))
	return
}

func (m *fullTransferPhase) getDirectoryContentsAql(repoKey, relativePath string, paginationOffset int) (result *servicesUtils.AqlSearchResult, err error) {
	query := generateFolderContentsAqlQuery(repoKey, relativePath, paginationOffset)
	return runAql(m.context, m.srcRtDetails, query)
}

func generateFolderContentsAqlQuery(repoKey, relativePath string, paginationOffset int) string {
	query := fmt.Sprintf(`items.find({"type":"any","$or":[{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}]})`, repoKey, relativePath)
	query += `.include("repo","path","name","type")`
	query += fmt.Sprintf(`.sort({"$asc":["name"]}).offset(%d).limit(%d)`, paginationOffset*AqlPaginationLimit, AqlPaginationLimit)
	return query
}
