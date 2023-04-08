package sdcli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	cli     *http.Client
	baseURL string
}

func NewClient(baseURL string, httpCli *http.Client) (*Client, error) {
	cli := &Client{
		cli:     httpCli,
		baseURL: baseURL,
	}

	return cli, nil
}

type Error struct {
	Err      error
	Msg      string
	Response *http.Response
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %+v", e.Msg, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func wrapError(err error, resp *http.Response, format string, args ...any) *Error {
	return &Error{
		Err:      err,
		Msg:      fmt.Sprintf(format, args...),
		Response: resp,
	}
}

func (c *Client) doReq(ctx context.Context, path, method string, body any, expectedStatus int, result any) error {
	var (
		b   io.Reader
		err error
	)
	if body != nil {
		buf := bytes.NewBuffer(nil)
		if err = json.NewEncoder(buf).Encode(body); err != nil {
			return wrapError(err, nil, "failed to encode body")
		}
		b = buf
	}

	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/sdapi/v1%s", c.baseURL, path), b)
	if err != nil {
		return wrapError(err, nil, "failed to initialize request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.cli.Do(req)
	if err != nil {
		return wrapError(err, nil, "failed to do request")
	}

	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return wrapError(err, resp, "failed to read response body")
	}

	if resp.StatusCode != expectedStatus {
		return wrapError(nil, resp, "got bad status %d, body: %s", resp.StatusCode, string(data))
	}

	if err := json.Unmarshal(data, result); err != nil {
		return wrapError(err, resp, "failed to parse response")
	}

	return nil
}

type SdSettings struct {
}

type Txt2ImageOption struct {
	Prompt           string           `json:"prompt,omitempty"`
	NegativePrompt   string           `json:"negative_prompt,omitempty"`
	Steps            int              `json:"steps,omitempty"`
	CfgScale         int              `json:"cfg_scale,omitempty"`
	Width            int              `json:"width,omitempty"`
	Height           int              `json:"height,omitempty"`
	SamplerIndex     string           `json:"sampler_index,omitempty"`
	OverrideSettings *OptionsResponse `json:"override_settings,omitempty"`
}

type Txt2ImageResponse struct {
	Images     []string    `json:"images"`
	Parameters interface{} `json:"paramters"`
	Info       string      `json:"info"`
}

func (c *Client) Txt2Img(ctx context.Context, opt Txt2ImageOption) ([]image.Image, error) {
	res := new(Txt2ImageResponse)
	if err := c.doReq(ctx, "/txt2img", http.MethodPost, &opt, http.StatusOK, res); err != nil {
		return nil, err
	}

	imgs := make([]image.Image, 0, len(res.Images))

	for _, raw := range res.Images {
		raw = strings.SplitN(raw, ",", 1)[0]
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			fmt.Println(err)
			continue
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			fmt.Printf("png error: %v\n", err)
		}

		imgs = append(imgs, img)
	}

	return imgs, nil
}

type ProgressResponse struct {
	Progress    float32 `json:"progress"`
	ETARelative float32 `json:"eta_relative"`
	State       struct {
		Skipped       bool   `json:"skipped"`
		Interrupted   bool   `json:"interrupted"`
		Job           string `json:"job"`
		JobCount      int    `json:"job_count"`
		JobTimestamp  string `json:"job_timestamp"`
		JobNo         int    `json:"job_no"`
		SamplingStep  int    `json:"sampling_step"`
		SamplingSteps int    `json:"sampling_steps"`
	} `json:"state"`
	CurrentImage string `json:"current_image"`
	TextInfo     string `json:"textinfo"`
}

func (c *Client) Progress(ctx context.Context) (*ProgressResponse, error) {
	res := new(ProgressResponse)
	if err := c.doReq(ctx, "/progress", http.MethodGet, nil, http.StatusOK, res); err != nil {
		return nil, err
	}

	return res, nil
}

type OptionsResponse struct {
	SdModelCheckpoint string `json:"sd_model_checkpoint,omitempty"`
}

func (c *Client) GetOptions(ctx context.Context) (*OptionsResponse, error) {
	res := new(OptionsResponse)
	if err := c.doReq(ctx, "/options", http.MethodGet, nil, http.StatusOK, res); err != nil {
		return nil, err
	}

	return res, nil
}

type ModelsResponse struct {
	Title     string      `json:"title"`
	ModelName string      `json:"model_name"`
	Hash      string      `json:"hash"`
	Sha256    string      `json:"sha256"`
	Filename  string      `json:"filename"`
	Config    interface{} `json:"config"`
}

func (c *Client) GetModels(ctx context.Context) ([]*ModelsResponse, error) {
	res := []*ModelsResponse{}
	if err := c.doReq(ctx, "/sd-models", http.MethodGet, nil, http.StatusOK, &res); err != nil {
		return nil, err
	}

	return res, nil
}

type MemoryResponse struct {
	RAM struct {
		Free  int64 `json:"free"`
		Used  int64 `json:"used"`
		Total int64 `json:"total"`
	} `json:"ram"`
	Cuda struct {
		System struct {
			Free  int64 `json:"free"`
			Used  int64 `json:"used"`
			Total int64 `json:"total"`
		} `json:"system"`
		Active struct {
			Current int64 `json:"current"`
			Peak    int64 `json:"peak"`
		} `json:"active"`
		Allocated struct {
			Current int64 `json:"current"`
			Peak    int64 `json:"peak"`
		} `json:"allocated"`
		Reserved struct {
			Current int64 `json:"current"`
			Peak    int64 `json:"peak"`
		} `json:"reserved"`
		Inactive struct {
			Current int64 `json:"current"`
			Peak    int64 `json:"peak"`
		} `json:"inactive"`
		Events struct {
			Retries int `json:"retries"`
			Peak    int `json:"peak"`
		} `json:"events"`
	} `json:"cuda"`
}

func (c *Client) GetMemory(ctx context.Context) (*MemoryResponse, error) {
	res := new(MemoryResponse)
	if err := c.doReq(ctx, "/memory", http.MethodGet, nil, http.StatusOK, res); err != nil {
		return nil, err
	}

	return res, nil
}
