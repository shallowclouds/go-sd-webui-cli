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
	cli                *http.Client
	baseURL            string
	username, password string
}

// NewClient creates the API client, leave username and password empty if not set.
func NewClient(baseURL, username, password string, httpCli *http.Client) (*Client, error) {
	if len(baseURL) == 0 {
		baseURL = "http://127.0.0.1:7860"
	}
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
	// If any.
	if len(c.username) != 0 && len(c.password) != 0 {
		req.SetBasicAuth(c.username, c.password)
	}

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

func Img2RawBase64(img image.Image) string {
	buf := &bytes.Buffer{}
	png.Encode(buf, img)

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func Img2Base64(img image.Image) string {
	buf := &bytes.Buffer{}
	png.Encode(buf, img)

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func ImgBytes2Base64(data []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

type Txt2ImageOption struct {
	Prompt                            string           `json:"prompt,omitempty"`
	NegativePrompt                    string           `json:"negative_prompt,omitempty"`
	Steps                             int              `json:"steps,omitempty"`
	CfgScale                          float32          `json:"cfg_scale,omitempty"`
	Width                             int              `json:"width,omitempty"`
	Height                            int              `json:"height,omitempty"`
	SamplerIndex                      string           `json:"sampler_index,omitempty"`
	OverrideSettings                  *OptionsResponse `json:"override_settings,omitempty"`
	EnableHR                          bool             `json:"enable_hr,omitempty"`
	DenoisingStrength                 float32          `json:"denoising_strenght,omitempty"`
	FirstPhaseWidth                   int              `json:"firstphase_width,omitempty"`
	FirstPhaseHeight                  int              `json:"firstphase_height,omitempty"`
	HRScale                           float32          `json:"hr_scale,omitempty"`
	HrUpscaler                        string           `json:"hr_upscaler,omitempty"`
	HrSecondPassSteps                 int              `json:"hr_second_pass_steps,omitempty"`
	HrResizeX                         int              `json:"hr_resize_x,omitempty"`
	HrResizeY                         int              `json:"hr_resize_y,omitempty"`
	Styles                            []string         `json:"styles,omitempty"`
	Seed                              int              `json:"seed,omitempty"`
	Subseed                           int              `json:"subseed,omitempty"`
	SubseedStrength                   float32          `json:"subseed_strength,omitempty"`
	SeedResizeFromH                   int              `json:"seed_resize_from_h,omitempty"`
	SeedResizeFromW                   int              `json:"seed_resize_from_w,omitempty"`
	SamplerName                       string           `json:"sampler_name,omitempty"`
	BatchSize                         int              `json:"batch_size,omitempty"`
	NIter                             int              `json:"n_iter,omitempty"`
	RestoreFaces                      bool             `json:"restore_faces,omitempty"`
	Tiling                            bool             `json:"tiling,omitempty"`
	Eta                               float32          `json:"eta,omitempty"`
	SChurn                            float32          `json:"s_churn,omitempty"`
	STmax                             float32          `json:"s_tmax,omitempty"`
	STmin                             float32          `json:"s_tmin,omitempty"`
	SNoise                            float32          `json:"s_noise,omitempty"`
	OverrideSettingsRestoreAfterwards bool             `json:"override_settings_restore_afterwards,omitempty"`
	ScriptArgs                        []interface{}    `json:"script_args,omitempty"`
	ScriptName                        string           `json:"script_name,omitempty"`
}

type Txt2ImageResponse struct {
	Images     []string         `json:"images"`
	Parameters *Txt2ImageOption `json:"parameters"`
	Info       string           `json:"info"`

	ParsedImages []image.Image `json:"-"`
	RawImages    [][]byte      `json:"-"`
}

func (c *Client) Txt2Img(ctx context.Context, opt Txt2ImageOption) (*Txt2ImageResponse, error) {
	res := new(Txt2ImageResponse)
	if err := c.doReq(ctx, "/txt2img", http.MethodPost, &opt, http.StatusOK, res); err != nil {
		return nil, err
	}

	imgs := make([]image.Image, 0, len(res.Images))
	raws := make([][]byte, 0, len(res.Images))

	for _, raw := range res.Images {
		raw = strings.SplitN(raw, ",", 1)[0]
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			// Should not happen.
			continue
		}

		raws = append(raws, data)

		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			// Should not happen.
			continue
		}

		imgs = append(imgs, img)
	}

	res.ParsedImages = imgs
	res.RawImages = raws

	return res, nil
}

type Img2ImgOption struct {
	InitImages                        []string         `json:"init_images,omitempty"`
	ResizeMode                        int              `json:"resize_mode,omitempty"`
	DenoisingStrength                 float32          `json:"denoising_strength,omitempty"`
	ImageCfgScale                     float32          `json:"image_cfg_scale,omitempty"`
	Mask                              string           `json:"mask,omitempty"`
	MaskBlur                          int              `json:"mask_blur,omitempty"`
	InpaintingFill                    int              `json:"inpainting_fill,omitempty"`
	InpaintFullRes                    bool             `json:"inpaint_full_res,omitempty"`
	InpaintFullResPadding             int              `json:"inpaint_full_res_padding,omitempty"`
	InpaintingMaskInvert              int              `json:"inpainting_mask_invert,omitempty"`
	InitialNoiseMultiplier            int              `json:"initial_noise_multiplier,omitempty"`
	Prompt                            string           `json:"prompt,omitempty"`
	Styles                            []string         `json:"styles,omitempty"`
	Seed                              int              `json:"seed,omitempty"`
	Subseed                           int              `json:"subseed,omitempty"`
	SubseedStrength                   float32          `json:"subseed_strength,omitempty"`
	SeedResizeFromH                   int              `json:"seed_resize_from_h,omitempty"`
	SeedResizeFromW                   int              `json:"seed_resize_from_w,omitempty"`
	SamplerName                       string           `json:"sampler_name,omitempty"`
	BatchSize                         int              `json:"batch_size,omitempty"`
	NIter                             int              `json:"n_iter,omitempty"`
	Steps                             int              `json:"steps,omitempty"`
	CfgScale                          float32          `json:"cfg_scale,omitempty"`
	Width                             int              `json:"width,omitempty"`
	Height                            int              `json:"height,omitempty"`
	RestoreFaces                      bool             `json:"restore_faces,omitempty"`
	Tiling                            bool             `json:"tiling,omitempty"`
	NegativePrompt                    string           `json:"negative_prompt,omitempty"`
	Eta                               float32          `json:"eta,omitempty"`
	SChurn                            float32          `json:"s_churn,omitempty"`
	STmax                             float32          `json:"s_tmax,omitempty"`
	STmin                             float32          `json:"s_tmin,omitempty"`
	SNoise                            int              `json:"s_noise,omitempty"`
	OverrideSettings                  *OptionsResponse `json:"override_settings,omitempty"`
	OverrideSettingsRestoreAfterwards bool             `json:"override_settings_restore_afterwards,omitempty"`
	ScriptArgs                        []interface{}    `json:"script_args,omitempty"`
	SamplerIndex                      string           `json:"sampler_index,omitempty"`
	IncludeInitImages                 bool             `json:"include_init_images,omitempty"`
	ScriptName                        string           `json:"script_name,omitempty"`
}

type Img2ImgResponse struct {
	Images     []string         `json:"images"`
	Parameters *Txt2ImageOption `json:"parameters"`
	Info       string           `json:"info"`

	ParsedImages []image.Image `json:"-"`
	RawImages    [][]byte      `json:"-"`
}

func (c *Client) Img2Img(ctx context.Context, opt Img2ImgOption) (*Img2ImgResponse, error) {
	res := new(Img2ImgResponse)
	if err := c.doReq(ctx, "/img2img", http.MethodPost, &opt, http.StatusOK, res); err != nil {
		return nil, err
	}

	imgs := make([]image.Image, 0, len(res.Images))
	raws := make([][]byte, 0, len(res.Images))

	for _, raw := range res.Images {
		raw = strings.SplitN(raw, ",", 1)[0]
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			// Should not happen.
			continue
		}

		raws = append(raws, data)

		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			// Should not happen.
			continue
		}

		imgs = append(imgs, img)
	}

	res.ParsedImages = imgs
	res.RawImages = raws

	return res, nil
}

