package mattermost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	baseURL    string
	botToken   string
	httpClient *http.Client
}

func NewClient(baseURL, botToken string) *Client {
	return &Client{
		baseURL:    baseURL,
		botToken:   botToken,
		httpClient: &http.Client{},
	}
}

// Post represents a Mattermost post.
type Post struct {
	ID        string `json:"id,omitempty"`
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	Props     Props  `json:"props,omitempty"`
}

// Props holds post properties including attachments.
type Props struct {
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment represents a Mattermost message attachment.
type Attachment struct {
	Text    string   `json:"text,omitempty"`
	Color   string   `json:"color,omitempty"`
	Actions []Action `json:"actions,omitempty"`
	Fields  []Field  `json:"fields,omitempty"`
}

// Action represents an interactive button.
type Action struct {
	ID          string      `json:"id,omitempty"`
	Name        string      `json:"name"`
	Type        string      `json:"type,omitempty"` // "button" or "select"
	Integration Integration `json:"integration"`
}

// Integration defines what happens when the action is triggered.
type Integration struct {
	URL     string         `json:"url"`
	Context map[string]any `json:"context,omitempty"`
}

// Field represents a key-value field in an attachment.
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// DialogRequest is used to open an interactive dialog.
type DialogRequest struct {
	TriggerID string `json:"trigger_id"`
	URL       string `json:"url"`
	Dialog    Dialog `json:"dialog"`
}

// Dialog defines the dialog structure.
type Dialog struct {
	Title       string          `json:"title"`
	CallbackID  string          `json:"callback_id,omitempty"`
	Elements    []DialogElement `json:"elements"`
	SubmitLabel string          `json:"submit_label,omitempty"`
}

// DialogElement represents a form field in a dialog.
type DialogElement struct {
	DisplayName string         `json:"display_name"`
	Name        string         `json:"name"`
	Type        string         `json:"type"` // "text", "textarea", "select"
	SubType     string         `json:"subtype,omitempty"`
	Placeholder string         `json:"placeholder,omitempty"`
	HelpText    string         `json:"help_text,omitempty"`
	Optional    bool           `json:"optional"`
	Options     []SelectOption `json:"options,omitempty"`
}

// SelectOption represents an option in a select element.
type SelectOption struct {
	Text  string `json:"text"`
	Value string `json:"value"`
}

// CreatePost creates a new post in a channel.
func (c *Client) CreatePost(post *Post) (*Post, error) {
	var result Post
	if err := c.doJSON("POST", "/api/v4/posts", post, &result); err != nil {
		return nil, fmt.Errorf("create post: %w", err)
	}
	return &result, nil
}

// UpdatePost updates an existing post.
func (c *Client) UpdatePost(postID string, post *Post) (*Post, error) {
	post.ID = postID
	var result Post
	if err := c.doJSON("PUT", "/api/v4/posts/"+postID, post, &result); err != nil {
		return nil, fmt.Errorf("update post: %w", err)
	}
	return &result, nil
}

// SendDM sends a direct message to a user.
func (c *Client) SendDM(userID, message string) error {
	// First, get or create a DM channel between bot and user
	var channel struct {
		ID string `json:"id"`
	}
	payload := []string{userID, "me"}
	if err := c.doJSON("POST", "/api/v4/channels/direct", payload, &channel); err != nil {
		return fmt.Errorf("create dm channel: %w", err)
	}

	_, err := c.CreatePost(&Post{
		ChannelID: channel.ID,
		Message:   message,
	})
	return err
}

// OpenDialog opens an interactive dialog for the user.
func (c *Client) OpenDialog(req *DialogRequest) error {
	return c.doJSON("POST", "/api/v4/actions/dialogs/open", req, nil)
}

// GetChannelByName looks up a channel by team ID and name.
func (c *Client) GetChannelByName(teamID, channelName string) (string, error) {
	var channel struct {
		ID string `json:"id"`
	}
	if err := c.doJSON("GET", fmt.Sprintf("/api/v4/teams/%s/channels/name/%s", teamID, channelName), nil, &channel); err != nil {
		return "", fmt.Errorf("get channel by name: %w", err)
	}
	return channel.ID, nil
}

// GetChannel retrieves channel info by ID.
func (c *Client) GetChannel(channelID string) (*ChannelInfo, error) {
	var info ChannelInfo
	if err := c.doJSON("GET", "/api/v4/channels/"+channelID, nil, &info); err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	return &info, nil
}

// ChannelInfo holds basic channel information.
type ChannelInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	TeamID string `json:"team_id"`
}

func (c *Client) doJSON(method, path string, body any, result any) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
