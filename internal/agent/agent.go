package agent

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"

	"github.com/btoll/agent-pete/internal/api"
	"github.com/btoll/agent-pete/internal/db"
	"github.com/btoll/agent-pete/internal/tool"
)

//type MessageStore interface {
//	CommitMessage(int, api.ServerMessage) (int, error)
//	GetPreviousMessages(*api.Request, int)
//	GetSkills()
//}

type Agent struct {
	conversationName string
	skillsDir        string
	store            *db.DB
	skills           *AvailableSkills
	tools            map[string]tool.Tool
	logger           *slog.Logger
	requestOptions   []api.ConfigOption
}

type AvailableSkills struct {
	AvailableSkills []Skill `json:"available_skills"`
}

type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Location    string `json:"location"`
}

func New(convName string, requestOptions []api.ConfigOption, logLevel *slog.LevelVar) *Agent {
	cwd, _ := os.Getwd()
	agent := &Agent{
		conversationName: convName,
		skillsDir:        filepath.Join(cwd, ".agents/skills/"),
		store:            db.New(),
		tools:            tool.Tools,
		logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		})),
		requestOptions: requestOptions,
	}
	agent.skills = getSkills(agent.skillsDir)
	return agent
}

func getSkills(skillsDir string) *AvailableSkills {
	// TODO: stat skillsDir
	// TODO: this assumes all SKILLs.md files have frontmatter
	type frontmatter struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	skills := []Skill{}
	filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, err error) error {
		if path == ".git" || path == "skills" {
			return fs.SkipDir
		}
		if !d.IsDir() && d.Name() == "SKILL.md" {
			file, _ := os.Open(path)
			scanner := bufio.NewScanner(file)
			builder := strings.Builder{}
			var yamlDelimiter int
			for scanner.Scan() {
				line := scanner.Text()
				if yamlDelimiter == 2 {
					break
				}
				if line == "---" && yamlDelimiter < 2 {
					yamlDelimiter++
				}
				builder.WriteString(line + "\n")
			}
			s := frontmatter{}
			_ = yaml.Unmarshal([]byte(builder.String()), &s)
			skills = append(skills, Skill{
				Name:        s.Name,
				Description: s.Description,
				Location:    path,
			})
		}
		return nil
	})
	return &AvailableSkills{
		AvailableSkills: skills,
	}
}

func (a *Agent) CallTool(toolCall api.ToolCall) (string, error) {
	funcName := toolCall.Function.Name
	if t, found := tool.Tools[funcName]; found {
		switch t.Function.Name {
		case "Add":
			a, found := toolCall.Function.Arguments["a"].(float64)
			if !found {
				return "", errors.New("argument `a` not found")
			}
			b, found := toolCall.Function.Arguments["b"].(float64)
			if !found {
				return "", errors.New("argument `b` not found")
			}
			return fmt.Sprintf("%v", tool.Add(a, b)), nil
		case "ReadFile":
			if filename, found := toolCall.Function.Arguments["filename"]; found {
				return tool.ReadFile(filename.(string))
			}
			return "", errors.New("argument `filename` not found")
		case "WriteFile":
			filename, found := toolCall.Function.Arguments["filename"]
			if !found {
				return "", errors.New("argument `filename` not found")
			}
			data, found := toolCall.Function.Arguments["data"]
			if !found {
				return "", errors.New("argument `data` not found")
			}
			return tool.WriteFile(filename.(string), data.(string))
		}
	}
	return "", fmt.Errorf("tool `%s` not found", funcName)
}

func (a *Agent) CommitMessage(msg api.ServerMessage) (int, error) {
	conversationID, err := a.store.GetConversationID(a.conversationName)
	if err != nil {
		return -1, err
	}
	return a.store.CommitMessage(conversationID, msg.GetRole(), msg.GetContent())
}