const (
	UpscalerNone               = "none"
	UpscalerLanczos            = "Lanczos"
	UpscalerNearest            = "Nearest"
	UpscalerLDSR               = "LDSR"
	UpscalerBSRGAN             = "BSRGAN"
	UpscalerESRGAN4x           = "ESRGAN_4x"
	UpscalerRESRGANGeneral4xV3 = "R-ESRGAN General 4xV3"
	UpscalerScuNETGAN          = "ScuNET GAN"
	UpscalerScuNETPSNR         = "ScuNET PSNR"
	UpscalerSwinIR4x           = "SwinIR 4x"
)

type ExtraSingleImgOption struct {
	// Sets the resize mode: 0 to upscale by upscaling_resize amount, 1 to upscale up to upscaling_resize_h x upscaling_resize_w.
	ResizeMode int `json:"resize_mode,omitempty"`
	// Should the backend return the generated image?
	ShowExtrasResults bool `json:"show_extras_results,omitempty"`
	// Sets the visibility of GFPGAN, values should be between 0 and 1.
	GfpganVisibility int `json:"gfpgan_visibility,omitempty"`
	// Sets the visibility of CodeFormer, values should be between 0 and 1.
	CodeformerVisibility int `json:"codeformer_visibility,omitempty"`
	// Sets the weight of CodeFormer, values should be between 0 and 1.
	CodeformerWeight int `json:"codeformer_weight,omitempty"`
	// By how much to upscale the image, only used when resize_mode=0.
	UpscalingResize int `json:"upscaling_resize,omitempty"`
	// Target width for the upscaler to hit. Only used when resize_mode=1.
	UpscalingResizeW int `json:"upscaling_resize_w,omitempty"`
	// Target height for the upscaler to hit. Only used when resize_mode=1.
	UpscalingResizeH int `json:"upscaling_resize_h,omitempty"`
	// Should the upscaler crop the image to fit in the chosen size?
	UpscalingCrop bool `json:"upscaling_crop,omitempty"`
	// The name of the main upscaler to use, it has to be one of this list: None , Lanczos , Nearest , ESRGAN_4x , R-ESRGAN 4x+ , R-ESRGAN 4x+ Anime6B , LDSR , ScuNET GAN , ScuNET PSNR , SwinIR 4x
	Upscaler1 string `json:"upscaler_1,omitempty"`
	// The name of the secondary upscaler to use, it has to be one of this list: None , Lanczos , Nearest , ESRGAN_4x , R-ESRGAN 4x+ , R-ESRGAN 4x+ Anime6B , LDSR , ScuNET GAN , ScuNET PSNR , SwinIR 4x
	Upscaler2 string `json:"upscaler_2,omitempty"`
	// Sets the visibility of secondary upscaler, values should be between 0 and 1.
	ExtrasUpscaler2Visibility int `json:"extras_upscaler_2_visibility,omitempty"`
	// Should the upscaler run before restoring faces?
	UpscaleFirst bool `json:"upscale_first,omitempty"`
	// Image to work on, must be a Base64 string containing the image's data.
	Image string `json:"image,omitempty"`
}

