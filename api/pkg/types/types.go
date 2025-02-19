package types

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type Module struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Cost     int    `json:"cost"`
	Template string `json:"template"`
}

type Interaction struct {
	ID        string      `json:"id"`
	Created   time.Time   `json:"created"`
	Updated   time.Time   `json:"updated"`
	Scheduled time.Time   `json:"scheduled"`
	Completed time.Time   `json:"completed"`
	Creator   CreatorType `json:"creator"` // e.g. User
	// this let's us know if this interaction is part of the fine tuning process
	// or if it's a chat interaction that the user is using to interact with the model
	// once it's been fine-tuned
	// for fine-tune models, we can filter out inference interactions
	// to get down to what actually matters
	Mode SessionMode `json:"mode"`
	// the ID of the runner that processed this interaction
	Runner   string            `json:"runner"`   // e.g. 0
	Message  string            `json:"message"`  // e.g. Prove pythagoras
	Progress int               `json:"progress"` // e.g. 0-100
	Files    []string          `json:"files"`    // list of filepath paths
	Finished bool              `json:"finished"` // if true, the message has finished being written to, and is ready for a response (e.g. from the other participant)
	Metadata map[string]string `json:"metadata"` // different modes and models can put values here - for example, the image fine tuning will keep labels here to display in the frontend
	State    InteractionState  `json:"state"`
	Status   string            `json:"status"`
	Error    string            `json:"error"`
	// we hoist this from files so a single interaction knows that it "Created a finetune file"
	LoraDir        string                     `json:"lora_dir"`
	DataPrepChunks map[string][]DataPrepChunk `json:"data_prep_chunks"`
	DataPrepStage  TextDataPrepStage          `json:"data_prep_stage"`
}

type InteractionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SessionOrigin struct {
	Type                SessionOriginType `json:"type"`
	ClonedSessionID     string            `json:"cloned_session_id"`
	ClonedInteractionID string            `json:"cloned_interaction_id"`
}

// gives us a quick way to add settings
type SessionMetadata struct {
	OriginalMode            SessionMode       `json:"original_mode"`
	Origin                  SessionOrigin     `json:"origin"`
	Shared                  bool              `json:"shared"`
	Avatar                  string            `json:"avatar"`
	Priority                bool              `json:"priority"`
	DocumentIDs             map[string]string `json:"document_ids"`
	DocumentGroupID         string            `json:"document_group_id"`
	ManuallyReviewQuestions bool              `json:"manually_review_questions"`
	SystemPrompt            string            `json:"system_prompt"`
	HelixVersion            string            `json:"helix_version"`
	// Evals are cool. Scores are strings of floats so we can distinguish ""
	// (not rated) from "0.0"
	EvalRunId               string   `json:"eval_run_id"`
	EvalUserScore           string   `json:"eval_user_score"`
	EvalUserReason          string   `json:"eval_user_reason"`
	EvalManualScore         string   `json:"eval_manual_score"`
	EvalManualReason        string   `json:"eval_manual_reason"`
	EvalAutomaticScore      string   `json:"eval_automatic_score"`
	EvalAutomaticReason     string   `json:"eval_automatic_reason"`
	EvalOriginalUserPrompts []string `json:"eval_original_user_prompts"`
}

// the packet we put a list of sessions into so pagination is supported and we know the total amount
type SessionsList struct {
	// the total number of sessions that match the query
	Counter *Counter `json:"counter"`
	// the list of sessions
	Sessions []*SessionSummary `json:"sessions"`
}

type SessionChatRequest struct {
	SessionID    string      `json:"session_id"` // If empty, we will start a new session
	Stream       bool        `json:"stream"`     // If true, we will stream the response
	Mode         SessionMode `json:"mode"`       // e.g. inference, finetune
	Type         SessionType `json:"type"`       // e.g. text, image
	LoraDir      string      `json:"lora_dir"`
	SystemPrompt string      `json:"system"`   // System message, only applicable when starting a new session
	Messages     []*Message  `json:"messages"` // Initial messages
	Tools        []string    `json:"tools"`    // Available tools to use in the session
	Model        string      `json:"model"`    // The model to use
}

