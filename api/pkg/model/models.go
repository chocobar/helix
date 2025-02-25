package model

import (
	"fmt"

	"github.com/helixml/helix/api/pkg/types"
)

func GetModel(modelName types.ModelName) (Model, error) {
	switch modelName {
	case types.Model_Axolotl_Mistral7b:
		return &Mistral7bInstruct01{}, nil
	case types.Model_Axolotl_SDXL:
		return &CogSDXL{}, nil
	case types.Model_Ollama_Mistral7b:
		return &OllamaMistral7bInstruct01{}, nil
	case types.Model_Ollama_Gemma7b:
		return &OllamaGemma7bInstruct01{}, nil
	default:
		return nil, fmt.Errorf("no model for model name %s", modelName)
	}
}

// rather then keep processing model names from sessions into instances of the model struct
// (just so we can ask it GetMemoryRequirements())
// this gives us an in memory cache of model instances we can quickly lookup from
func GetModels() (map[types.ModelName]Model, error) {
	models := map[types.ModelName]Model{}
	models[types.Model_Axolotl_Mistral7b] = &Mistral7bInstruct01{}
	models[types.Model_Axolotl_SDXL] = &CogSDXL{}

	// Ollama
	models[types.Model_Ollama_Mistral7b] = &OllamaMistral7bInstruct01{}
	models[types.Model_Ollama_Gemma7b] = &OllamaGemma7bInstruct01{}
	return models, nil
}

func GetLowestMemoryRequirement() (uint64, error) {
	models, err := GetModels()
	if err != nil {
		return 0, err
	}
	lowestMemoryRequirement := uint64(0)
	for _, model := range models {
		finetune := model.GetMemoryRequirements(types.SessionModeFinetune)
		if finetune > 0 && (lowestMemoryRequirement == 0 || finetune < lowestMemoryRequirement) {
			lowestMemoryRequirement = finetune
		}
		inference := model.GetMemoryRequirements(types.SessionModeInference)
		if inference > 0 && (lowestMemoryRequirement == 0 || inference < lowestMemoryRequirement) {
			lowestMemoryRequirement = inference
		}
	}
	return lowestMemoryRequirement, err
}
