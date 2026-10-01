package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func jsonToolResult(v any) *mcp.CallToolResult {
	data, _ := json.MarshalIndent(v, "", "  ")
	return mcp.NewToolResultText(string(data))
}

// optionalString returns a pointer to the argument when the caller passed it,
// so that an omitted field can be told apart from an empty one.
func optionalString(req mcp.CallToolRequest, key string) *string {
	if v, ok := req.GetArguments()[key]; ok {
		if s, ok := v.(string); ok {
			return &s
		}
	}
	return nil
}

func registerSpecTools(s *server.MCPServer, be Backend) {
	requireProject := func(req mcp.CallToolRequest) (string, *mcp.CallToolResult) {
		project := strings.TrimSpace(req.GetString("project", ""))
		if project == "" {
			return "", mcp.NewToolResultError("parameter 'project' is required")
		}
		return project, nil
	}
	requireSection := func(req mcp.CallToolRequest) (string, *mcp.CallToolResult) {
		id := strings.TrimSpace(req.GetString("section", ""))
		if id == "" {
			return "", mcp.NewToolResultError("parameter 'section' is required")
		}
		return id, nil
	}

	// Tool: get_project_spec
	s.AddTool(
		mcp.NewTool(
			"get_project_spec",
			mcp.WithDescription("Read the project specification, a wiki of linked pages. Without 'sections' it returns the main page (the entry point and index) plus the page list (id, title, size, outgoing links) without bodies. Links between pages are markdown links like [text](spec:<page-id>); pass page IDs in 'sections' to read only those pages, or full=true for the whole document."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithArray("sections", mcp.Description("IDs of the pages to read"), mcp.WithStringItems()),
			mcp.WithBoolean("full", mcp.Description("Return the whole specification as one markdown document (expensive on large projects)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, errRes := requireProject(req)
			if errRes != nil {
				return errRes, nil
			}
			if req.GetBool("full", false) {
				spec, err := be.GetProjectSpecification(project)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return jsonToolResult(map[string]string{"project": project, "specification": spec}), nil
			}
			if ids := req.GetStringSlice("sections", nil); len(ids) > 0 {
				sections, err := be.GetSpecSections(project, ids)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return jsonToolResult(map[string]any{"project": project, "sections": sections}), nil
			}
			infos, err := be.ListSpecSections(project)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			res := map[string]any{"project": project, "pages": infos}
			if len(infos) > 0 {
				main, err := be.GetSpecSections(project, []string{model.SpecMainID})
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				res["main"] = main[0]
				if unreachable := model.SpecUnreachable(infos); len(unreachable) > 0 {
					var ids []string
					for _, info := range infos {
						if unreachable[info.ID] {
							ids = append(ids, info.ID)
						}
					}
					res["unlinked"] = ids
				}
			}
			return jsonToolResult(res), nil
		},
	)

	// Tool: update_project_spec
	s.AddTool(
		mcp.NewTool(
			"update_project_spec",
			mcp.WithDescription("Replace the whole project specification. The markdown is split into pages at each '## ' heading; text before the first heading becomes the main page, which gets links to any page it does not reference. Prefer the per-page tools for incremental changes."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("specification", mcp.Description("Markdown content of the composite project specification"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, errRes := requireProject(req)
			if errRes != nil {
				return errRes, nil
			}
			if err := be.UpdateProjectSpecification(project, req.GetString("specification", "")); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("✔ Project specification for '%s' updated successfully.", project)), nil
		},
	)

	// Tool: add_spec_section
	s.AddTool(
		mcp.NewTool(
			"add_spec_section",
			mcp.WithDescription("Add a new page to the project specification. Remember to link it from the main page or another page with [text](spec:<page-id>) so it is reachable."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("title", mcp.Description("Page title"), mcp.Required()),
			mcp.WithString("section", mcp.Description("Page ID (lowercase letters, digits and dashes); derived from the title when omitted. Use it to create a page that is already linked.")),
			mcp.WithString("body", mcp.Description("Markdown content of the page")),
			mcp.WithNumber("position", mcp.Description("Position in the page list (the main page is always 0); omit to append at the end")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, errRes := requireProject(req)
			if errRes != nil {
				return errRes, nil
			}
			sec, err := be.AddSpecSection(project, req.GetString("section", ""), req.GetString("title", ""), req.GetString("body", ""), req.GetInt("position", -1))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("✔ Page '%s' added to '%s'.", sec.ID, project)), nil
		},
	)

	// Tool: update_spec_section
	s.AddTool(
		mcp.NewTool(
			"update_spec_section",
			mcp.WithDescription("Update the title and/or body of one specification page (use section=\"main\" for the main page). Omitted fields are left unchanged; the page ID, and so links to it, stays the same when renamed."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("section", mcp.Description("Page ID (required)"), mcp.Required()),
			mcp.WithString("title", mcp.Description("New title")),
			mcp.WithString("body", mcp.Description("New markdown content, replacing the current body")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, errRes := requireProject(req)
			if errRes != nil {
				return errRes, nil
			}
			id, errRes := requireSection(req)
			if errRes != nil {
				return errRes, nil
			}
			title, body := optionalString(req, "title"), optionalString(req, "body")
			if title == nil && body == nil {
				return mcp.NewToolResultError("provide 'title' and/or 'body'"), nil
			}
			if _, err := be.UpdateSpecSection(project, id, title, body); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("✔ Page '%s' in '%s' updated.", id, project)), nil
		},
	)

	// Tool: move_spec_section
	s.AddTool(
		mcp.NewTool(
			"move_spec_section",
			mcp.WithDescription("Move a specification page to another position in the page list. The main page always stays first."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("section", mcp.Description("Page ID (required)"), mcp.Required()),
			mcp.WithNumber("position", mcp.Description("New 0-based position (required)"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, errRes := requireProject(req)
			if errRes != nil {
				return errRes, nil
			}
			id, errRes := requireSection(req)
			if errRes != nil {
				return errRes, nil
			}
			if err := be.MoveSpecSection(project, id, req.GetInt("position", 0)); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("✔ Page '%s' in '%s' moved.", id, project)), nil
		},
	)

	// Tool: delete_spec_section
	s.AddTool(
		mcp.NewTool(
			"delete_spec_section",
			mcp.WithDescription("Delete one page of the project specification (not the main page). Links to it remain and show as missing pages, so update the pages that reference it."),
			mcp.WithString("project", mcp.Description("Project slug (required)"), mcp.Required()),
			mcp.WithString("section", mcp.Description("Page ID (required)"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			project, errRes := requireProject(req)
			if errRes != nil {
				return errRes, nil
			}
			id, errRes := requireSection(req)
			if errRes != nil {
				return errRes, nil
			}
			if err := be.DeleteSpecSection(project, id); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("✔ Page '%s' deleted from '%s'.", id, project)), nil
		},
	)
}
