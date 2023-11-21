package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/lukemarsden/helix/api/pkg/model"
	"github.com/lukemarsden/helix/api/pkg/server"
	"github.com/lukemarsden/helix/api/pkg/system"
	"github.com/lukemarsden/helix/api/pkg/types"
	"github.com/rs/zerolog/log"
)

// a long running instance of a loaded into memory model
// that can run multiple session tasks sequentially
// we keep state of the active text stream (if the model supports it)
// and are in charge of sending updates out of the model to the api
// to update it's state
type ModelInstance struct {
	id string

	model  model.Model
	filter types.SessionFilter

	finishChan chan bool

	runnerOptions     RunnerOptions
	httpClientOptions server.ClientOptions

	// these URLs will have the instance ID appended by the model instance
	// e.g. http://localhost:8080/api/v1/worker/task/:instanceid
	nextTaskURL string
	// this is used to read what the next session is
	// i.e. once the session has prepared - we can read the next session
	// and know what the Lora file is
	initialSessionURL string

	// we write responses to this function and they will be sent to the api
	responseHandler func(res *types.RunnerTaskResponse) error

	// we create a cancel context for the running process
	// which is derived from the main runner context
	ctx context.Context

	// the command we are currently executing
	currentCommand *exec.Cmd

	// the session that meant this model booted in the first place
	// used to know which lora type file we should download before
	// trying to start this model's python process
	initialSession *types.Session

	// the session currently running on this model
	currentSession *types.Session

	// if there is a value here - it will be fed into the running python
	// process next - it acts as a buffer for a session we want to run right away
	nextSession *types.Session

	// this is the session that we are preparing to run next
	// if there is a value here - then we return nil
	// because there is a task running (e.g. downloading files)
	// that we need to complete before we want this session to run
	queuedSession *types.Session

	// the timestamp of when this model instance either completed a job
	// or a new job was pulled and allocated
	// we use this timestamp to cleanup non-active model instances
	lastActivityTimestamp int64

	// the file handler we use to download and upload session files
	fileHandler *FileHandler
}

func NewModelInstance(
	ctx context.Context,

	// the session that meant this model instance is instantiated
	initialSession *types.Session,
	// these URLs will have the instance ID appended by the model instance
	// e.g. http://localhost:8080/api/v1/worker/task/:instanceid
	// we just pass http://localhost:8080/api/v1/worker/task
	nextTaskURL string,
	// these URLs will have the instance ID appended by the model instance
	// e.g. http://localhost:8080/api/v1/worker/initial_session/:instanceid
	initialSessionURL string,

	responseHandler func(res *types.RunnerTaskResponse) error,

	runnerOptions RunnerOptions,
) (*ModelInstance, error) {
	modelInstance, err := model.GetModel(initialSession.ModelName)
	if err != nil {
		return nil, err
	}
	id := system.GenerateUUID()

	// if this is empty string then we need to hoist it to be types.LORA_DIR_NONE
	// because then we are always specifically asking for a session that has no finetune file
	// if we left this blank we are saying "we don't care if it has one or not"
	useLoraDir := initialSession.LoraDir

	if useLoraDir == "" {
		useLoraDir = types.LORA_DIR_NONE
	}

	httpClientOptions := server.ClientOptions{
		Host:  runnerOptions.ApiHost,
		Token: runnerOptions.ApiToken,
	}

	return &ModelInstance{
		id:                id,
		ctx:               ctx,
		finishChan:        make(chan bool, 1),
		model:             modelInstance,
		responseHandler:   responseHandler,
		nextTaskURL:       fmt.Sprintf("%s/%s", nextTaskURL, id),
		initialSessionURL: fmt.Sprintf("%s/%s", initialSessionURL, id),
		initialSession:    initialSession,
		filter: types.SessionFilter{
			ModelName: initialSession.ModelName,
			Mode:      initialSession.Mode,
			LoraDir:   useLoraDir,
			Type:      initialSession.Type,
		},
		runnerOptions:     runnerOptions,
		httpClientOptions: httpClientOptions,
		fileHandler:       NewFileHandler(runnerOptions.ID, httpClientOptions),
	}, nil
}

/*



	QUEUE



*/

// this is the loading of a session onto a running model instance
// it also returns the task that will be fed down into the python code to execute
func (instance *ModelInstance) assignSessionTask(ctx context.Context, session *types.Session) (*types.RunnerTask, error) {
	// mark the instance as active so it doesn't get cleaned up
	instance.lastActivityTimestamp = time.Now().Unix()
	instance.currentSession = session

	task, err := instance.model.GetTask(session)
	if err != nil {
		return nil, err
	}
	task.SessionID = session.ID
	return task, nil
}

// to queue a session means to put it into a buffer and wait for the Python process to boot up and then "pull" it
func (instance *ModelInstance) queueSession(session *types.Session, isInitialSession bool) {
	instance.queuedSession = session
	instance.nextSession = nil

	log.Debug().
		Msgf("🔵 runner prepare session: %s", session.ID)

	preparedSession, err := instance.fileHandler.downloadSession(session, isInitialSession)

	if err != nil {
		log.Error().Msgf("error preparing session: %s", err.Error())
		instance.queuedSession = nil
		instance.nextSession = nil
		instance.errorSession(session, err)
		return
	}

	log.Debug().
		Msgf("🔵 runner assign next session: %s", preparedSession.ID)

	instance.queuedSession = nil
	instance.nextSession = preparedSession
}

