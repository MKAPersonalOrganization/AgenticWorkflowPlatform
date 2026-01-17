package core

import (
	"fmt"
)

// TodoStatus represents the status of a todo item.
type TodoStatus string

const (
	TodoStatusPending    TodoStatus = "pending"
	TodoStatusInProgress TodoStatus = "in_progress"
	TodoStatusCompleted  TodoStatus = "completed"
	TodoStatusCancelled  TodoStatus = "cancelled"
)

// Todo represents a single todo item with description and status.
type Todo struct {
	Description string     `json:"description"` // The description of the todo item
	Status      TodoStatus `json:"status"`      // The current status of the todo item
}

const WriteTodosToolDescription = `This tool can help you list out the current subtasks that are required to be completed for a given user request. The list of subtasks helps you keep track of the current task, organize complex queries and help ensure that you don't miss any steps. With this list, the user can also see the current progress you are making in executing a given task.

Depending on the task complexity, you should first divide a given task into subtasks and then use this tool to list out the subtasks that are required to be completed for a given user request.
Each of the subtasks should be clear and distinct. 

Use this tool for complex queries that require multiple steps. If you find that the request is actually complex after you have started executing the user task, create a todo list and use it. If execution of the user task requires multiple steps, planning and generally is higher complexity than a simple Q&A, use this tool.

DO NOT use this tool for simple tasks that can be completed in less than 2 steps. If the user query is simple and straightforward, do not use the tool. If you can respond with an answer in a single turn then this tool is not required.

## Task state definitions

- pending: Work has not begun on a given subtask.
- in_progress: Marked just prior to beginning work on a given subtask. You should only have one subtask as in_progress at a time.
- completed: Subtask was successfully completed with no errors or issues. If the subtask required more steps to complete, update the todo list with the subtasks. All steps should be identified as completed only when they are completed.
- cancelled: As you update the todo list, some tasks are not required anymore due to the dynamic nature of the task. In this case, mark the subtasks as cancelled.


## Methodology for using this tool
1. Use this todo list as soon as you receive a user request based on the complexity of the task.
2. Keep track of every subtask that you update the list with.
3. Mark a subtask as in_progress before you begin working on it. You should only have one subtask as in_progress at a time.
4. Update the subtask list as you proceed in executing the task. The subtask list is not static and should reflect your progress and current plans, which may evolve as you acquire new information.
5. Mark a subtask as completed when you have completed it.
6. Mark a subtask as cancelled if the subtask is no longer needed.
7. You must update the todo list as soon as you start, stop or cancel a subtask. Don't batch or wait to update the todo list.


## Examples of When to Use the Todo List

<example>
User request: Create a website with a React for creating fancy logos using gemini-2.5-flash-image

ToDo list created by the agent:
1. Initialize a new React project environment (e.g., using Vite).
2. Design and build the core UI components: a text input (prompt field) for the logo description, selection controls for style parameters (if the API supports them), and an image preview area.
3. Implement state management (e.g., React Context or Zustand) to manage the user's input prompt, the API loading status (pending, success, error), and the resulting image data.
4. Create an API service module within the React app (using "fetch" or "axios") to securely format and send the prompt data via an HTTP POST request to the specified "gemini-2.5-flash-image" (Gemini model) endpoint.
5. Implement asynchronous logic to handle the API call: show a loading indicator while the request is pending, retrieve the generated image (e.g., as a URL or base64 string) upon success, and display any errors.
6. Display the returned "fancy logo" from the API response in the preview area component.
7. Add functionality (e.g., a "Download" button) to allow the user to save the generated image file.
8. Deploy the application to a web server or hosting platform.

<reasoning>
The agent used the todo list to break the task into distinct, manageable steps:
1. Building an entire interactive web application from scratch is a highly complex, multi-stage process involving setup, UI development, logic integration, and deployment.
2. The agent inferred the core functionality required for a "logo creator," such as UI controls for customization (Task 3) and an export feature (Task 7), which must be tracked as distinct goals.
3. The agent rightly inferred the requirement of an API service model for interacting with the image model endpoint.
</reasoning>
</example>


## Examples of When NOT to Use the Todo List

<example>
User request: Ensure that the test <test file> passes.

Agent:
<Goes into a loop of running the test, identifying errors, and updating the code until the test passes.>

<reasoning>
The agent did not use the todo list because this task could be completed by a tight loop of execute test->edit->execute test.
</reasoning>
</example>
`

const WriteTodosSystemPrompt = `## ` + "`write_todos`" + `

You have access to the ` + "`write_todos`" + ` tool to help you manage and plan complex objectives.
Use this tool for complex objectives to ensure that you are tracking each necessary step and giving the user visibility into your progress.
This tool is very helpful for planning complex objectives, and for breaking down these larger complex objectives into smaller steps.

It is critical that you mark todos as completed as soon as you are done with a step. Do not batch up multiple steps before marking them as completed.
For simple objectives that only require a few steps, it is better to just complete the objective directly and NOT use this tool.
Writing todos takes time and tokens, use it when it is helpful for managing complex many-step problems! But not for simple few-step requests.

## Important To-Do List Usage Notes to Remember
- The ` + "`write_todos`" + ` tool should never be called multiple times in parallel.
- Don't be afraid to revise the To-Do list as you go. New information may reveal new tasks that need to be done, or old tasks that are irrelevant.`

// WriteTodosTool is the tool for managing the todo list.
type WriteTodosTool struct{}

// NewWriteTodosTool creates a new WriteTodosTool.
func NewWriteTodosTool() *WriteTodosTool {
	return &WriteTodosTool{}
}

func (t *WriteTodosTool) Name() string {
	return "write_todos"
}

func (t *WriteTodosTool) Description() string {
	return WriteTodosToolDescription
}

func (t *WriteTodosTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"todos": map[string]any{
				"type":        "array",
				"description": "The complete list of todo items. This will replace the existing list.",
				"items": map[string]any{
					"type":        "object",
					"description": "A single todo item.",
					"properties": map[string]any{
						"description": map[string]any{
							"type":        "string",
							"description": "The description of the task.",
						},
						"status": map[string]any{
							"type":        "string",
							"enum":        []string{"pending", "in_progress", "completed", "cancelled"},
							"description": "The current status of the task.",
						},
					},
					"required": []string{"description", "status"},
				},
			},
		},
		"required": []string{"todos"},
	}
}

func (t *WriteTodosTool) Invoke(ctx ToolContext, args any) (any, error) {
	var params struct {
		Todos []Todo `json:"todos"`
	}

	if err := UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse todo arguments: %w", err)
	}

	// Validation: only one in_progress task
	inProgressCount := 0
	for _, todo := range params.Todos {
		if todo.Status == TodoStatusInProgress {
			inProgressCount++
		}
	}

	if inProgressCount > 1 {
		return nil, fmt.Errorf("invalid parameters: only one task can be \"in_progress\" at a time")
	}

	// Format response for LLM
	var llmContent string
	if len(params.Todos) > 0 {
		llmContent = "Successfully updated the todo list. The current list is now:\n"
		for i, todo := range params.Todos {
			llmContent += fmt.Sprintf("%d. [%s] %s\n", i+1, todo.Status, todo.Description)
		}
	} else {
		llmContent = "Successfully cleared the todo list."
	}

	// Return tool result with command to update state
	return NewToolResultWithCommand(
		llmContent,
		&Command{
			Update: map[string]any{
				"todos": params.Todos,
			},
		},
	), nil
}