type ExtraSingleImgResponse struct {
	HTMLInfo string `json:"html_info"`
	Image    string `json:"image"`

	ParsedImage image.Image `json:"-"`
	RawImage    []byte      `json:"-"`
}

func (c *Client) ExtraSingleImg(ctx context.Context, opt ExtraSingleImgOption) (*ExtraSingleImgResponse, error) {
	res := new(ExtraSingleImgResponse)
	if err := c.doReq(ctx, "/extra-single-image", http.MethodPost, &opt, http.StatusOK, res); err != nil {
		return nil, err
	}

	raw := strings.SplitN(res.Image, ",", 1)[0]
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		// Should not happen.
	} else {
		res.RawImage = data
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			// Should not happen.
		} else {
			res.ParsedImage = img
		}
	}

	return res, nil
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

func (c *Client) GetProgress(ctx context.Context, skipCurrentImg bool) (*ProgressResponse, error) {
	res := new(ProgressResponse)
	if err := c.doReq(ctx, fmt.Sprintf("/progress?skip_current_image=%v", skipCurrentImg), http.MethodGet, nil, http.StatusOK, res); err != nil {
		return nil, err
	}

	return res, nil
}

type OptionsResponse struct {
	SdModelCheckpoint string `json:"sd_model_checkpoint,omitempty"`

	SamplesSave                        bool          `json:"samples_save,omitempty"`
	SamplesFormat                      string        `json:"samples_format,omitempty"`
	SamplesFilenamePattern             string        `json:"samples_filename_pattern,omitempty"`
	SaveImagesAddNumber                bool          `json:"save_images_add_number,omitempty"`
	GridSave                           bool          `json:"grid_save,omitempty"`
	GridFormat                         string        `json:"grid_format,omitempty"`
	GridExtendedFilename               bool          `json:"grid_extended_filename,omitempty"`
	GridOnlyIfMultiple                 bool          `json:"grid_only_if_multiple,omitempty"`
	GridPreventEmptySpots              bool          `json:"grid_prevent_empty_spots,omitempty"`
	NRows                              float32       `json:"n_rows,omitempty"`
	EnablePnginfo                      bool          `json:"enable_pnginfo,omitempty"`
	SaveTxt                            bool          `json:"save_txt,omitempty"`
	SaveImagesBeforeFaceRestoration    bool          `json:"save_images_before_face_restoration,omitempty"`
	SaveImagesBeforeHighresFix         bool          `json:"save_images_before_highres_fix,omitempty"`
	SaveImagesBeforeColorCorrection    bool          `json:"save_images_before_color_correction,omitempty"`
	JpegQuality                        float32       `json:"jpeg_quality,omitempty"`
	ExportFor4Chan                     bool          `json:"export_for_4chan,omitempty"`
	ImgDownscaleThreshold              float32       `json:"img_downscale_threshold,omitempty"`
	TargetSideLength                   float32       `json:"target_side_length,omitempty"`
	UseOriginalNameBatch               bool          `json:"use_original_name_batch,omitempty"`
	UseUpscalerNameAsSuffix            bool          `json:"use_upscaler_name_as_suffix,omitempty"`
	SaveSelectedOnly                   bool          `json:"save_selected_only,omitempty"`
	DoNotAddWatermark                  bool          `json:"do_not_add_watermark,omitempty"`
	TempDir                            string        `json:"temp_dir,omitempty"`
	CleanTempDirAtStart                bool          `json:"clean_temp_dir_at_start,omitempty"`
	OutdirSamples                      string        `json:"outdir_samples,omitempty"`
	OutdirTxt2ImgSamples               string        `json:"outdir_txt2img_samples,omitempty"`
	OutdirImg2ImgSamples               string        `json:"outdir_img2img_samples,omitempty"`
	OutdirExtrasSamples                string        `json:"outdir_extras_samples,omitempty"`
	OutdirGrids                        string        `json:"outdir_grids,omitempty"`
	OutdirTxt2ImgGrids                 string        `json:"outdir_txt2img_grids,omitempty"`
	OutdirImg2ImgGrids                 string        `json:"outdir_img2img_grids,omitempty"`
	OutdirSave                         string        `json:"outdir_save,omitempty"`
	SaveToDirs                         bool          `json:"save_to_dirs,omitempty"`
	GridSaveToDirs                     bool          `json:"grid_save_to_dirs,omitempty"`
	UseSaveToDirsForUI                 bool          `json:"use_save_to_dirs_for_ui,omitempty"`
	DirectoriesFilenamePattern         string        `json:"directories_filename_pattern,omitempty"`
	DirectoriesMaxPromptWords          float32       `json:"directories_max_prompt_words,omitempty"`
	ESRGANTile                         float32       `json:"ESRGAN_tile,omitempty"`
	ESRGANTileOverlap                  float32       `json:"ESRGAN_tile_overlap,omitempty"`
	RealesrganEnabledModels            []string      `json:"realesrgan_enabled_models,omitempty"`
	UpscalerForImg2Img                 string        `json:"upscaler_for_img2img,omitempty"`
	LdsrSteps                          float32       `json:"ldsr_steps,omitempty"`
	LdsrCached                         bool          `json:"ldsr_cached,omitempty"`
	SWINTile                           float32       `json:"SWIN_tile,omitempty"`
	SWINTileOverlap                    float32       `json:"SWIN_tile_overlap,omitempty"`
	FaceRestorationModel               string        `json:"face_restoration_model,omitempty"`
	CodeFormerWeight                   float32       `json:"code_former_weight,omitempty"`
	FaceRestorationUnload              bool          `json:"face_restoration_unload,omitempty"`
	ShowWarnings                       bool          `json:"show_warnings,omitempty"`
	MemmonPollRate                     float32       `json:"memmon_poll_rate,omitempty"`
	SamplesLogStdout                   bool          `json:"samples_log_stdout,omitempty"`
	MultipleTqdm                       bool          `json:"multiple_tqdm,omitempty"`
	PrintHypernetExtra                 bool          `json:"print_hypernet_extra,omitempty"`
	UnloadModelsWhenTraining           bool          `json:"unload_models_when_training,omitempty"`
	PinMemory                          bool          `json:"pin_memory,omitempty"`
	SaveOptimizerState                 bool          `json:"save_optimizer_state,omitempty"`
	SaveTrainingSettingsToTxt          bool          `json:"save_training_settings_to_txt,omitempty"`
	DatasetFilenameWordRegex           string        `json:"dataset_filename_word_regex,omitempty"`
	DatasetFilenameJoinString          string        `json:"dataset_filename_join_string,omitempty"`
	TrainingImageRepeatsPerEpoch       float32       `json:"training_image_repeats_per_epoch,omitempty"`
	TrainingWriteCsvEvery              float32       `json:"training_write_csv_every,omitempty"`
	TrainingXattentionOptimizations    bool          `json:"training_xattention_optimizations,omitempty"`
	TrainingEnableTensorboard          bool          `json:"training_enable_tensorboard,omitempty"`
	TrainingTensorboardSaveImages      bool          `json:"training_tensorboard_save_images,omitempty"`
	TrainingTensorboardFlushEvery      float32       `json:"training_tensorboard_flush_every,omitempty"`
	SdCheckpointCache                  float32       `json:"sd_checkpoint_cache,omitempty"`
	SdVaeCheckpointCache               float32       `json:"sd_vae_checkpoint_cache,omitempty"`
	SdVae                              string        `json:"sd_vae,omitempty"`
	SdVaeAsDefault                     bool          `json:"sd_vae_as_default,omitempty"`
	InpaintingMaskWeight               float32       `json:"inpainting_mask_weight,omitempty"`
	InitialNoiseMultiplier             float32       `json:"initial_noise_multiplier,omitempty"`
	Img2ImgColorCorrection             bool          `json:"img2img_color_correction,omitempty"`
	Img2ImgFixSteps                    bool          `json:"img2img_fix_steps,omitempty"`
	Img2ImgBackgroundColor             string        `json:"img2img_background_color,omitempty"`
	EnableQuantization                 bool          `json:"enable_quantization,omitempty"`
	EnableEmphasis                     bool          `json:"enable_emphasis,omitempty"`
	EnableBatchSeeds                   bool          `json:"enable_batch_seeds,omitempty"`
	CommaPaddingBacktrack              float32       `json:"comma_padding_backtrack,omitempty"`
	CLIPStopAtLastLayers               float32       `json:"CLIP_stop_at_last_layers,omitempty"`
	UpcastAttn                         bool          `json:"upcast_attn,omitempty"`
	UseOldEmphasisImplementation       bool          `json:"use_old_emphasis_implementation,omitempty"`
	UseOldKarrasSchedulerSigmas        bool          `json:"use_old_karras_scheduler_sigmas,omitempty"`
	NoDpmppSdeBatchDeterminism         bool          `json:"no_dpmpp_sde_batch_determinism,omitempty"`
	UseOldHiresFixWidthHeight          bool          `json:"use_old_hires_fix_width_height,omitempty"`
	InterrogateKeepModelsInMemory      bool          `json:"interrogate_keep_models_in_memory,omitempty"`
	InterrogateReturnRanks             bool          `json:"interrogate_return_ranks,omitempty"`
	InterrogateClipNumBeams            float32       `json:"interrogate_clip_num_beams,omitempty"`
	InterrogateClipMinLength           float32       `json:"interrogate_clip_min_length,omitempty"`
	InterrogateClipMaxLength           float32       `json:"interrogate_clip_max_length,omitempty"`
	InterrogateClipDictLimit           float32       `json:"interrogate_clip_dict_limit,omitempty"`
	InterrogateClipSkipCategories      []interface{} `json:"interrogate_clip_skip_categories,omitempty"`
	InterrogateDeepbooruScoreThreshold float32       `json:"interrogate_deepbooru_score_threshold,omitempty"`
	DeepbooruSortAlpha                 bool          `json:"deepbooru_sort_alpha,omitempty"`
	DeepbooruUseSpaces                 bool          `json:"deepbooru_use_spaces,omitempty"`
	DeepbooruEscape                    bool          `json:"deepbooru_escape,omitempty"`
	DeepbooruFilterTags                string        `json:"deepbooru_filter_tags,omitempty"`
	ExtraNetworksDefaultView           string        `json:"extra_networks_default_view,omitempty"`
	ExtraNetworksDefaultMultiplier     float32       `json:"extra_networks_default_multiplier,omitempty"`
	SdHypernetwork                     string        `json:"sd_hypernetwork,omitempty"`
	SdLora                             string        `json:"sd_lora,omitempty"`
	LoraApplyToOutputs                 bool          `json:"lora_apply_to_outputs,omitempty"`
	ReturnGrid                         bool          `json:"return_grid,omitempty"`
	DoNotShowImages                    bool          `json:"do_not_show_images,omitempty"`
	AddModelHashToInfo                 bool          `json:"add_model_hash_to_info,omitempty"`
	AddModelNameToInfo                 bool          `json:"add_model_name_to_info,omitempty"`
	DisableWeightsAutoSwap             bool          `json:"disable_weights_auto_swap,omitempty"`
	SendSeed                           bool          `json:"send_seed,omitempty"`
	SendSize                           bool          `json:"send_size,omitempty"`
	Font                               string        `json:"font,omitempty"`
	JsModalLightbox                    bool          `json:"js_modal_lightbox,omitempty"`
	JsModalLightboxInitiallyZoomed     bool          `json:"js_modal_lightbox_initially_zoomed,omitempty"`
	ShowProgressInTitle                bool          `json:"show_progress_in_title,omitempty"`
	SamplersInDropdown                 bool          `json:"samplers_in_dropdown,omitempty"`
	DimensionsAndBatchTogether         bool          `json:"dimensions_and_batch_together,omitempty"`
	KeyeditPrecisionAttention          float32       `json:"keyedit_precision_attention,omitempty"`
	KeyeditPrecisionExtra              float32       `json:"keyedit_precision_extra,omitempty"`
	Quicksettings                      string        `json:"quicksettings,omitempty"`
	UIReorder                          string        `json:"ui_reorder,omitempty"`
	UIExtraNetworksTabReorder          string        `json:"ui_extra_networks_tab_reorder,omitempty"`
	Localization                       string        `json:"localization,omitempty"`
	ShowProgressbar                    bool          `json:"show_progressbar,omitempty"`
	LivePreviewsEnable                 bool          `json:"live_previews_enable,omitempty"`
	ShowProgressGrid                   bool          `json:"show_progress_grid,omitempty"`
	ShowProgressEveryNSteps            float32       `json:"show_progress_every_n_steps,omitempty"`
	ShowProgressType                   string        `json:"show_progress_type,omitempty"`
	LivePreviewContent                 string        `json:"live_preview_content,omitempty"`
	LivePreviewRefreshPeriod           float32       `json:"live_preview_refresh_period,omitempty"`
	HideSamplers                       []interface{} `json:"hide_samplers,omitempty"`
	EtaDdim                            float32       `json:"eta_ddim,omitempty"`
	EtaAncestral                       float32       `json:"eta_ancestral,omitempty"`
	DdimDiscretize                     string        `json:"ddim_discretize,omitempty"`
	SChurn                             float32       `json:"s_churn,omitempty"`
	STmin                              float32       `json:"s_tmin,omitempty"`
	SNoise                             float32       `json:"s_noise,omitempty"`
	EtaNoiseSeedDelta                  float32       `json:"eta_noise_seed_delta,omitempty"`
	AlwaysDiscardNextToLastSigma       bool          `json:"always_discard_next_to_last_sigma,omitempty"`
	PostprocessingEnableInMainUI       []interface{} `json:"postprocessing_enable_in_main_ui,omitempty"`
	PostprocessingOperationOrder       []interface{} `json:"postprocessing_operation_order,omitempty"`
	UpscalingMaxImagesInCache          float32       `json:"upscaling_max_images_in_cache,omitempty"`
	DisabledExtensions                 []interface{} `json:"disabled_extensions,omitempty"`
	SdCheckpointHash                   string        `json:"sd_checkpoint_hash,omitempty"`
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
