package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type Client struct {
	genaiClient *genai.Client
	model       *genai.GenerativeModel
}

func NewClient(apiKey string, modelName string) (*Client, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	model := client.GenerativeModel(modelName)
	return &Client{
		genaiClient: client,
		model:       model,
	}, nil
}

func (c *Client) SegmentText(text string, maxLen int) ([]string, error) {
	ctx := context.Background()
	prompt := fmt.Sprintf(`
You are a professional video subtitle editor.
Your task is to segment the provided text into natural, semantically complete subtitle lines.

Target Audience: The video is likely in Chinese or English.
Constraints:
1. Max characters per segment: %d.
2. Separator: Use "|||" to separate distinct time-based segments.
3. No Line Breaks: Do NOT use "\n" or any other line break characters within a segment.
4. Remove Punctuation: REMOVE ALL punctuation marks (e.g., ，。？！「」".,?!) from the output. The output should contain ONLY text.
5. Merge Aggressively: Ignore original punctuation for segmentation. If multiple short phrases fit within the max character limit, MERGE them into a single line. Do NOT split just because there was a comma in the original text.
6. Output Format: Pure text with separators. No markdown, no explanations.

Examples:

Input:
甚至有網友笑說台灣人對Threads的熱情已經發展出獨特的社群文化，「台灣人用Threads已經到上廁所求救沒衛生紙、吃便當沒筷子都會有人送過去的程度」 (Max: 16)

Output:
甚至有網友笑說台灣人對Threads的熱情|||已經發展出獨特的社群文化|||台灣人用Threads已經到上廁所求救|||沒衛生紙吃便當沒筷子都會有人送過去的程度

Input:
Welcome to the video. Today, we are going to talk about artificial intelligence. (Max: 50)

Output:
Welcome to the video Today we are going to talk about artificial intelligence

Text to Segment:
%s
`, maxLen, text)

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generation error: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from gemini")
	}

	var resultText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			resultText += string(txt)
		}
	}

	// Split by custom separator
	rawSegments := strings.Split(resultText, "|||")
	var segments []string
	for _, seg := range rawSegments {
		seg = strings.TrimSpace(seg)
		if seg != "" {
			segments = append(segments, seg)
		}
	}

	return segments, nil
}

func (c *Client) Close() {
	c.genaiClient.Close()
}