func (a *Agent) ConvertTools(toolMessages []db.ToolMessage) []api.ToolCall {
	tc := make([]api.ToolCall, len(toolMessages))
	for i, toolMessage := range toolMessages {
		var m map[string]any
		_ = json.Unmarshal([]byte(toolMessage.Parameters), &m)
		tc[i] = api.ToolCall{
			ID: toolMessage.ID,
			Function: api.ToolCallFunction{
				Name:      toolMessage.Name,
				Arguments: m,
			},
		}
	}
	return tc
}

func (a *Agent) GetSystemPrompt() api.SystemMessage {
	builder := strings.Builder{}
	builder.WriteString(`
You are an agentic coding assistant with access to tools: ReadFile, WriteFile, and Add.

CRITICAL: You must call tools to complete tasks. Do not narrate or describe what you would do — actually call the tools.
`)
	b, _ := json.Marshal(a.skills)
	builder.WriteString(string(b))
	builder.WriteString(`
# How to Use Skills

**IMPORTANT: Automatic Skill Discovery**
When a user asks you to do something, FIRST check if any available skill matches their request. Then use that skill WITHOUT being explicitly told.

**To use a skill:**
1. Read the skill file from .skills/ using read_file (e.g., .skills/code_quality_analyzer.md)
2. Follow the instructions in that skill file
3. The skill may reference supporting files (scripts, benchmarks, templates) - read those as needed
4. Skills can invoke other skills (composability)

**Examples:**
- User: "Analyze this code" → Automatically use code_quality_analyzer skill
- User: "Document this project" → Automatically use technical_documentation_generator skill
- User: "Make a hello world" → Automatically use write_hello_world skill

**Key Principles:**
- Match user intent to skills automatically
- Always read the skill file first to get detailed instructions
- Skills contain step-by-step guidance - follow them precisely
- Skills may have supporting resources (scripts/, benchmarks/, templates/) - use them
- Never make up analysis - use the skill's methods and data

When you're done with a task, provide a clear response. Always be helpful and precise.
`)
	return api.SystemMessage{
		Role: "system",
		//				Content: "You are an agentic coding assistant with access to tools: ReadFile, WriteFile, and Add.\n\nCRITICAL: You must call tools to complete tasks. Do not narrate or describe what you would do — actually call the tools.\n\nWhen asked to run a skill:\n1. Call ReadFile with the skill definition file path (e.g., \"skills/problem-checker/SKILL.md\")\n2. Read and parse the exact content returned from that tool call\n3. Execute the steps described in the skill file using ReadFile and WriteFile\n4. Do not assume or hallucinate file contents — only use what tool calls return\n5. STOP after completing the requested skill. Do not read or execute any other skills unless explicitly asked.\n\nAvailable skills:\n - skills/problem-checker/SKILL.md: Evaluates problem.txt against 4 guidelines and writes results to problem_checker_results.md\n - skills/test-generation/SKILL.md: Generates a test suite in da_training_project_tests/ based on problem.txt with 2-3 intentional misalignments",
		Content: builder.String(),
	}
}

func (a *Agent) MakeRequest() (*api.Request, error) {
	request := api.NewRequest(
		a.tools,
		a.logger.WithGroup("api").With(slog.String("type", "Request")),
		a.requestOptions...,
	)
	request.Messages = append(request.Messages, a.GetSystemPrompt())
	dbMessages, err := a.GetPreviousMessages(30)
	if err != nil {
		return nil, err
	}
	for _, msg := range dbMessages {
		request.Messages = append(request.Messages, api.AssistantMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			ToolCalls: a.ConvertTools(msg.Tools),
		})
		if len(msg.Tools) > 0 {
			for _, tool := range msg.Tools {
				request.Messages = append(request.Messages, api.ToolMessage{
					Role:       "tool",
					Content:    tool.Result,
					ToolCallID: tool.ID,
				})
			}
		}
	}
	return request, nil
}

