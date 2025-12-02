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
4. Remove Punctuation: REMOVE punctuation marks (e.g., ，。？！「」?!), BUT PRESERVE decimal points in numbers (e.g. 3.14) and percentage signs (%%). The output should contain ONLY text and numbers.
5. Semantic Integrity: Each segment MUST form a complete semantic unit. DO NOT break in the middle of subject-verb-object structures or split phrases that belong together semantically. For Chinese text, keep related clauses together (e.g., "主語+動詞+受語" should stay in one segment if possible).
6. Semantic First, Length Second: Prioritize semantic completeness over filling to max length. It's better to have a slightly shorter segment than to force-merge unrelated phrases. Only merge phrases if they are semantically connected.
7. Output Format: Pure text with separators. No markdown, no explanations.

Examples:

Input:
今天天氣真好，我們去公園野餐吧！記得帶上你最喜歡的三明治和水果。我會準備一些飲料和甜點。到時候我們可以在湖邊找個舒服的地方坐下來，享受美好的午後時光。你覺得這個計畫怎麼樣？(Max: 16)

Output:
今天天氣真好我們去公園野餐吧|||記得帶上你最喜歡的三明治和水果|||我會準備一些飲料和甜點|||到時候我們可以在湖邊|||找個舒服的地方坐下來|||享受美好的午後時光|||你覺得這個計畫怎麼樣

Input:
Welcome to the video. Today, we are going to talk about artificial intelligence. (Max: 50)

Output:
Welcome to the video|||Today we are going to talk about artificial intelligence

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