type Message struct {
	ID        string           `json:"id"` // Interaction ID
	Role      CreatorType      `json:"role"`
	Content   MessageContent   `json:"content"`
	CreatedAt time.Time        `json:"created_at,omitempty"`
	UpdatedAt time.Time        `json:"updated_at,omitempty"`
	State     InteractionState `json:"state"`
}

type MessageContentType string

const (
	MessageContentTypeText MessageContentType = "text"
)

type MessageContent struct {
	ContentType MessageContentType `json:"content_type"` // text, image, multimodal_text
	// Parts is a list of strings or objects. For example for text, it's a list of strings, for
	// multi-modal it can be an object:
	// "parts": [
	// 		{
	// 				"content_type": "image_asset_pointer",
	// 				"asset_pointer": "file-service://file-28uHss2LgJ8HUEEVAnXa70Tg",
	// 				"size_bytes": 185427,
	// 				"width": 2048,
	// 				"height": 1020,
	// 				"fovea": null,
	// 				"metadata": null
	// 		},
	// 		"what is in the image?"
	// ]
	Parts []any `json:"parts"`
}

type Session struct {
	ID string `json:"id"`
	// name that goes in the UI - ideally autogenerated by AI but for now can be
	// named manually
	Name          string    `json:"name"`
	Created       time.Time `json:"created"`
	Updated       time.Time `json:"updated"`
	ParentSession string    `json:"parent_session"`
	// the bot this session was spawned from
	ParentBot string `json:"parent_bot"`
	// the bot this sessions lora file was added to
	ChildBot string          `json:"child_bot"`
	Metadata SessionMetadata `json:"config" gorm:"column:config;type:jsonb"` // named config for backward compat
	// e.g. inference, finetune
	Mode SessionMode `json:"mode"`
	// e.g. text, image
	Type SessionType `json:"type"`
	// huggingface model name e.g. mistralai/Mistral-7B-Instruct-v0.1 or
	// stabilityai/stable-diffusion-xl-base-1.0
	ModelName ModelName `json:"model_name"`
	// if type == finetune, we record a filestore path to e.g. lora file here
	// currently the only place you can do inference on a finetune is within the
	// session where the finetune was generated
	LoraDir string `json:"lora_dir"`
	// for now we just whack the entire history of the interaction in here, json
	// style
	Interactions Interactions `json:"interactions" gorm:"type:jsonb"`
	// uuid of owner entity
	Owner string `json:"owner"`
	// e.g. user, system, org
	OwnerType OwnerType `json:"owner_type"`
}

func (s Session) TableName() string {
	return "session"
}

type Interactions []*Interaction

func (m Interactions) Value() (driver.Value, error) {
	j, err := json.Marshal(m)
	return j, err
}

func (t *Interactions) Scan(src interface{}) error {
	source, ok := src.([]byte)
	if !ok {
		return errors.New("type assertion .([]byte) failed.")
	}
	var result Interactions
	if err := json.Unmarshal(source, &result); err != nil {
		return err
	}
	*t = result
	return nil
}

func (Interactions) GormDataType() string {
	return "json"
}

func (m SessionMetadata) Value() (driver.Value, error) {
	j, err := json.Marshal(m)
	return j, err
}

func (t *SessionMetadata) Scan(src interface{}) error {
	source, ok := src.([]byte)
	if !ok {
		return errors.New("type assertion .([]byte) failed.")
	}
	var result SessionMetadata
	if err := json.Unmarshal(source, &result); err != nil {
		return err
	}
	*t = result
	return nil
}

func (SessionMetadata) GormDataType() string {
	return "json"
}

type BotSessions struct {
	SessionID string `json:"session_id"`
	Name      string `json:"name"`
	PrePrompt string `json:"pre_prompt"`
}

type BotConfig struct {
	Description string        `json:"description"`
	Avatar      string        `json:"avatar"`
	Sessions    []BotSessions `json:"sessions"`
}

