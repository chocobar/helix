package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/helixml/helix/api/pkg/store"
	"github.com/helixml/helix/api/pkg/system"
	"github.com/helixml/helix/api/pkg/tools"
	"github.com/helixml/helix/api/pkg/types"
)

// listTools godoc
// @Summary List tools
// @Description List tools for the user. Tools are use by the LLMs to interact with external systems.
// @Tags    tools

// @Success 200 {object} types.Tool
// @Router /api/v1/tools [get]
// @Security BearerAuth
func (s *HelixAPIServer) listTools(rw http.ResponseWriter, r *http.Request) ([]*types.Tool, *system.HTTPError) {
	userContext := s.getRequestContext(r)

	tools, err := s.Store.ListTools(r.Context(), &store.ListToolsQuery{
		Owner:     userContext.Owner,
		OwnerType: userContext.OwnerType,
	})
	if err != nil {
		return nil, system.NewHTTPError500(err.Error())
	}

	return tools, nil
}

// createTool godoc
// @Summary Create new tool
// @Description Create new tool. Tools are used by the LLMs to interact with external systems.
// @Tags    tools

// @Success 200 {object} types.Tool
// @Param request    body types.Tool true "Request body with tool configuration. For API schemas, it can be base64 encoded.")
// @Router /api/v1/tools [post]
// @Security BearerAuth
func (s *HelixAPIServer) createTool(rw http.ResponseWriter, r *http.Request) (*types.Tool, *system.HTTPError) {
	var tool types.Tool
	err := json.NewDecoder(r.Body).Decode(&tool)
	if err != nil {
		return nil, system.NewHTTPError400("failed to decode request body, error: %s", err)
	}

	userContext := s.getRequestContext(r)

	// Getting existing tools for the user
	existingTools, err := s.Store.ListTools(r.Context(), &store.ListToolsQuery{
		Owner:     userContext.Owner,
		OwnerType: userContext.OwnerType,
	})
	if err != nil {
		return nil, system.NewHTTPError500(err.Error())
	}

	tool.Owner = userContext.Owner
	tool.OwnerType = userContext.OwnerType

	err = s.validateTool(&tool)
	if err != nil {
		return nil, system.NewHTTPError400(err.Error())
	}

	// Checking if the tool already exists
	for _, t := range existingTools {
		if t.Name == tool.Name {
			return nil, system.NewHTTPError400("tool (%s) with name %s already exists", t.ID, tool.Name)
		}
	}

	// Creating the tool
	created, err := s.Store.CreateTool(r.Context(), &tool)
	if err != nil {
		return nil, system.NewHTTPError500(err.Error())
	}

	return created, nil
}

// updateTool godoc
// @Summary Update an existing tool
// @Description Update existing tool
// @Tags    tools

// @Success 200 {object} types.Tool
// @Param request    body types.Tool true "Request body with tool configuration. For API schemas, it can be base64 encoded.")
// @Param id path string true "Tool ID"
// @Router /api/v1/tools/{id} [put]
// @Security BearerAuth
func (s *HelixAPIServer) updateTool(rw http.ResponseWriter, r *http.Request) (*types.Tool, *system.HTTPError) {
	userContext := s.getRequestContext(r)

	var tool types.Tool
	err := json.NewDecoder(r.Body).Decode(&tool)
	if err != nil {
		return nil, system.NewHTTPError400("failed to decode request body, error: %s", err)
	}

	id := getID(r)

	tool.ID = id

	err = s.validateTool(&tool)
	if err != nil {
		return nil, system.NewHTTPError400(err.Error())
	}

	// Getting existing tool
	existing, err := s.Store.GetTool(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, system.NewHTTPError404(store.ErrNotFound.Error())
		}
		return nil, system.NewHTTPError500(err.Error())
	}

	if existing.Owner != userContext.Owner {
		return nil, system.NewHTTPError404(store.ErrNotFound.Error())
	}

	tool.Owner = existing.Owner
	tool.OwnerType = existing.OwnerType

	// Updating the tool
	updated, err := s.Store.UpdateTool(r.Context(), &tool)
	if err != nil {
		return nil, system.NewHTTPError500(err.Error())
	}

	return updated, nil
}

func (s *HelixAPIServer) validateTool(tool *types.Tool) error {
	switch tool.ToolType {
	case types.ToolTypeAPI:
		// Validate the API
		if tool.Config.API == nil {
			return system.NewHTTPError400("API config is required for API tools")
		}

		if tool.Config.API.URL == "" {
			return system.NewHTTPError400("API URL is required for API tools")
		}

		if tool.Config.API.Schema == "" {
			return system.NewHTTPError400("API schema is required for API tools")
		}

		// If schema is base64 encoded, decode it
		decoded, err := base64.StdEncoding.DecodeString(tool.Config.API.Schema)
		if err == nil {
			tool.Config.API.Schema = string(decoded)
		}

		actions, err := tools.GetActionsFromSchema(tool.Config.API.Schema)
		if err != nil {
			return system.NewHTTPError400("failed to get actions from schema, error: %s", err)
		}

		if len(actions) == 0 {
			return system.NewHTTPError400("no actions found in the schema, please check the documentation for required fields (operationId, summary or description)")
		}

		tool.Config.API.Actions = actions

		_, err = s.Controller.Options.Planner.ValidateAndDefault(context.Background(), tool)
		if err != nil {
			return system.NewHTTPError400("failed to validate and default tool, error: %s", err)
		}

	default:
		return system.NewHTTPError400("invalid tool type %s, only API tools are supported at the moment", tool.ToolType)
	}

	return nil
}

// deleteTool godoc
// @Summary Delete tool
// @Description Delete tool. This removes the entry from the database, your models will not be able to use this tool anymore.
// @Tags    tools

// @Success 200
// @Param id path string true "Tool ID"
// @Router /api/v1/tools/{id} [delete]
// @Security BearerAuth
func (s *HelixAPIServer) deleteTool(rw http.ResponseWriter, r *http.Request) (*types.Tool, *system.HTTPError) {
	userContext := s.getRequestContext(r)

	id := getID(r)

	existing, err := s.Store.GetTool(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, system.NewHTTPError404(store.ErrNotFound.Error())
		}
		return nil, system.NewHTTPError500(err.Error())
	}

	if existing.Owner != userContext.Owner {
		return nil, system.NewHTTPError404(store.ErrNotFound.Error())
	}

	err = s.Store.DeleteTool(r.Context(), id)
	if err != nil {
		return nil, system.NewHTTPError500(err.Error())
	}

	return existing, nil
}