func (a *Agent) Loop() error {
	request, err := a.MakeRequest()
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("\nagent-pete > ")
		if !scanner.Scan() {
			break
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("scanner error: %v\n", err)
			break
		}
		msg := api.UserMessage{
			Role:    "user",
			Content: scanner.Text(),
		}
		request.Messages = append(request.Messages, msg)
		lastID, err := a.CommitMessage(msg)
		if err != nil {
			panic(err)
		}
		maxRetries := 3
		var loopErr error

	OuterLoop:
		for n := range maxRetries + 1 {
			loopErr = a.ExecuteAgent(request)
			if loopErr != nil {
				var networkErr *api.NetworkError
				var httpErr *api.HTTPError
				var parseErr *api.ParseError
				var unmarshalErr *api.UnmarshalError
				switch {
				case errors.As(loopErr, &networkErr):
					if !networkErr.Retryable {
						break OuterLoop
					}
				case errors.As(loopErr, &httpErr):
					if !httpErr.Retryable {
						break OuterLoop
					}
				case errors.As(loopErr, &parseErr):
					break OuterLoop
				case errors.As(loopErr, &unmarshalErr):
					break OuterLoop
				default:
					break OuterLoop
				}
				time.Sleep(time.Duration(math.Exp2(float64(n))) * time.Second)
				continue
			}
			break
		}

		status := "success"
		if loopErr != nil {
			status = "failed"
		} else {
			// TODO
		}
		err = a.store.UpdateMessageStatus(lastID, status)
		if err != nil {
			// TODO
		}
	}
	return err
}

func (a *Agent) ExecuteAgent(request *api.Request) error {
	for {
		toolCalls, lastID, err := a.ProcessResponse(request)
		if err != nil {
			return &api.InferenceError{
				Backend: "ollama",
				Model:   request.Model,
				Op:      "processResponse",
				Err:     err,
			}
		}

		if len(toolCalls) == 0 {
			break
		}

		err = a.ProcessToolCalls(request, toolCalls, lastID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) GetPreviousMessages(n int) ([]*db.Message, error) {
	conversationID, err := a.store.GetConversationID(a.conversationName)
	if err != nil {
		return nil, err
	}
	recentMessages, err := a.store.GetNRecentMessages(conversationID, n)
	if err != nil {
		return nil, err
	}
	for _, recentMessage := range recentMessages {
		if recentMessage.Role == "assistant" {
			m, err := a.store.GetToolCallsById(recentMessage.ID)
			if err != nil {
				// TODO
				continue
			}
			recentMessage.Tools = m
		}
	}
	return recentMessages, nil
}

func (a *Agent) ProcessResponse(request *api.Request) ([]api.ToolCall, int, error) {
	resp, err := request.Post()
	if err != nil {
		return nil, -1, fmt.Errorf("processResponse: %w", err)
	}
	assistantMsg := &api.AssistantMessage{
		Role:      resp.Role,
		Content:   resp.Content,
		ToolCalls: resp.Message.(*api.AssistantMessage).ToolCalls,
	}
	request.Logger.Debug("processResponse",
		slog.Any("AssistantMessage", assistantMsg),
	)
	request.Messages = append(request.Messages, assistantMsg)
	lastID, err := a.CommitMessage(assistantMsg)
	if err != nil {
		return nil, -1, err
	}
	return assistantMsg.ToolCalls, lastID, nil
}

func (a *Agent) ProcessToolCalls(request *api.Request, toolCalls []api.ToolCall, lastID int) error {
	for _, toolCall := range toolCalls {
		var content string
		res, err := a.CallTool(toolCall)
		if err != nil {
			content = err.Error()
		} else {
			content = res
		}
		request.Messages = append(request.Messages, &api.ToolMessage{
			Role:       "tool",
			ToolCallID: toolCall.ID,
			Content:    content,
		})
		request.Logger.Debug("processToolCalls",
			slog.Group("tool",
				slog.String("name", toolCall.Function.Name),
				slog.Any("arguments", toolCall.Function.Arguments),
			),
		)
		b, err := json.Marshal(toolCall.Function.Arguments)
		if err != nil {
			return err
		}
		err = a.store.CommitToolCall(lastID, toolCall.ID, toolCall.Function.Name, string(b), res)
		if err != nil {
			return err
		}
	}
	return nil
}