// a bot can spawn new sessions from it's finetune dir
type Bot struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	Owner     string    `json:"owner"`
	OwnerType OwnerType `json:"owner_type"`
	Config    BotConfig `json:"config"`
}

// things we can change about a session that are not interaction related
type SessionMetaUpdate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// uuid of owner entity
	Owner string `json:"owner"`
	// e.g. user, system, org
	OwnerType OwnerType `json:"owner_type"`
}

type SessionFilterModel struct {
	Mode      SessionMode `json:"mode"`
	ModelName ModelName   `json:"model_name"`
	LoraDir   string      `json:"lora_dir"`
}

type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

type SessionFilter struct {
	// e.g. inference, finetune
	Mode SessionMode `json:"mode"`
	// e.g. text, image
	Type SessionType `json:"type"`
	// huggingface model name e.g. mistralai/Mistral-7B-Instruct-v0.1 or
	// stabilityai/stable-diffusion-xl-base-1.0
	ModelName ModelName `json:"model_name"`
	// the filestore path to the file being used for finetuning
	LoraDir string `json:"lora_dir"`
	// this means "only give me sessions that will fit in this much ram"
	Memory uint64 `json:"memory"`

	// the list of model name / mode combos that we should skip over
	// normally used by runners that are running multiple types in parallel
	// who don't want another version of what they are already running
	Reject []SessionFilterModel `json:"reject"`

	// only accept sessions that were created more than this duration ago
	Older Duration `json:"older"`
}