/*



	EVENT HANDLERS



*/

func (instance *ModelInstance) errorSession(session *types.Session, err error) {
	apiUpdateErr := instance.responseHandler(&types.RunnerTaskResponse{
		Type:      types.WorkerTaskResponseTypeResult,
		SessionID: session.ID,
		Error:     err.Error(),
	})

	if apiUpdateErr != nil {
		log.Error().Msgf("Error reporting error to api: %v\n", apiUpdateErr.Error())
	}
}

/*



	PROCESS MANAGEMENT



*/

// we call this function from the text processors
func (instance *ModelInstance) taskResponseHandler(taskResponse *types.RunnerTaskResponse) {
	if instance.currentSession == nil {
		log.Error().Msgf("no current session")
		return
	}
	if instance.currentSession.ID != taskResponse.SessionID {
		log.Error().Msgf("current session ID mis-match: current=%s vs event=%s", instance.currentSession.ID, taskResponse.SessionID)
		return
	}
	taskResponse.Owner = instance.currentSession.Owner
	instance.lastActivityTimestamp = time.Now().Unix()

	var err error

	// if it's the final result then we need to upload the files first
	if taskResponse.Type == types.WorkerTaskResponseTypeResult {
		taskResponse, err = instance.fileHandler.uploadWorkerResponse(taskResponse)
		if err != nil {
			log.Error().Msgf("error uploading task result files: %s", err.Error())
			instance.currentSession = nil
			return
		}

		instance.currentSession = nil
	}

	// this will emit to the controller handler
	// i.e. the function defined in createModelInstance
	err = instance.responseHandler(taskResponse)
	if err != nil {
		log.Error().Msgf("error writing event: %s", err.Error())
		return
	}
}

// run the model process
// we pass the instance context in so we can cancel it using our stopProcess function
func (instance *ModelInstance) startProcess(session *types.Session) error {
	cmd, err := instance.model.GetCommand(instance.ctx, instance.filter, types.RunnerProcessConfig{
		InstanceID:        instance.id,
		NextTaskURL:       instance.nextTaskURL,
		InitialSessionURL: instance.initialSessionURL,
	})
	if err != nil {
		return err
	}
	if cmd == nil {
		return fmt.Errorf("no command to run")
	}

	log.Info().
		Msgf("🟢 run model instance: %s, %+v", cmd.Dir, cmd.Args)

	log.Info().
		Msgf("🟢 initial session: %s, %+v", session.ID, session)

	instance.currentCommand = cmd

	// Create pipes for stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// this buffer is so we can keep the last 10kb of stderr so if
	// there is an error we can send it to the api
	stderrBuf := system.NewLimitedBuffer(1024 * 10)

	stdoutWriters := []io.Writer{os.Stdout}
	stderrWriters := []io.Writer{os.Stderr, stderrBuf}

	// create the model textsream
	// this is responsible for chunking stdout into session outputs
	// and keeping track of the current session
	// each model knows how to parse it's own stdout differently
	// we pass a 'textStreamProcessor' function which will get events:
	//  * a new session has started
	//  * some more text has been generated (i.e. streaming output)
	//  * the result has been generated
	// in all cases - each model get's to decide what formatting
	// it's Python needs to use so that these text streams will
	// parse correctly
	stdout, stderr, err := instance.model.GetTextStreams(session.Mode, instance.taskResponseHandler)
	if err != nil {
		return err
	}

	if stdout != nil {
		go stdout.Start()
		stdoutWriters = append(stdoutWriters, stdout)
	}

	if stderr != nil {
		go stderr.Start()
		stderrWriters = append(stderrWriters, stderr)
	}

	go func() {
		_, err := io.Copy(io.MultiWriter(stdoutWriters...), stdoutPipe)
		if err != nil {
			log.Error().Msgf("Error copying stdout: %v", err)
		}
	}()

	// stream stderr to os.Stderr (so we can see it in the logs)
	// and also the error buffer we will use to post the error to the api
	go func() {
		_, err := io.Copy(io.MultiWriter(stderrWriters...), stderrPipe)
		if err != nil {
			log.Error().Msgf("Error copying stderr: %v", err)
		}
	}()

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		log.Error().Msgf("Failed to start command: %v\n", err.Error())
		return err
	}

	go func(cmd *exec.Cmd) {
		if err = cmd.Wait(); err != nil {
			log.Error().Msgf("Command ended with an error: %v\n", err.Error())

			// we are currently running a session and we got an error from the Python process
			// this normally means that a job caused an error so let's tell the api
			// that this interaction has it's Error field set
			if instance.currentSession != nil {
				instance.errorSession(instance.currentSession, err)
			}
		}

		log.Info().
			Msgf("🟢 stop model instance, exit code=%d", cmd.ProcessState.ExitCode())

		instance.finishChan <- true
	}(cmd)
	return nil
}

func (instance *ModelInstance) stopProcess() error {
	if instance.currentCommand == nil {
		return fmt.Errorf("no process to stop")
	}
	log.Info().Msgf("🟢 stop model process")
	if err := syscall.Kill(-instance.currentCommand.Process.Pid, syscall.SIGKILL); err != nil {
		log.Error().Msgf("error stopping model process: %s", err.Error())
		return err
	}
	log.Info().Msgf("🟢 stopped model process")
	return nil
}
