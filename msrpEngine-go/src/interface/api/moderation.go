package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type ModRequest struct {
	Input string `json:"input"`
}

type ModResponse struct {
	Results []struct {
		Flagged bool `json:"flagged"`
	} `json:"results"`
}

// CheckModeration calls the free OpenAI Moderation API to detect unsafe prompts.
func CheckModeration(input string) (bool, error) {
	// If no API key is provided, we can either skip or reject. Since it's a security feature,
	// we will skip if not configured, or you could strictly require it. We will try to fetch it.
	apiKey := os.Getenv("OPENAI_MODERATION_API_KEY")
	if apiKey == "" {
		// Fallback to the system OpenAI key if a specific moderation key isn't set
		apiKey = os.Getenv("SYSTEM_RESPONDER_API_KEY")
	}

	if apiKey == "" {
		return false, nil // Skip moderation if no keys available (can be changed to true/reject depending on strictness)
	}

	reqBody, _ := json.Marshal(ModRequest{Input: input})
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/moderations", bytes.NewBuffer(reqBody))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("moderation API returned status: %d", resp.StatusCode)
	}

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	var modResp ModResponse
	if err := json.Unmarshal(bodyBytes, &modResp); err != nil {
		return false, err
	}

	if len(modResp.Results) > 0 && modResp.Results[0].Flagged {
		return true, nil // Flagged as unsafe
	}

	return false, nil
}