type ApiKey struct {
	Owner     string    `json:"owner"`
	OwnerType OwnerType `json:"owner_type"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
}

type OwnerContext struct {
	Owner     string
	OwnerType OwnerType
}

type UserData struct {
	ID       string
	Email    string
	FullName string
}

type StripeUser struct {
	StripeID        string
	HelixID         string
	Email           string
	SubscriptionID  string
	SubscriptionURL string
}

type UserConfig struct {
	StripeSubscriptionActive bool   `json:"stripe_subscription_active"`
	StripeCustomerID         string `json:"stripe_customer_id"`
	StripeSubscriptionID     string `json:"stripe_subscription_id"`
}

// this lives in the database
// the ID is the keycloak user ID
// there might not be a record for every user
type UserMeta struct {
	ID     string     `json:"id"`
	Config UserConfig `json:"config"`
}

// passed between the api server and the controller
type RequestContext struct {
	Ctx       context.Context
	Admin     bool
	Owner     string
	OwnerType OwnerType
	Email     string
	FullName  string
}

type UserStatus struct {
	Admin  bool       `json:"admin"`
	User   string     `json:"user"`
	Config UserConfig `json:"config"`
}

type UserDetails struct {
	ID        string
	Username  string
	FirstName string
	LastName  string
	Email     string
}

// a single envelope that is broadcast to users
type WebsocketEvent struct {
	Type               WebsocketEventType  `json:"type"`
	SessionID          string              `json:"session_id"`
	Owner              string              `json:"owner"`
	Session            *Session            `json:"session"`
	WorkerTaskResponse *RunnerTaskResponse `json:"worker_task_response"`
}

// the context of a long running python process
// on a runner - this will be used to inject the env
// into the cmd returned by the model instance.GetCommand() function
type RunnerProcessConfig struct {
	// the id of the model instance
	InstanceID string `json:"instance_id"`
	// the URL to ask for more tasks
	// this will pop the task from the queue
	NextTaskURL string `json:"next_task_url"`
	// the URL to ask for what the session is (e.g. to know what finetune_file to load)
	// this is readonly and will not pop the session(task) from the queue
	InitialSessionURL string `json:"initial_session_url"`
	MockRunner        bool
	MockRunnerError   string
	MockRunnerDelay   int
}

// a session will run "tasks" on runners
// task's job is to take the most recent user interaction
// and add a response to it in the form of a system interaction
// the api controller will have already appended the system interaction
// to the very end of the Session.Interactions list
// our job is to fill in the Message and/or Files field of that interaction
type RunnerTask struct {
	SessionID string `json:"session_id"`
	// the string that we are calling the prompt that we will feed into the model
	Prompt string `json:"prompt"`

	// the directory that contains the lora training files
	LoraDir string `json:"lora_dir"`

	// this is the directory that contains the files used for fine tuning
	// i.e. it's the user files that will be the input to a finetune session
	DatasetDir string `json:"dataset_dir"`
}

type RunnerTaskResponse struct {
	// the python code must submit these fields back to the runner api
	Type      WorkerTaskResponseType `json:"type"`
	SessionID string                 `json:"session_id"`
	// this should be the latest system interaction
	// it is filled in by the model instance
	// based on currentSession
	InteractionID string `json:"interaction_id"`
	Owner         string `json:"owner"`
	// which fields the python code decides to fill in here depends
	// on what the type of model it is
	Message  string   `json:"message,omitempty"`  // e.g. Prove pythagoras
	Progress int      `json:"progress,omitempty"` // e.g. 0-100
	Status   string   `json:"status,omitempty"`   // e.g. updating X
	Files    []string `json:"files,omitempty"`    // list of filepath paths
	LoraDir  string   `json:"lora_dir,omitempty"`
	Error    string   `json:"error,omitempty"`
	Done     bool     `json:"done,omitempty"`
}

// this is returned by the api server so that clients can see what
// config it's using e.g. filestore prefix
type ServerConfigForFrontend struct {
	// used to prepend onto raw filestore paths to download files
	// the filestore path will have the user info in it - i.e.
	// it's a low level filestore path
	// if we are using an object storage thing - then this URL
	// can be the prefix to the bucket
	FilestorePrefix         string `json:"filestore_prefix"`
	StripeEnabled           bool   `json:"stripe_enabled"`
	SentryDSNFrontend       string `json:"sentry_dsn_frontend"`
	GoogleAnalyticsFrontend string `json:"google_analytics_frontend"`
	EvalUserID              string `json:"eval_user_id"`
}

type CreateSessionRequest struct {
	SessionID               string
	SessionMode             SessionMode
	SessionType             SessionType
	SystemPrompt            string // System message
	ParentSession           string
	ModelName               ModelName
	Owner                   string
	OwnerType               OwnerType
	UserInteractions        []*Interaction
	Priority                bool
	ManuallyReviewQuestions bool
}

type UpdateSessionRequest struct {
	SessionID       string
	UserInteraction *Interaction
	SessionMode     SessionMode
}

// a short version of a session that we keep for the dashboard
type SessionSummary struct {
	// these are all values of the last interaction
	Created       time.Time   `json:"created"`
	Updated       time.Time   `json:"updated"`
	Scheduled     time.Time   `json:"scheduled"`
	Completed     time.Time   `json:"completed"`
	SessionID     string      `json:"session_id"`
	Name          string      `json:"name"`
	InteractionID string      `json:"interaction_id"`
	ModelName     ModelName   `json:"model_name"`
	Mode          SessionMode `json:"mode"`
	Type          SessionType `json:"type"`
	Owner         string      `json:"owner"`
	LoraDir       string      `json:"lora_dir,omitempty"`
	// this is either the prompt or the summary of the training data
	Summary  string `json:"summary"`
	Priority bool   `json:"priority"`
}

type ModelInstanceState struct {
	ID               string      `json:"id"`
	ModelName        ModelName   `json:"model_name"`
	Mode             SessionMode `json:"mode"`
	LoraDir          string      `json:"lora_dir"`
	InitialSessionID string      `json:"initial_session_id"`
	// this is either the currently running session
	// or the queued session that will be run next but is currently downloading
	CurrentSession *SessionSummary   `json:"current_session"`
	JobHistory     []*SessionSummary `json:"job_history"`
	// how many seconds to wait before calling ourselves stale
	Timeout int `json:"timeout"`
	// when was the last activity seen on this instance
	LastActivity int `json:"last_activity"`
	// we let the server tell us if it thinks this
	// (even though we could work it out)
	Stale       bool   `json:"stale"`
	MemoryUsage uint64 `json:"memory"`
}

// the basic struct reported by a runner when it connects
// and keeps reporting it's status to the api server
// we expire these records after a certain amount of time
type RunnerState struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
	// the URL that the runner will POST to to get a task
	TotalMemory         uint64                `json:"total_memory"`
	FreeMemory          int64                 `json:"free_memory"`
	Labels              map[string]string     `json:"labels"`
	ModelInstances      []*ModelInstanceState `json:"model_instances"`
	SchedulingDecisions []string              `json:"scheduling_decisions"`
}

type DashboardData struct {
	SessionQueue              []*SessionSummary           `json:"session_queue"`
	Runners                   []*RunnerState              `json:"runners"`
	GlobalSchedulingDecisions []*GlobalSchedulingDecision `json:"global_scheduling_decisions"`
}

type GlobalSchedulingDecision struct {
	Created       time.Time     `json:"created"`
	RunnerID      string        `json:"runner_id"`
	SessionID     string        `json:"session_id"`
	InteractionID string        `json:"interaction_id"`
	ModelName     ModelName     `json:"model_name"`
	Mode          SessionMode   `json:"mode"`
	Filter        SessionFilter `json:"filter"`
}

// keep track of the state of the data prep
// no error means "success"
// we have a map[string][]DataPrepChunk
// where string is filename
type DataPrepChunk struct {
	Index         int    `json:"index"`
	PromptName    string `json:"prompt_name"`
	QuestionCount int    `json:"question_count"`
	Error         string `json:"error"`
}

// the thing we get from the LLM's
type DataPrepTextQuestionRaw struct {
	Question string `json:"question" yaml:"question"`
	Answer   string `json:"answer" yaml:"answer"`
}

type DataPrepTextQuestionPart struct {
	From  string `json:"from"`
	Value string `json:"value"`
}

type DataPrepTextQuestion struct {
	Conversations []DataPrepTextQuestionPart `json:"conversations"`
}

type Counter struct {
	Count int64 `json:"count"`
}

type ToolType string

const (
	ToolTypeAPI      ToolType = "api"
	ToolTypeFunction ToolType = "function"
)

type Tool struct {
	ID      string    `json:"id" gorm:"primaryKey"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	// uuid of owner entity
	Owner string `json:"owner" gorm:"index"`
	// e.g. user, system, org
	OwnerType   OwnerType `json:"owner_type"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ToolType    ToolType  `json:"tool_type"`
	// TODO: tool configuration
	// such as OpenAPI spec, function code, etc.
	Config ToolConfig `json:"config" gorm:"jsonb"`
}

type ToolConfig struct {
	API *ToolApiConfig `json:"api"`
}

func (m ToolConfig) Value() (driver.Value, error) {
	j, err := json.Marshal(m)
	return j, err
}

func (t *ToolConfig) Scan(src interface{}) error {
	source, ok := src.([]byte)
	if !ok {
		return errors.New("type assertion .([]byte) failed.")
	}
	var result ToolConfig
	if err := json.Unmarshal(source, &result); err != nil {
		return err
	}
	*t = result
	return nil
}

func (ToolConfig) GormDataType() string {
	return "json"
}

type ToolApiConfig struct {
	URL     string           `json:"url"` // Server override
	Schema  string           `json:"schema"`
	Actions []*ToolApiAction `json:"actions"` // Read-only, parsed from schema on creation

	Headers map[string]string `json:"headers"` // Headers (authentication, etc)
	Query   map[string]string `json:"query"`   // Query parameters that will be always set
}

// ToolApiConfig is parsed from the OpenAPI spec
type ToolApiAction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Method      string `json:"method"`
	Path        string `json:"path"`
}

// SessionToolBinding used to add tools to sessions
type SessionToolBinding struct {
	SessionID string `gorm:"primaryKey;index"`
	ToolID    string `gorm:"primaryKey"`
	Created   time.Time
	Updated   time.Time
}
